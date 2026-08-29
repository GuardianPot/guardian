package health

import (
	"context"
	"testing"
	"time"
)

const (
	testDeviceLow   = "018f0000-0000-7000-8000-000000000001"
	testDeviceHigh  = "018f0000-0000-7000-8000-000000000002"
	testEnvironment = "018f0000-0000-7000-8000-000000000010"
)

type serviceRepositoryStub struct {
	device      DeviceProjection
	environment []DeviceProjection
}

func (stub *serviceRepositoryStub) StoreHealthReport(context.Context, string, Report, time.Time) (ApplyOutcome, error) {
	return ApplyAccepted, nil
}
func (stub *serviceRepositoryStub) MarkHealthDisconnected(context.Context, string, time.Time) error {
	return nil
}
func (stub *serviceRepositoryStub) DeviceHealth(context.Context, string) (DeviceProjection, error) {
	return stub.device, nil
}
func (stub *serviceRepositoryStub) ActiveEnvironmentHealth(context.Context, string) ([]DeviceProjection, error) {
	return stub.environment, nil
}

func TestDeviceViewNeverInventsGreenBeforeEvidence(t *testing.T) {
	repository := &serviceRepositoryStub{device: DeviceProjection{
		DeviceID: testDeviceLow, EnvironmentID: testEnvironment, State: "active",
	}}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return testTime }
	view, err := service.Device(context.Background(), testDeviceLow)
	if err != nil {
		t.Fatal(err)
	}
	if view.Aggregate.Status != StatusUnknown || view.Aggregate.BlockingDeviceID != testDeviceLow {
		t.Fatalf("aggregate = %+v", view.Aggregate)
	}
	if len(view.Conditions) != 8 {
		t.Fatalf("conditions = %d", len(view.Conditions))
	}
	for _, condition := range view.Conditions {
		if condition.Status != StatusUnknown || condition.SourceDeviceID != testDeviceLow {
			t.Fatalf("condition = %+v", condition)
		}
	}
}

func TestEnvironmentViewUsesFalseUnknownTrueAndLowestUUIDTieBreak(t *testing.T) {
	lowConditions := healthyConditions(testTime)
	lowConditions[4].Status = StatusFalse
	lowConditions[4].Reason = "capacity_warning"
	highConditions := healthyConditions(testTime)
	highConditions[0].Status = StatusUnknown
	highConditions[0].Reason = "heartbeat_stale"
	highConditions[4].Status = StatusFalse
	highConditions[4].Reason = "capacity_critical"
	repository := &serviceRepositoryStub{environment: []DeviceProjection{
		{DeviceID: testDeviceHigh, EnvironmentID: testEnvironment, State: "active", Projection: &Projection{
			ReportID: "018f0000-0000-7000-8000-000000000012", Sequence: 1, ObservedAt: testTime,
			ReceivedAt: testTime, Conditions: highConditions,
		}},
		{DeviceID: testDeviceLow, EnvironmentID: testEnvironment, State: "active", Projection: &Projection{
			ReportID: "018f0000-0000-7000-8000-000000000011", Sequence: 1, ObservedAt: testTime,
			ReceivedAt: testTime, Conditions: lowConditions,
		}},
	}}
	service, _ := NewService(repository)
	service.now = func() time.Time { return testTime.Add(time.Second) }
	view, err := service.Environment(context.Background(), testEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	if view.Aggregate.Status != StatusFalse || view.Aggregate.BlockingType != TypeSpoolHealthy || view.Aggregate.BlockingDeviceID != testDeviceLow {
		t.Fatalf("aggregate = %+v", view.Aggregate)
	}
	if view.Conditions[4].SourceDeviceID != testDeviceLow {
		t.Fatalf("spool source = %q", view.Conditions[4].SourceDeviceID)
	}
}

func TestEnvironmentWithoutActiveDevicesIsExplicitUnknown(t *testing.T) {
	service, _ := NewService(&serviceRepositoryStub{})
	service.now = func() time.Time { return testTime }
	view, err := service.Environment(context.Background(), testEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	if view.Aggregate.Status != StatusUnknown || view.Aggregate.Reason != "no_active_devices" || view.Aggregate.BlockingDeviceID != "" {
		t.Fatalf("aggregate = %+v", view.Aggregate)
	}
}
