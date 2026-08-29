package devicechannel

import (
	"context"
	"errors"
	"fmt"

	devicev1 "github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel/gen/guardian/device/v1"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/health"
)

type HealthService interface {
	Ingest(context.Context, string, health.Report) error
	Disconnected(context.Context, string) error
}

type healthChannelHandler struct {
	service HealthService
}

func NewHealthChannelHandler(service HealthService) (HealthHandler, error) {
	if service == nil {
		return nil, errors.New("health service is required")
	}
	return &healthChannelHandler{service: service}, nil
}

func (h *healthChannelHandler) HealthReport(ctx context.Context, identity DeviceIdentity, input *devicev1.HealthReport) error {
	report, err := healthReportFromProto(input)
	if err != nil {
		return err
	}
	return h.service.Ingest(ctx, identity.DeviceID, report)
}

func (h *healthChannelHandler) ChannelClosed(ctx context.Context, identity DeviceIdentity) error {
	return h.service.Disconnected(ctx, identity.DeviceID)
}

func healthReportFromProto(input *devicev1.HealthReport) (health.Report, error) {
	if input == nil || input.ObservedAt == nil {
		return health.Report{}, health.ErrInvalidReport
	}
	report := health.Report{
		SchemaVersion: input.SchemaVersion,
		ReportID:      input.ReportId,
		Sequence:      input.Sequence,
		ObservedAt:    input.ObservedAt.AsTime(),
		Conditions:    make([]health.Condition, 0, len(input.Conditions)),
	}
	for _, inputCondition := range input.Conditions {
		if inputCondition == nil || inputCondition.LastTransitionTime == nil {
			return health.Report{}, health.ErrInvalidCondition
		}
		conditionType, err := healthTypeFromProto(inputCondition.Type)
		if err != nil {
			return health.Report{}, err
		}
		conditionStatus, err := healthStatusFromProto(inputCondition.Status)
		if err != nil {
			return health.Report{}, err
		}
		condition := health.Condition{
			Type: conditionType, Status: conditionStatus, Reason: inputCondition.Reason,
			Message: inputCondition.Message, LastTransitionTime: inputCondition.LastTransitionTime.AsTime(),
		}
		if inputCondition.ObservedRevision != nil {
			value := inputCondition.GetObservedRevision()
			condition.ObservedRevision = &value
		}
		report.Conditions = append(report.Conditions, condition)
	}
	if err := report.Validate(); err != nil {
		return health.Report{}, err
	}
	return report, nil
}

func healthTypeFromProto(value devicev1.HealthConditionType) (health.ConditionType, error) {
	types := map[devicev1.HealthConditionType]health.ConditionType{
		devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_EDGE_CONNECTED:              health.TypeEdgeConnected,
		devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_DEVICE_CERTIFICATE_READY:    health.TypeDeviceCertificateReady,
		devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_CONFIG_CONVERGED:            health.TypeConfigConverged,
		devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_LOCAL_DATABASE_HEALTHY:      health.TypeLocalDatabaseHealthy,
		devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_SPOOL_HEALTHY:               health.TypeSpoolHealthy,
		devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_CLOCK_QUALITY:               health.TypeClockQuality,
		devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_CONTAINER_RUNTIME_REACHABLE: health.TypeContainerRuntimeReachable,
		devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_PRIVILEGED_HELPER_REACHABLE: health.TypePrivilegedHelperReachable,
	}
	conditionType, ok := types[value]
	if !ok {
		return "", fmt.Errorf("%w: protobuf type", health.ErrInvalidConditionType)
	}
	return conditionType, nil
}

func healthStatusFromProto(value devicev1.HealthConditionStatus) (health.Status, error) {
	statuses := map[devicev1.HealthConditionStatus]health.Status{
		devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_TRUE:    health.StatusTrue,
		devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_FALSE:   health.StatusFalse,
		devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNKNOWN: health.StatusUnknown,
	}
	conditionStatus, ok := statuses[value]
	if !ok {
		return "", fmt.Errorf("%w: protobuf status", health.ErrInvalidConditionStatus)
	}
	return conditionStatus, nil
}

var _ HealthHandler = (*healthChannelHandler)(nil)
var _ HealthDisconnectHandler = (*healthChannelHandler)(nil)
