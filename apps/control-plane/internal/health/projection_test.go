package health

import (
	"errors"
	"testing"
	"time"
)

func TestApplyReportAcceptsExactDuplicateAndRejectsConflicts(t *testing.T) {
	firstReport := healthyReport(1, testReportID, testTime)
	projection, outcome, err := ApplyReport(nil, firstReport, testTime.Add(time.Second))
	if err != nil || outcome != ApplyAccepted {
		t.Fatalf("first apply outcome=%v error=%v", outcome, err)
	}
	duplicate, outcome, err := ApplyReport(&projection, firstReport, testTime.Add(2*time.Second))
	if err != nil || outcome != ApplyDuplicate || !duplicate.ReceivedAt.Equal(projection.ReceivedAt) {
		t.Fatalf("duplicate outcome=%v error=%v projection=%#v", outcome, err, duplicate)
	}
	conflict := firstReport
	conflict.ReportID = "01890f8e-7b5d-7cc3-88c4-dc0c0c07398f"
	if _, _, err := ApplyReport(&projection, conflict, testTime.Add(2*time.Second)); !errors.Is(err, ErrReportConflict) {
		t.Fatalf("conflict error = %v", err)
	}
	older := firstReport
	older.Sequence = 0
	if _, _, err := ApplyReport(&projection, older, testTime.Add(2*time.Second)); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("invalid older error = %v", err)
	}
	projection.Sequence = 2
	older.Sequence = 1
	if _, _, err := ApplyReport(&projection, older, testTime.Add(2*time.Second)); !errors.Is(err, ErrOutOfOrderReport) {
		t.Fatalf("out-of-order error = %v", err)
	}
}

