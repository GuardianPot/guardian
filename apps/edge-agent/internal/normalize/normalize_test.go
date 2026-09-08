package normalize

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	eventID       = "0198dc8c-c600-7000-8000-000000000001"
	environmentID = "0198dc8c-c600-7000-8000-000000000002"
	edgeID        = "0198dc8c-c600-7000-8000-000000000003"
	decoyID       = "0198dc8c-c600-7000-8000-000000000004"
)

func sampleEvent(id string) Event {
	at := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	return Event{
		Schema: Schema, EventID: id, EventTime: at, ObservedTime: at.Add(time.Second),
		EnvironmentID: environmentID, EdgeID: edgeID, DecoyID: decoyID,
		Protocol: "ssh", Action: "auth_failed", Sensitivity: "restricted",
		Provenance: Provenance{Pack: "ssh-cowrie", PackVersion: "0.1.0"},
	}
}

// nativeAdapter is the typed path: the pack emits the canonical event directly.
type nativeAdapter struct {
	name    string
	panics  bool
	fails   bool
	produce func(raw []byte) Event
}

func (a *nativeAdapter) Name() string { return a.name }

func (a *nativeAdapter) Normalize(raw []byte, _ SourcePath) (Event, error) {
	if a.panics {
		// The shape of a real parser bug: an index into attacker-shaped input.
		var empty []int
		_ = empty[len(raw)]
	}
	if a.fails {
		return Event{}, fmt.Errorf("cannot parse")
	}
	if a.produce != nil {
		return a.produce(raw), nil
	}
	var decoded Event
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return Event{}, err
	}
	return decoded, nil
}

type recordingQuarantine struct {
	mu      sync.Mutex
	records []QuarantineRecord
}

func (q *recordingQuarantine) Quarantine(record QuarantineRecord) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.records = append(q.records, record)
	return nil
}

