//go:build linux

package rtnetlink

import (
	"encoding/binary"
	"errors"

	"golang.org/x/sys/unix"
)

// Kernel enum values x/sys/unix does not export, named as the kernel headers
// name them. Each is exercised against a real kernel by the labs.
const (
	vethInfoPeer = 1 // VETH_INFO_PEER, <linux/veth.h>

	// IPv4DevconfForwarding and IPv4DevconfProxyARP index the per-interface
	// IPv4 settings in IFLA_INET_CONF (<linux/ip.h>, 1-based).
	IPv4DevconfForwarding = 1 // IPV4_DEVCONF_FORWARDING
	IPv4DevconfProxyARP   = 3 // IPV4_DEVCONF_PROXY_ARP
)

// Link is one network interface as the kernel reports it.
type Link struct {
	Index int
	Name  string
	Alias string
	Up    bool
}

func ifInfoPayload(index int, flags, changeMask uint32) []byte {
	payload := make([]byte, unix.SizeofIfInfomsg)
	payload[0] = unix.AF_UNSPEC
	binary.NativeEndian.PutUint32(payload[4:8], uint32(int32(index)))
	binary.NativeEndian.PutUint32(payload[8:12], flags)
	binary.NativeEndian.PutUint32(payload[12:16], changeMask)
	return payload
}

func parseLink(message Message) (Link, bool) {
	if message.Type != unix.RTM_NEWLINK || len(message.Data) < unix.SizeofIfInfomsg {
		return Link{}, false
	}
	link := Link{
		Index: int(int32(binary.NativeEndian.Uint32(message.Data[4:8]))),
		Up:    binary.NativeEndian.Uint32(message.Data[8:12])&unix.IFF_UP != 0,
	}
	attributes, err := ParseAttributes(message.Data[unix.SizeofIfInfomsg:])
	if err != nil {
		return Link{}, false
	}
	for _, attribute := range attributes {
		switch attribute.Kind() {
		case unix.IFLA_IFNAME:
			link.Name = StringValue(attribute.Value)
		case unix.IFLA_IFALIAS:
			link.Alias = StringValue(attribute.Value)
		}
	}
	return link, true
}

// Links returns every interface in the socket's namespace.
func (c *Conn) Links() ([]Link, error) {
	messages, err := c.Execute(unix.RTM_GETLINK, unix.NLM_F_REQUEST|unix.NLM_F_DUMP, ifInfoPayload(0, 0, 0))
	if err != nil {
		return nil, err
	}
	links := make([]Link, 0, len(messages))
	for _, message := range messages {
		if link, ok := parseLink(message); ok {
			links = append(links, link)
		}
	}
	return links, nil
}

// LinkByName returns the named interface, or nil if there is none.
func (c *Conn) LinkByName(name string) (*Link, error) {
	payload := AppendString(ifInfoPayload(0, 0, 0), unix.IFLA_IFNAME, name)
	messages, err := c.Execute(unix.RTM_GETLINK, lookup, payload)
	if errors.Is(err, unix.ENODEV) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for _, message := range messages {
		if link, ok := parseLink(message); ok && link.Name == name {
			return &link, nil
		}
	}
	return nil, nil
}

/*
CreateVeth creates a veth pair in the socket's namespace and marks the named end
with an interface alias.

The alias is the ownership marker for links, the way the address label is for
addresses and the protocol field is for routes and neighbours: `ip link`
prints it, and callers refuse to delete a link that does not carry theirs.
*/
func (c *Conn) CreateVeth(name, peer, alias string) error {
	peerInfo := AppendString(ifInfoPayload(0, 0, 0), unix.IFLA_IFNAME, peer)
	data := AppendNested(nil, vethInfoPeer, peerInfo)
	info := AppendString(nil, unix.IFLA_INFO_KIND, "veth")
	info = AppendNested(info, unix.IFLA_INFO_DATA, data)
	payload := AppendString(ifInfoPayload(0, 0, 0), unix.IFLA_IFNAME, name)
	payload = AppendNested(payload, unix.IFLA_LINKINFO, info)
	if _, err := c.Execute(unix.RTM_NEWLINK, change|unix.NLM_F_CREATE|unix.NLM_F_EXCL, payload); err != nil {
		return err
	}
	link, err := c.LinkByName(name)
	if err != nil || link == nil {
		return errors.Join(err, errors.New("created veth is not visible"))
	}
	// The kernel copies the alias without its terminator, so none is sent.
	aliased := AppendAttribute(ifInfoPayload(link.Index, 0, 0), unix.IFLA_IFALIAS, []byte(alias))
	if _, err := c.Execute(unix.RTM_NEWLINK, change, aliased); err != nil {
		// An unmarked link is one no caller would ever remove.
		_ = c.DeleteLink(link.Index)
		return err
	}
	return nil
}

