//go:build linux

package privileged

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/rtnetlink"
	"golang.org/x/sys/unix"
)

/*
The nftables lab.

`AC-SEC-003` is not a claim about a ruleset, it is a claim about a packet. A
test that only checked which rules exist would pass just as happily on a
ruleset that matches nothing — which is precisely how a hand-encoded netlink
policy fails, silently and while reporting success.

So this sends packets. The experiment is controlled: the same destination, the
same route, the same socket type, and only the source address differs. One
source is inside the decoy range the policy was built from and one is not.

It runs in the same throwaway container as the address lab, with CAP_NET_ADMIN
and its own network namespace. The destination is an address in the decoy's own
subnet that nothing answers on, so nothing leaves the container's bridge.
*/

const (
	labDecoyRange = "10.99.7.0/24"
	// Staged directly rather than through EnsureAddress, and zone-length on
	// purpose: the on-link route it brings is what lets the experiment send
	// without anything leaving the container.
	labEgressSourcePrefix = "10.99.7.40/24"
	labEgressTarget       = "10.99.7.99"
	labEgressTargetPort   = 9
)

func TestNftablesEgressPolicyAgainstALiveKernel(t *testing.T) {
	if os.Getenv(netlinkLabEnv) != "1" {
		t.Skip("the nftables lab changes a live ruleset and runs only in its own container")
	}
	if !holdsCapNetAdmin() {
		t.Fatal("the nftables lab requires CAP_NET_ADMIN")
	}
	allowlist, err := CompileAllowlist(AllowlistInput{
		Interfaces:    []string{labInterface},
		Namespaces:    []string{"guardian-decoy-lab"},
		AddressRanges: []string{labDecoyRange},
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewHostAdapter(allowlist).(hostAdapter)
	if adapter.nftables.State != privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE {
		t.Fatalf("nftables capability = %+v inside the lab", adapter.nftables)
	}

	// A decoy address to send from, and the on-link route that comes with it.
	routes, err := rtnetlink.DialRoute()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = routes.Close() })
	link, err := net.InterfaceByName(labInterface)
	if err != nil {
		t.Fatal(err)
	}
	decoy := netip.MustParsePrefix(labEgressSourcePrefix)
	if err := routes.AddAddress(link.Index, decoy, labInterface+guardianLabelSuffix); err != nil {
		t.Fatalf("could not stage a decoy address: %v", err)
	}
	t.Cleanup(func() { _ = routes.DeleteAddress(link.Index, decoy) })
	hostSource := primaryAddress(t, link, decoy.Addr())
	t.Cleanup(func() { deleteGuardianTable(t) })

	// Before any policy, both sources reach the destination the same way.
	if err := sendFrom(decoy.Addr()); err != nil {
		t.Fatalf("a decoy source could not send before the policy existed: %v", err)
	}
	if err := sendFrom(hostSource); err != nil {
		t.Fatalf("the host's own source could not send before the policy existed: %v", err)
	}

	operation := NftablesOperation{
		NamespaceName: "guardian-decoy-lab",
		Profile:       privilegedv1.NftablesProfile_NFTABLES_PROFILE_DEFAULT_DENY_EGRESS,
	}

	t.Run("the policy is applied and denies a decoy its outbound connection", func(t *testing.T) {
		result, err := adapter.ApplyNftablesPolicy(context.Background(), operation)
		if err != nil {
			t.Fatal(err)
		}
		if result.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED ||
			result.ReasonCode != "egress-policy-applied" {
			t.Fatalf("apply = %+v", result)
		}

		// AC-SEC-003, as a packet rather than as a ruleset.
		if err := sendFrom(decoy.Addr()); !errors.Is(err, unix.EPERM) {
			t.Fatalf("a decoy sent outbound after the policy was applied: %v", err)
		}
		// And the policy is scoped: the host itself is unaffected. A control
		// that also broke the host would be withdrawn the first time it ran.
		if err := sendFrom(hostSource); err != nil {
			t.Fatalf("the default-deny policy blocked the host's own traffic: %v", err)
		}
	})

	t.Run("applying it again reports no change", func(t *testing.T) {
		result, err := adapter.ApplyNftablesPolicy(context.Background(), operation)
		if err != nil {
			t.Fatal(err)
		}
		if result.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED ||
			result.ReasonCode != "egress-policy-already-applied" {
			t.Fatalf("second apply = %+v", result)
		}
	})

	/*
	 * A containment control that cannot notice its own removal is not a
	 * control. Somebody flushing the table has to read as unconverged on the
	 * next pass, not as "already applied".
	 */
	t.Run("a removed policy is noticed and restored", func(t *testing.T) {
		deleteGuardianTable(t)
		if err := sendFrom(decoy.Addr()); err != nil {
			t.Fatalf("egress was still denied after the table was deleted: %v", err)
		}
		result, err := adapter.ApplyNftablesPolicy(context.Background(), operation)
		if err != nil {
			t.Fatal(err)
		}
		if result.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED {
			t.Fatalf("reapply after removal = %+v", result)
		}
		if err := sendFrom(decoy.Addr()); !errors.Is(err, unix.EPERM) {
			t.Fatalf("the restored policy does not deny egress: %v", err)
		}
	})

	// A policy built from different ranges must not be mistaken for this one.
	t.Run("a different range set is not the same policy", func(t *testing.T) {
		connection, err := rtnetlink.Dial(unix.NETLINK_NETFILTER)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = connection.Close() }()
		observed, err := observedEgressPolicy(connection)
		if err != nil {
			t.Fatal(err)
		}
		other := buildEgressPolicy([]netip.Prefix{netip.MustParsePrefix("10.88.0.0/16")})
		if other.matches(observed) {
			t.Fatal("a policy for other ranges matched the installed one")
		}
		if !buildEgressPolicy([]netip.Prefix{decoy.Masked()}).matches(observed) {
			t.Fatal("the installed policy did not match the one that was applied")
		}
	})
}

