//go:build linux

package privileged

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"time"

	"golang.org/x/sys/unix"
)

/*
NETLINK_ROUTE, spoken directly.

This file is the only thing in Guardian that changes the customer's network,
and it talks to the kernel rather than to `ip` on purpose. The boundary this
package exists to hold forbids a process-execution primitive in the root
daemon — a security test greps for one — because an `exec` path in a process
holding CAP_NET_ADMIN is one unchecked string away from being an
arbitrary-command path. Hand-encoding two message types is the price of not
having that path at all, and it is a price worth paying once.

The message parser is written here rather than taken from `syscall` because
that one reinterprets the receive buffer through unsafe pointer casts. This one
does explicit bounds checks on a byte slice, which is what a root process
should be doing to something a kernel handed it.

Everything here is IPv4. `presence.Address` already refuses anything else, and
the ownership label below exists only for IPv4 addresses.
*/

const (
	// A root process must never block forever on a kernel that does not answer.
	netlinkReceiveTimeout = 2 * time.Second
	netlinkBufferBytes    = 64 << 10

	// guardianLabelSuffix marks every address this helper adds. IPv4 address
	// labels are the kernel's own ownership marker: `ip -4 addr show` prints
	// them, so an operator can see which addresses are Guardian's, and the
	// adapter refuses to remove one that is not.
	guardianLabelSuffix = ":gdn"
	// A label is capped at IFNAMSIZ-1 characters by the kernel, and convention
	// requires it to start with the interface name.
	maxLabelledInterface = unix.IFNAMSIZ - 1 - len(guardianLabelSuffix)
)

var errNetlinkTruncated = errors.New("netlink-message-truncated")

// guardianLabel returns the ownership label for an interface, and whether one
// fits. A name too long to label is refused rather than silently left unmarked:
// an unmarked address is one this adapter could later delete without knowing
// whether it put it there.
func guardianLabel(interfaceName string) (string, bool) {
	if interfaceName == "" || len(interfaceName) > maxLabelledInterface {
		return "", false
	}
	return interfaceName + guardianLabelSuffix, true
}

// hostAddress is one IPv4 address observed on an interface.
type hostAddress struct {
	prefix netip.Prefix
	label  string
}

type netlinkMessage struct {
	messageType uint16
	sequence    uint32
	portID      uint32
	data        []byte
}

type netlinkAttribute struct {
	attributeType uint16
	value         []byte
}

type netlinkConn struct {
	fd       int
	portID   uint32
	sequence uint32
}

func dialNetlink() (*netlinkConn, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_ROUTE)
	if err != nil {
		return nil, err
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	name, err := unix.Getsockname(fd)
	if err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	bound, ok := name.(*unix.SockaddrNetlink)
	if !ok {
		_ = unix.Close(fd)
		return nil, errors.New("netlink socket is not bound to a netlink address")
	}
	timeout := unix.NsecToTimeval(int64(netlinkReceiveTimeout))
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &timeout); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return &netlinkConn{fd: fd, portID: bound.Pid}, nil
}

func (c *netlinkConn) Close() error { return unix.Close(c.fd) }

// execute sends one request and reads the kernel's answer to it.
func (c *netlinkConn) execute(messageType, flags uint16, payload []byte) ([]netlinkMessage, error) {
	c.sequence++
	sequence := c.sequence
	total := unix.SizeofNlMsghdr + len(payload)
	request := make([]byte, unix.SizeofNlMsghdr, total)
	binary.NativeEndian.PutUint32(request[0:4], uint32(total))
	binary.NativeEndian.PutUint16(request[4:6], messageType)
	binary.NativeEndian.PutUint16(request[6:8], flags)
	binary.NativeEndian.PutUint32(request[8:12], sequence)
	binary.NativeEndian.PutUint32(request[12:16], c.portID)
	request = append(request, payload...)
	if err := unix.Sendto(c.fd, request, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, err
	}
	return c.receive(sequence)
}

