package privclient

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged"
	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type healthMemory struct {
	mu         sync.Mutex
	conditions []storage.HealthCondition
}

type flakyHealthStore struct {
	mu    sync.Mutex
	calls int
}

func (s *flakyHealthStore) SetHealth(context.Context, storage.HealthCondition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.calls == 1 {
		return errors.New("temporary-health-store-failure")
	}
	return nil
}

func (s *healthMemory) SetHealth(_ context.Context, condition storage.HealthCondition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conditions = append(s.conditions, condition)
	return nil
}

func (s *healthMemory) latest() storage.HealthCondition {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.conditions) == 0 {
		return storage.HealthCondition{}
	}
	return s.conditions[len(s.conditions)-1]
}

type verifierFunc func(privileged.PeerCredentials) (io.Closer, error)

func (f verifierFunc) Verify(peer privileged.PeerCredentials) (io.Closer, error) { return f(peer) }

type emptyCloser struct{}

func (emptyCloser) Close() error { return nil }

type runtimeProbeFunc func(context.Context) (bool, string)

func (function runtimeProbeFunc) ProbeRuntime(ctx context.Context) (bool, string) {
	return function(ctx)
}

type runningHelper struct {
	server   *privileged.Server
	listener net.Listener
	done     chan error
}

func startHelper(t *testing.T, path string) *runningHelper {
	t.Helper()
	allowlist, err := privileged.CompileAllowlist(privileged.AllowlistInput{})
	if err != nil {
		t.Fatal(err)
	}
	server, err := privileged.NewServer(privileged.ServerConfig{
		PeerPolicy: privileged.PeerPolicy{
			AllowedUID: uint32(os.Geteuid()), AllowedGID: uint32(os.Getegid()),
			Verifier: verifierFunc(func(privileged.PeerCredentials) (io.Closer, error) {
				return emptyCloser{}, nil
			}),
		},
		Allowlist: allowlist,
		Adapter:   privileged.UnsupportedAdapter{},
		Audit:     privileged.NewSlogAuditRecorder(nil),
		Runtime: runtimeProbeFunc(func(context.Context) (bool, string) {
			return true, "reachable"
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	return &runningHelper{server: server, listener: listener, done: done}
}

func (h *runningHelper) stop(t *testing.T) {
	t.Helper()
	h.server.Stop()
	_ = h.listener.Close()
	select {
	case <-h.done:
	case <-time.After(time.Second):
		t.Fatal("helper did not stop")
	}
}

func TestMissingHelperDegradesWithoutStartFailure(t *testing.T) {
	store := &healthMemory{}
	client := newClient(
		store,
		func() error { return os.ErrNotExist },
		func() (*grpc.ClientConn, error) { t.Fatal("connect called after verifier failure"); return nil, nil },
		20*time.Millisecond,
		50*time.Millisecond,
	)
	ctx, cancel := context.WithCancel(context.Background())
	if err := client.Start(ctx); err != nil {
		t.Fatalf("helper loss failed daemon component start: %v", err)
	}
	if client.Available() {
		t.Fatal("missing helper reported available")
	}
	if condition := store.latest(); condition.Status != "degraded" || condition.ReasonCode != "socket-missing" {
		t.Fatalf("health condition = %+v", condition)
	}
	cancel()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := client.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
}

func TestRecordHealthRetriesAfterStoreFailure(t *testing.T) {
	store := &flakyHealthStore{}
	client := &Client{store: store}
	if err := client.recordHealth(context.Background(), "degraded", "socket-missing"); err == nil {
		t.Fatal("first health-store failure was not returned")
	}
	if err := client.recordHealth(context.Background(), "degraded", "socket-missing"); err != nil {
		t.Fatalf("health update was not retried: %v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.calls != 2 {
		t.Fatalf("health-store calls = %d, want 2", store.calls)
	}
}

func TestHelperStopAndRestartUpdatesAvailabilityWithoutClientCrash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "helper.sock")
	helper := startHelper(t, path)
	store := &healthMemory{}
	verifier := func() error {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSocket == 0 {
			return os.ErrInvalid
		}
		return nil
	}
	connect := func() (*grpc.ClientConn, error) {
		dialer := &net.Dialer{}
		return grpc.NewClient(
			"passthrough:///guardian-test-helper",
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, "unix", path)
			}),
			grpc.WithTransportCredentials(privileged.NewClientTransportCredentials()),
		)
	}
	client := newClient(store, verifier, connect, 20*time.Millisecond, 100*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	waitForClient(t, func() bool { return client.Available() && store.latest().ReasonCode == "reachable" })

	_, err := client.EnsureAddress(context.Background(), &privilegedv1.EnsureAddressRequest{
		RequestId: "request-4000", InterfaceName: "guardian0", AddressPrefix: "192.0.2.40/32",
		DesiredState: privilegedv1.PresenceState_PRESENCE_STATE_PRESENT,
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("typed client call code = %s, error = %v", status.Code(err), err)
	}

	helper.stop(t)
	waitForClient(t, func() bool { return !client.Available() && store.latest().Status == "degraded" })
	helper = startHelper(t, path)
	defer helper.stop(t)
	waitForClient(t, func() bool { return client.Available() && store.latest().ReasonCode == "reachable" })

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := client.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	if condition := store.latest(); condition.Status != "stopped" || condition.ReasonCode != "shutdown" {
		t.Fatalf("stopped health = %+v", condition)
	}
}

func waitForClient(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("client state did not converge")
}
