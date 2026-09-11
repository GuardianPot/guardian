package privileged

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testWorkloadID = "guardian-workload-ssh-a"
	testDigest     = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func workloadJSON(overrides map[string]string) string {
	fields := map[string]string{
		"schema":       `"guardian.workload.v1"`,
		"workload_id":  `"` + testWorkloadID + `"`,
		"pack":         `"ssh-cowrie"`,
		"pack_version": `"0.1.0"`,
		"image":        `{"repository": "registry.example.internal/guardian/ssh-cowrie", "digest": "` + testDigest + `"}`,
		"ports":        `[{"port": 22, "protocol": "tcp"}]`,
		"privileges":   `{"capabilities": ["NET_BIND_SERVICE"]}`,
		"resources":    `{"cpu_millicores": 500, "memory_mib": 256, "pids": 128}`,
		"user":         `{"uid": 10001, "gid": 10001}`,
	}
	for key, value := range overrides {
		if value == "" {
			delete(fields, key)
			continue
		}
		fields[key] = value
	}
	parts := make([]string, 0, len(fields))
	for key, value := range fields {
		parts = append(parts, `"`+key+`": `+value)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func validWorkload(t *testing.T) Workload {
	t.Helper()
	workload, err := ParseWorkload([]byte(workloadJSON(nil)), testWorkloadID)
	if err != nil {
		t.Fatal(err)
	}
	return workload
}

func TestAValidDefinitionResolvesToADigestPinnedImage(t *testing.T) {
	workload := validWorkload(t)
	if got, want := workload.ImageReference(), "registry.example.internal/guardian/ssh-cowrie@"+testDigest; got != want {
		t.Fatalf("image reference = %q, want %q", got, want)
	}
}

// The version is read before anything else, so a definition written for a
// future format is refused whole rather than half-honoured.
func TestTheSchemaIsCheckedBeforeAnythingElse(t *testing.T) {
	raw := workloadJSON(map[string]string{
		"schema": `"guardian.workload.v2"`,
		"user":   `{"uid": 0, "gid": 0}`,
	})
	if _, err := ParseWorkload([]byte(raw), testWorkloadID); !errors.Is(err, ErrWorkloadSchema) {
		t.Fatalf("err = %v, want ErrWorkloadSchema", err)
	}
}

/*
 * Every way a definition could grant a decoy more than it should is refused.
 *
 * The file is root-owned, so an attacker is not the expected author. The
 * expected author is an operator making a mistake, and each case below is a
 * mistake that would otherwise have produced a weaker decoy without anyone
 * noticing.
 */
func TestADefinitionThatWouldWeakenTheDecoyIsRefused(t *testing.T) {
	for name, overrides := range map[string]map[string]string{
		// A tag can be moved to different content after it was approved.
		"a tag instead of a digest": {
			"image": `{"repository": "registry.example.internal/guardian/ssh-cowrie", "digest": "latest"}`,
		},
		"a digest with the wrong algorithm": {
			"image": `{"repository": "registry.example.internal/guardian/ssh-cowrie", "digest": "md5:0123"}`,
		},
		"a digest in upper case": {
			"image": `{"repository": "registry.example.internal/guardian/ssh-cowrie", "digest": "` + strings.ToUpper(testDigest) + `"}`,
		},
		"a repository with a scheme": {
			"image": `{"repository": "https://registry.example.internal/x", "digest": "` + testDigest + `"}`,
		},
		"a repository with a tag folded in": {
			"image": `{"repository": "registry.example.internal/x:latest", "digest": "` + testDigest + `"}`,
		},
		"a repository that climbs": {
			"image": `{"repository": "registry.example.internal/../x", "digest": "` + testDigest + `"}`,
		},
		"a capability outside the closed set": {"privileges": `{"capabilities": ["SYS_ADMIN"]}`},
		"raw sockets":                         {"privileges": `{"capabilities": ["NET_RAW"]}`},
		"every capability":                    {"privileges": `{"capabilities": ["ALL"]}`},
		"the one capability twice": {
			"privileges": `{"capabilities": ["NET_BIND_SERVICE", "NET_BIND_SERVICE"]}`,
		},
		"root inside the container":  {"user": `{"uid": 0, "gid": 10001}`},
		"the root group":             {"user": `{"uid": 10001, "gid": 0}`},
		"no user at all":             {"user": ""},
		"no resource limits":         {"resources": ""},
		"an unbounded memory limit":  {"resources": `{"cpu_millicores": 500, "memory_mib": 0, "pids": 128}`},
		"an unbounded process count": {"resources": `{"cpu_millicores": 500, "memory_mib": 256, "pids": 0}`},
		"no ports":                   {"ports": `[]`},
		"a port out of range":        {"ports": `[{"port": 70000, "protocol": "tcp"}]`},
		"an invented protocol":       {"ports": `[{"port": 22, "protocol": "sctp"}]`},
		// An unknown field is a restriction its author believed they had set.
		"a field this build does not know": {"privileged": `true`},
		"a mount, which has no field":      {"mounts": `[{"source": "/run/containerd/containerd.sock"}]`},
		"a pack name that is a path":       {"pack": `"../ssh"`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseWorkload([]byte(workloadJSON(overrides)), testWorkloadID)
			if err == nil {
				t.Fatal("the definition was accepted")
			}
			if !errors.Is(err, ErrWorkloadInvalid) {
				t.Fatalf("err = %v, want ErrWorkloadInvalid", err)
			}
		})
	}
}

// The id inside the file must be the id it was looked up by, so one workload's
// definition cannot be installed under another's name.
func TestADefinitionMustNameItself(t *testing.T) {
	raw := workloadJSON(map[string]string{"workload_id": `"guardian-workload-other"`})
	if _, err := ParseWorkload([]byte(raw), testWorkloadID); !errors.Is(err, ErrWorkloadInvalid) {
		t.Fatalf("err = %v, want ErrWorkloadInvalid", err)
	}
}

func TestADefinitionIsOneBoundedDocument(t *testing.T) {
	if _, err := ParseWorkload([]byte(workloadJSON(nil)+"{}"), testWorkloadID); !errors.Is(err, ErrWorkloadInvalid) {
		t.Fatalf("trailing document = %v, want ErrWorkloadInvalid", err)
	}
	oversized := make([]byte, MaxWorkloadBytes+1)
	if _, err := ParseWorkload(oversized, testWorkloadID); !errors.Is(err, ErrWorkloadTooLarge) {
		t.Fatalf("oversized = %v, want ErrWorkloadTooLarge", err)
	}
}

/*
 * The id is validated before it becomes part of a path.
 *
 * The Edge Agent chooses the id. Joining it into a filename unchecked would let
 * it choose which file a root process reads.
 */
func TestAWorkloadIDCannotReachOutsideTheDirectory(t *testing.T) {
	directory := t.TempDir()
	for _, id := range []string{
		"../../etc/shadow", "guardian-a/../../b", "/etc/passwd", "guardian-UPPER", "", "guardian-a.json",
	} {
		if _, err := LoadWorkload(directory, id); !errors.Is(err, ErrWorkloadInvalid) {
			t.Fatalf("LoadWorkload(%q) = %v, want ErrWorkloadInvalid", id, err)
		}
	}
}

func TestLoadingFromTheRootOwnedDirectory(t *testing.T) {
	directory := t.TempDir()
	if _, err := LoadWorkload(directory, testWorkloadID); !errors.Is(err, ErrWorkloadNotFound) {
		t.Fatalf("an uninstalled workload = %v, want ErrWorkloadNotFound", err)
	}
	path := filepath.Join(directory, testWorkloadID+".json")
	if err := os.WriteFile(path, []byte(workloadJSON(nil)), 0o600); err != nil {
		t.Fatal(err)
	}
	workload, err := LoadWorkload(directory, testWorkloadID)
	if err != nil {
		t.Fatal(err)
	}
	if workload.WorkloadID != testWorkloadID {
		t.Fatalf("loaded %q", workload.WorkloadID)
	}
}

// A symlink in the definition directory would let something outside it decide
// what a privileged process runs. It is refused, not followed.
func TestADefinitionThatIsNotARegularFileIsRefused(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(t.TempDir(), "elsewhere.json")
	if err := os.WriteFile(target, []byte(workloadJSON(nil)), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, testWorkloadID+".json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("this platform cannot create a symlink here: %v", err)
	}
	if _, err := LoadWorkload(directory, testWorkloadID); !errors.Is(err, ErrWorkloadInvalid) {
		t.Fatalf("a symlinked definition = %v, want ErrWorkloadInvalid", err)
	}
}
