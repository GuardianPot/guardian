package storage

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const maxSnapshotRows = 64

// IdentityRecord is the non-secret durable identity summary.
type IdentityRecord struct {
	CertificateSHA256 string    `json:"certificate_sha256"`
	NotBefore         time.Time `json:"not_before"`
	NotAfter          time.Time `json:"not_after"`
	EnrollmentStatus  string    `json:"enrollment_status"`
}

// RevisionRecord preserves desired, observed, and last-known-good progress.
type RevisionRecord struct {
	ObjectKind       string `json:"object_kind"`
	DesiredRevision  int64  `json:"desired_revision"`
	ObservedRevision int64  `json:"observed_revision"`
	LastGoodRevision int64  `json:"last_good_revision"`
	ConditionCode    string `json:"condition_code"`
}

// RetryRecord preserves bounded retry metadata across restart.
type RetryRecord struct {
	OperationID   string    `json:"operation_id"`
	Attempts      int       `json:"attempts"`
	NextAttempt   time.Time `json:"next_attempt"`
	LastErrorCode string    `json:"last_error_code"`
}

// HealthCondition is a redaction-safe product health state.
type HealthCondition struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	ReasonCode string    `json:"reason_code"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SpoolStats summarizes metadata without reading or exposing payload content.
type SpoolStats struct {
	Objects int   `json:"objects"`
	Bytes   int64 `json:"bytes"`
}

// Snapshot is a bounded, read-only diagnostic view.
type Snapshot struct {
	SchemaVersion  int                    `json:"schema_version"`
	Identity       *IdentityRecord        `json:"identity,omitempty"`
	Reconciliation *ReconciliationSummary `json:"reconciliation,omitempty"`
	Revisions      []RevisionRecord       `json:"revisions"`
	Retries        []RetryRecord          `json:"retries"`
	Health         []HealthCondition      `json:"health"`
	Queue          Stats                  `json:"queue"`
	Spool          SpoolStats             `json:"spool"`
}

// SetIdentity records only certificate metadata, never the private key.
func (s *Store) SetIdentity(ctx context.Context, record IdentityRecord) error {
	if decoded, err := hex.DecodeString(record.CertificateSHA256); err != nil || len(decoded) != 32 {
		return errors.New("identity certificate fingerprint must be SHA-256 hex")
	}
	if record.NotAfter.Before(record.NotBefore) {
		return errors.New("identity certificate validity window is invalid")
	}
	switch record.EnrollmentStatus {
	case "enrolled", "revoked", "unavailable":
	default:
		return errors.New("identity enrollment status is invalid")
	}
	if err := s.beforeMutation(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO identity_metadata (
    singleton, certificate_sha256, certificate_not_before, certificate_not_after, enrollment_status, updated_at
) VALUES (1, ?, ?, ?, ?, ?)
ON CONFLICT(singleton) DO UPDATE SET
    certificate_sha256 = excluded.certificate_sha256,
    certificate_not_before = excluded.certificate_not_before,
    certificate_not_after = excluded.certificate_not_after,
    enrollment_status = CASE
        WHEN identity_metadata.enrollment_status = 'revoked'
          AND identity_metadata.certificate_sha256 = excluded.certificate_sha256
        THEN 'revoked'
        ELSE excluded.enrollment_status
    END,
    updated_at = excluded.updated_at`,
		record.CertificateSHA256,
		record.NotBefore.UnixNano(),
		record.NotAfter.UnixNano(),
		record.EnrollmentStatus,
		s.now().UnixNano(),
	)
	return classifyError("record edge identity metadata", err)
}

