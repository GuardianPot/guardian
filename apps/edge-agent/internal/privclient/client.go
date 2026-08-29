package privclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/health"
	privileged "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged"
	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	componentName  = "privileged-helper"
	probeTimeout   = 2 * time.Second
	monitorPeriod  = 5 * time.Second
	shutdownReason = "shutdown"
)

var ErrUnavailable = errors.New("privileged-helper-unavailable")

type healthStore interface {
	SetHealth(context.Context, storage.HealthCondition) error
}

type socketVerifier func() error
type connectionFactory func() (*grpc.ClientConn, error)

// Client is the unprivileged, typed helper boundary. Helper loss changes
// health to degraded and is never returned as a component-start failure.
type Client struct {
	store            healthStore
	verify           socketVerifier
	connect          connectionFactory
	interval         time.Duration
	timeout          time.Duration
	available        atomic.Bool
	runtimeObserved  atomic.Bool
	runtimeAvailable atomic.Bool
	runtimeTimedOut  atomic.Bool

	mu         sync.Mutex
	started    bool
	cancel     context.CancelFunc
	done       chan struct{}
	connection *grpc.ClientConn
	service    privilegedv1.PrivilegedHelperServiceClient
	lastStatus string
	lastReason string
}

// New creates the production client for the one fixed helper socket.
func New(store healthStore) *Client {
	return newClient(store, defaultSocketVerifier, defaultConnectionFactory, monitorPeriod, probeTimeout)
}

func newClient(store healthStore, verify socketVerifier, connect connectionFactory, interval, timeout time.Duration) *Client {
	return &Client{store: store, verify: verify, connect: connect, interval: interval, timeout: timeout}
}

func (*Client) Name() string { return componentName }

func (c *Client) Available() bool { return c.available.Load() }

func (c *Client) HealthSnapshot() health.HelperSnapshot {
	c.mu.Lock()
	reason := strings.ReplaceAll(c.lastReason, "-", "_")
	c.mu.Unlock()
	return health.HelperSnapshot{
		Available: c.available.Load(), Reason: reason,
		RuntimeObserved: c.runtimeObserved.Load(), RuntimeAvailable: c.runtimeAvailable.Load(),
		RuntimeTimedOut: c.runtimeTimedOut.Load(),
	}
}

func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	monitorCtx, cancel := context.WithCancel(ctx)
	c.started = true
	c.cancel = cancel
	c.done = make(chan struct{})
	c.mu.Unlock()

	statusValue, reason := c.probe(monitorCtx)
	if err := c.recordHealth(ctx, statusValue, reason); err != nil {
		cancel()
		c.closeConnection()
		c.mu.Lock()
		c.started = false
		close(c.done)
		c.mu.Unlock()
		return fmt.Errorf("record privileged-helper health: %w", err)
	}
	go c.monitor(monitorCtx)
	return nil
}

func (c *Client) Stop(ctx context.Context) error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	cancel := c.cancel
	done := c.done
	c.started = false
	c.mu.Unlock()

	cancel()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	c.closeConnection()
	c.available.Store(false)
	return c.recordHealth(ctx, "stopped", shutdownReason)
}

func (c *Client) EnsureAddress(ctx context.Context, request *privilegedv1.EnsureAddressRequest) (*privilegedv1.EnsureAddressResponse, error) {
	service, err := c.serviceClient()
	if err != nil {
		return nil, err
	}
	callCtx, cancel := c.callContext(ctx)
	defer cancel()
	return service.EnsureAddress(callCtx, request)
}

func (c *Client) ApplyNftablesPolicy(ctx context.Context, request *privilegedv1.ApplyNftablesPolicyRequest) (*privilegedv1.ApplyNftablesPolicyResponse, error) {
	service, err := c.serviceClient()
	if err != nil {
		return nil, err
	}
	callCtx, cancel := c.callContext(ctx)
	defer cancel()
	return service.ApplyNftablesPolicy(callCtx, request)
}

func (c *Client) ReconcileContainer(ctx context.Context, request *privilegedv1.ReconcileContainerRequest) (*privilegedv1.ReconcileContainerResponse, error) {
	service, err := c.serviceClient()
	if err != nil {
		return nil, err
	}
	callCtx, cancel := c.callContext(ctx)
	defer cancel()
	return service.ReconcileContainer(callCtx, request)
}

func (c *Client) EnsureNetworkNamespace(ctx context.Context, request *privilegedv1.EnsureNetworkNamespaceRequest) (*privilegedv1.EnsureNetworkNamespaceResponse, error) {
	service, err := c.serviceClient()
	if err != nil {
		return nil, err
	}
	callCtx, cancel := c.callContext(ctx)
	defer cancel()
	return service.EnsureNetworkNamespace(callCtx, request)
}

