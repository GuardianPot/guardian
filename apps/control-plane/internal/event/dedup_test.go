package event

import (
	"fmt"
	"sync"
	"testing"
)

func identifier(n int) string {
	return fmt.Sprintf("0198dc8c-c600-7000-8000-%012d", n)
}

// AC-EV-001: the same identifier retried is a duplicate, not a second event.
func TestARetriedEventIdentifierIsRejected(t *testing.T) {
	dedup := NewDeduplicator(8)

	if !dedup.Accept(identifier(1)) {
		t.Fatal("the first delivery was rejected")
	}
	for attempt := 0; attempt < 5; attempt++ {
		if dedup.Accept(identifier(1)) {
			t.Fatalf("retry %d was accepted as a new event", attempt)
		}
	}
	if !dedup.Accept(identifier(2)) {
		t.Fatal("a genuinely new event was rejected")
	}
}

/*
 * The window is bounded, because an unbounded one is a memory-exhaustion path
 * reachable by anyone who can make a decoy emit events — which is what a decoy
 * is for.
 */
func TestTheWindowIsBoundedAndEvictsTheOldest(t *testing.T) {
	dedup := NewDeduplicator(4)
	for n := 1; n <= 10; n++ {
		dedup.Accept(identifier(n))
	}

	if got := dedup.Len(); got != 4 {
		t.Fatalf("remembered %d identifiers, want the 4-entry bound", got)
	}
	// The most recent are still known.
	if dedup.Accept(identifier(10)) {
		t.Fatal("the newest identifier was forgotten")
	}
	// The oldest was evicted, and the durable constraint is what catches it.
	if !dedup.Accept(identifier(1)) {
		t.Fatal("an evicted identifier was still remembered; the bound is not working")
	}
}

// A malformed identifier is neither accepted nor remembered: remembering one
// would let a caller poison the window with values no valid event can carry.
func TestAMalformedIdentifierIsNeverRemembered(t *testing.T) {
	dedup := NewDeduplicator(8)
	for _, bad := range []string{"", "not-a-uuid", "0198dc8c-c600-4000-8000-000000000001"} {
		if dedup.Accept(bad) {
			t.Fatalf("%q was accepted", bad)
		}
	}
	if dedup.Len() != 0 {
		t.Fatalf("remembered %d malformed identifiers", dedup.Len())
	}
}

// Ingest is concurrent, so exactly one caller may win a given identifier.
func TestOnlyOneConcurrentCallerWinsAnIdentifier(t *testing.T) {
	dedup := NewDeduplicator(1024)
	var wait sync.WaitGroup
	accepted := make(chan bool, 64)
	for worker := 0; worker < 64; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			accepted <- dedup.Accept(identifier(7))
		}()
	}
	wait.Wait()
	close(accepted)

	wins := 0
	for won := range accepted {
		if won {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("%d callers accepted the same identifier, want exactly 1", wins)
	}
}
