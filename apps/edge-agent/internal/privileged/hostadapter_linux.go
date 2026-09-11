//go:build linux

package privileged

import (
	"context"
	"errors"
	"net"
	"net/netip"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
)

/*
hostAdapter is the production adapter on Linux.

It fills exactly one typed operation — `EnsureAddress`, which `P2-W1` needs —
and leaves the other three reporting `unsupported`. That is deliberate: the
nftables and containerd adapters are `P2-W2` and `P2-W3`, each is a separate
privileged surface with its own review, and a helper that claimed a capability
it does not have would be the exact failure this product is built not to make.

The address half is written so that only one thing can go wrong badly — Guardian
removing an address it did not add — and that is refused by the label check in
both directions.
*/
type hostAdapter struct {
	address  AdapterCapability
	nftables AdapterCapability
	// decoyRanges is the compiled allowlist. The egress policy is built from
	// it, which is what keeps the set of addresses Guardian may place and the
	// set it denies egress for from ever drifting apart.
	decoyRanges []netip.Prefix
	containers  AdapterCapability
	runtime     *containerRuntime
}

// NewHostAdapter builds the Linux adapter and settles, once, what it can
// actually do. Each capability is probed rather than assumed, so a helper
// deployed without CAP_NET_ADMIN, without netlink, or without decoy ranges
// reports `unsupported` instead of failing every call at the point of use.
func NewHostAdapter(allowlist Allowlist) Adapter {
	ranges := make([]netip.Prefix, len(allowlist.addressRanges))
	copy(ranges, allowlist.addressRanges)
	adapter := hostAdapter{
		address:     probeAddressCapability(),
		nftables:    probeNftablesCapability(ranges),
		decoyRanges: ranges,
		containers:  containerCapability(allowlist),
	}
	adapter.runtime = &containerRuntime{
		dial:              func() (decoyRuntimeAPI, error) { return dialContainerd(containerdSocketPath) },
		workloadDirectory: DefaultWorkloadDirectory,
		egressReady:       adapter.egressReady,
	}
	return adapter
}

// containerCapability does not probe containerd. Whether the runtime answers
// right now is GetRuntimeStatus's question and changes minute to minute; this
// one is whether the helper can manage decoy containers at all on this host.
func containerCapability(allowlist Allowlist) AdapterCapability {
	if len(allowlist.workloads) == 0 {
		return AdapterCapability{
			State:      privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
			ReasonCode: "no-workloads-allowlisted",
		}
	}
	return AdapterCapability{
		State:      privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE,
		ReasonCode: "containerd-runtime-adapter",
	}
}

// egressReady is the P2-W2 ordering, checked where decoys are started: the
// default-deny policy for exactly the configured ranges must be installed now.
// Anything short of a positive answer is "not ready".
func (a hostAdapter) egressReady() error {
	if a.nftables.State != privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE {
		return errors.New(a.nftables.ReasonCode)
	}
	connection, err := dialNetlinkProtocol(unix.NETLINK_NETFILTER)
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }()
	observed, err := observedEgressPolicy(connection)
	if err != nil {
		return err
	}
	if !buildEgressPolicy(a.decoyRanges).matches(observed) {
		return errors.New("egress-policy-not-applied")
	}
	return nil
}

func probeAddressCapability() AdapterCapability {
	if !holdsCapNetAdmin() {
		return AdapterCapability{
			State:      privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
			ReasonCode: "no-cap-net-admin",
		}
	}
	connection, err := dialNetlink()
	if err != nil {
		// The service profile restricts address families; a helper that cannot
		// open NETLINK_ROUTE says so rather than discovering it per request.
		return AdapterCapability{
			State:      privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
			ReasonCode: "netlink-unavailable",
		}
	}
	_ = connection.Close()
	return AdapterCapability{
		State:      privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE,
		ReasonCode: "netlink-address-adapter",
	}
}

/*
probeNftablesCapability establishes three things before claiming the egress
policy can be applied: the capability, a working nf_tables subsystem, and a
decoy range to write a policy about.

The third is not pedantry. A policy built from an empty range list installs
rules that match nothing, and reporting that as `applied` would tell an operator
their decoys are contained when nothing is containing them. `AC-SEC-003` is the
one control standing between medium-interaction decoy software and the
internet, so the failure to avoid is a false claim of enforcement.
*/
func probeNftablesCapability(ranges []netip.Prefix) AdapterCapability {
	unsupported := func(reason string) AdapterCapability {
		return AdapterCapability{
			State:      privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
			ReasonCode: reason,
		}
	}
	// Configuration before capability, because this answer does not depend on
	// where the helper is running and is the one an operator can act on.
	if len(ranges) == 0 {
		return unsupported("no-decoy-ranges-configured")
	}
	if !holdsCapNetAdmin() {
		return unsupported("no-cap-net-admin")
	}
	connection, err := dialNetlinkProtocol(unix.NETLINK_NETFILTER)
	if err != nil {
		return unsupported("netlink-unavailable")
	}
	defer func() { _ = connection.Close() }()
	// Ask the subsystem a real question. A kernel without nf_tables answers
	// this with something other than "no such table".
	if _, err := observedEgressPolicy(connection); err != nil {
		return unsupported("nftables-unavailable")
	}
	return AdapterCapability{
		State:      privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE,
		ReasonCode: "nftables-egress-adapter",
	}
}

