// Package health owns the unprivileged Edge health contract and deterministic
// condition set. Runtime collectors and device-channel transport are wired by
// their approved integration packages.
package health

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	SchemaVersion      = "guardian.health.report.v1"
	MaxIdentifierBytes = 64
	MaxMessageBytes    = 512
	MaxReportBytes     = 16 * 1024
	HeartbeatInterval  = 30 * time.Second
	StaleAfter         = 90 * time.Second
	TimestampPrecision = time.Microsecond
	conditionCount     = 8
)

var (
	ErrInvalidCondition       = errors.New("invalid health condition")
	ErrInvalidReport          = errors.New("invalid health report")
	ErrInvalidMessage         = errors.New("invalid health condition message")
	ErrInvalidTimestamp       = errors.New("invalid health timestamp")
	ErrNonMonotonicTransition = errors.New("health transition time is not monotonic")
)

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

func ConditionTypes() []ConditionType {
	result := make([]ConditionType, len(conditionTypeOrder))
	copy(result, conditionTypeOrder[:])
	return result
}

type Status string

const (
	StatusTrue    Status = "True"
	StatusFalse   Status = "False"
	StatusUnknown Status = "Unknown"
)

type Condition struct {
	Type               ConditionType `json:"type"`
	Status             Status        `json:"status"`
	Reason             string        `json:"reason"`
	Message            string        `json:"message"`
	ObservedRevision   *uint64       `json:"observed_revision,omitempty"`
	LastTransitionTime time.Time     `json:"last_transition_time"`
}

type Observation struct {
	Type             ConditionType
	Status           Status
	Reason           string
	Message          string
	ObservedRevision *uint64
}

type Report struct {
	SchemaVersion string      `json:"schema_version"`
	ReportID      string      `json:"report_id"`
	Sequence      uint64      `json:"sequence"`
	ObservedAt    time.Time   `json:"observed_at"`
	Conditions    []Condition `json:"conditions"`
}

type Set struct {
	conditions [conditionCount]Condition
}

var reasonStatuses = map[ConditionType]map[string]Status{
	TypeEdgeConnected: {
		"connected": StatusTrue, "channel_disconnected": StatusFalse,
		"heartbeat_stale": StatusUnknown, "not_observed": StatusUnknown,
	},
	TypeDeviceCertificateReady: {
		"valid": StatusTrue, "rotation_window": StatusFalse, "expired": StatusFalse,
		"revoked": StatusFalse, "rotation_failed": StatusFalse,
		"clock_unreliable": StatusUnknown, "not_observed": StatusUnknown,
	},
	TypeConfigConverged: {
		"converged": StatusTrue, "revision_drift": StatusFalse, "not_observed": StatusUnknown,
	},
	TypeLocalDatabaseHealthy: {
		"ready": StatusTrue, "read_failed": StatusFalse, "write_failed": StatusFalse,
		"integrity_failed": StatusFalse, "not_observed": StatusUnknown,
	},
	TypeSpoolHealthy: {
		"ready": StatusTrue, "capacity_warning": StatusFalse, "capacity_critical": StatusFalse,
		"measurement_unavailable": StatusUnknown, "not_observed": StatusUnknown,
	},
	TypeClockQuality: {
		"synchronized": StatusTrue, "offset_exceeded": StatusFalse, "unsynchronized": StatusFalse,
		"measurement_unavailable": StatusUnknown, "not_observed": StatusUnknown,
	},
	TypeContainerRuntimeReachable: {
		"reachable": StatusTrue, "probe_failed": StatusFalse, "probe_timeout": StatusFalse,
		"not_observed": StatusUnknown,
	},
	TypePrivilegedHelperReachable: {
		"reachable": StatusTrue, "socket_missing": StatusFalse,
		"socket_verification_failed": StatusFalse, "connection_create_failed": StatusFalse,
		"rpc_timeout": StatusFalse, "peer_authentication_failed": StatusFalse,
		"rpc_unavailable": StatusFalse, "api_version_mismatch": StatusFalse,
		"not_observed": StatusUnknown,
	},
}

var forbiddenMessageFragments = [...]string{
	"password=", "password:", "authorization:", "bearer ", "cookie:",
	"token=", "token:", "session_token", "access_token", "refresh_token",
	"bootstrap_token", "enrollment_token", "api_key", "apikey", "secret=",
	"secret:", "recovery_code", "totp_secret", "set-cookie:", "private_key",
	"begin private key",
}

