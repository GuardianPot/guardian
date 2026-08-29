// Package devicechannel owns the authenticated, reconnecting, bounded Edge
// side of the outbound management stream.
package devicechannel

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net"
	"sync"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/health"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	HeartbeatInterval = 30 * time.Second
	StaleAfter        = 90 * time.Second
	HelloTimeout      = 10 * time.Second
	MinBackoff        = time.Second
	MaxBackoff        = 60 * time.Second
)

var ErrProtocolIncompatible = errors.New("device channel protocol is incompatible")

type DesiredHandler interface {
	DesiredState(context.Context, *devicev1.DesiredStateSnapshot) error
}

type ObservedAcknowledgementHandler interface {
	Acknowledgement(context.Context, *devicev1.Acknowledgement) error
}

type StateRecorder interface {
	SetChannelState(context.Context, string, string) error
}

type StateRecorderFunc func(context.Context, string, string) error

func (function StateRecorderFunc) SetChannelState(ctx context.Context, state, reason string) error {
	return function(ctx, state, reason)
}

type Config struct {
	Endpoint                       string
	AgentVersion                   string
	Credentials                    identity.Credentials
	RootCAs                        *x509.CertPool
	Logger                         *slog.Logger
	DesiredHandler                 DesiredHandler
	ObservedAcknowledgementHandler ObservedAcknowledgementHandler
	HealthAcknowledgementHandler   ObservedAcknowledgementHandler
	StateRecorder                  StateRecorder
}

type Client struct {
	endpoint                       string
	agentVersion                   string
	credentials                    identity.Credentials
	rootCAs                        *x509.CertPool
	logger                         *slog.Logger
	desiredHandler                 DesiredHandler
	observedAcknowledgementHandler ObservedAcknowledgementHandler
	healthAcknowledgementHandler   ObservedAcknowledgementHandler
	stateRecorder                  StateRecorder
	outgoing                       *requestQueue
	heartbeatInterval              time.Duration
	stableAfter                    time.Duration
	helloTimeout                   time.Duration
	jitter                         func(time.Duration) time.Duration

	mu      sync.RWMutex
	state   string
	reason  string
	started bool
	cancel  context.CancelFunc
	done    chan struct{}
}

func NewClient(config Config) (*Client, error) {
	host, _, err := net.SplitHostPort(config.Endpoint)
	if err != nil || host == "" || !validAgentVersion(config.AgentVersion) || config.Logger == nil || config.StateRecorder == nil {
		return nil, errors.New("device channel endpoint, agent version, logger, and state recorder are required")
	}
	if len(config.Credentials.Certificate.Certificate) == 0 || config.Credentials.Certificate.PrivateKey == nil || config.Credentials.Leaf == nil || !validUUIDv7(config.Credentials.Metadata.DeviceID) {
		return nil, errors.New("validated device channel identity is required")
	}
	return &Client{
		endpoint: config.Endpoint, agentVersion: config.AgentVersion, credentials: config.Credentials,
		rootCAs: config.RootCAs, logger: config.Logger, desiredHandler: config.DesiredHandler,
		observedAcknowledgementHandler: config.ObservedAcknowledgementHandler,
		healthAcknowledgementHandler:   config.HealthAcknowledgementHandler, stateRecorder: config.StateRecorder,
		outgoing: newRequestQueue(), heartbeatInterval: HeartbeatInterval, stableAfter: StaleAfter,
		helloTimeout: HelloTimeout, jitter: fullJitter, state: "disconnected", reason: "not_started",
	}, nil
}

func (*Client) Name() string { return "device-channel" }

func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return errors.New("device channel already started")
	}
	runContext, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.done = make(chan struct{})
	c.started = true
	c.mu.Unlock()
	if err := c.setState(ctx, "connecting", "startup"); err != nil {
		cancel()
		c.mu.Lock()
		c.cancel = nil
		c.done = nil
		c.started = false
		c.state = "disconnected"
		c.reason = "startup_state_failed"
		c.mu.Unlock()
		return err
	}
	go func() {
		defer close(c.done)
		c.run(runContext)
	}()
	return nil
}

func (c *Client) Stop(ctx context.Context) error {
	c.mu.RLock()
	cancel := c.cancel
	done := c.done
	c.mu.RUnlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return c.setState(ctx, "stopped", "shutdown")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) ConnectionState() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Client) EnqueueObserved(observed *devicev1.ObservedState) error {
	if err := validateObserved(observed); err != nil {
		return err
	}
	return c.outgoing.enqueue(&devicev1.ConnectRequest{Payload: &devicev1.ConnectRequest_ObservedState{ObservedState: observed}})
}

