// Package decoy owns the Edge side of the decoy lifecycle: it receives desired
// state, asks a Runtime to converge, and reports what it observed.
//
// This package never holds device identity. It imports neither the identity nor
// the privileged packages, and a source-level check in this repository asserts
// that, so a decoy cannot reach the device key material or a privileged
// operation even by mistake.
package decoy

import (
	"context"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
)

// Desired is one decoy this Edge has been asked to place. It is the wire object
// reduced to what a runtime needs, and it deliberately carries no credential,
// key, image reference, command, mount, or capability.
type Desired struct {
	DecoyID          string
	ZoneID           string
	Address          string
	Family           devicev1.DecoyFamily
	Persona          devicev1.DecoyPersona
	InteractionLevel devicev1.DecoyInteractionLevel
	Pack             string
	PackVersion      string
	PackDigest       string
	Enabled          bool
	SourceRevision   uint64
}

// Observed is what a runtime saw. Every field is what the runtime can prove,
// which is why a runtime that proves nothing returns StateUnknown and six
// Unknown conditions rather than an optimistic default.
type Observed struct {
	DecoyID           string
	State             devicev1.DecoyObservedLifecycle
	LastInteractionAt *time.Time
	Conditions        []Condition
}

// Condition is one bounded observation following the P1-W9 model.
type Condition struct {
	Type               devicev1.DecoyConditionType
	Status             devicev1.HealthConditionStatus
	Reason             string
	Message            string
	ObservedRevision   *uint64
	LastTransitionTime time.Time
}

// Runtime is the seam P2-W3 fills with containerd. Converge is given the
// complete desired set, not a delta, so a runtime is never asked to infer what
// changed, and it returns one observation per decoy it was asked about.
//
// A Runtime is not permitted to report `unmanaged`: the wire enum has no such
// value, because that is a statement about the Control Plane's reach rather
// than about the decoy.
type Runtime interface {
	Name() string
	Converge(context.Context, []Desired) ([]Observed, error)
}

// nullRuntime accepts desired state, converges nothing, and says so.
//
// This is not a stub for convenience. It is the shape that makes P2-W3
// replaceable, and it is what lets the contract be exercised end to end before
// any container exists. Its correctness criterion is precise: for every decoy
// it is asked about it must report unknown with every dimension Unknown, and it
// must never report deployed, healthy, or absent, because it has not looked.
type nullRuntime struct {
	now func() time.Time
}

// NewNullRuntime returns the runtime that admits it converges nothing. P2-W3
// replaces it with containerd; until then, an operator reading the console sees
// "unknown" for every decoy, which is exactly what is true.
func NewNullRuntime() Runtime { return &nullRuntime{now: time.Now} }

func (*nullRuntime) Name() string { return "null" }

func (r *nullRuntime) Converge(_ context.Context, desired []Desired) ([]Observed, error) {
	observedAt := r.now().UTC()
	observations := make([]Observed, 0, len(desired))
	for _, entry := range desired {
		observations = append(observations, Observed{
			DecoyID:    entry.DecoyID,
			State:      devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_UNKNOWN,
			Conditions: unknownConditions(observedAt, "no_runtime"),
		})
	}
	return observations, nil
}

// conditionOrder is the complete section 9.4 dimension set in the order the
// wire contract fixes. A report is always the whole set: a partial one would
// let a missing dimension read as an absent problem.
var conditionOrder = [...]devicev1.DecoyConditionType{
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_RUNTIME_HEALTHY,
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_ADDRESS_APPLIED,
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_PORT_RESPONDING,
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_TELEMETRY_REPORTING,
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_POLICY_APPLIED,
	devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_VERSION_MATCHES_DESIRED,
}

// ConditionTypes returns the canonical order as a defensive copy.
func ConditionTypes() []devicev1.DecoyConditionType {
	result := make([]devicev1.DecoyConditionType, len(conditionOrder))
	copy(result, conditionOrder[:])
	return result
}

func unknownConditions(at time.Time, reason string) []Condition {
	conditions := make([]Condition, 0, len(conditionOrder))
	for _, conditionType := range conditionOrder {
		conditions = append(conditions, Condition{
			Type:               conditionType,
			Status:             devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNKNOWN,
			Reason:             reason,
			LastTransitionTime: at,
		})
	}
	return conditions
}
