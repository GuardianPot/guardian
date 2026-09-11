//go:build linux

package privileged

import (
	"context"
	"encoding/binary"
	"errors"
	"net/netip"
	"strings"
	"testing"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"golang.org/x/sys/unix"
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
		"an IPv6 address, which cannot carry the ownership label": {
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
		// A label is capped at IFNAMSIZ-1 by the kernel. An interface whose name
		// leaves no room for one would get an unlabelled address, and an
		// unlabelled address is one this adapter could not later prove it added.
		"an interface name too long to label": {
			addressRequest("verylonginterface", "10.20.0.40/32", present),
			"interface-name-too-long-to-label",
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

// The label is the kernel's own ownership marker and it has a hard length
// limit. A name that does not fit is refused rather than left unmarked.
func TestGuardianLabelsFitTheKernelsLimit(t *testing.T) {
	for name, wantOK := range map[string]bool{
		"eth0":              true,
		"ens192":            true,
		"guardian0":         true,
		"br-decoy-01":       true,
		"":                  false,
		"enx00e04c680001":   false,
		"verylonginterface": false,
	} {
		label, ok := guardianLabel(name)
		if ok != wantOK {
			t.Fatalf("guardianLabel(%q) ok = %v, want %v", name, ok, wantOK)
		}
		if !ok {
			continue
		}
		if len(label) >= unix.IFNAMSIZ {
			t.Fatalf("label %q is %d bytes, kernel allows %d", label, len(label), unix.IFNAMSIZ-1)
		}
		if !strings.HasPrefix(label, name+":") {
			t.Fatalf("label %q does not begin with the interface name", label)
		}
	}
}

func TestBroadcastMatchesTheSubnet(t *testing.T) {
	for prefix, want := range map[string]string{
		"10.20.0.40/24":  "10.20.0.255",
		"10.20.0.40/16":  "10.20.255.255",
		"192.168.5.9/30": "192.168.5.11",
	} {
		broadcast, ok := broadcastFor(netip.MustParsePrefix(prefix))
		if !ok || broadcast.String() != want {
			t.Fatalf("broadcastFor(%s) = %v %v, want %s", prefix, broadcast, ok, want)
		}
	}
	// A /31 and a /32 have no broadcast address, and iproute2 attaches none.
	for _, prefix := range []string{"10.20.0.40/31", "10.20.0.40/32"} {
		if _, ok := broadcastFor(netip.MustParsePrefix(prefix)); ok {
			t.Fatalf("%s was given a broadcast address", prefix)
		}
	}
}

func encodedMessage(messageType uint16, sequence, portID uint32, payload []byte) []byte {
	message := make([]byte, unix.SizeofNlMsghdr)
	binary.NativeEndian.PutUint32(message[0:4], uint32(unix.SizeofNlMsghdr+len(payload)))
	binary.NativeEndian.PutUint16(message[4:6], messageType)
	binary.NativeEndian.PutUint32(message[8:12], sequence)
	binary.NativeEndian.PutUint32(message[12:16], portID)
	return append(message, payload...)
}

// What this adapter writes is what it reads back, attribute for attribute.
func TestNetlinkFramingRoundTrips(t *testing.T) {
	prefix := netip.MustParsePrefix("10.20.0.40/24")
	local := prefix.Addr().As4()
	payload := ifAddrPayload(7, prefix)
	payload = appendAttribute(payload, unix.IFA_LOCAL, local[:])
	payload = appendAttribute(payload, unix.IFA_LABEL, append([]byte("eth0:gdn"), 0))

	messages, err := parseNetlinkMessages(encodedMessage(unix.RTM_NEWADDR, 3, 99, payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("parsed %d messages, want 1", len(messages))
	}
	message := messages[0]
	if message.messageType != unix.RTM_NEWADDR || message.sequence != 3 || message.portID != 99 {
		t.Fatalf("header = %+v", message)
	}
	if message.data[0] != unix.AF_INET || message.data[1] != 24 {
		t.Fatalf("ifaddrmsg family/prefix = %d/%d", message.data[0], message.data[1])
	}
	if index := binary.NativeEndian.Uint32(message.data[4:8]); index != 7 {
		t.Fatalf("interface index = %d", index)
	}

	attributes, err := parseNetlinkAttributes(message.data[unix.SizeofIfAddrmsg:])
	if err != nil {
		t.Fatal(err)
	}
	if len(attributes) != 2 {
		t.Fatalf("parsed %d attributes, want 2", len(attributes))
	}
	if attributes[0].attributeType != unix.IFA_LOCAL || ipv4From(attributes[0].value) != prefix.Addr() {
		t.Fatalf("first attribute = %+v", attributes[0])
	}
	// The label is NUL-terminated on the wire, and the padding after it must
	// not become part of the next attribute.
	if attributes[1].attributeType != unix.IFA_LABEL ||
		string(attributes[1].value) != "eth0:gdn\x00" {
		t.Fatalf("second attribute = %q", attributes[1].value)
	}
}

/*
 * A length the kernel handed over is checked before it is used as a bound.
 *
 * This is the one place a root process parses a buffer it did not write. A
 * message claiming to be longer than what arrived means the read was truncated,
 * and the only safe response is to refuse the whole datagram.
 */
func TestATruncatedOrLyingLengthIsRefused(t *testing.T) {
	payload := ifAddrPayload(7, netip.MustParsePrefix("10.20.0.40/24"))
	payload = appendAttribute(payload, unix.IFA_LOCAL, []byte{10, 20, 0, 40})
	complete := encodedMessage(unix.RTM_NEWADDR, 3, 99, payload)

	if _, err := parseNetlinkMessages(complete[:len(complete)-4]); !errors.Is(err, errNetlinkTruncated) {
		t.Fatalf("a short read parsed as %v", err)
	}

	lying := append([]byte(nil), complete...)
	binary.NativeEndian.PutUint32(lying[0:4], ^uint32(0))
	if _, err := parseNetlinkMessages(lying); !errors.Is(err, errNetlinkTruncated) {
		t.Fatalf("an oversized declared length parsed as %v", err)
	}

	empty := append([]byte(nil), complete...)
	binary.NativeEndian.PutUint32(empty[0:4], 0)
	if _, err := parseNetlinkMessages(empty); !errors.Is(err, errNetlinkTruncated) {
		t.Fatalf("a zero declared length parsed as %v", err)
	}

	// The same rule one level down, where a zero-length attribute would
	// otherwise loop forever.
	if _, err := parseNetlinkAttributes([]byte{0, 0, 0, 0}); !errors.Is(err, errNetlinkTruncated) {
		t.Fatalf("a zero-length attribute parsed as %v", err)
	}
	if _, err := parseNetlinkAttributes([]byte{0xff, 0xff, 0, 0}); !errors.Is(err, errNetlinkTruncated) {
		t.Fatalf("an oversized attribute parsed as %v", err)
	}
}
