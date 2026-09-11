// Package presence owns the routed decoy address: which addresses this Edge
// has been asked to present, whether each one may safely be taken, and what is
// actually on the interface.
//
// It holds no privilege. Every operation that touches the host goes through the
// Driver seam, which the agent backs with the privileged helper client; this
// package decides *what* should happen and never *does* it. That split is what
// lets the reconciliation rules below be tested without root.
package presence

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"sync"
)

// Outcome is what the host reported about one address operation.
type Outcome int

const (
	// OutcomeUnknown is the honest default. A driver that did not act, or
	// cannot say whether it acted, returns this and never Applied.
	OutcomeUnknown Outcome = iota
	OutcomeApplied
	OutcomeFailed
	// OutcomeUnsupported is what a host without the privileged helper returns.
	// It is distinct from Failed: nothing went wrong, the capability is absent.
	OutcomeUnsupported
)

// Driver is the privileged seam. `P2-W1`'s netlink implementation fills it; the
// agent supplies one backed by the privileged helper, and tests supply a fake.
//
// Present and Absent are separate calls rather than one with a flag, because
// the two have different failure meanings: failing to add an address leaves
// nothing behind, and failing to remove one leaves an address on a live
// interface that Guardian believes it has released.
type Driver interface {
	Present(ctx context.Context, address Address) (Outcome, error)
	Absent(ctx context.Context, address Address) (Outcome, error)
}

// ConflictProbe answers AC-ON-004: is this address already claimed by some
// other host on the segment?
//
// It is a seam because answering it truthfully needs a privileged probe this
// repository does not yet have. The default implementation says it does not
// know, and the reconciler treats "does not know" as a refusal — see
// Reconciler.Reconcile.
type ConflictProbe interface {
	// InUse reports whether the address answers as another host. The second
	// return distinguishes "checked, and it is free" from "could not check".
	InUse(ctx context.Context, address Address) (inUse bool, checked bool, err error)
}

// Reservations is what this Edge believes it has taken.
//
// Persisted, because an orphan address outlives the process that created it. A
// reconciler that kept this in memory would forget, on restart, every address
// it had applied, and would then be unable to remove one the Control Plane had
// stopped asking for — the address would stay on the interface for as long as
// the host was up.
type Reservations interface {
	Load(ctx context.Context) ([]Address, error)
	Save(ctx context.Context, addresses []Address) error
}

// Address is one decoy's presence on one interface.
type Address struct {
	DecoyID       string
	InterfaceName string
	// Prefix is the decoy's address as a /32 identity, such as 10.20.0.40/32.
	// ADR 0016 binds decoy addresses this way on purpose: a zone-length prefix
	// would give the host a connected route for the whole subnet, and on an
	// interface that does not already carry that subnet Guardian would be
	// claiming routing for addresses it does not own.
	Prefix string
}

// Host returns the address without its prefix length. Anything other than a
// /32 is refused, for the reason on Prefix.
func (a Address) Host() (netip.Addr, error) {
	prefix, err := netip.ParsePrefix(a.Prefix)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%w: %q", ErrInvalidAddress, a.Prefix)
	}
	if prefix.Bits() != 32 {
		return netip.Addr{}, fmt.Errorf("%w: a decoy address is a /32 identity", ErrInvalidAddress)
	}
	if !prefix.Addr().Is4() || !prefix.Addr().IsPrivate() {
		return netip.Addr{}, fmt.Errorf("%w: not a private IPv4 host address", ErrInvalidAddress)
	}
	return prefix.Addr(), nil
}

func (a Address) valid() error {
	if a.DecoyID == "" || a.InterfaceName == "" {
		return fmt.Errorf("%w: decoy and interface are required", ErrInvalidAddress)
	}
	_, err := a.Host()
	return err
}

var (
	ErrInvalidAddress = errors.New("decoy address is invalid")
	ErrNoDriver       = errors.New("presence driver is required")
)

// Status is what happened to one address in one reconcile pass.
type Status string

