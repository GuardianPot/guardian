package normalize

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// The Edge's copy of the canonical event (P2-W10).
//
// Two modules, two declarations, one contract. `tests/integration/event` asserts
// this file and `apps/control-plane/internal/event` agree with
// `schemas/event/v1/canonical-event.schema.json`, the same way the health
// contract is kept in step across the same boundary.
//
// The Edge's copy carries no `ingested_time`, and that is not an omission: the
// field belongs to the Control Plane, and a struct on this side with nowhere to
// put it cannot send one.

const Schema = "guardian.event.v1"

var ErrInvalidEvent = errors.New("canonical event is invalid")

type Event struct {
	Schema        string    `json:"schema"`
	EventID       string    `json:"event_id"`
	EventTime     time.Time `json:"event_time"`
	ObservedTime  time.Time `json:"observed_time"`
	EnvironmentID string    `json:"environment_id"`
	EdgeID        string    `json:"edge_id"`
	DecoyID       string    `json:"decoy_id"`

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

// Auth has nowhere to put the secret an attacker supplied, on this side either.
// EV-03 requires credential material to be redacted, and the redaction that
// cannot be forgotten is the field that does not exist.
type Auth struct {
	Identity              *string `json:"identity"`
	Result                string  `json:"result"`
	SyntheticCredentialID *string `json:"synthetic_credential_id"`
}

type RawRef struct {
	BlobID      string `json:"blob_id"`
	MediaType   string `json:"media_type"`
	ByteLength  int    `json:"byte_length"`
	Truncated   bool   `json:"truncated"`
	Quarantined bool   `json:"quarantined"`
}

type Provenance struct {
	SourcePath     string  `json:"source_path"`
	Adapter        string  `json:"adapter"`
	AdapterVersion *string `json:"adapter_version"`
	Pack           string  `json:"pack"`
	PackVersion    string  `json:"pack_version"`
	RuntimeVersion *string `json:"runtime_version"`
}

var (
	protocols   = []string{"ssh", "http", "postgres", "smb", "tcp"}
	sensitivity = []string{"routine", "sensitive", "restricted"}
	authResults = []string{"succeeded", "failed", "unknown"}
	mediaTypes  = []string{"text/plain", "application/octet-stream", "application/json"}
	actions     = []string{
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

// Protocols, Actions, Sensitivities, AuthResults and MediaTypes expose the
// vocabularies for the cross-module contract check.
func Protocols() []string     { return slices.Clone(protocols) }
func Actions() []string       { return slices.Clone(actions) }
func Sensitivities() []string { return slices.Clone(sensitivity) }
func AuthResults() []string   { return slices.Clone(authResults) }
func MediaTypes() []string    { return slices.Clone(mediaTypes) }

const (
	maxIdentityBytes = 256
	maxSessionBytes  = 128
	maxAddressBytes  = 45
	maxRawBytes      = 1 << 20
)

// Validate rejects anything the Control Plane would reject, here, so a bad
// event costs a local check rather than a round trip and a rejected upload.
func (e Event) Validate() error {
	if e.Schema != Schema {
		return fmt.Errorf("%w: schema %q", ErrInvalidEvent, e.Schema)
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
	if e.EventTime.IsZero() || e.ObservedTime.IsZero() {
		return fmt.Errorf("%w: both timestamps are required", ErrInvalidEvent)
	}
	// An adapter cannot have seen an interaction before it happened.
	if e.ObservedTime.Before(e.EventTime) {
		return fmt.Errorf("%w: observed_time precedes event_time", ErrInvalidEvent)
	}
	if !slices.Contains(protocols, e.Protocol) {
		return fmt.Errorf("%w: protocol %q", ErrInvalidEvent, e.Protocol)
	}
	if !slices.Contains(actions, e.Action) {
		return fmt.Errorf("%w: action %q", ErrInvalidEvent, e.Action)
	}
	if !slices.Contains(sensitivity, e.Sensitivity) {
		return fmt.Errorf("%w: sensitivity %q", ErrInvalidEvent, e.Sensitivity)
	}
	if err := e.validateEndpoints(); err != nil {
		return err
	}
	if e.SessionID != nil && (*e.SessionID == "" || len(*e.SessionID) > maxSessionBytes) {
		return fmt.Errorf("%w: session_id is empty or oversized", ErrInvalidEvent)
	}
	if e.Auth != nil {
		if !slices.Contains(authResults, e.Auth.Result) {
			return fmt.Errorf("%w: auth result %q", ErrInvalidEvent, e.Auth.Result)
		}
		if e.Auth.Identity != nil && len(*e.Auth.Identity) > maxIdentityBytes {
			return fmt.Errorf("%w: auth identity exceeds %d bytes", ErrInvalidEvent, maxIdentityBytes)
		}
		if e.Auth.SyntheticCredentialID != nil && !ValidUUIDv7(*e.Auth.SyntheticCredentialID) {
			return fmt.Errorf("%w: synthetic_credential_id is not a UUIDv7", ErrInvalidEvent)
		}
	}
	if e.RawRef != nil {
		if !ValidUUIDv7(e.RawRef.BlobID) {
			return fmt.Errorf("%w: raw_ref blob_id is not a UUIDv7", ErrInvalidEvent)
		}
		if !slices.Contains(mediaTypes, e.RawRef.MediaType) {
			return fmt.Errorf("%w: raw_ref media type %q", ErrInvalidEvent, e.RawRef.MediaType)
		}
		if e.RawRef.ByteLength < 0 || e.RawRef.ByteLength > maxRawBytes {
			return fmt.Errorf("%w: raw_ref byte_length is out of range", ErrInvalidEvent)
		}
	}
	if e.Provenance.Pack == "" || e.Provenance.PackVersion == "" {
		return fmt.Errorf("%w: provenance pack identity is required", ErrInvalidEvent)
	}
	return nil
}

func (e Event) validateEndpoints() error {
	for name, address := range map[string]*string{
		"source_ip": e.SourceIP, "destination_ip": e.DestinationIP,
	} {
		if address != nil && (*address == "" || len(*address) > maxAddressBytes) {
			return fmt.Errorf("%w: %s is empty or oversized", ErrInvalidEvent, name)
		}
	}
	for name, port := range map[string]*int{
		"source_port": e.SourcePort, "destination_port": e.DestinationPort,
	} {
		if port != nil && (*port < 1 || *port > 65535) {
			return fmt.Errorf("%w: %s is outside 1..65535", ErrInvalidEvent, name)
		}
	}
	return nil
}

// ValidUUIDv7 accepts only the canonical lowercase form.
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
