// Package storage contains the Edge agent's durable local state.
package storage

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

var (
	// ErrEventConflict means that an event ID was reused with a different payload.
	ErrEventConflict = errors.New("event ID already exists with a different payload")
	// ErrNoEvent means that no event is currently available for delivery.
	ErrNoEvent = errors.New("no event available")
	// ErrNotInflight means that an operation requires an in-flight event.
	ErrNotInflight = errors.New("event is not in flight")
	// ErrStaleClaim means that a worker tried to finish an older lease.
	ErrStaleClaim = errors.New("event claim is stale")
)

const (
	statePending   = "pending"
	stateInflight  = "inflight"
	stateDelivered = "delivered"
)

// Event is a durable event returned by Claim.
type Event struct {
	ID       string
	Payload  []byte
	Attempts int
}

// Stats is a point-in-time count of queue state, useful for operational evidence.
type Stats struct {
	Pending   int
	Inflight  int
	Delivered int
}

// Queue is a SQLite-backed durable event queue.
type Queue struct {
	db *sql.DB
}

// Open opens or creates a queue database and applies its local durability settings.
func Open(path string) (*Queue, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	// A single connection serializes short queue transactions and avoids lock
	// contention while preserving SQLite's WAL durability semantics.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := configure(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Queue{db: db}, nil
}

func configure(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = FULL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA wal_autocheckpoint = 1000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("configure sqlite (%s): %w", pragma, err)
		}
	}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS durable_events (
		sequence INTEGER PRIMARY KEY AUTOINCREMENT,
		event_id TEXT NOT NULL UNIQUE,
		payload BLOB NOT NULL,
		state TEXT NOT NULL CHECK (state IN ('pending', 'inflight', 'delivered')),
		attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
		available_at INTEGER NOT NULL,
		lease_until INTEGER,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		delivered_at INTEGER
);
CREATE INDEX IF NOT EXISTS durable_events_available_idx
	ON durable_events (state, available_at, sequence);
`); err != nil {
		return fmt.Errorf("create durable event schema: %w", err)
	}
	return nil
}

// Close releases the SQLite handle. The database file remains durable on disk.
func (q *Queue) Close() error {
	if q == nil || q.db == nil {
		return nil
	}
	return q.db.Close()
}

// Enqueue adds an event exactly once. Re-enqueuing the same ID and payload is
// an idempotent no-op; reusing an ID with another payload is rejected.
func (q *Queue) Enqueue(ctx context.Context, eventID string, payload []byte) (bool, error) {
	if eventID == "" {
		return false, errors.New("event ID must not be empty")
	}
	now := time.Now().UnixNano()
	result, err := q.db.ExecContext(ctx, `
INSERT INTO durable_events (event_id, payload, state, attempts, available_at, created_at, updated_at)
VALUES (?, ?, ?, 0, ?, ?, ?)
ON CONFLICT(event_id) DO NOTHING`, eventID, payload, statePending, now, now, now)
	if err != nil {
		return false, fmt.Errorf("enqueue event %q: %w", eventID, err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("inspect enqueue event %q: %w", eventID, err)
	}
	if inserted == 1 {
		return true, nil
	}

	var stored []byte
	if err := q.db.QueryRowContext(ctx,
		"SELECT payload FROM durable_events WHERE event_id = ?", eventID).Scan(&stored); err != nil {
		return false, fmt.Errorf("read existing event %q: %w", eventID, err)
	}
	if !bytes.Equal(stored, payload) {
		return false, fmt.Errorf("%w: %s", ErrEventConflict, eventID)
	}
	return false, nil
}

// Claim leases the oldest available event. Expired leases become eligible for
// replay, which is the crash-recovery path after a process dies before Ack.
func (q *Queue) Claim(ctx context.Context, now time.Time, lease time.Duration) (Event, bool, error) {
	if lease <= 0 {
		return Event{}, false, errors.New("lease must be positive")
	}
	nowNanos := now.UnixNano()
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, false, fmt.Errorf("begin claim transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE durable_events
SET state = ?, lease_until = NULL, available_at = ?, updated_at = ?
WHERE state = ? AND lease_until IS NOT NULL AND lease_until <= ?`,
		statePending, nowNanos, nowNanos, stateInflight, nowNanos); err != nil {
		return Event{}, false, fmt.Errorf("requeue expired leases: %w", err)
	}

	var sequence int64
	var eventID string
	var payload []byte
	var attempts int
	err = tx.QueryRowContext(ctx, `
SELECT sequence, event_id, payload, attempts
FROM durable_events
WHERE state = ? AND available_at <= ?
ORDER BY sequence
LIMIT 1`, statePending, nowNanos).Scan(&sequence, &eventID, &payload, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return Event{}, false, fmt.Errorf("commit empty claim: %w", err)
		}
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, fmt.Errorf("select event for claim: %w", err)
	}
	newAttempts := attempts + 1
	leaseUntil := now.Add(lease).UnixNano()
	result, err := tx.ExecContext(ctx, `
UPDATE durable_events
SET state = ?, attempts = ?, lease_until = ?, updated_at = ?
WHERE sequence = ? AND state = ?`,
		stateInflight, newAttempts, leaseUntil, nowNanos, sequence, statePending)
	if err != nil {
		return Event{}, false, fmt.Errorf("lease event %q: %w", eventID, err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return Event{}, false, fmt.Errorf("inspect lease event %q: %w", eventID, err)
		}
		return Event{}, false, fmt.Errorf("%w: %s", ErrNoEvent, eventID)
	}
	if err := tx.Commit(); err != nil {
		return Event{}, false, fmt.Errorf("commit claim for %q: %w", eventID, err)
	}
	return Event{ID: eventID, Payload: append([]byte(nil), payload...), Attempts: newAttempts}, true, nil
}

