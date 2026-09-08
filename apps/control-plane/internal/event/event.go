// Package event owns the canonical decoy event: the shape every later stage
// reads, and the rules that decide whether one is allowed in.
//
// EV-02 fixes the field set. This makes it checkable, and adds the two things a
// field list cannot say on its own: what the Control Plane sets rather than
// accepts, and what an event may never carry at all.
package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

// Schema is the contract version this build implements.
const Schema = "guardian.event.v1"

var (
	// ErrSchemaVersion is explicit and separate: an event written to a contract
	// this build does not implement must not be partially read, because every
	// field below the version is interpreted under rules that may not apply.
	ErrSchemaVersion = errors.New("canonical event schema version is not implemented by this build")
	// ErrInvalidEvent is a well-formed contract, badly filled in.
	ErrInvalidEvent = errors.New("canonical event is invalid")
	// ErrAsserted is the one an Edge should never trigger: a field the Control
	// Plane owns arrived already filled in. Its own sentinel because it is a
	// trust-boundary violation rather than a formatting mistake.
	ErrAsserted = errors.New("canonical event asserts a field the Control Plane owns")
)

// MaxEventBytes bounds one encoded event. AC-SEC-008 requires an oversized or
// malformed payload not to crash the process; the first half of not crashing is
// refusing to allocate for it.
const MaxEventBytes = 32 << 10

// Event is one thing that happened at a decoy.
//
// It carries metadata and a reference to raw evidence, never the evidence. An
// envelope that could hold a transcript would put an attacker's bytes into
// every index, log line, and console that touches an event.
type Event struct {
	Schema        string     `json:"schema"`
	EventID       string     `json:"event_id"`
	EventTime     time.Time  `json:"event_time"`
	ObservedTime  time.Time  `json:"observed_time"`
	IngestedTime  *time.Time `json:"ingested_time"`
	EnvironmentID string     `json:"environment_id"`
	EdgeID        string     `json:"edge_id"`
	DecoyID       string     `json:"decoy_id"`

	SourceIP        *string `json:"source_ip"`
	SourcePort      *int    `json:"source_port"`
	DestinationIP   *string `json:"destination_ip"`
	DestinationPort *int    `json:"destination_port"`

	Protocol  string  `json:"protocol"`
	Action    string  `json:"action"`
	SessionID *string `json:"session_id"`

	Auth        *Auth      `json:"auth"`
	RawRef      *RawRef    `json:"raw_ref"`
	Sensitivity string     `json:"sensitivity"`
	Provenance  Provenance `json:"provenance"`
}

// Auth is who the attacker claimed to be and whether the decoy accepted it.
//
// There is deliberately no field for the secret they supplied. EV-03 requires
// credential material to be protected or redacted, and a struct with nowhere to
// put a password cannot leak one by accident.
type Auth struct {
	Identity              *string `json:"identity"`
	Result                string  `json:"result"`
	SyntheticCredentialID *string `json:"synthetic_credential_id"`
}

// RawRef points at bounded evidence stored elsewhere (P2-W13).
type RawRef struct {
	BlobID      string `json:"blob_id"`
	MediaType   string `json:"media_type"`
	ByteLength  int    `json:"byte_length"`
	Truncated   bool   `json:"truncated"`
	Quarantined bool   `json:"quarantined"`
}

// Provenance carries AC-EV-002's remainder: what produced this event and
// through which of P2-W11's two paths.
type Provenance struct {
	SourcePath     string  `json:"source_path"`
	Adapter        string  `json:"adapter"`
	AdapterVersion *string `json:"adapter_version"`
	Pack           string  `json:"pack"`
	PackVersion    string  `json:"pack_version"`
	RuntimeVersion *string `json:"runtime_version"`
}

var (
	protocols    = []string{"ssh", "http", "postgres", "smb", "tcp"}
	sensitivity  = []string{"routine", "sensitive", "restricted"}
	authResults  = []string{"succeeded", "failed", "unknown"}
	sourcePaths  = []string{"native_uds", "log_adapter"}
	adapters     = []string{"cowrie-json", "http-access", "postgres-wire", "smb-audit"}
	mediaTypes   = []string{"text/plain", "application/octet-stream", "application/json"}
	actionValues = []string{
		"connection_opened",
		"connection_closed",
		"auth_attempt",
		"auth_succeeded",
		"auth_failed",
		"command_executed",
		"request_received",
		"file_accessed",
		"query_executed",
		"session_transcript",
	}
)