func encode(t *testing.T, event Event) []byte {
	t.Helper()
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func newPipeline(t *testing.T, adapter Adapter, options ...Option) *Pipeline {
	t.Helper()
	pipeline, err := NewPipeline([]Adapter{adapter}, options...)
	if err != nil {
		t.Fatal(err)
	}
	return pipeline
}

func TestAWellFormedNativeEventIsAccepted(t *testing.T) {
	pipeline := newPipeline(t, &nativeAdapter{name: "cowrie-json"})

	outcome := pipeline.Accept("cowrie-json", encode(t, sampleEvent(eventID)), SourceNativeUDS)

	if outcome.Dropped || outcome.Event == nil {
		t.Fatalf("outcome = %+v, want an accepted event", outcome)
	}
	// Provenance is the pipeline's knowledge, not the parser's.
	if outcome.Event.Provenance.SourcePath != string(SourceNativeUDS) {
		t.Fatalf("source path = %q", outcome.Event.Provenance.SourcePath)
	}
	if outcome.Event.Provenance.Adapter != "cowrie-json" {
		t.Fatalf("adapter = %q", outcome.Event.Provenance.Adapter)
	}
}

/*
 * An adapter cannot overstate where an event came from.
 *
 * A parsed log line and a typed event from a pack Guardian ships are different
 * kinds of claim. An adapter that could label its output `native_uds` would be
 * inflating the confidence a reader places in it, so the pipeline overwrites
 * whatever it said.
 */
func TestAnAdapterCannotClaimAPathItDidNotComeThrough(t *testing.T) {
	adapter := &nativeAdapter{name: "http-access", produce: func([]byte) Event {
		event := sampleEvent(eventID)
		event.Provenance.SourcePath = string(SourceNativeUDS)
		event.Provenance.Adapter = "cowrie-json"
		return event
	}}
	pipeline := newPipeline(t, adapter)

	outcome := pipeline.Accept("http-access", []byte("anything"), SourceLogAdapter)

	if outcome.Dropped || outcome.Event == nil {
		t.Fatalf("outcome = %+v", outcome)
	}
	if outcome.Event.Provenance.SourcePath != string(SourceLogAdapter) {
		t.Fatalf("adapter's claimed path survived: %q", outcome.Event.Provenance.SourcePath)
	}
	if outcome.Event.Provenance.Adapter != "http-access" {
		t.Fatalf("adapter's claimed name survived: %q", outcome.Event.Provenance.Adapter)
	}
}

/*
 * AC-SEC-008: an oversized or malformed payload must not crash the Edge.
 *
 * The size check runs before the adapter, so an oversized payload costs a
 * length comparison rather than a parse.
 */
func TestOversizedPayloadIsRefusedBeforeAnythingParsesIt(t *testing.T) {
	parsed := 0
	adapter := &nativeAdapter{name: "cowrie-json", produce: func([]byte) Event {
		parsed++
		return sampleEvent(eventID)
	}}
	quarantine := &recordingQuarantine{}
	pipeline := newPipeline(t, adapter,
		WithLimits(Limits{MaxEventBytes: 128, MaxQuarantineBytes: 32}),
		WithQuarantine(quarantine),
	)

	outcome := pipeline.Accept("cowrie-json", []byte(strings.Repeat("a", 4096)), SourceNativeUDS)

	if !outcome.Dropped || outcome.Reason != DropOversized {
		t.Fatalf("outcome = %+v, want an oversized drop", outcome)
	}
	if parsed != 0 {
		t.Fatal("an oversized payload reached the adapter")
	}
	// Kept for inspection, but bounded: the material is attacker-controlled and
	// keeping all of it would let the attacker choose the disk cost.
	if len(quarantine.records) != 1 {
		t.Fatalf("quarantined %d records, want 1", len(quarantine.records))
	}
	record := quarantine.records[0]
	if len(record.Raw) != 32 || !record.Truncated {
		t.Fatalf("quarantine record = %d bytes truncated=%v, want a bounded, marked copy",
			len(record.Raw), record.Truncated)
	}
}

/*
 * A parser bug on attacker-shaped input becomes a dropped event, not a dead
 * Edge. An operator seeing `adapter_failed` is looking at a Guardian bug rather
 * than at an attack, which is why it is its own reason.
 */
func TestAPanickingAdapterDoesNotTakeTheEdgeWithIt(t *testing.T) {
	pipeline := newPipeline(t, &nativeAdapter{name: "cowrie-json", panics: true})

	outcome := pipeline.Accept("cowrie-json", []byte("{}"), SourceLogAdapter)

	if !outcome.Dropped || outcome.Reason != DropAdapterFailed {
		t.Fatalf("outcome = %+v, want an adapter_failed drop", outcome)
	}
	// And the pipeline still works afterwards.
	if got := pipeline.Drops()[DropAdapterFailed]; got != 1 {
		t.Fatalf("drop count = %d", got)
	}
}

// Hostile shapes reach the adapter and none of them may panic the process.
func TestHostileInputIsRefusedWithoutPanicking(t *testing.T) {
	quarantine := &recordingQuarantine{}
	pipeline := newPipeline(t, &nativeAdapter{name: "cowrie-json"}, WithQuarantine(quarantine))

	for name, raw := range map[string][]byte{
		"empty":               {},
		"not JSON":            []byte("\x00\x01\x02"),
		"truncated JSON":      []byte(`{"schema":`),
		"a JSON array":        []byte(`[1,2,3]`),
		"deep nesting":        []byte(strings.Repeat(`{"a":`, 500) + "null" + strings.Repeat("}", 500)),
		"wrong field type":    []byte(`{"schema":"guardian.event.v1","event_id":12345}`),
		"invalid UTF-8 bytes": {0xff, 0xfe, 0x7b, 0x7d},
	} {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("%s panicked: %v", name, recovered)
				}
			}()
			outcome := pipeline.Accept("cowrie-json", raw, SourceLogAdapter)
			if !outcome.Dropped {
				t.Fatalf("%s was accepted", name)
			}
		}()
	}
}

// An event the adapter produced but the contract refuses is dropped as invalid,
// distinguishably from one the adapter could not parse at all.
func TestAnInvalidEventIsDistinguishedFromAnUnparseableOne(t *testing.T) {
	invalid := &nativeAdapter{name: "cowrie-json", produce: func([]byte) Event {
		event := sampleEvent(eventID)
		event.Action = "exfiltrated_everything"
		return event
	}}
	if outcome := newPipeline(t, invalid).Accept("cowrie-json", []byte("{}"), SourceNativeUDS); outcome.Reason != DropInvalidEvent {
		t.Fatalf("reason = %q, want %q", outcome.Reason, DropInvalidEvent)
	}

	unparseable := &nativeAdapter{name: "cowrie-json", fails: true}
	if outcome := newPipeline(t, unparseable).Accept("cowrie-json", []byte("{}"), SourceNativeUDS); outcome.Reason != DropMalformed {
		t.Fatalf("reason = %q, want %q", outcome.Reason, DropMalformed)
	}
}

