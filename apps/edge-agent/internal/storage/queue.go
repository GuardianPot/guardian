package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	statePending   = "pending"
	stateInflight  = "inflight"
	stateDelivered = "delivered"
)

// Event is a durable event returned by Claim.
type Event struct {
	ID       string `json:"id"`
	Payload  []byte `json:"-"`
	Attempts int    `json:"attempts"`
}

// Stats is a point-in-time count of queue state.
type Stats struct {
	Pending   int `json:"pending"`
	Inflight  int `json:"inflight"`
	Delivered int `json:"delivered"`
}

// Enqueue stores bounded payload content in the filesystem CAS and only its
// digest/metadata in SQLite. Reusing an ID for the same content is idempotent.
func (s *Store) Enqueue(ctx context.Context, eventID string, payload []byte) (bool, error) {
	if !validCode(eventID) {
		return false, errors.New("event ID must be a bounded identifier")
	}
	if len(payload) > maxEventPayloadBytes {
		return false, fmt.Errorf("%w: maximum is %d bytes", ErrPayloadTooLarge, maxEventPayloadBytes)
	}
	if err := s.beforeMutation(); err != nil {
		return false, err
	}
	digest, relativePath, err := s.putPayload(payload)
	if err != nil {
		return false, err
	}
	now := s.now().UnixNano()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, classifyError("begin edge enqueue transaction", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO spool_objects (digest, relative_path, size_bytes, state, created_at)
VALUES (?, ?, ?, 'available', ?)
ON CONFLICT(digest) DO NOTHING`, digest, relativePath, len(payload), now); err != nil {
		return false, classifyError("record edge spool object", err)
	}
	result, err := tx.ExecContext(ctx, `
INSERT INTO durable_events (
    event_id, payload_digest, payload_size, state, attempts, available_at, created_at, updated_at
) VALUES (?, ?, ?, ?, 0, ?, ?, ?)
ON CONFLICT(event_id) DO NOTHING`, eventID, digest, len(payload), statePending, now, now, now)
	if err != nil {
		return false, classifyError("enqueue edge event", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return false, classifyError("inspect edge enqueue", err)
	}
	if inserted == 0 {
		var storedDigest string
		var storedSize int
		if err := tx.QueryRowContext(ctx,
			"SELECT payload_digest, payload_size FROM durable_events WHERE event_id = ?", eventID).Scan(&storedDigest, &storedSize); err != nil {
			return false, classifyError("read existing edge event", err)
		}
		if storedDigest != digest || storedSize != len(payload) {
			return false, fmt.Errorf("%w: %s", ErrEventConflict, eventID)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, classifyError("commit edge enqueue", err)
	}
	return inserted == 1, nil
}

// Claim leases the oldest available event. Expired leases become eligible for
// replay, preserving restart safety without duplicate acknowledgements.
func (s *Store) Claim(ctx context.Context, now time.Time, lease time.Duration) (Event, bool, error) {
	if lease <= 0 {
		return Event{}, false, errors.New("lease must be positive")
	}
	if err := s.beforeMutation(); err != nil {
		return Event{}, false, err
	}
	nowNanos := now.UnixNano()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, false, classifyError("begin edge claim transaction", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE durable_events
SET state = ?, lease_until = NULL, available_at = ?, updated_at = ?
WHERE state = ? AND lease_until IS NOT NULL AND lease_until <= ?`,
		statePending, nowNanos, nowNanos, stateInflight, nowNanos); err != nil {
		return Event{}, false, classifyError("requeue expired edge event leases", err)
	}

	var sequence int64
	var eventID, digest string
	var payloadSize, attempts int
	err = tx.QueryRowContext(ctx, `
SELECT sequence, event_id, payload_digest, payload_size, attempts
FROM durable_events
WHERE state = ? AND available_at <= ?
ORDER BY sequence
LIMIT 1`, statePending, nowNanos).Scan(&sequence, &eventID, &digest, &payloadSize, &attempts)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return Event{}, false, classifyError("commit empty edge claim", err)
		}
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, classifyError("select edge event for claim", err)
	}
	payload, err := s.readPayload(digest, payloadSize)
	if err != nil {
		return Event{}, false, err
	}
	newAttempts := attempts + 1
	result, err := tx.ExecContext(ctx, `
UPDATE durable_events
SET state = ?, attempts = ?, lease_until = ?, updated_at = ?
WHERE sequence = ? AND state = ?`,
		stateInflight, newAttempts, now.Add(lease).UnixNano(), nowNanos, sequence, statePending)
	if err != nil {
		return Event{}, false, classifyError("lease edge event", err)
	}
	if err := requireRowsAffected("lease edge event", result); err != nil {
		return Event{}, false, fmt.Errorf("%w: %s", ErrNoEvent, eventID)
	}
	if err := tx.Commit(); err != nil {
		return Event{}, false, classifyError("commit edge event claim", err)
	}
	return Event{ID: eventID, Payload: payload, Attempts: newAttempts}, true, nil
}