// MoveLinkToProcessNamespace moves an interface into the network namespace of
// a process. The namespace is named by the process rather than opened, which
// is what lets this work with CAP_NET_ADMIN alone.
func (c *Conn) MoveLinkToProcessNamespace(index int, pid uint32) error {
	payload := AppendUint32(ifInfoPayload(index, 0, 0), unix.IFLA_NET_NS_PID, pid)
	_, err := c.Execute(unix.RTM_NEWLINK, change, payload)
	return err
}

func (c *Conn) SetLinkUp(index int) error {
	_, err := c.Execute(unix.RTM_NEWLINK, change, ifInfoPayload(index, unix.IFF_UP, unix.IFF_UP))
	return err
}

// RenameLink renames a link that is down.
func (c *Conn) RenameLink(index int, name string) error {
	payload := AppendString(ifInfoPayload(index, 0, 0), unix.IFLA_IFNAME, name)
	_, err := c.Execute(unix.RTM_NEWLINK, change, payload)
	return err
}

// DeleteLink removes a link; for a veth this removes both ends.
func (c *Conn) DeleteLink(index int) error {
	_, err := c.Execute(unix.RTM_DELLINK, change, ifInfoPayload(index, 0, 0))
	return err
}

/*
SetIPv4Conf sets per-interface IPv4 settings (forwarding, proxy_arp) through
RTM_NEWLINK rather than /proc/sys.

The helper's /proc/sys is read-only by its service profile, and that is a good
property to keep for everything else under it. This path changes exactly the
settings named, on exactly the interface named, and needs only CAP_NET_ADMIN.
*/
func (c *Conn) SetIPv4Conf(index int, settings map[uint16]uint32) error {
	var conf []byte
	for id, value := range settings {
		conf = AppendUint32(conf, id, value)
	}
	inet := AppendNested(nil, unix.IFLA_INET_CONF, conf)
	spec := AppendNested(nil, unix.AF_INET, inet)
	payload := AppendNested(ifInfoPayload(index, 0, 0), unix.IFLA_AF_SPEC, spec)
	_, err := c.Execute(unix.RTM_NEWLINK, change, payload)
	return err
}

// IPv4Conf reads one per-interface IPv4 setting back from the kernel.
func (c *Conn) IPv4Conf(index int, id uint16) (uint32, error) {
	messages, err := c.Execute(unix.RTM_GETLINK, lookup, ifInfoPayload(index, 0, 0))
	if err != nil {
		return 0, err
	}
	for _, message := range messages {
		if message.Type != unix.RTM_NEWLINK || len(message.Data) < unix.SizeofIfInfomsg {
			continue
		}
		attributes, err := ParseAttributes(message.Data[unix.SizeofIfInfomsg:])
		if err != nil {
			return 0, err
		}
		for _, attribute := range attributes {
			if attribute.Kind() != unix.IFLA_AF_SPEC {
				continue
			}
			value, found, err := inetConfValue(attribute.Value, id)
			if err != nil || found {
				return value, err
			}
		}
	}
	return 0, errors.New("ipv4 settings not reported for this interface")
}

func inetConfValue(spec []byte, id uint16) (uint32, bool, error) {
	families, err := ParseAttributes(spec)
	if err != nil {
		return 0, false, err
	}
	for _, family := range families {
		if family.Kind() != unix.AF_INET {
			continue
		}
		inet, err := ParseAttributes(family.Value)
		if err != nil {
			return 0, false, err
		}
		for _, entry := range inet {
			if entry.Kind() != unix.IFLA_INET_CONF {
				continue
			}
			// Asymmetric on purpose, in the kernel: IFLA_INET_CONF is written as
			// nested attributes but reported as a plain u32 array, where the
			// setting with id N is element N-1. The lab found this.
			offset := (int(id) - 1) * 4
			if id == 0 || offset+4 > len(entry.Value) {
				return 0, false, ErrTruncated
			}
			return binary.NativeEndian.Uint32(entry.Value[offset : offset+4]), true, nil
		}
	}
	return 0, false, nil
}
