package devicechannel

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel/gen/guardian/device/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const testDeviceID = "0198f7c4-7b30-7f11-8a44-111111111111"
const otherDeviceID = "0198f7c4-7b30-7f11-8a44-222222222222"

func TestAuthenticatedNegotiationDuplicateReplacementAndRevocation(t *testing.T) {
	fixture := newTLSFixture(t)
	verifier := &testVerifier{deviceID: testDeviceID, serial: fixture.clientLeaf.SerialNumber.Text(16)}
	verifier.eligible.Store(true)
	server := newTestServer(t, fixture, verifier)
	server.recheckInterval = 10 * time.Millisecond
	server.staleAfter = time.Second

	first := openTestStream(t, server.Address(), fixture)
	negotiate(t, first, ProtocolMajor, ProtocolMinor, true)
	second := openTestStream(t, server.Address(), fixture)
	negotiate(t, second, ProtocolMajor, ProtocolMinor, true)
	if _, err := first.Recv(); status.Code(err) != codes.Aborted {
		t.Fatalf("replaced stream error = %v", err)
	}

	verifier.eligible.Store(false)
	if _, err := second.Recv(); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("revoked stream error = %v", err)
	}
}

func TestProtocolMismatchAndHeartbeatStalenessFailClosed(t *testing.T) {
	fixture := newTLSFixture(t)
	verifier := &testVerifier{deviceID: testDeviceID, serial: fixture.clientLeaf.SerialNumber.Text(16)}
	verifier.eligible.Store(true)
	server := newTestServer(t, fixture, verifier)

	mismatch := openTestStream(t, server.Address(), fixture)
	negotiate(t, mismatch, ProtocolMajor+1, 0, false)
	if _, err := mismatch.Recv(); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("protocol mismatch error = %v", err)
	}

	server.recheckInterval = 5 * time.Millisecond
	server.staleAfter = 20 * time.Millisecond
	stale := openTestStream(t, server.Address(), fixture)
	negotiate(t, stale, ProtocolMajor, ProtocolMinor, true)
	if _, err := stale.Recv(); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("stale stream error = %v", err)
	}
}

func TestInvalidClientCAIsRejectedBeforeApplicationFrames(t *testing.T) {
	fixture := newTLSFixture(t)
	other := newTLSFixture(t)
	other.roots = fixture.roots
	verifier := &testVerifier{deviceID: testDeviceID, serial: fixture.clientLeaf.SerialNumber.Text(16)}
	verifier.eligible.Store(true)
	server := newTestServer(t, fixture, verifier)

	connection := dialTestConnection(t, server.Address(), other)
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stream, err := devicev1.NewDeviceChannelServiceClient(connection).Connect(ctx)
	if err == nil {
		err = stream.Send(helloFrame(ProtocolMajor, ProtocolMinor))
		if err == nil {
			_, err = stream.Recv()
		}
	}
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("untrusted client error = %v", err)
	}
}

func TestExpiredCertificateAndIdentityMismatchFailClosed(t *testing.T) {
	fixture := newTLSFixture(t)
	now := time.Now().UTC()

	expired := fixture
	expired.clientCertificate, expired.clientLeaf = newClientIdentity(
		t, fixture.ca, fixture.caKey, testDeviceID, now.Add(-2*time.Hour), now.Add(-time.Hour),
	)
	expiredVerifier := &testVerifier{deviceID: testDeviceID, serial: expired.clientLeaf.SerialNumber.Text(16)}
	expiredVerifier.eligible.Store(true)
	expiredServer := newTestServer(t, fixture, expiredVerifier)
	expiredConnection := dialTestConnection(t, expiredServer.Address(), expired)
	defer expiredConnection.Close()
	expiredContext, cancelExpired := context.WithTimeout(context.Background(), time.Second)
	defer cancelExpired()
	expiredStream, err := devicev1.NewDeviceChannelServiceClient(expiredConnection).Connect(expiredContext)
	if err == nil {
		err = expiredStream.Send(helloFrame(ProtocolMajor, ProtocolMinor))
		if err == nil {
			_, err = expiredStream.Recv()
		}
	}
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expired client error = %v", err)
	}

	mismatch := fixture
	mismatch.clientCertificate, mismatch.clientLeaf = newClientIdentity(
		t, fixture.ca, fixture.caKey, otherDeviceID, now.Add(-time.Hour), now.Add(time.Hour),
	)
	mismatchVerifier := &testVerifier{deviceID: testDeviceID, serial: mismatch.clientLeaf.SerialNumber.Text(16)}
	mismatchVerifier.eligible.Store(true)
	mismatchServer := newTestServer(t, fixture, mismatchVerifier)
	mismatchStream := openTestStream(t, mismatchServer.Address(), mismatch)
	if err := mismatchStream.Send(helloFrame(ProtocolMajor, ProtocolMinor)); err != nil && !errors.Is(err, io.EOF) {
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("identity mismatch send error = %v", err)
		}
		return
	}
	if _, err := mismatchStream.Recv(); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("identity mismatch error = %v", err)
	}
}

