// Package devicechannel owns the authenticated, bounded Control Plane side of
// the outbound Edge management stream.
package devicechannel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel/gen/guardian/device/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const (
	HeartbeatInterval = 30 * time.Second
	StaleAfter        = 90 * time.Second
	HelloTimeout      = 10 * time.Second
)

// CertificateVerifier is the P1-W4 chain, URI SAN, active-serial, and durable
// device-state boundary.
type CertificateVerifier interface {
	VerifyCertificate(context.Context, *x509.Certificate) (deviceID string, serial string, err error)
}

// ReconciliationHandler owns only P1-W6 desired/observed state. P1-W9 health
// remains a separate seam so an installed reconciler cannot discard or
// acknowledge health truth.
type ReconciliationHandler interface {
	DesiredState(context.Context, DeviceIdentity) (*devicev1.DesiredStateSnapshot, error)
	ObservedState(context.Context, DeviceIdentity, *devicev1.ObservedState) error
	Acknowledgement(context.Context, DeviceIdentity, *devicev1.Acknowledgement) error
}

type HealthHandler interface {
	HealthReport(context.Context, DeviceIdentity, *devicev1.HealthReport) error
}

type HealthDisconnectHandler interface {
	ChannelClosed(context.Context, DeviceIdentity) error
}

type DeviceIdentity struct {
	DeviceID          string
	CertificateSerial string
}

type Config struct {
	Address            string
	TLSCertificateFile string
	TLSPrivateKeyFile  string
	DeviceCAPEM        []byte
	Verifier           CertificateVerifier
	Reconciliation     ReconciliationHandler
	Health             HealthHandler
	Logger             *slog.Logger
}

type Server struct {
	devicev1.UnimplementedDeviceChannelServiceServer

	address         string
	verifier        CertificateVerifier
	reconciliation  ReconciliationHandler
	health          HealthHandler
	logger          *slog.Logger
	grpc            *grpc.Server
	errors          chan error
	listener        net.Listener
	startOnce       sync.Once
	stopOnce        sync.Once
	stopped         chan struct{}
	startErr        error
	sessions        sessionRegistry
	healthRate      healthRateLimiter
	helloTimeout    time.Duration
	recheckInterval time.Duration
	staleAfter      time.Duration
}

func NewServer(config Config) (*Server, error) {
	if config.Address == "" || config.Verifier == nil || config.Logger == nil {
		return nil, errors.New("device channel address, verifier, and logger are required")
	}
	identity, err := tls.LoadX509KeyPair(config.TLSCertificateFile, config.TLSPrivateKeyFile)
	if err != nil {
		return nil, errors.New("load device channel TLS identity")
	}
	clientRoots := x509.NewCertPool()
	if len(config.DeviceCAPEM) == 0 || !clientRoots.AppendCertsFromPEM(config.DeviceCAPEM) {
		return nil, errors.New("load product device CA trust")
	}
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{identity},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientRoots,
	}
	server := &Server{
		address: config.Address, verifier: config.Verifier, reconciliation: config.Reconciliation,
		health: config.Health,
		logger: config.Logger, errors: make(chan error, 1), stopped: make(chan struct{}),
		helloTimeout: HelloTimeout, recheckInterval: HeartbeatInterval, staleAfter: StaleAfter,
	}
	server.sessions.active = make(map[string]*activeSession)
	server.healthRate.devices = make(map[string]*tokenBucket)
	server.grpc = grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.MaxRecvMsgSize(MaxMessageBytes),
		grpc.MaxSendMsgSize(MaxMessageBytes),
		grpc.MaxHeaderListSize(uint32(MaxHeaderBytes)),
		grpc.StatsHandler(otelgrpc.NewServerHandler(otelgrpc.WithPropagators(propagation.TraceContext{}))),
	)
	devicev1.RegisterDeviceChannelServiceServer(server.grpc, server)
	return server, nil
}

func (s *Server) Start() error {
	s.startOnce.Do(func() {
		s.listener, s.startErr = net.Listen("tcp", s.address)
		if s.startErr != nil {
			return
		}
		go func() {
			err := s.grpc.Serve(s.listener)
			if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				s.errors <- err
			}
			close(s.errors)
		}()
	})
	return s.startErr
}

func (s *Server) Address() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) Errors() <-chan error { return s.errors }

