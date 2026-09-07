// Package reconciliation owns revisioned desired/observed state semantics.
package reconciliation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxZones = 200
	// MaxDecoys keeps the bound P1-W6 reserved for placeholder objects, so the
	// device-channel message-size guarantee is unchanged now that the object
	// carries a real decoy.
	MaxDecoys              = 64
	MaxDisplayNameBytes    = 512
	MaxReasonCodeBytes     = 64
	MaxReconciliationTries = 6
	MaxPackNameBytes       = 64
	MaxPackVersionBytes    = 32
)

var (
	ErrInvalidSnapshot = errors.New("desired-state snapshot is invalid")
	ErrInvalidObserved = errors.New("observed state is invalid")
	ErrNotFound        = errors.New("desired state was not found")
)

type EdgeConfiguration struct {
	DeviceID      string `json:"device_id"`
	EnvironmentID string `json:"environment_id"`
}

type Zone struct {
	ZoneID         string `json:"zone_id"`
	DisplayName    string `json:"display_name"`
	CIDR           string `json:"cidr"`
	SourceRevision uint64 `json:"source_revision"`
}

// Decoy is one decoy the Control Plane asked this Edge to place. It has no
// key, certificate, credential, image reference, command, mount, or capability
// field, and the Edge resolves runtime detail from the pack the (Pack,
// PackVersion) pair names rather than from anything in this struct.
type Decoy struct {
	DecoyID          string `json:"decoy_id"`
	ZoneID           string `json:"zone_id"`
	DisplayName      string `json:"display_name"`
	Family           string `json:"family"`
	Persona          string `json:"persona"`
	InteractionLevel string `json:"interaction_level"`
	Address          string `json:"address"`
	Pack             string `json:"pack"`
	PackVersion      string `json:"pack_version"`
	PackDigest       string `json:"pack_digest"`
	DesiredState     string `json:"desired_state"`
	SourceRevision   uint64 `json:"source_revision"`
}

type Snapshot struct {
	MessageID         string            `json:"message_id"`
	Revision          uint64            `json:"revision"`
	EdgeConfiguration EdgeConfiguration `json:"edge_configuration"`
	Zones             []Zone            `json:"zones"`
	Decoys            []Decoy           `json:"decoys"`
}

type ConditionStatus string

const (
	ConditionPending   ConditionStatus = "pending"
	ConditionConverged ConditionStatus = "converged"
	ConditionRetrying  ConditionStatus = "retrying"
	ConditionFailed    ConditionStatus = "failed"
)

type Condition struct {
	Status             ConditionStatus `json:"status"`
	ReasonCode         string          `json:"reason_code"`
	AttemptCount       uint32          `json:"attempt_count"`
	RetryAt            *time.Time      `json:"retry_at,omitempty"`
	LastTransitionTime time.Time       `json:"last_transition_time"`
}

type ObservedState struct {
	MessageID        string    `json:"message_id"`
	DesiredRevision  uint64    `json:"desired_revision"`
	ObservedRevision uint64    `json:"observed_revision"`
	LastGoodRevision uint64    `json:"last_good_revision"`
	Condition        Condition `json:"condition"`
}

type Acknowledgement struct {
	MessageID string
	Revision  uint64
}

func ValidateSnapshot(snapshot Snapshot) error {
	if !validUUIDv7(snapshot.MessageID) || snapshot.Revision == 0 ||
		!validUUIDv7(snapshot.EdgeConfiguration.DeviceID) ||
		!validUUID(snapshot.EdgeConfiguration.EnvironmentID) ||
		len(snapshot.Zones) > MaxZones || len(snapshot.Decoys) > MaxDecoys {
		return ErrInvalidSnapshot
	}
	zoneCIDRs := make(map[string]string, len(snapshot.Zones))
	previous := ""
	for _, zone := range snapshot.Zones {
		if !validUUID(zone.ZoneID) || !validDisplayName(zone.DisplayName) ||
			zone.SourceRevision == 0 || !validPrivateIPv4Prefix(zone.CIDR) ||
			(previous != "" && zone.ZoneID <= previous) {
			return ErrInvalidSnapshot
		}
		previous = zone.ZoneID
		zoneCIDRs[zone.ZoneID] = zone.CIDR
	}
	previous = ""
	for _, decoy := range snapshot.Decoys {
		if !validUUIDv7(decoy.DecoyID) || !validUUID(decoy.ZoneID) ||
			!validDisplayName(decoy.DisplayName) || decoy.SourceRevision == 0 ||
			(previous != "" && decoy.DecoyID <= previous) {
			return ErrInvalidSnapshot
		}
		if !validDecoyFamily(decoy.Family) || !validDecoyPersona(decoy.Persona) ||
			!validInteractionLevel(decoy.Family, decoy.InteractionLevel) ||
			!validDesiredDecoyState(decoy.DesiredState) ||
			!validPackName(decoy.Pack) || !validPackVersion(decoy.PackVersion) ||
			!validPackDigest(decoy.PackDigest) {
			return ErrInvalidSnapshot
		}
		cidr, ok := zoneCIDRs[decoy.ZoneID]
		if !ok || !addressInPrefix(decoy.Address, cidr) {
			return ErrInvalidSnapshot
		}
		previous = decoy.DecoyID
	}
	return nil
}