// An unknown adapter name is refused rather than guessed at.
func TestAnUnknownPackIsRefused(t *testing.T) {
	pipeline := newPipeline(t, &nativeAdapter{name: "cowrie-json"})

	outcome := pipeline.Accept("shell-exec", []byte("{}"), SourceNativeUDS)

	if outcome.Reason != DropUnknownPack {
		t.Fatalf("reason = %q, want %q", outcome.Reason, DropUnknownPack)
	}
}

// AC-EV-001 on the Edge: a retry is recognised before it is spooled.
func TestARetriedEventIsDroppedAsADuplicate(t *testing.T) {
	pipeline := newPipeline(t, &nativeAdapter{name: "cowrie-json"}, WithDeduplicator(NewRecentIDs(64)))
	raw := encode(t, sampleEvent(eventID))

	if outcome := pipeline.Accept("cowrie-json", raw, SourceNativeUDS); outcome.Dropped {
		t.Fatalf("the first delivery was dropped: %+v", outcome)
	}
	outcome := pipeline.Accept("cowrie-json", raw, SourceNativeUDS)

	if outcome.Reason != DropDuplicate {
		t.Fatalf("reason = %q, want %q", outcome.Reason, DropDuplicate)
	}
}

/*
 * Deduplication runs before rate limiting, so a retry storm is recognised as
 * retries rather than eating the budget a real burst needs.
 */
func TestARetryStormDoesNotConsumeTheRateBudget(t *testing.T) {
	limiter := NewTokenBucket(0.0001, 4)
	pipeline := newPipeline(t, &nativeAdapter{name: "cowrie-json"},
		WithDeduplicator(NewRecentIDs(64)), WithLimiter(limiter))
	repeat := encode(t, sampleEvent(eventID))

	// One genuine event, then fifty retries of it.
	pipeline.Accept("cowrie-json", repeat, SourceNativeUDS)
	for attempt := 0; attempt < 50; attempt++ {
		pipeline.Accept("cowrie-json", repeat, SourceNativeUDS)
	}

	// Three tokens should remain for genuinely new events.
	accepted := 0
	for n := 2; n <= 4; n++ {
		id := fmt.Sprintf("0198dc8c-c600-7000-8000-%012d", n)
		if outcome := pipeline.Accept("cowrie-json", encode(t, sampleEvent(id)), SourceNativeUDS); !outcome.Dropped {
			accepted++
		}
	}
	if accepted != 3 {
		t.Fatalf("accepted %d new events after a retry storm, want 3", accepted)
	}
	if pipeline.Drops()[DropRateLimited] != 0 {
		t.Fatal("retries consumed the rate budget")
	}
}

// A sustained flood is held, and a rate-limited event is not quarantined:
// it is well-formed and unremarkable, and keeping every one is how a burst
// becomes a disk problem.
func TestASustainedFloodIsHeldWithoutBeingKept(t *testing.T) {
	quarantine := &recordingQuarantine{}
	pipeline := newPipeline(t, &nativeAdapter{name: "cowrie-json"},
		WithLimiter(NewTokenBucket(0.0001, 2)), WithQuarantine(quarantine))

	limited := 0
	for n := 1; n <= 10; n++ {
		id := fmt.Sprintf("0198dc8c-c600-7000-8000-%012d", n)
		if outcome := pipeline.Accept("cowrie-json", encode(t, sampleEvent(id)), SourceNativeUDS); outcome.Reason == DropRateLimited {
			limited++
		}
	}

	if limited != 8 {
		t.Fatalf("rate-limited %d of 10, want 8 after a burst of 2", limited)
	}
	if len(quarantine.records) != 0 {
		t.Fatalf("quarantined %d rate-limited events", len(quarantine.records))
	}
}