func (s *Server) Shutdown(ctx context.Context) error {
	s.stopOnce.Do(func() {
		go func() {
			s.grpc.GracefulStop()
			close(s.stopped)
		}()
	})
	select {
	case <-s.stopped:
		return nil
	case <-ctx.Done():
		s.grpc.Stop()
		return ctx.Err()
	}
}

// EnqueueDesired is the bounded P1-W6 integration seam. A successful return
// means queued for the current stream, not applied by the Edge.
func (s *Server) EnqueueDesired(deviceID string, desired *devicev1.DesiredStateSnapshot) error {
	if !validUUIDv7(deviceID) || validateDesiredState(desired, deviceID) != nil {
		return errors.New("desired-state envelope is invalid")
	}
	session := s.sessions.get(deviceID)
	if session == nil {
		return ErrNotConnected
	}
	return session.outgoing.enqueue(&devicev1.ConnectResponse{Payload: &devicev1.ConnectResponse_DesiredState{
		DesiredState: desired,
	}})
}

func (s *Server) Connect(stream grpc.BidiStreamingServer[devicev1.ConnectRequest, devicev1.ConnectResponse]) error {
	certificate, err := peerCertificate(stream.Context())
	if err != nil {
		return status.Error(codes.Unauthenticated, "authenticated device certificate is required")
	}
	deviceID, serial, err := s.verifier.VerifyCertificate(stream.Context(), certificate)
	if err != nil || !validUUIDv7(deviceID) || serial == "" {
		return status.Error(codes.Unauthenticated, "device certificate is not eligible")
	}
	identity := DeviceIdentity{DeviceID: deviceID, CertificateSerial: serial}
	request, err := receiveHello(stream, s.helloTimeout)
	if err != nil {
		return err
	}
	hello := request.GetHello()
	if err := validateHello(hello); err != nil {
		return status.Error(codes.InvalidArgument, "valid hello must be the first frame")
	}
	selection := &devicev1.ProtocolSelection{
		Selected: &devicev1.ProtocolVersion{Major: ProtocolMajor, Minor: ProtocolMinor},
		Accepted: protocolCompatible(hello.Protocol),
	}
	if !selection.Accepted {
		selection.Reason = "protocol_incompatible"
		_ = stream.Send(&devicev1.ConnectResponse{Payload: &devicev1.ConnectResponse_ProtocolSelection{ProtocolSelection: selection}})
		return status.Error(codes.FailedPrecondition, "device protocol is incompatible")
	}
	if err := stream.Send(&devicev1.ConnectResponse{Payload: &devicev1.ConnectResponse_ProtocolSelection{ProtocolSelection: selection}}); err != nil {
		return err
	}

	session := newActiveSession(stream.Context(), identity)
	s.sessions.activate(session)
	defer func() {
		wasCurrent := s.sessions.remove(session)
		session.cancel()
		if wasCurrent {
			if handler, ok := s.health.(HealthDisconnectHandler); ok {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := handler.ChannelClosed(ctx, session.identity); err != nil {
					s.logger.ErrorContext(ctx, "Record device channel disconnect failed", "device_id", deviceID)
				}
			}
		}
	}()
	if s.reconciliation != nil {
		desired, handlerErr := s.reconciliation.DesiredState(stream.Context(), identity)
		if handlerErr != nil {
			return status.Error(codes.Internal, "desired-state publication failed")
		}
		if desired != nil {
			if err := s.EnqueueDesired(deviceID, desired); err != nil {
				return status.Error(codes.ResourceExhausted, "desired-state publication is saturated")
			}
		}
	}
	s.logger.InfoContext(stream.Context(), "Device channel connected", "device_id", deviceID)

	receiveErrors := make(chan error, 1)
	go func() { receiveErrors <- s.receiveLoop(session, certificate, stream) }()
	sendErrors := make(chan error, 1)
	go func() { sendErrors <- s.sendLoop(session, stream) }()
	ticker := time.NewTicker(s.recheckInterval)
	defer ticker.Stop()

	for {
		select {
		case err := <-receiveErrors:
			if err == nil || errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case err := <-sendErrors:
			return err
		case <-ticker.C:
			if _, _, err := s.verifier.VerifyCertificate(stream.Context(), certificate); err != nil {
				return status.Error(codes.Unauthenticated, "device certificate is no longer eligible")
			}
			if time.Since(session.lastHeartbeat()) >= s.staleAfter {
				return status.Error(codes.DeadlineExceeded, "device heartbeat is stale")
			}
		case <-session.ctx.Done():
			return status.Error(codes.Aborted, "device session was replaced")
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func (s *Server) receiveLoop(session *activeSession, certificate *x509.Certificate, stream grpc.BidiStreamingServer[devicev1.ConnectRequest, devicev1.ConnectResponse]) error {
	for {
		request, err := stream.Recv()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		switch payload := request.Payload.(type) {
		case *devicev1.ConnectRequest_Heartbeat:
			if !validHeartbeat(payload.Heartbeat, now) {
				return status.Error(codes.InvalidArgument, "heartbeat is invalid")
			}
			session.touch(now)
		case *devicev1.ConnectRequest_ObservedState:
			if err := s.reverify(stream.Context(), certificate); err != nil {
				return err
			}
			if err := validateObservedState(payload.ObservedState); err != nil {
				return status.Error(codes.InvalidArgument, "observed state is invalid")
			}
			if session.duplicates.seen(payload.ObservedState.MessageId) {
				if err := session.enqueueAck(payload.ObservedState.MessageId, devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_OBSERVED_STATE, payload.ObservedState.ObservedRevision); err != nil {
					return status.Error(codes.ResourceExhausted, "device channel is saturated")
				}
				continue
			}
			if s.reconciliation == nil {
				return status.Error(codes.Unimplemented, "observed-state handler is unavailable")
			}
			if err := s.reconciliation.ObservedState(stream.Context(), session.identity, payload.ObservedState); err != nil {
				return status.Error(codes.Internal, "observed-state ingest failed")
			}
			session.duplicates.add(payload.ObservedState.MessageId)
			if err := session.enqueueAck(payload.ObservedState.MessageId, devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_OBSERVED_STATE, payload.ObservedState.ObservedRevision); err != nil {
				return status.Error(codes.ResourceExhausted, "device channel is saturated")
			}
		case *devicev1.ConnectRequest_HealthReport:
			if err := s.reverify(stream.Context(), certificate); err != nil {
				return err
			}
			if err := validateHealthReport(payload.HealthReport); err != nil {
				return status.Error(codes.InvalidArgument, "health report is invalid")
			}
			if !s.healthRate.allow(session.identity.DeviceID, now) {
				return status.Error(codes.ResourceExhausted, "health report rate exceeded")
			}
			if session.duplicates.seen(payload.HealthReport.ReportId) {
				if err := session.enqueueAck(payload.HealthReport.ReportId, devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_HEALTH_REPORT, payload.HealthReport.Sequence); err != nil {
					return status.Error(codes.ResourceExhausted, "device channel is saturated")
				}
				continue
			}
			if s.health == nil {
				return status.Error(codes.Unimplemented, "health-report handler is unavailable")
			}
			if err := s.health.HealthReport(stream.Context(), session.identity, payload.HealthReport); err != nil {
				return status.Error(codes.Internal, "health-report ingest failed")
			}
			session.duplicates.add(payload.HealthReport.ReportId)
			if err := session.enqueueAck(payload.HealthReport.ReportId, devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_HEALTH_REPORT, payload.HealthReport.Sequence); err != nil {
				return status.Error(codes.ResourceExhausted, "device channel is saturated")
			}
		case *devicev1.ConnectRequest_Acknowledgement:
			if err := s.reverify(stream.Context(), certificate); err != nil {
				return err
			}
			if err := validateAcknowledgement(payload.Acknowledgement); err != nil {
				return status.Error(codes.InvalidArgument, "acknowledgement is invalid")
			}
			if s.reconciliation == nil {
				return status.Error(codes.Unimplemented, "desired acknowledgement handler is unavailable")
			}
			if err := s.reconciliation.Acknowledgement(stream.Context(), session.identity, payload.Acknowledgement); err != nil {
				return status.Error(codes.Internal, "acknowledgement ingest failed")
			}
		default:
			return status.Error(codes.InvalidArgument, "unexpected device channel frame")
		}
	}
}

func (s *Server) sendLoop(session *activeSession, stream grpc.BidiStreamingServer[devicev1.ConnectRequest, devicev1.ConnectResponse]) error {
	for {
		select {
		case item := <-session.outgoing.items:
			err := stream.Send(item.frame)
			session.outgoing.sent(item)
			if err != nil {
				return err
			}
		case <-session.ctx.Done():
			return session.ctx.Err()
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func (s *Server) reverify(ctx context.Context, certificate *x509.Certificate) error {
	if _, _, err := s.verifier.VerifyCertificate(ctx, certificate); err != nil {
		return status.Error(codes.Unauthenticated, "device certificate is no longer eligible")
	}
	return nil
}

func peerCertificate(ctx context.Context) (*x509.Certificate, error) {
	peerInfo, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("peer is unavailable")
	}
	tlsInfo, ok := peerInfo.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.VerifiedChains) != 1 || len(tlsInfo.State.VerifiedChains[0]) == 0 {
		return nil, errors.New("verified client chain is unavailable")
	}
	return tlsInfo.State.VerifiedChains[0][0], nil
}

func receiveHello(stream grpc.BidiStreamingServer[devicev1.ConnectRequest, devicev1.ConnectResponse], timeout time.Duration) (*devicev1.ConnectRequest, error) {
	type result struct {
		request *devicev1.ConnectRequest
		err     error
	}
	resultChannel := make(chan result, 1)
	go func() {
		request, err := stream.Recv()
		resultChannel <- result{request: request, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case received := <-resultChannel:
		return received.request, received.err
	case <-timer.C:
		return nil, status.Error(codes.DeadlineExceeded, "device hello deadline exceeded")
	case <-stream.Context().Done():
		return nil, stream.Context().Err()
	}
}

type activeSession struct {
	identity      DeviceIdentity
	ctx           context.Context
	cancel        context.CancelFunc
	outgoing      *responseQueue
	heartbeatUnix atomic.Int64
	duplicates    recentIDs
}

func newActiveSession(parent context.Context, identity DeviceIdentity) *activeSession {
	ctx, cancel := context.WithCancel(parent)
	session := &activeSession{identity: identity, ctx: ctx, cancel: cancel, outgoing: newResponseQueue()}
	session.heartbeatUnix.Store(time.Now().UTC().UnixNano())
	session.duplicates.values = make(map[string]struct{})
	return session
}

func (s *activeSession) touch(now time.Time)      { s.heartbeatUnix.Store(now.UnixNano()) }
func (s *activeSession) lastHeartbeat() time.Time { return time.Unix(0, s.heartbeatUnix.Load()) }

func (s *activeSession) enqueueAck(messageID string, kind devicev1.AcknowledgementKind, revision uint64) error {
	return s.outgoing.enqueue(&devicev1.ConnectResponse{Payload: &devicev1.ConnectResponse_Acknowledgement{
		Acknowledgement: &devicev1.Acknowledgement{MessageId: messageID, Kind: kind, Revision: revision},
	}})
}

type sessionRegistry struct {
	mu     sync.RWMutex
	active map[string]*activeSession
}

func (r *sessionRegistry) activate(session *activeSession) {
	r.mu.Lock()
	previous := r.active[session.identity.DeviceID]
	r.active[session.identity.DeviceID] = session
	r.mu.Unlock()
	if previous != nil {
		previous.cancel()
	}
}

func (r *sessionRegistry) get(deviceID string) *activeSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active[deviceID]
}

func (r *sessionRegistry) remove(session *activeSession) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active[session.identity.DeviceID] == session {
		delete(r.active, session.identity.DeviceID)
		return true
	}
	return false
}

type recentIDs struct {
	mu     sync.Mutex
	order  []string
	values map[string]struct{}
}

func (r *recentIDs) seen(value string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.values[value]
	return ok
}

func (r *recentIDs) add(value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.values[value]; ok {
		return
	}
	const maximum = 256
	if len(r.order) == maximum {
		delete(r.values, r.order[0])
		r.order = r.order[1:]
	}
	r.order = append(r.order, value)
	r.values[value] = struct{}{}
}

type tokenBucket struct {
	tokens   float64
	updated  time.Time
	lastSeen time.Time
}

type healthRateLimiter struct {
	mu      sync.Mutex
	devices map[string]*tokenBucket
}

func (l *healthRateLimiter) allow(deviceID string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, bucket := range l.devices {
		if now.Sub(bucket.lastSeen) > 10*time.Minute {
			delete(l.devices, key)
		}
	}
	bucket := l.devices[deviceID]
	if bucket == nil {
		bucket = &tokenBucket{tokens: 3, updated: now}
		l.devices[deviceID] = bucket
	}
	elapsed := now.Sub(bucket.updated).Seconds()
	bucket.tokens = min(3, bucket.tokens+elapsed*(6.0/60.0))
	bucket.updated = now
	bucket.lastSeen = now
	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--
	return true
}
