package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const maxReconciliationAttempts uint32 = 6

var retryDelays = [...]time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}

var ErrReconciliationStateNotFound = errors.New("reconciliation state was not found")

type ReconciliationCandidate struct {
	MessageID         string
	Revision          uint64
	Digest            [sha256.Size]byte
	Payload           []byte
	TerminalReason    string
	ObservedMessageID string
	Now               time.Time
}

type ReconciliationRecord struct {
	DesiredMessageID  string
	DesiredRevision   uint64
	ObservedRevision  uint64
	LastGoodRevision  uint64
	DesiredDigest     [sha256.Size]byte
	DesiredPayload    []byte
	LastGoodDigest    []byte
	LastGoodPayload   []byte
	ConditionStatus   string
	ReasonCode        string
	AttemptCount      uint32
	RetryAt           *time.Time
	ObservedMessageID string
	ObservedPending   bool
	LastTransitionAt  time.Time
}

type ReconciliationAcceptance struct {
	Record      ReconciliationRecord
	ShouldApply bool
}

type ReconciliationResult struct {
	ExpectedMessageID string
	ExpectedRevision  uint64
	ExpectedDigest    [sha256.Size]byte
	ObservedMessageID string
	Success           bool
	Retryable         bool
	ReasonCode        string
	Now               time.Time
}

type ReconciliationSummary struct {
	DesiredRevision  uint64     `json:"desired_revision"`
	ObservedRevision uint64     `json:"observed_revision"`
	LastGoodRevision uint64     `json:"last_good_revision"`
	ConditionStatus  string     `json:"condition_status"`
	ReasonCode       string     `json:"reason_code"`
	AttemptCount     uint32     `json:"attempt_count"`
	RetryAt          *time.Time `json:"retry_at,omitempty"`
	ObservedPending  bool       `json:"observed_pending"`
}

