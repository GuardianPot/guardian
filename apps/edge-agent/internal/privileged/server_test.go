package privileged

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type memoryAudit struct {
	mu     sync.Mutex
	events []AuditEvent
}

func (a *memoryAudit) Record(_ context.Context, event AuditEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, event)
}

func (a *memoryAudit) snapshot() []AuditEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]AuditEvent(nil), a.events...)
}

type testAdapter struct {
	addressCalls atomic.Int32
	address      func(context.Context, AddressOperation) (AdapterResult, error)
}

type runtimeProberFunc func(context.Context) (bool, string)

func (function runtimeProberFunc) ProbeRuntime(ctx context.Context) (bool, string) {
	return function(ctx)
}

func (*testAdapter) Capabilities() map[privilegedv1.PrivilegedOperation]privilegedv1.CapabilityState {
	return map[privilegedv1.PrivilegedOperation]privilegedv1.CapabilityState{
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_ADDRESS:             privilegedv1.CapabilityState_CAPABILITY_STATE_AVAILABLE,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_NFTABLES_POLICY:     privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_CONTAINER_LIFECYCLE: privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
		privilegedv1.PrivilegedOperation_PRIVILEGED_OPERATION_NETWORK_NAMESPACE:   privilegedv1.CapabilityState_CAPABILITY_STATE_UNSUPPORTED,
	}
}

func (a *testAdapter) EnsureAddress(ctx context.Context, operation AddressOperation) (AdapterResult, error) {
	a.addressCalls.Add(1)
	if a.address != nil {
		return a.address(ctx, operation)
	}
	return AdapterResult{Outcome: privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED, ReasonCode: "already-converged"}, nil
}

func (*testAdapter) ApplyNftablesPolicy(context.Context, NftablesOperation) (AdapterResult, error) {
	return unsupportedResult(), nil
}

func (*testAdapter) ReconcileContainer(context.Context, ContainerOperation) (AdapterResult, error) {
	return unsupportedResult(), nil
}

func (*testAdapter) EnsureNetworkNamespace(context.Context, NamespaceOperation) (AdapterResult, error) {
	return unsupportedResult(), nil
}

type testRPCServer struct {
	server   *Server
	listener net.Listener
	conn     *grpc.ClientConn
	client   privilegedv1.PrivilegedHelperServiceClient
	audit    *memoryAudit
}

func startTestRPCServer(t *testing.T, adapter Adapter) *testRPCServer {
	t.Helper()
	allowlist, err := CompileAllowlist(AllowlistInput{
		Interfaces:    []string{"guardian0"},
		Namespaces:    []string{"guardian-decoy-a"},
		Workloads:     []string{"guardian-workload-a"},
		AddressRanges: []string{"192.0.2.0/24"},
	})
	if err != nil {
		t.Fatal(err)
	}
	audit := &memoryAudit{}
	server, err := NewServer(ServerConfig{
		PeerPolicy: PeerPolicy{
			AllowedUID: uint32(os.Geteuid()),
			AllowedGID: uint32(os.Getegid()),
			Verifier: verifierFunc(func(PeerCredentials) (io.Closer, error) {
				return emptyCloser{}, nil
			}),
		},
		Allowlist: allowlist,
		Adapter:   adapter,
		Audit:     audit,
		Runtime: runtimeProberFunc(func(context.Context) (bool, string) {
			return true, "reachable"
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "grpc.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	dialer := &net.Dialer{}
	conn, err := grpc.NewClient(
		"passthrough:///guardian-test-helper",
		grpc.WithAuthority("guardian-test-helper"),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", path)
		}),
		grpc.WithTransportCredentials(NewClientTransportCredentials()),
	)
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		server.Stop()
		_ = listener.Close()
		select {
		case <-serveDone:
		case <-time.After(time.Second):
			t.Error("gRPC server did not stop")
		}
	})
	return &testRPCServer{
		server: server, listener: listener, conn: conn,
		client: privilegedv1.NewPrivilegedHelperServiceClient(conn), audit: audit,
	}
}

