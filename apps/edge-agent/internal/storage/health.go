package storage

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/health"
)

var _ health.StateStore = (*Store)(nil)

func (s *Store) LoadHealthState(ctx context.Context) (health.DurableState, error) {
	var state health.DurableState
	var nextSequence string
	var pendingReportID, pendingSequence sql.NullString
	var pendingPayload []byte
	if err := s.db.QueryRowContext(ctx, `
SELECT next_sequence, pending_report_id, pending_sequence, pending_payload
FROM health_report_state WHERE singleton = 1`).Scan(
		&nextSequence, &pendingReportID, &pendingSequence, &pendingPayload,
	); err != nil {
		return health.DurableState{}, classifyError("read health report state", err)
	}
	parsedNext, err := strconv.ParseUint(nextSequence, 10, 64)
	if err != nil || parsedNext == 0 {
		return health.DurableState{}, ErrSchemaIncompatible
	}
	state.NextSequence = parsedNext
	rows, err := s.db.QueryContext(ctx, `
SELECT condition_type, status, reason_code, message,
       COALESCE(observed_revision, ''), last_transition_at
FROM canonical_health_conditions
ORDER BY CASE condition_type
    WHEN 'edge_connected' THEN 1
    WHEN 'device_certificate_ready' THEN 2
    WHEN 'config_converged' THEN 3
    WHEN 'local_database_healthy' THEN 4
    WHEN 'spool_healthy' THEN 5
    WHEN 'clock_quality' THEN 6
    WHEN 'container_runtime_reachable' THEN 7
    WHEN 'privileged_helper_reachable' THEN 8
END`)
	if err != nil {
		return health.DurableState{}, classifyError("read canonical health conditions", err)
	}
	defer rows.Close()
	for rows.Next() {
		var condition health.Condition
		var revision string
		var transition int64
		if err := rows.Scan(&condition.Type, &condition.Status, &condition.Reason, &condition.Message, &revision, &transition); err != nil {
			return health.DurableState{}, classifyError("scan canonical health condition", err)
		}
		if revision != "" {
			value, parseErr := strconv.ParseUint(revision, 10, 64)
			if parseErr != nil {
				return health.DurableState{}, ErrSchemaIncompatible
			}
			condition.ObservedRevision = &value
		}
		condition.LastTransitionTime = time.Unix(0, transition).UTC()
		if err := condition.Validate(); err != nil {
			return health.DurableState{}, ErrSchemaIncompatible
		}
		state.Conditions = append(state.Conditions, condition)
	}
	if err := rows.Err(); err != nil {
		return health.DurableState{}, classifyError("iterate canonical health conditions", err)
	}
	if len(state.Conditions) != 0 && len(state.Conditions) != len(health.ConditionTypes()) {
		return health.DurableState{}, ErrSchemaIncompatible
	}
	if pendingReportID.Valid != pendingSequence.Valid || pendingReportID.Valid != (len(pendingPayload) > 0) {
		return health.DurableState{}, ErrSchemaIncompatible
	}
	if pendingReportID.Valid {
		sequence, parseErr := strconv.ParseUint(pendingSequence.String, 10, 64)
		if parseErr != nil {
			return health.DurableState{}, ErrSchemaIncompatible
		}
		report, parseErr := health.ParseReport(pendingPayload)
		if parseErr != nil || report.ReportID != pendingReportID.String || report.Sequence != sequence {
			return health.DurableState{}, ErrSchemaIncompatible
		}
		state.PendingReport = &report
		state.PendingPayload = bytes.Clone(pendingPayload)
	}
	return state, nil
}

