package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

const (
	storageDesiredMessage  = "0198f7c4-7b30-7f11-8a44-111111111111"
	storageObservedMessage = "0198f7c4-7b30-7f11-8a44-222222222222"
)

func TestReconciliationAcceptanceIsAtomicAndRestartSafe(t *testing.T) {
	ctx := context.Background()
	options := Options{DatabasePath: filepath.Join(t.TempDir(), "edge.db"), SpoolDirectory: filepath.Join(t.TempDir(), "spool")}
	store, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	payload := []byte("deterministic desired state")
	candidate := ReconciliationCandidate{
		MessageID: storageDesiredMessage, Revision: 1, Digest: sha256.Sum256(payload), Payload: payload,
		ObservedMessageID: storageObservedMessage, Now: now,
	}
	store.beforeWrite = func() error { return errors.New("crash before commit") }
	if _, err := store.AcceptReconciliationCandidate(ctx, candidate); err == nil {
		t.Fatal("injected pre-commit failure was accepted")
	}
	store.beforeWrite = nil
	if _, err := store.ReconciliationState(ctx); !errors.Is(err, ErrReconciliationStateNotFound) {
		t.Fatalf("pre-commit failure left state: %v", err)
	}
	accepted, err := store.AcceptReconciliationCandidate(ctx, candidate)
	if err != nil || !accepted.ShouldApply || accepted.Record.ConditionStatus != "pending" {
		t.Fatalf("AcceptReconciliationCandidate() = (%+v, %v)", accepted, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	record, err := restarted.ReconciliationState(ctx)
	if err != nil || record.DesiredRevision != 1 || record.ConditionStatus != "pending" || !record.ObservedPending {
		t.Fatalf("restart state = (%+v, %v)", record, err)
	}
}

func TestSchemaVersionOneFailsClosedForExplicitRecovery(t *testing.T) {
	ctx := context.Background()
	options := Options{DatabasePath: filepath.Join(t.TempDir(), "edge.db"), SpoolDirectory: filepath.Join(t.TempDir(), "spool")}
	store, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "PRAGMA user_version = 1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, options); !errors.Is(err, ErrSchemaIncompatible) {
		t.Fatalf("version-1 open error = %v", err)
	}
}