/*
receive collects the reply to one sequence number.

Three filters, and each is a refusal rather than a convenience. A datagram from
any port other than 0 did not come from the kernel and is dropped unparsed; a
message carrying another sequence number or another port ID answers a different
request and is ignored. The socket's receive timeout is what stops this loop on
a kernel that never answers at all.
*/
func (c *netlinkConn) receive(sequence uint32) ([]netlinkMessage, error) {
	var collected []netlinkMessage
	for {
		// A fresh buffer per read: a dump spans several datagrams, and the
		// messages collected from earlier ones point into the buffer that read
		// them.
		buffer := make([]byte, netlinkBufferBytes)
		read, from, err := unix.Recvfrom(c.fd, buffer, 0)
		if err != nil {
			return nil, err
		}
		source, ok := from.(*unix.SockaddrNetlink)
		if !ok || source.Pid != 0 {
			continue
		}
		messages, err := parseNetlinkMessages(buffer[:read])
		if err != nil {
			return nil, err
		}
		for _, message := range messages {
			if message.sequence != sequence || message.portID != c.portID {
				continue
			}
			switch message.messageType {
			case unix.NLMSG_ERROR:
				if len(message.data) < 4 {
					return nil, errNetlinkTruncated
				}
				// A zero errno is the acknowledgement of a successful change.
				number := int32(binary.NativeEndian.Uint32(message.data[0:4]))
				if number == 0 {
					return collected, nil
				}
				return nil, unix.Errno(-number)
			case unix.NLMSG_DONE:
				return collected, nil
			default:
				collected = append(collected, message)
			}
		}
	}
}

// addressesOn returns every IPv4 address the kernel reports on one interface.
func (c *netlinkConn) addressesOn(index int) ([]hostAddress, error) {
	payload := make([]byte, unix.SizeofIfAddrmsg)
	payload[0] = unix.AF_INET
	messages, err := c.execute(unix.RTM_GETADDR, unix.NLM_F_REQUEST|unix.NLM_F_DUMP, payload)
	if err != nil {
		return nil, err
	}
	addresses := make([]hostAddress, 0, len(messages))
	for _, message := range messages {
		if message.messageType != unix.RTM_NEWADDR || len(message.data) < unix.SizeofIfAddrmsg {
			continue
		}
		if message.data[0] != unix.AF_INET {
			continue
		}
		if int(binary.NativeEndian.Uint32(message.data[4:8])) != index {
			continue
		}
		prefixLength := int(message.data[1])
		attributes, err := parseNetlinkAttributes(message.data[unix.SizeofIfAddrmsg:])
		if err != nil {
			return nil, err
		}
		var local, interfaceAddress netip.Addr
		var label string
		for _, attribute := range attributes {
			switch attribute.attributeType {
			case unix.IFA_LOCAL:
				local = ipv4From(attribute.value)
			case unix.IFA_ADDRESS:
				interfaceAddress = ipv4From(attribute.value)
			case unix.IFA_LABEL:
				label = string(bytes.TrimRight(attribute.value, "\x00"))
			}
		}
		// IFA_LOCAL is the address the host answers on; IFA_ADDRESS is the peer
		// on a point-to-point link and identical everywhere else.
		if !local.IsValid() {
			local = interfaceAddress
		}
		if !local.IsValid() {
			continue
		}
		addresses = append(addresses, hostAddress{
			prefix: netip.PrefixFrom(local, prefixLength),
			label:  label,
		})
	}
	return addresses, nil
}

// findAddress returns the observed entry for an exact address and prefix
// length, or nil if the kernel does not report one.
func (c *netlinkConn) findAddress(index int, prefix netip.Prefix) (*hostAddress, error) {
	addresses, err := c.addressesOn(index)
	if err != nil {
		return nil, err
	}
	for _, address := range addresses {
		if address.prefix == prefix {
			found := address
			return &found, nil
		}
	}
	return nil, nil
}

func (c *netlinkConn) addAddress(index int, prefix netip.Prefix, label string) error {
	payload := ifAddrPayload(index, prefix)
	local := prefix.Addr().As4()
	payload = appendAttribute(payload, unix.IFA_LOCAL, local[:])
	payload = appendAttribute(payload, unix.IFA_ADDRESS, local[:])
	if broadcast, ok := broadcastFor(prefix); ok {
		value := broadcast.As4()
		payload = appendAttribute(payload, unix.IFA_BROADCAST, value[:])
	}
	payload = appendAttribute(payload, unix.IFA_LABEL, append([]byte(label), 0))
	// NLM_F_EXCL makes a pre-existing address an EEXIST the caller has to
	// resolve, rather than a silent overwrite of somebody else's entry.
	_, err := c.execute(unix.RTM_NEWADDR,
		unix.NLM_F_REQUEST|unix.NLM_F_ACK|unix.NLM_F_CREATE|unix.NLM_F_EXCL, payload)
	return err
}

