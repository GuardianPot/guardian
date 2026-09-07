package decoy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/reconciliation"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MaxDecoys is the desired-object bound the device channel guarantees.
const MaxDecoys = 64

// Publisher is the channel seam. The component publishes observed decoy state
// and never reads desired state from the channel directly: desired state
// arrives through the reconciler, which owns revision semantics.
type Publisher interface {
	PublishDecoyState(*devicev1.DecoyStateReport) error
}

type PublisherFunc func(*devicev1.DecoyStateReport) error

func (function PublisherFunc) PublishDecoyState(report *devicev1.DecoyStateReport) error {
	return function(report)
}

// Manager is the Edge decoy component. It is an Applier for the reconciler:
// the reconciler hands it a validated desired snapshot, it converges through
// the Runtime, and it publishes what the runtime observed.
//
// It holds no device identity and reaches no privileged operation. Its only
// outward capabilities are "ask the runtime to converge" and "publish an
// observation".
type Manager struct {
	runtime   Runtime
	publisher Publisher
	store     *storage.Store
	now       func() time.Time

	mu      sync.Mutex
	desired []Desired
}

func New(runtime Runtime, publisher Publisher, store *storage.Store) (*Manager, error) {
	if runtime == nil || publisher == nil || store == nil {
		return nil, errors.New("decoy runtime, publisher, and store are required")
	}
	return &Manager{runtime: runtime, publisher: publisher, store: store, now: time.Now}, nil
}

func (*Manager) Name() string { return "decoy-manager" }

// Start records the component's honest health. A null runtime is degraded, not
// healthy: the component works, but nothing it reports about a decoy is a
// confirmation, and the Edge health surface should say so.
func (m *Manager) Start(ctx context.Context) error {
	status, reason := "healthy", "ready"
	if m.runtime.Name() == "null" {
		status, reason = "degraded", "no-decoy-runtime"
	}
	return m.store.SetHealth(ctx, storage.HealthCondition{
		Name: "decoy-manager", Status: status, ReasonCode: reason,
	})
}

func (m *Manager) Stop(ctx context.Context) error {
	return m.store.SetHealth(ctx, storage.HealthCondition{
		Name: "decoy-manager", Status: "stopped", ReasonCode: "shutdown",
	})
}

// Apply converges the decoys in one desired-state snapshot and publishes the
// result. It satisfies the reconciler's Applier interface, so a decoy failure
// is a reconciliation failure with a bounded reason code rather than a silent
// divergence.
func (m *Manager) Apply(ctx context.Context, snapshot *devicev1.DesiredStateSnapshot) error {
	desired, err := desiredFromSnapshot(snapshot)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.desired = desired
	m.mu.Unlock()

	observations, err := m.runtime.Converge(ctx, desired)
	if err != nil {
		return &reconciliation.ApplyFailure{ReasonCode: "decoy_converge_failed", Retryable: true}
	}
	if err := m.publish(observations, revisionOf(snapshot)); err != nil {
		return &reconciliation.ApplyFailure{ReasonCode: "decoy_report_failed", Retryable: true}
	}
	return nil
}

// Report re-observes the current desired set without changing it. It exists so
// that a periodic sweep, or P2-W14's health probing, can refresh observed truth
// between desired-state revisions.
func (m *Manager) Report(ctx context.Context) error {
	m.mu.Lock()
	desired := make([]Desired, len(m.desired))
	copy(desired, m.desired)
	m.mu.Unlock()
	observations, err := m.runtime.Converge(ctx, desired)
	if err != nil {
		return err
	}
	var revision uint64
	for _, entry := range desired {
		if entry.SourceRevision > revision {
			revision = entry.SourceRevision
		}
	}
	return m.publish(observations, revision)
}

