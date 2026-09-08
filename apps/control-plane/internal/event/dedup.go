package event

import (
	"container/list"
	"sync"
)

/*
AC-EV-001: the same event_id retried must not produce duplicate evidence.

The rule lives here, next to the identifier it is about. The durable half — a
unique constraint at ingest — is `P2-W12`'s, and this is deliberately not a
substitute for it: a process-local set forgets on restart, and at-least-once
delivery outlives a process. What this gives the pipeline is the cheap rejection
in front of the expensive one, so a retry storm does not reach storage.

The distinction matters for reading the acceptance criterion honestly. This
makes a duplicate *detectable*; the database makes it *impossible*.
*/

// DefaultSeenCapacity bounds the recent-identifier window.
//
// Bounded because an unbounded set is a memory-exhaustion path reachable by
// anyone who can make a decoy emit events, which is the whole point of a decoy.
// Evicting the oldest is the right failure: a duplicate arriving after 64k
// distinct events is a retry nobody is waiting on, and the durable constraint
// catches it anyway.
const DefaultSeenCapacity = 65536

// Deduplicator remembers recent event identifiers.
type Deduplicator struct {
	capacity int

	mu    sync.Mutex
	seen  map[string]*list.Element
	order *list.List
}

func NewDeduplicator(capacity int) *Deduplicator {
	if capacity <= 0 {
		capacity = DefaultSeenCapacity
	}
	return &Deduplicator{
		capacity: capacity,
		seen:     make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

// Accept records an identifier and reports whether it is new.
//
// A malformed identifier is not accepted and not remembered: remembering one
// would let a caller poison the window with values no valid event can carry.
func (d *Deduplicator) Accept(eventID string) bool {
	if !ValidUUIDv7(eventID) {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, duplicate := d.seen[eventID]; duplicate {
		return false
	}
	d.seen[eventID] = d.order.PushFront(eventID)
	if d.order.Len() > d.capacity {
		oldest := d.order.Back()
		if oldest != nil {
			d.order.Remove(oldest)
			if key, ok := oldest.Value.(string); ok {
				delete(d.seen, key)
			}
		}
	}
	return true
}

// Len reports how many identifiers are remembered, for the bound's test.
func (d *Deduplicator) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.order.Len()
}