func validDecoyFamily(value string) bool {
	switch value {
	case "ssh", "http", "postgres", "smb":
		return true
	default:
		return false
	}
}

func validDecoyPersona(value string) bool {
	switch value {
	case "linux_admin_server", "internal_admin_web_app", "database_server", "windows_file_service_host":
		return true
	default:
		return false
	}
}

// validInteractionLevel enforces INT-01 on the wire as well as in the domain,
// so a decoy cannot arrive at an Edge claiming an interaction level its family
// is not approved for.
func validInteractionLevel(family, level string) bool {
	switch family {
	case "ssh":
		return level == "medium"
	case "http", "smb":
		return level == "low"
	case "postgres":
		return level == "low" || level == "medium"
	default:
		return false
	}
}

// validDesiredDecoyState omits `removed`: a removed decoy leaves the snapshot
// rather than travelling to the Edge as a tombstone.
func validDesiredDecoyState(value string) bool {
	return value == "deployed" || value == "disabled"
}

func validPackName(value string) bool {
	if len(value) < 1 || len(value) > MaxPackNameBytes {
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
	if len(value) < 5 || len(value) > MaxPackVersionBytes {
		return false
	}
	segments := strings.Split(value, ".")
	if len(segments) != 3 {
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

// validPackDigest accepts an empty digest. P2-W4 has not defined a manifest to
// hash, so the honest value today is "no digest", not an invented one.
func validPackDigest(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	decoded, err := hex.DecodeString(value[len("sha256:"):])
	return err == nil && len(decoded) == sha256.Size
}

func addressInPrefix(address, cidr string) bool {
	parsed, err := netip.ParseAddr(address)
	if err != nil || !parsed.Is4() || parsed.String() != address {
		return false
	}
	prefix, err := netip.ParsePrefix(cidr)
	return err == nil && prefix.Contains(parsed)
}

func ValidateObserved(observed ObservedState) error {
	if !validUUIDv7(observed.MessageID) || observed.DesiredRevision == 0 ||
		observed.ObservedRevision > observed.DesiredRevision ||
		observed.LastGoodRevision > observed.ObservedRevision ||
		!validCondition(observed.Condition) {
		return ErrInvalidObserved
	}
	if observed.Condition.Status == ConditionConverged &&
		(observed.ObservedRevision != observed.DesiredRevision || observed.LastGoodRevision != observed.ObservedRevision) {
		return ErrInvalidObserved
	}
	return nil
}

func MarshalSnapshot(snapshot Snapshot) ([]byte, error) {
	if err := ValidateSnapshot(snapshot); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal desired-state snapshot: %w", err)
	}
	return payload, nil
}

func ParseSnapshot(payload []byte) (Snapshot, error) {
	var snapshot Snapshot
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: decode snapshot", ErrInvalidSnapshot)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Snapshot{}, fmt.Errorf("%w: trailing snapshot content", ErrInvalidSnapshot)
	}
	if err := ValidateSnapshot(snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func ContentDigest(snapshot Snapshot) ([sha256.Size]byte, error) {
	content := struct {
		EdgeConfiguration EdgeConfiguration `json:"edge_configuration"`
		Zones             []Zone            `json:"zones"`
		Decoys            []Decoy           `json:"decoys"`
	}{snapshot.EdgeConfiguration, snapshot.Zones, snapshot.Decoys}
	payload, err := json.Marshal(content)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("marshal desired-state content: %w", err)
	}
	return sha256.Sum256(payload), nil
}

func DigestHex(digest [sha256.Size]byte) string { return hex.EncodeToString(digest[:]) }

func validCondition(condition Condition) bool {
	switch condition.Status {
	case ConditionPending, ConditionConverged, ConditionRetrying, ConditionFailed:
	default:
		return false
	}
	if !validCode(condition.ReasonCode) || condition.AttemptCount > MaxReconciliationTries || condition.LastTransitionTime.IsZero() {
		return false
	}
	if condition.Status == ConditionRetrying {
		return condition.RetryAt != nil && !condition.RetryAt.IsZero()
	}
	return condition.RetryAt == nil
}

func validCode(value string) bool {
	if len(value) < 1 || len(value) > MaxReasonCodeBytes || strings.TrimSpace(value) != value {
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

func validDisplayName(value string) bool {
	if !utf8.ValidString(value) || len(value) < 1 || len(value) > MaxDisplayNameBytes || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
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
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
	} {
		if prefix.Bits() >= root.Bits() && root.Contains(prefix.Addr()) {
			return true
		}
	}
	return false
}

func validUUID(value string) bool {
	if len(value) != 36 || value != strings.ToLower(value) || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	decoded, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil && len(decoded) == 16 && decoded[8]&0xc0 == 0x80
}

func validUUIDv7(value string) bool { return validUUID(value) && value[14] == '7' }
