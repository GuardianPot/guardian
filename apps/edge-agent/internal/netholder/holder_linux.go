//go:build linux

package netholder

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/rtnetlink"
	"golang.org/x/sys/unix"
)

// ErrAmbiguous is more than one candidate interface in the namespace. The
// holder configures none of them: a namespace it cannot describe exactly is one
// it will not put a decoy address in.
var ErrAmbiguous = errors.New("more than one decoy interface in the namespace")

/*
Converge brings the namespace to the state a decoy needs, once.

  - loopback up;
  - the helper's veth end, found by name, given the decoy's address as a /32;
  - that end renamed to eth0;
  - the link up;
  - a default route out of it, carrying Guardian's protocol.

The address goes on before the rename because the name is where the address
comes from. After the rename the /32 on eth0 is the record of it, so a holder
interrupted anywhere in the sequence finishes it on the next pass, and running
it again on a finished namespace changes nothing. A namespace the helper has not
attached yet is left exactly as it is.
*/
func Converge(connection *rtnetlink.Conn) (bool, error) {
	links, err := connection.Links()
	if err != nil {
		return false, err
	}
	changed := false
	var peer, inside *rtnetlink.Link
	var address netip.Addr
	for index := range links {
		link := links[index]
		switch {
		case link.Name == "lo":
			if !link.Up {
				if err := connection.SetLinkUp(link.Index); err != nil {
					return false, err
				}
				changed = true
			}
		case link.Name == InsideName:
			inside = &link
		default:
			if candidate, ok := DecodePeerName(link.Name); ok {
				if peer != nil {
					return false, ErrAmbiguous
				}
				peer, address = &link, candidate
			}
		}
	}
	switch {
	case peer != nil && inside != nil:
		return false, ErrAmbiguous
	case peer != nil:
		prefix := netip.PrefixFrom(address, 32)
		if err := connection.AddAddress(peer.Index, prefix, peer.Name); err != nil && !errors.Is(err, unix.EEXIST) {
			return false, err
		}
		if err := connection.RenameLink(peer.Index, InsideName); err != nil {
			return false, err
		}
		if _, err := finish(connection, peer.Index, false); err != nil {
			return false, err
		}
		return true, nil
	case inside != nil:
		addresses, err := connection.Addresses(inside.Index)
		if err != nil {
			return false, err
		}
		// eth0 without exactly one /32 is not one this holder configured.
		if len(addresses) != 1 || addresses[0].Prefix.Bits() != 32 {
			return changed, nil
		}
		finished, err := finish(connection, inside.Index, inside.Up)
		return changed || finished, err
	}
	return changed, nil
}

func finish(connection *rtnetlink.Conn, index int, up bool) (bool, error) {
	changed := false
	if !up {
		if err := connection.SetLinkUp(index); err != nil {
			return false, err
		}
		changed = true
	}
	// A device route with no gateway: the host's end of the veth answers ARP
	// for everything it can route (proxy_arp), which is all the decoy needs.
	err := connection.AddRoute(netip.PrefixFrom(netip.IPv4Unspecified(), 0), index, rtnetlink.ProtocolGuardian)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, unix.EEXIST):
		return changed, nil
	}
	return false, err
}

// Run converges repeatedly until the context ends. It polls rather than
// subscribing to link events: the attach happens once per holder, a fraction of
// a second of latency is invisible to anyone reaching the decoy, and a loop
// with no state is one fewer thing to get wrong in a privileged process.
func Run(ctx context.Context, interval time.Duration) error {
	connection, err := rtnetlink.DialRoute()
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		// Errors are retried on the next tick. The holder has no one to report
		// them to; what it achieves is observed from the host, where the decoy
		// either answers or does not.
		_, _ = Converge(connection)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
