package health

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// SchemaVersion is the closed Phase 1 health report envelope.
	SchemaVersion = "guardian.health.report.v1"
	// MaxIdentifierBytes bounds condition types and reason codes.
	MaxIdentifierBytes = 64
	// MaxMessageBytes bounds escaped, human-readable condition detail.
	MaxMessageBytes = 512
	// MaxReportBytes bounds one encoded full health snapshot.
	MaxReportBytes = 16 * 1024
	// HeartbeatInterval is the approved full-report cadence.
	HeartbeatInterval = 30 * time.Second
	// StaleAfter is measured from the Control Plane receive time.
	StaleAfter = 90 * time.Second
	// TimestampPrecision matches the PostgreSQL contract precision.
	TimestampPrecision = time.Microsecond
)

var (
	ErrInvalidCondition       = errors.New("invalid health condition")
	ErrInvalidConditionType   = errors.New("invalid health condition type")
	ErrInvalidConditionStatus = errors.New("invalid health condition status")
	ErrInvalidConditionReason = errors.New("invalid health condition reason")
	ErrInvalidMessage         = errors.New("invalid health condition message")
	ErrInvalidTimestamp       = errors.New("invalid health timestamp")
	ErrNonMonotonicTransition = errors.New("health transition time is not monotonic")
)

// ConditionType is one of the eight blocking Phase 1 health dimensions.
type ConditionType string

const (
	TypeEdgeConnected             ConditionType = "edge_connected"
	TypeDeviceCertificateReady    ConditionType = "device_certificate_ready"
	TypeConfigConverged           ConditionType = "config_converged"
	TypeLocalDatabaseHealthy      ConditionType = "local_database_healthy"
	TypeSpoolHealthy              ConditionType = "spool_healthy"
	TypeClockQuality              ConditionType = "clock_quality"
	TypeContainerRuntimeReachable ConditionType = "container_runtime_reachable"
	TypePrivilegedHelperReachable ConditionType = "privileged_helper_reachable"
)

var conditionTypeOrder = [...]ConditionType{
	TypeEdgeConnected,
	TypeDeviceCertificateReady,
	TypeConfigConverged,
	TypeLocalDatabaseHealthy,
	TypeSpoolHealthy,
	TypeClockQuality,
	TypeContainerRuntimeReachable,
	TypePrivilegedHelperReachable,
}

// ConditionTypes returns the canonical condition order as a defensive copy.
func ConditionTypes() []ConditionType {
	result := make([]ConditionType, len(conditionTypeOrder))
	copy(result, conditionTypeOrder[:])
	return result
}

// Status is the exact tri-state condition value exposed by every contract.
type Status string

const (
	StatusTrue    Status = "True"
	StatusFalse   Status = "False"
	StatusUnknown Status = "Unknown"
)

// Condition is one canonical, bounded health observation.
type Condition struct {
	Type               ConditionType `json:"type"`
	Status             Status        `json:"status"`
	Reason             string        `json:"reason"`
	Message            string        `json:"message"`
	ObservedRevision   *uint64       `json:"observed_revision,omitempty"`
	LastTransitionTime time.Time     `json:"last_transition_time"`
}

// Observation is the mutable input to a deterministic condition transition.
type Observation struct {
	Type             ConditionType
	Status           Status
	Reason           string
	Message          string
	ObservedRevision *uint64
}

var reasonStatuses = map[ConditionType]map[string]Status{
	TypeEdgeConnected: {
		"connected":            StatusTrue,
		"channel_disconnected": StatusFalse,
		"heartbeat_stale":      StatusUnknown,
		"not_observed":         StatusUnknown,
	},
	TypeDeviceCertificateReady: {
		"valid":            StatusTrue,
		"rotation_window":  StatusFalse,
		"expired":          StatusFalse,
		"revoked":          StatusFalse,
		"rotation_failed":  StatusFalse,
		"clock_unreliable": StatusUnknown,
		"not_observed":     StatusUnknown,
	},
	TypeConfigConverged: {
		"converged":      StatusTrue,
		"revision_drift": StatusFalse,
		"not_observed":   StatusUnknown,
	},
	TypeLocalDatabaseHealthy: {
		"ready":            StatusTrue,
		"read_failed":      StatusFalse,
		"write_failed":     StatusFalse,
		"integrity_failed": StatusFalse,
		"not_observed":     StatusUnknown,
	},
	TypeSpoolHealthy: {
		"ready":                   StatusTrue,
		"capacity_warning":        StatusFalse,
		"capacity_critical":       StatusFalse,
		"measurement_unavailable": StatusUnknown,
		"not_observed":            StatusUnknown,
	},
	TypeClockQuality: {
		"synchronized":            StatusTrue,
		"offset_exceeded":         StatusFalse,
		"unsynchronized":          StatusFalse,
		"measurement_unavailable": StatusUnknown,
		"not_observed":            StatusUnknown,
	},
	TypeContainerRuntimeReachable: {
		"reachable":     StatusTrue,
		"probe_failed":  StatusFalse,
		"probe_timeout": StatusFalse,
		"not_observed":  StatusUnknown,
	},
	TypePrivilegedHelperReachable: {
		"reachable":                  StatusTrue,
		"socket_missing":             StatusFalse,
		"socket_verification_failed": StatusFalse,
		"connection_create_failed":   StatusFalse,
		"rpc_timeout":                StatusFalse,
		"peer_authentication_failed": StatusFalse,
		"rpc_unavailable":            StatusFalse,
		"api_version_mismatch":       StatusFalse,
		"not_observed":               StatusUnknown,
	},
}