const (
	// StatusPresent is the only value that claims the address is on the
	// interface, and a driver has to have said so.
	StatusPresent Status = "present"
	StatusRemoved Status = "removed"
	// StatusRefusedConflict is AC-ON-004: another host answers on this address,
	// so Guardian did not take it. The existing host is untouched.
	StatusRefusedConflict Status = "refused_conflict"
	// StatusRefusedUnverified is the same refusal for a different reason: the
	// conflict probe could not answer, so Guardian will not claim the address.
	StatusRefusedUnverified Status = "refused_unverified"
	StatusFailed            Status = "failed"
	// StatusUnsupported is a host with no privileged helper. Nothing was
	// applied and nothing is claimed.
	StatusUnsupported Status = "unsupported"
)

// Result is one address's outcome, with the reason it ended that way.
type Result struct {
	Address Address
	Status  Status
	// Reason is a closed machine token, never a host error string: a netlink
	// message can name an interface or an address that is not Guardian's to
	// disclose.
	Reason string
}

// Report is one reconcile pass.
type Report struct {
	Results []Result
}

// Applied returns the addresses this Edge now believes it holds.
func (r Report) Applied() []Address {
	applied := make([]Address, 0, len(r.Results))
	for _, result := range r.Results {
		if result.Status == StatusPresent {
			applied = append(applied, result.Address)
		}
	}
	return applied
}

// unavailableProbe is the default. It cannot check, and says so.
type unavailableProbe struct{}

func (unavailableProbe) InUse(context.Context, Address) (bool, bool, error) {
	return false, false, nil
}

// Reconciler converges the host's decoy addresses on the desired set.
type Reconciler struct {
	driver       Driver
	probe        ConflictProbe
	reservations Reservations

	mu   sync.Mutex
	held map[string]Address
}

type Option func(*Reconciler)

// WithConflictProbe supplies the AC-ON-004 check. Without it the reconciler
// refuses to take any address, which is the fail-safe direction.
func WithConflictProbe(probe ConflictProbe) Option {
	return func(r *Reconciler) {
		if probe != nil {
			r.probe = probe
		}
	}
}

// WithReservations supplies persistence. Without it the reconciler still
// converges, but cannot remove an address it applied before a restart.
func WithReservations(reservations Reservations) Option {
	return func(r *Reconciler) {
		if reservations != nil {
			r.reservations = reservations
		}
	}
}

func New(driver Driver, options ...Option) (*Reconciler, error) {
	if driver == nil {
		return nil, ErrNoDriver
	}
	reconciler := &Reconciler{driver: driver, probe: unavailableProbe{}, held: map[string]Address{}}
	for _, option := range options {
		if option != nil {
			option(reconciler)
		}
	}
	return reconciler, nil
}

