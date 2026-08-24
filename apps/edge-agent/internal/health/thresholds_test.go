package health

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestCertificateThresholds(t *testing.T) {
	tests := []struct {
		name           string
		notAfter       time.Time
		revoked        bool
		rotationFailed bool
		clockReliable  bool
		status         Status
		reason         string
	}{
		{"valid", edgeTestTime.Add(CertificateRotationWindow + time.Microsecond), false, false, true, StatusTrue, "valid"},
		{"rotation-boundary", edgeTestTime.Add(CertificateRotationWindow), false, false, true, StatusFalse, "rotation_window"},
		{"expired", edgeTestTime, false, false, true, StatusFalse, "expired"},
		{"revoked", edgeTestTime.Add(20 * 24 * time.Hour), true, false, true, StatusFalse, "revoked"},
		{"rotation-failed", edgeTestTime.Add(20 * 24 * time.Hour), false, true, true, StatusFalse, "rotation_failed"},
		{"clock-unreliable", time.Time{}, false, false, false, StatusUnknown, "clock_unreliable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation, err := EvaluateCertificate(edgeTestTime, test.notAfter, test.revoked, test.rotationFailed, test.clockReliable)
			if err != nil || observation.Status != test.status || observation.Reason != test.reason {
				t.Fatalf("observation=%#v error=%v", observation, err)
			}
		})
	}
}

func TestClockThresholdsAndUnavailableMeasurement(t *testing.T) {
	tests := []struct {
		available    bool
		synchronized bool
		offset       time.Duration
		status       Status
		reason       string
	}{
		{false, false, 0, StatusUnknown, "measurement_unavailable"},
		{true, false, 0, StatusFalse, "unsynchronized"},
		{true, true, MaximumHealthyClockOffset, StatusTrue, "synchronized"},
		{true, true, -MaximumHealthyClockOffset, StatusTrue, "synchronized"},
		{true, true, MaximumHealthyClockOffset + time.Nanosecond, StatusFalse, "offset_exceeded"},
		{true, true, time.Duration(math.MinInt64), StatusFalse, "offset_exceeded"},
	}
	for _, test := range tests {
		observation := EvaluateClock(test.available, test.synchronized, test.offset)
		if observation.Status != test.status || observation.Reason != test.reason {
			t.Fatalf("offset=%s observation=%#v", test.offset, observation)
		}
	}
}

func TestSpoolThresholds(t *testing.T) {
	tests := []struct {
		name       string
		available  bool
		used       int64
		configured int64
		free       float64
		status     Status
		reason     string
	}{
		{"unavailable", false, 0, 0, 0, StatusUnknown, "measurement_unavailable"},
		{"ready", true, 799, 1000, 10.1, StatusTrue, "ready"},
		{"capacity-warning", true, 800, 1000, 50, StatusFalse, "capacity_warning"},
		{"disk-warning", true, 100, 1000, 10, StatusFalse, "capacity_warning"},
		{"capacity-critical", true, 950, 1000, 50, StatusFalse, "capacity_critical"},
		{"disk-critical", true, 100, 1000, 5, StatusFalse, "capacity_critical"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation, err := EvaluateSpool(test.available, test.used, test.configured, test.free)
			if err != nil || observation.Status != test.status || observation.Reason != test.reason {
				t.Fatalf("observation=%#v error=%v", observation, err)
			}
		})
	}
	if _, err := EvaluateSpool(true, -1, 100, 50); !errors.Is(err, ErrInvalidMeasurement) {
		t.Fatalf("invalid measurement error = %v", err)
	}
}

func TestDatabaseAndProbeEvidence(t *testing.T) {
	for _, test := range []struct {
		observation Observation
		status      Status
		reason      string
	}{
		{EvaluateDatabase(false, false, false, false), StatusUnknown, "not_observed"},
		{EvaluateDatabase(true, true, true, true), StatusTrue, "ready"},
		{EvaluateDatabase(true, false, true, true), StatusFalse, "read_failed"},
		{EvaluateDatabase(true, true, false, true), StatusFalse, "write_failed"},
		{EvaluateDatabase(true, true, true, false), StatusFalse, "integrity_failed"},
	} {
		if test.observation.Status != test.status || test.observation.Reason != test.reason {
			t.Fatalf("database observation = %#v", test.observation)
		}
	}
	if ProbeTimeout != 2*time.Second {
		t.Fatalf("probe timeout = %s", ProbeTimeout)
	}
	for _, test := range []struct {
		typeValue ConditionType
		available bool
		timedOut  bool
		status    Status
		reason    string
	}{
		{TypeContainerRuntimeReachable, true, false, StatusTrue, "reachable"},
		{TypeContainerRuntimeReachable, false, false, StatusFalse, "probe_failed"},
		{TypeContainerRuntimeReachable, false, true, StatusFalse, "probe_timeout"},
		{TypePrivilegedHelperReachable, true, false, StatusTrue, "reachable"},
		{TypePrivilegedHelperReachable, false, false, StatusFalse, "rpc_unavailable"},
		{TypePrivilegedHelperReachable, false, true, StatusFalse, "rpc_timeout"},
	} {
		observation, err := EvaluateProbe(test.typeValue, test.available, test.timedOut)
		if err != nil || observation.Status != test.status || observation.Reason != test.reason {
			t.Fatalf("probe observation=%#v error=%v", observation, err)
		}
	}
}

func TestFreshProbeRecoveryChangesOnlyItsCondition(t *testing.T) {
	set, err := NewUnknownSet(edgeTestTime)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := EvaluateProbe(TypeContainerRuntimeReachable, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := set.Observe(failed, edgeTestTime.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	recovered, err := EvaluateProbe(TypeContainerRuntimeReachable, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := set.Observe(recovered, edgeTestTime.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	conditions := set.Conditions()
	if conditions[6].Status != StatusTrue || conditions[6].Reason != "reachable" {
		t.Fatalf("runtime recovery = %#v", conditions[6])
	}
	if conditions[7].Status != StatusUnknown || conditions[7].Reason != "not_observed" {
		t.Fatalf("runtime recovery changed helper = %#v", conditions[7])
	}
}
