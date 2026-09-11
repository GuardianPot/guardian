package privileged

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
)

var cowrieEntrypoint = ImageEntrypoint{
	Args:       []string{"/cowrie/bin/cowrie", "start", "-n"},
	Env:        []string{"PATH=/usr/bin:/bin", "PYTHONPATH=/cowrie/src"},
	WorkingDir: "/cowrie",
}

func specFor(t *testing.T, workload Workload) (ContainerSpec, string) {
	t.Helper()
	spec, err := BuildContainerSpec(workload, cowrieEntrypoint)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	return spec, string(raw)
}

/*
 * AC-SEC-001 and AC-SEC-002, checked in what containerd would receive.
 *
 * The struct cannot express a bind mount, but the assertion is made against the
 * serialised spec anyway, because that is the thing a runtime acts on. Nothing
 * in it may name the runtime socket, the device key material, or any Guardian
 * state path, and no mount may take its source from the host.
 */
func TestTheSpecGivesTheDecoyNothingOfTheHost(t *testing.T) {
	spec, raw := specFor(t, validWorkload(t))
	for _, forbidden := range []string{
		"containerd.sock", "/run/containerd", "/var/run/docker", "docker.sock",
		"/var/lib/guardian", "/etc/guardian-edge", "guardian-edge-privd",
		`"bind"`, `"rbind"`,
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("the spec mentions %q", forbidden)
		}
	}
	kernelFilesystems := map[string]struct{}{
		"proc": {}, "tmpfs": {}, "shm": {}, "devpts": {}, "mqueue": {}, "sysfs": {},
	}
	for _, mount := range spec.Mounts {
		if strings.HasPrefix(mount.Source, "/") {
			t.Fatalf("%s is sourced from the host path %s", mount.Destination, mount.Source)
		}
		if _, ok := kernelFilesystems[mount.Source]; !ok {
			t.Fatalf("%s has source %q, which is not a kernel filesystem", mount.Destination, mount.Source)
		}
		for _, option := range mount.Options {
			if option == "bind" || option == "rbind" {
				t.Fatalf("%s is a bind mount", mount.Destination)
			}
		}
	}
}

// Every writable area is noexec or /dev: a file a decoy writes is not a file
// it can run, and the root it runs from cannot be changed at all.
func TestTheRootIsReadOnlyAndWritableAreasAreBounded(t *testing.T) {
	spec, _ := specFor(t, validWorkload(t))
	if !spec.Root.Readonly {
		t.Fatal("the root filesystem is writable")
	}
	for _, mount := range spec.Mounts {
		if mount.Type != "tmpfs" {
			continue
		}
		bounded := false
		for _, option := range mount.Options {
			if strings.HasPrefix(option, "size=") {
				bounded = true
			}
		}
		if !bounded {
			t.Fatalf("tmpfs %s has no size bound", mount.Destination)
		}
	}
}

/*
 * The capability sets are exactly the grant — and empty without one.
 *
 * Built from the grant rather than from a default with things removed, so the
 * sets cannot contain anything the workload did not ask for and the closed set
 * did not allow.
 */
func TestCapabilitiesAreExactlyTheGrant(t *testing.T) {
	spec, _ := specFor(t, validWorkload(t))
	capabilities := spec.Process.Capabilities
	for name, set := range map[string][]string{
		"bounding": capabilities.Bounding, "effective": capabilities.Effective,
		"permitted": capabilities.Permitted, "inheritable": capabilities.Inheritable,
		"ambient": capabilities.Ambient,
	} {
		if len(set) != 1 || set[0] != "CAP_NET_BIND_SERVICE" {
			t.Fatalf("%s = %v, want exactly CAP_NET_BIND_SERVICE", name, set)
		}
	}

	ungranted := validWorkload(t)
	ungranted.Privileges.Capabilities = nil
	spec, _ = specFor(t, ungranted)
	capabilities = spec.Process.Capabilities
	for name, set := range map[string][]string{
		"bounding": capabilities.Bounding, "effective": capabilities.Effective,
		"permitted": capabilities.Permitted, "inheritable": capabilities.Inheritable,
		"ambient": capabilities.Ambient,
	} {
		if len(set) != 0 {
			t.Fatalf("%s = %v for a workload that was granted nothing", name, set)
		}
	}
	if !spec.Process.NoNewPrivileges {
		t.Fatal("the decoy may gain privileges through exec")
	}
}

