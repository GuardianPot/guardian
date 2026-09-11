package presence

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/privclient"
	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
)

// The production client satisfies the seam with no adapter in between. Asserted
// in a test rather than in the package so `presence` keeps no dependency on the
// privileged client; if the client's signature drifts, this stops compiling
// instead of the agent failing to wire itself together at run time.
var _ HelperClient = (*privclient.Client)(nil)

// The helper's own rule, from `privileged.requestIDPattern`. It is restated
// here rather than imported because an ID this package mints that the helper
// would reject is a bug this test has to catch, and a shared constant would
// hide it.
var helperRequestID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{7,63}$`)

type recordedCall struct {
	requestID string
	state     privilegedv1.PresenceState
	prefix    string
	iface     string
}

type fakeHelper struct {
	available bool
	calls     []recordedCall
	outcome   privilegedv1.OperationOutcome
	reason    string
	err       error
	// answerID, when set, is returned instead of the request's own ID.
	answerID string
}

func (f *fakeHelper) Available() bool { return f.available }

func (f *fakeHelper) EnsureAddress(_ context.Context, request *privilegedv1.EnsureAddressRequest) (*privilegedv1.EnsureAddressResponse, error) {
	f.calls = append(f.calls, recordedCall{
		requestID: request.GetRequestId(),
		state:     request.GetDesiredState(),
		prefix:    request.GetAddressPrefix(),
		iface:     request.GetInterfaceName(),
	})
	if f.err != nil {
		return nil, f.err
	}
	answered := request.GetRequestId()
	if f.answerID != "" {
		answered = f.answerID
	}
	return &privilegedv1.EnsureAddressResponse{Result: &privilegedv1.OperationResult{
		RequestId: answered, Outcome: f.outcome, ReasonCode: f.reason,
	}}, nil
}

func helperFor(t *testing.T, helper *fakeHelper) Driver {
	t.Helper()
	driver, err := NewHelperDriver(helper)
	if err != nil {
		t.Fatal(err)
	}
	return driver
}

var decoyAddress = Address{DecoyID: "decoy-1", InterfaceName: "eth0", Prefix: "10.20.0.40/32"}

/*
 * Only an answer that says the host is in the requested state becomes presence.
 *
 * `applied` and `unchanged` both say that. Everything else — an unsupported
 * capability, an outcome the helper would not name, a transport failure — does
 * not, and the reconciler above turns each into something that is not a claim.
 */
func TestOnlyAConfirmedAnswerBecomesPresence(t *testing.T) {
	for name, testCase := range map[string]struct {
		outcome privilegedv1.OperationOutcome
		want    Outcome
	}{
		"applied": {privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, OutcomeApplied},
		"unchanged, which is the idempotent second pass": {
			privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED, OutcomeApplied,
		},
		"unsupported, which is a host without the capability": {
			privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNSUPPORTED, OutcomeUnsupported,
		},
		"an outcome the helper would not name": {
			privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNSPECIFIED, OutcomeUnknown,
		},
	} {
		t.Run(name, func(t *testing.T) {
			helper := &fakeHelper{available: true, outcome: testCase.outcome, reason: "address-added"}
			outcome, err := helperFor(t, helper).Present(context.Background(), decoyAddress)
			if err != nil {
				t.Fatal(err)
			}
			if outcome != testCase.want {
				t.Fatalf("outcome = %v, want %v", outcome, testCase.want)
			}
		})
	}
}

/*
 * Present asks for present and Absent asks for absent.
 *
 * This looks too obvious to test, and it is the one mistake in this file that
 * would take a decoy's address off the wire while reporting that it had been
 * placed — or place one while reporting a release.
 */
func TestTheDesiredStateIsNotInverted(t *testing.T) {
	helper := &fakeHelper{available: true, outcome: privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, reason: "ok"}
	driver := helperFor(t, helper)
	if _, err := driver.Present(context.Background(), decoyAddress); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Absent(context.Background(), decoyAddress); err != nil {
		t.Fatal(err)
	}
	if len(helper.calls) != 2 {
		t.Fatalf("issued %d calls, want 2", len(helper.calls))
	}
	if helper.calls[0].state != privilegedv1.PresenceState_PRESENCE_STATE_PRESENT {
		t.Fatalf("Present asked for %v", helper.calls[0].state)
	}
	if helper.calls[1].state != privilegedv1.PresenceState_PRESENCE_STATE_ABSENT {
		t.Fatalf("Absent asked for %v", helper.calls[1].state)
	}
	for _, call := range helper.calls {
		if call.prefix != decoyAddress.Prefix || call.iface != decoyAddress.InterfaceName {
			t.Fatalf("call carried %+v, want the address it was given", call)
		}
	}
}

/*
 * Every call carries a fresh request ID, and the helper caches by request ID.
 *
 * An ID derived from the address would let a call fifteen minutes later be
 * answered from that cache — and an address can be placed, released, and wanted
 * again inside that window, so the cached answer would describe a host state
 * that no longer exists.
 */
func TestEachCallCarriesAFreshIdentifierTheHelperWouldAccept(t *testing.T) {
	helper := &fakeHelper{available: true, outcome: privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, reason: "ok"}
	driver := helperFor(t, helper)
	seen := map[string]struct{}{}
	for attempt := 0; attempt < 16; attempt++ {
		if _, err := driver.Present(context.Background(), decoyAddress); err != nil {
			t.Fatal(err)
		}
	}
	for _, call := range helper.calls {
		if !helperRequestID.MatchString(call.requestID) {
			t.Fatalf("request id %q would be rejected by the helper", call.requestID)
		}
		if _, repeated := seen[call.requestID]; repeated {
			t.Fatalf("request id %q was reused", call.requestID)
		}
		seen[call.requestID] = struct{}{}
	}
	if len(seen) != 16 {
		t.Fatalf("16 calls produced %d distinct identifiers", len(seen))
	}
}

// A helper that answers about some other request has told us nothing about this
// address, and "nothing" is not presence.
func TestAnAnswerToADifferentRequestIsNotAnAnswer(t *testing.T) {
	helper := &fakeHelper{
		available: true,
		outcome:   privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED,
		reason:    "address-added",
		answerID:  "gdn-0000000000000000",
	}
	outcome, err := helperFor(t, helper).Present(context.Background(), decoyAddress)
	if !errors.Is(err, ErrHelperMismatch) {
		t.Fatalf("err = %v, want ErrHelperMismatch", err)
	}
	if outcome == OutcomeApplied {
		t.Fatal("a mismatched answer was treated as a placement")
	}
}

// An unreachable helper is an absent capability. The reconciler reports
// `unsupported` for it, which never claims the address is on the interface.
func TestAnUnreachableHelperIsUnsupportedAndIsNotCalled(t *testing.T) {
	helper := &fakeHelper{available: false}
	outcome, err := helperFor(t, helper).Present(context.Background(), decoyAddress)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != OutcomeUnsupported {
		t.Fatalf("outcome = %v, want unsupported", outcome)
	}
	if len(helper.calls) != 0 {
		t.Fatal("an RPC was issued to a helper reported as unavailable")
	}
}

func TestATransportFailureIsAFailureAndNotAPlacement(t *testing.T) {
	helper := &fakeHelper{available: true, err: errors.New("socket closed")}
	outcome, err := helperFor(t, helper).Present(context.Background(), decoyAddress)
	if err == nil {
		t.Fatal("a transport failure was reported as success")
	}
	if outcome != OutcomeFailed {
		t.Fatalf("outcome = %v, want failed", outcome)
	}
}

// An address this package would not accept never reaches a process holding
// CAP_NET_ADMIN.
func TestAnInvalidAddressNeverReachesTheHelper(t *testing.T) {
	helper := &fakeHelper{available: true, outcome: privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED}
	driver := helperFor(t, helper)
	for _, address := range []Address{
		{DecoyID: "decoy-1", InterfaceName: "eth0", Prefix: "not-a-prefix"},
		{DecoyID: "decoy-1", InterfaceName: "eth0", Prefix: "8.8.8.0/24"},
		// ADR 0016: a decoy address is a /32 identity, never the zone's prefix.
		{DecoyID: "decoy-1", InterfaceName: "eth0", Prefix: "10.20.0.40/24"},
		{DecoyID: "decoy-1", InterfaceName: "", Prefix: "10.20.0.40/32"},
		{DecoyID: "", InterfaceName: "eth0", Prefix: "10.20.0.40/32"},
	} {
		outcome, err := driver.Present(context.Background(), address)
		if err == nil {
			t.Fatalf("%+v was accepted", address)
		}
		if outcome == OutcomeApplied {
			t.Fatalf("%+v was reported as placed", address)
		}
	}
	if len(helper.calls) != 0 {
		t.Fatalf("%d invalid addresses reached the helper", len(helper.calls))
	}
}

func TestADriverNeedsAClient(t *testing.T) {
	if _, err := NewHelperDriver(nil); !errors.Is(err, ErrNoDriver) {
		t.Fatalf("err = %v, want ErrNoDriver", err)
	}
}
