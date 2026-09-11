package presence

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The real table format, as read from a Linux host during the P2-W1 spike.
const arpTable = `IP address       HW type     Flags       HW address            Mask     Device
172.17.0.1       0x1         0x2         de:52:7d:ed:a9:8d     *        eth0
172.17.99.234    0x1         0x0         00:00:00:00:00:00     *        eth0
10.20.0.7        0x1         0x2         00:00:00:00:00:00     *        eth0
malformed line
`

func TestNeighbourTableParsing(t *testing.T) {
	entries, err := parseNeighbours(strings.NewReader(arpTable))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("parsed %d entries, want 3", len(entries))
	}

	// A resolved neighbour: something replied and the kernel has its MAC.
	if got := entries["172.17.0.1"]; !got.complete() {
		t.Fatalf("gateway entry %+v should be complete", got)
	}
	// An entry the kernel created and never resolved.
	if got := entries["172.17.99.234"]; got.complete() {
		t.Fatalf("unresolved entry %+v claimed to be complete", got)
	}
	/*
	 * A complete flag with a zero hardware address is not evidence of a host.
	 * This is the one place where being wrong means either colliding with a
	 * production address or refusing a free one forever, so both halves are
	 * checked rather than trusting the flag alone.
	 */
	if got := entries["10.20.0.7"]; got.complete() {
		t.Fatalf("zero-MAC entry %+v claimed to be complete", got)
	}
}

