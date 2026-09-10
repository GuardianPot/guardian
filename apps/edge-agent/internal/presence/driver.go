package presence

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
)

/*
The privileged helper, as a Driver.

This is the translation between what this package decides and what the helper
can be asked. It runs unprivileged, in the agent, and holds nothing but a
client to a Unix socket — the privilege is entirely on the other side of that
socket, which is what the seam in presence.go exists to keep true.

The only interesting decisions here are what each helper answer means, and they
all resolve the same way: an answer that does not positively say the address is
where Guardian asked for it is never turned into presence.
*/

// HelperClient is as much of the privileged helper as placing an address
// needs. It is an interface so the mapping below can be tested without a
// helper, and so this package depends on the helper's contract rather than on
// its client implementation.
type HelperClient interface {
	// Available reports whether the helper is reachable now. A helper that is
	// not is an absent capability, not a failed operation.
	Available() bool
	EnsureAddress(context.Context, *privilegedv1.EnsureAddressRequest) (*privilegedv1.EnsureAddressResponse, error)
}

// ErrHelperMismatch is a helper answering with a request ID that is not the one
// asked about. It says nothing about the address in hand.
var ErrHelperMismatch = errors.New("privileged helper answered a different request")

type helperDriver struct{ client HelperClient }

// NewHelperDriver builds the production Driver. The agent supplies its
// privileged-helper client; nothing else in this package knows the helper
// exists.
func NewHelperDriver(client HelperClient) (Driver, error) {
	if client == nil {
		return nil, ErrNoDriver
	}
	return helperDriver{client: client}, nil
}

func (d helperDriver) Present(ctx context.Context, address Address) (Outcome, error) {
	return d.ensure(ctx, address, privilegedv1.PresenceState_PRESENCE_STATE_PRESENT)
}

func (d helperDriver) Absent(ctx context.Context, address Address) (Outcome, error) {
	return d.ensure(ctx, address, privilegedv1.PresenceState_PRESENCE_STATE_ABSENT)
}

func (d helperDriver) ensure(ctx context.Context, address Address, state privilegedv1.PresenceState) (Outcome, error) {
	if err := address.valid(); err != nil {
		return OutcomeFailed, err
	}
	if !d.client.Available() {
		// A host with no helper has no address capability. Reporting a failure
		// instead would make a reconciler retry forever against something that
		// was never going to answer, and would read as a broken deployment
		// rather than an absent one.
		return OutcomeUnsupported, nil
	}
	requestID, err := newRequestID()
	if err != nil {
		return OutcomeUnknown, fmt.Errorf("mint privileged request id: %w", err)
	}
	/*
	 * A fresh request ID every call, deliberately.
	 *
	 * The helper caches results per request ID for fifteen minutes, so an ID
	 * derived from the address would let a later call be answered from that
	 * cache. An address can be placed, released, and wanted again inside that
	 * window, and a cached "applied" would then be a claim about the host's
	 * past rather than its present. Replaying is safe instead: the operation
	 * converges by construction — add if absent, remove if present — so every
	 * call re-reads the interface and answers about what is there now.
	 */
	response, err := d.client.EnsureAddress(ctx, &privilegedv1.EnsureAddressRequest{
		RequestId:     requestID,
		InterfaceName: address.InterfaceName,
		AddressPrefix: address.Prefix,
		DesiredState:  state,
	})
	if err != nil {
		return OutcomeFailed, err
	}
	result := response.GetResult()
	if result.GetRequestId() != requestID {
		return OutcomeUnknown, ErrHelperMismatch
	}
	switch result.GetOutcome() {
	case privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED,
		privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED:
		// Both mean the host is now in the state that was asked for. The
		// difference between them is whether anything had to change, which is
		// the helper's business and not this package's.
		return OutcomeApplied, nil
	case privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNSUPPORTED:
		return OutcomeUnsupported, nil
	default:
		// Including the unspecified outcome. A helper that will not say what it
		// did has not said the address is there.
		return OutcomeUnknown, nil
	}
}

// newRequestID produces an opaque identifier matching the helper's request-ID
// rule (`^[a-z0-9][a-z0-9-]{7,63}$`). It carries no meaning: the helper uses it
// to correlate and to detect a reused ID carrying a different operation.
func newRequestID() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "gdn-" + hex.EncodeToString(raw), nil
}
