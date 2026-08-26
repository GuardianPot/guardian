package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/auth"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const authenticationThrottleWindow = 5 * time.Minute

var _ auth.Repository = (*Store)(nil)

func (s *Store) CreateBootstrapToken(ctx context.Context, record auth.BootstrapTokenRecord) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var users int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM guardian_auth.users`).Scan(&users); err != nil {
		return fmt.Errorf("count local users: %w", err)
	}
	if users != 0 {
		return auth.ErrBootstrapUnavailable
	}
	if _, err := tx.Exec(ctx, `
UPDATE guardian_auth.bootstrap_tokens
SET revoked_at = $1
WHERE consumed_at IS NULL AND revoked_at IS NULL`, record.CreatedAt); err != nil {
		return fmt.Errorf("retire prior bootstrap token: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO guardian_auth.bootstrap_tokens (
    token_id, token_hash, expires_at, created_at
) VALUES ($1, $2, $3, $4)`, record.TokenID, record.TokenHash[:], record.ExpiresAt, record.CreatedAt); err != nil {
		return fmt.Errorf("insert bootstrap token: %w", err)
	}
	snapshot, err := audit.NewSnapshot(map[string]any{
		"expires_at": record.ExpiresAt.Format(time.RFC3339Nano), "single_use": true,
	})
	if err != nil {
		return err
	}
	if err := appendAuthAudit(ctx, tx, audit.Event{
		OccurredAt:    record.CreatedAt,
		Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "bootstrap-cli"},
		Action:        audit.ActionBootstrapTokenCreated,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeBootstrapToken, ID: record.TokenID},
		CorrelationID: record.TokenID,
		After:         &snapshot,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) BootstrapOwner(ctx context.Context, record auth.BootstrapOwnerRecord) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var tokenID string
	var expiresAt time.Time
	var consumedAt, revokedAt *time.Time
	err = tx.QueryRow(ctx, `
SELECT token_id::text, expires_at, consumed_at, revoked_at
FROM guardian_auth.bootstrap_tokens
WHERE token_hash = $1
FOR UPDATE`, record.TokenHash[:]).Scan(&tokenID, &expiresAt, &consumedAt, &revokedAt)
	denied := false
	if errors.Is(err, pgx.ErrNoRows) {
		tokenID = "bootstrap-" + hex.EncodeToString(record.TokenHash[:8])
		denied = true
	} else if err != nil {
		return fmt.Errorf("lock bootstrap token: %w", err)
	} else if consumedAt != nil || revokedAt != nil || !record.CreatedAt.Before(expiresAt) {
		denied = true
	}
	if !denied {
		var users int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM guardian_auth.users`).Scan(&users); err != nil {
			return fmt.Errorf("count local users: %w", err)
		}
		denied = users != 0
	}
	if denied {
		if err := appendAuthAudit(ctx, tx, audit.Event{
			OccurredAt:    record.CreatedAt,
			Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "bootstrap-api"},
			Action:        audit.ActionBootstrapFailed,
			Object:        audit.ObjectRef{Type: audit.ObjectTypeBootstrapToken, ID: tokenID},
			CorrelationID: tokenID,
		}); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return auth.ErrAuthenticationDenied
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO guardian_auth.users (
    user_id, username, role, status, password_phc, totp_seed_envelope,
    last_totp_counter, created_at, updated_at
) VALUES ($1, $2, 'owner', 'pending_mfa', $3, $4, -1, $5, $5)`,
		record.UserID, record.Username, record.PasswordPHC, record.TOTPSeedEnvelope, record.CreatedAt); err != nil {
		return fmt.Errorf("insert owner account: %w", err)
	}
	for _, hash := range record.RecoveryHashes {
		if _, err := tx.Exec(ctx, `
INSERT INTO guardian_auth.recovery_codes (user_id, code_hash, created_at)
VALUES ($1, $2, $3)`, record.UserID, hash[:], record.CreatedAt); err != nil {
			return fmt.Errorf("insert recovery code: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
UPDATE guardian_auth.bootstrap_tokens
SET consumed_at = $2
WHERE token_id = $1`, tokenID, record.CreatedAt); err != nil {
		return fmt.Errorf("consume bootstrap token: %w", err)
	}
	snapshot, err := audit.NewSnapshot(map[string]any{
		"role": auth.RoleOwner, "status": string(auth.UserPendingMFA), "username": record.Username,
	})
	if err != nil {
		return err
	}
	if err := appendAuthAudit(ctx, tx, audit.Event{
		OccurredAt:    record.CreatedAt,
		Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: record.UserID},
		Action:        audit.ActionBootstrapSucceeded,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeUser, ID: record.UserID},
		CorrelationID: tokenID,
		After:         &snapshot,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) AllowAuthentication(
	ctx context.Context,
	accountScope, sourceScope [sha256.Size]byte,
	now time.Time,
) error {
	var blockedUntil *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT MAX(blocked_until)
FROM guardian_auth.authentication_throttles
WHERE scope_hash = $1 OR scope_hash = $2`, accountScope[:], sourceScope[:]).Scan(&blockedUntil)
	if err != nil {
		return fmt.Errorf("read authentication throttle: %w", err)
	}
	if blockedUntil != nil && now.Before(*blockedUntil) {
		return auth.ErrRateLimited
	}
	return nil
}

func (s *Store) CredentialByUsername(ctx context.Context, username string) (auth.Credential, error) {
	row, err := s.queries.GetAuthCredentialByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Credential{}, auth.ErrAuthenticationDenied
	}
	if err != nil {
		return auth.Credential{}, fmt.Errorf("read auth credential: %w", err)
	}
	return credentialFromValues(row.UserID, row.Username, row.Status, row.PasswordPhc,
		row.TotpSeedEnvelope, row.LastTotpCounter), nil
}

