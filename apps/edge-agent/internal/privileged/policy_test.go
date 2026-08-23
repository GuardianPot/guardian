package privileged

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"google.golang.org/grpc/codes"
)

func TestCompileAllowlistAndRequestValidation(t *testing.T) {
	policy, err := CompileAllowlist(AllowlistInput{
		Interfaces:    []string{"guardian0"},
		Namespaces:    []string{"guardian-decoy-a"},
		Workloads:     []string{"guardian-workload-a"},
		AddressRanges: []string{"192.0.2.0/24", "2001:db8::/48"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := policy.validateAddress(&privilegedv1.EnsureAddressRequest{
		RequestId: "request-0001", InterfaceName: "guardian0", AddressPrefix: "192.0.2.99/32",
		DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
	}); err != nil {
		t.Fatalf("valid address rejected: %v", err)
	}
	if err := policy.validateNftables(&privilegedv1.ApplyNftablesPolicyRequest{
		RequestId: "request-0002", NamespaceName: "guardian-decoy-a",
		Profile: privilegedv1.NftablesProfile_NFTABLES_PROFILE_DEFAULT_DENY_EGRESS,
	}); err != nil {
		t.Fatalf("valid nftables request rejected: %v", err)
	}
	if err := policy.validateContainer(&privilegedv1.ReconcileContainerRequest{
		RequestId: "request-0003", WorkloadId: "guardian-workload-a",
		DesiredState: privilegedv1.ContainerState_CONTAINER_STATE_RUNNING,
	}); err != nil {
		t.Fatalf("valid container request rejected: %v", err)
	}
	if err := policy.validateNamespace(&privilegedv1.EnsureNetworkNamespaceRequest{
		RequestId: "request-0004", NamespaceName: "guardian-decoy-a",
		DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_ABSENT,
	}); err != nil {
		t.Fatalf("valid namespace request rejected: %v", err)
	}

	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{
			name: "command injection interface",
			err: policy.validateAddress(&privilegedv1.EnsureAddressRequest{
				RequestId: "request-0010", InterfaceName: "eth0;id", AddressPrefix: "192.0.2.99/32",
				DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
			}),
			code: codes.InvalidArgument,
		},
		{
			name: "interface outside allowlist",
			err: policy.validateAddress(&privilegedv1.EnsureAddressRequest{
				RequestId: "request-0011", InterfaceName: "guardian1", AddressPrefix: "192.0.2.99/32",
				DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
			}),
			code: codes.PermissionDenied,
		},
		{
			name: "address outside allowlist",
			err: policy.validateAddress(&privilegedv1.EnsureAddressRequest{
				RequestId: "request-0012", InterfaceName: "guardian0", AddressPrefix: "198.51.100.1/32",
				DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
			}),
			code: codes.PermissionDenied,
		},
		{
			name: "path traversal namespace",
			err: policy.validateNamespace(&privilegedv1.EnsureNetworkNamespaceRequest{
				RequestId: "request-0013", NamespaceName: "../../host",
				DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
			}),
			code: codes.InvalidArgument,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violation, ok := asViolation(test.err)
			if !ok || violation.Code != test.code {
				t.Fatalf("error = %v, want violation code %s", test.err, test.code)
			}
		})
	}
}

func TestCompileAllowlistRejectsUnsafeRootPolicy(t *testing.T) {
	tests := []AllowlistInput{
		{Interfaces: []string{"eth0;reboot"}},
		{Namespaces: []string{"../../host"}},
		{Workloads: []string{"/run/containerd.sock"}},
		{AddressRanges: []string{"127.0.0.0/8"}},
		{AddressRanges: []string{"192.0.2.1/24"}},
		{AddressRanges: []string{"::ffff:192.0.2.0/120"}},
	}
	for _, input := range tests {
		if _, err := CompileAllowlist(input); err == nil {
			t.Fatalf("unsafe policy accepted: %+v", input)
		}
	}
}

func TestCompileAllowlistRejectsOversizedRootPolicy(t *testing.T) {
	interfaces := make([]string, maxAllowlistEntries+1)
	for index := range interfaces {
		interfaces[index] = "guardian0"
	}
	if _, err := CompileAllowlist(AllowlistInput{Interfaces: interfaces}); err == nil {
		t.Fatal("oversized root policy accepted")
	}
}

func TestIdempotencyCoalescesConcurrentRetriesAndRejectsConflicts(t *testing.T) {
	registry := newIdempotencyRegistry(4, time.Minute)
	var calls atomic.Int32
	operation := func(context.Context) (AdapterResult, error) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond)
		return AdapterResult{
			Outcome:    privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED,
			ReasonCode: "already-converged",
		}, nil
	}

	results := make(chan error, 8)
	for range 8 {
		go func() {
			result, err := registry.do(context.Background(), "request-1000", "same", operation)
			if err == nil && result.ReasonCode != "already-converged" {
				err = errors.New("unexpected-result")
			}
			results <- err
		}()
	}
	for range 8 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("adapter calls = %d, want 1", calls.Load())
	}
	if _, err := registry.do(context.Background(), "request-1000", "different", operation); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting retry error = %v", err)
	}
}
