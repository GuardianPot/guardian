package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openTestQueue(t *testing.T) *Queue {
	t.Helper()
	q, err := Open(filepath.Join(t.TempDir(), "edge.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })
	return q
}

func TestQueueUsesDurableWALSettings(t *testing.T) {
	q := openTestQueue(t)
	var journalMode string
	if err := q.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal mode = %q, want wal", journalMode)
	}
	var synchronous int
	if err := q.db.QueryRow("PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatalf("read synchronous mode: %v", err)
	}
	if synchronous != 2 { // SQLite FULL.
		t.Fatalf("synchronous mode = %d, want FULL (2)", synchronous)
	}
}

func TestQueueEnqueueIsIdempotentAndRejectsConflictingPayload(t *testing.T) {
	q := openTestQueue(t)
	ctx := context.Background()

	inserted, err := q.Enqueue(ctx, "event-1", []byte("first"))
	if err != nil || !inserted {
		t.Fatalf("first Enqueue() = (%t, %v), want (true, nil)", inserted, err)
	}
	inserted, err = q.Enqueue(ctx, "event-1", []byte("first"))
	if err != nil || inserted {
		t.Fatalf("duplicate Enqueue() = (%t, %v), want (false, nil)", inserted, err)
	}
	_, err = q.Enqueue(ctx, "event-1", []byte("different"))
	if !errors.Is(err, ErrEventConflict) {
		t.Fatalf("conflicting Enqueue() error = %v, want ErrEventConflict", err)
	}
}

func TestQueueCrashRecoveryReplaysOnceAndPreventsDuplicateDelivery(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "edge.db")
	q, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.Enqueue(ctx, "crash-event", []byte("durable payload")); err != nil {
		t.Fatal(err)
	}
	first, ok, err := q.Claim(ctx, time.Now(), 50*time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("first Claim() = (%+v, %t, %v), want an event", first, ok, err)
	}
	if first.Attempts != 1 {
		t.Fatalf("first attempt = %d, want 1", first.Attempts)
	}
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(75 * time.Millisecond)
	q, err = Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	replayed, ok, err := q.Claim(ctx, time.Now(), time.Second)
	if err != nil || !ok {
		t.Fatalf("replay Claim() = (%+v, %t, %v), want the expired event", replayed, ok, err)
	}
	if replayed.ID != first.ID || replayed.Attempts != 2 {
		t.Fatalf("replayed event = %+v, want same ID with attempt 2", replayed)
	}
	if err := q.Ack(ctx, replayed.ID, replayed.Attempts, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := q.Ack(ctx, replayed.ID, replayed.Attempts, time.Now()); err != nil {
		t.Fatalf("idempotent Ack() error = %v", err)
	}
	inserted, err := q.Enqueue(ctx, replayed.ID, replayed.Payload)
	if err != nil || inserted {
		t.Fatalf("post-delivery Enqueue() = (%t, %v), want (false, nil)", inserted, err)
	}
	if _, ok, err := q.Claim(ctx, time.Now(), time.Second); err != nil || ok {
		t.Fatalf("duplicate Claim() = (ok=%t, err=%v), want no event", ok, err)
	}
	stats, err := q.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats != (Stats{Delivered: 1}) {
		t.Fatalf("Stats() = %+v, want one delivered event", stats)
	}
}

func TestQueueFencesLateAckAndRetryFromAnExpiredClaim(t *testing.T) {
	q := openTestQueue(t)
	ctx := context.Background()
	if _, err := q.Enqueue(ctx, "fenced-event", []byte("payload")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Second)
	first, ok, err := q.Claim(ctx, now, time.Nanosecond)
	if err != nil || !ok {
		t.Fatalf("first Claim() = %+v, %t, %v", first, ok, err)
	}
	second, ok, err := q.Claim(ctx, now.Add(time.Millisecond), time.Second)
	if err != nil || !ok || second.Attempts != 2 {
		t.Fatalf("reclaimed Claim() = %+v, %t, %v", second, ok, err)
	}
	if err := q.Ack(ctx, first.ID, first.Attempts, now); !errors.Is(err, ErrStaleClaim) {
		t.Fatalf("late Ack() error = %v, want ErrStaleClaim", err)
	}
	if err := q.Retry(ctx, first.ID, first.Attempts, now, 0); !errors.Is(err, ErrStaleClaim) {
		t.Fatalf("late Retry() error = %v, want ErrStaleClaim", err)
	}
	if err := q.Ack(ctx, second.ID, second.Attempts, now); err != nil {
		t.Fatal(err)
	}
}

func TestQueueRetryAndBoundedFIFOFairness(t *testing.T) {
	q := openTestQueue(t)
	ctx := context.Background()
	for _, id := range []string{"retry-1", "retry-2", "retry-3"} {
		if _, err := q.Enqueue(ctx, id, []byte(id)); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Add(time.Second)

	first, ok, err := q.Claim(ctx, now.Add(time.Millisecond), time.Second)
	if err != nil || !ok || first.ID != "retry-1" {
		t.Fatalf("first claim = %+v, %t, %v", first, ok, err)
	}
	if err := q.Retry(ctx, first.ID, first.Attempts, now, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	second, ok, err := q.Claim(ctx, now.Add(2*time.Millisecond), time.Second)
	if err != nil || !ok || second.ID != "retry-2" {
		t.Fatalf("second claim = %+v, %t, %v", second, ok, err)
	}
	if err := q.Ack(ctx, second.ID, second.Attempts, now); err != nil {
		t.Fatal(err)
	}
	third, ok, err := q.Claim(ctx, now.Add(3*time.Millisecond), time.Second)
	if err != nil || !ok || third.ID != "retry-3" {
		t.Fatalf("third claim = %+v, %t, %v", third, ok, err)
	}
	if err := q.Ack(ctx, third.ID, third.Attempts, now); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := q.Claim(ctx, now.Add(50*time.Millisecond), time.Second); err != nil || ok {
		t.Fatalf("early retry claim = (ok=%t, err=%v), want no event", ok, err)
	}
	retried, ok, err := q.Claim(ctx, now.Add(101*time.Millisecond), time.Second)
	if err != nil || !ok || retried.ID != "retry-1" || retried.Attempts != 2 {
		t.Fatalf("retry claim = %+v, %t, %v, want retry-1 attempt 2", retried, ok, err)
	}
	if err := q.Ack(ctx, retried.ID, retried.Attempts, now); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 32; i++ {
		id := "fair-" + string(rune('A'+i))
		if _, err := q.Enqueue(ctx, id, []byte(id)); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 32; i++ {
		id := "fair-" + string(rune('A'+i))
		event, ok, err := q.Claim(ctx, time.Now(), time.Second)
		if err != nil || !ok || event.ID != id {
			t.Fatalf("fairness claim %d = %+v, %t, %v; want %s", i, event, ok, err, id)
		}
		if err := q.Ack(ctx, id, event.Attempts, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := q.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats != (Stats{Delivered: 35}) {
		t.Fatalf("final Stats() = %+v, want 35 delivered events", stats)
	}
}

func TestQueueRejectsInvalidOperations(t *testing.T) {
	q := openTestQueue(t)
	ctx := context.Background()
	if _, err := q.Enqueue(ctx, "", nil); err == nil {
		t.Fatal("empty event ID unexpectedly accepted")
	}
	if _, _, err := q.Claim(ctx, time.Now(), 0); err == nil {
		t.Fatal("zero lease unexpectedly accepted")
	}
	if err := q.Retry(ctx, "missing", 1, time.Now(), 0); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing Retry() error = %v, want sql.ErrNoRows", err)
	}
	if err := q.Ack(ctx, "missing", 1, time.Now()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing Ack() error = %v, want sql.ErrNoRows", err)
	}
}