// The image chooses its program. It does not choose who runs it.
func TestTheDecoyRunsAsTheWorkloadsUserNotTheImages(t *testing.T) {
	workload := validWorkload(t)
	spec, _ := specFor(t, workload)
	if spec.Process.User.UID != workload.User.UID || spec.Process.User.GID != workload.User.GID {
		t.Fatalf("user = %+v, want the workload's %+v", spec.Process.User, workload.User)
	}
	if spec.Process.User.UID == 0 || spec.Process.User.GID == 0 {
		t.Fatal("the decoy runs as root")
	}
	if strings.Join(spec.Process.Args, " ") != "/cowrie/bin/cowrie start -n" {
		t.Fatalf("args = %v, want the image's program", spec.Process.Args)
	}
}

/*
 * Every namespace is a new one.
 *
 * The namespace entry has no path field, so nothing can be joined; this checks
 * the set is complete. A decoy without its own network namespace would be on
 * the host's stack, and one without its own pid namespace could see the Edge
 * Agent's processes.
 */
func TestEveryNamespaceIsTheDecoysOwn(t *testing.T) {
	spec, _ := specFor(t, validWorkload(t))
	namespaces, err := json.Marshal(spec.Linux.Namespaces)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(namespaces), `"path"`) {
		t.Fatal("a namespace entry names an existing namespace to join")
	}
	present := map[string]bool{}
	for _, namespace := range spec.Linux.Namespaces {
		present[namespace.Type] = true
	}
	for _, required := range []string{"pid", "ipc", "uts", "mount", "network", "cgroup"} {
		if !present[required] {
			t.Fatalf("the decoy shares the host's %s namespace", required)
		}
	}
	if spec.Hostname == "" || spec.Hostname == "guardian-edge" {
		t.Fatalf("hostname = %q", spec.Hostname)
	}
}

// The limits the workload declared are the limits the kernel enforces.
func TestResourceLimitsAreTheWorkloads(t *testing.T) {
	workload := validWorkload(t)
	spec, _ := specFor(t, workload)
	resources := spec.Linux.Resources
	if *resources.Memory.Limit != int64(workload.Resources.MemoryMiB)<<20 {
		t.Fatalf("memory limit = %d", *resources.Memory.Limit)
	}
	// Swap equal to the limit means no swap at all on top of it.
	if *resources.Memory.Swap != *resources.Memory.Limit {
		t.Fatal("the decoy may swap beyond its memory limit")
	}
	if *resources.CPU.Quota != 50_000 || *resources.CPU.Period != 100_000 {
		t.Fatalf("cpu = %d/%d, want 500 millicores as 50000/100000", *resources.CPU.Quota, *resources.CPU.Period)
	}
	if resources.Pids.Limit != int64(workload.Resources.PIDs) {
		t.Fatalf("pids = %d", resources.Pids.Limit)
	}
	if len(resources.Devices) != 1 || resources.Devices[0].Allow || resources.Devices[0].Access != "rwm" {
		t.Fatalf("devices = %+v, want deny-all", resources.Devices)
	}
	if spec.Linux.CgroupsPath != "/guardian-decoy/"+workload.WorkloadID {
		t.Fatalf("cgroup = %q", spec.Linux.CgroupsPath)
	}
}

/*
 * The seccomp profile is a blocklist, and these are the entries it exists for.
 *
 * Named as a blocklist in the code and the review: weaker than a default-deny
 * allowlist, stronger than nothing, and not to be read as more than it is.
 */
