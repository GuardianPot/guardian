package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestInstallCreatesProtectedPairAndLoadRecoversJournal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Edge identity permission contract is Linux-only")
	}
	root := filepath.Join(t.TempDir(), "identity")
	certPath, keyPath := filepath.Join(root, "device.crt"), filepath.Join(root, "device.key")
	certificatePEM, privateKeyPEM := testIdentityPair(t, time.Now())
	if err := Install(certPath, keyPath, certificatePEM, privateKeyPEM); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(certPath, keyPath, time.Now()); err != nil {
		t.Fatal(err)
	}
	keyInfo, err := os.Stat(keyPath)
	if err != nil || keyInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("key permissions = %v, %v", keyInfo.Mode().Perm(), err)
	}
	journal := filepath.Join(root, ".guardian-identity-install")
	if err := os.WriteFile(journal, []byte("guardian.identity.install.v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(certPath, certPath+".guardian-old"); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(certPath, keyPath, time.Now()); err != nil {
		t.Fatalf("recover interrupted install: %v", err)
	}
}

func TestInstallDiscardsUnjournaledCrashStage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Edge identity permission contract is Linux-only")
	}
	root := filepath.Join(t.TempDir(), "identity")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := filepath.Join(root, "device.crt"), filepath.Join(root, "device.key")
	if err := os.WriteFile(certPath+installSuffix, []byte("interrupted-stage"), 0o600); err != nil {
		t.Fatal(err)
	}
	certificatePEM, privateKeyPEM := testIdentityPair(t, time.Now())
	if err := Install(certPath, keyPath, certificatePEM, privateKeyPEM); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(certPath, keyPath, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func testIdentityPair(t *testing.T, now time.Time) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
}