// sendFrom writes one datagram from an exact source address. Nothing answers at
// the destination; what is under test is whether the kernel accepts the send.
func sendFrom(source netip.Addr) error {
	connection, err := net.DialUDP("udp4",
		&net.UDPAddr{IP: source.AsSlice()},
		&net.UDPAddr{IP: net.ParseIP(labEgressTarget), Port: labEgressTargetPort})
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }()
	_, err = connection.Write([]byte{0})
	return err
}

// primaryAddress returns an address the container already had, so the control
// half of the experiment uses a source the policy must not touch.
func primaryAddress(t *testing.T, link *net.Interface, exclude netip.Addr) netip.Addr {
	t.Helper()
	addresses, err := link.Addrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, address := range addresses {
		prefix, err := netip.ParsePrefix(address.String())
		if err != nil || !prefix.Addr().Is4() || prefix.Addr() == exclude {
			continue
		}
		return prefix.Addr()
	}
	t.Fatalf("%s has no IPv4 address of its own to use as a control", link.Name)
	return netip.Addr{}
}

func deleteGuardianTable(t *testing.T) {
	t.Helper()
	connection, err := rtnetlink.Dial(unix.NETLINK_NETFILTER)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close() }()
	_ = connection.ExecuteBatch([]rtnetlink.BatchMessage{
		batchBoundary(unix.NFNL_MSG_BATCH_BEGIN),
		{
			Type:    nftMessageType(unix.NFT_MSG_NEWTABLE),
			Flags:   unix.NLM_F_REQUEST | unix.NLM_F_ACK | unix.NLM_F_CREATE,
			Payload: tablePayload(),
		},
		{
			Type:    nftMessageType(unix.NFT_MSG_DELTABLE),
			Flags:   unix.NLM_F_REQUEST | unix.NLM_F_ACK,
			Payload: tablePayload(),
		},
		batchBoundary(unix.NFNL_MSG_BATCH_END),
	})
}