var forbiddenMessageFragments = [...]string{
	"password=",
	"password:",
	"authorization:",
	"bearer ",
	"cookie:",
	"token=",
	"token:",
	"session_token",
	"access_token",
	"refresh_token",
	"bootstrap_token",
	"enrollment_token",
	"api_key",
	"apikey",
	"secret=",
	"secret:",
	"recovery_code",
	"totp_secret",
	"set-cookie:",
	"private_key",
	"begin private key",
}

// Validate enforces the closed vocabulary, status/reason pairing, bounds, and
// canonical timestamp contract.
func (condition Condition) Validate() error {
	if _, ok := reasonStatuses[condition.Type]; !ok {
		return fmt.Errorf("%w: %q", ErrInvalidConditionType, condition.Type)
	}
	if condition.Status != StatusTrue && condition.Status != StatusFalse && condition.Status != StatusUnknown {
		return fmt.Errorf("%w: %q", ErrInvalidConditionStatus, condition.Status)
	}
	if !validIdentifier(condition.Reason) {
		return fmt.Errorf("%w: malformed identifier", ErrInvalidConditionReason)
	}
	expectedStatus, ok := reasonStatuses[condition.Type][condition.Reason]
	if !ok || expectedStatus != condition.Status {
		return fmt.Errorf("%w: %s/%s/%s", ErrInvalidConditionReason, condition.Type, condition.Status, condition.Reason)
	}
	if err := validateMessage(condition.Message); err != nil {
		return err
	}
	if err := ValidateTimestamp(condition.LastTransitionTime); err != nil {
		return fmt.Errorf("%w: last transition time: %v", ErrInvalidCondition, err)
	}
	return nil
}

// Transition applies one observation. Status/reason changes advance the
// transition timestamp; message and revision refreshes preserve it.
func Transition(previous *Condition, observation Observation, observedAt time.Time) (Condition, error) {
	if err := ValidateTimestamp(observedAt); err != nil {
		return Condition{}, err
	}
	transitionTime := observedAt
	if previous != nil {
		if err := previous.Validate(); err != nil {
			return Condition{}, fmt.Errorf("previous condition: %w", err)
		}
		if previous.Type != observation.Type {
			return Condition{}, fmt.Errorf("%w: transition type changed", ErrInvalidConditionType)
		}
		if previous.Status == observation.Status && previous.Reason == observation.Reason {
			transitionTime = previous.LastTransitionTime
		} else if !observedAt.After(previous.LastTransitionTime) {
			return Condition{}, ErrNonMonotonicTransition
		}
	}
	next := Condition{
		Type:               observation.Type,
		Status:             observation.Status,
		Reason:             observation.Reason,
		Message:            observation.Message,
		ObservedRevision:   cloneRevision(observation.ObservedRevision),
		LastTransitionTime: transitionTime,
	}
	if err := next.Validate(); err != nil {
		return Condition{}, err
	}
	return next, nil
}

// UnknownConditions returns a complete initial snapshot without inventing
// healthy state before evidence exists.
func UnknownConditions(at time.Time) ([]Condition, error) {
	conditions := make([]Condition, 0, len(conditionTypeOrder))
	for _, conditionType := range conditionTypeOrder {
		reason := "not_observed"
		if conditionType == TypeEdgeConnected {
			reason = "heartbeat_stale"
		}
		condition, err := Transition(nil, Observation{
			Type:   conditionType,
			Status: StatusUnknown,
			Reason: reason,
		}, at)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, condition)
	}
	return conditions, nil
}

// ValidateTimestamp requires UTC, microsecond precision, and the representable
// PostgreSQL/JSON year range used by the Control Plane.
func ValidateTimestamp(value time.Time) error {
	minimum := time.Time{}.Add(TimestampPrecision)
	if value.Location() != time.UTC || value.Before(minimum) || value.Year() > 9999 || value.Nanosecond()%int(TimestampPrecision) != 0 {
		return ErrInvalidTimestamp
	}
	return nil
}

func validIdentifier(value string) bool {
	if len(value) < 1 || len(value) > MaxIdentifierBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' || index > 0 && character >= '0' && character <= '9' || index > 0 && character == '_' {
			continue
		}
		return false
	}
	return true
}

func validateMessage(message string) error {
	if len(message) > MaxMessageBytes || !utf8.ValidString(message) {
		return ErrInvalidMessage
	}
	for _, character := range message {
		if character < 0x20 && character != '\t' {
			return ErrInvalidMessage
		}
	}
	lower := strings.ToLower(message)
	for _, fragment := range forbiddenMessageFragments {
		if strings.Contains(lower, fragment) {
			return ErrInvalidMessage
		}
	}
	return nil
}

func cloneRevision(revision *uint64) *uint64 {
	if revision == nil {
		return nil
	}
	copyValue := *revision
	return &copyValue
}

func cloneCondition(condition Condition) Condition {
	copyCondition := condition
	copyCondition.ObservedRevision = cloneRevision(condition.ObservedRevision)
	return copyCondition
}
