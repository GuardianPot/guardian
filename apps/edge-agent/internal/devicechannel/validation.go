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
		len(desired.Zones) > 200 || len(desired.PlaceholderDecoys) > 64 {
		return errors.New("desired state is invalid")
	}
	zones := make(map[string]struct{}, len(desired.Zones))
	previous := ""
	for _, zone := range desired.Zones {
		if zone == nil || !validUUID(zone.ZoneId) || !validText(zone.DisplayName, 512) ||
			zone.SourceRevision == 0 || !validPrivateIPv4Prefix(zone.Cidr) || (previous != "" && zone.ZoneId <= previous) {
			return errors.New("desired-state zone is invalid")
		}
		previous = zone.ZoneId
		zones[zone.ZoneId] = struct{}{}
	}
	previous = ""
	for _, decoy := range desired.PlaceholderDecoys {
		if decoy == nil || !validUUID(decoy.ObjectId) || !validUUID(decoy.ZoneId) ||
			!validText(decoy.DisplayName, 512) || (previous != "" && decoy.ObjectId <= previous) {
			return errors.New("placeholder desired object is invalid")
		}
		if _, ok := zones[decoy.ZoneId]; !ok {
			return errors.New("placeholder desired object references an unknown zone")
		}
		previous = decoy.ObjectId
	}
	return nil
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
	if ack == nil || !validUUIDv7(ack.MessageId) || (ack.Kind != devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_OBSERVED_STATE && ack.Kind != devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_HEALTH_REPORT) {
		return errors.New("acknowledgement is invalid")
	}
	return nil
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
