//go:build linux

package rtnetlink

import (
	"encoding/binary"
	"net/netip"
	"time"

	"golang.org/x/sys/unix"
)

// ProtocolGuardian marks the routes and neighbour entries Guardian creates, in
// the kernel's own `protocol` field. It is unassigned in iproute2's
// rt_protos, so `ip route show proto 71` and `ip neigh show proxy` name them,
// and deletions that carry it match only Guardian's entries.
const ProtocolGuardian = 71

// More kernel enum values x/sys/unix does not export.
const (
	ndaProtocol     = 12 // NDA_PROTOCOL, <linux/neighbour.h>
	ndtaName        = 1  // NDTA_NAME
	ndtaParms       = 6  // NDTA_PARMS
	ndtpaIfindex    = 1  // NDTPA_IFINDEX
	ndtpaProxyDelay = 13 // NDTPA_PROXY_DELAY, milliseconds
	sizeofNdtmsg    = 4  // struct ndtmsg
)

// Route is one IPv4 route in the main table.
type Route struct {
	Destination netip.Prefix
	OutputIndex int
	Protocol    uint8
}

func routePayload(destination netip.Prefix, protocol uint8) []byte {
	payload := make([]byte, unix.SizeofRtMsg)
	payload[0] = unix.AF_INET
	payload[1] = byte(destination.Bits())
	payload[4] = unix.RT_TABLE_MAIN
	payload[5] = protocol
	payload[6] = unix.RT_SCOPE_LINK
	payload[7] = unix.RTN_UNICAST
	if destination.Bits() > 0 {
		address := destination.Addr().As4()
		payload = AppendAttribute(payload, unix.RTA_DST, address[:])
	}
	return payload
}

// AddRoute adds a device route: destination reached directly out of an
// interface, with no gateway.
func (c *Conn) AddRoute(destination netip.Prefix, outputIndex int, protocol uint8) error {
	payload := AppendUint32(routePayload(destination, protocol), unix.RTA_OIF, uint32(outputIndex))
	_, err := c.Execute(unix.RTM_NEWROUTE, change|unix.NLM_F_CREATE|unix.NLM_F_EXCL, payload)
	return err
}

// DeleteRoute removes a route. The protocol is part of the match, so a caller
// passing its own cannot remove anyone else's route to the same destination.
func (c *Conn) DeleteRoute(destination netip.Prefix, outputIndex int, protocol uint8) error {
	payload := AppendUint32(routePayload(destination, protocol), unix.RTA_OIF, uint32(outputIndex))
	_, err := c.Execute(unix.RTM_DELROUTE, change, payload)
	return err
}

// Routes returns the main table's IPv4 routes carrying a protocol.
func (c *Conn) Routes(protocol uint8) ([]Route, error) {
	request := make([]byte, unix.SizeofRtMsg)
	request[0] = unix.AF_INET
	messages, err := c.Execute(unix.RTM_GETROUTE, unix.NLM_F_REQUEST|unix.NLM_F_DUMP, request)
	if err != nil {
		return nil, err
	}
	var routes []Route
	for _, message := range messages {
		if message.Type != unix.RTM_NEWROUTE || len(message.Data) < unix.SizeofRtMsg {
			continue
		}
		if message.Data[4] != unix.RT_TABLE_MAIN || message.Data[5] != protocol {
			continue
		}
		route := Route{Protocol: message.Data[5]}
		destination := netip.IPv4Unspecified()
		attributes, err := ParseAttributes(message.Data[unix.SizeofRtMsg:])
		if err != nil {
			return nil, err
		}
		for _, attribute := range attributes {
			switch attribute.Kind() {
			case unix.RTA_DST:
				destination = IPv4From(attribute.Value)
			case unix.RTA_OIF:
				if len(attribute.Value) == 4 {
					route.OutputIndex = int(binary.NativeEndian.Uint32(attribute.Value))
				}
			}
		}
		route.Destination = netip.PrefixFrom(destination, int(message.Data[1]))
		routes = append(routes, route)
	}
	return routes, nil
}

// ProxyNeighbour is one proxy-ARP entry: the host answers ARP for Address on
// the interface.
type ProxyNeighbour struct {
	Address  netip.Addr
	Index    int
	Protocol uint8
}

func neighbourPayload(index int) []byte {
	payload := make([]byte, unix.SizeofNdMsg)
	payload[0] = unix.AF_INET
	binary.NativeEndian.PutUint32(payload[4:8], uint32(int32(index)))
	binary.NativeEndian.PutUint16(payload[8:10], unix.NUD_PERMANENT)
	payload[10] = unix.NTF_PROXY
	return payload
}

// AddProxyNeighbour makes the host answer ARP for an address on an interface.
func (c *Conn) AddProxyNeighbour(index int, address netip.Addr, protocol uint8) error {
	value := address.As4()
	payload := AppendAttribute(neighbourPayload(index), unix.NDA_DST, value[:])
	payload = AppendAttribute(payload, ndaProtocol, []byte{protocol})
	_, err := c.Execute(unix.RTM_NEWNEIGH, change|unix.NLM_F_CREATE|unix.NLM_F_EXCL, payload)
	return err
}

func (c *Conn) DeleteProxyNeighbour(index int, address netip.Addr) error {
	value := address.As4()
	payload := AppendAttribute(neighbourPayload(index), unix.NDA_DST, value[:])
	_, err := c.Execute(unix.RTM_DELNEIGH, change, payload)
	return err
}

// ProxyNeighbours returns the proxy-ARP entries on one interface.
func (c *Conn) ProxyNeighbours(index int) ([]ProxyNeighbour, error) {
	messages, err := c.Execute(unix.RTM_GETNEIGH, unix.NLM_F_REQUEST|unix.NLM_F_DUMP, neighbourPayload(0))
	if err != nil {
		return nil, err
	}
	var entries []ProxyNeighbour
	for _, message := range messages {
		if message.Type != unix.RTM_NEWNEIGH || len(message.Data) < unix.SizeofNdMsg {
			continue
		}
		if message.Data[0] != unix.AF_INET || message.Data[10]&unix.NTF_PROXY == 0 {
			continue
		}
		entry := ProxyNeighbour{Index: int(int32(binary.NativeEndian.Uint32(message.Data[4:8])))}
		if entry.Index != index {
			continue
		}
		attributes, err := ParseAttributes(message.Data[unix.SizeofNdMsg:])
		if err != nil {
			return nil, err
		}
		for _, attribute := range attributes {
			switch attribute.Kind() {
			case unix.NDA_DST:
				entry.Address = IPv4From(attribute.Value)
			case ndaProtocol:
				if len(attribute.Value) >= 1 {
					entry.Protocol = attribute.Value[0]
				}
			}
		}
		if entry.Address.IsValid() {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// SetProxyDelay sets how long the kernel waits before answering ARP for a
// proxied address on one interface. The default is up to 0.8s, which a real
// host does not have and which a careful scanner can time.
func (c *Conn) SetProxyDelay(index int, delay time.Duration) error {
	payload := make([]byte, sizeofNdtmsg)
	payload[0] = unix.AF_INET
	payload = AppendString(payload, ndtaName, "arp_cache")
	parameters := AppendUint32(nil, ndtpaIfindex, uint32(index))
	milliseconds := make([]byte, 8)
	binary.NativeEndian.PutUint64(milliseconds, uint64(delay.Milliseconds()))
	parameters = AppendAttribute(parameters, ndtpaProxyDelay, milliseconds)
	payload = AppendNested(payload, ndtaParms, parameters)
	_, err := c.Execute(unix.RTM_SETNEIGHTBL, change, payload)
	return err
}
