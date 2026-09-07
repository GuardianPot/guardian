package deception

import (
	"errors"
	"testing"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel/gen/guardian/device/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var wireConditionOrder = [...]devicev1.DecoyConditionType{
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_RUNTIME_HEALTHY,
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_ADDRESS_APPLIED,
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_PORT_RESPONDING,
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_TELEMETRY_REPORTING,
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_POLICY_APPLIED,
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_VERSION_MATCHES_DESIRED,
}

func wireReport(observedAt time.Time) *devicev1.DecoyStateReport {
	conditions := make([]*devicev1.DecoyCondition, 0, len(wireConditionOrder))
	for _, conditionType := range wireConditionOrder {
		conditions = append(conditions, &devicev1.DecoyCondition{
			Type:               conditionType,
			Status:             devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNKNOWN,
			Reason:             "not_observed",
			LastTransitionTime: timestamppb.New(observedAt),
		})
	}
	return &devicev1.DecoyStateReport{
		ReportId: testReportID, ObservedAt: timestamppb.New(observedAt),
		Decoys: []*devicev1.DecoyObservation{{
			DecoyId:         testDecoyID,
			State:           devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_UNKNOWN,
			DesiredRevision: 7,
			Conditions:      conditions,
		}},
	}
}

// The reporting device comes from the mTLS identity, never from a field in the
// report, so one Edge cannot attribute an observation to another.
func TestReportFromWireTakesTheDeviceFromTheAuthenticatedIdentity(t *testing.T) {
	observedAt := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	report, err := reportFromWire(testDeviceID, wireReport(observedAt))
	if err != nil {
		t.Fatal(err)
	}
	if report.DeviceID != testDeviceID {
		t.Fatalf("report device = %q", report.DeviceID)
	}
	if len(report.Observations) != 1 {
		t.Fatalf("observations = %d", len(report.Observations))
	}
	observation := report.Observations[0]
	if observation.ReportingDeviceID != testDeviceID {
		t.Fatalf("observation device = %q", observation.ReportingDeviceID)
	}
	if observation.State != ObservedUnknown {
		t.Fatalf("observed state = %q", observation.State)
	}
	if observation.DesiredRevision == nil || *observation.DesiredRevision != 7 {
		t.Fatalf("desired revision = %v", observation.DesiredRevision)
	}
	types := ConditionTypes()
	for index, condition := range observation.Conditions {
		if condition.Type != types[index] || condition.Status != StatusUnknown {
			t.Fatalf("condition %d = %+v", index, condition)
		}
	}
}

func TestReportFromWireRejectsUnspecifiedAndOutOfOrderValues(t *testing.T) {
	observedAt := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	cases := map[string]func(*devicev1.DecoyStateReport){
		"unspecified lifecycle": func(r *devicev1.DecoyStateReport) {
			r.Decoys[0].State = devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_UNSPECIFIED
		},
		"unspecified condition type": func(r *devicev1.DecoyStateReport) {
			r.Decoys[0].Conditions[0].Type = devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_UNSPECIFIED
		},
		"unspecified condition status": func(r *devicev1.DecoyStateReport) {
			r.Decoys[0].Conditions[1].Status = devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNSPECIFIED
		},
		"reordered conditions": func(r *devicev1.DecoyStateReport) {
			r.Decoys[0].Conditions[0], r.Decoys[0].Conditions[1] = r.Decoys[0].Conditions[1], r.Decoys[0].Conditions[0]
		},
		"partial condition set": func(r *devicev1.DecoyStateReport) {
			r.Decoys[0].Conditions = r.Decoys[0].Conditions[:3]
		},
		"missing observation timestamp": func(r *devicev1.DecoyStateReport) { r.ObservedAt = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			report := wireReport(observedAt)
			mutate(report)
			if _, err := reportFromWire(testDeviceID, report); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("reportFromWire() = %v, want rejection", err)
			}
		})
	}
}
