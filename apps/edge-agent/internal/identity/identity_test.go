package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadValidProtectedIdentity(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	certPath, keyPath := writeIdentity(t, now.Add(-time.Hour), now.Add(time.Hour), nil)
	metadata, err := Load(certPath, keyPath, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata.CertificateSHA256) != 64 || !metadata.NotAfter.Equal(now.Add(time.Hour)) {
		t.Fatalf("metadata = %+v", metadata)
	}
}

func TestLoadRejectsMissingExpiredMismatchedAndUnsafeIdentity(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	if _, err := Load(filepath.Join(t.TempDir(), "missing.crt"), filepath.Join(t.TempDir(), "missing.key"), now); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing identity error = %v", err)
	}

	expiredCert, expiredKey := writeIdentity(t, now.Add(-2*time.Hour), now.Add(-time.Hour), nil)
	if _, err := Load(expiredCert, expiredKey, now); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired identity error = %v", err)
	}

	certPath, _ := writeIdentity(t, now.Add(-time.Hour), now.Add(time.Hour), nil)
	_, anotherKey := writeIdentity(t, now.Add(-time.Hour), now.Add(time.Hour), nil)
	if _, err := Load(certPath, anotherKey, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mismatched identity error = %v", err)
	}

	_, keyPath := writeIdentity(t, now.Add(-time.Hour), now.Add(time.Hour), nil)
	if err := os.Chmod(keyPath, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(certPath, keyPath, now); !errors.Is(err, ErrPermissions) {
		t.Fatalf("unsafe key permissions error = %v", err)
	}

	_, oversizedKey := writeIdentity(t, now.Add(-time.Hour), now.Add(time.Hour), nil)
	if err := os.WriteFile(oversizedKey, make([]byte, maxIdentityFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(certPath, oversizedKey, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized identity error = %v", err)
	}
}

func TestLoadRejectsIdentitySymlink(t *testing.T) {
	now := time.Now().UTC()
	certPath, keyPath := writeIdentity(t, now.Add(-time.Hour), now.Add(time.Hour), nil)
	symlinkPath := filepath.Join(t.TempDir(), "device.key")
	if err := os.Symlink(keyPath, symlinkPath); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(certPath, symlinkPath, now); !errors.Is(err, ErrPermissions) {
		t.Fatalf("symlink identity error = %v", err)
	}
}

func writeIdentity(t *testing.T, notBefore, notAfter time.Time, key *ecdsa.PrivateKey) (string, string) {
	t.Helper()
	if key == nil {
		var err error
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(notAfter.UnixNano()),
		Subject:      pkix.Name{CommonName: "guardian-edge-test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	certPath := filepath.Join(root, "device.crt")
	keyPath := filepath.Join(root, "device.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}
