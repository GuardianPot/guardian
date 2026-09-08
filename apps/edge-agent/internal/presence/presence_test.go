package presence

import (
	"context"
	"errors"
	"testing"
)

type call struct {
	operation string
	address   Address
}

type fakeDriver struct {
	calls   []call
	present func(Address) (Outcome, error)
	absent  func(Address) (Outcome, error)
}

func (d *fakeDriver) Present(_ context.Context, address Address) (Outcome, error) {
	d.calls = append(d.calls, call{"present", address})
	if d.present != nil {
		return d.present(address)
	}
	return OutcomeApplied, nil
}

func (d *fakeDriver) Absent(_ context.Context, address Address) (Outcome, error) {
	d.calls = append(d.calls, call{"absent", address})
	if d.absent != nil {
		return d.absent(address)
	}
	return OutcomeApplied, nil
}

// freeProbe checked the segment and found nothing answering.
type freeProbe struct{}

func (freeProbe) InUse(context.Context, Address) (bool, bool, error) { return false, true, nil }

// occupiedProbe found another host on the address.
type occupiedProbe struct{}

func (occupiedProbe) InUse(context.Context, Address) (bool, bool, error) { return true, true, nil }

type memoryReservations struct {
	stored []Address
	saves  int
}

func (m *memoryReservations) Load(context.Context) ([]Address, error) { return m.stored, nil }

func (m *memoryReservations) Save(_ context.Context, addresses []Address) error {
	m.stored = addresses
	m.saves++
	return nil
}

func address(id, prefix string) Address {
	return Address{DecoyID: id, InterfaceName: "eth0", Prefix: prefix}
}

func statuses(report Report) map[string]Status {
	out := make(map[string]Status, len(report.Results))
	for _, result := range report.Results {
		out[result.Address.DecoyID] = result.Status
	}
	return out
}

func operations(driver *fakeDriver) []string {
	out := make([]string, 0, len(driver.calls))
	for _, made := range driver.calls {
		out = append(out, made.operation+" "+made.address.Prefix)
	}
	return out
}

/*
 * AC-ON-004. The address answers as another host, so Guardian does not take it
 * and does not touch the host that has it.
 */
func TestAddressClaimedByAnotherHostIsRefused(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := New(driver, WithConflictProbe(occupiedProbe{}))
	if err != nil {
		t.Fatal(err)
	}

	report, err := reconciler.Reconcile(context.Background(), []Address{address("a", "10.20.0.40/24")})
	if err != nil {
		t.Fatal(err)
	}

	if got := statuses(report)["a"]; got != StatusRefusedConflict {
		t.Fatalf("status = %q, want %q", got, StatusRefusedConflict)
	}
	// The existing host is unaffected: nothing was applied and nothing removed.
	if len(driver.calls) != 0 {
		t.Fatalf("a refused address still reached the host: %v", operations(driver))
	}
	if len(reconciler.Held()) != 0 {
		t.Fatal("a refused address was recorded as held")
	}
}

/*
 * The same refusal when the probe cannot answer.
 *
 * "I could not check" is not "it is free". Taking an address that turns out to
 * belong to a production host is how a deception product causes the outage it
 * was bought to prevent, so the default is to refuse.
 */
func TestAddressIsRefusedWhenTheConflictProbeCannotAnswer(t *testing.T) {
	driver := &fakeDriver{}
	// No WithConflictProbe: the default probe cannot check.
	reconciler, err := New(driver)
	if err != nil {
		t.Fatal(err)
	}

	report, err := reconciler.Reconcile(context.Background(), []Address{address("a", "10.20.0.40/24")})
	if err != nil {
		t.Fatal(err)
	}

	if got := statuses(report)["a"]; got != StatusRefusedUnverified {
		t.Fatalf("status = %q, want %q", got, StatusRefusedUnverified)
	}
	if len(driver.calls) != 0 {
		t.Fatalf("an unverified address reached the host: %v", operations(driver))
	}
}