func (s *Store) AcceptReconciliationCandidate(ctx context.Context, candidate ReconciliationCandidate) (ReconciliationAcceptance, error) {
	if err := validateCandidate(candidate); err != nil {
		return ReconciliationAcceptance{}, err
	}
	if err := s.beforeMutation(); err != nil {
		return ReconciliationAcceptance{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReconciliationAcceptance{}, classifyError("begin reconciliation acceptance", err)
	}
	defer tx.Rollback()

	record, err := readReconciliationTx(ctx, tx)
	if errors.Is(err, sql.ErrNoRows) {
		record = ReconciliationRecord{
			DesiredMessageID: candidate.MessageID, DesiredRevision: candidate.Revision,
			DesiredDigest: candidate.Digest, DesiredPayload: append([]byte(nil), candidate.Payload...),
			ConditionStatus: "pending", ReasonCode: "accepted", AttemptCount: 0,
			ObservedMessageID: candidate.ObservedMessageID, ObservedPending: true,
			LastTransitionAt: candidate.Now.UTC(),
		}
		shouldApply := true
		if candidate.TerminalReason != "" {
			record.ConditionStatus = "failed"
			record.ReasonCode = candidate.TerminalReason
			record.AttemptCount = 1
			shouldApply = false
		}
		if err := saveReconciliationTx(ctx, tx, record, candidate.Now); err != nil {
			return ReconciliationAcceptance{}, err
		}
		if err := tx.Commit(); err != nil {
			return ReconciliationAcceptance{}, classifyError("commit reconciliation acceptance", err)
		}
		return ReconciliationAcceptance{Record: record, ShouldApply: shouldApply}, nil
	}
	if err != nil {
		return ReconciliationAcceptance{}, err
	}

	shouldApply := false
	switch {
	case candidate.Revision < record.DesiredRevision:
		record.ObservedPending = true
	case candidate.Revision == record.DesiredRevision && bytes.Equal(candidate.Digest[:], record.DesiredDigest[:]):
		record.ObservedPending = true
		shouldApply = record.ConditionStatus == "pending" ||
			(record.ConditionStatus == "retrying" && record.RetryAt != nil && !record.RetryAt.After(candidate.Now))
	case candidate.Revision == record.DesiredRevision:
		record.ConditionStatus = "failed"
		record.ReasonCode = "revision_conflict"
		record.AttemptCount = 1
		record.RetryAt = nil
		record.ObservedMessageID = candidate.ObservedMessageID
		record.ObservedPending = true
		record.LastTransitionAt = candidate.Now.UTC()
	default:
		record.DesiredMessageID = candidate.MessageID
		record.DesiredRevision = candidate.Revision
		record.DesiredDigest = candidate.Digest
		record.DesiredPayload = append(record.DesiredPayload[:0], candidate.Payload...)
		record.ConditionStatus = "pending"
		record.ReasonCode = "accepted"
		record.AttemptCount = 0
		record.RetryAt = nil
		record.ObservedMessageID = candidate.ObservedMessageID
		record.ObservedPending = true
		record.LastTransitionAt = candidate.Now.UTC()
		shouldApply = true
		if candidate.TerminalReason != "" {
			record.ConditionStatus = "failed"
			record.ReasonCode = candidate.TerminalReason
			record.AttemptCount = 1
			shouldApply = false
		}
	}
	if err := saveReconciliationTx(ctx, tx, record, candidate.Now); err != nil {
		return ReconciliationAcceptance{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReconciliationAcceptance{}, classifyError("commit reconciliation acceptance", err)
	}
	return ReconciliationAcceptance{Record: record, ShouldApply: shouldApply}, nil
}

func (s *Store) RecordReconciliationResult(ctx context.Context, result ReconciliationResult) (ReconciliationRecord, error) {
	if !validUUIDv7Storage(result.ExpectedMessageID) || result.ExpectedRevision == 0 ||
		!validUUIDv7Storage(result.ObservedMessageID) || result.Now.IsZero() ||
		(result.Success && result.Retryable) || (!result.Success && !validCode(result.ReasonCode)) {
		return ReconciliationRecord{}, errors.New("reconciliation result is invalid")
	}
	if err := s.beforeMutation(); err != nil {
		return ReconciliationRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReconciliationRecord{}, classifyError("begin reconciliation result", err)
	}
	defer tx.Rollback()
	record, err := readReconciliationTx(ctx, tx)
	if err != nil {
		return ReconciliationRecord{}, err
	}
	if record.DesiredMessageID != result.ExpectedMessageID || record.DesiredRevision != result.ExpectedRevision ||
		!bytes.Equal(record.DesiredDigest[:], result.ExpectedDigest[:]) {
		return ReconciliationRecord{}, errors.New("reconciliation candidate was superseded")
	}
	attempts := record.AttemptCount + 1
	if attempts > maxReconciliationAttempts {
		attempts = maxReconciliationAttempts
	}
	record.AttemptCount = attempts
	record.ObservedMessageID = result.ObservedMessageID
	record.ObservedPending = true
	record.LastTransitionAt = result.Now.UTC()
	record.RetryAt = nil
	if result.Success {
		record.ObservedRevision = record.DesiredRevision
		record.LastGoodRevision = record.DesiredRevision
		record.LastGoodDigest = append(record.LastGoodDigest[:0], record.DesiredDigest[:]...)
		record.LastGoodPayload = append(record.LastGoodPayload[:0], record.DesiredPayload...)
		record.ConditionStatus = "converged"
		record.ReasonCode = "applied"
	} else if result.Retryable && int(attempts) <= len(retryDelays) {
		retryAt := result.Now.UTC().Add(retryDelays[attempts-1])
		record.ConditionStatus = "retrying"
		record.ReasonCode = result.ReasonCode
		record.RetryAt = &retryAt
	} else {
		record.ConditionStatus = "failed"
		if result.Retryable {
			record.ReasonCode = "retry_exhausted"
		} else {
			record.ReasonCode = result.ReasonCode
		}
	}
	if err := saveReconciliationTx(ctx, tx, record, result.Now); err != nil {
		return ReconciliationRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReconciliationRecord{}, classifyError("commit reconciliation result", err)
	}
	return record, nil
}

func (s *Store) ReconciliationState(ctx context.Context) (ReconciliationRecord, error) {
	record, err := readReconciliationDB(ctx, s.db)
	if errors.Is(err, sql.ErrNoRows) {
		return ReconciliationRecord{}, ErrReconciliationStateNotFound
	}
	return record, err
}

func (s *Store) AcknowledgeObserved(ctx context.Context, messageID string, observedRevision uint64) error {
	if !validUUIDv7Storage(messageID) {
		return errors.New("observed acknowledgement is invalid")
	}
	if err := s.beforeMutation(); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE reconciliation_state
SET observed_pending = 0, updated_at = ?
WHERE singleton = 1 AND observed_message_id = ? AND observed_revision = ?`,
		s.now().UTC().UnixNano(), messageID, observedRevision)
	if err != nil {
		return classifyError("acknowledge observed state", err)
	}
	return requireRowsAffected("acknowledge observed state", result)
}

func readReconciliationDB(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (ReconciliationRecord, error) {
	return scanReconciliation(queryer.QueryRowContext(ctx, reconciliationSelect))
}

func readReconciliationTx(ctx context.Context, tx *sql.Tx) (ReconciliationRecord, error) {
	return scanReconciliation(tx.QueryRowContext(ctx, reconciliationSelect))
}

const reconciliationSelect = `
SELECT desired_message_id, desired_revision, observed_revision, last_good_revision,
       desired_digest, desired_payload, last_good_digest, last_good_payload,
       condition_status, reason_code, attempt_count, retry_at,
       observed_message_id, observed_pending, last_transition_at
FROM reconciliation_state WHERE singleton = 1`

func scanReconciliation(row *sql.Row) (ReconciliationRecord, error) {
	var record ReconciliationRecord
	var desiredRevision, observedRevision, lastGoodRevision int64
	var desiredDigest []byte
	var retryAt sql.NullInt64
	var pending int
	var transition int64
	err := row.Scan(
		&record.DesiredMessageID, &desiredRevision, &observedRevision, &lastGoodRevision,
		&desiredDigest, &record.DesiredPayload, &record.LastGoodDigest, &record.LastGoodPayload,
		&record.ConditionStatus, &record.ReasonCode, &record.AttemptCount, &retryAt,
		&record.ObservedMessageID, &pending, &transition,
	)
	if err != nil {
		return ReconciliationRecord{}, classifyError("read reconciliation state", err)
	}
	if len(desiredDigest) != sha256.Size {
		return ReconciliationRecord{}, ErrSchemaIncompatible
	}
	copy(record.DesiredDigest[:], desiredDigest)
	record.DesiredRevision = uint64(desiredRevision)
	record.ObservedRevision = uint64(observedRevision)
	record.LastGoodRevision = uint64(lastGoodRevision)
	record.ObservedPending = pending == 1
	record.LastTransitionAt = time.Unix(0, transition).UTC()
	if retryAt.Valid {
		value := time.Unix(0, retryAt.Int64).UTC()
		record.RetryAt = &value
	}
	return record, nil
}

func saveReconciliationTx(ctx context.Context, tx *sql.Tx, record ReconciliationRecord, now time.Time) error {
	var retryAt any
	if record.RetryAt != nil {
		retryAt = record.RetryAt.UTC().UnixNano()
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO reconciliation_state (
    singleton, desired_message_id, desired_revision, observed_revision,
    last_good_revision, desired_digest, desired_payload, last_good_digest,
    last_good_payload, condition_status, reason_code, attempt_count, retry_at,
    observed_message_id, observed_pending, last_transition_at, updated_at
) VALUES (1,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(singleton) DO UPDATE SET
    desired_message_id = excluded.desired_message_id,
    desired_revision = excluded.desired_revision,
    observed_revision = excluded.observed_revision,
    last_good_revision = excluded.last_good_revision,
    desired_digest = excluded.desired_digest,
    desired_payload = excluded.desired_payload,
    last_good_digest = excluded.last_good_digest,
    last_good_payload = excluded.last_good_payload,
    condition_status = excluded.condition_status,
    reason_code = excluded.reason_code,
    attempt_count = excluded.attempt_count,
    retry_at = excluded.retry_at,
    observed_message_id = excluded.observed_message_id,
    observed_pending = excluded.observed_pending,
    last_transition_at = excluded.last_transition_at,
    updated_at = excluded.updated_at`,
		record.DesiredMessageID, record.DesiredRevision, record.ObservedRevision,
		record.LastGoodRevision, record.DesiredDigest[:], record.DesiredPayload,
		nullBytes(record.LastGoodDigest), nullBytes(record.LastGoodPayload),
		record.ConditionStatus, record.ReasonCode, record.AttemptCount, retryAt,
		record.ObservedMessageID, boolInt(record.ObservedPending),
		record.LastTransitionAt.UTC().UnixNano(), now.UTC().UnixNano())
	if err != nil {
		return classifyError("persist reconciliation state", err)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO state_revisions (
    object_kind, desired_revision, observed_revision, last_good_revision, condition_code, updated_at
) VALUES ('desired-state', ?, ?, ?, ?, ?)
ON CONFLICT(object_kind) DO UPDATE SET
    desired_revision = excluded.desired_revision,
    observed_revision = excluded.observed_revision,
    last_good_revision = excluded.last_good_revision,
    condition_code = excluded.condition_code,
    updated_at = excluded.updated_at`,
		record.DesiredRevision, record.ObservedRevision, record.LastGoodRevision,
		record.ConditionStatus+"."+record.ReasonCode, now.UTC().UnixNano())
	if err != nil {
		return classifyError("project reconciliation revision", err)
	}
	nextAttempt := int64(0)
	if record.RetryAt != nil {
		nextAttempt = record.RetryAt.UTC().UnixNano()
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO retry_metadata (operation_id, attempts, next_attempt_at, last_error_code, updated_at)
VALUES ('desired-state', ?, ?, ?, ?)
ON CONFLICT(operation_id) DO UPDATE SET
    attempts = excluded.attempts,
    next_attempt_at = excluded.next_attempt_at,
    last_error_code = excluded.last_error_code,
    updated_at = excluded.updated_at`,
		record.AttemptCount, nextAttempt, record.ReasonCode, now.UTC().UnixNano())
	return classifyError("project reconciliation retry", err)
}

func validateCandidate(candidate ReconciliationCandidate) error {
	if !validUUIDv7Storage(candidate.MessageID) || candidate.Revision == 0 ||
		!validUUIDv7Storage(candidate.ObservedMessageID) || candidate.Now.IsZero() ||
		len(candidate.Payload) < 1 || len(candidate.Payload) > maxEventPayloadBytes ||
		(candidate.TerminalReason != "" && !validCode(candidate.TerminalReason)) {
		return errors.New("reconciliation candidate is invalid")
	}
	return nil
}

func validUUIDv7Storage(value string) bool {
	if len(value) != 36 || value != strings.ToLower(value) || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || value[14] != '7' || !strings.ContainsRune("89ab", rune(value[19])) {
		return false
	}
	decoded, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil && len(decoded) == 16
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func (record ReconciliationRecord) String() string {
	return fmt.Sprintf("desired=%d observed=%d last_good=%d status=%s reason=%s attempts=%d",
		record.DesiredRevision, record.ObservedRevision, record.LastGoodRevision,
		record.ConditionStatus, record.ReasonCode, record.AttemptCount)
}
