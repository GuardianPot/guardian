package devicechannel

import (
	"errors"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/health"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const MaxHealthReportBytes = 16 << 10

func healthReportFrame(report health.Report) (*devicev1.ConnectRequest, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	conditions := make([]*devicev1.HealthCondition, 0, len(report.Conditions))
	for _, condition := range report.Conditions {
		converted := &devicev1.HealthCondition{
			Type:               healthType(condition.Type),
			Status:             healthStatus(condition.Status),
			Reason:             condition.Reason,
			Message:            condition.Message,
			LastTransitionTime: timestamppb.New(condition.LastTransitionTime),
		}
		if condition.ObservedRevision != nil {
			revision := *condition.ObservedRevision
			converted.ObservedRevision = &revision
		}
		conditions = append(conditions, converted)
	}
	frame := &devicev1.ConnectRequest{Payload: &devicev1.ConnectRequest_HealthReport{HealthReport: &devicev1.HealthReport{
		SchemaVersion: report.SchemaVersion,
		ReportId:      report.ReportID,
		Sequence:      report.Sequence,
		ObservedAt:    timestamppb.New(report.ObservedAt),
		Conditions:    conditions,
	}}}
	if proto.Size(frame.GetHealthReport()) > MaxHealthReportBytes {
		return nil, errors.New("health report exceeds channel limit")
	}
	return frame, nil
}

func healthType(value health.ConditionType) devicev1.HealthConditionType {
	switch value {
	case health.TypeEdgeConnected:
		return devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_EDGE_CONNECTED
	case health.TypeDeviceCertificateReady:
		return devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_DEVICE_CERTIFICATE_READY
	case health.TypeConfigConverged:
		return devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_CONFIG_CONVERGED
	case health.TypeLocalDatabaseHealthy:
		return devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_LOCAL_DATABASE_HEALTHY
	case health.TypeSpoolHealthy:
		return devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_SPOOL_HEALTHY
	case health.TypeClockQuality:
		return devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_CLOCK_QUALITY
	case health.TypeContainerRuntimeReachable:
		return devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_CONTAINER_RUNTIME_REACHABLE
	case health.TypePrivilegedHelperReachable:
		return devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_PRIVILEGED_HELPER_REACHABLE
	default:
		return devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_UNSPECIFIED
	}
}

func healthStatus(value health.Status) devicev1.HealthConditionStatus {
	switch value {
	case health.StatusTrue:
		return devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_TRUE
	case health.StatusFalse:
		return devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_FALSE
	case health.StatusUnknown:
		return devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNKNOWN
	default:
		return devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNSPECIFIED
	}
}