// Restore reloads what a previous process claimed, so this one can release it.
//
// Called before the first Reconcile. Without it an address applied before a
// restart is invisible to this process and stays on the interface forever.
func (r *Reconciler) Restore(ctx context.Context) error {
	if r.reservations == nil {
		return nil
	}
	stored, err := r.reservations.Load(ctx)
	if err != nil {
		return fmt.Errorf("load decoy address reservations: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, address := range stored {
		if address.valid() == nil {
			r.held[address.DecoyID] = address
		}
	}
	return nil
}

/*
Reconcile converges the interface on `desired` and reports what happened.

Three rules, and the order matters.

Removals run first. An address being released may be one another decoy is about
to take — the Control Plane allows an address to move between decoys — and
applying the new one first would be a duplicate on the wire.

An address is taken only after the conflict probe says it is free. AC-ON-004
requires a deployment onto an address another host already answers on to be
refused fail-safe, with the existing host unaffected, and "the probe could not
tell me" is not "it is free". Guardian refuses in both cases: taking an address
that turns out to belong to a production host is how a deception product causes
the outage it was bought to prevent.

A failed apply releases immediately. The acceptance criterion is that a failed
deploy leaves no orphan IP, and the window where one exists is between a
partial apply and the next reconcile — so the release happens here, not later.
*/
func (r *Reconciler) Reconcile(ctx context.Context, desired []Address) (Report, error) {
	wanted := make(map[string]Address, len(desired))
	report := Report{}
	for _, address := range desired {
		if err := address.valid(); err != nil {
			report.Results = append(report.Results, Result{
				Address: address, Status: StatusFailed, Reason: "invalid_address",
			})
			continue
		}
		wanted[address.DecoyID] = address
	}

	r.mu.Lock()
	held := make(map[string]Address, len(r.held))
	for id, address := range r.held {
		held[id] = address
	}
	r.mu.Unlock()

	// Removals first: an address leaving one decoy may be arriving at another.
	for _, id := range sortedKeys(held) {
		current := held[id]
		next, stillWanted := wanted[id]
		if stillWanted && next.Prefix == current.Prefix && next.InterfaceName == current.InterfaceName {
			continue
		}
		outcome, err := r.driver.Absent(ctx, current)
		result := Result{Address: current, Status: StatusRemoved, Reason: "released"}
		switch {
		case err != nil || outcome == OutcomeFailed:
			// Still forgotten locally. Holding a reservation for an address the
			// host would not release would make every later pass retry a
			// removal that already failed, and would block the address from
			// being retried by whoever is meant to have it.
			result.Status, result.Reason = StatusFailed, "release_failed"
		case outcome == OutcomeUnsupported:
			result.Status, result.Reason = StatusUnsupported, "no_privileged_helper"
		}
		delete(held, id)
		report.Results = append(report.Results, result)
	}

	for _, id := range sortedKeys(wanted) {
		address := wanted[id]
		if existing, ok := held[id]; ok && existing == address {
			report.Results = append(report.Results, Result{
				Address: address, Status: StatusPresent, Reason: "already_present",
			})
			continue
		}
		result := r.take(ctx, address)
		if result.Status == StatusPresent {
			held[id] = address
		}
		report.Results = append(report.Results, result)
	}

	r.mu.Lock()
	r.held = held
	r.mu.Unlock()
	if err := r.persist(ctx, held); err != nil {
		return report, err
	}
	return report, nil
}

// take applies one address, after establishing that it is free to take.
func (r *Reconciler) take(ctx context.Context, address Address) Result {
	inUse, checked, err := r.probe.InUse(ctx, address)
	switch {
	case err != nil:
		return Result{Address: address, Status: StatusRefusedUnverified, Reason: "probe_failed"}
	case !checked:
		// The fail-safe direction. Guardian does not know the address is free,
		// so it does not take it.
		return Result{Address: address, Status: StatusRefusedUnverified, Reason: "probe_unavailable"}
	case inUse:
		// AC-ON-004. The other host keeps the address; nothing is applied.
		return Result{Address: address, Status: StatusRefusedConflict, Reason: "address_in_use"}
	}

	outcome, err := r.driver.Present(ctx, address)
	switch {
	case err != nil || outcome == OutcomeFailed:
		// No orphan: the apply may have partially succeeded, so the release is
		// issued now rather than left for a later pass.
		_, _ = r.driver.Absent(ctx, address)
		return Result{Address: address, Status: StatusFailed, Reason: "apply_failed"}
	case outcome == OutcomeUnsupported:
		return Result{Address: address, Status: StatusUnsupported, Reason: "no_privileged_helper"}
	case outcome != OutcomeApplied:
		// A driver that will not say it applied the address has not applied it
		// as far as this package is concerned.
		_, _ = r.driver.Absent(ctx, address)
		return Result{Address: address, Status: StatusFailed, Reason: "apply_unconfirmed"}
	}
	return Result{Address: address, Status: StatusPresent, Reason: "applied"}
}

// ReleaseAll removes every address this Edge holds. Used on shutdown, so a
// stopped agent does not leave decoy addresses answering on the network.
func (r *Reconciler) ReleaseAll(ctx context.Context) Report {
	report, _ := r.Reconcile(ctx, nil)
	return report
}

// Held reports what this Edge believes it holds, for the observed-state path.
func (r *Reconciler) Held() []Address {
	r.mu.Lock()
	defer r.mu.Unlock()
	held := make([]Address, 0, len(r.held))
	for _, id := range sortedKeys(r.held) {
		held = append(held, r.held[id])
	}
	return held
}

func (r *Reconciler) persist(ctx context.Context, held map[string]Address) error {
	if r.reservations == nil {
		return nil
	}
	addresses := make([]Address, 0, len(held))
	for _, id := range sortedKeys(held) {
		addresses = append(addresses, held[id])
	}
	if err := r.reservations.Save(ctx, addresses); err != nil {
		return fmt.Errorf("save decoy address reservations: %w", err)
	}
	return nil
}

// sortedKeys keeps a pass deterministic. Two runs over the same desired set
// must issue the same operations in the same order, or a failure is not
// reproducible.
func sortedKeys(addresses map[string]Address) []string {
	keys := make([]string, 0, len(addresses))
	for key := range addresses {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
