package netholder

import (
	"net/netip"
	"testing"
)

func TestInterfaceNamesCarryTheDecoyAddress(t *testing.T) {
	address := netip.MustParseAddr("172.30.20.99")
	if got := PeerName(address); got != "gdpac1e1463" {
		t.Fatalf("peer name = %q", got)
	}
	if got := HostName(address); got != "gdnac1e1463" {
		t.Fatalf("host name = %q", got)
	}
	// Both fit the kernel's 15-character interface names with room to spare.
	if len(PeerName(address)) > 15 || len(HostName(address)) > 15 {
		t.Fatal("an interface name exceeds IFNAMSIZ")
	}
	decoded, ok := DecodePeerName(PeerName(address))
	if !ok || decoded != address {
		t.Fatalf("round trip = %v %v", decoded, ok)
	}
}

// The holder configures only what the helper could have named. Anything else
// in its namespace is not a decoy interface.
func TestOnlyTheHelpersExactFormIsADecoyInterface(t *testing.T) {
	for _, name := range []string{
		"eth0", "lo", "gdn0a141e28", // the host end's prefix, not the peer's
		"gdp0a141e2", "gdp0a141e2800", // too short, too long
		"gdp0A141E28",  // upper case
		"gdpzz141e28",  // not hex
		"gdp7f000001",  // 127.0.0.1
		"gdp00000000",  // 0.0.0.0
		"gdpe0000001",  // 224.0.0.1
		"gdpa9fe0001",  // 169.254.0.1
		"gdpffffffff",  // 255.255.255.255
		"xgdp0a141e28", // not a prefix
	} {
		if address, ok := DecodePeerName(name); ok {
			t.Fatalf("%q decoded to %v", name, address)
		}
	}
	// Which unicast addresses are decoys is the helper's allowlist to decide.
	// A decoy zone need not be RFC 1918 space.
	if address, ok := DecodePeerName("gdpc000020a"); !ok || address != netip.MustParseAddr("192.0.2.10") {
		t.Fatalf("a unicast address outside private space = %v %v", address, ok)
	}
}
