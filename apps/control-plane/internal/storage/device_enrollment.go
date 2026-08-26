package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicepki"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage/dbgen"
	"github.com/jackc/pgx/v5"
)

var _ devices.Repository = (*Store)(nil)

const throttleWindow = 5 * time.Minute

// InitializeDeviceCA persists authenticated ciphertext only and refuses silent
// replacement of an existing product CA.
func (s *Store) InitializeDeviceCA(ctx context.Context, material devicepki.Material) error {
	command, err := s.pool.Exec(ctx, `
INSERT INTO guardian_devices.certificate_authority (
    singleton, certificate_pem, private_key_envelope, not_after
) VALUES (true, $1, $2, $3)
ON CONFLICT (singleton) DO NOTHING`,
		material.CertificatePEM, material.PrivateKeyEnvelope, material.NotAfter)
	if err != nil {
		return fmt.Errorf("initialize device CA: %w", err)
	}
	if command.RowsAffected() != 1 {
		return errors.New("device CA is already initialized")
	}
	return nil
}

// DeviceCAMaterial loads encrypted CA material without opening it.
func (s *Store) DeviceCAMaterial(ctx context.Context) (devicepki.Material, error) {
	var material devicepki.Material
	err := s.pool.QueryRow(ctx, `
SELECT certificate_pem, private_key_envelope, not_after
FROM guardian_devices.certificate_authority
WHERE singleton = true`).Scan(
		&material.CertificatePEM, &material.PrivateKeyEnvelope, &material.NotAfter)
	if errors.Is(err, pgx.ErrNoRows) {
		return devicepki.Material{}, errors.New("device CA is not initialized")
	}
	if err != nil {
		return devicepki.Material{}, fmt.Errorf("load device CA: %w", err)
	}
	material.NotAfter = material.NotAfter.UTC()
	return material, nil
}

