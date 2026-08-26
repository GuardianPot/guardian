package enrollment

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity"
)

func TestClientEnrollsOverTLSWithoutSendingPrivateKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Edge protected-file contract is Linux-only")
	}
	now := time.Now().UTC()
	caCertificate, caKey, caPEM := testCA(t, now)
	const deviceID = "0198dc8c-c600-7000-8000-000000000004"
	const environmentID = "0198dc8c-c600-7000-8000-000000000003"
	const token = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/enrollments" || request.Header.Get("Authorization") != "Bearer "+token {
			t.Fatalf("request = %s authorization=%q", request.URL.Path, request.Header.Get("Authorization"))
		}
		if request.TLS == nil {
			t.Fatal("TLS state is missing")
		}
		if request.TLS.Version != tls.VersionTLS13 {
			t.Fatalf("TLS version = %x", request.TLS.Version)
		}
		var input struct {
			CSRPEM string `json:"csr_pem"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(input.CSRPEM, "PRIVATE KEY") {
			t.Fatal("enrollment request leaked private key")
		}
		csrBlock, _ := pem.Decode([]byte(input.CSRPEM))
		csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
		if err != nil || csr.CheckSignature() != nil {
			t.Fatalf("CSR error = %v", err)
		}
		leafPEM, serial := testDeviceCertificate(t, caCertificate, caKey, csr, deviceID, now)
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		writer.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"contract_version": "guardian.device.v1", "device_id": deviceID,
			"environment_id": environmentID, "certificate_serial": serial,
			"certificate_pem": string(leafPEM), "ca_certificate_pem": string(caPEM),
			"not_before": now.Add(-time.Minute), "not_after": now.Add(30 * 24 * time.Hour),
		})
	}))
	defer server.Close()
	root := filepath.Join(t.TempDir(), "identity")
	certPath, keyPath := filepath.Join(root, "device.crt"), filepath.Join(root, "device.key")
	client := &Client{HTTP: server.Client()}
	result, err := client.Enroll(context.Background(), strings.TrimPrefix(server.URL, "https://"), []byte(token), certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.DeviceID != deviceID || result.EnvironmentID != environmentID || result.Serial == "" {
		t.Fatalf("result = %+v", result)
	}
	if _, err := identity.Load(certPath, keyPath, now); err != nil {
		t.Fatalf("load installed identity: %v", err)
	}
	keyInfo, err := os.Stat(keyPath)
	if err != nil || keyInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("private key permissions = %v, %v", keyInfo.Mode().Perm(), err)
	}
}

func TestClientRotatesWithProductMTLSIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Edge protected-file contract is Linux-only")
	}
	now := time.Now().UTC()
	caCertificate, caKey, caPEM := testCA(t, now)
	const deviceID = "0198dc8c-c600-7000-8000-000000000004"
	const environmentID = "0198dc8c-c600-7000-8000-000000000003"
	_, initialCSRPEM, initialKeyPEM, err := newIdentityRequest()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(initialKeyPEM)
	initialBlock, _ := pem.Decode(initialCSRPEM)
	initialCSR, err := x509.ParseCertificateRequest(initialBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	initialLeaf, initialSerial := testDeviceCertificateWithSerial(t, caCertificate, caKey, initialCSR, deviceID, now, 41)
	root := filepath.Join(t.TempDir(), "identity")
	certPath, keyPath := filepath.Join(root, "device.crt"), filepath.Join(root, "device.key")
	if err := identity.Install(certPath, keyPath, append(initialLeaf, caPEM...), initialKeyPEM); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.TLS == nil || request.TLS.Version != tls.VersionTLS13 || len(request.TLS.VerifiedChains) != 1 {
			t.Fatal("rotation request did not present one verified TLS 1.3 device chain")
		}
		if request.TLS.PeerCertificates[0].SerialNumber.Text(16) != initialSerial {
			t.Fatal("rotation request presented an unexpected active serial")
		}
		var input struct {
			CSRPEM string `json:"csr_pem"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(input.CSRPEM, "PRIVATE KEY") {
			t.Fatal("rotation request leaked private key")
		}
		csrBlock, _ := pem.Decode([]byte(input.CSRPEM))
		csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
		if err != nil || csr.CheckSignature() != nil {
			t.Fatalf("rotation CSR error = %v", err)
		}
		leafPEM, serial := testDeviceCertificateWithSerial(t, caCertificate, caKey, csr, deviceID, now, 42)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"contract_version": "guardian.device.v1", "device_id": deviceID,
			"environment_id": environmentID, "certificate_serial": serial,
			"certificate_pem": string(leafPEM), "ca_certificate_pem": string(caPEM),
			"not_before": now.Add(-time.Minute), "not_after": now.Add(30 * 24 * time.Hour),
		})
	}))
	clientRoots := x509.NewCertPool()
	clientRoots.AddCert(caCertificate)
	server.TLS = &tls.Config{
		MinVersion: tls.VersionTLS13,
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientRoots,
	}
	server.StartTLS()
	defer server.Close()

	client := &Client{HTTP: server.Client()}
	result, err := client.Rotate(context.Background(), strings.TrimPrefix(server.URL, "https://"), certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.DeviceID != deviceID || result.EnvironmentID != environmentID || result.Serial == initialSerial {
		t.Fatalf("rotation result = %+v", result)
	}
	metadata, err := identity.Load(certPath, keyPath, now)
	if err != nil || metadata.CertificateSHA256 == "" {
		t.Fatalf("load rotated identity = %+v %v", metadata, err)
	}
}

