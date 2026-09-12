package privileged

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

/*
The OCI runtime spec Guardian writes for a decoy.

This file is where `AC-SEC-001` and `AC-SEC-002` are actually decided. Every
other statement about decoy isolation in this repository is a statement about
what cannot be *asked for*; this is the one place that says what is *granted*.

It is a small hand-written spec rather than a general one on purpose. The
runtime specification has hundreds of fields, most of which loosen something,
and a builder that starts from a full struct grants whatever it forgets to
clear. This one starts from nothing and adds only what a decoy needs, so the
things that are absent — host namespaces, a privileged flag, a device
allowance, a bind mount of anything at all — are absent because there is no
code here that could produce them.

The rule for reading it: if a grant is not written below, a decoy does not get
it.
*/

const (
	ociVersion = "1.2.0"

	// The container's own view. A decoy has no reason to know the host's name,
	// and a real hostname would be a fact about the customer's estate leaking
	// into software an attacker is talking to. P2-W5..W8 set a persona
	// hostname; until one does, this is deliberately generic.
	defaultDecoyHostname = "localhost"

	// cpuQuotaPeriod is the standard 100ms accounting window. A millicore
	// budget becomes a quota against it.
	cpuQuotaPeriod = 100_000
)

// ContainerSpec is the subset of the OCI runtime specification Guardian sets.
// Fields are omitted rather than zeroed where the runtime's default is already
// the restrictive answer.
type ContainerSpec struct {
	Version  string       `json:"ociVersion"`
	Process  *specProcess `json:"process"`
	Root     *specRoot    `json:"root"`
	Hostname string       `json:"hostname"`
	Mounts   []specMount  `json:"mounts"`
	Linux    *specLinux   `json:"linux"`
}

type specProcess struct {
	Terminal        bool              `json:"terminal"`
	User            specUser          `json:"user"`
	Args            []string          `json:"args,omitempty"`
	Env             []string          `json:"env"`
	Cwd             string            `json:"cwd"`
	Capabilities    *specCapabilities `json:"capabilities"`
	NoNewPrivileges bool              `json:"noNewPrivileges"`
	OOMScoreAdj     *int              `json:"oomScoreAdj,omitempty"`
}

type specUser struct {
	UID int `json:"uid"`
	GID int `json:"gid"`
}

type specCapabilities struct {
	Bounding    []string `json:"bounding"`
	Effective   []string `json:"effective"`
	Permitted   []string `json:"permitted"`
	Inheritable []string `json:"inheritable"`
	Ambient     []string `json:"ambient"`
}

type specRoot struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly"`
}

type specMount struct {
	Destination string   `json:"destination"`
	Type        string   `json:"type"`
	Source      string   `json:"source"`
	Options     []string `json:"options"`
}

type specLinux struct {
	Namespaces    []specNamespace `json:"namespaces"`
	Resources     *specResources  `json:"resources"`
	MaskedPaths   []string        `json:"maskedPaths"`
	ReadonlyPaths []string        `json:"readonlyPaths"`
	Seccomp       *specSeccomp    `json:"seccomp,omitempty"`
	CgroupsPath   string          `json:"cgroupsPath,omitempty"`
}

type specNamespace struct {
	Type string `json:"type"`
	// Path joins an existing namespace instead of creating one. It is set in
	// exactly one case: a decoy's network namespace, to its own holder's
	// (ADR 0019), and only in the form holderNamespacePattern admits. Every
	// other namespace, and every holder namespace, is new.
	Path string `json:"path,omitempty"`
}

type specResources struct {
	Memory  *specMemory        `json:"memory"`
	CPU     *specCPU           `json:"cpu"`
	Pids    *specPids          `json:"pids"`
	Devices []specDeviceCgroup `json:"devices"`
}

type specMemory struct {
	Limit *int64 `json:"limit"`
	Swap  *int64 `json:"swap"`
}

type specCPU struct {
	Quota  *int64  `json:"quota"`
	Period *uint64 `json:"period"`
}

type specPids struct {
	Limit int64 `json:"limit"`
}

type specDeviceCgroup struct {
	Allow  bool   `json:"allow"`
	Type   string `json:"type,omitempty"`
	Access string `json:"access,omitempty"`
}

type specSeccomp struct {
	DefaultAction   string        `json:"defaultAction"`
	DefaultErrnoRet *uint         `json:"defaultErrnoRet,omitempty"`
	Architectures   []string      `json:"architectures,omitempty"`
	Syscalls        []specSyscall `json:"syscalls"`
}

type specSyscall struct {
	Names    []string         `json:"names"`
	Action   string           `json:"action"`
	ErrnoRet *uint            `json:"errnoRet,omitempty"`
	Args     []specSyscallArg `json:"args,omitempty"`
}

type specSyscallArg struct {
	Index    uint   `json:"index"`
	Value    uint64 `json:"value"`
	ValueTwo uint64 `json:"valueTwo,omitempty"`
	Op       string `json:"op"`
}

