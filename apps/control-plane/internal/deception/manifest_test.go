package deception

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The manifest of the SSH pack, as committed, used as the base every negative
// case mutates. Starting from a real manifest means a rejection is caused by
// the one thing the case changed.
func validManifest(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "decoys", "ssh-cowrie", "manifest.json"))
	if err != nil {
		t.Fatalf("read ssh-cowrie manifest: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse ssh-cowrie manifest: %v", err)
	}
	return manifest
}

func encode(t *testing.T, manifest map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

/*
 * P2-W4 acceptance: unsupported privilege request rejected.
 *
 * This is the criterion the package exists for. A manifest asking for authority
 * Guardian will not grant is the one validation failure that is a security
 * event rather than a typo, so it has its own sentinel and the rejection names
 * what was refused.
 */
func TestUnsupportedPrivilegeRequestIsRejected(t *testing.T) {
	for _, capability := range []string{
		"CAP_SYS_ADMIN",
		"CAP_NET_ADMIN",
		"CAP_NET_RAW",
		"CAP_SYS_PTRACE",
		"CAP_DAC_OVERRIDE",
		"SYS_ADMIN",
		"ALL",
		"",
	} {
		manifest := validManifest(t)
		manifest["privileges"] = map[string]any{"capabilities": []string{capability}}

		_, err := ParseManifest(encode(t, manifest))

		if !errors.Is(err, ErrManifestPrivilege) {
			t.Fatalf("capability %q = %v, want ErrManifestPrivilege", capability, err)
		}
		// The pack author has to learn which request was refused; a generic
		// failure sends them looking through the whole manifest.
		if capability != "" && !strings.Contains(err.Error(), capability) {
			t.Fatalf("rejection of %q does not name it: %v", capability, err)
		}
	}
}

// The grant that does exist is granted, so the check above is not simply
// refusing everything.
func TestTheOneGrantableCapabilityIsAccepted(t *testing.T) {
	manifest := validManifest(t)
	manifest["privileges"] = map[string]any{"capabilities": []string{"NET_BIND_SERVICE"}}
	if _, err := ParseManifest(encode(t, manifest)); err != nil {
		t.Fatalf("NET_BIND_SERVICE was refused: %v", err)
	}
	// And an empty request is the expected case, not an error.
	manifest["privileges"] = map[string]any{"capabilities": []string{}}
	if _, err := ParseManifest(encode(t, manifest)); err != nil {
		t.Fatalf("a manifest asking for nothing was refused: %v", err)
	}
}

// A duplicate is refused too. Two grants of the same capability is not twice
// the authority, but it is a manifest nobody reviewed carefully.
func TestDuplicateCapabilityIsRejected(t *testing.T) {
	manifest := validManifest(t)
	manifest["privileges"] = map[string]any{
		"capabilities": []string{"NET_BIND_SERVICE", "NET_BIND_SERVICE"},
	}
	if _, err := ParseManifest(encode(t, manifest)); !errors.Is(err, ErrManifestPrivilege) {
		t.Fatalf("duplicate capability = %v, want ErrManifestPrivilege", err)
	}
}

/*
 * P2-W4 acceptance: manifest version mismatch explicit.
 *
 * A manifest this build does not implement is refused with its own error and is
 * never partially read. Every field below the version is interpreted under
 * rules that may not apply to it, so reporting a field error for a document
 * written against a different contract sends the reader after the wrong thing.
 */
func TestManifestVersionMismatchIsExplicit(t *testing.T) {
	for _, schema := range []string{
		"guardian.decoy.manifest.v2",
		"guardian.decoy.manifest.v0",
		"guardian.decoy.pack_index.v1",
		"",
	} {
		manifest := validManifest(t)
		manifest["schema"] = schema
		// Also break a field, to prove the version is reported rather than the
		// consequence of reading a document written to other rules.
		manifest["ports"] = []any{map[string]any{"port": 0, "protocol": "sctp"}}

		_, err := ParseManifest(encode(t, manifest))

		if !errors.Is(err, ErrManifestVersion) {
			t.Fatalf("schema %q = %v, want ErrManifestVersion", schema, err)
		}
		if errors.Is(err, ErrManifestInvalid) {
			t.Fatalf("schema %q was reported as a field error: %v", schema, err)
		}
	}
}

// An unknown property is refused rather than ignored: a manifest carrying one
// is a manifest whose author expected something this build will not do.
func TestUnknownManifestPropertyIsRejected(t *testing.T) {
	for _, field := range []string{"image", "command", "entrypoint", "mounts", "privileged", "host_network"} {
		manifest := validManifest(t)
		manifest[field] = "anything"
		if _, err := ParseManifest(encode(t, manifest)); !errors.Is(err, ErrManifestInvalid) {
			t.Fatalf("unknown property %q = %v, want ErrManifestInvalid", field, err)
		}
	}
}

// AC-SEC-003. `deny` is the only policy, and an exception must name a private
// destination and say why it exists.
func TestEgressCannotBeOpenedUp(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"a permissive policy": func(m map[string]any) {
			m["egress"] = map[string]any{"policy": "allow", "allow": []any{}}
		},
		"a public destination": func(m map[string]any) {
			m["egress"] = map[string]any{"policy": "deny", "allow": []any{map[string]any{
				"destination": "8.8.8.8", "port": 53, "protocol": "udp", "reason": "dns",
			}}}
		},
		"an unexplained exception": func(m map[string]any) {
			m["egress"] = map[string]any{"policy": "deny", "allow": []any{map[string]any{
				"destination": "10.20.0.5", "port": 443, "protocol": "tcp", "reason": "",
			}}}
		},
	} {
		manifest := validManifest(t)
		mutate(manifest)
		if _, err := ParseManifest(encode(t, manifest)); !errors.Is(err, ErrManifestInvalid) {
			t.Fatalf("%s = %v, want ErrManifestInvalid", name, err)
		}
	}
}

