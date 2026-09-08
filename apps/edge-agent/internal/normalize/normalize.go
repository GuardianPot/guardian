// Package normalize turns what a decoy pack produces into canonical events.
//
// Everything entering here has been shaped, directly or indirectly, by whoever
// is attacking the decoy: a Cowrie session log records what they typed, an HTTP
// access line records the path they asked for. So the package is written as if
// its input is hostile, because it is.
//
// The architecture is two paths and one pipeline. A native pack writes typed
// events to a socket the Edge owns; a third-party pack writes a log the Edge
// reads. Both converge on the same bounds, the same validation, the same
// deduplication, and the same closed set of reasons for throwing something away.
package normalize

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// SourcePath is which of the two ingestion paths produced an event.
//
// It travels into the canonical event's provenance because the two do not
// warrant equal confidence: a typed event from a pack Guardian ships is a
// different kind of claim from a line parsed out of third-party software's log
// format, and a reader is entitled to know which one they are looking at.
type SourcePath string

const (
	SourceNativeUDS  SourcePath = "native_uds"
	SourceLogAdapter SourcePath = "log_adapter"
)

/*
Limits bound everything the pipeline will read.

AC-SEC-008 requires an oversized or malformed payload not to crash the Edge.
Refusing to allocate for one is most of that: every bound below is checked
before the thing it bounds is parsed, not after.
*/
type Limits struct {
	// MaxEventBytes bounds one encoded event or one log line.
	MaxEventBytes int
	// MaxBatchEvents bounds how many events one read may yield, so a single
	// oversized batch cannot occupy the pipeline indefinitely.
	MaxBatchEvents int
	// MaxQuarantineBytes bounds what is kept from a rejected payload. The
	// material is attacker-controlled, so keeping all of it would let the
	// attacker choose how much disk a malformed event costs.
	MaxQuarantineBytes int
}

// DefaultLimits are deliberately small. A decoy event is metadata and a
// reference; anything approaching these bounds is not a decoy event.
func DefaultLimits() Limits {
	return Limits{MaxEventBytes: 32 << 10, MaxBatchEvents: 256, MaxQuarantineBytes: 4 << 10}
}

func (l Limits) normalised() Limits {
	if l.MaxEventBytes <= 0 {
		l.MaxEventBytes = DefaultLimits().MaxEventBytes
	}
	if l.MaxBatchEvents <= 0 {
		l.MaxBatchEvents = DefaultLimits().MaxBatchEvents
	}
	if l.MaxQuarantineBytes <= 0 {
		l.MaxQuarantineBytes = DefaultLimits().MaxQuarantineBytes
	}
	return l
}

/*
DropReason is the closed set of reasons the pipeline discards something.

Closed, and recorded on every drop, because "the Edge dropped some events" is
the least useful sentence an operator can be told about a deception sensor.
Each value below distinguishes a different operator response: a rate limit
means the decoy is busy, a malformed payload means the adapter and the pack
disagree, and a panic means the adapter has a bug.
*/
type DropReason string

const (
	DropOversized    DropReason = "oversized"
	DropMalformed    DropReason = "malformed"
	DropUnknownPack  DropReason = "unknown_pack"
	DropInvalidEvent DropReason = "invalid_event"
	DropDuplicate    DropReason = "duplicate"
	DropRateLimited  DropReason = "rate_limited"
	// DropAdapterFailed covers an adapter that panicked. The pipeline recovers
	// rather than letting it reach the process, because an adapter parses
	// attacker-influenced text and AC-SEC-008 says that must not crash the Edge.
	DropAdapterFailed DropReason = "adapter_failed"
)

// DropReasons returns the vocabulary in declaration order.
func DropReasons() []DropReason {
	return []DropReason{
		DropOversized,
		DropMalformed,
		DropUnknownPack,
		DropInvalidEvent,
		DropDuplicate,
		DropRateLimited,
		DropAdapterFailed,
	}
}

var ErrNoAdapter = errors.New("normalize: an adapter is required")

// Adapter turns one pack's output into a canonical event.
//
// Per pack, because only the pack knows its own format. The pipeline gives an
// adapter bytes that are already within bounds and expects an event or an
// error; it never expects an adapter to be careful, which is why a panicking
// one is contained rather than trusted.
type Adapter interface {
	// Name is the closed adapter identifier from the decoy manifest.
	Name() string
	// Normalize parses one payload. It must not retain the slice.
	Normalize(raw []byte, source SourcePath) (Event, error)
}

// Quarantine keeps rejected material for inspection.
//
// A malformed event is evidence of something — a pack change, an adapter bug,
// or an attacker probing the parser — and dropping it silently throws that
// away. What is kept is bounded and is never re-parsed.
type Quarantine interface {
	Quarantine(record QuarantineRecord) error
}

// QuarantineRecord is one rejected payload, bounded.
type QuarantineRecord struct {
	Adapter    string
	Source     SourcePath
	Reason     DropReason
	ObservedAt time.Time
	// Raw is truncated to the limit. Attacker-controlled: never rendered, never
	// re-parsed, never used to make a decision.
	Raw []byte
	// Truncated records whether the bound cut the material short, so a reader
	// never mistakes a fragment for the whole payload.
	Truncated bool
}

// Outcome is what happened to one payload.
type Outcome struct {
	Event   *Event
	Dropped bool
	Reason  DropReason
}

// Deduplicator rejects an identifier it has already seen.
type Deduplicator interface {
	Accept(eventID string) bool
}

// Limiter bounds how fast events are accepted.
type Limiter interface {
	Allow(now time.Time) bool
}

