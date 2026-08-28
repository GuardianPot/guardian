package devicechannel

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

const edgeTestDeviceID = "0198f7c4-7b30-7f11-8a44-111111111111"

func TestClientNegotiatesAcknowledgesAndReconnectsAfterControlRestart(t *testing.T) {
	fixture := newEdgeTLSFixture(t)
	server := startEdgeTestServer(t, "127.0.0.1:0", fixture, true)
	states := &stateCapture{changed: make(chan stateChange, 32)}
	acknowledgements := make(chan *devicev1.Acknowledgement, 4)
	client, err := NewClient(Config{
		Endpoint: server.address(), AgentVersion: "guardian-edge/test", Credentials: fixture.client,
		RootCAs: fixture.roots, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		StateRecorder: states, AcknowledgementHandler: acknowledgementHandlerFunc(func(_ context.Context, ack *devicev1.Acknowledgement) error {
			acknowledgements <- ack
			return nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	client.heartbeatInterval = 10 * time.Millisecond
	client.stableAfter = 30 * time.Millisecond
	client.helloTimeout = time.Second
	client.jitter = func(time.Duration) time.Duration { return time.Millisecond }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	waitForState(t, states.changed, "connected", time.Second)

	messageID := "0198f7c4-7b30-7f11-8a44-222222222222"
	if err := client.EnqueueObserved(messageID, 7, 7); err != nil {
		t.Fatal(err)
	}
	select {
	case ack := <-acknowledgements:
		if ack.MessageId != messageID || ack.Revision != 7 {
			t.Fatalf("acknowledgement = %+v", ack)
		}
	case <-time.After(time.Second):
		t.Fatal("observed-state acknowledgement was not received")
	}

	address := server.address()
	server.stop()
	waitForState(t, states.changed, "connecting", time.Second)
	replacement := startEdgeTestServer(t, address, fixture, true)
	defer replacement.stop()
	waitForState(t, states.changed, "connected", 2*time.Second)

	stopContext, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := client.Stop(stopContext); err != nil {
		t.Fatal(err)
	}
	if client.ConnectionState() != "stopped" {
		t.Fatalf("final state = %s", client.ConnectionState())
	}

	restartedStates := &stateCapture{changed: make(chan stateChange, 8)}
	restarted, err := NewClient(Config{
		Endpoint: replacement.address(), AgentVersion: "guardian-edge/test", Credentials: fixture.client,
		RootCAs: fixture.roots, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), StateRecorder: restartedStates,
	})
	if err != nil {
		t.Fatal(err)
	}
	restarted.helloTimeout = time.Second
	restartedContext, cancelRestarted := context.WithCancel(context.Background())
	defer cancelRestarted()
	if err := restarted.Start(restartedContext); err != nil {
		t.Fatal(err)
	}
	waitForState(t, restartedStates.changed, "connected", time.Second)
	restartedStopContext, cancelRestartedStop := context.WithTimeout(context.Background(), time.Second)
	defer cancelRestartedStop()
	if err := restarted.Stop(restartedStopContext); err != nil {
		t.Fatal(err)
	}
}

func TestStartFailureDoesNotLeaveClientRunning(t *testing.T) {
	fixture := newEdgeTLSFixture(t)
	client, err := NewClient(Config{
		Endpoint: "127.0.0.1:8443", AgentVersion: "guardian-edge/test", Credentials: fixture.client,
		RootCAs: fixture.roots, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		StateRecorder: StateRecorderFunc(func(context.Context, string, string) error {
			return errors.New("storage unavailable")
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err == nil {
		t.Fatal("start succeeded with a failing state recorder")
	}
	client.mu.RLock()
	started, cancel, done := client.started, client.cancel, client.done
	client.mu.RUnlock()
	if started || cancel != nil || done != nil {
		t.Fatalf("failed start left lifecycle state: started=%t cancel=%v done=%v", started, cancel != nil, done != nil)
	}
}

func TestClientRecordsProtocolIncompatibility(t *testing.T) {
	fixture := newEdgeTLSFixture(t)
	server := startEdgeTestServer(t, "127.0.0.1:0", fixture, false)
	defer server.stop()
	states := &stateCapture{changed: make(chan stateChange, 32)}
	client, err := NewClient(Config{
		Endpoint: server.address(), AgentVersion: "guardian-edge/test", Credentials: fixture.client,
		RootCAs: fixture.roots, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), StateRecorder: states,
	})
	if err != nil {
		t.Fatal(err)
	}
	client.jitter = func(time.Duration) time.Duration { return 10 * time.Millisecond }
	client.helloTimeout = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(time.Second)
	for {
		select {
		case state := <-states.changed:
			if state.reason == "protocol_incompatible" {
				stopContext, stopCancel := context.WithTimeout(context.Background(), time.Second)
				defer stopCancel()
				if err := client.Stop(stopContext); err != nil {
					t.Fatal(err)
				}
				return
			}
		case <-deadline:
			t.Fatal("protocol incompatibility was not recorded")
		}
	}
}

func TestRequestQueueAndBackoffAreBounded(t *testing.T) {
	queue := newRequestQueue()
	for index := 0; index < MaxQueueFrames; index++ {
		if err := queue.enqueue(observedFrame(uint64(index + 1))); err != nil {
			t.Fatalf("enqueue %d: %v", index, err)
		}
	}
	if err := queue.enqueue(observedFrame(65)); !errors.Is(err, ErrBackpressure) {
		t.Fatalf("saturated queue error = %v", err)
	}
	if backoff(1) != time.Second || backoff(7) != 60*time.Second || backoff(30) != 60*time.Second {
		t.Fatalf("backoff sequence = %s %s %s", backoff(1), backoff(7), backoff(30))
	}
	for index := 0; index < 100; index++ {
		value := fullJitter(10 * time.Millisecond)
		if value < 0 || value > 10*time.Millisecond {
			t.Fatalf("full jitter = %s", value)
		}
	}
}

func observedFrame(revision uint64) *devicev1.ConnectRequest {
	return &devicev1.ConnectRequest{Payload: &devicev1.ConnectRequest_ObservedState{ObservedState: &devicev1.ObservedState{
		MessageId: "0198f7c4-7b30-7f11-8a44-222222222222", DesiredRevision: revision, ObservedRevision: revision,
	}}}
}

type stateChange struct{ state, reason string }

type stateCapture struct {
	mu      sync.Mutex
	changed chan stateChange
}

func (capture *stateCapture) SetChannelState(_ context.Context, state, reason string) error {
	capture.mu.Lock()
	capture.changed <- stateChange{state: state, reason: reason}
	capture.mu.Unlock()
	return nil
}

func waitForState(t *testing.T, changes <-chan stateChange, wanted string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case change := <-changes:
			if change.state == wanted {
				return
			}
		case <-deadline:
			t.Fatalf("state %q was not observed", wanted)
		}
	}
}

type acknowledgementHandlerFunc func(context.Context, *devicev1.Acknowledgement) error

func (function acknowledgementHandlerFunc) Acknowledgement(ctx context.Context, ack *devicev1.Acknowledgement) error {
	return function(ctx, ack)
}

type edgeTLSFixture struct {
	ca     *x509.Certificate
	caKey  *ecdsa.PrivateKey
	server tls.Certificate
	client identity.Credentials
	roots  *x509.CertPool
}

func newEdgeTLSFixture(t *testing.T) edgeTLSFixture {
	t.Helper()
	now := time.Now().UTC()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Guardian Edge test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	ca, _ := x509.ParseCertificate(caDER)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	serverKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serverTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	serverDER, _ := x509.CreateCertificate(rand.Reader, serverTemplate, ca, &serverKey.PublicKey, caKey)
	serverKeyDER, _ := x509.MarshalPKCS8PrivateKey(serverKey)
	server, err := tls.X509KeyPair(append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}), caPEM...), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: serverKeyDER}))
	if err != nil {
		t.Fatal(err)
	}

	clientKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	deviceURI, _ := url.Parse("urn:guardian:device:" + edgeTestDeviceID)
	clientTemplate := &x509.Certificate{SerialNumber: big.NewInt(3), URIs: []*url.URL{deviceURI},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	clientDER, _ := x509.CreateCertificate(rand.Reader, clientTemplate, ca, &clientKey.PublicKey, caKey)
	clientKeyDER, _ := x509.MarshalPKCS8PrivateKey(clientKey)
	clientPair, err := tls.X509KeyPair(append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}), caPEM...), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKeyDER}))
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(clientDER)
	clientPair.Leaf = leaf
	sha := sha256Sum(leaf.Raw)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return edgeTLSFixture{ca: ca, caKey: caKey, server: server, roots: roots, client: identity.Credentials{
		Certificate: clientPair, Leaf: leaf, Metadata: identity.Metadata{DeviceID: edgeTestDeviceID,
			CertificateSerial: leaf.SerialNumber.Text(16), CertificateSHA256: sha, NotBefore: leaf.NotBefore.UTC(), NotAfter: leaf.NotAfter.UTC()},
	}}
}

