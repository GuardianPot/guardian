//go:build linux

package privileged

import (
	"context"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/netholder"
	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/rtnetlink"
	containersapi "github.com/containerd/containerd/api/services/containers/v1"
	tasksapi "github.com/containerd/containerd/api/services/tasks/v1"
	tasktypes "github.com/containerd/containerd/api/types/task"
	"golang.org/x/sys/unix"
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

The strongest evidence here comes from inside the decoy and from the network
around it. A probe script runs as the decoy's own process and exits with a code
naming the first check that failed, so "the decoy cannot see the runtime
socket" is a fact the decoy reported about itself. The lab script builds ADR
0019's topology around the container — an attacker's zone behind `edge0` and a
production host behind `mgmt0` — and the test reaches the decoy from the zone
the way an attacker would.
*/

const (
	containerdLabEnv = "GUARDIAN_CONTAINERD_LAB"
	labWorkloadID    = "guardian-workload-lab"
	// busybox 1.37.0, pinned by its multi-arch index digest, so the lab also
	// exercises choosing this host's manifest out of an index.
	labImageRepository = "docker.io/library/busybox"
	labImageDigest     = "sha256:9db7b59979c38555a39def84a31fb98b5296952f9e3afd4f6f11f05b07adfab0"
	labProbeUID        = 10001

	// The topology lab.sh builds.
	labZoneInterface    = "edge0"
	labContainerdRange  = "172.30.20.96/28"
	labDecoyAddress     = "172.30.20.99"
	labAttackerAddress  = "172.30.20.1"
	labProductionTarget = "10.50.0.20:9"
	labBanner           = "SSH-2.0-guardian-lab"
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
[ "$(hostname)" = "localhost" ] || exit 21
for _ in $(seq 50); do ip -4 addr show dev eth0 2>/dev/null | grep -q 'inet 172.30.20.99/32' && break; sleep 0.1; done
[ "$(ls /sys/class/net | xargs)" = "eth0 lo" ] || exit 20
ip -4 addr show dev eth0 | grep -q 'inet 172.30.20.99/32' || exit 25
ip addr add 172.30.20.98/32 dev eth0 2>/dev/null && exit 26
nc -w 3 172.30.20.1 9 </dev/null && exit 27
exit 0
`

var probeFailures = map[uint32]string{
	10: "the runtime socket is visible", 11: "the runtime directory is visible",
	12: "the decoy is not running as the workload's user", 13: "the root filesystem is writable",
	14: "the effective capabilities are not exactly NET_BIND_SERVICE",
	15: "the bounding set is not exactly NET_BIND_SERVICE", 16: "no_new_privs is not set",
	17: "the writable tmpfs is not writable", 18: "Guardian state is visible",
	19: "Guardian configuration is visible", 20: "the decoy's interfaces are not exactly eth0 and lo",
	21: "the decoy sees a hostname other than its own",
	22: "the telemetry directory is not writable by the decoy",
	23: "a file the decoy wrote to its telemetry directory could be executed",
	24: "no seccomp filter is in force",
	25: "the holder did not give the decoy its address as a /32 on eth0",
	26: "the decoy could change its own network",
	27: "the decoy opened a connection into the zone",
}

func TestContainerdRuntimeAgainstARealContainerd(t *testing.T) {
	if os.Getenv(containerdLabEnv) != "1" {
		t.Skip("the containerd lab runs containers and runs only in its own privileged container")
	}
	holderBinary := os.Getenv("GUARDIAN_HOLDER_BINARY")
	if holderBinary == "" {
		t.Fatal("the lab script did not install the network holder")
	}
	directory := t.TempDir()
	writeLabWorkload(t, directory, 256)

	allowlist, err := CompileAllowlist(AllowlistInput{
		Interfaces:    []string{labZoneInterface},
		Namespaces:    []string{"guardian-decoy-lab"},
		Workloads:     []string{labWorkloadID},
		AddressRanges: []string{labContainerdRange},
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHostAdapter(allowlist).(hostAdapter)
	adapter.runtime.workloadDirectory = directory
	adapter.runtime.holderBinary = holderBinary
	stateDirectory := t.TempDir()
	adapter.runtime.network = &hostNetwork{stateDirectory: stateDirectory}
	program := []string{"/bin/sh", "-c", isolationProbe}
	adapter.runtime.entrypoint = func(ImageEntrypoint) ImageEntrypoint {
		return ImageEntrypoint{Args: program}
	}
	adapter.runtime.startFailed = func(err error) { t.Logf("runtime start error: %v", err) }
	client, err := dialContainerd(containerdSocketPath)
	if err != nil {
		t.Fatal(err)
	}
	routes, err := rtnetlink.DialRoute()
	if err != nil {
		t.Fatal(err)
	}
	decoyPrefix := labDecoyAddress + "/32"
	t.Cleanup(func() {
		_, _ = adapter.ReconcileContainer(context.Background(), ContainerOperation{
			WorkloadID: labWorkloadID, DesiredState: privilegedv1.ContainerState_CONTAINER_STATE_ABSENT,
		})
		_, _ = adapter.EnsureAddress(context.Background(), AddressOperation{
			InterfaceName: labZoneInterface, AddressPrefix: decoyPrefix,
			DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_ABSENT,
		})
		deleteGuardianTable(t)
		_ = routes.Close()
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
	decoyAddress := netip.MustParseAddr(labDecoyAddress)
	zone, err := routes.LinkByName(labZoneInterface)
	if err != nil || zone == nil {
		t.Fatalf("the lab has no %s: %v", labZoneInterface, err)
	}
	forwarding := func() uint32 {
		value, err := routes.IPv4Conf(zone.Index, rtnetlink.IPv4DevconfForwarding)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	// assertDetached is the state removal must leave: no link of Guardian's, and
	// the zone interface forwarding exactly as it did before Guardian.
	assertDetached := func(t *testing.T) {
		t.Helper()
		if link, _ := routes.LinkByName(netholder.HostName(decoyAddress)); link != nil {
			t.Fatalf("the decoy's veth survived: %+v", link)
		}
		if forwarding() != 0 {
			t.Fatal("forwarding on the zone interface was not restored")
		}
		if entries, _ := os.ReadDir(stateDirectory); len(entries) != 0 {
			t.Fatal("a forwarding record outlived the last decoy")
		}
	}

	t.Run("no decoy starts before its egress policy", func(t *testing.T) {
		deleteGuardianTable(t)
		_, err := reconcile(running)
		expectRefusal(t, err, "egress-policy-not-applied")
		if container, _ := client.getContainer(context.Background(), labWorkloadID); container != nil {
			t.Fatal("a container was created before egress was confirmed")
		}
		if forwarding() != 0 {
			t.Fatal("forwarding was enabled before egress was confirmed")
		}
		result, err := adapter.ApplyNftablesPolicy(context.Background(), NftablesOperation{
			NamespaceName: "guardian-decoy-lab",
			Profile:       privilegedv1.NftablesProfile_NFTABLES_PROFILE_DEFAULT_DENY_EGRESS,
		})
		if err != nil || result.Outcome != applied {
			t.Fatalf("egress policy = %+v %v", result, err)
		}
		result, err = adapter.EnsureAddress(context.Background(), AddressOperation{
			InterfaceName: labZoneInterface, AddressPrefix: decoyPrefix,
			DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
		})
		if err != nil || result.ReasonCode != "address-added" {
			t.Fatalf("presence = %+v %v", result, err)
		}
	})

	// Kept open for the probe: a decoy that could connect would be accepted by
	// the kernel's backlog without anyone calling Accept.
	attacker := listenInNamespace(t, "zone", labAttackerAddress+":9")
	t.Cleanup(func() { _ = attacker.Close() })

	/*
	 * AC-SEC-001 and AC-SEC-002 from inside the decoy, and ADR 0019 from both
	 * sides: the decoy reports its own isolation and its own network, and the
	 * host carries exactly the three things the ADR gives it.
	 */
	t.Run("the decoy reports its own isolation and its network", func(t *testing.T) {
		mustReconcile(t, running, applied, "container-started")
		status := waitForExit(t, client, labWorkloadID, 60*time.Second)
		if status != 0 {
			t.Fatalf("isolation probe exited %d: %s", status, probeFailures[status])
		}
		assertCgroupLimits(t, "268435456", "128", "50000 100000")

		host, err := routes.LinkByName(netholder.HostName(decoyAddress))
		if err != nil || host == nil {
			t.Fatalf("the decoy's veth is missing: %v", err)
		}
		if owner, ok := parseHolderLinkAlias(host.Alias); !ok || owner.workloadID != labWorkloadID || owner.zone != labZoneInterface {
			t.Fatalf("veth alias = %q", host.Alias)
		}
		guardianRoutes, err := routes.Routes(rtnetlink.ProtocolGuardian)
		if err != nil || len(guardianRoutes) != 1 ||
			guardianRoutes[0].Destination != netip.PrefixFrom(decoyAddress, 32) || guardianRoutes[0].OutputIndex != host.Index {
			t.Fatalf("guardian routes = %+v %v", guardianRoutes, err)
		}
		if holds, _ := routes.HoldsAddress(decoyAddress); holds {
			t.Fatal("the host holds the decoy's address itself")
		}
		if forwarding() != 1 {
			t.Fatal("forwarding is not enabled on the zone interface")
		}
		if _, err := os.Stat(filepath.Join(stateDirectory, labZoneInterface)); err != nil {
			t.Fatalf("forwarding was enabled without a record that it had been off: %v", err)
		}
		mustReconcile(t, absent, applied, "container-removed")
		assertDetached(t)
	})

	t.Run("an attacker in the zone reaches the decoy and nothing else through the edge", func(t *testing.T) {
		program = []string{"/bin/sh", "-c", "echo " + labBanner + " > /tmp/index.html && exec httpd -f -p 80 -h /tmp"}
		mustReconcile(t, running, applied, "container-started")
		waitForBanner(t)

		production := listenPacketInNamespace(t, "prod", labProductionTarget)
		defer func() { _ = production.Close() }()
		// The control: the Edge itself reaches production, so silence below is
		// the policy and not a broken lab.
		if err := sendDatagram(labProductionTarget, "edge-control"); err != nil {
			t.Fatal(err)
		}
		if got := readDatagram(production); got != "edge-control" {
			t.Fatalf("production did not hear the edge: %q", got)
		}
		if err := inNetworkNamespace("zone", func() error { return sendDatagram(labProductionTarget, "zone-through-edge") }); err != nil {
			t.Fatal(err)
		}
		if got := readDatagram(production); got != "" {
			t.Fatalf("the zone used the edge as a router into production: %q", got)
		}
	})

	/*
	 * Holder and decoy restart together. A killed holder leaves its namespace
	 * alive around the decoy with nothing to configure it; the next pass brings
	 * both back, and the attacker reaches the decoy again.
	 */
	t.Run("a replaced holder takes its decoy with it into a new namespace", func(t *testing.T) {
		holderBefore := taskPIDOf(t, client, holderID(labWorkloadID))
		decoyBefore := taskPIDOf(t, client, labWorkloadID)
		if _, err := client.tasks.Kill(scoped(context.Background()), &tasksapi.KillRequest{
			ContainerID: holderID(labWorkloadID), Signal: uint32(syscall.SIGKILL), All: true,
		}); err != nil {
			t.Fatal(err)
		}
		waitForExit(t, client, holderID(labWorkloadID), 10*time.Second)
		mustReconcile(t, running, applied, "holder-replaced")
		if after := taskPIDOf(t, client, holderID(labWorkloadID)); after == holderBefore {
			t.Fatal("the holder was not replaced")
		}
		if after := taskPIDOf(t, client, labWorkloadID); after == decoyBefore {
			t.Fatal("the decoy stayed in its dead holder's namespace")
		}
		waitForBanner(t)
		mustReconcile(t, absent, applied, "container-removed")
		assertDetached(t)
	})

	t.Run("a decoy runs, converges, and is restarted when it dies", func(t *testing.T) {
		program = []string{"/bin/sleep", "3600"}
		mustReconcile(t, running, applied, "container-started")
		mustReconcile(t, running, unchanged, "container-already-running")
		before := taskPIDOf(t, client, labWorkloadID)

		// A crash, from the reconciler's point of view: the process is gone.
		if _, err := client.tasks.Kill(scoped(context.Background()), &tasksapi.KillRequest{
			ContainerID: labWorkloadID, Signal: uint32(syscall.SIGKILL), All: true,
		}); err != nil {
			t.Fatal(err)
		}
		waitForExit(t, client, labWorkloadID, 10*time.Second)
		mustReconcile(t, running, applied, "container-restarted")
		if after := taskPIDOf(t, client, labWorkloadID); after == 0 || after == before {
			t.Fatalf("pid before %d, after %d: the decoy was not restarted", before, after)
		}
	})

	t.Run("stopping keeps the containers and ends the processes and the host side", func(t *testing.T) {
		mustReconcile(t, stopped, applied, "container-stopped")
		mustReconcile(t, stopped, unchanged, "container-already-stopped")
		for _, id := range []string{labWorkloadID, holderID(labWorkloadID)} {
			if process, _ := client.getTask(context.Background(), id); process != nil {
				t.Fatalf("stopped %s still has a task: %v", id, process.GetStatus())
			}
			if container, _ := client.getContainer(context.Background(), id); container == nil {
				t.Fatalf("stopping removed %s", id)
			}
		}
		assertDetached(t)
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
		for _, id := range []string{labWorkloadID, holderID(labWorkloadID)} {
			if _, err := client.containers.Get(otherNamespace, &containersapi.GetContainerRequest{ID: id}); !isNotFound(err) {
				t.Fatalf("%s was visible from the default namespace: %v", id, err)
			}
		}
		mustReconcile(t, absent, applied, "container-removed")
		mustReconcile(t, absent, unchanged, "container-already-absent")
		if container, _ := client.getContainer(context.Background(), holderID(labWorkloadID)); container != nil {
			t.Fatal("removal left the holder")
		}
		assertDetached(t)
	})
}

func writeLabWorkload(t *testing.T, directory string, memoryMiB int) {
	t.Helper()
	raw := workloadJSON(map[string]string{
		"workload_id": `"` + labWorkloadID + `"`,
		"image":       `{"repository": "` + labImageRepository + `", "digest": "` + labImageDigest + `"}`,
		"ports":       `[{"port": 80, "protocol": "tcp"}]`,
		"resources":   `{"cpu_millicores": 500, "memory_mib": ` + itoa(memoryMiB) + `, "pids": 128}`,
		"user":        `{"uid": 10001, "gid": 10001}`,
		"network":     `{"interface": "` + labZoneInterface + `", "address": "` + labDecoyAddress + `"}`,
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

func waitForExit(t *testing.T, client *containerdClient, id string, bound time.Duration) uint32 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), bound)
	defer cancel()
	response, err := client.tasks.Wait(scoped(ctx), &tasksapi.WaitRequest{ContainerID: id})
	if err != nil {
		t.Fatalf("%s did not exit: %v", id, err)
	}
	return response.GetExitStatus()
}

func taskPIDOf(t *testing.T, client *containerdClient, id string) uint32 {
	t.Helper()
	process, err := client.getTask(context.Background(), id)
	if err != nil || process == nil || process.GetStatus() != tasktypes.Status_RUNNING {
		t.Fatalf("%s has no running task: %v %v", id, process, err)
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

/*
inNetworkNamespace runs fn with its sockets created in one of the lab's named
namespaces. A socket belongs to the namespace it was created in, so what fn
opens stays there after it returns.

The thread is never unlocked: it ends in another namespace, and a locked thread
whose goroutine exits is discarded by the runtime rather than reused.
*/
func inNetworkNamespace(name string, fn func() error) error {
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		handle, err := os.Open("/run/netns/" + name)
		if err != nil {
			done <- err
			return
		}
		defer func() { _ = handle.Close() }()
		if err := unix.Setns(int(handle.Fd()), unix.CLONE_NEWNET); err != nil {
			done <- err
			return
		}
		done <- fn()
	}()
	return <-done
}

func listenInNamespace(t *testing.T, namespace, address string) net.Listener {
	t.Helper()
	var listener net.Listener
	if err := inNetworkNamespace(namespace, func() (err error) {
		listener, err = net.Listen("tcp4", address)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return listener
}

func listenPacketInNamespace(t *testing.T, namespace, address string) net.PacketConn {
	t.Helper()
	var socket net.PacketConn
	if err := inNetworkNamespace(namespace, func() (err error) {
		socket, err = net.ListenPacket("udp4", address)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return socket
}

func sendDatagram(address, payload string) error {
	connection, err := net.Dial("udp4", address)
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }()
	_, err = connection.Write([]byte(payload))
	return err
}

// readDatagram waits three seconds, long enough for a first datagram to wait
// out neighbour resolution, and returns "" if nothing came.
func readDatagram(socket net.PacketConn) string {
	buffer := make([]byte, 256)
	_ = socket.SetReadDeadline(time.Now().Add(3 * time.Second))
	read, _, err := socket.ReadFrom(buffer)
	if err != nil {
		return ""
	}
	return string(buffer[:read])
}

// waitForBanner polls the decoy from the attacker's zone until it answers.
func waitForBanner(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var last string
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = inNetworkNamespace("zone", func() error {
			connection, err := net.DialTimeout("tcp4", labDecoyAddress+":80", 2*time.Second)
			if err != nil {
				return err
			}
			defer func() { _ = connection.Close() }()
			_ = connection.SetDeadline(time.Now().Add(3 * time.Second))
			if _, err := io.WriteString(connection, "GET / HTTP/1.0\r\n\r\n"); err != nil {
				return err
			}
			raw, err := io.ReadAll(connection)
			last = string(raw)
			return err
		})
		if lastErr == nil && strings.Contains(last, labBanner) {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("the attacker never reached the decoy: %q %v", last, lastErr)
}
