package devicechannel

import (
	"errors"
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

func validateDesiredState(desired *devicev1.DesiredStateSnapshot) error {
	if desired == nil || desired.Revision == 0 || !validUUIDv7(desired.MessageId) {
		return errors.New("desired state is invalid")
	}
	return nil
}

func validateAcknowledgement(ack *devicev1.Acknowledgement) error {
	if ack == nil || !validUUIDv7(ack.MessageId) || (ack.Kind != devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_OBSERVED_STATE && ack.Kind != devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_HEALTH_REPORT) {
		return errors.New("acknowledgement is invalid")
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