// Pipeline is the one path from a pack's output to a canonical event.
type Pipeline struct {
	adapters   map[string]Adapter
	limits     Limits
	dedup      Deduplicator
	limiter    Limiter
	quarantine Quarantine
	now        func() time.Time

	mu    sync.Mutex
	drops map[DropReason]int
}

type Option func(*Pipeline)

func WithLimits(limits Limits) Option {
	return func(p *Pipeline) { p.limits = limits.normalised() }
}

func WithDeduplicator(dedup Deduplicator) Option {
	return func(p *Pipeline) {
		if dedup != nil {
			p.dedup = dedup
		}
	}
}

func WithLimiter(limiter Limiter) Option {
	return func(p *Pipeline) {
		if limiter != nil {
			p.limiter = limiter
		}
	}
}

func WithQuarantine(quarantine Quarantine) Option {
	return func(p *Pipeline) {
		if quarantine != nil {
			p.quarantine = quarantine
		}
	}
}

func NewPipeline(adapters []Adapter, options ...Option) (*Pipeline, error) {
	if len(adapters) == 0 {
		return nil, ErrNoAdapter
	}
	pipeline := &Pipeline{
		adapters: make(map[string]Adapter, len(adapters)),
		limits:   DefaultLimits(),
		now:      time.Now,
		drops:    map[DropReason]int{},
	}
	for _, adapter := range adapters {
		if adapter == nil || adapter.Name() == "" {
			return nil, ErrNoAdapter
		}
		pipeline.adapters[adapter.Name()] = adapter
	}
	for _, option := range options {
		if option != nil {
			option(pipeline)
		}
	}
	return pipeline, nil
}

/*
Accept runs one payload through the pipeline.

The order is the security property. Size is checked before anything is parsed,
so an oversized payload costs a length comparison. The adapter runs inside a
recover, so a parser bug on attacker-influenced input becomes a dropped event
rather than a dead Edge. Validation runs before deduplication, so a malformed
identifier cannot enter the dedup window. Deduplication runs before rate
limiting, so a retry storm is recognised as retries rather than eating the
budget a real burst needs.
*/
func (p *Pipeline) Accept(adapterName string, raw []byte, source SourcePath) Outcome {
	observedAt := p.now()

	if len(raw) > p.limits.MaxEventBytes {
		// Quarantined without being parsed. Whatever this is, it is not a decoy
		// event, and finding out what it is must not cost more than refusing it.
		return p.drop(adapterName, source, DropOversized, raw, observedAt)
	}
	adapter, known := p.adapters[adapterName]
	if !known {
		return p.drop(adapterName, source, DropUnknownPack, raw, observedAt)
	}

	event, err := p.runAdapter(adapter, raw, source)
	if err != nil {
		reason := DropMalformed
		if errors.Is(err, errAdapterPanicked) {
			reason = DropAdapterFailed
		}
		return p.drop(adapterName, source, reason, raw, observedAt)
	}
	if err := event.Validate(); err != nil {
		return p.drop(adapterName, source, DropInvalidEvent, raw, observedAt)
	}
	// Provenance is set here rather than trusted from the adapter: which path an
	// event arrived through is the pipeline's knowledge, not the parser's, and
	// an adapter that could claim `native_uds` for a parsed log line would be
	// overstating the confidence a reader should place in it.
	event.Provenance.SourcePath = string(source)
	event.Provenance.Adapter = adapter.Name()

	if p.dedup != nil && !p.dedup.Accept(event.EventID) {
		return p.drop(adapterName, source, DropDuplicate, nil, observedAt)
	}
	if p.limiter != nil && !p.limiter.Allow(observedAt) {
		// Not quarantined: a rate-limited event is well-formed and unremarkable,
		// and keeping every one of them is how a burst becomes a disk problem.
		return p.drop(adapterName, source, DropRateLimited, nil, observedAt)
	}
	return Outcome{Event: &event}
}

var errAdapterPanicked = errors.New("adapter panicked")

// runAdapter contains the adapter.
//
// An adapter parses text an attacker wrote. A slice index it got wrong is a
// panic, and AC-SEC-008 says that must not take the Edge with it, so the panic
// is converted into a drop with its own reason — an operator seeing
// `adapter_failed` is looking at a Guardian bug, not at an attack.
func (p *Pipeline) runAdapter(adapter Adapter, raw []byte, source SourcePath) (event Event, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			event = Event{}
			err = fmt.Errorf("%w: %s", errAdapterPanicked, adapter.Name())
		}
	}()
	return adapter.Normalize(raw, source)
}

func (p *Pipeline) drop(
	adapterName string,
	source SourcePath,
	reason DropReason,
	raw []byte,
	observedAt time.Time,
) Outcome {
	p.mu.Lock()
	p.drops[reason]++
	p.mu.Unlock()

	if raw != nil && p.quarantine != nil {
		kept := raw
		truncated := false
		if len(kept) > p.limits.MaxQuarantineBytes {
			kept = kept[:p.limits.MaxQuarantineBytes]
			truncated = true
		}
		// Copied, so the caller's buffer can be reused without the quarantine
		// record changing under whoever reads it later.
		stored := make([]byte, len(kept))
		copy(stored, kept)
		_ = p.quarantine.Quarantine(QuarantineRecord{
			Adapter: adapterName, Source: source, Reason: reason,
			ObservedAt: observedAt, Raw: stored, Truncated: truncated,
		})
	}
	return Outcome{Dropped: true, Reason: reason}
}

// Drops reports the count per reason, for the health and metrics paths.
func (p *Pipeline) Drops() map[DropReason]int {
	p.mu.Lock()
	defer p.mu.Unlock()
	snapshot := make(map[DropReason]int, len(p.drops))
	for reason, count := range p.drops {
		snapshot[reason] = count
	}
	return snapshot
}
