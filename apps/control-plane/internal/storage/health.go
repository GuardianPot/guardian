package storage

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/health"
	"github.com/jackc/pgx/v5"
)

var _ health.Repository = (*Store)(nil)

func (s *Store) StoreHealthReport(ctx context.Context, deviceID string, report health.Report, receivedAt time.Time) (health.ApplyOutcome, error) {
	if err := report.Validate(); err != nil {
		return 0, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return 0, fmt.Errorf("begin health report transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	environmentID, err := lockHealthDevice(ctx, tx, deviceID, true)
	if err != nil {
		return 0, err
	}
	current, err := loadHealthProjection(ctx, tx, deviceID, true)
	if err != nil {
		return 0, err
	}
	if current != nil && current.ReportID == "" {
		current = nil
	}
	next, outcome, err := health.ApplyReport(current, report, receivedAt)
	if err != nil {
		return 0, err
	}
	if outcome == health.ApplyDuplicate {
		if err := tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("commit duplicate health report: %w", err)
		}
		return outcome, nil
	}
	payload, err := health.MarshalReport(report)
	if err != nil {
		return 0, err
	}
	if err := saveHealthProjection(ctx, tx, deviceID, environmentID, next, payload); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit health report: %w", err)
	}
	return outcome, nil
}

func (s *Store) MarkHealthDisconnected(ctx context.Context, deviceID string, at time.Time) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin health disconnect transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	environmentID, err := lockHealthDevice(ctx, tx, deviceID, false)
	if err != nil {
		return err
	}
	current, err := loadHealthProjection(ctx, tx, deviceID, true)
	if err != nil {
		return err
	}
	if current == nil {
		conditions, unknownErr := health.UnknownConditions(at)
		if unknownErr != nil {
			return unknownErr
		}
		current = &health.Projection{ReceivedAt: at, ObservedAt: at, Conditions: conditions}
	}
	next, err := current.MarkDisconnected(at)
	if err != nil {
		return err
	}
	if err := saveHealthProjection(ctx, tx, deviceID, environmentID, next, nil); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit health disconnect: %w", err)
	}
	return nil
}

func (s *Store) DeviceHealth(ctx context.Context, deviceID string) (health.DeviceProjection, error) {
	var record health.DeviceProjection
	err := s.pool.QueryRow(ctx, `
SELECT device_id::text, environment_id::text, state
FROM guardian_devices.devices
WHERE device_id = $1`, deviceID).Scan(&record.DeviceID, &record.EnvironmentID, &record.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return health.DeviceProjection{}, health.ErrNotFound
	}
	if err != nil {
		return health.DeviceProjection{}, fmt.Errorf("read health device: %w", err)
	}
	record.Projection, err = loadHealthProjection(ctx, s.pool, deviceID, false)
	if err != nil {
		return health.DeviceProjection{}, err
	}
	return record, nil
}

