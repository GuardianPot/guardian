package decoy

import (
	"context"
	"testing"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
)

const testDecoyID = "0198f7c4-7b30-7f11-8a44-666666666666"

func desiredSnapshot() *devicev1.DesiredStateSnapshot {
	return &devicev1.DesiredStateSnapshot{
		MessageId: "0198f7c4-7b30-7f11-8a44-333333333333", Revision: 4,
		EdgeConfiguration: &devicev1.EdgeConfiguration{
			DeviceId:      "0198f7c4-7b30-7f11-8a44-111111111111",
			EnvironmentId: "0198f7c4-7b30-7f11-8a44-222222222222",
		},
		Decoys: []*devicev1.DecoyDesiredObject{{
			DecoyId: testDecoyID, ZoneId: "0198f7c4-7b30-7f11-8a44-444444444444",
			DisplayName: "Finance file server", Family: devicev1.DecoyFamily_DECOY_FAMILY_SMB,
			Persona:          devicev1.DecoyPersona_DECOY_PERSONA_WINDOWS_FILE_SERVICE_HOST,
			InteractionLevel: devicev1.DecoyInteractionLevel_DECOY_INTERACTION_LEVEL_LOW,
			Address:          "10.20.0.40", Pack: "smb-fileshare", PackVersion: "0.1.0",
			DesiredState:   devicev1.DecoyDesiredLifecycle_DECOY_DESIRED_LIFECYCLE_DEPLOYED,
			SourceRevision: 2,
		}},
	}
}

// The null runtime's whole contract: it may report only that it does not know.
// If this test ever passes with a deployed or absent state, the Edge is
// claiming coverage it has not looked for.
func TestNullRuntimeReportsOnlyUnknown(t *testing.T) {
	runtime := NewNullRuntime()
	desired, err := desiredFromSnapshot(desiredSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	observations, err := runtime.Converge(context.Background(), desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 1 {
		t.Fatalf("Converge() returned %d observations, want 1", len(observations))
	}
	observation := observations[0]
	if observation.DecoyID != testDecoyID {
		t.Fatalf("observed decoy = %q", observation.DecoyID)
	}
	if observation.State != devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_UNKNOWN {
		t.Fatalf("null runtime reported %v; it converges nothing and may report only unknown", observation.State)
	}
	if len(observation.Conditions) != len(conditionOrder) {
		t.Fatalf("null runtime reported %d conditions, want the complete set of %d",
			len(observation.Conditions), len(conditionOrder))
	}
	for index, condition := range observation.Conditions {
		if condition.Type != conditionOrder[index] {
			t.Fatalf("condition %d = %v, want %v", index, condition.Type, conditionOrder[index])
		}
		if condition.Status != devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNKNOWN {
			t.Fatalf("condition %v status = %v; nothing observed it", condition.Type, condition.Status)
		}
	}
}

// A report is always the complete ordered dimension set. A runtime that
// reports two dimensions must not have the other four read as absent problems.
func TestPartialRuntimeConditionsAreCompletedAsUnknown(t *testing.T) {
	observedAt := time.Now().UTC()
	wire := conditionsToWire([]Condition{{
		Type:               devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_RUNTIME_HEALTHY,
		Status:             devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_TRUE,
		Reason:             "running",
		LastTransitionTime: observedAt,
	}}, observedAt)
	if len(wire) != len(conditionOrder) {
		t.Fatalf("conditionsToWire() returned %d conditions, want %d", len(wire), len(conditionOrder))
	}
	for index, condition := range wire {
		if condition.Type != conditionOrder[index] {
			t.Fatalf("condition %d = %v, want %v", index, condition.Type, conditionOrder[index])
		}
		if index == 0 {
			continue
		}
		if condition.Status != devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNKNOWN {
			t.Fatalf("unreported condition %v = %v, want Unknown", condition.Type, condition.Status)
		}
	}
}

func TestDesiredFromSnapshotCarriesTheLifecycleAndRejectsOverBound(t *testing.T) {
	snapshot := desiredSnapshot()
	desired, err := desiredFromSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !desired[0].Enabled || desired[0].Pack != "smb-fileshare" || desired[0].SourceRevision != 2 {
		t.Fatalf("desired = %+v", desired[0])
	}
	snapshot.Decoys[0].DesiredState = devicev1.DecoyDesiredLifecycle_DECOY_DESIRED_LIFECYCLE_DISABLED
	disabled, err := desiredFromSnapshot(snapshot)
	if err != nil || disabled[0].Enabled {
		t.Fatalf("disabled decoy = %+v, err = %v", disabled, err)
	}
	for len(snapshot.Decoys) <= MaxDecoys {
		snapshot.Decoys = append(snapshot.Decoys, snapshot.Decoys[0])
	}
	if _, err := desiredFromSnapshot(snapshot); err == nil {
		t.Fatal("desiredFromSnapshot() accepted more than the 64-decoy bound")
	}
}
