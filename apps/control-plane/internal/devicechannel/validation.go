package devicechannel

import (
	"errors"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"

	devicev1 "github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel/gen/guardian/device/v1"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/health"
	"google.golang.org/protobuf/proto"
)

const (
	ProtocolMajor        = 1
	ProtocolMinor        = 0
	MaxAgentVersionBytes = 64
	MaxHealthReportBytes = 16 << 10
)

func validateHello(hello *devicev1.EdgeHello) error {
	if hello == nil || hello.Protocol == nil || hello.Protocol.Major == 0 {
		return errors.New("hello protocol is required")
	}
	if !validBoundedText(hello.AgentVersion, MaxAgentVersionBytes) {
		return errors.New("agent version is invalid")
	}
	return nil
}

func protocolCompatible(version *devicev1.ProtocolVersion) bool {
	return version != nil && version.Major == ProtocolMajor && version.Minor <= ProtocolMinor
}

func validateDesiredState(desired *devicev1.DesiredStateSnapshot, deviceID string) error {
	if desired == nil || desired.Revision == 0 || !validUUIDv7(desired.MessageId) || desired.EdgeConfiguration == nil ||
		desired.EdgeConfiguration.DeviceId != deviceID || !validUUID(desired.EdgeConfiguration.EnvironmentId) ||
		len(desired.Zones) > 200 || len(desired.PlaceholderDecoys) > 64 {
		return errors.New("desired state is invalid")
	}
	zones := make(map[string]struct{}, len(desired.Zones))
	previous := ""
	for _, zone := range desired.Zones {
		if zone == nil || !validUUID(zone.ZoneId) || !validBoundedText(zone.DisplayName, 512) ||
			zone.SourceRevision == 0 || !validPrivateIPv4Prefix(zone.Cidr) || (previous != "" && zone.ZoneId <= previous) {
			return errors.New("desired-state zone is invalid")
		}
		previous = zone.ZoneId
		zones[zone.ZoneId] = struct{}{}
	}
	previous = ""
	for _, decoy := range desired.PlaceholderDecoys {
		if decoy == nil || !validUUID(decoy.ObjectId) || !validUUID(decoy.ZoneId) ||
			!validBoundedText(decoy.DisplayName, 512) || (previous != "" && decoy.ObjectId <= previous) {
			return errors.New("placeholder desired object is invalid")
		}
		if _, ok := zones[decoy.ZoneId]; !ok {
			return errors.New("placeholder desired object references an unknown zone")
		}
		previous = decoy.ObjectId
	}
	return nil
}

func validateObservedState(observed *devicev1.ObservedState) error {
	if observed == nil || !validUUIDv7(observed.MessageId) || observed.DesiredRevision == 0 ||
		observed.ObservedRevision > observed.DesiredRevision || observed.LastGoodRevision > observed.ObservedRevision ||
		!validReconciliationCondition(observed.Condition) {
		return errors.New("observed state is invalid")
	}
	if observed.Condition.Status == devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_CONVERGED &&
		(observed.ObservedRevision != observed.DesiredRevision || observed.LastGoodRevision != observed.ObservedRevision) {
		return errors.New("converged observed state is inconsistent")
	}
	return nil
}

func validReconciliationCondition(condition *devicev1.ReconciliationCondition) bool {
	if condition == nil || condition.LastTransitionTime == nil || condition.LastTransitionTime.CheckValid() != nil ||
		!validCode(condition.ReasonCode) || condition.AttemptCount > 6 {
		return false
	}
	switch condition.Status {
	case devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_PENDING,
		devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_CONVERGED,
		devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_FAILED:
		return condition.RetryAt == nil
	case devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_RETRYING:
		return condition.RetryAt != nil && condition.RetryAt.CheckValid() == nil
	default:
		return false
	}
}

func validateAcknowledgement(ack *devicev1.Acknowledgement) error {
	if ack == nil || !validUUIDv7(ack.MessageId) || ack.Kind != devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_DESIRED_STATE {
		return errors.New("acknowledgement is invalid")
	}
	return nil
}

