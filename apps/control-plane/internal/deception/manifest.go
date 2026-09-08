package deception

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// The decoy pack manifest (P2-W4).
//
// P2-W15 gave the decoy record nowhere to put a runtime detail: no image,
// command, mount, capability, or port field exists in the API, on the wire, or
// in the database. This is the only place one can enter, and it enters from a
// committed file rather than from an operator, which is what keeps the API
// surface unable to describe a container.
//
// A manifest is a checked declaration, not a verified artefact. Signature
// enforcement is the Phase 5 gate AC-SEC-004, and nothing here verifies
// anything about a pack that has not been built yet.

// ManifestSchema is the contract version this build implements. A manifest
// carrying anything else is refused rather than coerced.
const ManifestSchema = "guardian.decoy.manifest.v1"

var (
	// ErrManifestVersion is the P2-W4 acceptance criterion "manifest version
	// mismatch explicit". It is a distinct sentinel so a caller can tell a
	// manifest this build does not implement from one that is merely wrong: the
	// first is a deployment problem and the second is a pack bug.
	ErrManifestVersion = errors.New("decoy manifest schema version is not implemented by this build")
	// ErrManifestPrivilege is the other acceptance criterion, "unsupported
	// privilege request rejected". Also its own sentinel, because a manifest
	// asking for authority Guardian will not grant is the one validation
	// failure that is a security event rather than a typo.
	ErrManifestPrivilege = errors.New("decoy manifest requests an unsupported privilege")
	ErrManifestInvalid   = errors.New("decoy manifest is invalid")
)

// GrantableCapabilities is the whole set of Linux capabilities Guardian will
// add to a decoy's otherwise empty set.
//
// One entry, and it is there under protest: a decoy presenting SSH on 22 or
// HTTP on 80 has no other way to bind the port that makes it convincing.
// ADR 0009 and the P0-W6 review set the rest of the baseline — dropped
// capabilities, read-only root, no bind mounts, no runtime sockets,
// no-new-privileges — and no manifest field can express a departure from any of
// it, because no such field exists.
//
// A request outside this set is rejected, not filtered. Dropping it silently
// would leave a pack author believing they had been granted something they had
// not, and the pack would fail in a way that looks like the runtime's fault.
var GrantableCapabilities = []string{"NET_BIND_SERVICE"}

// Manifest is one decoy pack, as declared.
type Manifest struct {
	Schema           string             `json:"schema"`
	Pack             string             `json:"pack"`
	Version          string             `json:"version"`
	Family           Family             `json:"family"`
	InteractionLevel InteractionLevel   `json:"interaction_level"`
	Ports            []ManifestPort     `json:"ports"`
	Network          ManifestNetwork    `json:"network"`
	Privileges       ManifestPrivileges `json:"privileges"`
	Telemetry        ManifestTelemetry  `json:"telemetry"`
	HealthProbe      ManifestProbe      `json:"health_probe"`
	Resources        ManifestResources  `json:"resources"`
	Egress           ManifestEgress     `json:"egress"`
	Persona          ManifestPersona    `json:"persona"`
}

type ManifestPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

// ManifestNetwork carries the placement mode. `host_network` is not a value
// this type can hold, which is the point: a decoy that shared the host's
// network namespace would be reachable as the host.
type ManifestNetwork struct {
	Mode string `json:"mode"`
}

type ManifestPrivileges struct {
	Capabilities []string `json:"capabilities"`
}

type ManifestTelemetry struct {
	Adapter string `json:"adapter"`
}

// ManifestProbe is how P2-W14 decides whether a decoy is functionally alive.
// A connection attempt and nothing else: a probe that could run a command
// would be a second execution path into the decoy.
type ManifestProbe struct {
	Type           string `json:"type"`
	Port           int    `json:"port"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type ManifestResources struct {
	CPUMillicores int `json:"cpu_millicores"`
	MemoryMiB     int `json:"memory_mib"`
	PIDs          int `json:"pids"`
}

// ManifestEgress carries AC-SEC-003. The policy is deny and there is no other
// value; `Allow` is the enumerated exception list, and an empty one means
// nothing is permitted rather than everything.
type ManifestEgress struct {
	Policy string               `json:"policy"`
	Allow  []ManifestEgressRule `json:"allow"`
}

type ManifestEgressRule struct {
	Destination string `json:"destination"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	Reason      string `json:"reason"`
}

// ManifestPersona is DC-11 content metadata: bounded text a pack presents.
// It names no file, path, template, or command, because content that can name
// an artefact is a second way to get one into the decoy.
type ManifestPersona struct {
	Personas        []Persona `json:"personas"`
	HostnamePattern string    `json:"hostname_pattern"`
	Banner          string    `json:"banner"`
}

