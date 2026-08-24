package health

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrOutOfOrderReport  = errors.New("health report is out of order")
	ErrReportConflict    = errors.New("health report sequence conflicts with accepted truth")
	ErrTransitionHistory = errors.New("health report rewrites transition history")
)

// ApplyOutcome distinguishes a newly accepted report from an exact idempotent
// retry. Rejected reports never mutate a projection.
type ApplyOutcome uint8

const (
	ApplyAccepted ApplyOutcome = iota + 1
	ApplyDuplicate
)

// Projection is the latest accepted full report plus the trusted server
// receive time used for staleness.
type Projection struct {
	ReportID           string
	Sequence           uint64
	ObservedAt         time.Time
	ReceivedAt         time.Time
	Conditions         []Condition
	connectionOverride *Condition
}

// ApplyReport validates ordering, exact duplicate semantics, and Edge
// transition history before producing a defensive projection copy.
func ApplyReport(current *Projection, report Report, receivedAt time.Time) (Projection, ApplyOutcome, error) {
	if err := report.Validate(); err != nil {
		return Projection{}, 0, err
	}
	if err := ValidateTimestamp(receivedAt); err != nil {
		return Projection{}, 0, fmt.Errorf("receive time: %w", err)
	}
	if current != nil {
		if report.Sequence < current.Sequence {
			return Projection{}, 0, ErrOutOfOrderReport
		}
		if report.Sequence == current.Sequence {
			if sameAcceptedReport(*current, report) {
				return cloneProjection(*current), ApplyDuplicate, nil
			}
			return Projection{}, 0, ErrReportConflict
		}
		if err := validateTransitionHistory(current.Conditions, report.Conditions); err != nil {
			return Projection{}, 0, err
		}
	}
	projection := Projection{
		ReportID:   report.ReportID,
		Sequence:   report.Sequence,
		ObservedAt: report.ObservedAt,
		ReceivedAt: receivedAt,
		Conditions: cloneConditions(report.Conditions),
	}
	return projection, ApplyAccepted, nil
}

// EffectiveConditions returns latest known truth and derives only the
// connectivity stale transition from trusted server time.
func (projection Projection) EffectiveConditions(at time.Time) ([]Condition, error) {
	if err := ValidateTimestamp(at); err != nil {
		return nil, err
	}
	if err := ValidateTimestamp(projection.ReceivedAt); err != nil {
		return nil, fmt.Errorf("projection receive time: %w", err)
	}
	if at.Before(projection.ReceivedAt) {
		return nil, ErrInvalidTimestamp
	}
	if err := validateCompleteConditions(projection.Conditions); err != nil {
		return nil, err
	}
	conditions := cloneConditions(projection.Conditions)
	if projection.connectionOverride != nil {
		if err := projection.connectionOverride.Validate(); err != nil || projection.connectionOverride.Type != TypeEdgeConnected {
			return nil, ErrInvalidCondition
		}
		if at.Before(projection.connectionOverride.LastTransitionTime) {
			return nil, ErrInvalidTimestamp
		}
		conditions[0] = cloneCondition(*projection.connectionOverride)
		return conditions, nil
	}
	if at.Sub(projection.ReceivedAt) <= StaleAfter {
		return conditions, nil
	}
	staleAt := projection.ReceivedAt.Add(StaleAfter)
	current := conditions[0]
	if current.Status == StatusUnknown && current.Reason == "heartbeat_stale" {
		return conditions, nil
	}
	conditions[0] = Condition{
		Type:               TypeEdgeConnected,
		Status:             StatusUnknown,
		Reason:             "heartbeat_stale",
		Message:            "Health report is stale.",
		LastTransitionTime: staleAt,
	}
	return conditions, nil
}