func validateHealthReport(report *devicev1.HealthReport) error {
	if report == nil || proto.Size(report) > MaxHealthReportBytes || report.SchemaVersion != health.SchemaVersion || report.Sequence == 0 || !validUUIDv7(report.ReportId) {
		return errors.New("health report envelope is invalid")
	}
	if err := report.ObservedAt.CheckValid(); err != nil {
		return errors.New("health report observed timestamp is invalid")
	}
	observedAt := report.ObservedAt.AsTime()
	if err := health.ValidateTimestamp(observedAt); err != nil || len(report.Conditions) != 8 {
		return errors.New("health report shape is invalid")
	}
	types := health.ConditionTypes()
	for index, condition := range report.Conditions {
		if condition == nil || int(condition.Type) != index+1 || string(types[index]) != healthTypeName(condition.Type) || condition.LastTransitionTime == nil {
			return errors.New("health condition order is invalid")
		}
		if err := condition.LastTransitionTime.CheckValid(); err != nil {
			return errors.New("health transition timestamp is invalid")
		}
		transitionAt := condition.LastTransitionTime.AsTime()
		if transitionAt.After(observedAt) {
			return errors.New("health transition is after observation")
		}
		converted := health.Condition{
			Type:               types[index],
			Status:             healthStatus(condition.Status),
			Reason:             condition.Reason,
			Message:            condition.Message,
			LastTransitionTime: transitionAt,
		}
		if condition.ObservedRevision != nil {
			revision := condition.GetObservedRevision()
			converted.ObservedRevision = &revision
		}
		if err := converted.Validate(); err != nil {
			return errors.New("health condition is invalid")
		}
	}
	return nil
}

func healthTypeName(value devicev1.HealthConditionType) string {
	switch value {
	case devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_EDGE_CONNECTED:
		return string(health.TypeEdgeConnected)
	case devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_DEVICE_CERTIFICATE_READY:
		return string(health.TypeDeviceCertificateReady)
	case devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_CONFIG_CONVERGED:
		return string(health.TypeConfigConverged)
	case devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_LOCAL_DATABASE_HEALTHY:
		return string(health.TypeLocalDatabaseHealthy)
	case devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_SPOOL_HEALTHY:
		return string(health.TypeSpoolHealthy)
	case devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_CLOCK_QUALITY:
		return string(health.TypeClockQuality)
	case devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_CONTAINER_RUNTIME_REACHABLE:
		return string(health.TypeContainerRuntimeReachable)
	case devicev1.HealthConditionType_HEALTH_CONDITION_TYPE_PRIVILEGED_HELPER_REACHABLE:
		return string(health.TypePrivilegedHelperReachable)
	default:
		return ""
	}
}

func healthStatus(value devicev1.HealthConditionStatus) health.Status {
	switch value {
	case devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_TRUE:
		return health.StatusTrue
	case devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_FALSE:
		return health.StatusFalse
	case devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNKNOWN:
		return health.StatusUnknown
	default:
		return ""
	}
}

func validBoundedText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validUUIDv7(value string) bool {
	return validUUID(value) && value[14] == '7'
}

func validUUID(value string) bool {
	if len(value) != 36 || value != strings.ToLower(value) || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || !strings.ContainsRune("89ab", rune(value[19])) {
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

func validPrivateIPv4Prefix(value string) bool {
	prefix, err := netip.ParsePrefix(value)
	if err != nil || !prefix.Addr().Is4() || prefix != prefix.Masked() || prefix.String() != value {
		return false
	}
	for _, root := range []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.168.0.0/16"),
	} {
		if prefix.Bits() >= root.Bits() && root.Contains(prefix.Addr()) {
			return true
		}
	}
	return false
}

func validCode(value string) bool {
	if len(value) < 1 || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validHeartbeat(heartbeat *devicev1.Heartbeat, now time.Time) bool {
	if heartbeat == nil || heartbeat.SentAt == nil || heartbeat.SentAt.CheckValid() != nil {
		return false
	}
	sentAt := heartbeat.SentAt.AsTime()
	return sentAt.After(time.Time{}) && sentAt.Before(now.Add(5*time.Minute))
}