// A malformed line must not make an otherwise usable table unreadable: an
// unreadable table refuses every address.
func TestAMalformedRowDoesNotPoisonTheTable(t *testing.T) {
	entries, err := parseNeighbours(strings.NewReader(
		"header\nnot enough fields\n172.17.0.1 0x1 0xZZ de:52:7d:ed:a9:8d * eth0\n" +
			"172.17.0.2       0x1         0x2         de:52:7d:ed:a9:8e     *        eth0\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries["172.17.0.2"].complete() {
		t.Fatalf("entries = %+v, want only the well-formed row", entries)
	}
}

func probeWith(table map[string]neighbour, tableErr error, nudge func(context.Context, Address) error) *NeighbourProbe {
	return &NeighbourProbe{
		table:    func() (map[string]neighbour, error) { return table, tableErr },
		nudge:    nudge,
		settle:   50 * time.Millisecond,
		interval: 5 * time.Millisecond,
	}
}

func noNudge(context.Context, Address) error { return nil }

/*
 * AC-ON-004: another host answers, so the address is in use.
 *
 * The already-resolved case short-circuits: the answer is in the table before
 * anything is sent, so nothing is sent.
 */
func TestAResolvedNeighbourIsReportedInUseWithoutSendingAnything(t *testing.T) {
	sent := 0
	probe := probeWith(
		map[string]neighbour{"10.20.0.40": {flags: 0x2, hardware: "de:52:7d:ed:a9:8d", device: "eth0"}},
		nil,
		func(context.Context, Address) error { sent++; return nil },
	)

	inUse, checked, err := probe.InUse(context.Background(), address("a", "10.20.0.40/32"))

	if err != nil || !inUse || !checked {
		t.Fatalf("inUse=%v checked=%v err=%v, want a checked conflict", inUse, checked, err)
	}
	if sent != 0 {
		t.Fatalf("sent %d datagrams for an address already in the table", sent)
	}
}

// The kernel created the entry and failed to resolve it: nothing answered.
func TestAnUnresolvedNeighbourIsReportedFree(t *testing.T) {
	probe := probeWith(
		map[string]neighbour{"10.20.0.40": {flags: 0x0, hardware: "00:00:00:00:00:00", device: "eth0"}},
		nil,
		noNudge,
	)

	inUse, checked, err := probe.InUse(context.Background(), address("a", "10.20.0.40/32"))

	if err != nil || inUse || !checked {
		t.Fatalf("inUse=%v checked=%v err=%v, want checked and free", inUse, checked, err)
	}
}

// Nothing in the table after the probe window is the same answer: free.
func TestNoEntryAfterTheWindowIsReportedFree(t *testing.T) {
	probe := probeWith(map[string]neighbour{}, nil, noNudge)

	inUse, checked, err := probe.InUse(context.Background(), address("a", "10.20.0.40/32"))

	if err != nil || inUse || !checked {
		t.Fatalf("inUse=%v checked=%v err=%v, want checked and free", inUse, checked, err)
	}
}

/*
 * The mechanism is the table. Without it nothing else can be interpreted, so
 * the probe reports that it did not check — and the reconciler refuses.
 */
func TestAnUnreadableTableReportsNotChecked(t *testing.T) {
	probe := probeWith(nil, errors.New("permission denied"), noNudge)

	inUse, checked, err := probe.InUse(context.Background(), address("a", "10.20.0.40/32"))

	if err != nil || inUse || checked {
		t.Fatalf("inUse=%v checked=%v err=%v, want not checked", inUse, checked, err)
	}
}

// A datagram that could not be sent is not evidence that the address is free.
func TestAFailedSendReportsNotChecked(t *testing.T) {
	probe := probeWith(map[string]neighbour{}, nil, func(context.Context, Address) error {
		return errors.New("network unreachable")
	})

	inUse, checked, err := probe.InUse(context.Background(), address("a", "10.20.0.40/32"))

	if err != nil || inUse || checked {
		t.Fatalf("inUse=%v checked=%v err=%v, want not checked", inUse, checked, err)
	}
}

// A malformed address never reaches the network.
func TestAnInvalidAddressIsNeverProbed(t *testing.T) {
	sent := 0
	probe := probeWith(map[string]neighbour{}, nil, func(context.Context, Address) error {
		sent++
		return nil
	})

	_, checked, err := probe.InUse(context.Background(), Address{
		DecoyID: "a", InterfaceName: "eth0", Prefix: "8.8.8.8/24",
	})

	if err == nil {
		t.Fatal("a public address was accepted for probing")
	}
	if checked || sent != 0 {
		t.Fatalf("checked=%v sent=%d; an invalid address must not be probed", checked, sent)
	}
}

/*
 * On a host that cannot read a neighbour cache the probe cannot answer, and
 * the reconciler therefore refuses every address. A developer machine must not
 * be able to claim one.
 */
func TestOnAnUnsupportedPlatformTheProbeRefusesToAnswer(t *testing.T) {
	probe := NewNeighbourProbe()
	if runtime.GOOS == "linux" {
		t.Skip("this host can read a neighbour cache")
	}

	_, checked, err := probe.InUse(context.Background(), address("a", "10.20.0.40/32"))

	if err != nil || checked {
		t.Fatalf("checked=%v err=%v, want an unchecked answer off Linux", checked, err)
	}
}

// End to end through the reconciler: an occupied address is refused and the
// host is never touched.
func TestTheProbeAndReconcilerRefuseAnOccupiedAddressTogether(t *testing.T) {
	driver := &fakeDriver{}
	probe := probeWith(
		map[string]neighbour{"10.20.0.40": {flags: 0x2, hardware: "de:52:7d:ed:a9:8d", device: "eth0"}},
		nil,
		noNudge,
	)
	reconciler, err := New(driver, WithConflictProbe(probe))
	if err != nil {
		t.Fatal(err)
	}

	report, err := reconciler.Reconcile(context.Background(), []Address{address("a", "10.20.0.40/32")})
	if err != nil {
		t.Fatal(err)
	}

	if got := statuses(report)["a"]; got != StatusRefusedConflict {
		t.Fatalf("status = %q, want %q", got, StatusRefusedConflict)
	}
	if len(driver.calls) != 0 {
		t.Fatalf("the host was touched for a refused address: %v", operations(driver))
	}
}