func sha256Sum(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

type edgeTestServer struct {
	devicev1.UnimplementedDeviceChannelServiceServer
	listener net.Listener
	server   *grpc.Server
	accept   bool
	stopOnce sync.Once
}

func startEdgeTestServer(t *testing.T, address string, fixture edgeTLSFixture, accept bool) *edgeTestServer {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	clientRoots := x509.NewCertPool()
	clientRoots.AddCert(fixture.ca)
	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{fixture.server},
		ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots,
	})))
	server := &edgeTestServer{listener: listener, server: grpcServer, accept: accept}
	devicev1.RegisterDeviceChannelServiceServer(grpcServer, server)
	go grpcServer.Serve(listener)
	t.Cleanup(server.stop)
	return server
}

func (s *edgeTestServer) address() string { return s.listener.Addr().String() }

func (s *edgeTestServer) stop() {
	s.stopOnce.Do(func() {
		s.server.Stop()
		_ = s.listener.Close()
	})
}

func (s *edgeTestServer) Connect(stream grpc.BidiStreamingServer[devicev1.ConnectRequest, devicev1.ConnectResponse]) error {
	first, err := stream.Recv()
	if err != nil || first.GetHello() == nil {
		return status.Error(codes.InvalidArgument, "hello required")
	}
	selection := &devicev1.ProtocolSelection{Selected: &devicev1.ProtocolVersion{Major: ProtocolMajor, Minor: ProtocolMinor}, Accepted: s.accept}
	if !s.accept {
		selection.Reason = "protocol_incompatible"
		_ = stream.Send(&devicev1.ConnectResponse{Payload: &devicev1.ConnectResponse_ProtocolSelection{ProtocolSelection: selection}})
		return status.Error(codes.FailedPrecondition, "protocol incompatible")
	}
	if err := stream.Send(&devicev1.ConnectResponse{Payload: &devicev1.ConnectResponse_ProtocolSelection{ProtocolSelection: selection}}); err != nil {
		return err
	}
	for {
		request, err := stream.Recv()
		if err != nil {
			return err
		}
		if observed := request.GetObservedState(); observed != nil {
			if err := stream.Send(&devicev1.ConnectResponse{Payload: &devicev1.ConnectResponse_Acknowledgement{Acknowledgement: &devicev1.Acknowledgement{
				MessageId: observed.MessageId, Kind: devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_OBSERVED_STATE, Revision: observed.ObservedRevision,
			}}}); err != nil {
				return err
			}
		}
	}
}
