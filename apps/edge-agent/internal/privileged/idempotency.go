package privileged

import (
	"context"
	"errors"
	"sync"
	"time"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
)

var ErrIdempotencyConflict = errors.New("request-id-reused-with-different-operation")

type cachedResult struct {
	fingerprint string
	result      AdapterResult
	createdAt   time.Time
}

type pendingResult struct {
	fingerprint string
	done        chan struct{}
	result      AdapterResult
	err         error
}

type idempotencyRegistry struct {
	mu      sync.Mutex
	entries map[string]cachedResult
	pending map[string]*pendingResult
	order   []string
	limit   int
	ttl     time.Duration
	now     func() time.Time
}

func newIdempotencyRegistry(limit int, ttl time.Duration) *idempotencyRegistry {
	return &idempotencyRegistry{
		entries: make(map[string]cachedResult),
		pending: make(map[string]*pendingResult),
		limit:   limit,
		ttl:     ttl,
		now:     time.Now,
	}
}

func (r *idempotencyRegistry) do(
	ctx context.Context,
	requestID string,
	fingerprint string,
	operation func(context.Context) (AdapterResult, error),
) (AdapterResult, error) {
	for {
		r.mu.Lock()
		r.expireLocked()
		if cached, ok := r.entries[requestID]; ok {
			r.mu.Unlock()
			if cached.fingerprint != fingerprint {
				return AdapterResult{}, ErrIdempotencyConflict
			}
			return cached.result, nil
		}
		if pending, ok := r.pending[requestID]; ok {
			if pending.fingerprint != fingerprint {
				r.mu.Unlock()
				return AdapterResult{}, ErrIdempotencyConflict
			}
			r.mu.Unlock()
			select {
			case <-ctx.Done():
				return AdapterResult{}, ctx.Err()
			case <-pending.done:
				return pending.result, pending.err
			}
		}

		pending := &pendingResult{fingerprint: fingerprint, done: make(chan struct{})}
		r.pending[requestID] = pending
		r.mu.Unlock()

		result, err := operation(ctx)
		if err == nil && result.Outcome == privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNSPECIFIED {
			err = errors.New("adapter-returned-unspecified-outcome")
		}

		r.mu.Lock()
		pending.result = result
		pending.err = err
		delete(r.pending, requestID)
		if err == nil {
			r.entries[requestID] = cachedResult{
				fingerprint: fingerprint,
				result:      result,
				createdAt:   r.now(),
			}
			r.order = append(r.order, requestID)
			r.trimLocked()
		}
		close(pending.done)
		r.mu.Unlock()
		return result, err
	}
}

func (r *idempotencyRegistry) expireLocked() {
	cutoff := r.now().Add(-r.ttl)
	kept := r.order[:0]
	for _, requestID := range r.order {
		entry, ok := r.entries[requestID]
		if !ok {
			continue
		}
		if entry.createdAt.Before(cutoff) {
			delete(r.entries, requestID)
			continue
		}
		kept = append(kept, requestID)
	}
	r.order = kept
}

func (r *idempotencyRegistry) trimLocked() {
	for len(r.entries) > r.limit && len(r.order) != 0 {
		requestID := r.order[0]
		r.order = r.order[1:]
		delete(r.entries, requestID)
	}
}
