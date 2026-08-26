package api

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
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type readinessFunc func(context.Context) error

func (fn readinessFunc) Ready(ctx context.Context) error { return fn(ctx) }

func TestHealthEndpointsAreTruthfulAndSanitized(t *testing.T) {
	secret := "postgres://guardian:do-not-leak@db/guardian"
	server := NewServer("127.0.0.1:0", readinessFunc(func(context.Context) error {
		return errors.New(secret)
	}), slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, test := range []struct {
		path       string
		statusCode int
		status     string
	}{
		{path: "/livez", statusCode: http.StatusOK, status: "live"},
		{path: "/readyz", statusCode: http.StatusServiceUnavailable, status: "not_ready"},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != test.statusCode || !strings.Contains(response.Body.String(), test.status) {
			t.Fatalf("GET %s = %d %q", test.path, response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), secret) {
			t.Fatalf("GET %s leaked an internal database error", test.path)
		}
	}
}

func TestReadyEndpointReturnsReady(t *testing.T) {
	server := NewServer("127.0.0.1:0", readinessFunc(func(context.Context) error { return nil }), slog.Default())
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "ready") {
		t.Fatalf("GET /readyz = %d %q", response.Code, response.Body.String())
	}
}

func TestStartPreservesInitialBindFailure(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	server := NewServer(occupied.Addr().String(), readinessFunc(func(context.Context) error { return nil }), slog.Default())
	first := server.Start()
	second := server.Start()
	if first == nil || second == nil {
		t.Fatalf("Start() errors = (%v, %v), want persistent bind failure", first, second)
	}
}

func TestTLSListenerVerifiesDeviceChainAndRequiresTLS13(t *testing.T) {
	now := time.Now().UTC()
	deviceCA, deviceCAKey, deviceCAPEM, _ := apiTestCertificateAuthority(t, "device CA", now)
	serverCA, serverCAKey, serverCAPEM, _ := apiTestCertificateAuthority(t, "server CA", now)
	serverCertificatePEM, serverKeyPEM := apiTestLeaf(t, serverCA, serverCAKey, now, false)
	clientCertificatePEM, clientKeyPEM := apiTestLeaf(t, deviceCA, deviceCAKey, now, true)
	directory := t.TempDir()
	serverCertificateFile := filepath.Join(directory, "server.crt")
	serverKeyFile := filepath.Join(directory, "server.key")
	if err := os.WriteFile(serverCertificateFile, serverCertificatePEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serverKeyFile, serverKeyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	server := NewServer("127.0.0.1:0", readinessFunc(func(context.Context) error { return nil }), discardLogger(),
		WithDeviceService(deviceServiceStub{}), WithTLSFiles(serverCertificateFile, serverKeyFile, deviceCAPEM))
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Shutdown(context.Background()) }()
	clientIdentity, err := tls.X509KeyPair(append(clientCertificatePEM, deviceCAPEM...), clientKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	serverRoots := x509.NewCertPool()
	if !serverRoots.AppendCertsFromPEM(serverCAPEM) {
		t.Fatal("append server CA")
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS13,
		RootCAs:    serverRoots,
		Certificates: []tls.Certificate{
			clientIdentity,
		},
	}}}
	request, err := http.NewRequest(http.MethodPost, "https://"+server.Address()+"/v1/device-certificates:rotate", strings.NewReader(`{"csr_pem":"csr"}`))
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated || response.TLS == nil || response.TLS.Version != tls.VersionTLS13 {
		t.Fatalf("mTLS rotation response = %d TLS=%+v", response.StatusCode, response.TLS)
	}
}

func apiTestCertificateAuthority(t *testing.T, name string, now time.Time) (*x509.Certificate, *ecdsa.PrivateKey, []byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: name},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, key,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func apiTestLeaf(t *testing.T, authority *x509.Certificate, authorityKey *ecdsa.PrivateKey, now time.Time, client bool) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature,
	}
	if client {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, authority, &key.PublicKey, authorityKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}
