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
	MaxZones               = 200
	MaxPlaceholderDecoys   = 64
	MaxDisplayNameBytes    = 512
	MaxReasonCodeBytes     = 64
	MaxReconciliationTries = 6
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

type PlaceholderDecoy struct {
	ObjectID    string `json:"object_id"`
	ZoneID      string `json:"zone_id"`
	DisplayName string `json:"display_name"`
}

type Snapshot struct {
	MessageID         string             `json:"message_id"`
	Revision          uint64             `json:"revision"`
	EdgeConfiguration EdgeConfiguration  `json:"edge_configuration"`
	Zones             []Zone             `json:"zones"`
	PlaceholderDecoys []PlaceholderDecoy `json:"placeholder_decoys"`
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
		len(snapshot.Zones) > MaxZones || len(snapshot.PlaceholderDecoys) > MaxPlaceholderDecoys {
		return ErrInvalidSnapshot
	}
	zoneIDs := make(map[string]struct{}, len(snapshot.Zones))
	previous := ""
	for _, zone := range snapshot.Zones {
		if !validUUID(zone.ZoneID) || !validDisplayName(zone.DisplayName) ||
			zone.SourceRevision == 0 || !validPrivateIPv4Prefix(zone.CIDR) ||
			(previous != "" && zone.ZoneID <= previous) {
			return ErrInvalidSnapshot
		}
		previous = zone.ZoneID
		zoneIDs[zone.ZoneID] = struct{}{}
	}
	previous = ""
	for _, decoy := range snapshot.PlaceholderDecoys {
		if !validUUID(decoy.ObjectID) || !validUUID(decoy.ZoneID) ||
			!validDisplayName(decoy.DisplayName) ||
			(previous != "" && decoy.ObjectID <= previous) {
			return ErrInvalidSnapshot
		}
		if _, ok := zoneIDs[decoy.ZoneID]; !ok {
			return ErrInvalidSnapshot
		}
		previous = decoy.ObjectID
	}
	return nil
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
		EdgeConfiguration EdgeConfiguration  `json:"edge_configuration"`
		Zones             []Zone             `json:"zones"`
		PlaceholderDecoys []PlaceholderDecoy `json:"placeholder_decoys"`
	}{snapshot.EdgeConfiguration, snapshot.Zones, snapshot.PlaceholderDecoys}
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
