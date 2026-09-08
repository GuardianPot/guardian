package event

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	eventID       = "0198dc8c-c600-7000-8000-000000000001"
	environmentID = "0198dc8c-c600-7000-8000-000000000002"
	edgeID        = "0198dc8c-c600-7000-8000-000000000003"
	decoyID       = "0198dc8c-c600-7000-8000-000000000004"
)

func valid() map[string]any {
	return map[string]any{
		"schema":         Schema,
		"event_id":       eventID,
		"event_time":     "2026-09-08T12:00:00Z",
		"observed_time":  "2026-09-08T12:00:01Z",
		"environment_id": environmentID,
		"edge_id":        edgeID,
		"decoy_id":       decoyID,
		"protocol":       "ssh",
		"action":         "auth_failed",
		"sensitivity":    "restricted",
		"auth":           map[string]any{"identity": "root", "result": "failed"},
		"provenance": map[string]any{
			"source_path": "native_uds", "adapter": "cowrie-json",
			"pack": "ssh-cowrie", "pack_version": "0.1.0",
		},
	}
}

func encode(t *testing.T, event map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAWellFormedEventIsAccepted(t *testing.T) {
	parsed, err := Parse(encode(t, valid()))
	if err != nil {
		t.Fatalf("a well-formed event was refused: %v", err)
	}
	if parsed.EventID != eventID || parsed.Protocol != "ssh" {
		t.Fatalf("parsed = %+v", parsed)
	}
	if parsed.IngestedTime != nil {
		t.Fatal("an event from an Edge arrived with an ingest time already set")
	}
}

/*
 * The trust boundary. ingested_time is the Control Plane's, and an Edge that
 * could assert it could backdate an event past a retention boundary. That is a
 * boundary violation rather than a bad value, so it has its own error.
 */
func TestAnEdgeCannotAssertItsOwnIngestTime(t *testing.T) {
	event := valid()
	event["ingested_time"] = "2020-01-01T00:00:00Z"

	_, err := Parse(encode(t, event))

	if !errors.Is(err, ErrAsserted) {
		t.Fatalf("err = %v, want ErrAsserted", err)
	}
	if errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("a boundary violation was reported as a formatting error: %v", err)
	}
}

// The Control Plane sets it, and that is the only way it is ever set.
func TestIngestStampsTheControlPlanesOwnTime(t *testing.T) {
	parsed, err := Parse(encode(t, valid()))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 8, 13, 0, 0, 0, time.UTC)

	stored := parsed.Ingest(at)

	if stored.IngestedTime == nil || !stored.IngestedTime.Equal(at) {
		t.Fatalf("ingested_time = %v, want %v", stored.IngestedTime, at)
	}
	// AC-EV-003: all three are kept.
	if stored.EventTime.IsZero() || stored.ObservedTime.IsZero() {
		t.Fatalf("ingest lost a timestamp: %+v", stored)
	}
}

/*
 * A version this build does not implement is refused on its own, before any
 * field is read. Every field below the version is interpreted under rules that
 * may not apply to it.
 */
func TestSchemaVersionMismatchIsExplicit(t *testing.T) {
	for _, schema := range []string{"guardian.event.v2", "guardian.telemetry.v1", ""} {
		event := valid()
		event["schema"] = schema
		event["protocol"] = "gopher" // also broken, and must not be what is reported

		_, err := Parse(encode(t, event))

		if !errors.Is(err, ErrSchemaVersion) {
			t.Fatalf("schema %q = %v, want ErrSchemaVersion", schema, err)
		}
		if errors.Is(err, ErrInvalidEvent) {
			t.Fatalf("schema %q was reported as a field error: %v", schema, err)
		}
	}
}

/*
 * AC-SEC-008: an oversized or malformed payload must not crash the process.
 *
 * The size bound is checked before decoding, so a hostile payload is refused
 * before anything allocates for it.
 */