func (c *Client) monitor(ctx context.Context) {
	defer close(c.done)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			statusValue, reason := c.probe(ctx)
			_ = c.recordHealth(context.Background(), statusValue, reason)
		}
	}
}

func (c *Client) probe(ctx context.Context) (string, string) {
	c.runtimeObserved.Store(false)
	c.runtimeAvailable.Store(false)
	c.runtimeTimedOut.Store(false)
	if err := c.verify(); err != nil {
		c.available.Store(false)
		c.closeConnection()
		if errors.Is(err, os.ErrNotExist) {
			return "degraded", "socket-missing"
		}
		return "degraded", "socket-verification-failed"
	}
	service, err := c.ensureConnection()
	if err != nil {
		c.available.Store(false)
		return "degraded", "connection-create-failed"
	}
	probeCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := service.GetStatus(probeCtx, &privilegedv1.GetStatusRequest{})
	if err != nil {
		c.available.Store(false)
		c.closeConnection()
		switch status.Code(err) {
		case codes.DeadlineExceeded:
			return "degraded", "rpc-timeout"
		case codes.Unauthenticated, codes.PermissionDenied:
			return "degraded", "peer-authentication-failed"
		default:
			return "degraded", "rpc-unavailable"
		}
	}
	if response.GetApiVersion() != privileged.APIVersion {
		c.available.Store(false)
		c.closeConnection()
		return "degraded", "api-version-mismatch"
	}
	c.available.Store(true)
	runtimeCtx, runtimeCancel := context.WithTimeout(ctx, c.timeout)
	defer runtimeCancel()
	runtime, runtimeErr := service.GetRuntimeStatus(runtimeCtx, &privilegedv1.GetRuntimeStatusRequest{})
	if runtimeErr != nil {
		c.runtimeObserved.Store(true)
		c.runtimeTimedOut.Store(status.Code(runtimeErr) == codes.DeadlineExceeded)
		return "healthy", "reachable"
	}
	switch runtime.GetReachability() {
	case privilegedv1.RuntimeReachability_RUNTIME_REACHABILITY_REACHABLE:
		c.runtimeObserved.Store(true)
		c.runtimeAvailable.Store(true)
	case privilegedv1.RuntimeReachability_RUNTIME_REACHABILITY_UNREACHABLE:
		c.runtimeObserved.Store(true)
		c.runtimeTimedOut.Store(runtime.GetReasonCode() == "probe-timeout")
	default:
		c.runtimeObserved.Store(false)
	}
	return "healthy", "reachable"
}

func (c *Client) ensureConnection() (privilegedv1.PrivilegedHelperServiceClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.service != nil {
		return c.service, nil
	}
	connection, err := c.connect()
	if err != nil {
		return nil, err
	}
	c.connection = connection
	c.service = privilegedv1.NewPrivilegedHelperServiceClient(connection)
	return c.service, nil
}

func (c *Client) closeConnection() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connection != nil {
		_ = c.connection.Close()
	}
	c.connection = nil
	c.service = nil
}

func (c *Client) serviceClient() (privilegedv1.PrivilegedHelperServiceClient, error) {
	if !c.available.Load() {
		return nil, ErrUnavailable
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.service == nil {
		return nil, ErrUnavailable
	}
	return c.service, nil
}

func (c *Client) callContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= c.timeout {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.timeout)
}

func (c *Client) recordHealth(ctx context.Context, statusValue, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastStatus == statusValue && c.lastReason == reason {
		return nil
	}
	if err := c.store.SetHealth(ctx, storage.HealthCondition{Name: componentName, Status: statusValue, ReasonCode: reason}); err != nil {
		return err
	}
	c.lastStatus = statusValue
	c.lastReason = reason
	return nil
}

func defaultSocketVerifier() error {
	group, err := user.LookupGroup("guardian-edge")
	if err != nil {
		return errors.New("guardian-edge-group-unavailable")
	}
	gid, err := strconv.ParseUint(group.Gid, 10, 32)
	if err != nil {
		return errors.New("guardian-edge-group-invalid")
	}
	return privileged.VerifyUnixSocket(privileged.SocketOptions{
		Path:          privileged.DefaultSocketPath,
		OwnerUID:      0,
		GroupGID:      uint32(gid),
		DirectoryMode: 0o750,
		SocketMode:    0o660,
	})
}

func defaultConnectionFactory() (*grpc.ClientConn, error) {
	dialer := &net.Dialer{}
	return grpc.NewClient(
		"passthrough:///guardian-edge-privd",
		grpc.WithAuthority("guardian-edge-privd"),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", privileged.DefaultSocketPath)
		}),
		grpc.WithTransportCredentials(privileged.NewClientTransportCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(privileged.MaxMessageBytes),
			grpc.MaxCallSendMsgSize(privileged.MaxMessageBytes),
		),
	)
}