const (
	maxIdentityBytes = 256
	maxSessionBytes  = 128
	maxAddressBytes  = 45
	maxRawBytes      = 1 << 20
	maxRuntimeBytes  = 64
)

// Parse decodes one event as it arrived from an Edge.
//
// Three things happen in this order and the order is the point. The size bound
// comes first, so a hostile payload is refused before it is decoded. The schema
// version comes next, on its own, so an event written to another contract is
// never partially interpreted. Only then are the fields read.
func Parse(raw []byte) (Event, error) {
	if len(raw) > MaxEventBytes {
		return Event{}, fmt.Errorf("%w: event exceeds %d bytes", ErrInvalidEvent, MaxEventBytes)
	}
	var probe struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Event{}, fmt.Errorf("%w: event is not valid JSON", ErrInvalidEvent)
	}
	if probe.Schema != Schema {
		return Event{}, fmt.Errorf("%w: found %q, want %q", ErrSchemaVersion, probe.Schema, Schema)
	}
	var decoded Event
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return Event{}, fmt.Errorf("%w: event fields are malformed", ErrInvalidEvent)
	}
	if err := decoded.ValidateFromEdge(); err != nil {
		return Event{}, err
	}
	return decoded, nil
}

// ValidateFromEdge checks an event as an Edge is allowed to send it.
//
// Separate from Validate because two fields are the Control Plane's to set and
// an Edge asserting either is a trust-boundary violation, not a bad value:
//
//   - ingested_time. An Edge that could assert when the Control Plane accepted
//     an event could backdate it past a retention boundary.
//   - edge_id is checked by the caller against the authenticated device
//     identity, so one Edge cannot attribute an event to another. This function
//     enforces the shape; the ingest path enforces the match.
func (e Event) ValidateFromEdge() error {
	if e.IngestedTime != nil {
		return fmt.Errorf("%w: ingested_time", ErrAsserted)
	}
	return e.Validate()
}

// Validate enforces every rule the schema states, in Go, so an event that
// reached this build without passing the JSON Schema is still refused.
func (e Event) Validate() error {
	if e.Schema != Schema {
		return fmt.Errorf("%w: found %q, want %q", ErrSchemaVersion, e.Schema, Schema)
	}
	for name, id := range map[string]string{
		"event_id":       e.EventID,
		"environment_id": e.EnvironmentID,
		"edge_id":        e.EdgeID,
		"decoy_id":       e.DecoyID,
	} {
		if !ValidUUIDv7(id) {
			return fmt.Errorf("%w: %s is not a UUIDv7", ErrInvalidEvent, name)
		}
	}
	if err := e.validateTimes(); err != nil {
		return err
	}
	if !slices.Contains(protocols, e.Protocol) {
		return fmt.Errorf("%w: protocol %q is outside the closed set", ErrInvalidEvent, e.Protocol)
	}
	if !slices.Contains(actionValues, e.Action) {
		return fmt.Errorf("%w: action %q is outside the closed set", ErrInvalidEvent, e.Action)
	}
	if !slices.Contains(sensitivity, e.Sensitivity) {
		return fmt.Errorf("%w: sensitivity %q is outside the closed set", ErrInvalidEvent, e.Sensitivity)
	}
	if err := e.validateEndpoints(); err != nil {
		return err
	}
	if e.SessionID != nil && (*e.SessionID == "" || len(*e.SessionID) > maxSessionBytes) {
		return fmt.Errorf("%w: session_id is empty or exceeds %d bytes", ErrInvalidEvent, maxSessionBytes)
	}
	if err := e.validateAuth(); err != nil {
		return err
	}
	if err := e.validateRawRef(); err != nil {
		return err
	}
	return e.validateProvenance()
}