// MarkDisconnected records explicit channel closure as trusted Control Plane
// truth without rewriting the Edge report or its clock-based transition
// history. A newer accepted full report clears this override.
func (projection Projection) MarkDisconnected(at time.Time) (Projection, error) {
	if err := ValidateTimestamp(at); err != nil {
		return Projection{}, err
	}
	if err := ValidateTimestamp(projection.ReceivedAt); err != nil {
		return Projection{}, fmt.Errorf("projection receive time: %w", err)
	}
	if err := validateCompleteConditions(projection.Conditions); err != nil {
		return Projection{}, err
	}
	if at.Before(projection.ReceivedAt) {
		return Projection{}, ErrInvalidTimestamp
	}
	next := cloneProjection(projection)
	transitionTime := at
	if next.connectionOverride != nil && next.connectionOverride.Status == StatusFalse && next.connectionOverride.Reason == "channel_disconnected" {
		if at.Before(next.connectionOverride.LastTransitionTime) {
			return Projection{}, ErrInvalidTimestamp
		}
		transitionTime = next.connectionOverride.LastTransitionTime
	}
	override := Condition{
		Type:               TypeEdgeConnected,
		Status:             StatusFalse,
		Reason:             "channel_disconnected",
		Message:            "Device channel is disconnected.",
		LastTransitionTime: transitionTime,
	}
	next.connectionOverride = &override
	return next, nil
}

// Aggregate is the false-green-safe summary of all blocking conditions.
type Aggregate struct {
	Status       Status
	BlockingType ConditionType
	Reason       string
}

// AggregateConditions gives known failure precedence over unknown truth, and
// unknown truth precedence over green.
func AggregateConditions(conditions []Condition) (Aggregate, error) {
	if err := validateCompleteConditions(conditions); err != nil {
		return Aggregate{}, err
	}
	for _, condition := range conditions {
		if condition.Status == StatusFalse {
			return Aggregate{Status: StatusFalse, BlockingType: condition.Type, Reason: condition.Reason}, nil
		}
	}
	for _, condition := range conditions {
		if condition.Status == StatusUnknown {
			return Aggregate{Status: StatusUnknown, BlockingType: condition.Type, Reason: condition.Reason}, nil
		}
	}
	return Aggregate{Status: StatusTrue}, nil
}

func validateTransitionHistory(previous, next []Condition) error {
	if err := validateCompleteConditions(previous); err != nil {
		return err
	}
	if err := validateCompleteConditions(next); err != nil {
		return err
	}
	for index := range previous {
		oldCondition := previous[index]
		newCondition := next[index]
		if oldCondition.Status == newCondition.Status && oldCondition.Reason == newCondition.Reason {
			if !oldCondition.LastTransitionTime.Equal(newCondition.LastTransitionTime) {
				return fmt.Errorf("%w: %s refresh changed transition time", ErrTransitionHistory, oldCondition.Type)
			}
			continue
		}
		if !newCondition.LastTransitionTime.After(oldCondition.LastTransitionTime) {
			return fmt.Errorf("%w: %s state change is not later", ErrTransitionHistory, oldCondition.Type)
		}
	}
	return nil
}

func validateCompleteConditions(conditions []Condition) error {
	if len(conditions) != len(conditionTypeOrder) {
		return ErrIncompleteReport
	}
	for index, condition := range conditions {
		if condition.Type != conditionTypeOrder[index] {
			return ErrNonCanonicalReport
		}
		if err := condition.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func sameAcceptedReport(projection Projection, report Report) bool {
	if projection.ReportID != report.ReportID || projection.Sequence != report.Sequence || !projection.ObservedAt.Equal(report.ObservedAt) || len(projection.Conditions) != len(report.Conditions) {
		return false
	}
	for index := range projection.Conditions {
		left := projection.Conditions[index]
		right := report.Conditions[index]
		if left.Type != right.Type || left.Status != right.Status || left.Reason != right.Reason || left.Message != right.Message || !left.LastTransitionTime.Equal(right.LastTransitionTime) || !sameRevision(left.ObservedRevision, right.ObservedRevision) {
			return false
		}
	}
	return true
}

func sameRevision(left, right *uint64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func cloneConditions(conditions []Condition) []Condition {
	result := make([]Condition, len(conditions))
	for index, condition := range conditions {
		copyCondition := condition
		copyCondition.ObservedRevision = cloneRevision(condition.ObservedRevision)
		result[index] = copyCondition
	}
	return result
}

func cloneProjection(projection Projection) Projection {
	copyProjection := projection
	copyProjection.Conditions = cloneConditions(projection.Conditions)
	if projection.connectionOverride != nil {
		copyOverride := cloneCondition(*projection.connectionOverride)
		copyProjection.connectionOverride = &copyOverride
	}
	return copyProjection
}
