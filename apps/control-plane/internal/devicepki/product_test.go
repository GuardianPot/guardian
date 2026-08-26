package devicepki

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/secretstore"
)

const testDeviceID = "0198dc8c-c600-7000-8000-000000000004"

func TestProductAuthoritySealsCAAndIssuesBoundDeviceCertificate(t *testing.T) {
	now := time.Date(2026, 8, 24, 20, 0, 0, 0, time.UTC)
	secrets, err := secretstore.NewLocal(bytes.Repeat([]byte{0x31}, secretstore.MasterKeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	material, err := GenerateMaterial(secrets, now)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(material.PrivateKeyEnvelope, []byte("PRIVATE")) {
		t.Fatal("sealed CA material contains plaintext marker")
	}
	authority, err := LoadProductAuthority(material, secrets, now)
	if err != nil {
		t.Fatal(err)
	}
	csr := newCSR(t, elliptic.P256())
	issued, err := authority.Issue(testDeviceID, csr, now)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := ParseCertificate(issued.PEM)
	if err != nil {
		t.Fatal(err)
	}
	deviceID, err := authority.Verify(certificate, now)
	if err != nil || deviceID != testDeviceID {
		t.Fatalf("verify = %q, %v", deviceID, err)
	}
	if issued.NotAfter.Sub(now) != DeviceCertificateLifetime || len(issued.Fingerprint) != 64 {
		t.Fatalf("issued metadata = %+v", issued)
	}
	if !RotationDue(issued.NotAfter, issued.NotAfter.Add(-RotationWindow)) || RotationDue(issued.NotAfter, now) {
		t.Fatal("rotation window is not deterministic")
	}
}

func TestProductAuthorityRejectsWrongContextTamperAndUnsupportedCSR(t *testing.T) {
	now := time.Now().UTC()
	secrets, _ := secretstore.NewLocal(bytes.Repeat([]byte{0x41}, secretstore.MasterKeyBytes))
	material, err := GenerateMaterial(secrets, now)
	if err != nil {
		t.Fatal(err)
	}
	material.PrivateKeyEnvelope[len(material.PrivateKeyEnvelope)-1] ^= 0xff
	if _, err := LoadProductAuthority(material, secrets, now); !errors.Is(err, ErrInvalidCAMaterial) {
		t.Fatalf("tampered material error = %v", err)
	}
	material, _ = GenerateMaterial(secrets, now)
	authority, _ := LoadProductAuthority(material, secrets, now)
	if _, err := authority.Issue("not-a-uuid", newCSR(t, elliptic.P256()), now); !errors.Is(err, ErrInvalidDeviceID) {
		t.Fatalf("device id error = %v", err)
	}
	if _, err := authority.Issue(testDeviceID, []byte("not a csr"), now); !errors.Is(err, ErrInvalidCSR) {
		t.Fatalf("malformed CSR error = %v", err)
	}
	if _, err := authority.Issue(testDeviceID, newCSR(t, elliptic.P384()), now); !errors.Is(err, ErrInvalidCSR) {
		t.Fatalf("unsupported CSR error = %v", err)
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{DNSNames: []string{"attacker.example"}}, key)
	csrWithExtensions := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})
	if _, err := authority.Issue(testDeviceID, csrWithExtensions, now); !errors.Is(err, ErrInvalidCSR) {
		t.Fatalf("CSR extension error = %v", err)
	}
}

func newCSR(t *testing.T, curve elliptic.Curve) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})
}
