package normalize

import (
	"bufio"
	"container/list"
	"io"
	"sync"
	"time"
)

/*
The rate and burst limiter, the deduplicator, and the bounded line reader.

These sit in front of the pipeline rather than inside an adapter because all
three are properties of "how much a decoy may say", and a decoy is a thing an
attacker can make noisy on purpose. Leaving them to each adapter would mean
every adapter getting them right, and the first one that did not would be the one
under attack.
*/

// TokenBucket bounds the sustained rate and the burst.
//
// Two numbers rather than one, because the shapes are different: a burst is a
// real attacker doing a lot at once and should get through, and a sustained
// flood is either a broken pack or someone trying to fill the spool. The bucket
// lets the first past and holds the second.
type TokenBucket struct {
	mu       sync.Mutex
	capacity float64
	tokens   float64
	perSec   float64
	last     time.Time
}

func NewTokenBucket(perSecond float64, burst int) *TokenBucket {
	if perSecond <= 0 {
		perSecond = 1
	}
	if burst <= 0 {
		burst = 1
	}
	return &TokenBucket{capacity: float64(burst), tokens: float64(burst), perSec: perSecond}
}

// Allow reports whether one event may pass now.
func (b *TokenBucket) Allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.last.IsZero() {
		b.last = now
	}
	// A clock that went backwards refills nothing rather than draining the
	// bucket: an Edge whose clock stepped must not stop reporting.
	if elapsed := now.Sub(b.last); elapsed > 0 {
		b.tokens += elapsed.Seconds() * b.perSec
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RecentIDs rejects an identifier it has already seen, within a bounded window.
//
// The Edge's half of AC-EV-001. The durable constraint is the Control Plane's;
// this stops a retry from being spooled and sent again in the first place.
type RecentIDs struct {
	capacity int

	mu    sync.Mutex
	seen  map[string]*list.Element
	order *list.List
}

func NewRecentIDs(capacity int) *RecentIDs {
	if capacity <= 0 {
		capacity = 8192
	}
	return &RecentIDs{
		capacity: capacity,
		seen:     make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

// Accept records an identifier and reports whether it is new.
func (r *RecentIDs) Accept(eventID string) bool {
	if !ValidUUIDv7(eventID) {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, seen := r.seen[eventID]; seen {
		return false
	}
	r.seen[eventID] = r.order.PushFront(eventID)
	if r.order.Len() > r.capacity {
		if oldest := r.order.Back(); oldest != nil {
			r.order.Remove(oldest)
			if key, ok := oldest.Value.(string); ok {
				delete(r.seen, key)
			}
		}
	}
	return true
}

/*
ReadLines feeds a log file's lines through the pipeline.

This is the third-party adapter path. The reader is bounded at the same limit
as an event, so a single enormous line — a log file an attacker has arranged to
contain one — cannot be read into memory. `bufio.Scanner` is given an explicit
buffer for that reason: its default is 64 KiB and its failure mode on a longer
line is an error rather than a partial read, which is what makes it safe here.

Lines are handed over one at a time and the batch is bounded, so one call
cannot occupy the pipeline indefinitely.
*/
func (p *Pipeline) ReadLines(adapterName string, reader io.Reader) []Outcome {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 4096), p.limits.MaxEventBytes)

	outcomes := make([]Outcome, 0, 16)
	for len(outcomes) < p.limits.MaxBatchEvents && scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		outcomes = append(outcomes, p.Accept(adapterName, line, SourceLogAdapter))
	}
	if err := scanner.Err(); err != nil {
		// A line past the bound. The offending bytes are not available — the
		// scanner refused to assemble them, which is the point — so the drop is
		// recorded without quarantining material nobody has.
		outcomes = append(outcomes, p.drop(adapterName, SourceLogAdapter, DropOversized, nil, p.now()))
	}
	return outcomes
}