// A checked, free address is applied. Without this the refusals above would be
// satisfied by a reconciler that refuses everything.
func TestAFreeAddressIsApplied(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := New(driver, WithConflictProbe(freeProbe{}))
	if err != nil {
		t.Fatal(err)
	}

	report, err := reconciler.Reconcile(context.Background(), []Address{address("a", "10.20.0.40/24")})
	if err != nil {
		t.Fatal(err)
	}

	if got := statuses(report)["a"]; got != StatusPresent {
		t.Fatalf("status = %q, want %q", got, StatusPresent)
	}
	if len(report.Applied()) != 1 {
		t.Fatalf("applied = %v", report.Applied())
	}
}

/*
 * P2-W1 acceptance: a failed deploy does not leave an orphan IP.
 *
 * The apply may have partially succeeded, so the release is issued in the same
 * pass rather than left for the next one. The window in which an orphan exists
 * is exactly the window this closes.
 */
func TestAFailedApplyLeavesNoOrphanAddress(t *testing.T) {
	driver := &fakeDriver{present: func(Address) (Outcome, error) {
		return OutcomeFailed, errors.New("netlink refused")
	}}
	reconciler, err := New(driver, WithConflictProbe(freeProbe{}))
	if err != nil {
		t.Fatal(err)
	}

	report, err := reconciler.Reconcile(context.Background(), []Address{address("a", "10.20.0.40/24")})
	if err != nil {
		t.Fatal(err)
	}

	if got := statuses(report)["a"]; got != StatusFailed {
		t.Fatalf("status = %q, want %q", got, StatusFailed)
	}
	if got := operations(driver); len(got) != 2 || got[1] != "absent 10.20.0.40/24" {
		t.Fatalf("a failed apply did not release the address: %v", got)
	}
	if len(reconciler.Held()) != 0 {
		t.Fatal("a failed address is being held")
	}
}

// A driver that will not confirm it applied the address has not applied it.
// Anything else would let an unknown become a claim of presence.
func TestAnUnconfirmedApplyIsNotPresence(t *testing.T) {
	driver := &fakeDriver{present: func(Address) (Outcome, error) { return OutcomeUnknown, nil }}
	reconciler, err := New(driver, WithConflictProbe(freeProbe{}))
	if err != nil {
		t.Fatal(err)
	}

	report, _ := reconciler.Reconcile(context.Background(), []Address{address("a", "10.20.0.40/24")})

	if got := statuses(report)["a"]; got != StatusFailed {
		t.Fatalf("status = %q, want %q", got, StatusFailed)
	}
	if len(report.Applied()) != 0 {
		t.Fatal("an unconfirmed apply was reported as applied")
	}
}

// A host with no privileged helper is unsupported, not failed: nothing went
// wrong, the capability is absent. It is still never reported as present.
func TestNoPrivilegedHelperIsUnsupportedRatherThanPresent(t *testing.T) {
	driver := &fakeDriver{present: func(Address) (Outcome, error) { return OutcomeUnsupported, nil }}
	reconciler, err := New(driver, WithConflictProbe(freeProbe{}))
	if err != nil {
		t.Fatal(err)
	}

	report, _ := reconciler.Reconcile(context.Background(), []Address{address("a", "10.20.0.40/24")})

	if got := statuses(report)["a"]; got != StatusUnsupported {
		t.Fatalf("status = %q, want %q", got, StatusUnsupported)
	}
	if len(report.Applied()) != 0 {
		t.Fatal("an unsupported host was reported as holding the address")
	}
}