func TestOversizedAndMalformedInputAreRefusedWithoutPanicking(t *testing.T) {
	for name, raw := range map[string][]byte{
		"an oversized document":  []byte(`{"schema":"` + Schema + `","pad":"` + strings.Repeat("a", MaxEventBytes) + `"}`),
		"truncated JSON":         []byte(`{"schema":"guardian.event.v1","event_id":`),
		"not JSON at all":        []byte("\x00\x01\x02 not json"),
		"an empty document":      {},
		"a JSON array":           []byte(`[1,2,3]`),
		"a deeply nested object": []byte(`{"schema":"` + Schema + `","auth":` + strings.Repeat(`{"identity":`, 200) + `null` + strings.Repeat(`}`, 200) + `}`),
	} {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("%s panicked: %v", name, recovered)
				}
			}()
			if _, err := Parse(raw); err == nil {
				t.Fatalf("%s was accepted", name)
			}
		}()
	}
}

// A wildly out-of-range field must not be coerced into something plausible.
func TestClosedVocabulariesRejectAnythingOutsideThem(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"an invented protocol":    func(e map[string]any) { e["protocol"] = "gopher" },
		"an invented action":      func(e map[string]any) { e["action"] = "exfiltrated_everything" },
		"an invented sensitivity": func(e map[string]any) { e["sensitivity"] = "public" },
		"an invented auth result": func(e map[string]any) {
			e["auth"] = map[string]any{"result": "maybe"}
		},
		"an invented source path": func(e map[string]any) {
			p := e["provenance"].(map[string]any)
			p["source_path"] = "trust_me"
		},
		"an adapter that does not exist": func(e map[string]any) {
			p := e["provenance"].(map[string]any)
			p["adapter"] = "shell-exec"
		},
		"an invented media type": func(e map[string]any) {
			e["raw_ref"] = map[string]any{
				"blob_id": decoyID, "media_type": "text/html", "byte_length": 10, "truncated": false,
			}
		},
	} {
		event := valid()
		mutate(event)
		if _, err := Parse(encode(t, event)); !errors.Is(err, ErrInvalidEvent) {
			t.Fatalf("%s = %v, want ErrInvalidEvent", name, err)
		}
	}
}

/*
 * An adapter cannot have seen an interaction before it happened. The reverse
 * gap is allowed and expected: a log adapter reads a line some time after it
 * was written, which is exactly why the two timestamps are separate fields.
 */
func TestObservedTimeMayNotPrecedeEventTime(t *testing.T) {
	event := valid()
	event["event_time"] = "2026-09-08T12:00:05Z"
	event["observed_time"] = "2026-09-08T12:00:00Z"

	if _, err := Parse(encode(t, event)); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("err = %v, want ErrInvalidEvent", err)
	}

	// The other direction is fine.
	event["event_time"] = "2026-09-08T12:00:00Z"
	event["observed_time"] = "2026-09-08T12:05:00Z"
	if _, err := Parse(encode(t, event)); err != nil {
		t.Fatalf("a delayed observation was refused: %v", err)
	}
}

/*
 * EV-03: credential material is protected or redacted.
 *
 * The envelope has nowhere to put a secret, and this asserts it structurally
 * rather than by inspection: a field carrying the attacker's password is not
 * something the type can hold, so an adapter cannot leak one by accident.
 */
func TestTheEnvelopeHasNowhereToPutACredential(t *testing.T) {
	raw, err := json.Marshal(Event{})
	if err != nil {
		t.Fatal(err)
	}
	var shape map[string]any
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"password", "secret", "credential", "token", "passphrase", "key", "hash"}
	var walk func(string, any)
	walk = func(path string, node any) {
		object, ok := node.(map[string]any)
		if !ok {
			return
		}
		for key, child := range object {
			for _, banned := range forbidden {
				// `synthetic_credential_id` names a credential Guardian planted,
				// never one an attacker supplied, so it is the deliberate
				// exception and is spelled out rather than pattern-matched.
				if strings.Contains(strings.ToLower(key), banned) && key != "synthetic_credential_id" {
					t.Fatalf("the envelope carries %s%s, which could hold a secret", path, key)
				}
			}
			walk(path+key+".", child)
		}
	}
	walk("", shape)

	// And an event that tries to smuggle one is refused rather than ignored:
	// the decoder does not accept unknown fields into Auth.
	event := valid()
	event["auth"] = map[string]any{"result": "failed", "identity": "root", "password": "hunter2"}
	parsed, err := Parse(encode(t, event))
	if err == nil && parsed.Auth != nil {
		// Unknown keys are dropped by encoding/json rather than rejected, so the
		// guarantee is that nothing survives, not that the event is refused.
		if serialised, _ := json.Marshal(parsed); strings.Contains(string(serialised), "hunter2") {
			t.Fatal("a supplied password survived into the canonical event")
		}
	}
}

