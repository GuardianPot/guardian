//go:build linux

package rtnetlink

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"
)

func TestBroadcastMatchesTheSubnet(t *testing.T) {
	for prefix, want := range map[string]string{
		"10.20.0.40/24":  "10.20.0.255",
		"10.20.0.40/16":  "10.20.255.255",
		"192.168.5.9/30": "192.168.5.11",
	} {
		broadcast, ok := BroadcastFor(netip.MustParsePrefix(prefix))
		if !ok || broadcast.String() != want {
			t.Fatalf("BroadcastFor(%s) = %v %v, want %s", prefix, broadcast, ok, want)
		}
	}
	// A /31 and a /32 have no broadcast address, and iproute2 attaches none.
	for _, prefix := range []string{"10.20.0.40/31", "10.20.0.40/32"} {
		if _, ok := BroadcastFor(netip.MustParsePrefix(prefix)); ok {
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
	payload = AppendAttribute(payload, unix.IFA_LOCAL, local[:])
	payload = AppendAttribute(payload, unix.IFA_LABEL, append([]byte("eth0:gdn"), 0))

	messages, err := ParseMessages(encodedMessage(unix.RTM_NEWADDR, 3, 99, payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("parsed %d messages, want 1", len(messages))
	}
	message := messages[0]
	if message.Type != unix.RTM_NEWADDR || message.Sequence != 3 || message.PortID != 99 {
		t.Fatalf("header = %+v", message)
	}
	if message.Data[0] != unix.AF_INET || message.Data[1] != 24 {
		t.Fatalf("ifaddrmsg family/prefix = %d/%d", message.Data[0], message.Data[1])
	}
	if index := binary.NativeEndian.Uint32(message.Data[4:8]); index != 7 {
		t.Fatalf("interface index = %d", index)
	}

	attributes, err := ParseAttributes(message.Data[unix.SizeofIfAddrmsg:])
	if err != nil {
		t.Fatal(err)
	}
	if len(attributes) != 2 {
		t.Fatalf("parsed %d attributes, want 2", len(attributes))
	}
	if attributes[0].Type != unix.IFA_LOCAL || IPv4From(attributes[0].Value) != prefix.Addr() {
		t.Fatalf("first attribute = %+v", attributes[0])
	}
	// The label is NUL-terminated on the wire, and the padding after it must
	// not become part of the next attribute.
	if attributes[1].Type != unix.IFA_LABEL ||
		string(attributes[1].Value) != "eth0:gdn\x00" {
		t.Fatalf("second attribute = %q", attributes[1].Value)
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
	payload = AppendAttribute(payload, unix.IFA_LOCAL, []byte{10, 20, 0, 40})
	complete := encodedMessage(unix.RTM_NEWADDR, 3, 99, payload)

	if _, err := ParseMessages(complete[:len(complete)-4]); !errors.Is(err, ErrTruncated) {
		t.Fatalf("a short read parsed as %v", err)
	}

	lying := append([]byte(nil), complete...)
	binary.NativeEndian.PutUint32(lying[0:4], ^uint32(0))
	if _, err := ParseMessages(lying); !errors.Is(err, ErrTruncated) {
		t.Fatalf("an oversized declared length parsed as %v", err)
	}

	empty := append([]byte(nil), complete...)
	binary.NativeEndian.PutUint32(empty[0:4], 0)
	if _, err := ParseMessages(empty); !errors.Is(err, ErrTruncated) {
		t.Fatalf("a zero declared length parsed as %v", err)
	}

	// The same rule one level down, where a zero-length attribute would
	// otherwise loop forever.
	if _, err := ParseAttributes([]byte{0, 0, 0, 0}); !errors.Is(err, ErrTruncated) {
		t.Fatalf("a zero-length attribute parsed as %v", err)
	}
	if _, err := ParseAttributes([]byte{0xff, 0xff, 0, 0}); !errors.Is(err, ErrTruncated) {
		t.Fatalf("an oversized attribute parsed as %v", err)
	}
}
