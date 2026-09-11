//go:build linux

package privileged

import (
	"context"
	"net"
	"net/netip"
	"os"
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
Guardian refuses to remove an address it did not add.
*/

const (
	netlinkLabEnv       = "GUARDIAN_NETLINK_LAB"
	labInterface        = "eth0"
	labDecoyPrefix      = "10.99.7.40/32"
	labForeignPrefix    = "10.99.7.41/32"
	labForeignLabelName = labInterface + ":host"
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
	before := addressStrings(t, connection, link.Index)
	t.Cleanup(func() {
		_ = connection.DeleteAddress(link.Index, decoy)
		_ = connection.DeleteAddress(link.Index, foreign)
	})

	t.Run("an address is added, observed, and idempotent", func(t *testing.T) {
		result := ensure(t, adapter, privilegedv1.PresenceState_PRESENCE_STATE_PRESENT, labDecoyPrefix)
		if result.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED ||
			result.ReasonCode != "address-added" {
			t.Fatalf("first apply = %+v", result)
		}
		observed, err := connection.FindAddress(link.Index, decoy)
		if err != nil || observed == nil {
			t.Fatalf("the kernel does not report the address it accepted: %v", err)
		}
		// The label is what makes the address provably Guardian's later.
		if observed.Label != labInterface+guardianLabelSuffix {
			t.Fatalf("label = %q, want %q", observed.Label, labInterface+guardianLabelSuffix)
		}
		// And read back through the standard library, which has its own netlink
		// implementation. Agreement between this adapter's decoder and an
		// unrelated one is what makes "the address is on the interface" a fact
		// about the kernel rather than about this file.
		if !stdlibReports(t, labInterface, labDecoyPrefix) {
			t.Fatal("the standard library does not see the address this adapter added")
		}

		// A reconciler converges repeatedly. The second pass must report no
		// change, or every pass would look like a fresh deployment.
		again := ensure(t, adapter, privilegedv1.PresenceState_PRESENCE_STATE_PRESENT, labDecoyPrefix)
		if again.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED ||
			again.ReasonCode != "address-already-present" {
			t.Fatalf("second apply = %+v", again)
		}
	})

	t.Run("an address is removed, and removing it twice is not a failure", func(t *testing.T) {
		result := ensure(t, adapter, privilegedv1.PresenceState_PRESENCE_STATE_ABSENT, labDecoyPrefix)
		if result.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED ||
			result.ReasonCode != "address-removed" {
			t.Fatalf("first release = %+v", result)
		}
		observed, err := connection.FindAddress(link.Index, decoy)
		if err != nil {
			t.Fatal(err)
		}
		if observed != nil {
			t.Fatal("the address is still on the interface after a reported removal")
		}
		if stdlibReports(t, labInterface, labDecoyPrefix) {
			t.Fatal("the standard library still sees an address this adapter reported removing")
		}
		again := ensure(t, adapter, privilegedv1.PresenceState_PRESENCE_STATE_ABSENT, labDecoyPrefix)
		if again.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED ||
			again.ReasonCode != "address-already-absent" {
			t.Fatalf("second release = %+v", again)
		}
	})

	/*
	 * The refusal this whole file exists for.
	 *
	 * An address on the interface that Guardian did not label belongs to the
	 * host. Removing one would take a production address off the wire, which is
	 * the outage a deception product must never cause; adopting one would mean
	 * Guardian could remove it later. Both directions are refused, and the
	 * address is still there afterwards.
	 */
	t.Run("an address the host owns is never taken or removed", func(t *testing.T) {
		if err := connection.AddAddress(link.Index, foreign, labForeignLabelName); err != nil {
			t.Fatalf("could not stage a host-owned address: %v", err)
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
		observed, err := connection.FindAddress(link.Index, foreign)
		if err != nil {
			t.Fatal(err)
		}
		if observed == nil || observed.Label != labForeignLabelName {
			t.Fatalf("the host's address did not survive the refusal: %+v", observed)
		}
		if err := connection.DeleteAddress(link.Index, foreign); err != nil {
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

// stdlibReports answers the same question through net.Interface.Addrs, which
// reaches the kernel by a netlink path this package did not write.
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