func holdsCapNetAdmin() bool {
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capget(&header, &data[0]); err != nil {
		return false
	}
	const capability = uint(unix.CAP_NET_ADMIN)
	return data[capability>>5].Effective&(1<<(capability&31)) != 0
}

func (a hostAdapter) Capabilities() map[privilegedv1.PrivilegedOperation]AdapterCapability {
	return map[privilegedv1.PrivilegedOperation]AdapterCapability{
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_ADDRESS:             a.address,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_NFTABLES_POLICY:     a.nftables,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_CONTAINER_LIFECYCLE: a.containers,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_NETWORK_NAMESPACE:   notImplemented(),
	}
}

/*
EnsureAddress adds or removes one decoy address on one host interface.

The rules, in the order they are applied:

 1. An adapter that cannot do address work reports `unsupported` and touches
    nothing. `presence.Reconciler` maps that to a status that never claims the
    address is present.
 2. Guardian labels every address it adds. An address already on the interface
    without that label belongs to the host, and the request is refused in both
    directions — taking one over would mean deleting, later, an address Guardian
    never placed. This is the failure that would cause a customer outage, and it
    is the one refusal in this file that is not about tidiness.
 3. Adding an address that is already Guardian's, or removing one that is
    already gone, is `unchanged`. The helper is called repeatedly by a
    reconciler; converging twice must not report two changes.

The netlink calls are not context-aware — they are blocking syscalls bounded by
the socket's own receive timeout — so cancellation is observed on entry and the
timeout does the rest.
*/
func (a hostAdapter) EnsureAddress(ctx context.Context, operation AddressOperation) (AdapterResult, error) {
	if a.address.State != privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE {
		return AdapterResult{
			Outcome:    privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNSUPPORTED,
			ReasonCode: a.address.ReasonCode,
		}, nil
	}
	if err := ctx.Err(); err != nil {
		return AdapterResult{}, err
	}
	switch operation.DesiredState {
	case privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
		privilegedv1.PresenceState_PRESENCE_STATE_ABSENT:
	default:
		return AdapterResult{}, violation(codes.InvalidArgument, "invalid-presence-state")
	}
	prefix, err := netip.ParsePrefix(operation.AddressPrefix)
	if err != nil || !prefix.Addr().Is4() || prefix.Addr().Is4In6() {
		// IPv4 only: the ownership label the rules above depend on exists only
		// for IPv4 addresses, so an IPv6 decoy address would be unmarked.
		return AdapterResult{}, violation(codes.InvalidArgument, "unsupported-address-family")
	}
	label, ok := guardianLabel(operation.InterfaceName)
	if !ok {
		return AdapterResult{}, violation(codes.FailedPrecondition, "interface-name-too-long-to-label")
	}
	link, err := net.InterfaceByName(operation.InterfaceName)
	if err != nil {
		if operation.DesiredState == privilegedv1.PresenceState_PRESENCE_STATE_ABSENT {
			// No interface, no address on it. Removal has nothing to do.
			return AdapterResult{
				Outcome:    privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED,
				ReasonCode: "interface-absent",
			}, nil
		}
		return AdapterResult{}, violation(codes.FailedPrecondition, "interface-not-found")
	}

	connection, err := dialNetlink()
	if err != nil {
		return AdapterResult{}, err
	}
	defer func() { _ = connection.Close() }()

	existing, err := connection.findAddress(link.Index, prefix)
	if err != nil {
		return AdapterResult{}, err
	}
	if operation.DesiredState == privilegedv1.PresenceState_PRESENCE_STATE_ABSENT {
		return removeAddress(connection, link.Index, prefix, label, existing)
	}
	return placeAddress(connection, link.Index, prefix, label, existing)
}

func placeAddress(connection *netlinkConn, index int, prefix netip.Prefix, label string, existing *hostAddress) (AdapterResult, error) {
	if existing != nil {
		if existing.label != label {
			return AdapterResult{}, violation(codes.FailedPrecondition, "address-held-by-host")
		}
		return unchanged("address-already-present"), nil
	}
	err := connection.addAddress(index, prefix, label)
	switch {
	case err == nil:
		return applied("address-added"), nil
	case errors.Is(err, unix.EEXIST):
		// Something added the address between the dump and the write. Whose it
		// is decides whether this is convergence or a collision.
		raced, lookupErr := connection.findAddress(index, prefix)
		if lookupErr != nil {
			return AdapterResult{}, lookupErr
		}
		if raced == nil || raced.label != label {
			return AdapterResult{}, violation(codes.FailedPrecondition, "address-held-by-host")
		}
		return unchanged("address-already-present"), nil
	case errors.Is(err, unix.EPERM), errors.Is(err, unix.EACCES):
		return unsupportedOutcome("no-cap-net-admin"), nil
	}
	return AdapterResult{}, err
}

