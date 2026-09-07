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
	// MaxDecoys is the P1-W6 desired-object bound, unchanged now that the
	// object is a real decoy.
	MaxDecoys = 64
	// MaxDecoyReportBytes bounds one encoded Edge decoy report, matching the
	// health report bound so one hostile peer cannot spend more on decoys.
	MaxDecoyReportBytes = 16 << 10
	// DecoyConditionCount is the complete P2-W15 section 9.4 dimension set. A
	// partial report is rejected rather than silently completed.
	DecoyConditionCount = 6
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
		len(desired.Zones) > 200 || len(desired.Decoys) > MaxDecoys {
		return errors.New("desired state is invalid")
	}
	zones := make(map[string]string, len(desired.Zones))
	previous := ""
	for _, zone := range desired.Zones {
		if zone == nil || !validUUID(zone.ZoneId) || !validBoundedText(zone.DisplayName, 512) ||
			zone.SourceRevision == 0 || !validPrivateIPv4Prefix(zone.Cidr) || (previous != "" && zone.ZoneId <= previous) {
			return errors.New("desired-state zone is invalid")
		}
		previous = zone.ZoneId
		zones[zone.ZoneId] = zone.Cidr
	}
	previous = ""
	for _, decoy := range desired.Decoys {
		if decoy == nil || !validUUIDv7(decoy.DecoyId) || !validUUID(decoy.ZoneId) ||
			!validBoundedText(decoy.DisplayName, 512) || decoy.SourceRevision == 0 ||
			(previous != "" && decoy.DecoyId <= previous) {
			return errors.New("decoy desired object is invalid")
		}
		if !validDecoyVocabulary(decoy) || !validPackReference(decoy) {
			return errors.New("decoy desired object is outside the closed vocabulary")
		}
		cidr, ok := zones[decoy.ZoneId]
		if !ok {
			return errors.New("decoy desired object references an unknown zone")
		}
		if !addressInPrefix(decoy.Address, cidr) {
			return errors.New("decoy desired object is placed outside its zone")
		}
		previous = decoy.DecoyId
	}
	return nil
}

// validDecoyVocabulary rejects an unspecified enum value as well as an unknown
// one, so a peer that omits a field cannot have it read as a default.
// interaction level is checked against family because INT-01 pairs them.
func validDecoyVocabulary(decoy *devicev1.DecoyDesiredObject) bool {
	switch decoy.DesiredState {
	case devicev1.DecoyDesiredLifecycle_DECOY_DESIRED_LIFECYCLE_DEPLOYED,
		devicev1.DecoyDesiredLifecycle_DECOY_DESIRED_LIFECYCLE_DISABLED:
	default:
		return false
	}
	switch decoy.Persona {
	case devicev1.DecoyPersona_DECOY_PERSONA_LINUX_ADMIN_SERVER,
		devicev1.DecoyPersona_DECOY_PERSONA_INTERNAL_ADMIN_WEB_APP,
		devicev1.DecoyPersona_DECOY_PERSONA_DATABASE_SERVER,
		devicev1.DecoyPersona_DECOY_PERSONA_WINDOWS_FILE_SERVICE_HOST:
	default:
		return false
	}
	switch decoy.Family {
	case devicev1.DecoyFamily_DECOY_FAMILY_SSH:
		return decoy.InteractionLevel == devicev1.DecoyInteractionLevel_DECOY_INTERACTION_LEVEL_MEDIUM
	case devicev1.DecoyFamily_DECOY_FAMILY_HTTP, devicev1.DecoyFamily_DECOY_FAMILY_SMB:
		return decoy.InteractionLevel == devicev1.DecoyInteractionLevel_DECOY_INTERACTION_LEVEL_LOW
	case devicev1.DecoyFamily_DECOY_FAMILY_POSTGRES:
		return decoy.InteractionLevel == devicev1.DecoyInteractionLevel_DECOY_INTERACTION_LEVEL_LOW ||
			decoy.InteractionLevel == devicev1.DecoyInteractionLevel_DECOY_INTERACTION_LEVEL_MEDIUM
	default:
		return false
	}
}

// validPackReference bounds the opaque (pack, version, digest) triple. This
// package never parses a manifest; P2-W4 owns what one contains.
func validPackReference(decoy *devicev1.DecoyDesiredObject) bool {
	if !validPackName(decoy.Pack) || !validPackVersion(decoy.PackVersion) {
		return false
	}
	// An empty digest is correct until P2-W4 supplies a manifest to hash.
	if decoy.PackDigest == "" {
		return true
	}
	return len(decoy.PackDigest) == len("sha256:")+64 && strings.HasPrefix(decoy.PackDigest, "sha256:") &&
		validLowerHex(decoy.PackDigest[len("sha256:"):])
}

