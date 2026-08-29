//go:build integration

package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/reconciliation"
)

const reconciliationDeviceID = "0198f7c4-7b30-7f11-8a44-111111111111"

func TestDesiredStatePublicationObservationAndAuditAreDurable(t *testing.T) {
	ctx := context.Background()
	databaseURL := createTestDatabase(t)
	if _, err := Migrate(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, databaseURL, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var environmentID, zoneID string
	if err := store.pool.QueryRow(ctx, `
INSERT INTO guardian_environment.environments (organization_id, display_name, name_key)
SELECT organization_id, 'Reconciliation environment', 'reconciliation environment'
FROM guardian_environment.organizations WHERE singleton
RETURNING environment_id::text`).Scan(&environmentID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `
INSERT INTO guardian_devices.devices (device_id, environment_id, display_name, state, created_at, updated_at)
VALUES ($1, $2, 'reconciliation-edge', 'active', clock_timestamp(), clock_timestamp())`,
		reconciliationDeviceID, environmentID); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `
INSERT INTO guardian_environment.zones (environment_id, display_name, name_key, network)
VALUES ($1, 'Primary', 'primary', '10.20.0.0/24')
RETURNING zone_id::text`, environmentID).Scan(&zoneID); err != nil {
		t.Fatal(err)
	}

	first, err := store.EnsureCurrent(ctx, reconciliationDeviceID, "device-channel")
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || first.EdgeConfiguration.EnvironmentID != environmentID || len(first.Zones) != 1 || first.Zones[0].ZoneID != zoneID {
		t.Fatalf("first desired state = %+v", first)
	}
	second, err := store.EnsureCurrent(ctx, reconciliationDeviceID, "device-channel")
	if err != nil || second.MessageID != first.MessageID || second.Revision != first.Revision {
		t.Fatalf("idempotent desired state = (%+v, %v)", second, err)
	}

	if _, err := store.pool.Exec(ctx, `
UPDATE guardian_environment.zones
SET display_name = 'Primary updated', revision = revision + 1, updated_at = clock_timestamp()
WHERE zone_id = $1`, zoneID); err != nil {
		t.Fatal(err)
	}
	updated, err := store.EnsureCurrent(ctx, reconciliationDeviceID, "device-channel")
	if err != nil || updated.Revision != 2 || updated.MessageID == first.MessageID || updated.Zones[0].SourceRevision != 2 {
		t.Fatalf("updated desired state = (%+v, %v)", updated, err)
	}

	const concurrentReaders = 8
	results := make(chan reconciliation.Snapshot, concurrentReaders)
	errorsFound := make(chan error, concurrentReaders)
	var wait sync.WaitGroup
	for index := 0; index < concurrentReaders; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value, callErr := store.EnsureCurrent(ctx, reconciliationDeviceID, "device-channel")
			if callErr != nil {
				errorsFound <- callErr
				return
			}
			results <- value
		}()
	}
	wait.Wait()
	close(results)
	close(errorsFound)
	for callErr := range errorsFound {
		t.Errorf("concurrent EnsureCurrent() error = %v", callErr)
	}
	for value := range results {
		if value.MessageID != updated.MessageID || value.Revision != 2 {
			t.Errorf("concurrent desired state = %+v", value)
		}
	}

	if err := store.AcknowledgeDesired(ctx, reconciliationDeviceID, reconciliation.Acknowledgement{
		MessageID: updated.MessageID, Revision: updated.Revision,
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	observed := reconciliation.ObservedState{
		MessageID:       "0198f7c4-7b30-7f11-8a44-222222222222",
		DesiredRevision: 2, ObservedRevision: 2, LastGoodRevision: 2,
		Condition: reconciliation.Condition{
			Status: reconciliation.ConditionConverged, ReasonCode: "applied",
			AttemptCount: 1, LastTransitionTime: now,
		},
	}
	if err := store.RecordObserved(ctx, reconciliationDeviceID, observed); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordObserved(ctx, reconciliationDeviceID, observed); err != nil {
		t.Fatalf("duplicate observation was not idempotent: %v", err)
	}

	var revisions, audits, observations int
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM guardian_reconciliation.desired_state_revisions WHERE device_id = $1`, reconciliationDeviceID).Scan(&revisions); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM guardian_audit.records WHERE action = 'desired_state.revision.published' AND correlation_id = $1`, reconciliationDeviceID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM guardian_reconciliation.observed_state WHERE device_id = $1 AND condition_status = 'converged'`, reconciliationDeviceID).Scan(&observations); err != nil {
		t.Fatal(err)
	}
	if revisions != 2 || audits != 2 || observations != 1 {
		t.Fatalf("durable evidence counts = revisions:%d audits:%d observations:%d", revisions, audits, observations)
	}
}
