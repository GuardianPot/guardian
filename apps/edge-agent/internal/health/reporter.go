package health

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
)

type Publisher interface {
	PublishHealth(Report) error
}

type PublisherFunc func(Report) error

func (function PublisherFunc) PublishHealth(report Report) error { return function(report) }

type Collector interface {
	Collect(context.Context, time.Time) ([]Observation, error)
}

type CollectorFunc func(context.Context, time.Time) ([]Observation, error)

func (function CollectorFunc) Collect(ctx context.Context, now time.Time) ([]Observation, error) {
	return function(ctx, now)
}

type Reporter struct {
	store     StateStore
	publisher Publisher
	collector Collector
	interval  time.Duration
	now       func() time.Time
	newID     func(time.Time) (string, error)

	mu                      sync.Mutex
	channelObservation      Observation
	channelRevision         uint64
	reportedChannelRevision uint64
	cancel                  context.CancelFunc
	done                    chan struct{}
	trigger                 chan struct{}
}

func NewReporter(store StateStore, publisher Publisher, collector Collector) (*Reporter, error) {
	if store == nil || publisher == nil || collector == nil {
		return nil, errors.New("health store, publisher, and collector are required")
	}
	return &Reporter{
		store: store, publisher: publisher, collector: collector, interval: HeartbeatInterval,
		now:                func() time.Time { return time.Now().UTC().Truncate(TimestampPrecision) },
		newID:              newHealthUUIDv7,
		channelObservation: Observation{Type: TypeEdgeConnected, Status: StatusUnknown, Reason: "not_observed"},
		trigger:            make(chan struct{}, 1),
	}, nil
}

func (*Reporter) Name() string { return "health-reporter" }

func (r *Reporter) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.cancel != nil {
		r.mu.Unlock()
		return errors.New("health reporter already started")
	}
	runContext, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.done = make(chan struct{})
	done := r.done
	r.mu.Unlock()
	if err := r.cycle(runContext); err != nil {
		cancel()
		r.mu.Lock()
		r.cancel = nil
		r.done = nil
		r.mu.Unlock()
		return err
	}
	go r.loop(runContext, done)
	return nil
}

func (r *Reporter) Stop(ctx context.Context) error {
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.mu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		r.mu.Lock()
		if r.done == done {
			r.cancel = nil
			r.done = nil
		}
		r.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Reporter) SetChannelState(_ context.Context, state, _ string) error {
	observation := Observation{Type: TypeEdgeConnected, Status: StatusFalse, Reason: "channel_disconnected"}
	switch state {
	case "connected":
		observation.Status, observation.Reason = StatusTrue, "connected"
	case "connecting":
		observation.Status, observation.Reason = StatusUnknown, "heartbeat_stale"
	case "degraded", "disconnected", "stopped":
	default:
		observation.Status, observation.Reason = StatusUnknown, "not_observed"
	}
	r.mu.Lock()
	r.channelObservation = observation
	r.channelRevision++
	started := r.cancel != nil
	r.mu.Unlock()
	if started {
		select {
		case r.trigger <- struct{}{}:
		default:
		}
	}
	return nil
}

func (r *Reporter) Acknowledgement(ctx context.Context, acknowledgement *devicev1.Acknowledgement) error {
	if acknowledgement == nil || acknowledgement.Kind != devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_HEALTH_REPORT || acknowledgement.MessageId == "" || acknowledgement.Revision == 0 {
		return ErrAcknowledgementMismatch
	}
	if err := r.store.AcknowledgeHealthReport(ctx, acknowledgement.MessageId, acknowledgement.Revision); err != nil {
		return err
	}
	r.mu.Lock()
	dirty := r.channelRevision != r.reportedChannelRevision
	started := r.cancel != nil
	r.mu.Unlock()
	if dirty && started {
		select {
		case r.trigger <- struct{}{}:
		default:
		}
	}
	return nil
}

func (r *Reporter) loop(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.cycle(ctx)
		case <-r.trigger:
			_ = r.cycle(ctx)
		}
	}
}

func (r *Reporter) cycle(ctx context.Context) error {
	state, err := r.store.LoadHealthState(ctx)
	if err != nil {
		return fmt.Errorf("load health state: %w", err)
	}
	if state.PendingReport != nil {
		return r.publisher.PublishHealth(*state.PendingReport)
	}
	now := r.now()
	var set Set
	if len(state.Conditions) == 0 {
		set, err = NewUnknownSet(now.Add(-TimestampPrecision))
	} else {
		set, err = NewSet(state.Conditions)
	}
	if err != nil {
		return err
	}
	observations, err := r.collector.Collect(ctx, now)
	if err != nil {
		return fmt.Errorf("collect health evidence: %w", err)
	}
	r.mu.Lock()
	channelObservation := r.channelObservation
	channelRevision := r.channelRevision
	r.mu.Unlock()
	observations = append(observations, channelObservation)
	seen := make(map[ConditionType]struct{}, len(observations))
	for _, observation := range observations {
		if _, duplicate := seen[observation.Type]; duplicate {
			return fmt.Errorf("%w: duplicate observation %s", ErrInvalidCondition, observation.Type)
		}
		seen[observation.Type] = struct{}{}
		if err := set.Observe(observation, now); err != nil {
			return err
		}
	}
	if len(seen) != conditionCount {
		return fmt.Errorf("%w: collector returned %d of %d conditions", ErrInvalidCondition, len(seen), conditionCount)
	}
	reportID, err := r.newID(now)
	if err != nil {
		return err
	}
	report, err := set.Report(reportID, state.NextSequence, now)
	if err != nil {
		return err
	}
	payload, err := MarshalReport(report)
	if err != nil {
		return err
	}
	if err := r.store.PersistHealthReport(ctx, report, payload); err != nil {
		return fmt.Errorf("persist health report: %w", err)
	}
	r.mu.Lock()
	if r.reportedChannelRevision < channelRevision {
		r.reportedChannelRevision = channelRevision
	}
	r.mu.Unlock()
	return r.publisher.PublishHealth(report)
}

func newHealthUUIDv7(now time.Time) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[6:]); err != nil {
		return "", fmt.Errorf("generate health UUIDv7: %w", err)
	}
	milliseconds := uint64(now.UTC().UnixMilli())
	value[0] = byte(milliseconds >> 40)
	value[1] = byte(milliseconds >> 32)
	value[2] = byte(milliseconds >> 24)
	value[3] = byte(milliseconds >> 16)
	value[4] = byte(milliseconds >> 8)
	value[5] = byte(milliseconds)
	value[6] = (value[6] & 0x0f) | 0x70
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], value[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], value[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], value[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], value[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], value[10:16])
	return string(encoded), nil
}
