//go:build linux

package privileged

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	containersapi "github.com/containerd/containerd/api/services/containers/v1"
	tasksapi "github.com/containerd/containerd/api/services/tasks/v1"
	tasktypes "github.com/containerd/containerd/api/types/task"
	"google.golang.org/grpc/metadata"
)

/*
The containerd lab.

A real containerd 2.x, a real runc, and a real image pulled from a registry by
digest, inside a throwaway privileged container that `task privileged:containerd`
builds. Privileged because containerd itself needs to mount, create cgroups,
and create namespaces; the privilege belongs to the lab's runtime, not to the
code under test, which reaches containerd only through its socket as the helper
does.

The strongest evidence here comes from inside the decoy. A probe script runs as
the decoy's own process and exits with a code naming the first check that
failed, so "the decoy cannot see the runtime socket" is a fact the decoy
reported about itself rather than a property of a spec nobody ran.
*/

const (
	containerdLabEnv = "GUARDIAN_CONTAINERD_LAB"
	labWorkloadID    = "guardian-workload-lab"
	// busybox 1.37.0, pinned by its multi-arch index digest, so the lab also
	// exercises choosing this host's manifest out of an index.
	labImageRepository = "docker.io/library/busybox"
	labImageDigest     = "sha256:9db7b59979c38555a39def84a31fb98b5296952f9e3afd4f6f11f05b07adfab0"
	labProbeUID        = 10001
)

// isolationProbe exits 0 only if every property holds, and otherwise with the
// code of the first one that does not.
const isolationProbe = `
test -e /run/containerd/containerd.sock && exit 10
test -e /run/containerd && exit 11
[ "$(id -u):$(id -g)" = "10001:10001" ] || exit 12
touch /written-to-root 2>/dev/null && exit 13
grep -Eq '^CapEff:[[:space:]]+0000000000000400$' /proc/self/status || exit 14
grep -Eq '^CapBnd:[[:space:]]+0000000000000400$' /proc/self/status || exit 15
grep -Eq '^NoNewPrivs:[[:space:]]+1$' /proc/self/status || exit 16
grep -Eq '^Seccomp:[[:space:]]+2$' /proc/self/status || exit 24
echo ok > /tmp/probe || exit 17
echo event > /var/log/guardian/probe.log || exit 22
cp /bin/busybox /var/log/guardian/run 2>/dev/null && /var/log/guardian/run true 2>/dev/null && exit 23
test -e /var/lib/guardian && exit 18
test -e /etc/guardian-edge && exit 19
[ "$(ls /sys/class/net)" = "lo" ] || exit 20
[ "$(hostname)" = "localhost" ] || exit 21
exit 0
`

var probeFailures = map[uint32]string{
	10: "the runtime socket is visible", 11: "the runtime directory is visible",
	12: "the decoy is not running as the workload's user", 13: "the root filesystem is writable",
	14: "the effective capabilities are not exactly NET_BIND_SERVICE",
	15: "the bounding set is not exactly NET_BIND_SERVICE", 16: "no_new_privs is not set",
	17: "the writable tmpfs is not writable", 18: "Guardian state is visible",
	19: "Guardian configuration is visible", 20: "the decoy has a network interface besides lo",
	21: "the decoy sees a hostname other than its own",
	22: "the telemetry directory is not writable by the decoy",
	23: "a file the decoy wrote to its telemetry directory could be executed",
	24: "no seccomp filter is in force",
}