// ImageEntrypoint is what the image's own configuration says to run. The OCI
// spec requires a command, and the pack's image is the only thing that knows
// its program, so this is the one part of the spec that comes from image
// content rather than from Guardian.
//
// It is accepted because it cannot grant anything: it runs inside the
// container, as the uid Guardian chose, with the capabilities Guardian chose.
// What the image's configuration may *not* do is choose its user, add a
// capability, or name a mount — there is no parameter here through which it
// could.
type ImageEntrypoint struct {
	Args       []string
	Env        []string
	WorkingDir string
}

const (
	maxEntrypointArgs   = 64
	maxEntrypointEnv    = 128
	maxEntrypointString = 4096
)

var ErrEntrypointInvalid = errors.New("image entrypoint is invalid")

func (e ImageEntrypoint) validate() error {
	if len(e.Args) == 0 || len(e.Args) > maxEntrypointArgs {
		return fmt.Errorf("%w: args", ErrEntrypointInvalid)
	}
	if len(e.Env) > maxEntrypointEnv {
		return fmt.Errorf("%w: env", ErrEntrypointInvalid)
	}
	for _, value := range append(append([]string{}, e.Args...), e.Env...) {
		if len(value) > maxEntrypointString || strings.ContainsRune(value, 0) {
			return fmt.Errorf("%w: value", ErrEntrypointInvalid)
		}
	}
	for _, variable := range e.Env {
		name, _, found := strings.Cut(variable, "=")
		if !found || name == "" {
			return fmt.Errorf("%w: env entry", ErrEntrypointInvalid)
		}
	}
	if e.WorkingDir != "" && (!path.IsAbs(e.WorkingDir) || path.Clean(e.WorkingDir) != e.WorkingDir) {
		return fmt.Errorf("%w: working directory", ErrEntrypointInvalid)
	}
	return nil
}

/*
BuildContainerSpec produces the spec for one workload.

Everything a decoy is denied is denied by construction:

  - The root filesystem is read-only, so a decoy cannot persist a change it
    makes; whatever it needs to write goes to the tmpfs mounts below.
  - Every namespace is new and none has a path, so nothing is joined. There is
    no host network, no host pid, no host ipc.
  - The capability sets are exactly what the workload declared, validated
    against a closed set of one. Not "the default minus some" — the sets are
    built from the grant and are empty when there is no grant.
  - No mount has a host source. The only sources are `proc`, `tmpfs`,
    `devpts`, `mqueue`, and `sysfs`, which are kernel filesystems rather than
    host paths. `AC-SEC-002` (no runtime socket) and `AC-SEC-001` (no device
    key path) hold because there is no code path here that can emit a bind
    mount at all.
  - Resource limits are always present, because the workload validation
    already refused a definition without them.
*/
func BuildContainerSpec(workload Workload, entrypoint ImageEntrypoint) (ContainerSpec, error) {
	return buildContainerSpec(workload, entrypoint, hostPlatform())
}

// BuildJoinedContainerSpec is the decoy spec with its network namespace joined
// to its holder's. The path is checked against the one form it may take before
// it is written.
func BuildJoinedContainerSpec(workload Workload, entrypoint ImageEntrypoint, networkNamespace string) (ContainerSpec, error) {
	return buildJoinedContainerSpec(workload, entrypoint, networkNamespace, hostPlatform())
}

func buildJoinedContainerSpec(workload Workload, entrypoint ImageEntrypoint, networkNamespace string, platform specPlatform) (ContainerSpec, error) {
	if !holderNamespacePattern.MatchString(networkNamespace) {
		return ContainerSpec{}, fmt.Errorf("%w: network namespace", ErrWorkloadInvalid)
	}
	spec, err := buildContainerSpec(workload, entrypoint, platform)
	if err != nil {
		return ContainerSpec{}, err
	}
	for index := range spec.Linux.Namespaces {
		if spec.Linux.Namespaces[index].Type == "network" {
			spec.Linux.Namespaces[index].Path = networkNamespace
		}
	}
	return spec, nil
}