func TestServerTypedAllowlistIdempotencyAndAudit(t *testing.T) {
	adapter := &testAdapter{address: func(context.Context, AddressOperation) (AdapterResult, error) {
		time.Sleep(20 * time.Millisecond)
		return AdapterResult{Outcome: privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED, ReasonCode: "already-converged"}, nil
	}}
	fixture := startTestRPCServer(t, adapter)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	statusResponse, err := fixture.client.GetStatus(ctx, &privilegedv1.GetStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	runtimeResponse, err := fixture.client.GetRuntimeStatus(ctx, &privilegedv1.GetRuntimeStatusRequest{})
	if err != nil || runtimeResponse.GetReachability() != privilegedv1.RuntimeReachability_RUNTIME_REACHABILITY_REACHABLE || runtimeResponse.GetReasonCode() != "reachable" {
		t.Fatalf("GetRuntimeStatus() = (%+v, %v)", runtimeResponse, err)
	}
	if statusResponse.GetApiVersion() != APIVersion || len(statusResponse.GetCapabilities()) != 4 {
		t.Fatalf("status response = %+v", statusResponse)
	}

	request := &privilegedv1.EnsureAddressRequest{
		RequestId: "request-2000", InterfaceName: "guardian0", AddressPrefix: "192.0.2.20/32",
		DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
	}
	results := make(chan error, 8)
	for range 8 {
		go func() {
			response, err := fixture.client.EnsureAddress(ctx, request)
			if err == nil && response.GetResult().GetOutcome() != privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED {
				err = fmt.Errorf("unexpected response: %v", response)
			}
			results <- err
		}()
	}
	for range 8 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if adapter.addressCalls.Load() != 1 {
		t.Fatalf("address adapter calls = %d, want 1", adapter.addressCalls.Load())
	}

	conflict := &privilegedv1.EnsureAddressRequest{
		RequestId: "request-2000", InterfaceName: "guardian0", AddressPrefix: "192.0.2.21/32",
		DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
	}
	if _, err := fixture.client.EnsureAddress(ctx, conflict); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("conflicting retry code = %s, error = %v", status.Code(err), err)
	}

	injection := &privilegedv1.EnsureAddressRequest{
		RequestId: "request-2001", InterfaceName: "eth0;touch", AddressPrefix: "192.0.2.21/32",
		DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
	}
	if _, err := fixture.client.EnsureAddress(ctx, injection); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("injection code = %s, error = %v", status.Code(err), err)
	}

	notAllowed := &privilegedv1.EnsureAddressRequest{
		RequestId: "request-2002", InterfaceName: "guardian1", AddressPrefix: "192.0.2.21/32",
		DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
	}
	if _, err := fixture.client.EnsureAddress(ctx, notAllowed); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("not-allowlisted code = %s, error = %v", status.Code(err), err)
	}

	invalidRequestID := &privilegedv1.EnsureAddressRequest{
		RequestId: "secret\npayload", InterfaceName: "guardian0", AddressPrefix: "192.0.2.21/32",
		DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
	}
	if _, err := fixture.client.EnsureAddress(ctx, invalidRequestID); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid request ID code = %s, error = %v", status.Code(err), err)
	}

	unknownField := &privilegedv1.EnsureNetworkNamespaceRequest{
		RequestId: "request-2003", NamespaceName: "guardian-decoy-a",
		DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
	}
	unknownField.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	if _, err := fixture.client.EnsureNetworkNamespace(ctx, unknownField); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unknown protobuf field code = %s, error = %v", status.Code(err), err)
	}

	var unknownResponse privilegedv1.GetStatusResponse
	if err := fixture.conn.Invoke(ctx, "/guardian.privileged.v1.PrivilegedHelperService/Unknown", &privilegedv1.GetStatusRequest{}, &unknownResponse); status.Code(err) != codes.Unimplemented {
		t.Fatalf("unknown method code = %s, error = %v", status.Code(err), err)
	}

	oversized := &privilegedv1.EnsureAddressRequest{RequestId: strings.Repeat("a", MaxMessageBytes*2)}
	if _, err := fixture.client.EnsureAddress(ctx, oversized); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("oversized message code = %s, error = %v", status.Code(err), err)
	}

	waitForAudit(t, fixture.audit, func(events []AuditEvent) bool {
		var sawAccepted, sawRejected, sawUnknown, sawOversized bool
		for _, event := range events {
			sawAccepted = sawAccepted || event.Outcome == "accepted"
			sawRejected = sawRejected || event.Outcome == "rejected"
			sawUnknown = sawUnknown || strings.HasSuffix(event.Method, "/Unknown")
			sawOversized = sawOversized || event.ReasonCode == "grpc-resourceexhausted"
			rendered := fmt.Sprintf("%+v", event)
			if strings.Contains(rendered, "eth0;touch") || strings.Contains(rendered, "secret") {
				t.Fatal("audit event leaked request payload")
			}
		}
		return sawAccepted && sawRejected && sawUnknown && sawOversized
	})
}

func TestAuditMethodRejectsUntrustedLogValues(t *testing.T) {
	for _, method := range []string{"", "/service/method;payload", "/" + strings.Repeat("a", 256) + "/Method"} {
		if got := safeAuditMethod(method); got != "unknown" {
			t.Fatalf("unsafe audit method %q recorded as %q", method, got)
		}
	}
}

func TestServerCancellationIsBoundedAndAudited(t *testing.T) {
	adapter := &testAdapter{address: func(ctx context.Context, _ AddressOperation) (AdapterResult, error) {
		<-ctx.Done()
		return AdapterResult{}, ctx.Err()
	}}
	fixture := startTestRPCServer(t, adapter)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := fixture.client.EnsureAddress(ctx, &privilegedv1.EnsureAddressRequest{
		RequestId: "request-3000", InterfaceName: "guardian0", AddressPrefix: "192.0.2.30/32",
		DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
	})
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("deadline code = %s, error = %v", status.Code(err), err)
	}
	waitForAudit(t, fixture.audit, func(events []AuditEvent) bool {
		for _, event := range events {
			if event.RequestID == "request-3000" && event.Outcome == "cancelled" {
				return true
			}
		}
		return false
	})
}

func waitForAudit(t *testing.T, audit *memoryAudit, predicate func([]AuditEvent) bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if predicate(audit.snapshot()) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected audit event not found: %+v", audit.snapshot())
}
