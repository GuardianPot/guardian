package devicepki

import (
	"crypto/tls"
	"errors"
	"net"
	"testing"
)

func TestDevicePKIEnrollmentReplayRotationAndRevocation(t *testing.T) {
	authority, err := NewTestAuthority()
	if err != nil {
		t.Fatal(err)
	}

	challenge, err := authority.IssueChallenge("device-001")
	if err != nil {
		t.Fatal(err)
	}
	deviceKey, request, err := NewDeviceRequest("device-001", challenge.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := authority.Enroll(request)
	if err != nil {
		t.Fatalf("Enroll() error = %v", err)
	}
	if err := authority.VerifyCertificate(certificate); err != nil {
		t.Fatalf("new certificate verification error = %v", err)
	}
	if _, err := authority.Enroll(request); !errors.Is(err, ErrChallengeConsumed) {
		t.Fatalf("replayed enrollment error = %v, want ErrChallengeConsumed", err)
	}

	rotationChallenge, err := authority.IssueChallenge("device-001")
	if err != nil {
		t.Fatal(err)
	}
	rotatedKey, rotationRequest, err := NewDeviceRequest("device-001", rotationChallenge.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := authority.Rotate("device-001", certificate, rotationRequest)
	if err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}
	if err := authority.VerifyCertificate(rotated); err != nil {
		t.Fatalf("rotated certificate verification error = %v", err)
	}
	if err := authority.VerifyCertificate(certificate); !errors.Is(err, ErrRevoked) {
		t.Fatalf("old certificate verification error = %v, want ErrRevoked", err)
	}
	if err := authority.Revoke("device-001", rotated); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if err := authority.Revoke("device-001", rotated); err != nil {
		t.Fatalf("idempotent Revoke() error = %v", err)
	}
	if err := authority.VerifyCertificate(rotated); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked certificate verification error = %v, want ErrRevoked", err)
	}
	if deviceKey == nil || rotatedKey == nil {
		t.Fatal("device key material was not generated")
	}
}

func TestDevicePKIRejectsForgedProofWithoutConsumingChallenge(t *testing.T) {
	authority, err := NewTestAuthority()
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := authority.IssueChallenge("device-forged")
	if err != nil {
		t.Fatal(err)
	}
	_, request, err := NewDeviceRequest("device-forged", challenge.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	request.Signature[0] ^= 0xff
	if _, err := authority.Enroll(request); !errors.Is(err, ErrInvalidProof) {
		t.Fatalf("forged proof error = %v, want ErrInvalidProof", err)
	}
	if _, err := authority.Enroll(request); !errors.Is(err, ErrInvalidProof) {
		t.Fatalf("forged proof retry error = %v, want ErrInvalidProof", err)
	}
	// The challenge remains available, but the request signature must be
	// restored by the device before enrollment can succeed.
	_, validRequest, err := NewDeviceRequest("device-forged", challenge.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.Enroll(validRequest); err != nil {
		t.Fatalf("valid proof after forgery error = %v", err)
	}
}

func TestDevicePKIMTLS13RequiresActiveDeviceCertificate(t *testing.T) {
	authority, err := NewTestAuthority()
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := authority.IssueChallenge("device-tls")
	if err != nil {
		t.Fatal(err)
	}
	deviceKey, request, err := NewDeviceRequest("device-tls", challenge.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	deviceCertificate, err := authority.Enroll(request)
	if err != nil {
		t.Fatal(err)
	}
	serverCertificate, err := authority.IssueServerIdentity("localhost")
	if err != nil {
		t.Fatal(err)
	}
	clientConfig, err := authority.ClientTLSConfig(deviceCertificate, deviceKey, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := authority.ServerTLSConfig(serverCertificate)
	serverState, clientState, serverErr, clientErr := handshakePair(serverConfig, clientConfig)
	if serverErr != nil || clientErr != nil {
		t.Fatalf("mTLS handshake failed: server=%v client=%v", serverErr, clientErr)
	}
	if serverState.Version != tls.VersionTLS13 || clientState.Version != tls.VersionTLS13 {
		t.Fatalf("TLS versions = server %x/client %x, want TLS 1.3", serverState.Version, clientState.Version)
	}

	if err := authority.Revoke("device-tls", deviceCertificate); err != nil {
		t.Fatal(err)
	}
	_, _, serverErr, clientErr = handshakePair(serverConfig, clientConfig)
	if serverErr == nil && clientErr == nil {
		t.Fatal("revoked device certificate completed mTLS handshake")
	}
}

func handshakePair(serverConfig, clientConfig *tls.Config) (tls.ConnectionState, tls.ConnectionState, error, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return tls.ConnectionState{}, tls.ConnectionState{}, err, nil
	}
	defer listener.Close()
	type serverResult struct {
		state tls.ConnectionState
		err   error
	}
	serverResults := make(chan serverResult, 1)
	go func() {
		raw, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverResults <- serverResult{err: acceptErr}
			return
		}
		server := tls.Server(raw, serverConfig.Clone())
		handshakeErr := server.Handshake()
		state := server.ConnectionState()
		_ = raw.Close()
		serverResults <- serverResult{state: state, err: handshakeErr}
	}()
	raw, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		return tls.ConnectionState{}, tls.ConnectionState{}, err, nil
	}
	client := tls.Client(raw, clientConfig.Clone())
	clientErr := client.Handshake()
	clientState := client.ConnectionState()
	_ = raw.Close()
	server := <-serverResults
	return server.state, clientState, server.err, clientErr
}
