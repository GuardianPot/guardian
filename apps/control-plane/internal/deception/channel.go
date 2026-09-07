package deception

import (
	"context"
	"errors"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel"
	devicev1 "github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel/gen/guardian/device/v1"
)

// ChannelHandler is the only path by which observed decoy state enters the
// Control Plane. The REST surface has no equivalent, so an operator cannot
// assert that a decoy is healthy; only a device that reported it can.
type ChannelHandler struct {
	service *Service
}

func NewChannelHandler(service *Service) (*ChannelHandler, error) {
	if service == nil {
		return nil, errors.New("decoy service is required")
	}
	return &ChannelHandler{service: service}, nil
}

func (h *ChannelHandler) DecoyStateReport(
	ctx context.Context,
	identity devicechannel.DeviceIdentity,
	wire *devicev1.DecoyStateReport,
) error {
	report, err := reportFromWire(identity.DeviceID, wire)
	if err != nil {
		return err
	}
	return h.service.RecordObservations(ctx, report)
}

func reportFromWire(deviceID string, wire *devicev1.DecoyStateReport) (Report, error) {
	if wire == nil || wire.ObservedAt == nil || wire.ObservedAt.CheckValid() != nil {
		return Report{}, ErrInvalidInput
	}
	observedAt := wire.ObservedAt.AsTime().UTC()
	observations := make([]Observation, 0, len(wire.Decoys))
	for _, entry := range wire.Decoys {
		if entry == nil {
			return Report{}, ErrInvalidInput
		}
		state, ok := observedLifecycleFromWire(entry.State)
		if !ok {
			return Report{}, ErrInvalidInput
		}
		observation := Observation{
			DecoyID: entry.DecoyId, State: state,
			ReportingDeviceID: deviceID, ReportedAt: &observedAt,
		}
		if entry.DesiredRevision != 0 {
			revision := entry.DesiredRevision
			observation.DesiredRevision = &revision
		}
		if entry.LastInteractionAt != nil {
			if entry.LastInteractionAt.CheckValid() != nil {
				return Report{}, ErrInvalidInput
			}
			interaction := entry.LastInteractionAt.AsTime().UTC()
			observation.LastInteractionAt = &interaction
		}
		conditions, err := conditionsFromWire(entry.Conditions)
		if err != nil {
			return Report{}, err
		}
		observation.Conditions = conditions
		observations = append(observations, observation)
	}
	report := Report{
		ReportID: wire.ReportId, ObservedAt: observedAt,
		DeviceID: deviceID, Observations: observations,
	}
	if err := ValidateReport(report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func conditionsFromWire(wire []*devicev1.DecoyCondition) ([]Condition, error) {
	types := ConditionTypes()
	if len(wire) != len(types) {
		return nil, ErrInvalidInput
	}
	conditions := make([]Condition, 0, len(wire))
	for index, entry := range wire {
		if entry == nil || entry.LastTransitionTime == nil || entry.LastTransitionTime.CheckValid() != nil {
			return nil, ErrInvalidInput
		}
		conditionType, ok := conditionTypeFromWire(entry.Type)
		if !ok || conditionType != types[index] {
			return nil, ErrInvalidInput
		}
		status, ok := statusFromWire(entry.Status)
		if !ok {
			return nil, ErrInvalidInput
		}
		condition := Condition{
			Type: conditionType, Status: status, Reason: entry.Reason, Message: entry.Message,
			LastTransitionTime: entry.LastTransitionTime.AsTime().UTC(),
		}
		if entry.ObservedRevision != nil {
			revision := entry.GetObservedRevision()
			condition.ObservedRevision = &revision
		}
		conditions = append(conditions, condition)
	}
	return conditions, nil
}

// observedLifecycleFromWire deliberately has no `unmanaged` case: the wire enum
// has no such value, because SEC-06 is projected from device state rather than
// self-reported by the device that lost management.
func observedLifecycleFromWire(value devicev1.DecoyObservedLifecycle) (ObservedLifecycle, bool) {
	switch value {
	case devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_UNKNOWN:
		return ObservedUnknown, true
	case devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_DEPLOYED:
		return ObservedDeployed, true
	case devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_DEGRADED:
		return ObservedDegraded, true
	case devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_ABSENT:
		return ObservedAbsent, true
	default:
		return "", false
	}
}

func conditionTypeFromWire(value devicev1.DecoyConditionType) (ConditionType, bool) {
	switch value {
	case devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_RUNTIME_HEALTHY:
		return ConditionRuntimeHealthy, true
	case devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_ADDRESS_APPLIED:
		return ConditionAddressApplied, true
	case devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_PORT_RESPONDING:
		return ConditionPortResponding, true
	case devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_TELEMETRY_REPORTING:
		return ConditionTelemetryReporting, true
	case devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_POLICY_APPLIED:
		return ConditionPolicyApplied, true
	case devicev1.DecoyConditionType_DECOY_CONDITION_TYPE_VERSION_MATCHES_DESIRED:
		return ConditionVersionMatchesDesired, true
	default:
		return "", false
	}
}

func statusFromWire(value devicev1.HealthConditionStatus) (Status, bool) {
	switch value {
	case devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_TRUE:
		return StatusTrue, true
	case devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_FALSE:
		return StatusFalse, true
	case devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNKNOWN:
		return StatusUnknown, true
	default:
		return "", false
	}
}

var _ devicechannel.DecoyHandler = (*ChannelHandler)(nil)