func (s *Store) CreateEnrollmentToken(ctx context.Context, record devices.EnrollmentTokenRecord, actorID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin enrollment token transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `
INSERT INTO guardian_devices.devices (
    device_id, environment_id, display_name, state, created_at, updated_at
) VALUES ($1, $2, $3, 'pending', $4, $4)`,
		record.DeviceID, record.EnvironmentID, record.DeviceName, record.CreatedAt); err != nil {
		return fmt.Errorf("insert pending device: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO guardian_devices.enrollment_tokens (
    token_id, device_id, token_hash, expires_at, created_at
) VALUES ($1, $2, $3, $4, $5)`,
		record.TokenID, record.DeviceID, record.TokenHash[:], record.ExpiresAt, record.CreatedAt); err != nil {
		return fmt.Errorf("insert enrollment token: %w", err)
	}
	snapshot, err := audit.NewSnapshot(map[string]any{
		"device_id": record.DeviceID, "environment_id": record.EnvironmentID,
		"expires_at": record.ExpiresAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	_, err = (auditQueryAppender{queries: dbgen.New(tx)}).Append(ctx, audit.Event{
		OccurredAt:    record.CreatedAt,
		Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: actorID},
		Action:        audit.ActionEnrollmentTokenCreated,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeEnrollmentToken, ID: record.TokenID},
		CorrelationID: record.TokenID,
		After:         &snapshot,
	})
	if err != nil {
		return fmt.Errorf("append enrollment token audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit enrollment token transaction: %w", err)
	}
	return nil
}

// CreateReenrollmentToken explicitly re-enables the stable device record while
// permanently retiring all prior authentication material in one transaction.
func (s *Store) CreateReenrollmentToken(ctx context.Context, record devices.EnrollmentTokenRecord, actorID string) (string, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return "", fmt.Errorf("begin re-enrollment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var deviceName, previousState string
	err = tx.QueryRow(ctx, `
SELECT display_name, state
FROM guardian_devices.devices
WHERE device_id = $1 AND environment_id = $2
FOR UPDATE`, record.DeviceID, record.EnvironmentID).Scan(&deviceName, &previousState)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", devices.ErrInvalidInput
	}
	if err != nil {
		return "", fmt.Errorf("lock device for re-enrollment: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE guardian_devices.enrollment_tokens
SET revoked_at = $2
WHERE device_id = $1 AND consumed_at IS NULL AND revoked_at IS NULL`,
		record.DeviceID, record.CreatedAt); err != nil {
		return "", fmt.Errorf("retire prior enrollment tokens: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE guardian_devices.certificates
SET state = 'revoked', revoked_at = $2
WHERE device_id = $1 AND state = 'active'`, record.DeviceID, record.CreatedAt); err != nil {
		return "", fmt.Errorf("retire prior device certificate: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE guardian_devices.devices
SET state = 'pending', updated_at = $2
WHERE device_id = $1`, record.DeviceID, record.CreatedAt); err != nil {
		return "", fmt.Errorf("re-enable device for re-enrollment: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO guardian_devices.enrollment_tokens (
    token_id, device_id, token_hash, expires_at, created_at
) VALUES ($1, $2, $3, $4, $5)`,
		record.TokenID, record.DeviceID, record.TokenHash[:], record.ExpiresAt, record.CreatedAt); err != nil {
		return "", fmt.Errorf("insert re-enrollment token: %w", err)
	}
	snapshot, err := audit.NewSnapshot(map[string]any{
		"device_id": record.DeviceID, "environment_id": record.EnvironmentID,
		"expires_at":     record.ExpiresAt.Format(time.RFC3339Nano),
		"previous_state": previousState, "re_enrollment": true,
	})
	if err != nil {
		return "", err
	}
	_, err = (auditQueryAppender{queries: dbgen.New(tx)}).Append(ctx, audit.Event{
		OccurredAt:    record.CreatedAt,
		Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: actorID},
		Action:        audit.ActionEnrollmentTokenCreated,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeEnrollmentToken, ID: record.TokenID},
		CorrelationID: record.TokenID,
		After:         &snapshot,
	})
	if err != nil {
		return "", fmt.Errorf("append re-enrollment token audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit re-enrollment transaction: %w", err)
	}
	return deviceName, nil
}

func (s *Store) ListEnrollmentTokens(ctx context.Context, environmentID string) ([]devices.TokenSummary, error) {
	rows, err := s.pool.Query(ctx, `
SELECT t.token_id::text, t.device_id::text, d.environment_id::text,
       d.display_name, t.expires_at, t.consumed_at, t.revoked_at
FROM guardian_devices.enrollment_tokens t
JOIN guardian_devices.devices d ON d.device_id = t.device_id
WHERE d.environment_id = $1
ORDER BY t.created_at DESC, t.token_id DESC
LIMIT 200`, environmentID)
	if err != nil {
		return nil, fmt.Errorf("list enrollment tokens: %w", err)
	}
	defer rows.Close()
	result := []devices.TokenSummary{}
	for rows.Next() {
		var item devices.TokenSummary
		if err := rows.Scan(&item.TokenID, &item.DeviceID, &item.EnvironmentID, &item.DeviceName,
			&item.ExpiresAt, &item.ConsumedAt, &item.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan enrollment token: %w", err)
		}
		item.ExpiresAt = item.ExpiresAt.UTC()
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enrollment tokens: %w", err)
	}
	return result, nil
}

func (s *Store) RevokeEnrollmentToken(ctx context.Context, environmentID, tokenID, actorID, correlationID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var deviceID string
	err = tx.QueryRow(ctx, `
UPDATE guardian_devices.enrollment_tokens t
SET revoked_at = clock_timestamp()
FROM guardian_devices.devices d
WHERE t.token_id = $1 AND t.device_id = d.device_id AND d.environment_id = $2
  AND t.consumed_at IS NULL AND t.revoked_at IS NULL
RETURNING t.device_id::text`, tokenID, environmentID).Scan(&deviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return devices.ErrTokenInvalid
	}
	if err != nil {
		return fmt.Errorf("revoke enrollment token: %w", err)
	}
	_, err = (auditQueryAppender{queries: dbgen.New(tx)}).Append(ctx, audit.Event{
		OccurredAt:    time.Now().UTC(),
		Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: actorID},
		Action:        audit.ActionEnrollmentTokenRevoked,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeEnrollmentToken, ID: tokenID},
		CorrelationID: correlationID,
	})
	if err != nil {
		return fmt.Errorf("append token revocation audit: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *Store) Enroll(
	ctx context.Context,
	tokenHash [sha256.Size]byte,
	now time.Time,
	issue func(string) (devices.Certificate, error),
) (devices.Certificate, error) {
	if issue == nil {
		return devices.Certificate{}, errors.New("certificate issuer is required")
	}
	// ReadCommitted plus the token/device row locks serializes contenders while
	// allowing the loser to observe consumed_at and return a bounded replay
	// denial instead of leaking a PostgreSQL serialization error.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return devices.Certificate{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var tokenID, deviceID, environmentID, state string
	var expiresAt time.Time
	var consumedAt, revokedAt *time.Time
	err = tx.QueryRow(ctx, `
SELECT t.token_id::text, t.device_id::text, d.environment_id::text, d.state, t.expires_at,
       t.consumed_at, t.revoked_at
FROM guardian_devices.enrollment_tokens t
JOIN guardian_devices.devices d ON d.device_id = t.device_id
WHERE t.token_hash = $1
FOR UPDATE OF t, d`, tokenHash[:]).Scan(
		&tokenID, &deviceID, &environmentID, &state, &expiresAt, &consumedAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return devices.Certificate{}, devices.ErrTokenInvalid
	}
	if err != nil {
		return devices.Certificate{}, fmt.Errorf("lock enrollment token: %w", err)
	}
	var denial error
	switch {
	case revokedAt != nil:
		denial = devices.ErrTokenRevoked
	case consumedAt != nil:
		denial = devices.ErrTokenConsumed
	case !now.Before(expiresAt):
		denial = devices.ErrTokenExpired
	case state == string(devices.DeviceDisabled):
		denial = devices.ErrDeviceDisabled
	case state == string(devices.DeviceRevoked):
		denial = devices.ErrDeviceRevoked
	}
	if denial != nil {
		if err := appendEnrollmentFailure(ctx, tx, deviceID, tokenID, now, denial); err != nil {
			return devices.Certificate{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return devices.Certificate{}, err
		}
		return devices.Certificate{}, denial
	}
	certificate, err := issue(deviceID)
	if err != nil {
		if auditErr := appendEnrollmentFailure(ctx, tx, deviceID, tokenID, now, devices.ErrTokenInvalid); auditErr != nil {
			return devices.Certificate{}, errors.Join(err, auditErr)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return devices.Certificate{}, errors.Join(err, commitErr)
		}
		return devices.Certificate{}, err
	}
	certificate.EnvironmentID = environmentID
	if _, err := tx.Exec(ctx, `
INSERT INTO guardian_devices.certificates (
    serial, device_id, fingerprint_sha256, certificate_pem,
    not_before, not_after, state, created_at
) VALUES ($1, $2, $3, $4, $5, $6, 'active', $7)`,
		certificate.Serial, deviceID, certificate.Fingerprint, certificate.PEM,
		certificate.NotBefore, certificate.NotAfter, now); err != nil {
		return devices.Certificate{}, fmt.Errorf("insert device certificate: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE guardian_devices.enrollment_tokens SET consumed_at = $2 WHERE token_id = $1`,
		tokenID, now); err != nil {
		return devices.Certificate{}, fmt.Errorf("consume enrollment token: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE guardian_devices.devices SET state = 'active', updated_at = $2 WHERE device_id = $1`,
		deviceID, now); err != nil {
		return devices.Certificate{}, fmt.Errorf("activate enrolled device: %w", err)
	}
	queries := dbgen.New(tx)
	for _, event := range []audit.Event{
		{
			OccurredAt: now, Actor: audit.Actor{Type: audit.ActorTypeDevice, ID: deviceID},
			Action:        audit.ActionEnrollmentSucceeded,
			Object:        audit.ObjectRef{Type: audit.ObjectTypeDevice, ID: deviceID},
			CorrelationID: tokenID,
		},
		{
			OccurredAt: now, Actor: audit.Actor{Type: audit.ActorTypeSystem, ID: "device-ca"},
			Action:        audit.ActionCertificateIssued,
			Object:        audit.ObjectRef{Type: audit.ObjectTypeDeviceCertificate, ID: certificate.Serial},
			CorrelationID: tokenID,
		},
	} {
		if _, err := (auditQueryAppender{queries: queries}).Append(ctx, event); err != nil {
			return devices.Certificate{}, fmt.Errorf("append enrollment audit: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return devices.Certificate{}, fmt.Errorf("commit enrollment: %w", err)
	}
	return certificate, nil
}

func (s *Store) Rotate(
	ctx context.Context,
	deviceID, currentSerial string,
	now time.Time,
	issue func(string) (devices.Certificate, error),
) (devices.Certificate, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return devices.Certificate{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var state, environmentID string
	var notAfter time.Time
	err = tx.QueryRow(ctx, `
SELECT d.state, d.environment_id::text, c.not_after
FROM guardian_devices.devices d
JOIN guardian_devices.certificates c ON c.device_id = d.device_id
WHERE d.device_id = $1 AND c.serial = $2 AND c.state = 'active'
FOR UPDATE OF d, c`, deviceID, currentSerial).Scan(&state, &environmentID, &notAfter)
	if errors.Is(err, pgx.ErrNoRows) {
		return devices.Certificate{}, devices.ErrCertificateStale
	}
	if err != nil {
		return devices.Certificate{}, fmt.Errorf("lock active certificate: %w", err)
	}
	if state == string(devices.DeviceDisabled) {
		return devices.Certificate{}, devices.ErrDeviceDisabled
	}
	if state == string(devices.DeviceRevoked) {
		return devices.Certificate{}, devices.ErrDeviceRevoked
	}
	if !devicepki.RotationDue(notAfter, now) {
		return devices.Certificate{}, errors.New("certificate is outside the rotation window")
	}
	certificate, err := issue(deviceID)
	if err != nil {
		return devices.Certificate{}, err
	}
	certificate.EnvironmentID = environmentID
	if _, err := tx.Exec(ctx, `
UPDATE guardian_devices.certificates
SET state = 'revoked', revoked_at = $3
WHERE device_id = $1 AND serial = $2 AND state = 'active'`,
		deviceID, currentSerial, now); err != nil {
		return devices.Certificate{}, fmt.Errorf("revoke prior device certificate: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO guardian_devices.certificates (
    serial, device_id, fingerprint_sha256, certificate_pem,
    not_before, not_after, state, created_at
) VALUES ($1, $2, $3, $4, $5, $6, 'active', $7)`,
		certificate.Serial, deviceID, certificate.Fingerprint,
		certificate.PEM, certificate.NotBefore, certificate.NotAfter, now); err != nil {
		return devices.Certificate{}, fmt.Errorf("rotate device certificate: %w", err)
	}
	_, err = (auditQueryAppender{queries: dbgen.New(tx)}).Append(ctx, audit.Event{
		OccurredAt:    now,
		Actor:         audit.Actor{Type: audit.ActorTypeDevice, ID: deviceID},
		Action:        audit.ActionCertificateRotated,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeDeviceCertificate, ID: certificate.Serial},
		CorrelationID: deviceID,
	})
	if err != nil {
		return devices.Certificate{}, fmt.Errorf("append rotation audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return devices.Certificate{}, err
	}
	return certificate, nil
}

func (s *Store) SetDeviceState(ctx context.Context, environmentID, deviceID string, state devices.DeviceState, actorID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	command, err := tx.Exec(ctx, `
UPDATE guardian_devices.devices SET state = $2, updated_at = clock_timestamp()
WHERE device_id = $1 AND environment_id = $3`, deviceID, string(state), environmentID)
	if err != nil {
		return fmt.Errorf("set device state: %w", err)
	}
	if command.RowsAffected() != 1 {
		return devices.ErrInvalidInput
	}
	if state == devices.DeviceRevoked {
		if _, err := tx.Exec(ctx, `
UPDATE guardian_devices.certificates
SET state = 'revoked', revoked_at = clock_timestamp()
WHERE device_id = $1 AND state = 'active'`, deviceID); err != nil {
			return fmt.Errorf("revoke active device certificate: %w", err)
		}
	}
	action := audit.ActionDeviceDisabled
	if state == devices.DeviceRevoked {
		action = audit.ActionDeviceRevoked
	}
	_, err = (auditQueryAppender{queries: dbgen.New(tx)}).Append(ctx, audit.Event{
		OccurredAt:    time.Now().UTC(),
		Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: actorID},
		Action:        action,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeDevice, ID: deviceID},
		CorrelationID: deviceID,
	})
	if err != nil {
		return fmt.Errorf("append device state audit: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *Store) CertificateEligible(ctx context.Context, deviceID, serial string) error {
	var deviceState, certificateState string
	var notBefore, notAfter time.Time
	err := s.pool.QueryRow(ctx, `
SELECT d.state, c.state, c.not_before, c.not_after
FROM guardian_devices.devices d
JOIN guardian_devices.certificates c ON c.device_id = d.device_id
WHERE d.device_id = $1 AND c.serial = $2`, deviceID, serial).Scan(
		&deviceState, &certificateState, &notBefore, &notAfter)
	if errors.Is(err, pgx.ErrNoRows) {
		return devices.ErrCertificateStale
	}
	if err != nil {
		return fmt.Errorf("read certificate eligibility: %w", err)
	}
	if deviceState == string(devices.DeviceDisabled) {
		return devices.ErrDeviceDisabled
	}
	if deviceState == string(devices.DeviceRevoked) {
		return devices.ErrDeviceRevoked
	}
	if certificateState != "active" {
		return devices.ErrCertificateRevoked
	}
	now := time.Now().UTC()
	if now.Before(notBefore) || !now.Before(notAfter) {
		return devices.ErrCertificateStale
	}
	return nil
}

func (s *Store) AllowEnrollmentAttempt(
	ctx context.Context,
	tokenScope, sourceScope [sha256.Size]byte,
	now time.Time,
) error {
	var blockedUntil *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT MAX(blocked_until)
FROM guardian_devices.enrollment_throttles
WHERE scope_hash = $1 OR scope_hash = $2`, tokenScope[:], sourceScope[:]).Scan(&blockedUntil)
	if err != nil {
		return fmt.Errorf("read enrollment throttle: %w", err)
	}
	if blockedUntil != nil && now.Before(*blockedUntil) {
		return devices.ErrEnrollmentRateLimited
	}
	return nil
}

func (s *Store) RecordEnrollmentFailure(
	ctx context.Context,
	tokenScope, sourceScope [sha256.Size]byte,
	now time.Time,
) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	for _, scope := range [][sha256.Size]byte{tokenScope, sourceScope} {
		var windowStart time.Time
		var failures int
		err := tx.QueryRow(ctx, `
SELECT window_started_at, failures
FROM guardian_devices.enrollment_throttles
WHERE scope_hash = $1
FOR UPDATE`, scope[:]).Scan(&windowStart, &failures)
		if errors.Is(err, pgx.ErrNoRows) {
			windowStart, failures = now, 0
		} else if err != nil {
			return fmt.Errorf("lock enrollment throttle: %w", err)
		}
		if now.Sub(windowStart) >= throttleWindow {
			windowStart, failures = now, 0
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
		_, err = tx.Exec(ctx, `
INSERT INTO guardian_devices.enrollment_throttles (
    scope_hash, window_started_at, failures, blocked_until, updated_at
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (scope_hash) DO UPDATE SET
    window_started_at = EXCLUDED.window_started_at,
    failures = EXCLUDED.failures,
    blocked_until = EXCLUDED.blocked_until,
    updated_at = EXCLUDED.updated_at`,
			scope[:], windowStart, failures, blockedUntil, now)
		if err != nil {
			return fmt.Errorf("record enrollment throttle: %w", err)
		}
	}
	objectID := "enrollment-" + hex.EncodeToString(tokenScope[:8])
	snapshot, err := audit.NewSnapshot(map[string]any{"result": "denied", "reason": "enrollment_attempt_failed"})
	if err != nil {
		return err
	}
	_, err = (auditQueryAppender{queries: dbgen.New(tx)}).Append(ctx, audit.Event{
		OccurredAt:    now,
		Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "enrollment-api"},
		Action:        audit.ActionSecurityActionDenied,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeSecurityAction, ID: objectID},
		CorrelationID: objectID,
		After:         &snapshot,
	})
	if err != nil {
		return fmt.Errorf("append enrollment denial audit: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *Store) ClearEnrollmentThrottle(
	ctx context.Context,
	tokenScope, sourceScope [sha256.Size]byte,
) error {
	_, err := s.pool.Exec(ctx, `
DELETE FROM guardian_devices.enrollment_throttles
WHERE scope_hash = $1 OR scope_hash = $2`, tokenScope[:], sourceScope[:])
	if err != nil {
		return fmt.Errorf("clear enrollment throttle: %w", err)
	}
	return nil
}

func appendEnrollmentFailure(ctx context.Context, tx pgx.Tx, deviceID, tokenID string, now time.Time, cause error) error {
	snapshot, err := audit.NewSnapshot(map[string]any{"result": "denied", "reason": cause.Error()})
	if err != nil {
		return err
	}
	_, err = (auditQueryAppender{queries: dbgen.New(tx)}).Append(ctx, audit.Event{
		OccurredAt:    now,
		Actor:         audit.Actor{Type: audit.ActorTypeDevice, ID: deviceID},
		Action:        audit.ActionEnrollmentFailed,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeDevice, ID: deviceID},
		CorrelationID: tokenID,
		After:         &snapshot,
	})
	return err
}
