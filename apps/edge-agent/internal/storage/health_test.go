package storage

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/health"
)

func TestHealthReportStateSurvivesRestartAndOnlyMatchingAckClearsPending(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	options := Options{DatabasePath: filepath.Join(root, "edge.db"), SpoolDirectory: filepath.Join(root, "spool")}
	store, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	set, err := health.NewUnknownSet(now)
	if err != nil {
		t.Fatal(err)
	}
	report, err := set.Report("0198dc8c-c600-7000-8000-000000000090", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := health.MarshalReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PersistHealthReport(ctx, report, payload); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	state, err := restarted.LoadHealthState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.NextSequence != 2 || state.PendingReport == nil || state.PendingReport.ReportID != report.ReportID ||
		!bytes.Equal(state.PendingPayload, payload) || len(state.Conditions) != 8 {
		t.Fatalf("restarted health state = %+v", state)
	}
	if err := restarted.AcknowledgeHealthReport(ctx, report.ReportID, 2); !errors.Is(err, health.ErrAcknowledgementMismatch) {
		t.Fatalf("mismatched acknowledgement error = %v", err)
	}
	stillPending, err := restarted.LoadHealthState(ctx)
	if err != nil || stillPending.PendingReport == nil {
		t.Fatalf("mismatch cleared pending report: (%+v, %v)", stillPending, err)
	}
	if err := restarted.AcknowledgeHealthReport(ctx, report.ReportID, report.Sequence); err != nil {
		t.Fatal(err)
	}
	acknowledged, err := restarted.LoadHealthState(ctx)
	if err != nil || acknowledged.PendingReport != nil || len(acknowledged.PendingPayload) != 0 || acknowledged.NextSequence != 2 {
		t.Fatalf("acknowledged health state = (%+v, %v)", acknowledged, err)
	}
}

func TestSchemaTwoRequiresApprovedDevelopmentReset(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	options := Options{DatabasePath: filepath.Join(root, "edge.db"), SpoolDirectory: filepath.Join(root, "spool")}
	store, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `PRAGMA user_version = 2`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, options); !errors.Is(err, ErrSchemaIncompatible) {
		t.Fatalf("schema 2 open error = %v", err)
	}
}
