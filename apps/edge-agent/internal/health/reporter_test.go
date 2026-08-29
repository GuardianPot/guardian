package health

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
)

type reporterStoreStub struct {
	mu    sync.Mutex
	state DurableState
}

func (stub *reporterStoreStub) LoadHealthState(context.Context) (DurableState, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	state := stub.state
	state.Conditions = append([]Condition(nil), stub.state.Conditions...)
	state.PendingPayload = bytes.Clone(stub.state.PendingPayload)
	if stub.state.PendingReport != nil {
		report := *stub.state.PendingReport
		state.PendingReport = &report
	}
	return state, nil
}

func (stub *reporterStoreStub) PersistHealthReport(_ context.Context, report Report, payload []byte) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.state.PendingReport != nil {
		return ErrReportPending
	}
	copyReport := report
	stub.state.PendingReport = &copyReport
	stub.state.PendingPayload = bytes.Clone(payload)
	stub.state.Conditions = report.Conditions
	stub.state.NextSequence = report.Sequence + 1
	return nil
}

func (stub *reporterStoreStub) AcknowledgeHealthReport(_ context.Context, reportID string, sequence uint64) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.state.PendingReport == nil || stub.state.PendingReport.ReportID != reportID || stub.state.PendingReport.Sequence != sequence {
		return ErrAcknowledgementMismatch
	}
	stub.state.PendingReport = nil
	stub.state.PendingPayload = nil
	return nil
}

type publisherStub struct {
	mu      sync.Mutex
	reports []Report
}

func (stub *publisherStub) PublishHealth(report Report) error {
	stub.mu.Lock()
	stub.reports = append(stub.reports, report)
	stub.mu.Unlock()
	return nil
}

func TestReporterPersistsBeforePublishReplaysAndAdvancesOnlyAfterMatchingAck(t *testing.T) {
	now := time.Date(2026, 8, 29, 13, 0, 0, 0, time.UTC)
	store := &reporterStoreStub{state: DurableState{NextSequence: 1}}
	collector := CollectorFunc(func(context.Context, time.Time) ([]Observation, error) {
		return []Observation{
			{Type: TypeDeviceCertificateReady, Status: StatusTrue, Reason: "valid"},
			{Type: TypeConfigConverged, Status: StatusUnknown, Reason: "not_observed"},
			{Type: TypeLocalDatabaseHealthy, Status: StatusTrue, Reason: "ready"},
			{Type: TypeSpoolHealthy, Status: StatusTrue, Reason: "ready"},
			{Type: TypeClockQuality, Status: StatusTrue, Reason: "synchronized"},
			{Type: TypeContainerRuntimeReachable, Status: StatusTrue, Reason: "reachable"},
			{Type: TypePrivilegedHelperReachable, Status: StatusTrue, Reason: "reachable"},
		}, nil
	})
	firstPublisher := &publisherStub{}
	first, err := NewReporter(store, firstPublisher, collector)
	if err != nil {
		t.Fatal(err)
	}
	first.now = func() time.Time { return now }
	first.newID = func(time.Time) (string, error) { return "0198dc8c-c600-7000-8000-000000000091", nil }
	_ = first.SetChannelState(context.Background(), "connected", "negotiated")
	if err := first.cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(firstPublisher.reports) != 1 || firstPublisher.reports[0].Sequence != 1 {
		t.Fatalf("first published reports = %+v", firstPublisher.reports)
	}
	pendingPayload := bytes.Clone(store.state.PendingPayload)

	restartPublisher := &publisherStub{}
	restarted, _ := NewReporter(store, restartPublisher, collector)
	restarted.now = func() time.Time { return now.Add(time.Second) }
	restarted.newID = func(time.Time) (string, error) { return "0198dc8c-c600-7000-8000-000000000092", nil }
	_ = restarted.SetChannelState(context.Background(), "connected", "negotiated")
	if err := restarted.cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(restartPublisher.reports) != 1 || restartPublisher.reports[0].ReportID != firstPublisher.reports[0].ReportID ||
		!bytes.Equal(pendingPayload, store.state.PendingPayload) {
		t.Fatalf("restart did not replay exact pending report: %+v", restartPublisher.reports)
	}
	if err := restarted.Acknowledgement(context.Background(), &devicev1.Acknowledgement{
		MessageId: firstPublisher.reports[0].ReportID,
		Kind:      devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_HEALTH_REPORT,
		Revision:  2,
	}); err != ErrAcknowledgementMismatch {
		t.Fatalf("mismatched acknowledgement error = %v", err)
	}
	if err := restarted.Acknowledgement(context.Background(), &devicev1.Acknowledgement{
		MessageId: firstPublisher.reports[0].ReportID,
		Kind:      devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_HEALTH_REPORT,
		Revision:  1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := restarted.cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := restartPublisher.reports[len(restartPublisher.reports)-1]; got.Sequence != 2 || got.ReportID == firstPublisher.reports[0].ReportID {
		t.Fatalf("post-ack report = %+v", got)
	}
}
