package deception

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

const (
	// DefaultListLimit and MaxListLimit follow the environment and zone page
	// conventions so the console meets one pagination rule, not two.
	DefaultListLimit int32 = 50
	MaxListLimit     int32 = 200
	// MaxDecoysPerDevice is the device-channel desired-state bound. It keeps
	// the value P1-W6 reserved, so the message-size guarantee is unchanged.
	MaxDecoysPerDevice = 64
	MaxNameRunes       = 128
	MaxNameBytes       = 512
	MaxRequestIDBytes  = 128
	// MaxConditionMessageBytes bounds Edge-supplied condition detail.
	MaxConditionMessageBytes = 512
)

var (
	ErrInvalidInput       = errors.New("decoy input is invalid")
	ErrNotFound           = errors.New("decoy resource was not found")
	ErrNameConflict       = errors.New("decoy display name conflicts")
	ErrAddressConflict    = errors.New("decoy address conflicts")
	ErrAddressOutsideZone = errors.New("decoy address is outside its zone")
	ErrUnknownPack        = errors.New("decoy pack version is not in the index")
	ErrPreconditionFailed = errors.New("decoy revision precondition failed")
	// ErrDecoyBudgetExhausted is returned rather than silently dropping a decoy
	// from the desired state. An operator who asked for a decoy must never have
	// it disappear without a signal.
	ErrDecoyBudgetExhausted = errors.New("decoy budget for this environment is exhausted")
)

// Family is the closed DC-01 family set.
type Family string

const (
	FamilySSH      Family = "ssh"
	FamilyHTTP     Family = "http"
	FamilyPostgres Family = "postgres"
	FamilySMB      Family = "smb"
)

// Persona is the closed DC-11 curated persona set. There is no generative
// persona builder, so this is a token vocabulary and never free text.
type Persona string

const (
	PersonaLinuxAdminServer       Persona = "linux_admin_server"
	PersonaInternalAdminWebApp    Persona = "internal_admin_web_app"
	PersonaDatabaseServer         Persona = "database_server"
	PersonaWindowsFileServiceHost Persona = "windows_file_service_host"
)

// InteractionLevel carries INT-01. High interaction is out of the MVP and has
// no value here.
type InteractionLevel string

const (
	InteractionLow    InteractionLevel = "low"
	InteractionMedium InteractionLevel = "medium"
)

// DesiredState is what the operator asked for.
type DesiredState string

const (
	DesiredDeployed DesiredState = "deployed"
	DesiredDisabled DesiredState = "disabled"
	DesiredRemoved  DesiredState = "removed"
)

// ObservedLifecycle is what the network reported. It is never merged into
// DesiredState: the two answer different questions.
//
// ObservedUnknown is the default and the only honest value for a decoy no Edge
// has reported on. ObservedUnmanaged is projected by the Control Plane from
// device state per SEC-06; an Edge never reports it about itself.
type ObservedLifecycle string

const (
	ObservedUnknown   ObservedLifecycle = "unknown"
	ObservedDeployed  ObservedLifecycle = "deployed"
	ObservedDegraded  ObservedLifecycle = "degraded"
	ObservedAbsent    ObservedLifecycle = "absent"
	ObservedUnmanaged ObservedLifecycle = "unmanaged"
)

// ConditionType is the closed decoy health dimension set from P2-W15 section
// 9.4. They are separate from the start because P2-W14's acceptance is that
// killing the process, removing the address, and breaking telemetry each yield
// a distinct degraded state, which one collapsed field cannot express.
type ConditionType string

const (
	ConditionRuntimeHealthy        ConditionType = "runtime_healthy"
	ConditionAddressApplied        ConditionType = "address_applied"
	ConditionPortResponding        ConditionType = "port_responding"
	ConditionTelemetryReporting    ConditionType = "telemetry_reporting"
	ConditionPolicyApplied         ConditionType = "policy_applied"
	ConditionVersionMatchesDesired ConditionType = "version_matches_desired"
)

