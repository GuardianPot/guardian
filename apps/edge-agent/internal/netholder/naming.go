/*
Package netholder is the decoy network holder (ADR 0019): the process that
owns a decoy's network namespace and configures it from inside.

It runs in a Guardian container with CAP_NET_ADMIN over that namespace only.
It listens on nothing, reads nothing but netlink, and takes no arguments: the
one fact it needs — which address the decoy answers on — is carried by the name
of the interface the privileged helper moves in. That keeps the holder's input
to something the helper created and the kernel reports, rather than a command
line or a file an attacker-facing process could reach.
*/
package netholder

import (
	"encoding/hex"
	"net/netip"
	"strings"
)

const (
	// peerPrefix names the veth end the helper moves into a holder's namespace;
	// the rest of the name is the decoy's IPv4 address in hex.
	peerPrefix = "gdp"
	// HostPrefix names the end that stays in the host's namespace. The egress
	// policy matches it: whatever arrives from a decoy's veth is a decoy's.
	HostPrefix = "gdn"
	// InsideName is what the holder renames its end to, so a decoy sees an
	// ordinary eth0 rather than an interface named after its own address.
	InsideName = "eth0"
)

// PeerName is the interface name the helper gives the holder's end of the veth.
func PeerName(address netip.Addr) string { return peerPrefix + addressHex(address) }

// HostName is the interface name of the helper's end of the veth.
func HostName(address netip.Addr) string { return HostPrefix + addressHex(address) }

func addressHex(address netip.Addr) string {
	value := address.As4()
	return hex.EncodeToString(value[:])
}

// DecodePeerName recovers the decoy address from an interface name, and refuses
// anything that is not exactly the helper's form for a unicast IPv4 address.
// Upper-case hex, a short or long suffix, or a loopback, multicast, link-local,
// unspecified, or broadcast address are all refused: the holder configures only
// what the helper could have named. Which unicast addresses are decoy addresses
// is the helper's allowlist to decide, not the holder's.
func DecodePeerName(name string) (netip.Addr, bool) {
	suffix, found := strings.CutPrefix(name, peerPrefix)
	if !found || len(suffix) != 8 || strings.ToLower(suffix) != suffix {
		return netip.Addr{}, false
	}
	raw, err := hex.DecodeString(suffix)
	if err != nil {
		return netip.Addr{}, false
	}
	address := netip.AddrFrom4([4]byte(raw))
	if address.IsUnspecified() || address.IsLoopback() || address.IsMulticast() ||
		address.IsLinkLocalUnicast() || address == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
		return netip.Addr{}, false
	}
	return address, true
}
