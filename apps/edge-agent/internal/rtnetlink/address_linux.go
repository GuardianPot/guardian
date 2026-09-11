//go:build linux

package rtnetlink

import (
	"bytes"
	"encoding/binary"
	"net/netip"

	"golang.org/x/sys/unix"
)

// Address is one IPv4 address observed on an interface.
type Address struct {
	Prefix netip.Prefix
	Label  string
}

// Addresses returns every IPv4 address the kernel reports on one interface.
func (c *Conn) Addresses(index int) ([]Address, error) {
	payload := make([]byte, unix.SizeofIfAddrmsg)
	payload[0] = unix.AF_INET
	messages, err := c.Execute(unix.RTM_GETADDR, unix.NLM_F_REQUEST|unix.NLM_F_DUMP, payload)
	if err != nil {
		return nil, err
	}
	addresses := make([]Address, 0, len(messages))
	for _, message := range messages {
		if message.Type != unix.RTM_NEWADDR || len(message.Data) < unix.SizeofIfAddrmsg {
			continue
		}
		if message.Data[0] != unix.AF_INET {
			continue
		}
		if int(binary.NativeEndian.Uint32(message.Data[4:8])) != index {
			continue
		}
		prefixLength := int(message.Data[1])
		attributes, err := ParseAttributes(message.Data[unix.SizeofIfAddrmsg:])
		if err != nil {
			return nil, err
		}
		var local, interfaceAddress netip.Addr
		var label string
		for _, attribute := range attributes {
			switch attribute.Type {
			case unix.IFA_LOCAL:
				local = IPv4From(attribute.Value)
			case unix.IFA_ADDRESS:
				interfaceAddress = IPv4From(attribute.Value)
			case unix.IFA_LABEL:
				label = string(bytes.TrimRight(attribute.Value, "\x00"))
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
		addresses = append(addresses, Address{Prefix: netip.PrefixFrom(local, prefixLength), Label: label})
	}
	return addresses, nil
}

// FindAddress returns the observed entry for an exact address and prefix
// length, or nil if the kernel does not report one.
func (c *Conn) FindAddress(index int, prefix netip.Prefix) (*Address, error) {
	addresses, err := c.Addresses(index)
	if err != nil {
		return nil, err
	}
	for _, address := range addresses {
		if address.Prefix == prefix {
			found := address
			return &found, nil
		}
	}
	return nil, nil
}

// AddAddress adds an IPv4 address with the given label. NLM_F_EXCL makes a
// pre-existing address an EEXIST the caller has to resolve, rather than a
// silent overwrite of somebody else's entry.
func (c *Conn) AddAddress(index int, prefix netip.Prefix, label string) error {
	payload := ifAddrPayload(index, prefix)
	local := prefix.Addr().As4()
	payload = AppendAttribute(payload, unix.IFA_LOCAL, local[:])
	payload = AppendAttribute(payload, unix.IFA_ADDRESS, local[:])
	if broadcast, ok := BroadcastFor(prefix); ok {
		value := broadcast.As4()
		payload = AppendAttribute(payload, unix.IFA_BROADCAST, value[:])
	}
	payload = AppendString(payload, unix.IFA_LABEL, label)
	_, err := c.Execute(unix.RTM_NEWADDR, change|unix.NLM_F_CREATE|unix.NLM_F_EXCL, payload)
	return err
}

func (c *Conn) DeleteAddress(index int, prefix netip.Prefix) error {
	payload := ifAddrPayload(index, prefix)
	local := prefix.Addr().As4()
	payload = AppendAttribute(payload, unix.IFA_LOCAL, local[:])
	payload = AppendAttribute(payload, unix.IFA_ADDRESS, local[:])
	_, err := c.Execute(unix.RTM_DELADDR, change, payload)
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