func validPackName(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for index, character := range value {
		if index == 0 && (character < 'a' || character > 'z') {
			return false
		}
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validPackVersion(value string) bool {
	segments := strings.Split(value, ".")
	if len(value) > 32 || len(segments) != 3 {
		return false
	}
	for _, segment := range segments {
		if len(segment) < 1 || len(segment) > 6 || (len(segment) > 1 && segment[0] == '0') {
			return false
		}
		for _, character := range segment {
			if character < '0' || character > '9' {
				return false
			}
		}
	}
	return true
}

func validLowerHex(value string) bool {
	for _, character := range value {
		if (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') {
			continue
		}
		return false
	}
	return true
}

func addressInPrefix(address, cidr string) bool {
	parsed, err := netip.ParseAddr(address)
	if err != nil || !parsed.Is4() || parsed.String() != address {
		return false
	}
	prefix, err := netip.ParsePrefix(cidr)
	return err == nil && prefix.Contains(parsed)
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

// validateDecoyStateReport is the trust boundary for everything an Edge says
// about its decoys. It never accepts DECOY_OBSERVED_LIFECYCLE_UNSPECIFIED, so
// an omitted state cannot be read as a default, and it never accepts an
// `unmanaged` claim: SEC-06 is a Control Plane projection from device state,
// and the device that has lost management is precisely the one whose
// self-report about that cannot be trusted.
func validateDecoyStateReport(report *devicev1.DecoyStateReport) error {
	if report == nil || proto.Size(report) > MaxDecoyReportBytes || !validUUIDv7(report.ReportId) ||
		len(report.Decoys) > MaxDecoys {
		return errors.New("decoy report envelope is invalid")
	}
	if err := report.ObservedAt.CheckValid(); err != nil {
		return errors.New("decoy report observation timestamp is invalid")
	}
	observedAt := report.ObservedAt.AsTime()
	if err := health.ValidateTimestamp(observedAt); err != nil {
		return errors.New("decoy report observation timestamp is out of range")
	}
	seen := make(map[string]struct{}, len(report.Decoys))
	for _, observation := range report.Decoys {
		if observation == nil || !validUUIDv7(observation.DecoyId) {
			return errors.New("decoy observation identity is invalid")
		}
		if _, duplicate := seen[observation.DecoyId]; duplicate {
			return errors.New("decoy observation is repeated")
		}
		seen[observation.DecoyId] = struct{}{}
		if err := validateDecoyObservation(observation, observedAt); err != nil {
			return err
		}
	}
	return nil
}

func validateDecoyObservation(observation *devicev1.DecoyObservation, observedAt time.Time) error {
	switch observation.State {
	case devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_UNKNOWN,
		devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_DEPLOYED,
		devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_DEGRADED,
		devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_ABSENT:
	default:
		return errors.New("decoy observed lifecycle is invalid")
	}
	if observation.LastInteractionAt != nil {
		if err := observation.LastInteractionAt.CheckValid(); err != nil {
			return errors.New("decoy interaction timestamp is invalid")
		}
		if observation.LastInteractionAt.AsTime().After(observedAt) {
			return errors.New("decoy interaction is after the observation")
		}
	}
	if len(observation.Conditions) != DecoyConditionCount {
		return errors.New("decoy conditions must be the complete ordered set")
	}
	for index, condition := range observation.Conditions {
		if condition == nil || int(condition.Type) != index+1 || condition.LastTransitionTime == nil {
			return errors.New("decoy condition order is invalid")
		}
		switch condition.Status {
		case devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_TRUE,
			devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_FALSE,
			devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNKNOWN:
		default:
			return errors.New("decoy condition status is invalid")
		}
		if !validReasonIdentifier(condition.Reason) || len(condition.Message) > 512 ||
			!utf8.ValidString(condition.Message) {
			return errors.New("decoy condition reason or message is invalid")
		}
		for _, character := range condition.Message {
			if character < 0x20 || character == 0x7f {
				return errors.New("decoy condition message contains a control character")
			}
		}
		if err := condition.LastTransitionTime.CheckValid(); err != nil {
			return errors.New("decoy condition transition timestamp is invalid")
		}
		if condition.LastTransitionTime.AsTime().After(observedAt) {
			return errors.New("decoy condition transition is after the observation")
		}
	}
	return nil
}

func validReasonIdentifier(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for index, character := range value {
		if index == 0 && (character < 'a' || character > 'z') {
			return false
		}
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' {
			continue
		}
		return false
	}
	return true
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
