package devicechannel

import (
	"errors"
	"net/netip"
	"strings"
	"unicode/utf8"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
)

const (
	ProtocolMajor        = 1
	ProtocolMinor        = 0
	MaxAgentVersionBytes = 64
	// MaxDecoys is the P1-W6 desired-object bound, unchanged now that the
	// object is a real decoy.
	MaxDecoys = 64
	// DecoyConditionCount is the complete P2-W15 section 9.4 dimension set.
	DecoyConditionCount = 6
)

func validAgentVersion(value string) bool {
	if value == "" || len(value) > MaxAgentVersionBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
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
		if zone == nil || !validUUID(zone.ZoneId) || !validText(zone.DisplayName, 512) ||
			zone.SourceRevision == 0 || !validPrivateIPv4Prefix(zone.Cidr) || (previous != "" && zone.ZoneId <= previous) {
			return errors.New("desired-state zone is invalid")
		}
		previous = zone.ZoneId
		zones[zone.ZoneId] = zone.Cidr
	}
	previous = ""
	for _, decoy := range desired.Decoys {
		if decoy == nil || !validUUIDv7(decoy.DecoyId) || !validUUID(decoy.ZoneId) ||
			!validText(decoy.DisplayName, 512) || decoy.SourceRevision == 0 ||
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
		// The Edge repeats the placement check rather than trusting it. A
		// decoy answering outside the zone it claims is exactly the failure
		// that puts a fake service on a production address.
		if !addressInPrefix(decoy.Address, cidr) {
			return errors.New("decoy desired object is placed outside its zone")
		}
		previous = decoy.DecoyId
	}
	return nil
}

// validDecoyVocabulary rejects an unspecified enum as well as an unknown one,
// so a Control Plane that omits a field cannot have it read as a default.
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

func validPackReference(decoy *devicev1.DecoyDesiredObject) bool {
	if !validPackName(decoy.Pack) || !validPackVersion(decoy.PackVersion) {
		return false
	}
	if decoy.PackDigest == "" {
		return true
	}
	if len(decoy.PackDigest) != len("sha256:")+64 || !strings.HasPrefix(decoy.PackDigest, "sha256:") {
		return false
	}
	for _, character := range decoy.PackDigest[len("sha256:"):] {
		if (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') {
			continue
		}
		return false
	}
	return true
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

func addressInPrefix(address, cidr string) bool {
	parsed, err := netip.ParseAddr(address)
	if err != nil || !parsed.Is4() || parsed.String() != address {
		return false
	}
	prefix, err := netip.ParsePrefix(cidr)
	return err == nil && prefix.Contains(parsed)
}

// validateDecoyStateReport bounds what this Edge is willing to publish about
// its own decoys. The Control Plane validates it again; a peer never gets to
// decide what is well formed.
func validateDecoyStateReport(report *devicev1.DecoyStateReport) error {
	if report == nil || !validUUIDv7(report.ReportId) || report.ObservedAt == nil ||
		report.ObservedAt.CheckValid() != nil || len(report.Decoys) > MaxDecoys {
		return errors.New("decoy state report envelope is invalid")
	}
	observedAt := report.ObservedAt.AsTime()
	seen := make(map[string]struct{}, len(report.Decoys))
	for _, observation := range report.Decoys {
		if observation == nil || !validUUIDv7(observation.DecoyId) {
			return errors.New("decoy observation identity is invalid")
		}
		if _, duplicate := seen[observation.DecoyId]; duplicate {
			return errors.New("decoy observation is repeated")
		}
		seen[observation.DecoyId] = struct{}{}
		switch observation.State {
		case devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_UNKNOWN,
			devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_DEPLOYED,
			devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_DEGRADED,
			devicev1.DecoyObservedLifecycle_DECOY_OBSERVED_LIFECYCLE_ABSENT:
		default:
			return errors.New("decoy observed lifecycle is invalid")
		}
		if len(observation.Conditions) != DecoyConditionCount {
			return errors.New("decoy conditions must be the complete ordered set")
		}
		for index, condition := range observation.Conditions {
			if condition == nil || int(condition.Type) != index+1 || condition.LastTransitionTime == nil ||
				condition.LastTransitionTime.CheckValid() != nil ||
				condition.LastTransitionTime.AsTime().After(observedAt) {
				return errors.New("decoy condition is invalid")
			}
			switch condition.Status {
			case devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_TRUE,
				devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_FALSE,
				devicev1.HealthConditionStatus_HEALTH_CONDITION_STATUS_UNKNOWN:
			default:
				return errors.New("decoy condition status is invalid")
			}
			if !validDecoyReason(condition.Reason) || len(condition.Message) > 512 {
				return errors.New("decoy condition reason or message is invalid")
			}
		}
	}
	return nil
}

func validDecoyReason(value string) bool {
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

func validateDesiredEnvelope(desired *devicev1.DesiredStateSnapshot) error {
	if desired == nil || desired.Revision == 0 || !validUUIDv7(desired.MessageId) {
		return errors.New("desired-state envelope is invalid")
	}
	return nil
}

func validObservedCondition(condition *devicev1.ReconciliationCondition) bool {
	if condition == nil || condition.LastTransitionTime == nil || condition.LastTransitionTime.CheckValid() != nil ||
		!validReasonCode(condition.ReasonCode) || condition.AttemptCount > 6 {
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

func validReasonCode(value string) bool {
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

func validateAcknowledgement(ack *devicev1.Acknowledgement) error {
	if ack == nil || !validUUIDv7(ack.MessageId) {
		return errors.New("acknowledgement is invalid")
	}
	switch ack.Kind {
	case devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_OBSERVED_STATE,
		devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_HEALTH_REPORT,
		devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_DECOY_STATE_REPORT:
		return nil
	default:
		return errors.New("acknowledgement is invalid")
	}
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

func validText(value string, maximum int) bool {
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