func NewUnknownSet(at time.Time) (Set, error) {
	if err := ValidateTimestamp(at); err != nil {
		return Set{}, err
	}
	var set Set
	for index, conditionType := range conditionTypeOrder {
		set.conditions[index] = Condition{
			Type: conditionType, Status: StatusUnknown, Reason: "not_observed",
			LastTransitionTime: at,
		}
	}
	return set, nil
}

func (set *Set) Observe(observation Observation, observedAt time.Time) error {
	if err := ValidateTimestamp(observedAt); err != nil {
		return err
	}
	index, ok := conditionIndex(observation.Type)
	if !ok {
		return fmt.Errorf("%w: unknown type", ErrInvalidCondition)
	}
	previous := set.conditions[index]
	transitionTime := observedAt
	if previous.Type != "" {
		if err := previous.Validate(); err != nil {
			return err
		}
		if previous.Status == observation.Status && previous.Reason == observation.Reason {
			transitionTime = previous.LastTransitionTime
		} else if !observedAt.After(previous.LastTransitionTime) {
			return ErrNonMonotonicTransition
		}
	}
	next := Condition{
		Type: observation.Type, Status: observation.Status, Reason: observation.Reason,
		Message: observation.Message, ObservedRevision: cloneRevision(observation.ObservedRevision),
		LastTransitionTime: transitionTime,
	}
	if err := next.Validate(); err != nil {
		return err
	}
	set.conditions[index] = next
	return nil
}

func (set Set) Conditions() []Condition {
	result := make([]Condition, len(set.conditions))
	for index, condition := range set.conditions {
		result[index] = cloneCondition(condition)
	}
	return result
}

func (set Set) Report(reportID string, sequence uint64, observedAt time.Time) (Report, error) {
	report := Report{
		SchemaVersion: SchemaVersion, ReportID: reportID, Sequence: sequence,
		ObservedAt: observedAt, Conditions: set.Conditions(),
	}
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	return report, nil
}

func (condition Condition) Validate() error {
	statuses, ok := reasonStatuses[condition.Type]
	if !ok || !validIdentifier(condition.Reason) || statuses[condition.Reason] != condition.Status {
		return ErrInvalidCondition
	}
	if condition.Status != StatusTrue && condition.Status != StatusFalse && condition.Status != StatusUnknown {
		return ErrInvalidCondition
	}
	if err := validateMessage(condition.Message); err != nil {
		return err
	}
	if err := ValidateTimestamp(condition.LastTransitionTime); err != nil {
		return err
	}
	return nil
}

func (report Report) Validate() error {
	if report.SchemaVersion != SchemaVersion || report.Sequence == 0 || !validUUIDv7(report.ReportID) {
		return ErrInvalidReport
	}
	if err := ValidateTimestamp(report.ObservedAt); err != nil {
		return ErrInvalidReport
	}
	if len(report.Conditions) != len(conditionTypeOrder) {
		return ErrInvalidReport
	}
	for index, condition := range report.Conditions {
		if condition.Type != conditionTypeOrder[index] || condition.LastTransitionTime.After(report.ObservedAt) {
			return ErrInvalidReport
		}
		if err := condition.Validate(); err != nil {
			return ErrInvalidReport
		}
	}
	encoded, err := json.Marshal(report)
	if err != nil || len(encoded) > MaxReportBytes {
		return ErrInvalidReport
	}
	return nil
}

func ValidateTimestamp(value time.Time) error {
	minimum := time.Time{}.Add(TimestampPrecision)
	if value.Location() != time.UTC || value.Before(minimum) || value.Year() > 9999 || value.Nanosecond()%int(TimestampPrecision) != 0 {
		return ErrInvalidTimestamp
	}
	return nil
}

func conditionIndex(conditionType ConditionType) (int, bool) {
	for index, candidate := range conditionTypeOrder {
		if candidate == conditionType {
			return index, true
		}
	}
	return 0, false
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

func validUUIDv7(value string) bool {
	if len(value) != 36 || value != strings.ToLower(value) || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || value[14] != '7' || !strings.ContainsRune("89ab", rune(value[19])) {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
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
