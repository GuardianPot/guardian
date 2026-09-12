//go:build linux

package privileged

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/netholder"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/rtnetlink"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
)

/*
The host side of a decoy's network (ADR 0019).

For each running decoy the helper owns exactly three things in the host's
namespace, all created with the CAP_NET_ADMIN it already has:

  - a veth whose other end it moved into the holder's namespace by pid;
  - a /32 route to the decoy's address out of that veth, with proxy_arp on the
    host end so the decoy's own neighbour lookups are answered;
  - forwarding on the zone interface, which is what lets the host deliver what
    its proxy-ARP entry attracted.

The proxy-ARP entry itself is EnsureAddress's, because it is what "the address
is present" now means.

Ownership is marked the way the kernel lets each object be marked. The veth
carries an interface alias naming the workload, the zone, and the holder's pid;
the route carries Guardian's protocol. Nothing without those marks is changed.

Forwarding is the one setting that existed before Guardian. It is changed only
if it was off, the fact that it was off is recorded first, and it is turned off
again when the last decoy on that interface goes. The record lives in the
helper's runtime directory, a tmpfs, so it and the setting it describes are
both gone after a reboot.
*/

const (
	// DefaultNetworkStateDirectory holds one marker per zone interface on which
	// Guardian enabled forwarding. It is inside the helper's RuntimeDirectory.
	DefaultNetworkStateDirectory = "/run/guardian-edge-privd/forwarding"

	holderLinkAliasPrefix = "guardian-decoy "
)

type hostNetwork struct {
	stateDirectory string
	// One attachment changes at a time. Forwarding is shared by every decoy on
	// a zone interface, and a detach deciding "the last one is gone" must not
	// interleave with an attach adding the next.
	mutex sync.Mutex
}

type holderLink struct {
	workloadID string
	zone       string
	pid        uint32
}

func holderLinkAlias(workloadID, zone string, pid uint32) string {
	return holderLinkAliasPrefix + workloadID + " " + zone + " " + strconv.FormatUint(uint64(pid), 10)
}

// parseHolderLinkAlias accepts only the exact form holderLinkAlias writes.
func parseHolderLinkAlias(alias string) (holderLink, bool) {
	rest, found := strings.CutPrefix(alias, holderLinkAliasPrefix)
	if !found {
		return holderLink{}, false
	}
	fields := strings.Split(rest, " ")
	if len(fields) != 3 || !resourcePattern.MatchString(fields[0]) || !validInterfaceName(fields[1]) {
		return holderLink{}, false
	}
	pid, err := strconv.ParseUint(fields[2], 10, 32)
	if err != nil || pid < 2 || strconv.FormatUint(pid, 10) != fields[2] {
		return holderLink{}, false
	}
	return holderLink{workloadID: fields[0], zone: fields[1], pid: uint32(pid)}, true
}

/*
attach connects a running holder to the host. The caller has already confirmed
the egress policy is installed, which is the ordering ADR 0019 requires before
forwarding is enabled.
*/
func (n *hostNetwork) attach(workload Workload, holderPID uint32) (bool, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	address := workload.Network.DecoyAddress()
	connection, err := rtnetlink.DialRoute()
	if err != nil {
		return false, err
	}
	defer func() { _ = connection.Close() }()

	zone, err := connection.LinkByName(workload.Network.Interface)
	if err != nil {
		return false, err
	}
	if zone == nil {
		return false, violation(codes.FailedPrecondition, "interface-not-found")
	}
	hostName := netholder.HostName(address)
	alias := holderLinkAlias(workload.WorkloadID, zone.Name, holderPID)
	changed := false

	// A link of this workload's under another name was made for an address the
	// definition no longer names.
	links, err := connection.Links()
	if err != nil {
		return false, err
	}
	for _, link := range links {
		owner, ok := parseHolderLinkAlias(link.Alias)
		if !ok || owner.workloadID != workload.WorkloadID || link.Name == hostName {
			continue
		}
		if err := deleteLink(connection, link.Index); err != nil {
			return false, err
		}
		changed = true
	}

	host, err := connection.LinkByName(hostName)
	if err != nil {
		return false, err
	}
	if host != nil && host.Alias != alias {
		owner, ok := parseHolderLinkAlias(host.Alias)
		switch {
		case !ok:
			return false, violation(codes.FailedPrecondition, "interface-held-by-host")
		case owner.workloadID != workload.WorkloadID:
			return false, violation(codes.FailedPrecondition, "decoy-address-in-use")
		}
		// This workload's, attached to a holder that has since been replaced or
		// through a zone the definition no longer names.
		if err := deleteLink(connection, host.Index); err != nil {
			return false, err
		}
		host, changed = nil, true
	}
	if host == nil {
		if host, err = createAttachment(connection, hostName, netholder.PeerName(address), alias, holderPID); err != nil {
			return false, err
		}
		changed = true
	}
	routed, err := ensureDecoyRoute(connection, netip.PrefixFrom(address, 32), host.Index)
	if err != nil {
		return false, err
	}
	forwarded, err := n.enableForwarding(connection, *zone)
	if err != nil {
		return false, err
	}
	return changed || routed || forwarded, nil
}

