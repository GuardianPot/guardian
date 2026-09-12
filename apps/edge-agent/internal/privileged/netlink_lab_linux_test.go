//go:build linux

package privileged

import (
	"context"
	"net"
	"net/netip"
	"os"
	"strings"
	"testing"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/rtnetlink"
)

/*
The netlink lab.

Everything here mutates a real interface, so it runs only inside the throwaway
container `task privileged:netlink` builds: its own network namespace, its own
`eth0`, CAP_NET_ADMIN and nothing else. Nothing outside that namespace can be
reached, which is what makes it safe to assert against a live kernel rather
than a fake.

The two properties worth the container are the ones a unit test cannot show:
that the messages this adapter encodes are accepted by a real kernel, and that
Guardian refuses to take over or remove what it did not add.
*/

const (
	netlinkLabEnv    = "GUARDIAN_NETLINK_LAB"
	labInterface     = "eth0"
	labDecoyPrefix   = "10.99.7.40/32"
	labForeignPrefix = "10.99.7.41/32"
	labHeldPrefix    = "10.99.7.42/32"
	// staticProtocol is iproute2's `static`, what an operator's `ip neigh add`
	// would carry.
	staticProtocol = 4
)

func TestNetlinkAddressAdapterAgainstALiveKernel(t *testing.T) {
	if os.Getenv(netlinkLabEnv) != "1" {
		t.Skip("the netlink lab mutates a live interface and runs only in its own container")
	}
	// Not a skip: the task grants the capability, so its absence means the lab
	// is misconfigured and the evidence would silently not exist.
	if !holdsCapNetAdmin() {
		t.Fatal("the netlink lab requires CAP_NET_ADMIN")
	}
	link, err := net.InterfaceByName(labInterface)
	if err != nil {
		t.Fatalf("the lab container has no %s: %v", labInterface, err)
	}
	adapter := NewHostAdapter(Allowlist{}).(hostAdapter)
	if adapter.address.State != privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE {
		t.Fatalf("address capability = %+v inside the lab", adapter.address)
	}

	connection, err := rtnetlink.DialRoute()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	decoy := netip.MustParsePrefix(labDecoyPrefix)
	foreign := netip.MustParsePrefix(labForeignPrefix)
	held := netip.MustParsePrefix(labHeldPrefix)
	before := addressStrings(t, connection, link.Index)
	t.Cleanup(func() {
		for _, prefix := range []netip.Prefix{decoy, foreign} {
			_ = connection.DeleteProxyNeighbour(link.Index, prefix.Addr())
		}
		_ = connection.DeleteAddress(link.Index, held)
	})

	t.Run("a proxy entry is added, observed, and idempotent", func(t *testing.T) {
		result := ensure(t, adapter, privilegedv1.PresenceState_PRESENCE_STATE_PRESENT, labDecoyPrefix)
		if result.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED ||
			result.ReasonCode != "address-added" {
			t.Fatalf("first apply = %+v", result)
		}
		entry, err := proxyEntry(connection, link.Index, decoy.Addr())
		if err != nil || entry == nil {
			t.Fatalf("the kernel does not report the entry it accepted: %v", err)
		}
		// The protocol is what makes the entry provably Guardian's later.
		if entry.Protocol != rtnetlink.ProtocolGuardian {
			t.Fatalf("protocol = %d, want %d", entry.Protocol, rtnetlink.ProtocolGuardian)
		}
		// ADR 0019: the address lives in the decoy's namespace, never on the host.
		if holds, err := connection.HoldsAddress(decoy.Addr()); err != nil || holds {
			t.Fatalf("the host holds the decoy address itself: %v %v", holds, err)
		}
		if stdlibReports(t, labInterface, labDecoyPrefix) {
			t.Fatal("the standard library sees the decoy address on the host's interface")
		}
		if delay := readProcSys(t, "net/ipv4/neigh/"+labInterface+"/proxy_delay"); delay != "0" {
			t.Fatalf("proxy_delay = %q, want 0: a delayed answer is a tell", delay)
		}

		// A reconciler converges repeatedly. The second pass must report no
		// change, or every pass would look like a fresh deployment.
		again := ensure(t, adapter, privilegedv1.PresenceState_PRESENCE_STATE_PRESENT, labDecoyPrefix)
		if again.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED ||
			again.ReasonCode != "address-already-present" {
			t.Fatalf("second apply = %+v", again)
		}
	})

	t.Run("a proxy entry is removed, and removing it twice is not a failure", func(t *testing.T) {
		result := ensure(t, adapter, privilegedv1.PresenceState_PRESENCE_STATE_ABSENT, labDecoyPrefix)
		if result.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED ||
			result.ReasonCode != "address-removed" {
			t.Fatalf("first release = %+v", result)
		}
		if entry, err := proxyEntry(connection, link.Index, decoy.Addr()); err != nil || entry != nil {
			t.Fatalf("the entry survived a reported removal: %+v %v", entry, err)
		}
		again := ensure(t, adapter, privilegedv1.PresenceState_PRESENCE_STATE_ABSENT, labDecoyPrefix)
		if again.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED ||
			again.ReasonCode != "address-already-absent" {
			t.Fatalf("second release = %+v", again)
		}
	})

	/*
	 * The refusals this whole file exists for.
	 *
	 * A proxy entry without Guardian's protocol, or an address a host interface
	 * holds as its own, belongs to the host. Removing one would take a
	 * production answer off the wire, which is the outage a deception product
	 * must never cause; adopting one would mean Guardian could remove it later.
	 */
	t.Run("a proxy entry the host made is never taken or removed", func(t *testing.T) {
		if err := connection.AddProxyNeighbour(link.Index, foreign.Addr(), staticProtocol); err != nil {
			t.Fatalf("could not stage a host-owned entry: %v", err)
		}
		for _, state := range []privilegedv1.PresenceState{
			privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
			privilegedv1.PresenceState_PRESENCE_STATE_ABSENT,
		} {
			_, err := adapter.EnsureAddress(context.Background(),
				addressRequest(labInterface, labForeignPrefix, state))
			refusal, ok := asViolation(err)
			if !ok || refusal.ReasonCode != "address-held-by-host" {
				t.Fatalf("state %v = %v, want a host-owned refusal", state, err)
			}
		}
		entry, err := proxyEntry(connection, link.Index, foreign.Addr())
		if err != nil || entry == nil || entry.Protocol != staticProtocol {
			t.Fatalf("the host's entry did not survive the refusal: %+v %v", entry, err)
		}
		if err := connection.DeleteProxyNeighbour(link.Index, foreign.Addr()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("an address the host holds is never proxied", func(t *testing.T) {
		if err := connection.AddAddress(link.Index, held, labInterface+":host"); err != nil {
			t.Fatalf("could not stage a host-held address: %v", err)
		}
		_, err := adapter.EnsureAddress(context.Background(),
			addressRequest(labInterface, labHeldPrefix, privilegedv1.PresenceState_PRESENCE_STATE_PRESENT))
		if refusal, ok := asViolation(err); !ok || refusal.ReasonCode != "address-held-by-host" {
			t.Fatalf("err = %v, want a host-owned refusal", err)
		}
		if entry, _ := proxyEntry(connection, link.Index, held.Addr()); entry != nil {
			t.Fatal("a proxy entry was added for an address the host holds")
		}
		if err := connection.DeleteAddress(link.Index, held); err != nil {
			t.Fatal(err)
		}
	})

	// Nothing the interface started with was disturbed by any of it.
	after := addressStrings(t, connection, link.Index)
	for address := range before {
		if _, kept := after[address]; !kept {
			t.Fatalf("%s was on %s before the lab ran and is gone", address, labInterface)
		}
	}
	for address := range after {
		if _, expected := before[address]; !expected {
			t.Fatalf("%s was left behind on %s", address, labInterface)
		}
	}
	if entries, err := connection.ProxyNeighbours(link.Index); err != nil || len(entries) != 0 {
		t.Fatalf("proxy entries left behind: %+v %v", entries, err)
	}
}

func ensure(t *testing.T, adapter hostAdapter, state privilegedv1.PresenceState, prefix string) AdapterResult {
	t.Helper()
	result, err := adapter.EnsureAddress(context.Background(), addressRequest(labInterface, prefix, state))
	if err != nil {
		t.Fatalf("EnsureAddress(%v, %s): %v", state, prefix, err)
	}
	if !reasonCodePattern.MatchString(result.ReasonCode) {
		t.Fatalf("reason %q is not a closed token", result.ReasonCode)
	}
	return result
}

// stdlibReports answers "does the host hold this address" through
// net.Interface.Addrs, which reaches the kernel by a netlink path this package
// did not write.
func stdlibReports(t *testing.T, interfaceName, prefix string) bool {
	t.Helper()
	link, err := net.InterfaceByName(interfaceName)
	if err != nil {
		t.Fatal(err)
	}
	addresses, err := link.Addrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, address := range addresses {
		if address.String() == prefix {
			return true
		}
	}
	return false
}

func addressStrings(t *testing.T, connection *rtnetlink.Conn, index int) map[string]struct{} {
	t.Helper()
	addresses, err := connection.Addresses(index)
	if err != nil {
		t.Fatal(err)
	}
	set := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		set[address.Prefix.String()] = struct{}{}
	}
	return set
}

func readProcSys(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile("/proc/sys/" + path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}