func (c *Client) EnqueueHealth(report health.Report) error {
	frame, err := healthReportFrame(report)
	if err != nil {
		return err
	}
	return c.outgoing.enqueue(frame)
}

func (c *Client) run(ctx context.Context) {
	attempt := 0
	for ctx.Err() == nil {
		connectedFor, err := c.connectOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		reason := "connection_lost"
		if errors.Is(err, ErrProtocolIncompatible) {
			reason = "protocol_incompatible"
		}
		_ = c.setState(ctx, "degraded", reason)
		if connectedFor >= c.stableAfter {
			attempt = 0
		} else if attempt < 30 {
			attempt++
		}
		delay := c.jitter(backoff(attempt))
		_ = c.setState(ctx, "connecting", "reconnect_backoff")
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		}
	}
}

func (c *Client) connectOnce(ctx context.Context) (time.Duration, error) {
	host, _, _ := net.SplitHostPort(c.endpoint)
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		ServerName:   host,
		Certificates: []tls.Certificate{c.credentials.Certificate},
		RootCAs:      c.rootCAs,
	}
	connection, err := grpc.NewClient(c.endpoint,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageBytes), grpc.MaxCallSendMsgSize(MaxMessageBytes)),
		grpc.WithMaxHeaderListSize(uint32(MaxHeaderBytes)),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler(otelgrpc.WithPropagators(propagation.TraceContext{}))),
		grpc.WithDisableRetry(),
	)
	if err != nil {
		return 0, err
	}
	defer connection.Close()
	sessionContext, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := devicev1.NewDeviceChannelServiceClient(connection).Connect(sessionContext)
	if err != nil {
		return 0, err
	}
	if err := stream.Send(&devicev1.ConnectRequest{Payload: &devicev1.ConnectRequest_Hello{Hello: &devicev1.EdgeHello{
		Protocol: &devicev1.ProtocolVersion{Major: ProtocolMajor, Minor: ProtocolMinor}, AgentVersion: c.agentVersion,
	}}}); err != nil {
		return 0, err
	}
	response, err := receiveSelection(sessionContext, cancel, stream, c.helloTimeout)
	if err != nil {
		return 0, err
	}
	selection := response.GetProtocolSelection()
	if selection == nil || selection.Selected == nil {
		return 0, status.Error(codes.InvalidArgument, "protocol selection is invalid")
	}
	if !selection.Accepted || selection.Selected.Major != ProtocolMajor || selection.Selected.Minor > ProtocolMinor {
		return 0, ErrProtocolIncompatible
	}
	connectedAt := time.Now()
	if err := c.setState(sessionContext, "connected", "negotiated"); err != nil {
		return 0, err
	}
	c.logger.InfoContext(sessionContext, "Device channel connected", "control_plane_endpoint", c.endpoint)
	c.outgoing.wake()
	receiveErrors := make(chan error, 1)
	go func() { receiveErrors <- c.receiveLoop(sessionContext, stream) }()
	heartbeats := time.NewTicker(c.heartbeatInterval)
	defer heartbeats.Stop()

	for {
		select {
		case <-heartbeats.C:
			if err := stream.Send(&devicev1.ConnectRequest{Payload: &devicev1.ConnectRequest_Heartbeat{Heartbeat: &devicev1.Heartbeat{
				SentAt: timestamppb.Now(),
			}}}); err != nil {
				return time.Since(connectedAt), err
			}
		case <-c.outgoing.ready:
			item, ok := c.outgoing.peek()
			if !ok {
				continue
			}
			if err := stream.Send(item.frame); err != nil {
				c.outgoing.wake()
				return time.Since(connectedAt), err
			}
			c.outgoing.sent(item)
		case err := <-receiveErrors:
			return time.Since(connectedAt), err
		case <-sessionContext.Done():
			return time.Since(connectedAt), sessionContext.Err()
		}
	}
}