// SetRevision atomically persists desired, observed, and last-known-good state.
func (s *Store) SetRevision(ctx context.Context, record RevisionRecord) error {
	if !validCode(record.ObjectKind) || !validCode(record.ConditionCode) {
		return errors.New("revision object and condition codes must be bounded identifiers")
	}
	if record.DesiredRevision < 0 || record.ObservedRevision < 0 || record.LastGoodRevision < 0 {
		return errors.New("revision values must not be negative")
	}
	if err := s.beforeMutation(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO state_revisions (
    object_kind, desired_revision, observed_revision, last_good_revision, condition_code, updated_at
) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(object_kind) DO UPDATE SET
    desired_revision = excluded.desired_revision,
    observed_revision = excluded.observed_revision,
    last_good_revision = excluded.last_good_revision,
    condition_code = excluded.condition_code,
    updated_at = excluded.updated_at`,
		record.ObjectKind,
		record.DesiredRevision,
		record.ObservedRevision,
		record.LastGoodRevision,
		record.ConditionCode,
		s.now().UnixNano(),
	)
	return classifyError("record edge state revision", err)
}

// SetRetry persists only a redaction-safe error code, never a raw remote error.
func (s *Store) SetRetry(ctx context.Context, record RetryRecord) error {
	if !validCode(record.OperationID) || !validCode(record.LastErrorCode) {
		return errors.New("retry operation and error codes must be bounded identifiers")
	}
	if record.Attempts < 0 {
		return errors.New("retry attempts must not be negative")
	}
	if err := s.beforeMutation(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO retry_metadata (operation_id, attempts, next_attempt_at, last_error_code, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(operation_id) DO UPDATE SET
    attempts = excluded.attempts,
    next_attempt_at = excluded.next_attempt_at,
    last_error_code = excluded.last_error_code,
    updated_at = excluded.updated_at`,
		record.OperationID,
		record.Attempts,
		record.NextAttempt.UnixNano(),
		record.LastErrorCode,
		s.now().UnixNano(),
	)
	return classifyError("record edge retry metadata", err)
}