func TestContainerdRuntimeAgainstARealContainerd(t *testing.T) {
	if os.Getenv(containerdLabEnv) != "1" {
		t.Skip("the containerd lab runs containers and runs only in its own privileged container")
	}
	directory := t.TempDir()
	writeLabWorkload(t, directory, 256)

	allowlist, err := CompileAllowlist(AllowlistInput{
		Namespaces:    []string{"guardian-decoy-lab"},
		Workloads:     []string{labWorkloadID},
		AddressRanges: []string{labDecoyRange},
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHostAdapter(allowlist).(hostAdapter)
	adapter.runtime.workloadDirectory = directory
	program := []string{"/bin/sh", "-c", isolationProbe}
	adapter.runtime.entrypoint = func(ImageEntrypoint) ImageEntrypoint {
		return ImageEntrypoint{Args: program}
	}
	adapter.runtime.startFailed = func(err error) { t.Logf("runtime start error: %v", err) }
	client, err := dialContainerd(containerdSocketPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = adapter.ReconcileContainer(context.Background(), ContainerOperation{
			WorkloadID: labWorkloadID, DesiredState: privilegedv1.ContainerState_CONTAINER_STATE_ABSENT,
		})
		deleteGuardianTable(t)
		_ = client.Close()
	})
	reconcile := func(state privilegedv1.ContainerState) (AdapterResult, error) {
		return adapter.ReconcileContainer(context.Background(), ContainerOperation{WorkloadID: labWorkloadID, DesiredState: state})
	}
	mustReconcile := func(t *testing.T, state privilegedv1.ContainerState, outcome privilegedv1.OperationOutcome, reason string) {
		t.Helper()
		result, err := reconcile(state)
		expectResult(t, result, err, outcome, reason)
	}
	const (
		applied   = privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED
		unchanged = privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED
	)

	t.Run("no decoy starts before its egress policy", func(t *testing.T) {
		deleteGuardianTable(t)
		_, err := reconcile(running)
		expectRefusal(t, err, "egress-policy-not-applied")
		if container, _ := client.getContainer(context.Background(), labWorkloadID); container != nil {
			t.Fatal("a container was created before egress was confirmed")
		}
		result, err := adapter.ApplyNftablesPolicy(context.Background(), NftablesOperation{
			NamespaceName: "guardian-decoy-lab",
			Profile:       privilegedv1.NftablesProfile_NFTABLES_PROFILE_DEFAULT_DENY_EGRESS,
		})
		if err != nil || result.Outcome != applied {
			t.Fatalf("egress policy = %+v %v", result, err)
		}
	})

	/*
	 * AC-SEC-001 and AC-SEC-002 from inside the decoy: the image is pulled by
	 * digest, the container is built from Guardian's spec, and the decoy's own
	 * process reports what it can and cannot see.
	 */
	t.Run("the decoy reports its own isolation", func(t *testing.T) {
		mustReconcile(t, running, applied, "container-started")
		status := waitForExit(t, client, 30*time.Second)
		if status != 0 {
			t.Fatalf("isolation probe exited %d: %s", status, probeFailures[status])
		}
		assertCgroupLimits(t, "268435456", "128", "50000 100000")
		mustReconcile(t, absent, applied, "container-removed")
	})

	t.Run("a decoy runs, converges, and is restarted when it dies", func(t *testing.T) {
		program = []string{"/bin/sleep", "3600"}
		mustReconcile(t, running, applied, "container-started")
		mustReconcile(t, running, unchanged, "container-already-running")
		before := taskPID(t, client)

		// A crash, from the reconciler's point of view: the process is gone.
		if _, err := client.tasks.Kill(scoped(context.Background()), &tasksapi.KillRequest{
			ContainerID: labWorkloadID, Signal: uint32(syscall.SIGKILL), All: true,
		}); err != nil {
			t.Fatal(err)
		}
		waitForExit(t, client, 10*time.Second)
		mustReconcile(t, running, applied, "container-restarted")
		if after := taskPID(t, client); after == 0 || after == before {
			t.Fatalf("pid before %d, after %d: the decoy was not restarted", before, after)
		}
	})

	t.Run("stopping keeps the container and ends the process", func(t *testing.T) {
		mustReconcile(t, stopped, applied, "container-stopped")
		mustReconcile(t, stopped, unchanged, "container-already-stopped")
		if process, _ := client.getTask(context.Background(), labWorkloadID); process != nil {
			t.Fatalf("a stopped decoy still has a task: %v", process.GetStatus())
		}
		if container, _ := client.getContainer(context.Background(), labWorkloadID); container == nil {
			t.Fatal("stopping removed the container")
		}
	})

	t.Run("an edited definition replaces the container", func(t *testing.T) {
		writeLabWorkload(t, directory, 128)
		mustReconcile(t, running, applied, "container-recreated")
		assertCgroupLimits(t, "134217728", "128", "50000 100000")
	})

	t.Run("a start that fails leaves nothing behind", func(t *testing.T) {
		program = []string{"/does-not-exist"}
		_, err := reconcile(running)
		expectRefusal(t, err, "container-start-failed")
		if container, _ := client.getContainer(context.Background(), labWorkloadID); container != nil {
			t.Fatal("the failed start left a container")
		}
		if process, _ := client.getTask(context.Background(), labWorkloadID); process != nil {
			t.Fatal("the failed start left a task")
		}
	})

	/*
	 * The helper's containers live in a containerd namespace nothing else
	 * uses. Looked up from the default namespace, a Guardian decoy does not
	 * exist, which is the other side of the helper being unable to touch
	 * anyone else's containers.
	 */
	t.Run("the decoy is invisible outside Guardian's namespace", func(t *testing.T) {
		program = []string{"/bin/sleep", "3600"}
		mustReconcile(t, running, applied, "container-started")
		otherNamespace := metadata.AppendToOutgoingContext(context.Background(), "containerd-namespace", "default")
		if _, err := client.containers.Get(otherNamespace, &containersapi.GetContainerRequest{ID: labWorkloadID}); !isNotFound(err) {
			t.Fatalf("the decoy was visible from the default namespace: %v", err)
		}
		mustReconcile(t, absent, applied, "container-removed")
		mustReconcile(t, absent, unchanged, "container-already-absent")
	})
}

func writeLabWorkload(t *testing.T, directory string, memoryMiB int) {
	t.Helper()
	raw := workloadJSON(map[string]string{
		"workload_id": `"` + labWorkloadID + `"`,
		"image":       `{"repository": "` + labImageRepository + `", "digest": "` + labImageDigest + `"}`,
		"resources":   `{"cpu_millicores": 500, "memory_mib": ` + itoa(memoryMiB) + `, "pids": 128}`,
		"user":        `{"uid": 10001, "gid": 10001}`,
	})
	if err := os.WriteFile(filepath.Join(directory, labWorkloadID+".json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
}

func itoa(value int) string {
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

func waitForExit(t *testing.T, client *containerdClient, bound time.Duration) uint32 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), bound)
	defer cancel()
	response, err := client.tasks.Wait(scoped(ctx), &tasksapi.WaitRequest{ContainerID: labWorkloadID})
	if err != nil {
		t.Fatalf("the decoy did not exit: %v", err)
	}
	return response.GetExitStatus()
}

func taskPID(t *testing.T, client *containerdClient) uint32 {
	t.Helper()
	process, err := client.getTask(context.Background(), labWorkloadID)
	if err != nil || process == nil || process.GetStatus() != tasktypes.Status_RUNNING {
		t.Fatalf("no running task: %v %v", process, err)
	}
	return process.GetPid()
}

// assertCgroupLimits reads the limits from the host side of the cgroup, which
// is what the kernel enforces, rather than trusting the spec that asked for
// them.
func assertCgroupLimits(t *testing.T, memory, pids, cpu string) {
	t.Helper()
	base := filepath.Join("/sys/fs/cgroup", "guardian-decoy", labWorkloadID)
	for file, want := range map[string]string{"memory.max": memory, "pids.max": pids, "cpu.max": cpu, "memory.swap.max": "0"} {
		raw, err := os.ReadFile(filepath.Join(base, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if got := strings.TrimSpace(string(raw)); got != want {
			t.Fatalf("%s = %q, want %q", file, got, want)
		}
	}
}