func (c *Client) receiveLoop(ctx context.Context, stream grpc.BidiStreamingClient[devicev1.ConnectRequest, devicev1.ConnectResponse]) error {
	duplicates := make(map[string]struct{})
	order := make([]string, 0, 256)
	for {
		response, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch payload := response.Payload.(type) {
		case *devicev1.ConnectResponse_DesiredState:
			if err := validateDesiredEnvelope(payload.DesiredState); err != nil {
				return status.Error(codes.InvalidArgument, "desired state is invalid")
			}
			if _, seen := duplicates[payload.DesiredState.MessageId]; !seen {
				if c.desiredHandler == nil {
					return status.Error(codes.Unimplemented, "desired-state handler is unavailable")
				}
				if err := c.desiredHandler.DesiredState(ctx, payload.DesiredState); err != nil {
					return status.Error(codes.Internal, "desired-state handling failed")
				}
				if len(order) == cap(order) {
					delete(duplicates, order[0])
					order = order[1:]
				}
				order = append(order, payload.DesiredState.MessageId)
				duplicates[payload.DesiredState.MessageId] = struct{}{}
			}
			if err := c.outgoing.enqueue(&devicev1.ConnectRequest{Payload: &devicev1.ConnectRequest_Acknowledgement{
				Acknowledgement: &devicev1.Acknowledgement{MessageId: payload.DesiredState.MessageId, Kind: devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_DESIRED_STATE, Revision: payload.DesiredState.Revision},
			}}); err != nil {
				return status.Error(codes.ResourceExhausted, "device channel is saturated")
			}
		case *devicev1.ConnectResponse_Acknowledgement:
			if err := validateAcknowledgement(payload.Acknowledgement); err != nil {
				return status.Error(codes.InvalidArgument, "acknowledgement is invalid")
			}
			switch payload.Acknowledgement.Kind {
			case devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_OBSERVED_STATE:
				if c.observedAcknowledgementHandler == nil {
					return status.Error(codes.Unimplemented, "observed acknowledgement handler is unavailable")
				}
				if err := c.observedAcknowledgementHandler.Acknowledgement(ctx, payload.Acknowledgement); err != nil {
					return status.Error(codes.Internal, "observed acknowledgement handling failed")
				}
			case devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_HEALTH_REPORT:
				if c.healthAcknowledgementHandler == nil {
					return status.Error(codes.Unimplemented, "health acknowledgement handler is unavailable")
				}
				if err := c.healthAcknowledgementHandler.Acknowledgement(ctx, payload.Acknowledgement); err != nil {
					return status.Error(codes.Internal, "health acknowledgement handling failed")
				}
			default:
				return status.Error(codes.InvalidArgument, "acknowledgement kind is invalid")
			}
		default:
			return status.Error(codes.InvalidArgument, "unexpected control channel frame")
		}
	}
}

func (c *Client) setState(ctx context.Context, state, reason string) error {
	c.mu.Lock()
	c.state = state
	c.reason = reason
	c.mu.Unlock()
	return c.stateRecorder.SetChannelState(ctx, state, reason)
}

func receiveSelection(ctx context.Context, cancel context.CancelFunc, stream grpc.BidiStreamingClient[devicev1.ConnectRequest, devicev1.ConnectResponse], timeout time.Duration) (*devicev1.ConnectResponse, error) {
	type result struct {
		response *devicev1.ConnectResponse
		err      error
	}
	results := make(chan result, 1)
	go func() {
		response, err := stream.Recv()
		results <- result{response: response, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case received := <-results:
		return received.response, received.err
	case <-timer.C:
		cancel()
		return nil, status.Error(codes.DeadlineExceeded, "protocol selection deadline exceeded")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func validateObserved(observed *devicev1.ObservedState) error {
	if observed == nil || !validUUIDv7(observed.MessageId) || observed.DesiredRevision == 0 ||
		observed.ObservedRevision > observed.DesiredRevision || observed.LastGoodRevision > observed.ObservedRevision ||
		!validObservedCondition(observed.Condition) {
		return errors.New("observed state is invalid")
	}
	if observed.Condition.Status == devicev1.ReconciliationConditionStatus_RECONCILIATION_CONDITION_STATUS_CONVERGED &&
		(observed.ObservedRevision != observed.DesiredRevision || observed.LastGoodRevision != observed.ObservedRevision) {
		return errors.New("converged observed state is inconsistent")
	}
	return nil
}

func backoff(attempt int) time.Duration {
	maximum := MinBackoff
	for index := 1; index < attempt && maximum < MaxBackoff; index++ {
		maximum *= 2
		if maximum > MaxBackoff {
			maximum = MaxBackoff
		}
	}
	return maximum
}

func fullJitter(maximum time.Duration) time.Duration {
	if maximum <= 0 {
		return 0
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(maximum)+1))
	if err != nil {
		return maximum
	}
	return time.Duration(value.Int64())
}