// Ack marks an in-flight event delivered. The attempt fences late workers and
// repeated acknowledgement after delivery is idempotent.
func (s *Store) Ack(ctx context.Context, eventID string, attempt int, now time.Time) error {
	if attempt <= 0 {
		return errors.New("claim attempt must be positive")
	}
	if err := s.beforeMutation(); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE durable_events
SET state = ?, lease_until = NULL, delivered_at = ?, updated_at = ?
WHERE event_id = ? AND state = ? AND attempts = ?`,
		stateDelivered, now.UnixNano(), now.UnixNano(), eventID, stateInflight, attempt)
	if err != nil {
		return classifyError("ack edge event", err)
	}
	if count, err := result.RowsAffected(); err != nil {
		return classifyError("inspect edge event acknowledgement", err)
	} else if count == 1 {
		return nil
	}
	return s.classifyClaimState(ctx, eventID, attempt, true)
}

// Retry returns an in-flight event to the queue after a bounded backoff.
func (s *Store) Retry(ctx context.Context, eventID string, attempt int, now time.Time, backoff time.Duration) error {
	if attempt <= 0 {
		return errors.New("claim attempt must be positive")
	}
	if backoff < 0 {
		return errors.New("backoff must not be negative")
	}
	if err := s.beforeMutation(); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE durable_events
SET state = ?, lease_until = NULL, available_at = ?, updated_at = ?
WHERE event_id = ? AND state = ? AND attempts = ?`,
		statePending, now.Add(backoff).UnixNano(), now.UnixNano(), eventID, stateInflight, attempt)
	if err != nil {
		return classifyError("retry edge event", err)
	}
	if count, err := result.RowsAffected(); err != nil {
		return classifyError("inspect edge event retry", err)
	} else if count == 1 {
		return nil
	}
	return s.classifyClaimState(ctx, eventID, attempt, false)
}

// Stats reports queue counts without changing delivery state.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT state, COUNT(*) FROM durable_events GROUP BY state")
	if err != nil {
		return Stats{}, classifyError("read edge queue statistics", err)
	}
	defer rows.Close()
	var stats Stats
	for rows.Next() {
		var state string
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			return Stats{}, classifyError("scan edge queue statistics", err)
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
	return stats, classifyError("iterate edge queue statistics", rows.Err())
}

func (s *Store) classifyClaimState(ctx context.Context, eventID string, attempt int, deliveredIsSuccess bool) error {
	var state string
	var currentAttempt int
	if err := s.db.QueryRowContext(ctx,
		"SELECT state, attempts FROM durable_events WHERE event_id = ?", eventID).Scan(&state, &currentAttempt); err != nil {
		return classifyError("read edge event claim state", err)
	}
	if deliveredIsSuccess && state == stateDelivered {
		return nil
	}
	if state == stateInflight && currentAttempt != attempt {
		return fmt.Errorf("%w: %s attempt %d", ErrStaleClaim, eventID, attempt)
	}
	return fmt.Errorf("%w: %s", ErrNotInflight, eventID)
}

func (s *Store) putPayload(payload []byte) (string, string, error) {
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	relativePath := filepath.Join("objects", "sha256", digest[:2], digest)
	directory := filepath.Join(s.options.SpoolDirectory, filepath.Dir(relativePath))
	if err := ensureDirectory(directory, 0o700); err != nil {
		return "", "", err
	}
	target := filepath.Join(s.options.SpoolDirectory, relativePath)
	if _, err := os.Lstat(target); err == nil {
		if _, readErr := s.readPayload(digest, len(payload)); readErr != nil {
			return "", "", readErr
		}
		return digest, relativePath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", classifyError("inspect edge spool object", err)
	}

	temporary, err := os.CreateTemp(directory, ".incoming-*")
	if err != nil {
		return "", "", classifyError("create edge spool object", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return "", "", classifyError("protect edge spool object", err)
	}
	if _, err := temporary.Write(payload); err != nil {
		temporary.Close()
		return "", "", classifyError("write edge spool object", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return "", "", classifyError("sync edge spool object", err)
	}
	if err := temporary.Close(); err != nil {
		return "", "", classifyError("close edge spool object", err)
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return "", "", classifyError("publish edge spool object", err)
	}
	removeTemporary = false
	if err := os.Chmod(target, 0o600); err != nil {
		return "", "", classifyError("protect published edge spool object", err)
	}
	if err := syncDirectory(directory); err != nil {
		return "", "", err
	}
	return digest, relativePath, nil
}

func (s *Store) readPayload(digest string, expectedSize int) ([]byte, error) {
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != sha256.Size || expectedSize < 0 || expectedSize > maxEventPayloadBytes {
		return nil, fmt.Errorf("%w: invalid payload metadata", ErrSpoolObjectMissing)
	}
	relativePath := filepath.Join("objects", "sha256", digest[:2], digest)
	path := filepath.Join(s.options.SpoolDirectory, relativePath)
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() != int64(expectedSize) {
		return nil, fmt.Errorf("%w: digest %s", ErrSpoolObjectMissing, digest)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, classifyError("open edge spool object", err)
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, int64(maxEventPayloadBytes+1)))
	if err != nil {
		return nil, classifyError("read edge spool object", err)
	}
	if len(payload) != expectedSize || len(payload) > maxEventPayloadBytes {
		return nil, fmt.Errorf("%w: digest %s size mismatch", ErrSpoolObjectMissing, digest)
	}
	actual := sha256.Sum256(payload)
	if !bytes.Equal(actual[:], decoded) {
		return nil, fmt.Errorf("%w: digest %s content mismatch", ErrSpoolObjectMissing, digest)
	}
	return payload, nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return classifyError("open edge spool directory", err)
	}
	defer directory.Close()
	return classifyError("sync edge spool directory", directory.Sync())
}