func TestSeccompDeniesTheEscalationSurfaces(t *testing.T) {
	spec, _ := specFor(t, validWorkload(t))
	seccomp := spec.Linux.Seccomp
	if seccomp == nil || len(seccomp.Syscalls) != 1 || seccomp.Syscalls[0].Action != "SCMP_ACT_ERRNO" {
		t.Fatalf("seccomp = %+v", seccomp)
	}
	denied := map[string]bool{}
	for _, name := range seccomp.Syscalls[0].Names {
		if denied[name] {
			t.Fatalf("%s is listed twice", name)
		}
		denied[name] = true
	}
	for _, required := range []string{
		"mount", "umount2", "pivot_root", "setns", "unshare", "ptrace", "bpf",
		"init_module", "finit_module", "kexec_load", "keyctl", "perf_event_open",
		"userfaultfd", "io_uring_setup", "open_tree", "move_mount", "fsopen",
	} {
		if !denied[required] {
			t.Fatalf("%s is not denied", required)
		}
	}
	if !sort.StringsAreSorted(seccomp.Syscalls[0].Names) {
		t.Fatal("the profile is not in a stable order")
	}
}

// Two builds of the same workload are the same bytes, so a recorded digest of
// the spec means something.
func TestTheSpecIsDeterministic(t *testing.T) {
	_, first := specFor(t, validWorkload(t))
	_, second := specFor(t, validWorkload(t))
	if first != second {
		t.Fatal("the same workload produced two different specs")
	}
}

func TestTheBuilderRefusesWhatValidationRefuses(t *testing.T) {
	workload := validWorkload(t)
	workload.User.UID = 0
	if _, err := BuildContainerSpec(workload, cowrieEntrypoint); !errors.Is(err, ErrWorkloadInvalid) {
		t.Fatalf("a root workload = %v, want ErrWorkloadInvalid", err)
	}
	workload = validWorkload(t)
	workload.Privileges.Capabilities = []string{"SYS_ADMIN"}
	if _, err := BuildContainerSpec(workload, cowrieEntrypoint); !errors.Is(err, ErrWorkloadInvalid) {
		t.Fatalf("an ungrantable capability = %v, want ErrWorkloadInvalid", err)
	}
}

func TestAnImageEntrypointIsBoundedAndWellFormed(t *testing.T) {
	workload := validWorkload(t)
	for name, entrypoint := range map[string]ImageEntrypoint{
		"no program":               {},
		"a NUL in an argument":     {Args: []string{"/bin/sh\x00-c"}},
		"too many arguments":       {Args: make([]string, maxEntrypointArgs+1)},
		"an oversized argument":    {Args: []string{strings.Repeat("a", maxEntrypointString+1)}},
		"an env entry with no '='": {Args: []string{"/bin/decoy"}, Env: []string{"PATH"}},
		"an env entry with no name": {
			Args: []string{"/bin/decoy"}, Env: []string{"=value"},
		},
		"a relative working directory": {Args: []string{"/bin/decoy"}, WorkingDir: "cowrie"},
		"a working directory that climbs": {
			Args: []string{"/bin/decoy"}, WorkingDir: "/cowrie/../etc",
		},
	} {
		if _, err := BuildContainerSpec(workload, entrypoint); !errors.Is(err, ErrEntrypointInvalid) {
			t.Fatalf("%s = %v, want ErrEntrypointInvalid", name, err)
		}
	}
	spec, err := BuildContainerSpec(workload, ImageEntrypoint{Args: []string{"/bin/decoy"}})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Process.Cwd != "/" || len(spec.Process.Env) != 1 || !strings.HasPrefix(spec.Process.Env[0], "PATH=") {
		t.Fatalf("defaults = cwd %q env %v", spec.Process.Cwd, spec.Process.Env)
	}
}