const (
	maxManifestBytes   = 16 << 10
	maxPortsPerPack    = 8
	maxEgressRules     = 4
	maxBannerBytes     = 512
	maxHostnameBytes   = 64
	maxProbeTimeoutSec = 30
)

var (
	networkModes      = []string{"routed_secondary_ip"}
	telemetryAdapters = []string{"cowrie-json", "http-access", "postgres-wire", "smb-audit"}
	probeTypes        = []string{"tcp_connect"}
	protocols         = []string{"tcp", "udp"}
)

// ParseManifest decodes and validates one manifest.
//
// The version is checked first and on its own. A manifest this build does not
// implement must not be partially read: every field below it is interpreted
// under rules that may not apply, and reporting a field error for a document
// written against a different contract sends the reader after the wrong thing.
func ParseManifest(raw []byte) (Manifest, error) {
	if len(raw) > maxManifestBytes {
		return Manifest{}, fmt.Errorf("%w: manifest exceeds %d bytes", ErrManifestInvalid, maxManifestBytes)
	}
	// Unknown fields are rejected rather than ignored. A manifest carrying a
	// property this build does not know is a manifest whose author expected
	// something this build will not do.
	var probe struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Manifest{}, fmt.Errorf("%w: manifest is not valid JSON", ErrManifestInvalid)
	}
	if probe.Schema != ManifestSchema {
		return Manifest{}, fmt.Errorf("%w: found %q, want %q", ErrManifestVersion, probe.Schema, ManifestSchema)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("%w: %s", ErrManifestInvalid, "manifest carries an unknown or malformed field")
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// Validate enforces every rule the schema states, in Go, so a manifest that
// reached this build without passing the JSON Schema is still refused.
func (m Manifest) Validate() error {
	if m.Schema != ManifestSchema {
		return fmt.Errorf("%w: found %q, want %q", ErrManifestVersion, m.Schema, ManifestSchema)
	}
	// Privileges first among the field checks: it is the one whose failure is a
	// security event, and a manifest that fails it should say so even if it
	// also has a malformed port.
	if err := m.validatePrivileges(); err != nil {
		return err
	}
	if !validPackName(m.Pack) {
		return fmt.Errorf("%w: pack name is not a lowercase token", ErrManifestInvalid)
	}
	if !validPackVersion(m.Version) {
		return fmt.Errorf("%w: version is not a three-part number", ErrManifestInvalid)
	}
	if !m.Family.Valid() {
		return fmt.Errorf("%w: family is outside the closed DC-01 set", ErrManifestInvalid)
	}
	if err := InteractionLevelFor(m.Family, m.InteractionLevel); err != nil {
		return fmt.Errorf("%w: %s", ErrManifestInvalid, "interaction level contradicts INT-01 for this family")
	}
	if err := m.validatePorts(); err != nil {
		return err
	}
	if !slices.Contains(networkModes, m.Network.Mode) {
		return fmt.Errorf("%w: network mode %q is not implemented", ErrManifestInvalid, m.Network.Mode)
	}
	if !slices.Contains(telemetryAdapters, m.Telemetry.Adapter) {
		return fmt.Errorf("%w: telemetry adapter %q does not exist", ErrManifestInvalid, m.Telemetry.Adapter)
	}
	if err := m.validateProbe(); err != nil {
		return err
	}
	if err := m.validateResources(); err != nil {
		return err
	}
	if err := m.validateEgress(); err != nil {
		return err
	}
	return m.validatePersona()
}

// validatePrivileges carries the "unsupported privilege request rejected"
// acceptance criterion. It names the capability it refused, because a pack
// author needs to know which request was the problem.
func (m Manifest) validatePrivileges() error {
	if len(m.Privileges.Capabilities) > len(GrantableCapabilities) {
		return fmt.Errorf("%w: more capabilities than Guardian grants", ErrManifestPrivilege)
	}
	seen := make(map[string]struct{}, len(m.Privileges.Capabilities))
	for _, capability := range m.Privileges.Capabilities {
		if !slices.Contains(GrantableCapabilities, capability) {
			return fmt.Errorf("%w: %q", ErrManifestPrivilege, capability)
		}
		if _, duplicate := seen[capability]; duplicate {
			return fmt.Errorf("%w: %q requested twice", ErrManifestPrivilege, capability)
		}
		seen[capability] = struct{}{}
	}
	return nil
}

func (m Manifest) validatePorts() error {
	if len(m.Ports) == 0 || len(m.Ports) > maxPortsPerPack {
		return fmt.Errorf("%w: a pack declares 1..%d ports", ErrManifestInvalid, maxPortsPerPack)
	}
	seen := make(map[ManifestPort]struct{}, len(m.Ports))
	for _, port := range m.Ports {
		if port.Port < 1 || port.Port > 65535 || !slices.Contains(protocols, port.Protocol) {
			return fmt.Errorf("%w: port %d/%s is not a valid listener", ErrManifestInvalid, port.Port, port.Protocol)
		}
		if _, duplicate := seen[port]; duplicate {
			return fmt.Errorf("%w: port %d/%s is declared twice", ErrManifestInvalid, port.Port, port.Protocol)
		}
		seen[port] = struct{}{}
	}
	return nil
}

// validateProbe also requires the probed port to be one the pack declares. A
// probe against a port the decoy never listens on would report a healthy decoy
// as absent, or an absent one as healthy, depending on what else is bound.
func (m Manifest) validateProbe() error {
	if !slices.Contains(probeTypes, m.HealthProbe.Type) {
		return fmt.Errorf("%w: probe type %q is not implemented", ErrManifestInvalid, m.HealthProbe.Type)
	}
	if m.HealthProbe.TimeoutSeconds < 1 || m.HealthProbe.TimeoutSeconds > maxProbeTimeoutSec {
		return fmt.Errorf("%w: probe timeout is outside 1..%d seconds", ErrManifestInvalid, maxProbeTimeoutSec)
	}
	for _, port := range m.Ports {
		if port.Port == m.HealthProbe.Port {
			return nil
		}
	}
	return fmt.Errorf("%w: probe port %d is not one the pack declares", ErrManifestInvalid, m.HealthProbe.Port)
}

func (m Manifest) validateResources() error {
	switch {
	case m.Resources.CPUMillicores < 10 || m.Resources.CPUMillicores > 2000:
		return fmt.Errorf("%w: cpu_millicores is outside 10..2000", ErrManifestInvalid)
	case m.Resources.MemoryMiB < 16 || m.Resources.MemoryMiB > 1024:
		return fmt.Errorf("%w: memory_mib is outside 16..1024", ErrManifestInvalid)
	case m.Resources.PIDs < 8 || m.Resources.PIDs > 512:
		return fmt.Errorf("%w: pids is outside 8..512", ErrManifestInvalid)
	}
	return nil
}

// validateEgress carries AC-SEC-003. `deny` is the only policy, and every
// exception names a private destination and a reason: an allowance nobody can
// explain is one nobody will remove.
func (m Manifest) validateEgress() error {
	if m.Egress.Policy != "deny" {
		return fmt.Errorf("%w: egress policy %q is not deny", ErrManifestInvalid, m.Egress.Policy)
	}
	if len(m.Egress.Allow) > maxEgressRules {
		return fmt.Errorf("%w: more than %d egress exceptions", ErrManifestInvalid, maxEgressRules)
	}
	for _, rule := range m.Egress.Allow {
		if _, err := NormalizeAddress(rule.Destination); err != nil {
			return fmt.Errorf("%w: egress destination is not a private IPv4 host address", ErrManifestInvalid)
		}
		if rule.Port < 1 || rule.Port > 65535 || !slices.Contains(protocols, rule.Protocol) {
			return fmt.Errorf("%w: egress rule names an invalid port or protocol", ErrManifestInvalid)
		}
		if rule.Reason == "" || len(rule.Reason) > 256 {
			return fmt.Errorf("%w: every egress exception states a reason", ErrManifestInvalid)
		}
	}
	return nil
}

func (m Manifest) validatePersona() error {
	if len(m.Persona.Personas) == 0 || len(m.Persona.Personas) > 4 {
		return fmt.Errorf("%w: a pack presents 1..4 curated personas", ErrManifestInvalid)
	}
	seen := make(map[Persona]struct{}, len(m.Persona.Personas))
	for _, persona := range m.Persona.Personas {
		if !persona.Valid() {
			return fmt.Errorf("%w: persona %q is outside the closed DC-11 set", ErrManifestInvalid, persona)
		}
		if _, duplicate := seen[persona]; duplicate {
			return fmt.Errorf("%w: persona %q is declared twice", ErrManifestInvalid, persona)
		}
		seen[persona] = struct{}{}
	}
	if m.Persona.HostnamePattern == "" || len(m.Persona.HostnamePattern) > maxHostnameBytes {
		return fmt.Errorf("%w: hostname pattern is empty or too long", ErrManifestInvalid)
	}
	if len(m.Persona.Banner) > maxBannerBytes {
		return fmt.Errorf("%w: banner exceeds %d bytes", ErrManifestInvalid, maxBannerBytes)
	}
	return nil
}

func validPackName(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for index, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
		case index > 0 && (character >= '0' && character <= '9' || character == '-'):
		default:
			return false
		}
	}
	return true
}

func validPackVersion(value string) bool {
	parts := 1
	digits := 0
	for _, character := range value {
		switch {
		case character == '.':
			if digits == 0 {
				return false
			}
			parts++
			digits = 0
		case character >= '0' && character <= '9':
			digits++
			if digits > 6 {
				return false
			}
		default:
			return false
		}
	}
	return parts == 3 && digits > 0
}
