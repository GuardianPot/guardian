package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/reconciliation"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage/dbgen"
	"github.com/jackc/pgx/v5"
)

var _ reconciliation.Repository = (*Store)(nil)

func (s *Store) EnsureCurrent(ctx context.Context, deviceID, actorID string) (reconciliation.Snapshot, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return reconciliation.Snapshot{}, fmt.Errorf("begin desired-state transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var environmentID string
	if err := tx.QueryRow(ctx, `
SELECT environment_id::text
FROM guardian_devices.devices
WHERE device_id = $1 AND state = 'active'
FOR UPDATE`, deviceID).Scan(&environmentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return reconciliation.Snapshot{}, reconciliation.ErrNotFound
		}
		return reconciliation.Snapshot{}, fmt.Errorf("lock active device for desired state: %w", err)
	}

	rows, err := tx.Query(ctx, `
SELECT zone_id::text, display_name, network::text, revision
FROM guardian_environment.zones
WHERE environment_id = $1
ORDER BY zone_id`, environmentID)
	if err != nil {
		return reconciliation.Snapshot{}, fmt.Errorf("read desired-state zones: %w", err)
	}
	zones := []reconciliation.Zone{}
	for rows.Next() {
		var zone reconciliation.Zone
		var revision int64
		if err := rows.Scan(&zone.ZoneID, &zone.DisplayName, &zone.CIDR, &revision); err != nil {
			rows.Close()
			return reconciliation.Snapshot{}, fmt.Errorf("scan desired-state zone: %w", err)
		}
		zone.SourceRevision = uint64(revision)
		zones = append(zones, zone)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return reconciliation.Snapshot{}, fmt.Errorf("iterate desired-state zones: %w", err)
	}
	rows.Close()
	sort.Slice(zones, func(i, j int) bool { return zones[i].ZoneID < zones[j].ZoneID })

	candidate := reconciliation.Snapshot{
		EdgeConfiguration: reconciliation.EdgeConfiguration{DeviceID: deviceID, EnvironmentID: environmentID},
		Zones:             zones, PlaceholderDecoys: []reconciliation.PlaceholderDecoy{},
	}
	digest, err := reconciliation.ContentDigest(candidate)
	if err != nil {
		return reconciliation.Snapshot{}, err
	}

	var latestRevision int64
	var latestDigest, latestPayload []byte
	err = tx.QueryRow(ctx, `
SELECT revision, content_sha256, payload
FROM guardian_reconciliation.desired_state_revisions
WHERE device_id = $1
ORDER BY revision DESC
LIMIT 1`, deviceID).Scan(&latestRevision, &latestDigest, &latestPayload)
	if err == nil && bytes.Equal(latestDigest, digest[:]) {
		snapshot, parseErr := reconciliation.ParseSnapshot(latestPayload)
		if parseErr != nil {
			return reconciliation.Snapshot{}, fmt.Errorf("parse stored desired state: %w", parseErr)
		}
		if err := tx.Commit(ctx); err != nil {
			return reconciliation.Snapshot{}, fmt.Errorf("commit desired-state read: %w", err)
		}
		return snapshot, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return reconciliation.Snapshot{}, fmt.Errorf("read latest desired state: %w", err)
	}

	var messageID string
	if err := tx.QueryRow(ctx, "SELECT uuidv7()::text").Scan(&messageID); err != nil {
		return reconciliation.Snapshot{}, fmt.Errorf("create desired-state message identity: %w", err)
	}
	candidate.MessageID = messageID
	candidate.Revision = uint64(latestRevision + 1)
	payload, err := reconciliation.MarshalSnapshot(candidate)
	if err != nil {
		return reconciliation.Snapshot{}, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO guardian_reconciliation.desired_state_revisions (
    device_id, revision, message_id, content_sha256, payload
) VALUES ($1, $2, $3, $4, $5::jsonb)`,
		deviceID, candidate.Revision, candidate.MessageID, digest[:], payload); err != nil {
		return reconciliation.Snapshot{}, fmt.Errorf("persist desired-state revision: %w", err)
	}
	after, err := audit.NewSnapshot(map[string]any{
		"device_id": deviceID, "environment_id": environmentID,
		"revision": candidate.Revision, "content_sha256": reconciliation.DigestHex(digest),
		"zone_count": len(candidate.Zones), "placeholder_decoy_count": len(candidate.PlaceholderDecoys),
	})
	if err != nil {
		return reconciliation.Snapshot{}, fmt.Errorf("build desired-state audit snapshot: %w", err)
	}
	_, err = (auditQueryAppender{queries: dbgen.New(tx)}).Append(ctx, audit.Event{
		OccurredAt: time.Now().UTC(), Actor: audit.Actor{Type: audit.ActorTypeSystem, ID: actorID},
		Action:        audit.ActionDesiredStatePublished,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeDesiredStateRevision, ID: candidate.MessageID},
		CorrelationID: deviceID, After: &after,
	})
	if err != nil {
		return reconciliation.Snapshot{}, fmt.Errorf("append desired-state audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return reconciliation.Snapshot{}, fmt.Errorf("commit desired-state revision: %w", err)
	}
	return candidate, nil
}

func (s *Store) RecordObserved(ctx context.Context, deviceID string, observed reconciliation.ObservedState) error {
	if err := reconciliation.ValidateObserved(observed); err != nil {
		return err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin observed-state transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `
INSERT INTO guardian_reconciliation.observed_messages (device_id, message_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING`, deviceID, observed.MessageID)
	if err != nil {
		return fmt.Errorf("record observed-state message: %w", err)
	}
	if result.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1 FROM guardian_reconciliation.desired_state_revisions
    WHERE device_id = $1 AND revision = $2
)`, deviceID, observed.DesiredRevision).Scan(&exists); err != nil {
		return fmt.Errorf("verify observed desired revision: %w", err)
	}
	if !exists {
		return reconciliation.ErrInvalidObserved
	}
	_, err = tx.Exec(ctx, `
INSERT INTO guardian_reconciliation.observed_state (
    device_id, message_id, desired_revision, observed_revision,
    last_good_revision, condition_status, reason_code, attempt_count,
    retry_at, last_transition_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT (device_id) DO UPDATE SET
    message_id = excluded.message_id,
    desired_revision = excluded.desired_revision,
    observed_revision = excluded.observed_revision,
    last_good_revision = excluded.last_good_revision,
    condition_status = excluded.condition_status,
    reason_code = excluded.reason_code,
    attempt_count = excluded.attempt_count,
    retry_at = excluded.retry_at,
    last_transition_at = excluded.last_transition_at,
    updated_at = clock_timestamp()
WHERE excluded.desired_revision >= guardian_reconciliation.observed_state.desired_revision`,
		deviceID, observed.MessageID, observed.DesiredRevision, observed.ObservedRevision,
		observed.LastGoodRevision, observed.Condition.Status, observed.Condition.ReasonCode,
		observed.Condition.AttemptCount, observed.Condition.RetryAt, observed.Condition.LastTransitionTime)
	if err != nil {
		return fmt.Errorf("project observed state: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit observed state: %w", err)
	}
	return nil
}

func (s *Store) AcknowledgeDesired(ctx context.Context, deviceID string, acknowledgement reconciliation.Acknowledgement) error {
	result, err := s.pool.Exec(ctx, `
UPDATE guardian_reconciliation.desired_state_revisions
SET acknowledged_at = COALESCE(acknowledged_at, clock_timestamp())
WHERE device_id = $1 AND message_id = $2 AND revision = $3`,
		deviceID, acknowledgement.MessageID, acknowledgement.Revision)
	if err != nil {
		return fmt.Errorf("acknowledge desired state: %w", err)
	}
	if result.RowsAffected() != 1 {
		return reconciliation.ErrInvalidObserved
	}
	return nil
}
