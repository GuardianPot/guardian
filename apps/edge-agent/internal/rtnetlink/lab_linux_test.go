//go:build linux

package rtnetlink

import (
	"errors"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

/*
The route-family lab: every operation the helper needs for ADR 0019's host
side, against a real kernel, in a container holding CAP_NET_ADMIN and nothing
else. That capability set is the point: each call below is one the helper will
make under the same bound.

Moving a link into another process's namespace is exercised by the containerd
lab instead, because this container has no second namespace to move into.
*/

const labEnv = "GUARDIAN_NETLINK_LAB"

func TestRouteFamilyOperationsAgainstALiveKernel(t *testing.T) {
	if os.Getenv(labEnv) != "1" {
		t.Skip("the route-family lab changes a live namespace and runs only in its own container")
	}
	connection, err := DialRoute()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	const host, peer, alias = "gdnlab0", "gdplab0", "guardian-decoy lab"
	t.Cleanup(func() {
		if link, _ := connection.LinkByName(host); link != nil {
			_ = connection.DeleteLink(link.Index)
		}
	})

	t.Run("a veth pair is created and carries its ownership alias", func(t *testing.T) {
		if err := connection.CreateVeth(host, peer, alias); err != nil {
			t.Fatal(err)
		}
		link, err := connection.LinkByName(host)
		if err != nil || link == nil {
			t.Fatalf("host end = %v %v", link, err)
		}
		if link.Alias != alias {
			t.Fatalf("alias = %q, want %q", link.Alias, alias)
		}
		if other, _ := connection.LinkByName(peer); other == nil {
			t.Fatal("the peer end does not exist")
		}
		if missing, err := connection.LinkByName("gdnabsent9"); err != nil || missing != nil {
			t.Fatalf("an absent link = %v %v, want nil", missing, err)
		}
		if err := connection.CreateVeth(host, peer, alias); !errors.Is(err, unix.EEXIST) {
			t.Fatalf("a second create = %v, want EEXIST rather than an overwrite", err)
		}
	})

	link, err := connection.LinkByName(host)
	if err != nil || link == nil {
		t.Fatal("the lab veth is missing")
	}

	t.Run("a link is renamed while down and then brought up", func(t *testing.T) {
		peerLink, _ := connection.LinkByName(peer)
		if err := connection.RenameLink(peerLink.Index, "gdplab1"); err != nil {
			t.Fatal(err)
		}
		if renamed, _ := connection.LinkByName("gdplab1"); renamed == nil || renamed.Index != peerLink.Index {
			t.Fatal("the rename did not take")
		}
		if err := connection.SetLinkUp(link.Index); err != nil {
			t.Fatal(err)
		}
		if up, _ := connection.LinkByName(host); up == nil || !up.Up {
			t.Fatal("the link is not up")
		}
	})

	/*
	 * Forwarding and proxy_arp through netlink. The helper's /proc/sys is
	 * read-only by its service profile; this is the path it has, and it is
	 * read back both through netlink and through /proc, which reach the same
	 * setting by two unrelated routes.
	 */
	t.Run("per-interface IPv4 settings are set and read back", func(t *testing.T) {
		if err := connection.SetIPv4Conf(link.Index, map[uint16]uint32{
			IPv4DevconfForwarding: 1, IPv4DevconfProxyARP: 1,
		}); err != nil {
			t.Fatal(err)
		}
		for id, name := range map[uint16]string{IPv4DevconfForwarding: "forwarding", IPv4DevconfProxyARP: "proxy_arp"} {
			value, err := connection.IPv4Conf(link.Index, id)
			if err != nil || value != 1 {
				t.Fatalf("%s via netlink = %d %v", name, value, err)
			}
			if got := readSysctl(t, "net/ipv4/conf/"+host+"/"+name); got != "1" {
				t.Fatalf("%s via /proc = %q", name, got)
			}
		}
		if err := connection.SetIPv4Conf(link.Index, map[uint16]uint32{IPv4DevconfForwarding: 0}); err != nil {
			t.Fatal(err)
		}
		if value, _ := connection.IPv4Conf(link.Index, IPv4DevconfForwarding); value != 0 {
			t.Fatal("forwarding could not be turned back off")
		}
	})

	/*
	 * Routes and proxy neighbours carry Guardian's protocol, and a deletion
	 * carrying it does not match someone else's entry for the same address.
	 */
	t.Run("a /32 route is Guardian's and only Guardian's is removed", func(t *testing.T) {
		decoy := netip.MustParsePrefix("10.99.7.40/32")
		if err := connection.AddRoute(decoy, link.Index, ProtocolGuardian); err != nil {
			t.Fatal(err)
		}
		routes, err := connection.Routes(ProtocolGuardian)
		if err != nil || len(routes) != 1 || routes[0].Destination != decoy || routes[0].OutputIndex != link.Index {
			t.Fatalf("guardian routes = %+v %v", routes, err)
		}
		// A route to the same destination with another protocol is not ours.
		if err := connection.DeleteRoute(decoy, link.Index, 4 /* static */); err == nil {
			t.Fatal("a deletion under another protocol removed Guardian's route")
		}
		if err := connection.DeleteRoute(decoy, link.Index, ProtocolGuardian); err != nil {
			t.Fatal(err)
		}
		if routes, _ := connection.Routes(ProtocolGuardian); len(routes) != 0 {
			t.Fatalf("routes left behind: %+v", routes)
		}
	})

	t.Run("a proxy neighbour is added with Guardian's protocol and removed", func(t *testing.T) {
		address := netip.MustParseAddr("10.99.7.40")
		if err := connection.AddProxyNeighbour(link.Index, address, ProtocolGuardian); err != nil {
			t.Fatal(err)
		}
		entries, err := connection.ProxyNeighbours(link.Index)
		if err != nil || len(entries) != 1 || entries[0].Address != address || entries[0].Protocol != ProtocolGuardian {
			t.Fatalf("proxy entries = %+v %v", entries, err)
		}
		if err := connection.DeleteProxyNeighbour(link.Index, address); err != nil {
			t.Fatal(err)
		}
		if entries, _ := connection.ProxyNeighbours(link.Index); len(entries) != 0 {
			t.Fatalf("proxy entries left behind: %+v", entries)
		}
	})

	t.Run("the proxy-ARP delay is removed", func(t *testing.T) {
		if err := connection.SetProxyDelay(link.Index, 0); err != nil {
			t.Fatal(err)
		}
		if got := readSysctl(t, "net/ipv4/neigh/"+host+"/proxy_delay"); got != "0" {
			t.Fatalf("proxy_delay = %q, want 0", got)
		}
	})

	t.Run("deleting one end removes the pair", func(t *testing.T) {
		if err := connection.DeleteLink(link.Index); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
		for _, name := range []string{host, "gdplab1"} {
			if remaining, _ := connection.LinkByName(name); remaining != nil {
				t.Fatalf("%s survived the deletion", name)
			}
		}
	})
}

func readSysctl(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile("/proc/sys/" + path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}