func removeAddress(connection *netlinkConn, index int, prefix netip.Prefix, label string, existing *hostAddress) (AdapterResult, error) {
	if existing == nil {
		return unchanged("address-already-absent"), nil
	}
	if existing.label != label {
		// The single most important refusal in this package.
		return AdapterResult{}, violation(codes.FailedPrecondition, "address-held-by-host")
	}
	err := connection.deleteAddress(index, prefix)
	switch {
	case err == nil:
		return applied("address-removed"), nil
	case errors.Is(err, unix.EADDRNOTAVAIL), errors.Is(err, unix.ENOENT):
		return unchanged("address-already-absent"), nil
	case errors.Is(err, unix.EPERM), errors.Is(err, unix.EACCES):
		return unsupportedOutcome("no-cap-net-admin"), nil
	}
	return AdapterResult{}, err
}

/*
ApplyNftablesPolicy installs `AC-SEC-003`: a decoy cannot open an outbound
connection.

What is installed does not depend on the request. The profile is the only one
the contract defines, and the addresses it covers come from the helper's
root-controlled startup arguments, so an RPC caller chooses *when* the policy is
applied and never *what* it says. The namespace argument names which allowlisted
Guardian namespace the operator is asserting the profile for; applying it for a
second namespace produces the same ruleset and reports no change.

The result is read back from the kernel before it is reported. A transaction the
kernel acknowledged is not the same fact as a ruleset that is present, and for a
containment control only the second one is worth reporting.
*/
func (a hostAdapter) ApplyNftablesPolicy(ctx context.Context, operation NftablesOperation) (AdapterResult, error) {
	if a.nftables.State != privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE {
		return unsupportedOutcome(a.nftables.ReasonCode), nil
	}
	if err := ctx.Err(); err != nil {
		return AdapterResult{}, err
	}
	if operation.Profile != privilegedv1.NftablesProfile_NFTABLES_PROFILE_DEFAULT_DENY_EGRESS {
		return AdapterResult{}, violation(codes.InvalidArgument, "invalid-nftables-profile")
	}
	if len(a.decoyRanges) == 0 {
		// Unreachable while the capability probe holds, and checked anyway: an
		// empty policy is the one outcome that must never be called applied.
		return unsupportedOutcome("no-decoy-ranges-configured"), nil
	}
	connection, err := dialNetlinkProtocol(unix.NETLINK_NETFILTER)
	if err != nil {
		return AdapterResult{}, err
	}
	defer func() { _ = connection.Close() }()

	ruleset := buildEgressPolicy(a.decoyRanges)
	observed, err := observedEgressPolicy(connection)
	if err != nil {
		return AdapterResult{}, err
	}
	if ruleset.matches(observed) {
		return unchanged("egress-policy-already-applied"), nil
	}
	if err := applyEgressPolicy(connection, ruleset); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			return unsupportedOutcome("no-cap-net-admin"), nil
		}
		return AdapterResult{}, err
	}
	confirmed, err := observedEgressPolicy(connection)
	if err != nil {
		return AdapterResult{}, err
	}
	if !ruleset.matches(confirmed) {
		return AdapterResult{}, violation(codes.Internal, "egress-policy-not-confirmed")
	}
	return applied("egress-policy-applied"), nil
}

// ReconcileContainer converges one decoy container; see container_runtime.go
// for the rules.
func (a hostAdapter) ReconcileContainer(ctx context.Context, operation ContainerOperation) (AdapterResult, error) {
	if a.containers.State != privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE || a.runtime == nil {
		if a.containers.ReasonCode == "" {
			return unsupportedResult(), nil
		}
		return unsupportedOutcome(a.containers.ReasonCode), nil
	}
	return a.runtime.reconcile(ctx, operation)
}

func (hostAdapter) EnsureNetworkNamespace(context.Context, NamespaceOperation) (AdapterResult, error) {
	return unsupportedResult(), nil
}

func applied(reason string) AdapterResult {
	return AdapterResult{
		Outcome:    privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED,
		ReasonCode: reason,
	}
}

func unchanged(reason string) AdapterResult {
	return AdapterResult{
		Outcome:    privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED,
		ReasonCode: reason,
	}
}

func unsupportedOutcome(reason string) AdapterResult {
	return AdapterResult{
		Outcome:    privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNSUPPORTED,
		ReasonCode: reason,
	}
}

var _ Adapter = hostAdapter{}