func (s *Store) PersistHealthReport(ctx context.Context, report health.Report, payload []byte) error {
	canonical, err := health.MarshalReport(report)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical, payload) || report.Sequence == math.MaxUint64 {
		if report.Sequence == math.MaxUint64 {
			return health.ErrSequenceExhausted
		}
		return health.ErrInvalidReport
	}
	if err := s.beforeMutation(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return classifyError("begin health report transaction", err)
	}
	defer tx.Rollback()
	var nextSequence string
	var pendingID sql.NullString
	if err := tx.QueryRowContext(ctx, `
SELECT next_sequence, pending_report_id FROM health_report_state WHERE singleton = 1`).Scan(&nextSequence, &pendingID); err != nil {
		return classifyError("lock health report state", err)
	}
	parsedNext, err := strconv.ParseUint(nextSequence, 10, 64)
	if err != nil || parsedNext == 0 {
		return ErrSchemaIncompatible
	}
	if pendingID.Valid {
		return health.ErrReportPending
	}
	if report.Sequence != parsedNext {
		return health.ErrInvalidReport
	}
	now := s.now().UTC()
	result, err := tx.ExecContext(ctx, `
UPDATE health_report_state SET
    next_sequence = ?, pending_report_id = ?, pending_sequence = ?,
    pending_payload = ?, pending_created_at = ?, updated_at = ?
WHERE singleton = 1 AND pending_report_id IS NULL`,
		strconv.FormatUint(parsedNext+1, 10), report.ReportID,
		strconv.FormatUint(report.Sequence, 10), payload, now.UnixNano(), now.UnixNano())
	if err != nil {
		return classifyError("persist pending health report", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return health.ErrReportPending
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM canonical_health_conditions`); err != nil {
		return classifyError("replace canonical health conditions", err)
	}
	for _, condition := range report.Conditions {
		var revision any
		if condition.ObservedRevision != nil {
			revision = strconv.FormatUint(*condition.ObservedRevision, 10)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO canonical_health_conditions (
    condition_type, status, reason_code, message, observed_revision,
    last_transition_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`, condition.Type, condition.Status, condition.Reason,
			condition.Message, revision, condition.LastTransitionTime.UnixNano(), now.UnixNano()); err != nil {
			return classifyError("persist canonical health condition", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return classifyError("commit health report", err)
	}
	return nil
}

func (s *Store) AcknowledgeHealthReport(ctx context.Context, reportID string, sequence uint64) error {
	if err := s.beforeMutation(); err != nil {
		return err
	}
	now := s.now().UTC()
	result, err := s.db.ExecContext(ctx, `
UPDATE health_report_state SET
    pending_report_id = NULL, pending_sequence = NULL, pending_payload = NULL,
    pending_created_at = NULL, acknowledged_report_id = ?,
    acknowledged_sequence = ?, acknowledged_at = ?, updated_at = ?
WHERE singleton = 1 AND pending_report_id = ? AND pending_sequence = ?`,
		reportID, strconv.FormatUint(sequence, 10), now.UnixNano(), now.UnixNano(),
		reportID, strconv.FormatUint(sequence, 10))
	if err != nil {
		return classifyError("acknowledge health report", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return classifyError("read health acknowledgement result", err)
	}
	if affected != 1 {
		return health.ErrAcknowledgementMismatch
	}
	return nil
}

func (s *Store) ProbeHealth(ctx context.Context) (readOK, writeOK, integrityOK bool) {
	readOK = s.db.QueryRowContext(ctx, `SELECT 1`).Scan(new(int)) == nil
	if tx, err := s.db.BeginTx(ctx, nil); err == nil {
		_, writeErr := tx.ExecContext(ctx, `UPDATE health_report_state SET updated_at = updated_at WHERE singleton = 1`)
		writeOK = writeErr == nil
		_ = tx.Rollback()
	}
	integrityOK = quickCheck(ctx, s.db) == nil
	return readOK, writeOK, integrityOK
}

func (s *Store) HealthSpoolStats(ctx context.Context) (usedBytes int64, directory string, err error) {
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(size_bytes), 0) FROM spool_objects`).Scan(&usedBytes); err != nil {
		return 0, "", classifyError("read health spool usage", err)
	}
	return usedBytes, s.options.SpoolDirectory, nil
}

func (s *Store) HealthReconciliation(ctx context.Context) (desired, observed uint64, state string, found bool, err error) {
	record, err := s.ReconciliationState(ctx)
	if errors.Is(err, ErrReconciliationStateNotFound) {
		return 0, 0, "", false, nil
	}
	if err != nil {
		return 0, 0, "", false, err
	}
	return record.DesiredRevision, record.ObservedRevision, record.ConditionStatus, true, nil
}

func (s *Store) HealthIdentity(ctx context.Context) (notAfter time.Time, enrollmentStatus string, found bool, err error) {
	var notAfterUnix int64
	err = s.db.QueryRowContext(ctx, `
SELECT certificate_not_after, enrollment_status FROM identity_metadata WHERE singleton = 1`).Scan(
		&notAfterUnix, &enrollmentStatus,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, "", false, nil
	}
	if err != nil {
		return time.Time{}, "", false, classifyError("read health identity", err)
	}
	return time.Unix(0, notAfterUnix).UTC(), enrollmentStatus, true, nil
}