// createAttachment makes the veth, moves its peer into the holder, and brings
// the host end up. A failure part-way removes the pair, which deleting either
// end does.
func createAttachment(connection *rtnetlink.Conn, hostName, peerName, alias string, pid uint32) (*rtnetlink.Link, error) {
	if err := connection.CreateVeth(hostName, peerName, alias); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return nil, violation(codes.FailedPrecondition, "interface-held-by-host")
		}
		return nil, err
	}
	host, err := connection.LinkByName(hostName)
	if err != nil || host == nil {
		return nil, errors.Join(err, errors.New("created veth is not visible"))
	}
	fail := func(err error) (*rtnetlink.Link, error) {
		_ = connection.DeleteLink(host.Index)
		return nil, err
	}
	peer, err := connection.LinkByName(peerName)
	if err != nil || peer == nil {
		return fail(errors.Join(err, errors.New("veth peer is not visible")))
	}
	if err := connection.MoveLinkToProcessNamespace(peer.Index, pid); err != nil {
		if errors.Is(err, unix.ESRCH) {
			return fail(violation(codes.Aborted, "holder-changed-before-start"))
		}
		return fail(err)
	}
	if err := connection.SetIPv4Conf(host.Index, map[uint16]uint32{
		rtnetlink.IPv4DevconfForwarding: 1, rtnetlink.IPv4DevconfProxyARP: 1,
	}); err != nil {
		return fail(err)
	}
	if err := connection.SetLinkUp(host.Index); err != nil {
		return fail(err)
	}
	return host, nil
}

// ensureDecoyRoute installs the /32 route out of the veth. A Guardian route to
// the same address out of another link is stale and replaced; anyone else's is
// refused.
func ensureDecoyRoute(connection *rtnetlink.Conn, decoy netip.Prefix, index int) (bool, error) {
	routes, err := connection.Routes(rtnetlink.ProtocolGuardian)
	if err != nil {
		return false, err
	}
	for _, route := range routes {
		if route.Destination != decoy {
			continue
		}
		if route.OutputIndex == index {
			return false, nil
		}
		if err := connection.DeleteRoute(decoy, route.OutputIndex, rtnetlink.ProtocolGuardian); err != nil && !errors.Is(err, unix.ESRCH) {
			return false, err
		}
	}
	err = connection.AddRoute(decoy, index, rtnetlink.ProtocolGuardian)
	if errors.Is(err, unix.EEXIST) {
		return false, violation(codes.FailedPrecondition, "route-held-by-host")
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (n *hostNetwork) enableForwarding(connection *rtnetlink.Conn, zone rtnetlink.Link) (bool, error) {
	current, err := connection.IPv4Conf(zone.Index, rtnetlink.IPv4DevconfForwarding)
	if err != nil {
		return false, err
	}
	if current == 1 {
		return false, nil
	}
	// Recorded before it is changed, so a helper that dies between the two
	// still restores a value it actually found.
	if err := os.MkdirAll(n.stateDirectory, 0o700); err != nil {
		return false, err
	}
	if err := os.WriteFile(filepath.Join(n.stateDirectory, zone.Name), []byte("0\n"), 0o600); err != nil {
		return false, err
	}
	if err := connection.SetIPv4Conf(zone.Index, map[uint16]uint32{rtnetlink.IPv4DevconfForwarding: 1}); err != nil {
		return false, err
	}
	return true, nil
}

// detach removes every link this workload owns, and restores forwarding on any
// zone interface no Guardian link uses any more.
func (n *hostNetwork) detach(workloadID string) (bool, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	connection, err := rtnetlink.DialRoute()
	if err != nil {
		return false, err
	}
	defer func() { _ = connection.Close() }()

	links, err := connection.Links()
	if err != nil {
		return false, err
	}
	changed := false
	inUse := map[string]struct{}{}
	for _, link := range links {
		owner, ok := parseHolderLinkAlias(link.Alias)
		if !ok {
			continue
		}
		if owner.workloadID != workloadID {
			inUse[owner.zone] = struct{}{}
			continue
		}
		// The route out of the link goes with it.
		if err := deleteLink(connection, link.Index); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, n.restoreForwarding(connection, inUse)
}

func (n *hostNetwork) restoreForwarding(connection *rtnetlink.Conn, inUse map[string]struct{}) error {
	entries, err := os.ReadDir(n.stateDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if _, used := inUse[name]; used || !validInterfaceName(name) || !entry.Type().IsRegular() {
			continue
		}
		zone, err := connection.LinkByName(name)
		if err != nil {
			return err
		}
		if zone != nil {
			if err := connection.SetIPv4Conf(zone.Index, map[uint16]uint32{rtnetlink.IPv4DevconfForwarding: 0}); err != nil {
				return err
			}
		}
		if err := os.Remove(filepath.Join(n.stateDirectory, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func deleteLink(connection *rtnetlink.Conn, index int) error {
	if err := connection.DeleteLink(index); err != nil && !errors.Is(err, unix.ENODEV) {
		return err
	}
	return nil
}