/*
validateTimes carries AC-EV-003.

Both timestamps are required and neither may be zero. observed_time may not
precede event_time by any amount: an adapter cannot have seen an interaction
before it happened, and an event claiming otherwise is either a broken clock or
a forged record. The reverse gap is allowed and expected — a log adapter reads a
line some time after it was written, which is exactly why the two fields are
separate.
*/
func (e Event) validateTimes() error {
	if e.EventTime.IsZero() || e.ObservedTime.IsZero() {
		return fmt.Errorf("%w: event_time and observed_time are required", ErrInvalidEvent)
	}
	if e.ObservedTime.Before(e.EventTime) {
		return fmt.Errorf("%w: observed_time precedes event_time", ErrInvalidEvent)
	}
	return nil
}

func (e Event) validateEndpoints() error {
	for name, address := range map[string]*string{
		"source_ip":      e.SourceIP,
		"destination_ip": e.DestinationIP,
	} {
		if address == nil {
			continue
		}
		if *address == "" || len(*address) > maxAddressBytes {
			return fmt.Errorf("%w: %s is empty or exceeds %d bytes", ErrInvalidEvent, name, maxAddressBytes)
		}
	}
	for name, port := range map[string]*int{
		"source_port":      e.SourcePort,
		"destination_port": e.DestinationPort,
	} {
		if port != nil && (*port < 1 || *port > 65535) {
			return fmt.Errorf("%w: %s is outside 1..65535", ErrInvalidEvent, name)
		}
	}
	return nil
}

func (e Event) validateAuth() error {
	if e.Auth == nil {
		return nil
	}
	if !slices.Contains(authResults, e.Auth.Result) {
		return fmt.Errorf("%w: auth result %q is outside the closed set", ErrInvalidEvent, e.Auth.Result)
	}
	if e.Auth.Identity != nil && len(*e.Auth.Identity) > maxIdentityBytes {
		return fmt.Errorf("%w: auth identity exceeds %d bytes", ErrInvalidEvent, maxIdentityBytes)
	}
	if e.Auth.SyntheticCredentialID != nil && !ValidUUIDv7(*e.Auth.SyntheticCredentialID) {
		return fmt.Errorf("%w: synthetic_credential_id is not a UUIDv7", ErrInvalidEvent)
	}
	return nil
}

func (e Event) validateRawRef() error {
	if e.RawRef == nil {
		return nil
	}
	if !ValidUUIDv7(e.RawRef.BlobID) {
		return fmt.Errorf("%w: raw_ref blob_id is not a UUIDv7", ErrInvalidEvent)
	}
	if !slices.Contains(mediaTypes, e.RawRef.MediaType) {
		return fmt.Errorf("%w: raw_ref media type %q is outside the closed set", ErrInvalidEvent, e.RawRef.MediaType)
	}
	if e.RawRef.ByteLength < 0 || e.RawRef.ByteLength > maxRawBytes {
		return fmt.Errorf("%w: raw_ref byte_length is outside 0..%d", ErrInvalidEvent, maxRawBytes)
	}
	return nil
}

func (e Event) validateProvenance() error {
	p := e.Provenance
	if !slices.Contains(sourcePaths, p.SourcePath) {
		return fmt.Errorf("%w: provenance source_path %q is outside the closed set", ErrInvalidEvent, p.SourcePath)
	}
	if !slices.Contains(adapters, p.Adapter) {
		return fmt.Errorf("%w: provenance adapter %q does not exist", ErrInvalidEvent, p.Adapter)
	}
	if p.Pack == "" || len(p.Pack) > 64 || p.PackVersion == "" || len(p.PackVersion) > 32 {
		return fmt.Errorf("%w: provenance pack identity is missing or oversized", ErrInvalidEvent)
	}
	if p.RuntimeVersion != nil && len(*p.RuntimeVersion) > maxRuntimeBytes {
		return fmt.Errorf("%w: provenance runtime_version exceeds %d bytes", ErrInvalidEvent, maxRuntimeBytes)
	}
	return nil
}

// Ingest stamps the Control Plane's own timestamp and returns the stored form.
//
// This is the only way ingested_time is ever set. AC-EV-003 wants all three
// timestamps kept, and keeping one an Edge supplied would make it a claim
// rather than a record.
func (e Event) Ingest(at time.Time) Event {
	stamped := at.UTC()
	e.IngestedTime = &stamped
	return e
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

// Actions returns the closed action vocabulary in declaration order.
func Actions() []string { return slices.Clone(actionValues) }

// Adapters returns the closed adapter vocabulary in declaration order.
func Adapters() []string { return slices.Clone(adapters) }