func buildContainerSpec(workload Workload, entrypoint ImageEntrypoint, platform specPlatform) (ContainerSpec, error) {
	if err := workload.validate(); err != nil {
		return ContainerSpec{}, err
	}
	if err := entrypoint.validate(); err != nil {
		return ContainerSpec{}, err
	}
	granted := grantedCapabilities(workload)
	seccomp, err := translateSeccomp(granted, platform)
	if err != nil {
		// No decoy runs without a profile. A platform the profile does not
		// cover is a platform this build does not deploy decoys on.
		return ContainerSpec{}, err
	}
	env := entrypoint.Env
	if len(env) == 0 {
		env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	}
	cwd := entrypoint.WorkingDir
	if cwd == "" {
		cwd = "/"
	}
	memoryLimit := int64(workload.Resources.MemoryMiB) << 20
	cpuQuota := int64(workload.Resources.CPUMillicores) * cpuQuotaPeriod / 1000
	period := uint64(cpuQuotaPeriod)
	// A decoy is the first thing that should die under host memory pressure.
	// It is bait; the agent watching it is not.
	oomScore := 500

	return ContainerSpec{
		Version: ociVersion,
		Process: &specProcess{
			Terminal: false,
			// The workload's user, never the image's: an image configured to
			// run as root does not get to.
			User: specUser{UID: workload.User.UID, GID: workload.User.GID},
			Args: append([]string(nil), entrypoint.Args...),
			Env:  append([]string(nil), env...),
			Cwd:  cwd,
			Capabilities: &specCapabilities{
				Bounding:    granted,
				Effective:   granted,
				Permitted:   granted,
				Inheritable: granted,
				// Ambient, so an unprivileged decoy process keeps the one
				// capability it was granted across exec. Without it a decoy
				// running as a non-root user could not bind port 22, and the
				// workaround would be to run it as root.
				Ambient: granted,
			},
			NoNewPrivileges: true,
			OOMScoreAdj:     &oomScore,
		},
		Root: &specRoot{Path: "rootfs", Readonly: true},
		// Not the host's name: see defaultDecoyHostname.
		Hostname: defaultDecoyHostname,
		Mounts:   decoyMounts(workload),
		Linux: &specLinux{
			Namespaces: []specNamespace{
				{Type: "pid"}, {Type: "ipc"}, {Type: "uts"},
				{Type: "mount"}, {Type: "network"}, {Type: "cgroup"},
			},
			Resources: &specResources{
				Memory: &specMemory{Limit: &memoryLimit, Swap: &memoryLimit},
				CPU:    &specCPU{Quota: &cpuQuota, Period: &period},
				Pids:   &specPids{Limit: int64(workload.Resources.PIDs)},
				// Deny every device. The tmpfs /dev below supplies the handful
				// of character devices a process needs, and nothing else is
				// reachable.
				Devices: []specDeviceCgroup{{Allow: false, Access: "rwm"}},
			},
			MaskedPaths:   maskedPaths(),
			ReadonlyPaths: readonlyPaths(),
			Seccomp:       seccomp,
			CgroupsPath:   workload.CgroupPath(),
		},
	}, nil
}

func grantedCapabilities(workload Workload) []string {
	granted := make([]string, 0, len(workload.Privileges.Capabilities))
	for _, capability := range workload.Privileges.Capabilities {
		granted = append(granted, "CAP_"+capability)
	}
	sort.Strings(granted)
	return granted
}

/*
decoyMounts is every filesystem a decoy gets.

There is no `source` here that names a host path. Each entry is a kernel
filesystem the container gets its own instance of, so this list cannot express
"give the decoy something of the host's" — which is the property `AC-SEC-002`
needs, and it is structural rather than a check that could be forgotten.

The writable areas are tmpfs and bounded, because the root filesystem is
read-only and a decoy still has to be able to write a log its telemetry adapter
will read.
*/
func decoyMounts(workload Workload) []specMount {
	return []specMount{
		{Destination: "/proc", Type: "proc", Source: "proc",
			Options: []string{"nosuid", "noexec", "nodev"}},
		{Destination: "/dev", Type: "tmpfs", Source: "tmpfs",
			Options: []string{"nosuid", "strictatime", "mode=755", "size=65536k"}},
		{Destination: "/dev/pts", Type: "devpts", Source: "devpts",
			Options: []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"}},
		{Destination: "/dev/shm", Type: "tmpfs", Source: "shm",
			Options: []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"}},
		{Destination: "/dev/mqueue", Type: "mqueue", Source: "mqueue",
			Options: []string{"nosuid", "noexec", "nodev"}},
		{Destination: "/sys", Type: "sysfs", Source: "sysfs",
			Options: []string{"nosuid", "noexec", "nodev", "ro"}},
		{Destination: "/tmp", Type: "tmpfs", Source: "tmpfs",
			Options: []string{"nosuid", "nodev", "mode=1777", "size=32m"}},
		// Where a pack writes what its telemetry adapter reads. Bounded, and
		// noexec so that a file a decoy writes is not a file it can run.
		{Destination: "/var/log/guardian", Type: "tmpfs", Source: "tmpfs",
			Options: []string{"nosuid", "nodev", "noexec", "mode=0750", "size=32m",
				"uid=" + strconv.Itoa(workload.User.UID), "gid=" + strconv.Itoa(workload.User.GID)}},
	}
}

// maskedPaths hides kernel interfaces that describe the host. Several of these
// are how a process works out that it is in a container and what the machine
// around it is; a decoy learning that is a decoy telling an attacker.
func maskedPaths() []string {
	return []string{
		"/proc/acpi", "/proc/asound", "/proc/interrupts", "/proc/kcore",
		"/proc/keys", "/proc/latency_stats", "/proc/sched_debug",
		"/proc/scsi", "/proc/timer_list", "/proc/timer_stats",
		"/sys/firmware", "/sys/devices/virtual/powercap",
	}
}

func readonlyPaths() []string {
	return []string{
		"/proc/bus", "/proc/fs", "/proc/irq", "/proc/sys", "/proc/sysrq-trigger",
	}
}

// CgroupPath is the slice a workload's container is accounted under, so an
// operator can see Guardian's decoys as a group and nothing else lands in it.
func (w Workload) CgroupPath() string {
	return "/guardian-decoy/" + w.WorkloadID
}
