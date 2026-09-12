//go:build linux

package privileged

import (
	"context"
	"testing"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"google.golang.org/grpc/codes"
)

// available is an adapter that believes it can do address work, so the checks
// below are reached. Whether it really can is a separate question, and the
// lab test is where that one is answered.
func available() hostAdapter {
	return hostAdapter{address: AdapterCapability{
		State:      privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE,
		ReasonCode: "netlink-address-adapter",
	}}
}

func addressRequest(name, prefix string, state privilegedv1.PresenceState) AddressOperation {
	return AddressOperation{InterfaceName: name, AddressPrefix: prefix, DesiredState: state}
}

/*
 * Every refusal below happens before a netlink socket is opened.
 *
 * That ordering is the point: an operation this adapter cannot express safely
 * must be rejected while it is still an argument, not discovered halfway
 * through changing a live interface.
 */
func TestTheAdapterRefusesBeforeItTouchesTheHost(t *testing.T) {
	present := privilegedv1.PresenceState_PRESENCE_STATE_PRESENT
	for name, testCase := range map[string]struct {
		operation AddressOperation
		reason    string
	}{
		"an unspecified desired state": {
			addressRequest("lo", "10.20.0.40/32", privilegedv1.PresenceState_PRESENCE_STATE_UNSPECIFIED),
			"invalid-presence-state",
		},
		"an IPv6 address, which proxy ARP cannot answer for": {
			addressRequest("lo", "fd00::1/64", present), "unsupported-address-family",
		},
		"an address that is not a prefix at all": {
			addressRequest("lo", "10.20.0.40", present), "unsupported-address-family",
		},
		// ADR 0016: a zone-length prefix would install a connected route for
		// the whole subnet on the interface.
		"a zone-length prefix rather than a /32 identity": {
			addressRequest("lo", "10.20.0.40/24", present), "address-must-be-host-identity",
		},
		"an IPv4-mapped IPv6 address wearing IPv4's clothes": {
			addressRequest("lo", "::ffff:10.20.0.40/120", present), "unsupported-address-family",
		},
		"an interface that does not exist": {
			addressRequest("gdnabsent0", "10.20.0.40/32", present), "interface-not-found",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := available().EnsureAddress(context.Background(), testCase.operation)
			refusal, ok := asViolation(err)
			if !ok {
				t.Fatalf("err = %v, want a typed refusal", err)
			}
			if refusal.ReasonCode != testCase.reason {
				t.Fatalf("reason = %q, want %q", refusal.ReasonCode, testCase.reason)
			}
			if refusal.Code == codes.OK {
				t.Fatal("a refusal carried an OK status")
			}
		})
	}
}

// Removing an address from an interface that is not there has nothing to do,
// and reporting a failure would make a reconciler retry forever.
func TestRemovingFromAnAbsentInterfaceIsNotAFailure(t *testing.T) {
	result, err := available().EnsureAddress(context.Background(),
		addressRequest("gdnabsent0", "10.20.0.40/32", privilegedv1.PresenceState_PRESENCE_STATE_ABSENT))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED ||
		result.ReasonCode != "interface-absent" {
		t.Fatalf("result = %+v", result)
	}
}

/*
 * An adapter that cannot do the work says so, and does not try.
 *
 * `presence.Reconciler` maps `unsupported` to a status that never claims the
 * address is on the interface, so a host without the capability reports no
 * coverage rather than imaginary coverage.
 */
func TestAnAdapterWithoutTheCapabilityClaimsNothing(t *testing.T) {
	adapter := hostAdapter{address: AdapterCapability{
		State:      privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
		ReasonCode: "no-cap-net-admin",
	}}
	for _, state := range []privilegedv1.PresenceState{
		privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
		privilegedv1.PresenceState_PRESENCE_STATE_ABSENT,
	} {
		result, err := adapter.EnsureAddress(context.Background(),
			addressRequest("lo", "10.20.0.40/32", state))
		if err != nil {
			t.Fatal(err)
		}
		if result.Outcome != privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNSUPPORTED ||
			result.ReasonCode != "no-cap-net-admin" {
			t.Fatalf("result = %+v", result)
		}
	}
	// Network namespaces stay unimplemented: creating one needs CAP_SYS_ADMIN,
	// which the helper does not hold.
	for _, operation := range []privilegedv1.PrivilegedOperation{
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_NETWORK_NAMESPACE,
	} {
		capability := adapter.Capabilities()[operation]
		if capability.State != privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED ||
			capability.ReasonCode != unsupportedReason {
			t.Fatalf("%v = %+v, want an unimplemented capability", operation, capability)
		}
	}
}

// An operation an adapter did not mention is unsupported. A missing entry is
// not a claim, and the status must never turn one into an available capability.
func TestAnUnmentionedOperationIsNeverAvailable(t *testing.T) {
	server, err := NewServer(ServerConfig{
		Adapter: silentAdapter{},
		Audit:   &memoryAudit{},
		Runtime: runtimeProberFunc(func(context.Context) (bool, string) { return false, "probe-failed" }),
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.GetStatus(context.Background(), &privilegedv1.GetStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.GetCapabilities()) != 4 {
		t.Fatalf("reported %d capabilities, want 4", len(response.GetCapabilities()))
	}
	for _, capability := range response.GetCapabilities() {
		if capability.GetState() != privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED {
			t.Fatalf("%v = %v, want unsupported", capability.GetOperation(), capability.GetState())
		}
		if !reasonCodePattern.MatchString(capability.GetReasonCode()) {
			t.Fatalf("reason %q is not a closed token", capability.GetReasonCode())
		}
	}
}

// silentAdapter reports nothing at all, which is the shape of a future adapter
// that forgets an operation.
type silentAdapter struct{ UnsupportedAdapter }

func (silentAdapter) Capabilities() map[privilegedv1.PrivilegedOperation]AdapterCapability {
	return nil
}

// Whatever a probe finds, it is never reported as a capability Guardian has
// unless it was actually established.
func TestTheProbedCapabilityIsOneOfTwoHonestAnswers(t *testing.T) {
	capability := probeAddressCapability()
	switch capability.State {
	case privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE:
		if capability.ReasonCode != "netlink-address-adapter" {
			t.Fatalf("reason = %q", capability.ReasonCode)
		}
		if !holdsCapNetAdmin() {
			t.Fatal("address work was claimed without CAP_NET_ADMIN")
		}
	case privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED:
		if capability.ReasonCode != "no-cap-net-admin" && capability.ReasonCode != "netlink-unavailable" {
			t.Fatalf("reason = %q, want the cause of the refusal", capability.ReasonCode)
		}
	default:
		t.Fatalf("state = %v, which is neither available nor unsupported", capability.State)
	}
	if !reasonCodePattern.MatchString(capability.ReasonCode) {
		t.Fatalf("reason %q is not a closed token", capability.ReasonCode)
	}
}
