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

// Adapter is the only boundary through which typed privileged operations can
// reach Linux/runtime implementations. It deliberately exposes no command,
// executable, filesystem path, runtime socket, or raw ruleset argument.
type Adapter interface {
	Capabilities() map[privilegedv1.PrivilegedOperation]privilegedv1.CapabilityState
	EnsureAddress(context.Context, AddressOperation) (AdapterResult, error)
	ApplyNftablesPolicy(context.Context, NftablesOperation) (AdapterResult, error)
	ReconcileContainer(context.Context, ContainerOperation) (AdapterResult, error)
	EnsureNetworkNamespace(context.Context, NamespaceOperation) (AdapterResult, error)
}

// UnsupportedAdapter is the honest Phase 1 production implementation. Phase 2
// replaces individual typed operations without widening this interface.
type UnsupportedAdapter struct{}

func (UnsupportedAdapter) Capabilities() map[privilegedv1.PrivilegedOperation]privilegedv1.CapabilityState {
	return map[privilegedv1.PrivilegedOperation]privilegedv1.CapabilityState{
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_ADDRESS:             privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_NFTABLES_POLICY:     privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_CONTAINER_LIFECYCLE: privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_NETWORK_NAMESPACE:   privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
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
