//go:build integration

package storage

import (
	"context"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/health"
)

func TestHealthProjectionPersistsAckDuplicateDisconnectAndRecovery(t *testing.T) {
	ctx := context.Background()
	databaseURL := createTestDatabase(t)
	if _, err := Migrate(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, databaseURL, 4)
	if err != nil {
		t.Fatal(err)
	}
	environmentService, err := environment.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	configured, err := environmentService.CreateEnvironment(ctx, "Health integration", environment.Mutation{ActorID: "owner-1"})
	if err != nil {
		t.Fatal(err)
	}
	const deviceID = "0198dc8c-c600-7000-8000-000000000044"
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := store.pool.Exec(ctx, `
INSERT INTO guardian_devices.devices (device_id, environment_id, display_name, state, created_at, updated_at)
VALUES ($1, $2, 'health-edge', 'active', $3, $3)`, deviceID, configured.EnvironmentID, now); err != nil {
		t.Fatal(err)
	}
	first := integrationHealthReport(1, "0198dc8c-c600-7000-8000-000000000045", now)
	if outcome, err := store.StoreHealthReport(ctx, deviceID, first, now.Add(time.Second)); err != nil || outcome != health.ApplyAccepted {
		t.Fatalf("first report = (%v, %v)", outcome, err)
	}
	if outcome, err := store.StoreHealthReport(ctx, deviceID, first, now.Add(2*time.Second)); err != nil || outcome != health.ApplyDuplicate {
		t.Fatalf("duplicate report = (%v, %v)", outcome, err)
	}
	if err := store.MarkHealthDisconnected(ctx, deviceID, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	store.Close()

	restarted, err := Open(ctx, databaseURL, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	record, err := restarted.DeviceHealth(ctx, deviceID)
	if err != nil {
		t.Fatal(err)
	}
	conditions, err := record.Projection.EffectiveConditions(now.Add(4 * time.Second))
	if err != nil || conditions[0].Status != health.StatusFalse || conditions[0].Reason != "channel_disconnected" {
		t.Fatalf("disconnected conditions = (%+v, %v)", conditions, err)
	}
	second := integrationHealthReport(2, "0198dc8c-c600-7000-8000-000000000046", now.Add(5*time.Second))
	for index := range second.Conditions {
		second.Conditions[index].LastTransitionTime = now
	}
	if _, err := restarted.StoreHealthReport(ctx, deviceID, second, now.Add(6*time.Second)); err != nil {
		t.Fatal(err)
	}
	recovered, err := restarted.DeviceHealth(ctx, deviceID)
	if err != nil {
		t.Fatal(err)
	}
	recoveredConditions, err := recovered.Projection.EffectiveConditions(now.Add(7 * time.Second))
	if err != nil || recoveredConditions[0].Status != health.StatusTrue || recovered.Projection.DisconnectedAt != nil {
		t.Fatalf("recovered projection = (%+v, %v)", recovered, err)
	}
}

func integrationHealthReport(sequence uint64, reportID string, at time.Time) health.Report {
	reasons := map[health.ConditionType]string{
		health.TypeEdgeConnected: "connected", health.TypeDeviceCertificateReady: "valid",
		health.TypeConfigConverged: "converged", health.TypeLocalDatabaseHealthy: "ready",
		health.TypeSpoolHealthy: "ready", health.TypeClockQuality: "synchronized",
		health.TypeContainerRuntimeReachable: "reachable", health.TypePrivilegedHelperReachable: "reachable",
	}
	conditions := make([]health.Condition, 0, 8)
	for _, conditionType := range health.ConditionTypes() {
		conditions = append(conditions, health.Condition{
			Type: conditionType, Status: health.StatusTrue, Reason: reasons[conditionType],
			Message: "Healthy.", LastTransitionTime: at,
		})
	}
	return health.Report{SchemaVersion: health.SchemaVersion, ReportID: reportID, Sequence: sequence, ObservedAt: at, Conditions: conditions}
}