func TestValidateRejectsCertificateKeyMismatch(t *testing.T) {
	now := time.Now().UTC()
	caCertificate, caKey, caPEM := testCA(t, now)
	localKey, _, localPrivatePEM, err := newIdentityRequest()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(localPrivatePEM)
	_, otherCSRPEM, _, err := newIdentityRequest()
	if err != nil {
		t.Fatal(err)
	}
	csrBlock, _ := pem.Decode(otherCSRPEM)
	csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	const deviceID = "0198dc8c-c600-7000-8000-000000000004"
	leafPEM, serial := testDeviceCertificate(t, caCertificate, caKey, csr, deviceID, now)
	response := responseEnvelope{
		ContractVersion: "guardian.device.v1", DeviceID: deviceID,
		EnvironmentID:     "0198dc8c-c600-7000-8000-000000000003",
		CertificateSerial: serial, CertificatePEM: string(leafPEM), CACertificatePEM: string(caPEM),
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(30 * 24 * time.Hour),
	}
	root := filepath.Join(t.TempDir(), "identity")
	err = validateAndInstall(response, localKey, localPrivatePEM,
		filepath.Join(root, "device.crt"), filepath.Join(root, "device.key"), now)
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("mismatched response error = %v", err)
	}
}

func testCA(t *testing.T, now time.Time) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test device CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0),
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func testDeviceCertificate(
	t *testing.T,
	ca *x509.Certificate,
	caKey *ecdsa.PrivateKey,
	csr *x509.CertificateRequest,
	deviceID string,
	now time.Time,
) ([]byte, string) {
	return testDeviceCertificateWithSerial(t, ca, caKey, csr, deviceID, now, 42)
}

func testDeviceCertificateWithSerial(
	t *testing.T,
	ca *x509.Certificate,
	caKey *ecdsa.PrivateKey,
	csr *x509.CertificateRequest,
	deviceID string,
	now time.Time,
	serialValue int64,
) ([]byte, string) {
	t.Helper()
	identityURI, _ := url.Parse("urn:guardian:device:" + deviceID)
	serial := big.NewInt(serialValue)
	template := &x509.Certificate{
		SerialNumber: serial, URIs: []*url.URL{identityURI},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(30 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, csr.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), serial.Text(16)
}