// SetHealth persists a bounded condition and reason code.
func (s *Store) SetHealth(ctx context.Context, condition HealthCondition) error {
	if !validCode(condition.Name) || !validCode(condition.ReasonCode) {
		return errors.New("health name and reason must be bounded identifiers")
	}
	switch condition.Status {
	case "healthy", "degraded", "failed", "stopped", "unknown":
	default:
		return errors.New("health status is invalid")
	}
	if err := s.beforeMutation(); err != nil {
		return err
	}
	now := s.now().UTC()
	if !condition.UpdatedAt.IsZero() {
		now = condition.UpdatedAt.UTC()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO component_health (name, status, reason_code, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
    status = excluded.status,
    reason_code = excluded.reason_code,
    updated_at = excluded.updated_at`,
		condition.Name, condition.Status, condition.ReasonCode, now.UnixNano())
	return classifyError("record edge health condition", err)
}

// Snapshot returns a bounded diagnostic view and performs no writes.
func (s *Store) Snapshot(ctx context.Context) (Snapshot, error) {
	version, err := readSchemaVersion(ctx, s.db)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{SchemaVersion: version, Revisions: []RevisionRecord{}, Retries: []RetryRecord{}, Health: []HealthCondition{}}

	var fingerprint, status string
	var notBefore, notAfter int64
	err = s.db.QueryRowContext(ctx, `
SELECT certificate_sha256, certificate_not_before, certificate_not_after, enrollment_status
FROM identity_metadata WHERE singleton = 1`).Scan(&fingerprint, &notBefore, &notAfter, &status)
	if err == nil {
		snapshot.Identity = &IdentityRecord{
			CertificateSHA256: fingerprint,
			NotBefore:         time.Unix(0, notBefore).UTC(),
			NotAfter:          time.Unix(0, notAfter).UTC(),
			EnrollmentStatus:  status,
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, classifyError("read edge identity metadata", err)
	}

	if snapshot.Revisions, err = s.readRevisions(ctx); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Retries, err = s.readRetries(ctx); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Health, err = s.readHealth(ctx); err != nil {
		return Snapshot{}, err
	}
	reconciliationState, reconciliationErr := s.ReconciliationState(ctx)
	if reconciliationErr == nil {
		snapshot.Reconciliation = &ReconciliationSummary{
			DesiredRevision:  reconciliationState.DesiredRevision,
			ObservedRevision: reconciliationState.ObservedRevision,
			LastGoodRevision: reconciliationState.LastGoodRevision,
			ConditionStatus:  reconciliationState.ConditionStatus,
			ReasonCode:       reconciliationState.ReasonCode,
			AttemptCount:     reconciliationState.AttemptCount,
			RetryAt:          reconciliationState.RetryAt,
			ObservedPending:  reconciliationState.ObservedPending,
		}
	} else if !errors.Is(reconciliationErr, ErrReconciliationStateNotFound) {
		return Snapshot{}, reconciliationErr
	}
	if snapshot.Queue, err = s.Stats(ctx); err != nil {
		return Snapshot{}, err
	}
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(size_bytes), 0) FROM spool_objects WHERE state = 'available'`).Scan(
		&snapshot.Spool.Objects, &snapshot.Spool.Bytes); err != nil {
		return Snapshot{}, classifyError("read edge spool statistics", err)
	}
	return snapshot, nil
}

func (s *Store) readRevisions(ctx context.Context) ([]RevisionRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT object_kind, desired_revision, observed_revision, last_good_revision, condition_code
FROM state_revisions ORDER BY object_kind LIMIT ?`, maxSnapshotRows)
	if err != nil {
		return nil, classifyError("read edge state revisions", err)
	}
	defer rows.Close()
	records := []RevisionRecord{}
	for rows.Next() {
		var record RevisionRecord
		if err := rows.Scan(&record.ObjectKind, &record.DesiredRevision, &record.ObservedRevision, &record.LastGoodRevision, &record.ConditionCode); err != nil {
			return nil, classifyError("scan edge state revision", err)
		}
		records = append(records, record)
	}
	return records, classifyError("iterate edge state revisions", rows.Err())
}

func (s *Store) readRetries(ctx context.Context) ([]RetryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT operation_id, attempts, next_attempt_at, last_error_code
FROM retry_metadata ORDER BY next_attempt_at, operation_id LIMIT ?`, maxSnapshotRows)
	if err != nil {
		return nil, classifyError("read edge retry metadata", err)
	}
	defer rows.Close()
	records := []RetryRecord{}
	for rows.Next() {
		var record RetryRecord
		var nextAttempt int64
		if err := rows.Scan(&record.OperationID, &record.Attempts, &nextAttempt, &record.LastErrorCode); err != nil {
			return nil, classifyError("scan edge retry metadata", err)
		}
		record.NextAttempt = time.Unix(0, nextAttempt).UTC()
		records = append(records, record)
	}
	return records, classifyError("iterate edge retry metadata", rows.Err())
}

func (s *Store) readHealth(ctx context.Context) ([]HealthCondition, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT name, status, reason_code, updated_at
FROM component_health ORDER BY name LIMIT ?`, maxSnapshotRows)
	if err != nil {
		return nil, classifyError("read edge health conditions", err)
	}
	defer rows.Close()
	conditions := []HealthCondition{}
	for rows.Next() {
		var condition HealthCondition
		var updatedAt int64
		if err := rows.Scan(&condition.Name, &condition.Status, &condition.ReasonCode, &updatedAt); err != nil {
			return nil, classifyError("scan edge health condition", err)
		}
		condition.UpdatedAt = time.Unix(0, updatedAt).UTC()
		conditions = append(conditions, condition)
	}
	return conditions, classifyError("iterate edge health conditions", rows.Err())
}

func (s *Store) beforeMutation() error {
	if s.readOnly {
		return errors.New("edge store is read-only")
	}
	if s.beforeWrite != nil {
		if err := s.beforeWrite(); err != nil {
			return classifyError("edge storage write barrier", err)
		}
	}
	return nil
}

func validCode(value string) bool {
	if len(value) < 1 || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func requireRowsAffected(operation string, result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return classifyError("inspect "+operation, err)
	}
	if count != 1 {
		return fmt.Errorf("%s affected %d rows", operation, count)
	}
	return nil
}
