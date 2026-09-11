//go:build linux

package rtnetlink

import (
	"encoding/binary"
	"errors"
	"net/netip"

	"golang.org/x/sys/unix"
)

// Alignment is the boundary netlink messages and attributes are padded to.
const Alignment = 4

// ErrTruncated is a length the kernel declared that the bytes received do not
// support. The only safe response is to refuse the whole datagram.
var ErrTruncated = errors.New("netlink-message-truncated")

// Message is one netlink message: its header fields and its payload.
type Message struct {
	Type     uint16
	Sequence uint32
	PortID   uint32
	Data     []byte
}

// Attribute is one `struct rtattr` (or `nlattr`) and its value.
type Attribute struct {
	Type  uint16
	Value []byte
}

// Kind is the attribute type without the nested flag.
func (a Attribute) Kind() uint16 { return a.Type &^ unix.NLA_F_NESTED }

// ParseMessages walks a received datagram. Every length the kernel supplied is
// checked against what is actually there before it is used as a bound: a
// message claiming to be longer than the buffer is a truncated read, not
// something to slice with.
func ParseMessages(buffer []byte) ([]Message, error) {
	var messages []Message
	for len(buffer) >= unix.SizeofNlMsghdr {
		length := int(binary.NativeEndian.Uint32(buffer[0:4]))
		if length < unix.SizeofNlMsghdr || length > len(buffer) {
			return nil, ErrTruncated
		}
		messages = append(messages, Message{
			Type:     binary.NativeEndian.Uint16(buffer[4:6]),
			Sequence: binary.NativeEndian.Uint32(buffer[8:12]),
			PortID:   binary.NativeEndian.Uint32(buffer[12:16]),
			Data:     buffer[unix.SizeofNlMsghdr:length],
		})
		aligned := Align(length)
		if aligned >= len(buffer) {
			break
		}
		buffer = buffer[aligned:]
	}
	return messages, nil
}

// ParseAttributes walks a run of attributes with the same bounds discipline.
func ParseAttributes(payload []byte) ([]Attribute, error) {
	var attributes []Attribute
	for len(payload) >= unix.SizeofRtAttr {
		length := int(binary.NativeEndian.Uint16(payload[0:2]))
		if length < unix.SizeofRtAttr || length > len(payload) {
			return nil, ErrTruncated
		}
		attributes = append(attributes, Attribute{
			Type:  binary.NativeEndian.Uint16(payload[2:4]),
			Value: payload[unix.SizeofRtAttr:length],
		})
		aligned := Align(length)
		if aligned >= len(payload) {
			break
		}
		payload = payload[aligned:]
	}
	return attributes, nil
}

func Align(length int) int {
	return (length + Alignment - 1) &^ (Alignment - 1)
}

// AppendAttribute appends one `struct rtattr` and pads to the 4-byte alignment
// netlink requires between attributes.
func AppendAttribute(payload []byte, attributeType uint16, value []byte) []byte {
	header := make([]byte, unix.SizeofRtAttr)
	binary.NativeEndian.PutUint16(header[0:2], uint16(unix.SizeofRtAttr+len(value)))
	binary.NativeEndian.PutUint16(header[2:4], attributeType)
	payload = append(payload, header...)
	payload = append(payload, value...)
	for len(payload)%Alignment != 0 {
		payload = append(payload, 0)
	}
	return payload
}

// AppendNested appends an attribute whose value is itself attributes.
func AppendNested(payload []byte, attributeType uint16, inner []byte) []byte {
	return AppendAttribute(payload, attributeType|unix.NLA_F_NESTED, inner)
}

// AppendString appends a NUL-terminated string attribute.
func AppendString(payload []byte, attributeType uint16, value string) []byte {
	return AppendAttribute(payload, attributeType, append([]byte(value), 0))
}

// AppendUint32 appends a host-order u32, which is what the route family uses.
// (nftables is the exception and encodes its own.)
func AppendUint32(payload []byte, attributeType uint16, value uint32) []byte {
	encoded := make([]byte, 4)
	binary.NativeEndian.PutUint32(encoded, value)
	return AppendAttribute(payload, attributeType, encoded)
}

// StringValue reads a NUL-terminated string attribute.
func StringValue(value []byte) string {
	for index, character := range value {
		if character == 0 {
			return string(value[:index])
		}
	}
	return string(value)
}

// BroadcastFor computes the broadcast address iproute2 would attach. A /31 or
// /32 has none.
func BroadcastFor(prefix netip.Prefix) (netip.Addr, bool) {
	if !prefix.Addr().Is4() || prefix.Bits() > 30 || prefix.Bits() < 0 {
		return netip.Addr{}, false
	}
	network := prefix.Masked().Addr().As4()
	value := binary.BigEndian.Uint32(network[:]) | (^uint32(0) >> uint(prefix.Bits()))
	var broadcast [4]byte
	binary.BigEndian.PutUint32(broadcast[:], value)
	return netip.AddrFrom4(broadcast), true
}

// IPv4From reads a 4-byte address attribute; anything else is invalid.
func IPv4From(value []byte) netip.Addr {
	if len(value) != 4 {
		return netip.Addr{}
	}
	return netip.AddrFrom4([4]byte(value))
}