// Ack marks an in-flight event delivered. The attempt fences late workers from
// acknowledging a newer lease. Ack is idempotent after delivery.
func (q *Queue) Ack(ctx context.Context, eventID string, attempt int, now time.Time) error {
	if attempt <= 0 {
		return errors.New("claim attempt must be positive")
	}
	result, err := q.db.ExecContext(ctx, `
UPDATE durable_events
SET state = ?, lease_until = NULL, delivered_at = ?, updated_at = ?
WHERE event_id = ? AND state = ? AND attempts = ?`, stateDelivered, now.UnixNano(), now.UnixNano(), eventID, stateInflight, attempt)
	if err != nil {
		return fmt.Errorf("ack event %q: %w", eventID, err)
	}
	if count, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("inspect ack event %q: %w", eventID, err)
	} else if count == 1 {
		return nil
	}

	var state string
	var currentAttempt int
	if err := q.db.QueryRowContext(ctx,
		"SELECT state, attempts FROM durable_events WHERE event_id = ?", eventID).Scan(&state, &currentAttempt); err != nil {
		return fmt.Errorf("read ack event %q: %w", eventID, err)
	}
	if state == stateDelivered {
		return nil
	}
	if state == stateInflight && currentAttempt != attempt {
		return fmt.Errorf("%w: %s attempt %d", ErrStaleClaim, eventID, attempt)
	}
	return fmt.Errorf("%w: %s", ErrNotInflight, eventID)
}

// Retry returns an in-flight event to the queue after a failed delivery. The
// attempt fences late workers from retrying a newer lease.
func (q *Queue) Retry(ctx context.Context, eventID string, attempt int, now time.Time, backoff time.Duration) error {
	if attempt <= 0 {
		return errors.New("claim attempt must be positive")
	}
	if backoff < 0 {
		return errors.New("backoff must not be negative")
	}
	nowNanos := now.UnixNano()
	result, err := q.db.ExecContext(ctx, `
UPDATE durable_events
SET state = ?, lease_until = NULL, available_at = ?, updated_at = ?
WHERE event_id = ? AND state = ? AND attempts = ?`, statePending, now.Add(backoff).UnixNano(), nowNanos, eventID, stateInflight, attempt)
	if err != nil {
		return fmt.Errorf("retry event %q: %w", eventID, err)
	}
	if count, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("inspect retry event %q: %w", eventID, err)
	} else if count == 1 {
		return nil
	}
	var state string
	var currentAttempt int
	if err := q.db.QueryRowContext(ctx,
		"SELECT state, attempts FROM durable_events WHERE event_id = ?", eventID).Scan(&state, &currentAttempt); err != nil {
		return fmt.Errorf("read retry event %q: %w", eventID, err)
	}
	if state == stateInflight && currentAttempt != attempt {
		return fmt.Errorf("%w: %s attempt %d", ErrStaleClaim, eventID, attempt)
	}
	return fmt.Errorf("%w: %s", ErrNotInflight, eventID)
}

// Stats reports current queue counts without changing delivery state.
func (q *Queue) Stats(ctx context.Context) (Stats, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT state, COUNT(*) FROM durable_events GROUP BY state`)
	if err != nil {
		return Stats{}, fmt.Errorf("read queue stats: %w", err)
	}
	defer rows.Close()
	var stats Stats
	for rows.Next() {
		var state string
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			return Stats{}, fmt.Errorf("scan queue stats: %w", err)
		}
		switch state {
		case statePending:
			stats.Pending = count
		case stateInflight:
			stats.Inflight = count
		case stateDelivered:
			stats.Delivered = count
		}
	}
	if err := rows.Err(); err != nil {
		return Stats{}, fmt.Errorf("iterate queue stats: %w", err)
	}
	return stats, nil
}