// A probe against a port the pack never listens on would report a healthy decoy
// as absent, or an absent one as healthy, depending on what else is bound.
func TestHealthProbeMustTargetADeclaredPort(t *testing.T) {
	manifest := validManifest(t)
	manifest["health_probe"] = map[string]any{"type": "tcp_connect", "port": 9999, "timeout_seconds": 5}
	if _, err := ParseManifest(encode(t, manifest)); !errors.Is(err, ErrManifestInvalid) {
		t.Fatalf("probe on an undeclared port = %v, want ErrManifestInvalid", err)
	}
}

func TestPlacementAndAdapterAreClosedSets(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"host networking": func(m map[string]any) {
			m["network"] = map[string]any{"mode": "host_network"}
		},
		"an adapter that does not exist": func(m map[string]any) {
			m["telemetry"] = map[string]any{"adapter": "shell-exec"}
		},
		"a persona outside DC-11": func(m map[string]any) {
			persona := m["persona"].(map[string]any)
			persona["personas"] = []string{"domain_controller"}
		},
	} {
		manifest := validManifest(t)
		mutate(manifest)
		if _, err := ParseManifest(encode(t, manifest)); !errors.Is(err, ErrManifestInvalid) {
			t.Fatalf("%s = %v, want ErrManifestInvalid", name, err)
		}
	}
}

// An unbounded decoy is a denial-of-service surface on the host that is
// supposed to be protecting the network, so the bounds are required.
func TestResourceBoundsAreRequired(t *testing.T) {
	for name, resources := range map[string]map[string]any{
		"no limits at all":       {},
		"an unbounded cpu":       {"cpu_millicores": 0, "memory_mib": 256, "pids": 128},
		"an excessive memory":    {"cpu_millicores": 500, "memory_mib": 65536, "pids": 128},
		"an unbounded pid count": {"cpu_millicores": 500, "memory_mib": 256, "pids": 0},
	} {
		manifest := validManifest(t)
		manifest["resources"] = resources
		if _, err := ParseManifest(encode(t, manifest)); !errors.Is(err, ErrManifestInvalid) {
			t.Fatalf("%s = %v, want ErrManifestInvalid", name, err)
		}
	}
}

/*
 * Every committed manifest is valid and agrees with the pack index.
 *
 * The index is what the Control Plane resolves a decoy's family and interaction
 * level from; the manifest is what P2-W3 will build a runtime from. If they
 * disagree, a decoy renders as one thing in the console and behaves as another.
 */
func TestCommittedManifestsAreValidAndAgreeWithTheIndex(t *testing.T) {
	packs := Packs()
	if len(packs) == 0 {
		t.Fatal("the pack index is empty")
	}
	families := make(map[Family]struct{}, len(packs))
	for _, entry := range packs {
		path := filepath.Join("..", "..", "..", "..", "decoys", entry.Pack, "manifest.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("every indexed pack needs a manifest: %v", err)
		}
		manifest, err := ParseManifest(raw)
		if err != nil {
			t.Fatalf("%s manifest is invalid: %v", entry.Pack, err)
		}
		switch {
		case manifest.Pack != entry.Pack:
			t.Fatalf("%s manifest names pack %q", entry.Pack, manifest.Pack)
		case manifest.Version != entry.Version:
			t.Fatalf("%s manifest is version %q, index says %q", entry.Pack, manifest.Version, entry.Version)
		case manifest.Family != entry.Family:
			t.Fatalf("%s manifest is family %q, index says %q", entry.Pack, manifest.Family, entry.Family)
		case manifest.InteractionLevel != entry.InteractionLevel:
			t.Fatalf("%s manifest is %q interaction, index says %q",
				entry.Pack, manifest.InteractionLevel, entry.InteractionLevel)
		}
		// Every pack denies egress. Cowrie is medium-interaction software and
		// this is the field that keeps it off the internet (AC-SEC-003).
		if manifest.Egress.Policy != "deny" || len(manifest.Egress.Allow) != 0 {
			t.Fatalf("%s manifest opens egress: %+v", entry.Pack, manifest.Egress)
		}
		families[manifest.Family] = struct{}{}
	}
	// One pack per DC-01 family, which is what the Phase 2 exit gate needs.
	if len(families) != 4 {
		t.Fatalf("manifests cover %d families, want all four", len(families))
	}
}

// The schema file and the Go build must name the same contract version.
func TestManifestSchemaFileMatchesTheBuild(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "schemas", "decoy", "v1", "decoy-manifest.schema.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest schema: %v", err)
	}
	var schema struct {
		ID         string `json:"$id"`
		Properties struct {
			Schema struct {
				Const string `json:"const"`
			} `json:"schema"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse manifest schema: %v", err)
	}
	if schema.Properties.Schema.Const != ManifestSchema {
		t.Fatalf("schema file pins %q, build implements %q", schema.Properties.Schema.Const, ManifestSchema)
	}
	if !strings.HasSuffix(schema.ID, "/decoy/v1/decoy-manifest.schema.json") {
		t.Fatalf("manifest schema $id = %q", schema.ID)
	}
}
