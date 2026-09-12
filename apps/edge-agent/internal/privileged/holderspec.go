package privileged

import (
	"errors"
	"fmt"
	"os"
	"regexp"
)

/*
The network holder's spec (ADR 0019).

A holder is Guardian's own process, not attacker-facing, and it is the only
container this helper creates with a host-sourced mount: the holder binary,
bind-mounted read-only from the Edge package onto an otherwise empty root. It
has no image, so there is nothing to build, sign, or pull for it; the binary has
the same provenance as the helper that starts it.

It holds CAP_NET_ADMIN, which it needs to configure the namespace it owns, and
nothing else. Every other restriction a decoy has, it has too.
*/

const (
	// DefaultHolderBinary is where the Edge package installs the holder.
	DefaultHolderBinary = "/usr/libexec/guardian-edge/guardian-netholder"
	holderMountPoint    = "/netholder"
	holderUID           = 65534
	holderGID           = 65534
	// holderSuffix contains a character a workload id cannot, so no allowlisted
	// workload can be named after another workload's holder.
	holderSuffix = ".holder"
)

var (
	ErrHolderBinaryUntrusted = errors.New("network holder binary is not root-owned and read-only to others")
	// holderNamespacePattern is the only namespace path a decoy spec may carry:
	// the network namespace of a process the helper has just asked containerd
	// about. It cannot name a file, a bind mount, another namespace type, or
	// pid 1, which on every host is not a holder.
	holderNamespacePattern = regexp.MustCompile(`^/proc/([2-9]|[1-9][0-9]{1,9})/ns/net$`)
)

// HolderID is the containerd id of a workload's holder.
func (w Workload) HolderID() string { return holderID(w.WorkloadID) }

func holderID(workloadID string) string { return workloadID + holderSuffix }

// holderNamespacePath names the network namespace a decoy joins.
func holderNamespacePath(pid uint32) string { return fmt.Sprintf("/proc/%d/ns/net", pid) }

// BuildHolderSpec produces the holder's spec.
func BuildHolderSpec(workload Workload, binary string) (ContainerSpec, error) {
	return buildHolderSpec(workload, binary, hostPlatform())
}

func buildHolderSpec(workload Workload, binary string, platform specPlatform) (ContainerSpec, error) {
	if err := workload.validate(); err != nil {
		return ContainerSpec{}, err
	}
	granted := []string{"CAP_NET_ADMIN"}
	seccomp, err := translateSeccomp(granted, platform)
	if err != nil {
		return ContainerSpec{}, err
	}
	memoryLimit := int64(32 << 20)
	cpuQuota := int64(10_000)
	period := uint64(cpuQuotaPeriod)
	return ContainerSpec{
		Version: ociVersion,
		Process: &specProcess{
			User: specUser{UID: holderUID, GID: holderGID},
			Args: []string{holderMountPoint},
			Env:  []string{},
			Cwd:  "/",
			Capabilities: &specCapabilities{
				Bounding: granted, Effective: granted, Permitted: granted,
				Inheritable: granted, Ambient: granted,
			},
			NoNewPrivileges: true,
		},
		Root:     &specRoot{Path: "rootfs", Readonly: true},
		Hostname: defaultDecoyHostname,
		Mounts: []specMount{
			// runc's init reads /proc/sys/kernel/cap_last_cap inside the
			// container before it drops capabilities; the lab found a holder
			// without /proc could not start.
			{Destination: "/proc", Type: "proc", Source: "proc",
				Options: []string{"nosuid", "noexec", "nodev"}},
			{Destination: "/dev", Type: "tmpfs", Source: "tmpfs",
				Options: []string{"nosuid", "noexec", "strictatime", "mode=755", "size=1024k"}},
			// The one host-sourced mount Guardian writes, and it is Guardian's
			// own binary: read-only, no setuid, no devices.
			{Destination: holderMountPoint, Type: "bind", Source: binary,
				Options: []string{"bind", "ro", "nosuid", "nodev"}},
		},
		Linux: &specLinux{
			Namespaces: []specNamespace{
				{Type: "pid"}, {Type: "ipc"}, {Type: "uts"},
				{Type: "mount"}, {Type: "network"}, {Type: "cgroup"},
			},
			Resources: &specResources{
				Memory:  &specMemory{Limit: &memoryLimit, Swap: &memoryLimit},
				CPU:     &specCPU{Quota: &cpuQuota, Period: &period},
				Pids:    &specPids{Limit: 16},
				Devices: []specDeviceCgroup{{Allow: false, Access: "rwm"}},
			},
			MaskedPaths:   maskedPaths(),
			ReadonlyPaths: readonlyPaths(),
			Seccomp:       seccomp,
			CgroupsPath:   workload.CgroupPath() + holderSuffix,
		},
	}, nil
}

// verifyHolderBinary refuses a holder binary anyone but root could have
// replaced. runc bind-mounts and executes it with CAP_NET_ADMIN inside a
// decoy's namespace, so its integrity is the helper's to check, not
// containerd's.
func verifyHolderBinary(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHolderBinaryUntrusted, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 {
		return ErrHolderBinaryUntrusted
	}
	return verifyRootOwned(info)
}
