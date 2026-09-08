package presence

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

/*
NeighbourProbe answers AC-ON-004 without privilege.

The obvious implementation of "does another host already answer on this
address" is an ARP probe, which needs a raw socket, which needs root and a new
operation on the privileged helper's contract. This does not do that.

Instead it asks the kernel to answer the question it already knows how to ask.
Sending one datagram to an on-link address makes the kernel resolve the
neighbour; the result lands in the neighbour table, which is a world-readable
file. A complete entry with a hardware address means something replied. An
entry that never completed means nothing did.

Verified unprivileged on Linux: uid 1000 reads /proc/net/arp, the default
gateway resolves with flags 0x2 and a real MAC, and an unused address in the
same subnet stays at flags 0x0 with a zero MAC.

This is a pre-check on one address the operator already chose. It is not
discovery: it takes a single address per call, sends a single datagram, and
enumerates nothing. There is no code path here that walks a range or a port
list, and there is no reason to add one.
*/
type NeighbourProbe struct {
	// table reads the kernel's neighbour cache. A seam so the parsing can be
	// tested on any platform, including the one this repository is developed on.
	table func() (map[string]neighbour, error)
	// nudge sends the datagram that makes the kernel resolve the address.
	nudge func(context.Context, Address) error
	// settle bounds how long the kernel is given to resolve or give up.
	settle time.Duration
	// interval is how often the table is re-read while waiting.
	interval time.Duration
}

// neighbour is one row of the kernel's cache.
type neighbour struct {
	flags    int
	hardware string
	device   string
}

// complete reports whether the kernel resolved this neighbour to a host.
//
// ATF_COM (0x2) is the kernel's own "this entry is complete". The zero
// hardware address is checked as well because a complete entry without one is
// not evidence of a host, and this is the one place where being wrong means
// either colliding with production or refusing a free address forever.
func (n neighbour) complete() bool {
	return n.flags&0x2 != 0 && n.hardware != "" && n.hardware != "00:00:00:00:00:00"
}

const (
	// defaultSettle is how long the kernel gets. Linux gives up on an
	// unanswered ARP well inside this, and a decoy address is probed only when
	// it is new or moved — a steady-state reconcile probes nothing.
	defaultSettle = 2 * time.Second
	// discardPort is RFC 863. The datagram is never read by anything; it exists
	// only to make the kernel ask who has the address.
	discardPort = "9"
)

var errProbeUnsupported = errors.New("neighbour probing is not supported on this platform")

// NewNeighbourProbe returns the probe for this host.
//
// On a platform without a readable neighbour cache it returns a probe that
// reports it cannot check, which the reconciler treats as a refusal. That is
// the fail-safe direction: a developer machine that cannot answer the question
// must not be able to claim an address.
func NewNeighbourProbe() *NeighbourProbe {
	probe := &NeighbourProbe{settle: defaultSettle, interval: 150 * time.Millisecond, nudge: sendNudge}
	if runtime.GOOS != "linux" {
		probe.table = func() (map[string]neighbour, error) { return nil, errProbeUnsupported }
		return probe
	}
	probe.table = func() (map[string]neighbour, error) { return readNeighbourFile("/proc/net/arp") }
	return probe
}

// InUse implements ConflictProbe.
func (p *NeighbourProbe) InUse(ctx context.Context, address Address) (bool, bool, error) {
	host, err := address.Host()
	if err != nil {
		return false, false, err
	}
	// Read the table first. If the answer is already there — the address is a
	// live host this Edge has spoken to recently — there is no reason to send
	// anything at all.
	if entries, err := p.table(); err == nil {
		if entry, ok := entries[host.String()]; ok && entry.complete() {
			return true, true, nil
		}
	} else {
		// The table is the whole mechanism. Without it nothing below can be
		// interpreted, so the honest answer is "not checked".
		return false, false, nil
	}

	if err := p.nudge(ctx, address); err != nil {
		// A refused send is not evidence that the address is free.
		return false, false, nil
	}

	deadline := time.Now().Add(p.settle)
	for {
		select {
		case <-ctx.Done():
			return false, false, ctx.Err()
		case <-time.After(p.interval):
		}
		entries, err := p.table()
		if err != nil {
			return false, false, nil
		}
		entry, present := entries[host.String()]
		switch {
		case present && entry.complete():
			return true, true, nil
		case present && entry.flags == 0:
			// The kernel created the entry and failed to resolve it. That is a
			// definite "nothing answered", not a timeout.
			return false, true, nil
		}
		if time.Now().After(deadline) {
			// Nothing answered within the window. Reported as checked-and-free
			// rather than unknown: an address that does not resolve after a
			// direct probe is the strongest evidence available without root,
			// and treating it as unknown would refuse every free address and
			// make the driver unable to place anything.
			return false, true, nil
		}
	}
}

// sendNudge sends one datagram so the kernel resolves the neighbour.
//
// UDP to the discard port, one byte, no reply read, connection closed
// immediately. Nothing here waits for or interprets a response: the reply that
// matters is the ARP one, and the kernel handles that.
func sendNudge(ctx context.Context, address Address) error {
	host, err := address.Host()
	if err != nil {
		return err
	}
	dialer := net.Dialer{Timeout: time.Second}
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(host.String(), discardPort))
	if err != nil {
		return fmt.Errorf("probe %s: %w", address.InterfaceName, err)
	}
	defer conn.Close()
	_, err = conn.Write([]byte{0})
	return err
}

func readNeighbourFile(path string) (map[string]neighbour, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return parseNeighbours(file)
}

// parseNeighbours reads the /proc/net/arp table.
//
// Columns are: IP address, HW type, Flags, HW address, Mask, Device. A row that
// does not parse is skipped rather than failing the read: one malformed line
// must not make an otherwise usable table unreadable, because an unreadable
// table refuses every address.
func parseNeighbours(reader io.Reader) (map[string]neighbour, error) {
	entries := map[string]neighbour{}
	scanner := bufio.NewScanner(reader)
	// The header.
	if !scanner.Scan() {
		return entries, scanner.Err()
	}
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 {
			continue
		}
		flags, err := strconv.ParseInt(strings.TrimPrefix(fields[2], "0x"), 16, 32)
		if err != nil {
			continue
		}
		entries[fields[0]] = neighbour{flags: int(flags), hardware: fields[3], device: fields[5]}
	}
	return entries, scanner.Err()
}