func (s *Store) ActiveEnvironmentHealth(ctx context.Context, environmentID string) ([]health.DeviceProjection, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (
SELECT 1 FROM guardian_environment.environments WHERE environment_id = $1
)`, environmentID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("verify health environment: %w", err)
	}
	if !exists {
		return nil, health.ErrNotFound
	}
	rows, err := s.pool.Query(ctx, `
SELECT device_id::text
FROM guardian_devices.devices
WHERE environment_id = $1 AND state = 'active'
ORDER BY device_id`, environmentID)
	if err != nil {
		return nil, fmt.Errorf("list active health devices: %w", err)
	}
	var deviceIDs []string
	for rows.Next() {
		var deviceID string
		if err := rows.Scan(&deviceID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan active health device: %w", err)
		}
		deviceIDs = append(deviceIDs, deviceID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate active health devices: %w", err)
	}
	rows.Close()
	records := make([]health.DeviceProjection, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		record, readErr := s.DeviceHealth(ctx, deviceID)
		if readErr != nil {
			return nil, readErr
		}
		records = append(records, record)
	}
	return records, nil
}

type healthQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func lockHealthDevice(ctx context.Context, tx pgx.Tx, deviceID string, requireActive bool) (string, error) {
	query := `SELECT environment_id::text FROM guardian_devices.devices WHERE device_id = $1`
	if requireActive {
		query += ` AND state = 'active'`
	}
	query += ` FOR UPDATE`
	var environmentID string
	if err := tx.QueryRow(ctx, query, deviceID).Scan(&environmentID); errors.Is(err, pgx.ErrNoRows) {
		return "", health.ErrNotFound
	} else if err != nil {
		return "", fmt.Errorf("lock health device: %w", err)
	}
	return environmentID, nil
}

func loadHealthProjection(ctx context.Context, queryer healthQueryer, deviceID string, forUpdate bool) (*health.Projection, error) {
	query := `
SELECT COALESCE(report_id::text, ''), sequence::text, observed_at, received_at, disconnected_at
FROM guardian_health.device_projections
WHERE device_id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var projection health.Projection
	var sequence string
	var observedAt *time.Time
	err := queryer.QueryRow(ctx, query, deviceID).Scan(
		&projection.ReportID, &sequence, &observedAt, &projection.ReceivedAt, &projection.DisconnectedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read health projection: %w", err)
	}
	projection.Sequence, err = strconv.ParseUint(sequence, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse health sequence: %w", err)
	}
	if observedAt != nil {
		projection.ObservedAt = *observedAt
	} else {
		projection.ObservedAt = projection.ReceivedAt
	}
	rows, err := queryer.Query(ctx, `
SELECT condition_type, status, reason_code, message,
       COALESCE(observed_revision::text, ''), last_transition_time
FROM guardian_health.device_conditions
WHERE device_id = $1
ORDER BY array_position(ARRAY[
    'edge_connected', 'device_certificate_ready', 'config_converged',
    'local_database_healthy', 'spool_healthy', 'clock_quality',
    'container_runtime_reachable', 'privileged_helper_reachable'
]::text[], condition_type)`, deviceID)
	if err != nil {
		return nil, fmt.Errorf("read health conditions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var condition health.Condition
		var revision string
		if err := rows.Scan(&condition.Type, &condition.Status, &condition.Reason, &condition.Message, &revision, &condition.LastTransitionTime); err != nil {
			return nil, fmt.Errorf("scan health condition: %w", err)
		}
		if revision != "" {
			value, parseErr := strconv.ParseUint(revision, 10, 64)
			if parseErr != nil {
				return nil, fmt.Errorf("parse health observed revision: %w", parseErr)
			}
			condition.ObservedRevision = &value
		}
		projection.Conditions = append(projection.Conditions, condition)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate health conditions: %w", err)
	}
	return &projection, nil
}

func saveHealthProjection(ctx context.Context, tx pgx.Tx, deviceID, environmentID string, projection health.Projection, payload []byte) error {
	var reportID any
	var observedAt any
	if projection.ReportID != "" {
		reportID = projection.ReportID
		observedAt = projection.ObservedAt
	}
	_, err := tx.Exec(ctx, `
INSERT INTO guardian_health.device_projections (
    device_id, environment_id, report_id, sequence, observed_at, received_at,
    disconnected_at, report_payload
) VALUES ($1,$2,$3,$4::numeric,$5,$6,$7,$8)
ON CONFLICT (device_id) DO UPDATE SET
    environment_id = excluded.environment_id,
    report_id = excluded.report_id,
    sequence = excluded.sequence,
    observed_at = excluded.observed_at,
    received_at = excluded.received_at,
    disconnected_at = excluded.disconnected_at,
    report_payload = excluded.report_payload`,
		deviceID, environmentID, reportID, strconv.FormatUint(projection.Sequence, 10),
		observedAt, projection.ReceivedAt, projection.DisconnectedAt, payload)
	if err != nil {
		return fmt.Errorf("persist health projection: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM guardian_health.device_conditions WHERE device_id = $1`, deviceID); err != nil {
		return fmt.Errorf("replace health conditions: %w", err)
	}
	for _, condition := range projection.Conditions {
		var revision any
		if condition.ObservedRevision != nil {
			revision = strconv.FormatUint(*condition.ObservedRevision, 10)
		}
		_, err := tx.Exec(ctx, `
INSERT INTO guardian_health.device_conditions (
    device_id, condition_type, status, reason_code, message,
    observed_revision, last_transition_time
) VALUES ($1,$2,$3,$4,$5,$6::numeric,$7)`,
			deviceID, condition.Type, condition.Status, condition.Reason, condition.Message,
			revision, condition.LastTransitionTime)
		if err != nil {
			return fmt.Errorf("persist health condition: %w", err)
		}
	}
	return nil
}
