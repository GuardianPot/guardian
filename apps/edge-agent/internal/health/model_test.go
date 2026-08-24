package health

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

var edgeTestTime = time.Date(2026, 8, 24, 18, 0, 0, 0, time.UTC)

const edgeTestReportID = "01890f8e-7b5d-7cc3-98c4-dc0c0c07398f"

func TestUnknownSetIsCompleteCanonicalAndNeverGreen(t *testing.T) {
	set, err := NewUnknownSet(edgeTestTime)
	if err != nil {
		t.Fatal(err)
	}
	conditions := set.Conditions()
	if len(conditions) != conditionCount {
		t.Fatalf("condition count = %d", len(conditions))
	}
	for index, condition := range conditions {
		if condition.Type != conditionTypeOrder[index] || condition.Status != StatusUnknown || condition.Reason != "not_observed" {
			t.Fatalf("condition %d = %#v", index, condition)
		}
	}
}

func TestObservePreservesTransitionTimeForRefreshAndRecoversOneCondition(t *testing.T) {
	set, err := NewUnknownSet(edgeTestTime)
	if err != nil {
		t.Fatal(err)
	}
	revision := uint64(7)
	if err := set.Observe(Observation{
		Type: TypeConfigConverged, Status: StatusFalse, Reason: "revision_drift",
		Message: "Revision differs.", ObservedRevision: &revision,
	}, edgeTestTime.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	refreshRevision := uint64(8)
	if err := set.Observe(Observation{
		Type: TypeConfigConverged, Status: StatusFalse, Reason: "revision_drift",
		Message: "Still differs.", ObservedRevision: &refreshRevision,
	}, edgeTestTime.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	condition := set.Conditions()[2]
	if !condition.LastTransitionTime.Equal(edgeTestTime.Add(time.Second)) || *condition.ObservedRevision != 8 {
		t.Fatalf("refresh = %#v", condition)
	}
	refreshRevision = 99
	if *set.Conditions()[2].ObservedRevision != 8 {
		t.Fatal("set retained caller revision pointer")
	}
	if err := set.Observe(Observation{
		Type: TypeConfigConverged, Status: StatusTrue, Reason: "converged",
	}, edgeTestTime.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	conditions := set.Conditions()
	if conditions[2].Status != StatusTrue || !conditions[2].LastTransitionTime.Equal(edgeTestTime.Add(3*time.Second)) {
		t.Fatalf("recovery = %#v", conditions[2])
	}
	if conditions[0].Status != StatusUnknown || conditions[3].Status != StatusUnknown {
		t.Fatal("recovery changed an unrelated condition")
	}
}

func TestObserveRejectsInvalidTimestampWithoutMutation(t *testing.T) {
	set, err := NewUnknownSet(edgeTestTime)
	if err != nil {
		t.Fatal(err)
	}
	before := set.Conditions()
	err = set.Observe(Observation{
		Type: TypeEdgeConnected, Status: StatusTrue, Reason: "connected",
	}, edgeTestTime.Add(time.Nanosecond))
	if !errors.Is(err, ErrInvalidTimestamp) {
		t.Fatalf("error = %v", err)
	}
	if after := set.Conditions(); after[0] != before[0] {
		t.Fatalf("failed observation mutated condition: %#v", after[0])
	}
}

func TestReportRequiresFullSnapshotAndUUIDv7(t *testing.T) {
	set, err := NewUnknownSet(edgeTestTime)
	if err != nil {
		t.Fatal(err)
	}
	report, err := set.Report(edgeTestReportID, 1, edgeTestTime)
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != SchemaVersion || len(report.Conditions) != conditionCount {
		t.Fatalf("report = %#v", report)
	}
	if encoded, err := jsonMarshal(report); err != nil || len(encoded) > MaxReportBytes {
		t.Fatalf("encoded bytes=%d error=%v", len(encoded), err)
	}
	for _, test := range []struct {
		id       string
		sequence uint64
	}{
		{"01890f8e-7b5d-6cc3-98c4-dc0c0c07398f", 1},
		{strings.ToUpper(edgeTestReportID), 1},
		{edgeTestReportID, 0},
	} {
		if _, err := set.Report(test.id, test.sequence, edgeTestTime); !errors.Is(err, ErrInvalidReport) {
			t.Fatalf("id=%q sequence=%d error=%v", test.id, test.sequence, err)
		}
	}
}

func TestConditionRejectsSecretLikeAndOversizedMessages(t *testing.T) {
	condition := Condition{
		Type: TypeLocalDatabaseHealthy, Status: StatusFalse, Reason: "read_failed",
		LastTransitionTime: edgeTestTime,
	}
	condition.Message = strings.Repeat("é", MaxMessageBytes/2)
	if err := condition.Validate(); err != nil {
		t.Fatalf("exact byte limit: %v", err)
	}
	condition.Message += "é"
	if !errors.Is(condition.Validate(), ErrInvalidMessage) {
		t.Fatalf("oversized error = %v", condition.Validate())
	}
	for _, message := range []string{"authorization: Bearer abc", "password=hunter2", "api_key=abc", "token: abc", "line\nbreak"} {
		condition.Message = message
		if !errors.Is(condition.Validate(), ErrInvalidMessage) {
			t.Fatalf("message %q error = %v", message, condition.Validate())
		}
	}
	condition.Message = "<script>alert(1)</script>"
	if err := condition.Validate(); err != nil {
		t.Fatalf("plain text hostile input should be safely representable: %v", err)
	}
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}
