//go:build linux

/*
Package rtnetlink speaks netlink to the kernel directly: the route family for
addresses, links, routes, and neighbours, and the framing nftables batches
ride on.

It is shared by two processes with very different privilege — the root helper,
which changes the host's network, and the decoy network holder, which changes
only its own namespace — so it holds mechanism and no policy. What may be
changed, and what counts as Guardian's to change, is decided by the callers.

It exists instead of `ip` or a netlink library for the same two reasons in both
places: no process-execution primitive in a process holding CAP_NET_ADMIN, and
no third-party parser handling bytes a kernel handed to a privileged process.
Every length the kernel supplies is checked against what arrived before it is
used as a bound.
*/
package rtnetlink

import (
	"encoding/binary"
	"errors"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// A privileged process must never block forever on a kernel that does not
	// answer.
	receiveTimeout = 2 * time.Second
	bufferBytes    = 64 << 10
)

// Conn is one netlink socket.
type Conn struct {
	fd       int
	portID   uint32
	sequence uint32
}

// DialRoute opens NETLINK_ROUTE, which address, link, route, and neighbour work
// uses.
func DialRoute() (*Conn, error) { return Dial(unix.NETLINK_ROUTE) }

// Dial opens a netlink socket of the given protocol.
func Dial(protocol int) (*Conn, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, protocol)
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
	timeout := unix.NsecToTimeval(int64(receiveTimeout))
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &timeout); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return &Conn{fd: fd, portID: bound.Pid}, nil
}

func (c *Conn) Close() error { return unix.Close(c.fd) }

// Execute sends one request and reads the kernel's answer to it.
func (c *Conn) Execute(messageType, flags uint16, payload []byte) ([]Message, error) {
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
func (c *Conn) receive(sequence uint32) ([]Message, error) {
	var collected []Message
	for {
		// A fresh buffer per read: a dump spans several datagrams, and the
		// messages collected from earlier ones point into the buffer that read
		// them.
		buffer := make([]byte, bufferBytes)
		read, from, err := unix.Recvfrom(c.fd, buffer, 0)
		if err != nil {
			return nil, err
		}
		source, ok := from.(*unix.SockaddrNetlink)
		if !ok || source.Pid != 0 {
			continue
		}
		messages, err := ParseMessages(buffer[:read])
		if err != nil {
			return nil, err
		}
		for _, message := range messages {
			if message.Sequence != sequence || message.PortID != c.portID {
				continue
			}
			switch message.Type {
			case unix.NLMSG_ERROR:
				if len(message.Data) < 4 {
					return nil, ErrTruncated
				}
				// A zero errno is the acknowledgement of a successful change.
				number := int32(binary.NativeEndian.Uint32(message.Data[0:4]))
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

// BatchMessage is one message inside a netlink transaction.
type BatchMessage struct {
	Type    uint16
	Flags   uint16
	Payload []byte
}

/*
ExecuteBatch sends several messages as one datagram and waits for the kernel to
acknowledge all of them.

nftables applies a batch atomically: either every message in it takes effect or
none does. That is what lets an egress policy be replaced without a window in
which it is absent, which for a default-deny control is the difference between
reconfiguration and a hole.
*/
func (c *Conn) ExecuteBatch(messages []BatchMessage) error {
	if len(messages) == 0 {
		return nil
	}
	var datagram []byte
	awaited := map[uint32]struct{}{}
	for _, message := range messages {
		c.sequence++
		// Only messages that asked to be acknowledged will be. The transaction
		// boundaries are handled by nfnetlink itself and answer nothing, so
		// waiting on the last sequence number rather than on these would wait
		// for a reply the kernel never sends.
		if message.Flags&unix.NLM_F_ACK != 0 {
			awaited[c.sequence] = struct{}{}
		}
		total := unix.SizeofNlMsghdr + len(message.Payload)
		header := make([]byte, unix.SizeofNlMsghdr)
		binary.NativeEndian.PutUint32(header[0:4], uint32(total))
		binary.NativeEndian.PutUint16(header[4:6], message.Type)
		binary.NativeEndian.PutUint16(header[6:8], message.Flags)
		binary.NativeEndian.PutUint32(header[8:12], c.sequence)
		binary.NativeEndian.PutUint32(header[12:16], c.portID)
		datagram = append(datagram, header...)
		datagram = append(datagram, message.Payload...)
		for len(datagram)%Alignment != 0 {
			datagram = append(datagram, 0)
		}
	}
	if err := unix.Sendto(c.fd, datagram, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return err
	}
	return c.acknowledge(awaited)
}

// acknowledge reads until every message that asked for an acknowledgement has
// been answered, and fails on the first non-zero errno. A batch is
// all-or-nothing, so one refusal means nothing in it was applied.
func (c *Conn) acknowledge(awaited map[uint32]struct{}) error {
	for len(awaited) != 0 {
		buffer := make([]byte, bufferBytes)
		read, from, err := unix.Recvfrom(c.fd, buffer, 0)
		if err != nil {
			return err
		}
		source, ok := from.(*unix.SockaddrNetlink)
		if !ok || source.Pid != 0 {
			continue
		}
		messages, err := ParseMessages(buffer[:read])
		if err != nil {
			return err
		}
		for _, message := range messages {
			if message.PortID != c.portID || message.Type != unix.NLMSG_ERROR {
				continue
			}
			if _, expected := awaited[message.Sequence]; !expected {
				continue
			}
			if len(message.Data) < 4 {
				return ErrTruncated
			}
			if number := int32(binary.NativeEndian.Uint32(message.Data[0:4])); number != 0 {
				return unix.Errno(-number)
			}
			delete(awaited, message.Sequence)
		}
	}
	return nil
}

// change is the flag set for a request that alters kernel state and must be
// acknowledged.
const change = unix.NLM_F_REQUEST | unix.NLM_F_ACK