func TestApplyReportProtectsTransitionHistory(t *testing.T) {
	firstReport := healthyReport(1, testReportID, testTime)
	projection, _, err := ApplyReport(nil, firstReport, testTime.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	next := healthyReport(2, "01890f8e-7b5d-7cc3-a8c4-dc0c0c07398f", testTime.Add(time.Second))
	next.Conditions[0].LastTransitionTime = testTime.Add(time.Second)
	if _, _, err := ApplyReport(&projection, next, testTime.Add(2*time.Second)); !errors.Is(err, ErrTransitionHistory) {
		t.Fatalf("rewritten refresh error = %v", err)
	}
	next.Conditions[0] = Condition{
		Type: TypeEdgeConnected, Status: StatusFalse, Reason: "channel_disconnected",
		LastTransitionTime: testTime,
	}
	if _, _, err := ApplyReport(&projection, next, testTime.Add(2*time.Second)); !errors.Is(err, ErrTransitionHistory) {
		t.Fatalf("non-monotonic change error = %v", err)
	}
}

func TestStalenessUsesServerReceiveTimeAndDoesNotMutateStoredTruth(t *testing.T) {
	report := healthyReport(1, testReportID, testTime.Add(24*time.Hour))
	receivedAt := testTime.Add(time.Second)
	projection, _, err := ApplyReport(nil, report, receivedAt)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := projection.EffectiveConditions(receivedAt.Add(StaleAfter))
	if err != nil || fresh[0].Status != StatusTrue {
		t.Fatalf("fresh conditions=%#v error=%v", fresh, err)
	}
	stale, err := projection.EffectiveConditions(receivedAt.Add(StaleAfter + TimestampPrecision))
	if err != nil {
		t.Fatal(err)
	}
	if stale[0].Status != StatusUnknown || stale[0].Reason != "heartbeat_stale" || !stale[0].LastTransitionTime.Equal(receivedAt.Add(StaleAfter)) {
		t.Fatalf("stale condition = %#v", stale[0])
	}
	if projection.Conditions[0].Status != StatusTrue {
		t.Fatal("effective staleness mutated accepted projection")
	}
	aggregate, err := AggregateConditions(stale)
	if err != nil || aggregate.Status != StatusUnknown || aggregate.BlockingType != TypeEdgeConnected {
		t.Fatalf("aggregate=%#v error=%v", aggregate, err)
	}
}

func TestDisconnectAndAggregateFailurePriority(t *testing.T) {
	projection := Projection{
		ReportID: testReportID, Sequence: 1, ObservedAt: testTime,
		ReceivedAt: testTime, Conditions: healthyConditions(testTime),
	}
	disconnected, err := projection.MarkDisconnected(testTime.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	effective, err := disconnected.EffectiveConditions(testTime.Add(2 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if effective[0].Status != StatusFalse || effective[1].Status != StatusTrue || disconnected.Conditions[0].Status != StatusTrue {
		t.Fatalf("effective=%#v stored=%#v", effective[:2], disconnected.Conditions[:2])
	}
	conditions, err := UnknownConditions(testTime)
	if err != nil {
		t.Fatal(err)
	}
	conditions[3] = Condition{Type: TypeLocalDatabaseHealthy, Status: StatusFalse, Reason: "read_failed", LastTransitionTime: testTime}
	aggregate, err := AggregateConditions(conditions)
	if err != nil || aggregate.Status != StatusFalse || aggregate.BlockingType != TypeLocalDatabaseHealthy {
		t.Fatalf("aggregate=%#v error=%v", aggregate, err)
	}
}

func TestEveryConditionBlocksGreen(t *testing.T) {
	conditions := healthyConditions(testTime)
	aggregate, err := AggregateConditions(conditions)
	if err != nil || aggregate.Status != StatusTrue {
		t.Fatalf("healthy aggregate=%#v error=%v", aggregate, err)
	}
	for index, condition := range conditions {
		degraded := cloneConditions(conditions)
		for reason, status := range reasonStatuses[condition.Type] {
			if status == StatusFalse {
				degraded[index].Status = status
				degraded[index].Reason = reason
				break
			}
		}
		aggregate, err := AggregateConditions(degraded)
		if err != nil || aggregate.Status != StatusFalse || aggregate.BlockingType != condition.Type {
			t.Fatalf("type=%s aggregate=%#v error=%v", condition.Type, aggregate, err)
		}
	}
}

func TestFreshReportClearsDisconnectOverrideWithoutRewritingEdgeHistory(t *testing.T) {
	firstReport := healthyReport(1, testReportID, testTime.Add(24*time.Hour))
	projection, _, err := ApplyReport(nil, firstReport, testTime)
	if err != nil {
		t.Fatal(err)
	}
	disconnected, err := projection.MarkDisconnected(testTime.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	duplicate, outcome, err := ApplyReport(&disconnected, firstReport, testTime.Add(2*time.Second))
	if err != nil || outcome != ApplyDuplicate {
		t.Fatalf("duplicate outcome=%v error=%v", outcome, err)
	}
	effective, err := duplicate.EffectiveConditions(testTime.Add(2 * time.Second))
	if err != nil || effective[0].Status != StatusFalse {
		t.Fatalf("duplicate effective=%#v error=%v", effective[0], err)
	}

	nextReport := healthyReport(2, "01890f8e-7b5d-7cc3-a8c4-dc0c0c07398f", testTime.Add(24*time.Hour+time.Second))
	for index := range nextReport.Conditions {
		nextReport.Conditions[index].LastTransitionTime = firstReport.Conditions[index].LastTransitionTime
	}
	reconnected, outcome, err := ApplyReport(&disconnected, nextReport, testTime.Add(3*time.Second))
	if err != nil || outcome != ApplyAccepted {
		t.Fatalf("reconnect outcome=%v error=%v", outcome, err)
	}
	effective, err = reconnected.EffectiveConditions(testTime.Add(3 * time.Second))
	if err != nil || effective[0].Status != StatusTrue {
		t.Fatalf("reconnected effective=%#v error=%v", effective[0], err)
	}
}