// The schema file and the Go build must name the same contract version, and the
// vocabularies must match: an event this build accepts and the contract forbids
// is a divergence nobody would notice until ingest.
func TestSchemaFileMatchesTheBuild(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "schemas", "event", "v1", "canonical-event.schema.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read canonical event schema: %v", err)
	}
	var schema struct {
		ID         string `json:"$id"`
		Properties struct {
			Schema struct {
				Const string `json:"const"`
			} `json:"schema"`
			Action struct {
				Enum []string `json:"enum"`
			} `json:"action"`
			Sensitivity struct {
				Enum []string `json:"enum"`
			} `json:"sensitivity"`
			Protocol struct {
				Enum []string `json:"enum"`
			} `json:"protocol"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse canonical event schema: %v", err)
	}
	if schema.Properties.Schema.Const != Schema {
		t.Fatalf("schema file pins %q, build implements %q", schema.Properties.Schema.Const, Schema)
	}
	if !strings.HasSuffix(schema.ID, "/event/v1/canonical-event.schema.json") {
		t.Fatalf("canonical event $id = %q", schema.ID)
	}
	for name, pair := range map[string][2][]string{
		"action":      {schema.Properties.Action.Enum, actionValues},
		"sensitivity": {schema.Properties.Sensitivity.Enum, sensitivity},
		"protocol":    {schema.Properties.Protocol.Enum, protocols},
	} {
		contract, build := pair[0], pair[1]
		if len(contract) != len(build) {
			t.Fatalf("%s: contract has %d values, build has %d", name, len(contract), len(build))
		}
		for index := range contract {
			if contract[index] != build[index] {
				t.Fatalf("%s[%d]: contract %q, build %q", name, index, contract[index], build[index])
			}
		}
	}
}

/*
 * AC-EV-002: from an evidence item the edge, decoy, runtime, schema, and raw
 * reference must be visible. This asserts the envelope can express all five,
 * and that provenance says which of P2-W11's two paths produced the event — a
 * native typed path and a third-party log parser do not warrant the same
 * confidence, and a reader is entitled to know which one this came through.
 */
func TestProvenanceCarriesEverythingAnEvidenceReaderNeeds(t *testing.T) {
	event := valid()
	event["raw_ref"] = map[string]any{
		"blob_id": decoyID, "media_type": "text/plain", "byte_length": 1024,
		"truncated": true, "quarantined": false,
	}
	provenance := event["provenance"].(map[string]any)
	provenance["runtime_version"] = "containerd 1.7.0"
	provenance["adapter_version"] = "0.1.0"

	parsed, err := Parse(encode(t, event))
	if err != nil {
		t.Fatal(err)
	}

	switch {
	case parsed.EdgeID == "" || parsed.DecoyID == "":
		t.Fatal("edge or decoy provenance is missing")
	case parsed.Provenance.RuntimeVersion == nil:
		t.Fatal("runtime provenance is missing")
	case parsed.Schema != Schema:
		t.Fatal("schema provenance is missing")
	case parsed.RawRef == nil || parsed.RawRef.BlobID == "":
		t.Fatal("raw evidence reference is missing")
	case parsed.Provenance.SourcePath != "native_uds":
		t.Fatalf("source path = %q", parsed.Provenance.SourcePath)
	}
	// Truncation is recorded rather than inferred, so a reader never mistakes a
	// truncated transcript for a complete one.
	if !parsed.RawRef.Truncated {
		t.Fatal("truncation was not carried through")
	}
}