// A burst gets through: that is what the burst allowance is for.
func TestABurstIsAllowedThrough(t *testing.T) {
	pipeline := newPipeline(t, &nativeAdapter{name: "cowrie-json"}, WithLimiter(NewTokenBucket(1, 16)))

	accepted := 0
	for n := 1; n <= 16; n++ {
		id := fmt.Sprintf("0198dc8c-c600-7000-8000-%012d", n)
		if outcome := pipeline.Accept("cowrie-json", encode(t, sampleEvent(id)), SourceNativeUDS); !outcome.Dropped {
			accepted++
		}
	}
	if accepted != 16 {
		t.Fatalf("a burst of 16 yielded %d, want all of them", accepted)
	}
}

/*
 * The log adapter path. A single enormous line — a log an attacker arranged to
 * contain one — is refused rather than read into memory, and the lines around
 * it still arrive.
 */
func TestAnEnormousLogLineIsRefusedAndTheRestSurvive(t *testing.T) {
	pipeline := newPipeline(t, &nativeAdapter{name: "cowrie-json"},
		WithLimits(Limits{MaxEventBytes: 4096, MaxBatchEvents: 64, MaxQuarantineBytes: 128}))

	first := encode(t, sampleEvent(eventID))
	monstrous := strings.Repeat("a", 100_000)
	log := string(first) + "\n" + monstrous + "\n"

	outcomes := pipeline.ReadLines("cowrie-json", strings.NewReader(log))

	if len(outcomes) < 2 {
		t.Fatalf("outcomes = %d, want the good line and a drop", len(outcomes))
	}
	if outcomes[0].Dropped {
		t.Fatalf("the well-formed line was dropped: %+v", outcomes[0])
	}
	last := outcomes[len(outcomes)-1]
	if !last.Dropped || last.Reason != DropOversized {
		t.Fatalf("last outcome = %+v, want an oversized drop", last)
	}
}

// One read cannot occupy the pipeline indefinitely.
func TestALogReadIsBounded(t *testing.T) {
	pipeline := newPipeline(t, &nativeAdapter{name: "cowrie-json"},
		WithLimits(Limits{MaxEventBytes: 4096, MaxBatchEvents: 5, MaxQuarantineBytes: 128}))

	var log strings.Builder
	for n := 1; n <= 100; n++ {
		id := fmt.Sprintf("0198dc8c-c600-7000-8000-%012d", n)
		log.Write(encode(t, sampleEvent(id)))
		log.WriteString("\n")
	}

	outcomes := pipeline.ReadLines("cowrie-json", strings.NewReader(log.String()))

	if len(outcomes) != 5 {
		t.Fatalf("read %d events from a 100-line log, want the 5-event bound", len(outcomes))
	}
}

// Every drop is counted under a reason from the closed set, because "the Edge
// dropped some events" is the least useful thing to tell an operator.
func TestEveryDropIsCountedUnderAClosedReason(t *testing.T) {
	known := map[DropReason]bool{}
	for _, reason := range DropReasons() {
		known[reason] = true
	}
	pipeline := newPipeline(t, &nativeAdapter{name: "cowrie-json"},
		WithLimits(Limits{MaxEventBytes: 64, MaxBatchEvents: 8, MaxQuarantineBytes: 16}),
		WithDeduplicator(NewRecentIDs(8)))

	pipeline.Accept("cowrie-json", []byte(strings.Repeat("a", 4096)), SourceNativeUDS)
	pipeline.Accept("unknown-pack", []byte("{}"), SourceNativeUDS)
	pipeline.Accept("cowrie-json", []byte("not json"), SourceNativeUDS)

	drops := pipeline.Drops()
	if len(drops) == 0 {
		t.Fatal("nothing was counted")
	}
	for reason, count := range drops {
		if !known[reason] {
			t.Fatalf("drop reason %q is outside the closed set", reason)
		}
		if count <= 0 {
			t.Fatalf("reason %q counted %d", reason, count)
		}
	}
}

// A clock that steps backwards must not stop an Edge reporting.
func TestABackwardClockDoesNotDrainTheBucket(t *testing.T) {
	bucket := NewTokenBucket(10, 4)
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

	for attempt := 0; attempt < 4; attempt++ {
		if !bucket.Allow(now) {
			t.Fatalf("burst token %d was refused", attempt)
		}
	}
	// The clock steps back an hour; the bucket must not be worse off than it was.
	if bucket.Allow(now.Add(-time.Hour)) {
		t.Fatal("a backward clock produced a token that had not been earned")
	}
	// And forward time still refills.
	if !bucket.Allow(now.Add(time.Second)) {
		t.Fatal("the bucket stopped refilling after a backward step")
	}
}