func (s *Store) CredentialByUserID(ctx context.Context, userID string) (auth.Credential, error) {
	value, err := parseUUID(userID)
	if err != nil {
		return auth.Credential{}, auth.ErrAuthenticationDenied
	}
	row, err := s.queries.GetAuthCredentialByUserID(ctx, value)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Credential{}, auth.ErrAuthenticationDenied
	}
	if err != nil {
		return auth.Credential{}, fmt.Errorf("read auth credential: %w", err)
	}
	return credentialFromValues(row.UserID, row.Username, row.Status, row.PasswordPhc,
		row.TotpSeedEnvelope, row.LastTotpCounter), nil
}

func (s *Store) RecordAuthenticationFailure(
	ctx context.Context,
	accountScope, sourceScope [sha256.Size]byte,
	now time.Time,
) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	for _, scope := range [][sha256.Size]byte{accountScope, sourceScope} {
		if err := updateAuthThrottle(ctx, tx, scope, now); err != nil {
			return err
		}
	}
	accountID := "account-" + hex.EncodeToString(accountScope[:8])
	if err := appendAuthAudit(ctx, tx, audit.Event{
		OccurredAt:    now,
		Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "auth-api"},
		Action:        audit.ActionLoginFailed,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeUser, ID: accountID},
		CorrelationID: accountID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CompleteTOTPLogin(ctx context.Context, completion auth.LoginCompletion) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var priorStatus string
	var lastCounter int64
	err = tx.QueryRow(ctx, `
SELECT status, last_totp_counter
FROM guardian_auth.users
WHERE user_id = $1
FOR UPDATE`, completion.UserID).Scan(&priorStatus, &lastCounter)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil &&
		(priorStatus != string(auth.UserPendingMFA) && priorStatus != string(auth.UserActive) ||
			lastCounter >= completion.TOTPCounter)) {
		return auth.ErrAuthenticationDenied
	}
	if err != nil {
		return fmt.Errorf("lock TOTP credential: %w", err)
	}
	command, err := tx.Exec(ctx, `
UPDATE guardian_auth.users
SET status = 'active', last_totp_counter = $2, updated_at = $3
WHERE user_id = $1`, completion.UserID, completion.TOTPCounter, completion.OccurredAt)
	if err != nil {
		return fmt.Errorf("advance TOTP counter: %w", err)
	}
	if command.RowsAffected() != 1 {
		return auth.ErrAuthenticationDenied
	}
	if err := insertAuthSession(ctx, tx, completion.Session); err != nil {
		return err
	}
	if err := clearAuthThrottles(ctx, tx, completion.AccountScope, completion.SourceScope); err != nil {
		return err
	}
	if err := appendAuthAudit(ctx, tx, audit.Event{
		OccurredAt:    completion.OccurredAt,
		Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: completion.UserID},
		Action:        audit.ActionLoginSucceeded,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeUser, ID: completion.UserID},
		CorrelationID: completion.Session.SessionID,
	}); err != nil {
		return err
	}
	if priorStatus == string(auth.UserPendingMFA) {
		if err := appendAuthAudit(ctx, tx, audit.Event{
			OccurredAt:    completion.OccurredAt,
			Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: completion.UserID},
			Action:        audit.ActionMFAEnrolled,
			Object:        audit.ObjectRef{Type: audit.ObjectTypeUser, ID: completion.UserID},
			CorrelationID: completion.Session.SessionID,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) CompleteRecoveryLogin(ctx context.Context, completion auth.RecoveryCompletion) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var status string
	if err := tx.QueryRow(ctx, `
SELECT status FROM guardian_auth.users WHERE user_id = $1 FOR UPDATE`, completion.UserID).Scan(&status); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("lock recovery credential: %w", err)
		}
		return auth.ErrAuthenticationDenied
	}
	if status != string(auth.UserActive) {
		return auth.ErrAuthenticationDenied
	}
	command, err := tx.Exec(ctx, `
UPDATE guardian_auth.recovery_codes
SET consumed_at = $3
WHERE user_id = $1 AND code_hash = $2 AND consumed_at IS NULL`,
		completion.UserID, completion.RecoveryHash[:], completion.OccurredAt)
	if err != nil {
		return fmt.Errorf("consume recovery code: %w", err)
	}
	if command.RowsAffected() != 1 {
		return auth.ErrAuthenticationDenied
	}
	revoked, err := revokeUserSessions(ctx, tx, completion.UserID, completion.OccurredAt, "recovery_login")
	if err != nil {
		return err
	}
	if err := insertAuthSession(ctx, tx, completion.Session); err != nil {
		return err
	}
	if err := clearAuthThrottles(ctx, tx, completion.AccountScope, completion.SourceScope); err != nil {
		return err
	}
	for _, sessionID := range revoked {
		if err := appendSessionRevoked(ctx, tx, completion.UserID, sessionID, completion.Session.SessionID, completion.OccurredAt); err != nil {
			return err
		}
	}
	for _, event := range []audit.Event{
		{
			OccurredAt:    completion.OccurredAt,
			Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: completion.UserID},
			Action:        audit.ActionRecoveryCodeUsed,
			Object:        audit.ObjectRef{Type: audit.ObjectTypeUser, ID: completion.UserID},
			CorrelationID: completion.Session.SessionID,
		},
		{
			OccurredAt:    completion.OccurredAt,
			Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: completion.UserID},
			Action:        audit.ActionLoginSucceeded,
			Object:        audit.ObjectRef{Type: audit.ObjectTypeUser, ID: completion.UserID},
			CorrelationID: completion.Session.SessionID,
		},
	} {
		if err := appendAuthAudit(ctx, tx, event); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) AuthenticateSession(
	ctx context.Context,
	tokenHash [sha256.Size]byte,
	now time.Time,
	idleExpiry time.Duration,
) (auth.Session, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return auth.Session{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var result auth.Session
	var status string
	var csrfHash []byte
	var revokedAt *time.Time
	err = tx.QueryRow(ctx, `
SELECT s.session_id::text, s.user_id::text, u.username, u.role, u.status,
       s.csrf_hash, s.created_at, s.last_seen_at, s.expires_at, s.revoked_at
FROM guardian_auth.sessions s
JOIN guardian_auth.users u ON u.user_id = s.user_id
WHERE s.token_hash = $1
FOR UPDATE OF s`, tokenHash[:]).Scan(
		&result.SessionID, &result.UserID, &result.Username, &result.Role, &status,
		&csrfHash, &result.CreatedAt, &result.LastSeenAt, &result.ExpiresAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Session{}, auth.ErrSessionInvalid
	}
	if err != nil {
		return auth.Session{}, fmt.Errorf("read auth session: %w", err)
	}
	invalid := revokedAt != nil || status != string(auth.UserActive) || !now.Before(result.ExpiresAt) ||
		now.Before(result.LastSeenAt) ||
		now.Sub(result.LastSeenAt) >= idleExpiry
	if invalid {
		if revokedAt == nil {
			if _, err := tx.Exec(ctx, `
UPDATE guardian_auth.sessions
SET revoked_at = $2, revocation_reason = 'expired'
WHERE session_id = $1`, result.SessionID, now); err != nil {
				return auth.Session{}, fmt.Errorf("expire auth session: %w", err)
			}
			if err := appendSessionRevoked(ctx, tx, result.UserID, result.SessionID, result.SessionID, now); err != nil {
				return auth.Session{}, err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return auth.Session{}, err
		}
		return auth.Session{}, auth.ErrSessionInvalid
	}
	if len(csrfHash) != sha256.Size {
		return auth.Session{}, auth.ErrSessionInvalid
	}
	copy(result.CSRFHash[:], csrfHash)
	if _, err := tx.Exec(ctx, `
UPDATE guardian_auth.sessions SET last_seen_at = $2 WHERE session_id = $1`, result.SessionID, now); err != nil {
		return auth.Session{}, fmt.Errorf("touch auth session: %w", err)
	}
	result.LastSeenAt = now
	if err := tx.Commit(ctx); err != nil {
		return auth.Session{}, err
	}
	return result, nil
}

func (s *Store) RevokeSession(ctx context.Context, userID, sessionID string, now time.Time, reason string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	command, err := tx.Exec(ctx, `
UPDATE guardian_auth.sessions
SET revoked_at = $3, revocation_reason = $4
WHERE user_id = $1 AND session_id = $2 AND revoked_at IS NULL`, userID, sessionID, now, reason)
	if err != nil {
		return fmt.Errorf("revoke auth session: %w", err)
	}
	if command.RowsAffected() != 1 {
		return auth.ErrSessionInvalid
	}
	action := audit.ActionSessionRevoked
	if reason == "logout" {
		action = audit.ActionLogout
	}
	if err := appendAuthAudit(ctx, tx, audit.Event{
		OccurredAt:    now,
		Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: userID},
		Action:        action,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeSession, ID: sessionID},
		CorrelationID: sessionID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ListSessions(ctx context.Context, userID, currentSessionID string, _ time.Time) ([]auth.Session, error) {
	value, err := parseUUID(userID)
	if err != nil {
		return nil, auth.ErrSessionInvalid
	}
	rows, err := s.queries.ListAuthSessions(ctx, value)
	if err != nil {
		return nil, fmt.Errorf("list auth sessions: %w", err)
	}
	result := make([]auth.Session, 0, len(rows))
	for _, row := range rows {
		var revokedAt *time.Time
		if row.RevokedAt.Valid {
			value := row.RevokedAt.Time.UTC()
			revokedAt = &value
		}
		result = append(result, auth.Session{
			SessionID: row.SessionID, UserID: row.UserID, Username: row.Username, Role: row.Role,
			CreatedAt: row.CreatedAt.Time.UTC(), LastSeenAt: row.LastSeenAt.Time.UTC(),
			ExpiresAt: row.ExpiresAt.Time.UTC(), RevokedAt: revokedAt,
			Current: row.SessionID == currentSessionID,
		})
	}
	return result, nil
}

func (s *Store) ChangePassword(ctx context.Context, change auth.PasswordChange) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	command, err := tx.Exec(ctx, `
UPDATE guardian_auth.users SET password_phc = $2, updated_at = $3
WHERE user_id = $1 AND status = 'active'`, change.UserID, change.PasswordPHC, change.OccurredAt)
	if err != nil {
		return fmt.Errorf("change local password: %w", err)
	}
	if command.RowsAffected() != 1 {
		return auth.ErrAuthenticationDenied
	}
	revoked, err := revokeUserSessions(ctx, tx, change.UserID, change.OccurredAt, "password_changed")
	if err != nil {
		return err
	}
	if err := insertAuthSession(ctx, tx, change.Session); err != nil {
		return err
	}
	for _, sessionID := range revoked {
		if err := appendSessionRevoked(ctx, tx, change.UserID, sessionID, change.Session.SessionID, change.OccurredAt); err != nil {
			return err
		}
	}
	if err := appendAuthAudit(ctx, tx, audit.Event{
		OccurredAt:    change.OccurredAt,
		Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: change.UserID},
		Action:        audit.ActionPasswordChanged,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeUser, ID: change.UserID},
		CorrelationID: change.Session.SessionID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func updateAuthThrottle(ctx context.Context, tx pgx.Tx, scope [sha256.Size]byte, now time.Time) error {
	if _, err := tx.Exec(ctx, `
INSERT INTO guardian_auth.authentication_throttles (
    scope_hash, window_started_at, failures, updated_at
) VALUES ($1, $2, 0, $2)
ON CONFLICT (scope_hash) DO NOTHING`, scope[:], now); err != nil {
		return fmt.Errorf("initialize authentication throttle: %w", err)
	}
	var windowStarted time.Time
	var failures int
	if err := tx.QueryRow(ctx, `
SELECT window_started_at, failures
FROM guardian_auth.authentication_throttles
WHERE scope_hash = $1
FOR UPDATE`, scope[:]).Scan(&windowStarted, &failures); err != nil {
		return fmt.Errorf("lock authentication throttle: %w", err)
	}
	if now.Sub(windowStarted) >= authenticationThrottleWindow {
		windowStarted, failures = now, 0
	}
	failures++
	var blockedUntil *time.Time
	if failures >= 5 {
		delay := time.Minute << min(failures-5, 4)
		if delay > 15*time.Minute {
			delay = 15 * time.Minute
		}
		value := now.Add(delay)
		blockedUntil = &value
	}
	if _, err := tx.Exec(ctx, `
UPDATE guardian_auth.authentication_throttles
SET window_started_at = $2, failures = $3, blocked_until = $4, updated_at = $5
WHERE scope_hash = $1`, scope[:], windowStarted, failures, blockedUntil, now); err != nil {
		return fmt.Errorf("record authentication throttle: %w", err)
	}
	return nil
}

func clearAuthThrottles(
	ctx context.Context,
	tx pgx.Tx,
	accountScope, _ [sha256.Size]byte,
) error {
	if _, err := tx.Exec(ctx, `
DELETE FROM guardian_auth.authentication_throttles
WHERE scope_hash = $1`, accountScope[:]); err != nil {
		return fmt.Errorf("clear authentication throttle: %w", err)
	}
	return nil
}

func insertAuthSession(ctx context.Context, tx pgx.Tx, record auth.SessionRecord) error {
	if _, err := tx.Exec(ctx, `
INSERT INTO guardian_auth.sessions (
    session_id, user_id, token_hash, csrf_hash, created_at, last_seen_at, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)`, record.SessionID, record.UserID,
		record.TokenHash[:], record.CSRFHash[:], record.CreatedAt, record.LastSeenAt, record.ExpiresAt); err != nil {
		return fmt.Errorf("insert auth session: %w", err)
	}
	return nil
}

func revokeUserSessions(ctx context.Context, tx pgx.Tx, userID string, now time.Time, reason string) ([]string, error) {
	rows, err := tx.Query(ctx, `
UPDATE guardian_auth.sessions
SET revoked_at = $2, revocation_reason = $3
WHERE user_id = $1 AND revoked_at IS NULL
RETURNING session_id::text`, userID, now, reason)
	if err != nil {
		return nil, fmt.Errorf("revoke user sessions: %w", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return nil, err
		}
		result = append(result, sessionID)
	}
	return result, rows.Err()
}

func appendSessionRevoked(
	ctx context.Context,
	tx pgx.Tx,
	userID, sessionID, correlationID string,
	now time.Time,
) error {
	return appendAuthAudit(ctx, tx, audit.Event{
		OccurredAt:    now,
		Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: userID},
		Action:        audit.ActionSessionRevoked,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeSession, ID: sessionID},
		CorrelationID: correlationID,
	})
}

func appendAuthAudit(ctx context.Context, tx pgx.Tx, event audit.Event) error {
	if _, err := (auditQueryAppender{queries: dbgen.New(tx)}).Append(ctx, event); err != nil {
		return fmt.Errorf("append authentication audit: %w", err)
	}
	return nil
}

func parseUUID(value string) (pgtype.UUID, error) {
	var result pgtype.UUID
	if err := result.Scan(value); err != nil || !result.Valid {
		return pgtype.UUID{}, errors.New("invalid UUID")
	}
	return result, nil
}

func credentialFromValues(userID, username, status, phc string, envelope []byte, counter int64) auth.Credential {
	return auth.Credential{
		UserID: userID, Username: username, Status: auth.UserStatus(status),
		PasswordPHC: phc, TOTPSeedEnvelope: envelope, LastTOTPCounter: counter,
	}
}