var conditionTypeOrder = [...]ConditionType{
	ConditionRuntimeHealthy,
	ConditionAddressApplied,
	ConditionPortResponding,
	ConditionTelemetryReporting,
	ConditionPolicyApplied,
	ConditionVersionMatchesDesired,
}

// ConditionTypes returns the canonical condition order as a defensive copy.
func ConditionTypes() []ConditionType {
	result := make([]ConditionType, len(conditionTypeOrder))
	copy(result, conditionTypeOrder[:])
	return result
}

// Status reuses the P1-W9 tri-state. One product, one meaning for True, False,
// and Unknown.
type Status string

const (
	StatusTrue    Status = "True"
	StatusFalse   Status = "False"
	StatusUnknown Status = "Unknown"
)

// Decoy is the desired object: what an operator asked Guardian to place. It
// carries no key, certificate, credential, image reference, command, mount, or
// capability, and there is no field in which one could be stored.
type Decoy struct {
	DecoyID          string           `json:"decoy_id"`
	EnvironmentID    string           `json:"environment_id"`
	ZoneID           string           `json:"zone_id"`
	DisplayName      string           `json:"display_name"`
	Family           Family           `json:"family"`
	Persona          Persona          `json:"persona"`
	InteractionLevel InteractionLevel `json:"interaction_level"`
	Address          string           `json:"address"`
	Pack             string           `json:"pack"`
	PackVersion      string           `json:"pack_version"`
	// PackDigest is empty until P2-W4 supplies a manifest to hash. An empty
	// digest is reported as null rather than as a fabricated value.
	PackDigest   string       `json:"pack_digest"`
	DesiredState DesiredState `json:"desired_state"`
	Revision     int64        `json:"revision"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// Condition is one bounded decoy observation following the P1-W9 model.
type Condition struct {
	Type               ConditionType `json:"type"`
	Status             Status        `json:"status"`
	Reason             string        `json:"reason"`
	Message            string        `json:"message"`
	ObservedRevision   *uint64       `json:"observed_revision,omitempty"`
	LastTransitionTime time.Time     `json:"last_transition_time"`
}

// Observation is the observed record, stored and served beside the desired
// object and never inside it.
type Observation struct {
	DecoyID           string            `json:"decoy_id"`
	State             ObservedLifecycle `json:"observed_state"`
	ReportingDeviceID string            `json:"reporting_device_id,omitempty"`
	ReportedAt        *time.Time        `json:"reported_at,omitempty"`
	// LastInteractionAt is a timestamp. It is not a finding, an incident, or
	// evidence; those belong to packages that own event intelligence.
	LastInteractionAt *time.Time  `json:"last_interaction_at,omitempty"`
	DesiredRevision   *uint64     `json:"desired_revision,omitempty"`
	Conditions        []Condition `json:"conditions"`
}

// View pairs one decoy with its observed record. The two are separate fields
// at every layer, because collapsing them is how a console ends up claiming a
// decoy is running because someone once asked for it.
type View struct {
	Decoy    Decoy       `json:"decoy"`
	Observed Observation `json:"observed"`
}

// Report is one Edge's complete decoy observation set.
type Report struct {
	ReportID     string
	ObservedAt   time.Time
	DeviceID     string
	Observations []Observation
}

// NormalizedName is the display-name pair stored for conflict detection.
type NormalizedName struct {
	DisplayName string
	NameKey     string
}

// Mutation is the audited actor context for one write.
type Mutation struct {
	ActorID    string
	RequestID  string
	OccurredAt time.Time
}

// Write is a validated decoy create or update. Every field here is either a
// closed token or a bounded operator string; nothing in it can name a runtime
// artifact.
type Write struct {
	ZoneID           string
	Name             NormalizedName
	Family           Family
	Persona          Persona
	InteractionLevel InteractionLevel
	Address          string
	Pack             string
	PackVersion      string
	PackDigest       string
}

// Repository is the storage boundary. Observed state is written only through
// RecordObservations, which the device channel reaches and the REST surface
// does not.
type Repository interface {
	ListDecoys(context.Context, string, int32) ([]View, error)
	Decoy(context.Context, string, string) (View, error)
	CreateDecoy(context.Context, string, Write, Mutation) (Decoy, error)
	UpdateDecoy(context.Context, string, string, Write, int64, Mutation) (Decoy, error)
	SetDecoyDesiredState(context.Context, string, string, DesiredState, int64, Mutation) (Decoy, error)
	RemoveDecoy(context.Context, string, string, int64, Mutation) (Decoy, error)
	RecordObservations(context.Context, Report) error
}

// Valid reports whether f is in the closed DC-01 family set.
func (f Family) Valid() bool {
	switch f {
	case FamilySSH, FamilyHTTP, FamilyPostgres, FamilySMB:
		return true
	default:
		return false
	}
}

// Valid reports whether p is in the closed DC-11 persona set.
func (p Persona) Valid() bool {
	switch p {
	case PersonaLinuxAdminServer, PersonaInternalAdminWebApp, PersonaDatabaseServer, PersonaWindowsFileServiceHost:
		return true
	default:
		return false
	}
}

// Valid reports whether l is an MVP interaction level.
func (l InteractionLevel) Valid() bool {
	return l == InteractionLow || l == InteractionMedium
}

// Valid reports whether s is a desired lifecycle state.
func (s DesiredState) Valid() bool {
	switch s {
	case DesiredDeployed, DesiredDisabled, DesiredRemoved:
		return true
	default:
		return false
	}
}

// Valid reports whether s is an observed lifecycle state.
func (s ObservedLifecycle) Valid() bool {
	switch s {
	case ObservedUnknown, ObservedDeployed, ObservedDegraded, ObservedAbsent, ObservedUnmanaged:
		return true
	default:
		return false
	}
}

// Valid reports whether t is a decoy condition dimension.
func (t ConditionType) Valid() bool {
	for _, candidate := range conditionTypeOrder {
		if candidate == t {
			return true
		}
	}
	return false
}

// Valid reports whether s is one of the three condition states.
func (s Status) Valid() bool {
	return s == StatusTrue || s == StatusFalse || s == StatusUnknown
}

// InteractionLevelFor applies INT-01: SSH is medium, HTTP and SMB are low, and
// a database surface may be either. It is the pairing rule, not a preference.
func InteractionLevelFor(family Family, level InteractionLevel) error {
	if !family.Valid() || !level.Valid() {
		return fmt.Errorf("%w: family and interaction level must be in the closed vocabularies", ErrInvalidInput)
	}
	switch family {
	case FamilySSH:
		if level != InteractionMedium {
			return fmt.Errorf("%w: SSH deception is medium interaction per INT-01", ErrInvalidInput)
		}
	case FamilyHTTP, FamilySMB:
		if level != InteractionLow {
			return fmt.Errorf("%w: HTTP and SMB deception are low interaction per INT-01", ErrInvalidInput)
		}
	case FamilyPostgres:
		// INT-01 permits either for a database surface.
	}
	return nil
}

// UnreportedObservation is the observed record of a decoy nothing has reported
// on. Every dimension is Unknown, and the state is unknown rather than healthy
// or absent. This is the single most important function in the package: a
// deception product that shows a decoy as deployed when nothing confirmed it
// is lying about its own coverage.
func UnreportedObservation(decoyID string, observedAt time.Time) Observation {
	conditions := make([]Condition, 0, len(conditionTypeOrder))
	for _, conditionType := range conditionTypeOrder {
		conditions = append(conditions, Condition{
			Type:               conditionType,
			Status:             StatusUnknown,
			Reason:             "not_observed",
			Message:            "",
			LastTransitionTime: observedAt.UTC(),
		})
	}
	return Observation{DecoyID: decoyID, State: ObservedUnknown, Conditions: conditions}
}

// NormalizeName applies the same NFC, control-character, and bound rules the
// environment domain applies, so one operator string means one thing product
// wide.
func NormalizeName(value string) (NormalizedName, error) {
	if !utf8.ValidString(value) {
		return NormalizedName{}, fmt.Errorf("%w: display name must be valid UTF-8", ErrInvalidInput)
	}
	display := norm.NFC.String(strings.TrimSpace(value))
	if display == "" || utf8.RuneCountInString(display) > MaxNameRunes || len(display) > MaxNameBytes {
		return NormalizedName{}, fmt.Errorf(
			"%w: display name must contain 1..%d code points and at most %d bytes",
			ErrInvalidInput, MaxNameRunes, MaxNameBytes,
		)
	}
	for _, r := range display {
		if unicode.IsControl(r) {
			return NormalizedName{}, fmt.Errorf("%w: display name contains a control character", ErrInvalidInput)
		}
	}
	return NormalizedName{DisplayName: display, NameKey: norm.NFC.String(cases.Fold().String(display))}, nil
}

// NormalizeAddress accepts one canonical private IPv4 host address. A decoy
// answers on a single address, so a prefix, a zero-padded octet, or an IPv6
// literal is rejected rather than coerced.
func NormalizeAddress(value string) (string, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return "", fmt.Errorf("%w: address must not be empty or padded", ErrInvalidInput)
	}
	address, err := netip.ParseAddr(value)
	if err != nil || !address.Is4() || address.String() != value {
		return "", fmt.Errorf("%w: address must be a canonical IPv4 host address", ErrInvalidInput)
	}
	if !address.IsPrivate() {
		return "", fmt.Errorf("%w: address must be inside an RFC1918 range", ErrInvalidInput)
	}
	return address.String(), nil
}

// AddressWithinZone rejects a decoy placed outside the zone it claims, so a
// misconfiguration cannot silently produce a decoy answering on a production
// address. Network and broadcast addresses of a routable prefix are excluded:
// they are not host addresses a decoy can answer on.
func AddressWithinZone(address, cidr string) error {
	parsedAddress, err := netip.ParseAddr(address)
	if err != nil || !parsedAddress.Is4() {
		return fmt.Errorf("%w: address is not a canonical IPv4 host address", ErrInvalidInput)
	}
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil || !prefix.Addr().Is4() || prefix != prefix.Masked() {
		return fmt.Errorf("%w: zone prefix is not canonical", ErrInvalidInput)
	}
	if !prefix.Contains(parsedAddress) {
		return fmt.Errorf("%w: %s is outside %s", ErrAddressOutsideZone, address, cidr)
	}
	if prefix.Bits() <= 30 {
		network := prefix.Addr()
		if parsedAddress == network {
			return fmt.Errorf("%w: %s is the zone network address", ErrAddressOutsideZone, address)
		}
		if parsedAddress == broadcastOf(prefix) {
			return fmt.Errorf("%w: %s is the zone broadcast address", ErrAddressOutsideZone, address)
		}
	}
	return nil
}

func broadcastOf(prefix netip.Prefix) netip.Addr {
	octets := prefix.Addr().As4()
	hostBits := 32 - prefix.Bits()
	for index := 0; index < hostBits; index++ {
		octets[3-index/8] |= 1 << (index % 8)
	}
	return netip.AddrFrom4(octets)
}

// NormalizeListLimit bounds one page at MaxListLimit.
func NormalizeListLimit(limit int32) (int32, error) {
	if limit == 0 {
		return DefaultListLimit, nil
	}
	if limit < 1 || limit > MaxListLimit {
		return 0, fmt.Errorf("%w: list limit must be between 1 and %d", ErrInvalidInput, MaxListLimit)
	}
	return limit, nil
}

// ValidateRevision rejects a non-positive optimistic-concurrency revision.
func ValidateRevision(revision int64) error {
	if revision < 1 {
		return fmt.Errorf("%w: revision must be positive", ErrInvalidInput)
	}
	return nil
}

// ValidateReport rejects an Edge decoy report that is not well formed. It is
// the trust boundary for everything an Edge says about decoys.
func ValidateReport(report Report) error {
	if !ValidUUIDv7(report.ReportID) || !ValidUUIDv7(report.DeviceID) || report.ObservedAt.IsZero() {
		return fmt.Errorf("%w: decoy report envelope is invalid", ErrInvalidInput)
	}
	if len(report.Observations) > MaxDecoysPerDevice {
		return fmt.Errorf("%w: decoy report exceeds the %d-decoy bound", ErrInvalidInput, MaxDecoysPerDevice)
	}
	seen := make(map[string]struct{}, len(report.Observations))
	for _, observation := range report.Observations {
		if !ValidUUIDv7(observation.DecoyID) {
			return fmt.Errorf("%w: decoy observation identity is invalid", ErrInvalidInput)
		}
		if _, duplicate := seen[observation.DecoyID]; duplicate {
			return fmt.Errorf("%w: decoy observation is repeated", ErrInvalidInput)
		}
		seen[observation.DecoyID] = struct{}{}
		if err := validateObservation(observation, report.ObservedAt); err != nil {
			return err
		}
	}
	return nil
}

func validateObservation(observation Observation, observedAt time.Time) error {
	// An Edge may not claim unmanaged: SEC-06 is a Control Plane projection
	// from device state, and a device that has lost management is exactly the
	// one whose self-report cannot be trusted to say so.
	if !observation.State.Valid() || observation.State == ObservedUnmanaged {
		return fmt.Errorf("%w: observed lifecycle is invalid", ErrInvalidInput)
	}
	if observation.LastInteractionAt != nil && observation.LastInteractionAt.After(observedAt) {
		return fmt.Errorf("%w: last interaction is after the observation", ErrInvalidInput)
	}
	if len(observation.Conditions) != len(conditionTypeOrder) {
		return fmt.Errorf("%w: decoy conditions must be the complete ordered set", ErrInvalidInput)
	}
	for index, condition := range observation.Conditions {
		if condition.Type != conditionTypeOrder[index] || !condition.Status.Valid() {
			return fmt.Errorf("%w: decoy condition order or status is invalid", ErrInvalidInput)
		}
		if !validReasonCode(condition.Reason) || len(condition.Message) > MaxConditionMessageBytes ||
			!utf8.ValidString(condition.Message) {
			return fmt.Errorf("%w: decoy condition reason or message is invalid", ErrInvalidInput)
		}
		for _, r := range condition.Message {
			if unicode.IsControl(r) {
				return fmt.Errorf("%w: decoy condition message contains a control character", ErrInvalidInput)
			}
		}
		if condition.LastTransitionTime.IsZero() || condition.LastTransitionTime.After(observedAt) {
			return fmt.Errorf("%w: decoy condition transition time is invalid", ErrInvalidInput)
		}
	}
	return nil
}

func validReasonCode(value string) bool {
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

func normalizeMutation(mutation Mutation) (Mutation, error) {
	if mutation.ActorID == "" || strings.TrimSpace(mutation.ActorID) != mutation.ActorID || len(mutation.ActorID) > 256 {
		return Mutation{}, fmt.Errorf("%w: actor identity is invalid", ErrInvalidInput)
	}
	if mutation.RequestID != "" && (strings.TrimSpace(mutation.RequestID) != mutation.RequestID ||
		len(mutation.RequestID) > MaxRequestIDBytes) {
		return Mutation{}, fmt.Errorf("%w: request identity is invalid", ErrInvalidInput)
	}
	for _, value := range []string{mutation.ActorID, mutation.RequestID} {
		for _, r := range value {
			if unicode.IsControl(r) {
				return Mutation{}, fmt.Errorf("%w: mutation identity contains a control character", ErrInvalidInput)
			}
		}
	}
	if mutation.OccurredAt.IsZero() {
		mutation.OccurredAt = time.Now().UTC()
	} else {
		mutation.OccurredAt = mutation.OccurredAt.UTC()
	}
	return mutation, nil
}

// ValidUUIDv7 accepts only the canonical lowercase UUIDv7 form every Guardian
// identity uses.
func ValidUUIDv7(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' ||
		value[14] != '7' {
		return false
	}
	switch value[19] {
	case '8', '9', 'a', 'b':
	default:
		return false
	}
	for index, character := range []byte(value) {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') {
			continue
		}
		return false
	}
	return true
}
