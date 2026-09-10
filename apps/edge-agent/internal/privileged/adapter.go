package privileged

import (
	"context"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
)

const unsupportedReason = "phase-2-adapter-not-implemented"

type AddressOperation struct {
	InterfaceName string
	AddressPrefix string
	DesiredState  privilegedv1.PresenceState
}

type NftablesOperation struct {
	NamespaceName string
	Profile       privilegedv1.NftablesProfile
}

type ContainerOperation struct {
	WorkloadID   string
	DesiredState privilegedv1.ContainerState
}

type NamespaceOperation struct {
	NamespaceName string
	DesiredState  privilegedv1.PresenceState
}

type AdapterResult struct {
	Outcome    privilegedv1.OperationOutcome
	ReasonCode string
}

// AdapterCapability is what an adapter can do for one operation, and why.
//
// The reason travels with the state because "unsupported" has more than one
// cause and they are not interchangeable to whoever is reading the status: a
// capability nobody has written yet, a helper running without CAP_NET_ADMIN,
// and a service profile that forbids netlink are three different problems with
// three different fixes.
type AdapterCapability struct {
	State      privilegedv1.CapabilityState
	ReasonCode string
}

// Adapter is the only boundary through which typed privileged operations can
// reach Linux/runtime implementations. It deliberately exposes no command,
// executable, filesystem path, runtime socket, or raw ruleset argument.
type Adapter interface {
	Capabilities() map[privilegedv1.PrivilegedOperation]AdapterCapability
	EnsureAddress(context.Context, AddressOperation) (AdapterResult, error)
	ApplyNftablesPolicy(context.Context, NftablesOperation) (AdapterResult, error)
	ReconcileContainer(context.Context, ContainerOperation) (AdapterResult, error)
	EnsureNetworkNamespace(context.Context, NamespaceOperation) (AdapterResult, error)
}

// UnsupportedAdapter is the honest implementation for a host Guardian cannot
// change: it does nothing and says so. Phase 2 replaces individual typed
// operations without widening this interface.
type UnsupportedAdapter struct{}

func (UnsupportedAdapter) Capabilities() map[privilegedv1.PrivilegedOperation]AdapterCapability {
	return map[privilegedv1.PrivilegedOperation]AdapterCapability{
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_ADDRESS:             notImplemented(),
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_NFTABLES_POLICY:     notImplemented(),
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_CONTAINER_LIFECYCLE: notImplemented(),
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_NETWORK_NAMESPACE:   notImplemented(),
	}
}

func notImplemented() AdapterCapability {
	return AdapterCapability{
		State:      privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
		ReasonCode: unsupportedReason,
	}
}

func (UnsupportedAdapter) EnsureAddress(context.Context, AddressOperation) (AdapterResult, error) {
	return unsupportedResult(), nil
}

func (UnsupportedAdapter) ApplyNftablesPolicy(context.Context, NftablesOperation) (AdapterResult, error) {
	return unsupportedResult(), nil
}

func (UnsupportedAdapter) ReconcileContainer(context.Context, ContainerOperation) (AdapterResult, error) {
	return unsupportedResult(), nil
}

func (UnsupportedAdapter) EnsureNetworkNamespace(context.Context, NamespaceOperation) (AdapterResult, error) {
	return unsupportedResult(), nil
}

func unsupportedResult() AdapterResult {
	return AdapterResult{
		Outcome:    privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNSUPPORTED,
		ReasonCode: unsupportedReason,
	}
}
