//go:build linux

package privileged

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

/*
 * The holder is the only container Guardian gives a host-sourced mount, and
 * it is Guardian's own binary. Everything else a decoy is denied, the holder
 * is denied too, and its one capability is the one it needs.
 */
func TestTheHolderGetsNetAdminItsBinaryAndNothingElse(t *testing.T) {
	spec, err := buildHolderSpec(validWorkload(t), DefaultHolderBinary, labPlatform)
	if err != nil {
		t.Fatal(err)
	}
	capabilities := spec.Process.Capabilities
	for name, set := range map[string][]string{
		"bounding": capabilities.Bounding, "effective": capabilities.Effective,
		"permitted": capabilities.Permitted, "inheritable": capabilities.Inheritable,
		"ambient": capabilities.Ambient,
	} {
		if len(set) != 1 || set[0] != "CAP_NET_ADMIN" {
			t.Fatalf("%s = %v, want exactly CAP_NET_ADMIN", name, set)
		}
	}
	if spec.Process.User.UID == 0 || spec.Process.User.GID == 0 || !spec.Process.NoNewPrivileges {
		t.Fatalf("process = %+v, want a non-root user with no_new_privs", spec.Process)
	}
	if !spec.Root.Readonly {
		t.Fatal("the holder's root is writable")
	}
	for _, namespace := range spec.Linux.Namespaces {
		if namespace.Path != "" {
			t.Fatalf("the holder joins %s", namespace.Path)
		}
	}
	if len(spec.Linux.Namespaces) != 6 {
		t.Fatalf("namespaces = %+v", spec.Linux.Namespaces)
	}
	binds := 0
	for _, mount := range spec.Mounts {
		if mount.Type != "bind" {
			if strings.HasPrefix(mount.Source, "/") {
				t.Fatalf("%s is sourced from the host", mount.Destination)
			}
			continue
		}
		binds++
		if mount.Source != DefaultHolderBinary || mount.Destination != holderMountPoint {
			t.Fatalf("bind mount = %+v, want only the holder binary", mount)
		}
		for _, required := range []string{"ro", "nosuid", "nodev"} {
			if !hasOption(mount.Options, required) {
				t.Fatalf("the holder binary is mounted without %s", required)
			}
		}
	}
	if binds != 1 || !reflect.DeepEqual(spec.Process.Args, []string{holderMountPoint}) {
		t.Fatalf("binds=%d args=%v", binds, spec.Process.Args)
	}
	// runc cannot start a container without /proc; the lab found this.
	if spec.Mounts[0].Destination != "/proc" || spec.Mounts[0].Type != "proc" {
		t.Fatalf("the holder has no /proc: %+v", spec.Mounts)
	}
	raw, _ := json.Marshal(spec)
	for _, forbidden := range []string{"containerd.sock", "/run/containerd", "/var/lib/guardian", "/etc/guardian-edge"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("the holder spec mentions %q", forbidden)
		}
	}
}

/*
 * A decoy joins exactly one namespace, the network namespace of its holder,
 * and is otherwise the same decoy: no mount, capability, or other namespace
 * changes because it has a network.
 */
func TestADecoyJoinsOnlyItsHoldersNetworkNamespace(t *testing.T) {
	workload := validWorkload(t)
	own, err := buildContainerSpec(workload, cowrieEntrypoint, labPlatform)
	if err != nil {
		t.Fatal(err)
	}
	joined, err := buildJoinedContainerSpec(workload, cowrieEntrypoint, "/proc/4242/ns/net", labPlatform)
	if err != nil {
		t.Fatal(err)
	}
	for index, namespace := range joined.Linux.Namespaces {
		want := own.Linux.Namespaces[index]
		if namespace.Type == "network" {
			want.Path = "/proc/4242/ns/net"
		}
		if namespace != want {
			t.Fatalf("namespace %d = %+v, want %+v", index, namespace, want)
		}
	}
	joined.Linux.Namespaces, own.Linux.Namespaces = nil, nil
	if !reflect.DeepEqual(joined, own) {
		t.Fatal("joining the holder changed something besides the network namespace")
	}
}

func TestADecoyCanJoinNothingButAHoldersNetworkNamespace(t *testing.T) {
	workload := validWorkload(t)
	for _, path := range []string{
		"/proc/1/ns/net", "/proc/0/ns/net", "/proc/self/ns/net", "/proc/42/ns/pid",
		"/var/run/netns/decoy", "/proc/42/ns/net/../../1/ns/net", "/proc/42/root/proc/1/ns/net",
		"/proc/042/ns/net", "", "/proc/42/ns/net ",
	} {
		if _, err := buildJoinedContainerSpec(workload, cowrieEntrypoint, path, labPlatform); !errors.Is(err, ErrWorkloadInvalid) {
			t.Fatalf("%q = %v, want ErrWorkloadInvalid", path, err)
		}
	}
}

// A holder's id cannot be any workload's, so allowlisting a workload can never
// hand the Edge Agent another decoy's holder.
func TestAHolderIsNeverAWorkload(t *testing.T) {
	if resourcePattern.MatchString(holderID(testWorkloadID)) {
		t.Fatalf("%q is a valid workload id", holderID(testWorkloadID))
	}
}

func TestAHolderBinaryAnyoneButRootCouldReplaceIsRefused(t *testing.T) {
	directory := t.TempDir()
	write := func(name string, mode os.FileMode) string {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte("\x7fELF"), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(write("target", 0o755), link); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"group-writable":  write("group", 0o775),
		"world-writable":  write("world", 0o757),
		"a symlink":       link,
		"a directory":     directory,
		"a missing path":  filepath.Join(directory, "absent"),
		"an empty string": "",
	} {
		if err := verifyHolderBinary(path); !errors.Is(err, ErrHolderBinaryUntrusted) {
			t.Fatalf("%s = %v, want ErrHolderBinaryUntrusted", name, err)
		}
	}
	// A file only root can write is accepted when root owns it, which in this
	// test is only true when the test itself runs as root.
	err := verifyHolderBinary(write("owned", 0o755))
	if os.Geteuid() == 0 && err != nil {
		t.Fatalf("a root-owned 0755 binary = %v", err)
	}
	if os.Geteuid() != 0 && !errors.Is(err, ErrHolderBinaryUntrusted) {
		t.Fatalf("a binary this user owns = %v, want ErrHolderBinaryUntrusted", err)
	}
}

func hasOption(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