// An address no longer desired is released.
func TestAnUndesiredAddressIsReleased(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := New(driver, WithConflictProbe(freeProbe{}))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := reconciler.Reconcile(ctx, []Address{address("a", "10.20.0.40/24")}); err != nil {
		t.Fatal(err)
	}

	report, err := reconciler.Reconcile(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	if got := statuses(report)["a"]; got != StatusRemoved {
		t.Fatalf("status = %q, want %q", got, StatusRemoved)
	}
	if len(reconciler.Held()) != 0 {
		t.Fatal("a released address is still held")
	}
}

/*
 * An address moving between decoys is released before it is taken.
 *
 * The Control Plane allows an address to move once the first decoy stops using
 * it. Applying the new holder first would put a duplicate on the wire.
 */
func TestAnAddressMovingBetweenDecoysIsReleasedFirst(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := New(driver, WithConflictProbe(freeProbe{}))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := reconciler.Reconcile(ctx, []Address{address("a", "10.20.0.40/24")}); err != nil {
		t.Fatal(err)
	}
	driver.calls = nil

	if _, err := reconciler.Reconcile(ctx, []Address{address("b", "10.20.0.40/24")}); err != nil {
		t.Fatal(err)
	}

	got := operations(driver)
	if len(got) != 2 || got[0] != "absent 10.20.0.40/24" || got[1] != "present 10.20.0.40/24" {
		t.Fatalf("operations = %v, want the release before the apply", got)
	}
}

/*
 * P2-W1 acceptance: reboot and reconcile restore the intended addresses.
 *
 * A restart is a new process with an empty memory and a host that still has
 * every address the old one applied. Reservations are what let the new process
 * know what it is holding — without them an address the Control Plane stopped
 * asking for would stay on the interface until the host went down.
 */
func TestAfterARestartTheReconcilerStillKnowsWhatItHolds(t *testing.T) {
	ctx := context.Background()
	reservations := &memoryReservations{}
	first, err := New(&fakeDriver{}, WithConflictProbe(freeProbe{}), WithReservations(reservations))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Reconcile(ctx, []Address{address("a", "10.20.0.40/24")}); err != nil {
		t.Fatal(err)
	}

	// A new process, with the reservations the old one left behind.
	driver := &fakeDriver{}
	second, err := New(driver, WithConflictProbe(freeProbe{}), WithReservations(reservations))
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	if len(second.Held()) != 1 {
		t.Fatalf("a restarted reconciler forgot what it holds: %v", second.Held())
	}

	// The Control Plane no longer wants it, and the new process can release it.
	report, err := second.Reconcile(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := statuses(report)["a"]; got != StatusRemoved {
		t.Fatalf("status = %q, want %q", got, StatusRemoved)
	}
	if got := operations(driver); len(got) != 1 || got[0] != "absent 10.20.0.40/24" {
		t.Fatalf("operations = %v, want a single release", got)
	}
}

// Re-applying an address already held issues no host operation. A reconciler
// that reapplied every pass would churn the interface for no reason.
func TestReconcilingAnUnchangedSetTouchesNothing(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := New(driver, WithConflictProbe(freeProbe{}))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	desired := []Address{address("a", "10.20.0.40/24")}
	if _, err := reconciler.Reconcile(ctx, desired); err != nil {
		t.Fatal(err)
	}
	driver.calls = nil

	report, err := reconciler.Reconcile(ctx, desired)
	if err != nil {
		t.Fatal(err)
	}

	if len(driver.calls) != 0 {
		t.Fatalf("an unchanged set touched the host: %v", operations(driver))
	}
	if got := statuses(report)["a"]; got != StatusPresent {
		t.Fatalf("status = %q, want %q", got, StatusPresent)
	}
}

// Shutdown releases everything, so a stopped agent leaves no decoy answering.
func TestReleaseAllRemovesEveryHeldAddress(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := New(driver, WithConflictProbe(freeProbe{}))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := reconciler.Reconcile(ctx, []Address{
		address("a", "10.20.0.40/24"),
		address("b", "10.20.0.41/24"),
	}); err != nil {
		t.Fatal(err)
	}

	reconciler.ReleaseAll(ctx)

	if len(reconciler.Held()) != 0 {
		t.Fatalf("addresses survived shutdown: %v", reconciler.Held())
	}
}

// A malformed or public address never reaches the host.
func TestAnInvalidAddressIsRefusedBeforeItReachesTheHost(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := New(driver, WithConflictProbe(freeProbe{}))
	if err != nil {
		t.Fatal(err)
	}

	report, err := reconciler.Reconcile(context.Background(), []Address{
		{DecoyID: "a", InterfaceName: "eth0", Prefix: "not-an-address"},
		{DecoyID: "b", InterfaceName: "eth0", Prefix: "8.8.8.8/24"},
		{DecoyID: "c", InterfaceName: "", Prefix: "10.20.0.40/24"},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"a", "b", "c"} {
		if got := statuses(report)[id]; got != StatusFailed {
			t.Fatalf("%s status = %q, want %q", id, got, StatusFailed)
		}
	}
	if len(driver.calls) != 0 {
		t.Fatalf("an invalid address reached the host: %v", operations(driver))
	}
}