func TestMalformedOversizedAndCancelledStreamsFailClosed(t *testing.T) {
	fixture := newTLSFixture(t)
	verifier := &testVerifier{deviceID: testDeviceID, serial: fixture.clientLeaf.SerialNumber.Text(16)}
	verifier.eligible.Store(true)
	server := newTestServer(t, fixture, verifier)

	malformed := openTestStream(t, server.Address(), fixture)
	if err := malformed.Send(&devicev1.ConnectRequest{Payload: &devicev1.ConnectRequest_Heartbeat{Heartbeat: &devicev1.Heartbeat{SentAt: timestamppb.Now()}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := malformed.Recv(); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("malformed first frame error = %v", err)
	}

	oversized := openTestStream(t, server.Address(), fixture)
	if err := oversized.Send(&devicev1.ConnectRequest{Payload: &devicev1.ConnectRequest_Hello{Hello: &devicev1.EdgeHello{
		Protocol:     &devicev1.ProtocolVersion{Major: ProtocolMajor, Minor: ProtocolMinor},
		AgentVersion: strings.Repeat("x", MaxMessageBytes+1),
	}}}); err != nil && status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("oversized send error = %v", err)
	}
	if _, err := oversized.Recv(); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("oversized receive error = %v", err)
	}

	connection := dialTestConnection(t, server.Address(), fixture)
	defer connection.Close()
	cancelContext, cancel := context.WithCancel(context.Background())
	cancelled, err := devicev1.NewDeviceChannelServiceClient(connection).Connect(cancelContext)
	if err != nil {
		t.Fatal(err)
	}
	negotiate(t, cancelled, ProtocolMajor, ProtocolMinor, true)
	cancel()
	if _, err := cancelled.Recv(); status.Code(err) != codes.Canceled {
		t.Fatalf("cancelled stream error = %v", err)
	}
}

func TestQueueAndHealthRateAreBounded(t *testing.T) {
	queue := newResponseQueue()
	for index := 0; index < MaxQueueFrames; index++ {
		frame := &devicev1.ConnectResponse{Payload: &devicev1.ConnectResponse_DesiredState{DesiredState: &devicev1.DesiredStateSnapshot{
			MessageId: "0198f7c4-7b30-7f11-8a44-111111111111", Revision: uint64(index + 1),
		}}}
		if err := queue.enqueue(frame); err != nil {
			t.Fatalf("enqueue %d: %v", index, err)
		}
	}
	if err := queue.enqueue(&devicev1.ConnectResponse{Payload: &devicev1.ConnectResponse_DesiredState{DesiredState: &devicev1.DesiredStateSnapshot{MessageId: testDeviceID, Revision: 65}}}); !errors.Is(err, ErrBackpressure) {
		t.Fatalf("saturated queue error = %v", err)
	}

	var limiter healthRateLimiter
	limiter.devices = make(map[string]*tokenBucket)
	now := time.Now().UTC()
	for index := 0; index < 3; index++ {
		if !limiter.allow(testDeviceID, now) {
			t.Fatalf("burst token %d rejected", index)
		}
	}
	if limiter.allow(testDeviceID, now) {
		t.Fatal("fourth immediate health report was accepted")
	}
	if !limiter.allow(testDeviceID, now.Add(10*time.Second)) {
		t.Fatal("refilled health token was rejected")
	}
}

func TestReceiveValidHeartbeatKeepsSessionFresh(t *testing.T) {
	fixture := newTLSFixture(t)
	verifier := &testVerifier{deviceID: testDeviceID, serial: fixture.clientLeaf.SerialNumber.Text(16)}
	verifier.eligible.Store(true)
	server := newTestServer(t, fixture, verifier)
	server.recheckInterval = 5 * time.Millisecond
	server.staleAfter = 40 * time.Millisecond
	stream := openTestStream(t, server.Address(), fixture)
	negotiate(t, stream, ProtocolMajor, ProtocolMinor, true)
	for index := 0; index < 3; index++ {
		time.Sleep(15 * time.Millisecond)
		if err := stream.Send(&devicev1.ConnectRequest{Payload: &devicev1.ConnectRequest_Heartbeat{Heartbeat: &devicev1.Heartbeat{SentAt: timestamppb.Now()}}}); err != nil {
			t.Fatal(err)
		}
	}
}