func (c *netlinkConn) deleteAddress(index int, prefix netip.Prefix) error {
	payload := ifAddrPayload(index, prefix)
	local := prefix.Addr().As4()
	payload = appendAttribute(payload, unix.IFA_LOCAL, local[:])
	payload = appendAttribute(payload, unix.IFA_ADDRESS, local[:])
	_, err := c.execute(unix.RTM_DELADDR, unix.NLM_F_REQUEST|unix.NLM_F_ACK, payload)
	return err
}

// ifAddrPayload builds the fixed `struct ifaddrmsg` header: family, prefix
// length, flags, scope, interface index.
func ifAddrPayload(index int, prefix netip.Prefix) []byte {
	payload := make([]byte, unix.SizeofIfAddrmsg)
	payload[0] = unix.AF_INET
	payload[1] = byte(prefix.Bits())
	payload[2] = 0
	payload[3] = unix.RT_SCOPE_UNIVERSE
	binary.NativeEndian.PutUint32(payload[4:8], uint32(index))
	return payload
}

// appendAttribute appends one `struct rtattr` and pads to the 4-byte alignment
// netlink requires between attributes.
func appendAttribute(payload []byte, attributeType uint16, value []byte) []byte {
	header := make([]byte, unix.SizeofRtAttr)
	binary.NativeEndian.PutUint16(header[0:2], uint16(unix.SizeofRtAttr+len(value)))
	binary.NativeEndian.PutUint16(header[2:4], attributeType)
	payload = append(payload, header...)
	payload = append(payload, value...)
	for len(payload)%netlinkAlignment != 0 {
		payload = append(payload, 0)
	}
	return payload
}

const netlinkAlignment = 4

// parseNetlinkMessages walks a received datagram. Every length the kernel
// supplied is checked against what is actually there before it is used as a
// bound: a message claiming to be longer than the buffer is a truncated read,
// not something to slice with.
func parseNetlinkMessages(buffer []byte) ([]netlinkMessage, error) {
	var messages []netlinkMessage
	for len(buffer) >= unix.SizeofNlMsghdr {
		length := int(binary.NativeEndian.Uint32(buffer[0:4]))
		if length < unix.SizeofNlMsghdr || length > len(buffer) {
			return nil, errNetlinkTruncated
		}
		messages = append(messages, netlinkMessage{
			messageType: binary.NativeEndian.Uint16(buffer[4:6]),
			sequence:    binary.NativeEndian.Uint32(buffer[8:12]),
			portID:      binary.NativeEndian.Uint32(buffer[12:16]),
			data:        buffer[unix.SizeofNlMsghdr:length],
		})
		aligned := netlinkAlign(length)
		if aligned >= len(buffer) {
			break
		}
		buffer = buffer[aligned:]
	}
	return messages, nil
}

func parseNetlinkAttributes(payload []byte) ([]netlinkAttribute, error) {
	var attributes []netlinkAttribute
	for len(payload) >= unix.SizeofRtAttr {
		length := int(binary.NativeEndian.Uint16(payload[0:2]))
		if length < unix.SizeofRtAttr || length > len(payload) {
			return nil, errNetlinkTruncated
		}
		attributes = append(attributes, netlinkAttribute{
			attributeType: binary.NativeEndian.Uint16(payload[2:4]),
			value:         payload[unix.SizeofRtAttr:length],
		})
		aligned := netlinkAlign(length)
		if aligned >= len(payload) {
			break
		}
		payload = payload[aligned:]
	}
	return attributes, nil
}

func netlinkAlign(length int) int {
	return (length + netlinkAlignment - 1) &^ (netlinkAlignment - 1)
}

// broadcastFor computes the broadcast address iproute2 would attach. A /31 or
// /32 has none.
func broadcastFor(prefix netip.Prefix) (netip.Addr, bool) {
	if !prefix.Addr().Is4() || prefix.Bits() > 30 || prefix.Bits() < 0 {
		return netip.Addr{}, false
	}
	network := prefix.Masked().Addr().As4()
	value := binary.BigEndian.Uint32(network[:]) | (^uint32(0) >> uint(prefix.Bits()))
	var broadcast [4]byte
	binary.BigEndian.PutUint32(broadcast[:], value)
	return netip.AddrFrom4(broadcast), true
}

func ipv4From(value []byte) netip.Addr {
	if len(value) != 4 {
		return netip.Addr{}
	}
	return netip.AddrFrom4([4]byte(value))
}