func (m *Manager) publish(observations []Observed, desiredRevision uint64) error {
	if len(observations) == 0 {
		// Nothing to say is not the same as everything is fine, and an empty
		// report would be indistinguishable from a report of no decoys. The
		// Control Plane already treats a decoy with no report as unknown, so
		// staying silent is the honest option.
		return nil
	}
	reportID, err := newUUIDv7(m.now().UTC())
	if err != nil {
		return err
	}
	observedAt := m.now().UTC()
	wire := make([]*devicev1.DecoyObservation, 0, len(observations))
	for _, observation := range observations {
		entry := &devicev1.DecoyObservation{
			DecoyId: observation.DecoyID, State: observation.State,
			DesiredRevision: desiredRevision,
			Conditions:      conditionsToWire(observation.Conditions, observedAt),
		}
		if observation.LastInteractionAt != nil && !observation.LastInteractionAt.After(observedAt) {
			entry.LastInteractionAt = timestamppb.New(observation.LastInteractionAt.UTC())
		}
		wire = append(wire, entry)
	}
	return m.publisher.PublishDecoyState(&devicev1.DecoyStateReport{
		ReportId: reportID, ObservedAt: timestamppb.New(observedAt), Decoys: wire,
	})
}

// conditionsToWire always emits the complete ordered set. A dimension the
// runtime did not report becomes Unknown, never absent and never favourable.
func conditionsToWire(conditions []Condition, observedAt time.Time) []*devicev1.DecoyCondition {
	supplied := make(map[devicev1.DecoyConditionType]Condition, len(conditions))
	for _, condition := range conditions {
		supplied[condition.Type] = condition
	}
	wire := make([]*devicev1.DecoyCondition, 0, len(conditionOrder))
	for _, conditionType := range conditionOrder {
		condition, ok := supplied[conditionType]
		if !ok {
			condition = Condition{
				Type:               conditionType,
				Status:             devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNKNOWN,
				Reason:             "not_observed",
				LastTransitionTime: observedAt,
			}
		}
		if condition.LastTransitionTime.IsZero() || condition.LastTransitionTime.After(observedAt) {
			condition.LastTransitionTime = observedAt
		}
		if condition.Reason == "" {
			condition.Reason = "not_observed"
		}
		entry := &devicev1.DecoyCondition{
			Type: conditionType, Status: condition.Status, Reason: condition.Reason,
			Message:            condition.Message,
			LastTransitionTime: timestamppb.New(condition.LastTransitionTime.UTC()),
		}
		if condition.ObservedRevision != nil {
			revision := *condition.ObservedRevision
			entry.ObservedRevision = &revision
		}
		wire = append(wire, entry)
	}
	return wire
}

func desiredFromSnapshot(snapshot *devicev1.DesiredStateSnapshot) ([]Desired, error) {
	if snapshot == nil {
		return nil, &reconciliation.ApplyFailure{ReasonCode: "invalid_snapshot"}
	}
	if len(snapshot.Decoys) > MaxDecoys {
		return nil, &reconciliation.ApplyFailure{ReasonCode: "decoy_bound_exceeded"}
	}
	desired := make([]Desired, 0, len(snapshot.Decoys))
	for _, entry := range snapshot.Decoys {
		if entry == nil {
			return nil, &reconciliation.ApplyFailure{ReasonCode: "invalid_snapshot"}
		}
		desired = append(desired, Desired{
			DecoyID: entry.DecoyId, ZoneID: entry.ZoneId, Address: entry.Address,
			Family: entry.Family, Persona: entry.Persona, InteractionLevel: entry.InteractionLevel,
			Pack: entry.Pack, PackVersion: entry.PackVersion, PackDigest: entry.PackDigest,
			Enabled:        entry.DesiredState == devicev1.DecoyDesiredLifecycle_DECOY_DESIRED_LIFECYCLE_DEPLOYED,
			SourceRevision: entry.SourceRevision,
		})
	}
	return desired, nil
}

func revisionOf(snapshot *devicev1.DesiredStateSnapshot) uint64 {
	if snapshot == nil {
		return 0
	}
	return snapshot.Revision
}

func newUUIDv7(now time.Time) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[6:]); err != nil {
		return "", fmt.Errorf("generate decoy report identity: %w", err)
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