type testVerifier struct {
	deviceID string
	serial   string
	eligible atomic.Bool
}

func (v *testVerifier) VerifyCertificate(_ context.Context, certificate *x509.Certificate) (string, string, error) {
	if !v.eligible.Load() || certificate == nil || certificate.SerialNumber.Text(16) != v.serial {
		return "", "", errors.New("ineligible")
	}
	if len(certificate.URIs) != 1 || certificate.URIs[0].String() != "urn:guardian:device:"+v.deviceID {
		return "", "", errors.New("identity mismatch")
	}
	return v.deviceID, v.serial, nil
}

type tlsFixture struct {
	caPEM             []byte
	serverCertificate string
	serverKey         string
	clientCertificate tls.Certificate
	clientLeaf        *x509.Certificate
	roots             *x509.CertPool
	ca                *x509.Certificate
	caKey             *ecdsa.PrivateKey
}

func newTLSFixture(t *testing.T) tlsFixture {
	t.Helper()
	now := time.Now().UTC()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Guardian test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, _ := x509.ParseCertificate(caDER)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	serverKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, _ := x509.CreateCertificate(rand.Reader, serverTemplate, ca, &serverKey.PublicKey, caKey)
	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
	serverKeyDER, _ := x509.MarshalPKCS8PrivateKey(serverKey)
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: serverKeyDER})

	clientCertificate, clientLeaf := newClientIdentity(t, ca, caKey, testDeviceID, now.Add(-time.Hour), now.Add(time.Hour))
	root := t.TempDir()
	serverCertificate := filepath.Join(root, "server.crt")
	serverPrivateKey := filepath.Join(root, "server.key")
	if err := os.WriteFile(serverCertificate, append(serverCertPEM, caPEM...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serverPrivateKey, serverKeyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return tlsFixture{caPEM: caPEM, serverCertificate: serverCertificate, serverKey: serverPrivateKey, clientCertificate: clientCertificate, clientLeaf: clientLeaf, roots: roots, ca: ca, caKey: caKey}
}

func newClientIdentity(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, deviceID string, notBefore, notAfter time.Time) (tls.Certificate, *x509.Certificate) {
	t.Helper()
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	deviceURI, err := url.Parse("urn:guardian:device:" + deviceID)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatal(err)
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: serial, URIs: []*url.URL{deviceURI}, NotBefore: notBefore, NotAfter: notAfter,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, ca, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	clientKeyDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	clientCertificate, err := tls.X509KeyPair(
		append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})...),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKeyDER}),
	)
	if err != nil {
		t.Fatal(err)
	}
	clientLeaf, err := x509.ParseCertificate(clientDER)
	if err != nil {
		t.Fatal(err)
	}
	clientCertificate.Leaf = clientLeaf
	return clientCertificate, clientLeaf
}

func newTestServer(t *testing.T, fixture tlsFixture, verifier CertificateVerifier) *Server {
	t.Helper()
	server, err := NewServer(Config{
		Address: "127.0.0.1:0", TLSCertificateFile: fixture.serverCertificate,
		TLSPrivateKeyFile: fixture.serverKey, DeviceCAPEM: fixture.caPEM,
		Verifier: verifier, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})
	return server
}

func dialTestConnection(t *testing.T, address string, fixture tlsFixture) *grpc.ClientConn {
	t.Helper()
	connection, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS13, ServerName: "localhost", RootCAs: fixture.roots,
			Certificates: []tls.Certificate{fixture.clientCertificate},
		})),
		grpc.WithDisableRetry(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return connection
}

func openTestStream(t *testing.T, address string, fixture tlsFixture) grpc.BidiStreamingClient[devicev1.ConnectRequest, devicev1.ConnectResponse] {
	t.Helper()
	connection := dialTestConnection(t, address, fixture)
	t.Cleanup(func() { connection.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	stream, err := devicev1.NewDeviceChannelServiceClient(connection).Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return stream
}

func negotiate(t *testing.T, stream grpc.BidiStreamingClient[devicev1.ConnectRequest, devicev1.ConnectResponse], major, minor uint32, accepted bool) {
	t.Helper()
	if err := stream.Send(helloFrame(major, minor)); err != nil {
		t.Fatal(err)
	}
	response, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if selection := response.GetProtocolSelection(); selection == nil || selection.Accepted != accepted {
		t.Fatalf("protocol selection = %+v", response)
	}
}

func helloFrame(major, minor uint32) *devicev1.ConnectRequest {
	return &devicev1.ConnectRequest{Payload: &devicev1.ConnectRequest_Hello{Hello: &devicev1.EdgeHello{
		Protocol: &devicev1.ProtocolVersion{Major: major, Minor: minor}, AgentVersion: "guardian-edge/test",
	}}}
}
