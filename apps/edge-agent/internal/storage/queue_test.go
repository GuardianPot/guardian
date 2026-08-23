package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) (*Store, Options) {
	t.Helper()
	root := t.TempDir()
	options := Options{
		DatabasePath:   filepath.Join(root, "state", "edge.db"),
		SpoolDirectory: filepath.Join(root, "spool"),
		BusyTimeout:    50 * time.Millisecond,
	}
	store, err := Open(context.Background(), options)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, options
}

func TestStoreUsesDurableWALAndMetadataOnlyQueue(t *testing.T) {
	store, _ := openTestStore(t)
	var journalMode string
	if err := store.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal mode = %q, want wal", journalMode)
	}
	var synchronous int
	if err := store.db.QueryRow("PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatal(err)
	}
	if synchronous != 2 {
		t.Fatalf("synchronous mode = %d, want FULL", synchronous)
	}
	rows, err := store.db.Query("PRAGMA table_info(durable_events)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var index, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&index, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == "payload" {
			t.Fatal("large payload column unexpectedly exists in SQLite")
		}
	}
}

func TestQueueEnqueueIsIdempotentAndUsesProtectedCAS(t *testing.T) {
	store, options := openTestStore(t)
	ctx := context.Background()
	payload := []byte("durable filesystem payload")
	inserted, err := store.Enqueue(ctx, "event-1", payload)
	if err != nil || !inserted {
		t.Fatalf("first Enqueue() = (%t, %v)", inserted, err)
	}
	inserted, err = store.Enqueue(ctx, "event-1", payload)
	if err != nil || inserted {
		t.Fatalf("duplicate Enqueue() = (%t, %v)", inserted, err)
	}
	if _, err := store.Enqueue(ctx, "event-1", []byte("different")); !errors.Is(err, ErrEventConflict) {
		t.Fatalf("conflicting Enqueue() error = %v", err)
	}

	digestBytes := sha256.Sum256(payload)
	digest := hex.EncodeToString(digestBytes[:])
	objectPath := filepath.Join(options.SpoolDirectory, "objects", "sha256", digest[:2], digest)
	stored, err := os.ReadFile(objectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, payload) {
		t.Fatalf("CAS payload = %q", stored)
	}
	info, err := os.Stat(objectPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("CAS permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestQueueCrashRecoveryReplaysAndFencesDuplicates(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	options := Options{DatabasePath: filepath.Join(root, "edge.db"), SpoolDirectory: filepath.Join(root, "spool")}
	store, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Enqueue(ctx, "crash-event", []byte("durable payload")); err != nil {
		t.Fatal(err)
	}
	first, ok, err := store.Claim(ctx, time.Now(), 25*time.Millisecond)
	if err != nil || !ok || first.Attempts != 1 {
		t.Fatalf("first Claim() = (%+v, %t, %v)", first, ok, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	store, err = Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	replayed, ok, err := store.Claim(ctx, time.Now(), time.Second)
	if err != nil || !ok || replayed.ID != first.ID || replayed.Attempts != 2 {
		t.Fatalf("replayed Claim() = (%+v, %t, %v)", replayed, ok, err)
	}
	if err := store.Ack(ctx, first.ID, first.Attempts, time.Now()); !errors.Is(err, ErrStaleClaim) {
		t.Fatalf("late Ack() error = %v", err)
	}
	if err := store.Retry(ctx, first.ID, first.Attempts, time.Now(), 0); !errors.Is(err, ErrStaleClaim) {
		t.Fatalf("late Retry() error = %v", err)
	}
	if err := store.Ack(ctx, replayed.ID, replayed.Attempts, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.Ack(ctx, replayed.ID, replayed.Attempts, time.Now()); err != nil {
		t.Fatalf("idempotent Ack() error = %v", err)
	}
	if _, ok, err := store.Claim(ctx, time.Now(), time.Second); err != nil || ok {
		t.Fatalf("duplicate Claim() = (ok=%t, err=%v)", ok, err)
	}
}

func TestQueueRetryFairnessAndPayloadLimit(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	for _, id := range []string{"retry-1", "retry-2", "retry-3"} {
		if _, err := store.Enqueue(ctx, id, []byte(id)); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Add(time.Second)
	first, _, err := store.Claim(ctx, now, time.Second)
	if err != nil || first.ID != "retry-1" {
		t.Fatalf("first claim = %+v, %v", first, err)
	}
	if err := store.Retry(ctx, first.ID, first.Attempts, now, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"retry-2", "retry-3"} {
		event, ok, err := store.Claim(ctx, now.Add(time.Millisecond), time.Second)
		if err != nil || !ok || event.ID != expected {
			t.Fatalf("fair claim = (%+v, %t, %v), want %s", event, ok, err, expected)
		}
		if err := store.Ack(ctx, event.ID, event.Attempts, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok, err := store.Claim(ctx, now.Add(50*time.Millisecond), time.Second); err != nil || ok {
		t.Fatalf("early retry claim = (ok=%t, err=%v)", ok, err)
	}
	retried, ok, err := store.Claim(ctx, now.Add(101*time.Millisecond), time.Second)
	if err != nil || !ok || retried.ID != first.ID || retried.Attempts != 2 {
		t.Fatalf("retry claim = (%+v, %t, %v)", retried, ok, err)
	}
	if _, err := store.Enqueue(ctx, "oversized", make([]byte, maxEventPayloadBytes+1)); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("oversized Enqueue() error = %v", err)
	}
}

func TestQueueRejectsMissingOrTamperedSpoolObjectWithoutLeasing(t *testing.T) {
	store, options := openTestStore(t)
	ctx := context.Background()
	payload := []byte("protected")
	if _, err := store.Enqueue(ctx, "tampered-event", payload); err != nil {
		t.Fatal(err)
	}
	digestBytes := sha256.Sum256(payload)
	digest := hex.EncodeToString(digestBytes[:])
	objectPath := filepath.Join(options.SpoolDirectory, "objects", "sha256", digest[:2], digest)
	if err := os.WriteFile(objectPath, []byte("corrupted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Claim(ctx, time.Now(), time.Second); !errors.Is(err, ErrSpoolObjectMissing) {
		t.Fatalf("tampered spool Claim() error = %v", err)
	}
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Pending != 1 || stats.Inflight != 0 {
		t.Fatalf("queue mutated after spool failure: %+v", stats)
	}
}
