package health

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestUnknownConditionsAreCompleteAndNeverGreen(t *testing.T) {
	conditions, err := UnknownConditions(testTime)
	if err != nil {
		t.Fatal(err)
	}
	if len(conditions) != len(conditionTypeOrder) {
		t.Fatalf("condition count = %d", len(conditions))
	}
	for index, condition := range conditions {
		expectedReason := "not_observed"
		if condition.Type == TypeEdgeConnected {
			expectedReason = "heartbeat_stale"
		}
		if condition.Type != conditionTypeOrder[index] || condition.Status != StatusUnknown || condition.Reason != expectedReason {
			t.Fatalf("condition %d = %#v", index, condition)
		}
	}
	aggregate, err := AggregateConditions(conditions)
	if err != nil {
		t.Fatal(err)
	}
	if aggregate.Status != StatusUnknown || aggregate.BlockingType != TypeEdgeConnected {
		t.Fatalf("aggregate = %#v", aggregate)
	}
}

func TestTransitionPreservesTimeForRefreshAndAdvancesForStateChange(t *testing.T) {
	revision := uint64(7)
	first, err := Transition(nil, Observation{
		Type: TypeConfigConverged, Status: StatusFalse, Reason: "revision_drift",
		Message: "Revision differs.", ObservedRevision: &revision,
	}, testTime)
	if err != nil {
		t.Fatal(err)
	}
	refreshRevision := uint64(8)
	refreshed, err := Transition(&first, Observation{
		Type: TypeConfigConverged, Status: StatusFalse, Reason: "revision_drift",
		Message: "Still differs.", ObservedRevision: &refreshRevision,
	}, testTime.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed.LastTransitionTime.Equal(first.LastTransitionTime) || refreshed.Message == first.Message || *refreshed.ObservedRevision != 8 {
		t.Fatalf("refresh = %#v", refreshed)
	}
	refreshRevision = 99
	if *refreshed.ObservedRevision != 8 {
		t.Fatal("transition retained caller revision pointer")
	}
	recovered, err := Transition(&refreshed, Observation{
		Type: TypeConfigConverged, Status: StatusTrue, Reason: "converged", Message: "Converged.",
	}, testTime.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !recovered.LastTransitionTime.Equal(testTime.Add(2 * time.Second)) {
		t.Fatalf("recovery transition = %s", recovered.LastTransitionTime)
	}
}

func TestTransitionRejectsNonMonotonicStateChange(t *testing.T) {
	previous := Condition{Type: TypeEdgeConnected, Status: StatusTrue, Reason: "connected", LastTransitionTime: testTime}
	_, err := Transition(&previous, Observation{
		Type: TypeEdgeConnected, Status: StatusFalse, Reason: "channel_disconnected",
	}, testTime)
	if !errors.Is(err, ErrNonMonotonicTransition) {
		t.Fatalf("error = %v", err)
	}
}

func TestConditionRejectsInvalidStatusReasonPairs(t *testing.T) {
	condition := Condition{
		Type: TypeDeviceCertificateReady, Status: StatusTrue, Reason: "expired",
		LastTransitionTime: testTime,
	}
	if !errors.Is(condition.Validate(), ErrInvalidConditionReason) {
		t.Fatalf("error = %v", condition.Validate())
	}
	condition.Type = "new_condition"
	if !errors.Is(condition.Validate(), ErrInvalidConditionType) {
		t.Fatalf("error = %v", condition.Validate())
	}
}

func TestConditionMessageUsesByteLimitAndRejectsSecretLikeContent(t *testing.T) {
	base := Condition{
		Type: TypeLocalDatabaseHealthy, Status: StatusFalse, Reason: "read_failed",
		LastTransitionTime: testTime,
	}
	base.Message = strings.Repeat("é", MaxMessageBytes/2)
	if err := base.Validate(); err != nil {
		t.Fatalf("exact byte limit: %v", err)
	}
	base.Message += "é"
	if !errors.Is(base.Validate(), ErrInvalidMessage) {
		t.Fatalf("oversized error = %v", base.Validate())
	}
	for _, message := range []string{"authorization: Bearer abc", "password=hunter2", "api_key=abc", "token: abc", "-----BEGIN PRIVATE KEY-----", "line\nbreak"} {
		base.Message = message
		if !errors.Is(base.Validate(), ErrInvalidMessage) {
			t.Fatalf("message %q error = %v", message, base.Validate())
		}
	}
	base.Message = "<script>alert(1)</script>"
	if err := base.Validate(); err != nil {
		t.Fatalf("hostile plain text must remain representable for escaped rendering: %v", err)
	}
}

func TestTimestampRequiresUTCAndMicrosecondPrecision(t *testing.T) {
	for _, value := range []time.Time{
		time.Time{},
		testTime.Add(time.Nanosecond),
		testTime.In(time.FixedZone("UTC-like", 0)),
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		if !errors.Is(ValidateTimestamp(value), ErrInvalidTimestamp) {
			t.Fatalf("timestamp %v unexpectedly valid", value)
		}
	}
	if err := ValidateTimestamp(time.Time{}.Add(time.Microsecond)); err != nil {
		t.Fatalf("minimum timestamp: %v", err)
	}
}
