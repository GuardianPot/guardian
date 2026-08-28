package app

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/config"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
)

func TestRunRefusesMissingIdentityWithoutCreatingState(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(root)
	err := Run(context.Background(), cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if !errors.Is(err, identity.ErrUnavailable) {
		t.Fatalf("Run() error = %v", err)
	}
	if _, err := os.Stat(cfg.DatabasePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database was created during identity failure: %v", err)
	}
}

func TestRunStartsAndStopsAllComponentsOnCancellation(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(root)
	writeAppIdentity(t, cfg.IdentityCertPath, cfg.IdentityKeyPath)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- Run(ctx, cfg, logger) }()

	deadline := time.Now().Add(3 * time.Second)
	for {
		store, err := storage.OpenReadOnly(context.Background(), storage.Options{
			DatabasePath: cfg.DatabasePath, SpoolDirectory: cfg.SpoolDirectory,
		})
		if err == nil {
			snapshot, snapshotErr := store.Snapshot(context.Background())
			store.Close()
			if snapshotErr == nil && hasHealth(snapshot.Health, "process", "healthy") {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("daemon did not become healthy: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Run() shutdown error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not stop after cancellation")
	}

	store, err := storage.OpenReadOnly(context.Background(), storage.Options{
		DatabasePath: cfg.DatabasePath, SpoolDirectory: cfg.SpoolDirectory,
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(context.Background())
	store.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !hasHealth(snapshot.Health, "process", "stopped") {
		t.Fatalf("process did not persist stopped state: %+v", snapshot.Health)
	}
	for _, name := range []string{"enrollment", "telemetry-spool", "device-channel", "reconciler", "privileged-helper", "health-reporter"} {
		if !hasHealth(snapshot.Health, name, "stopped") {
			t.Fatalf("component %s did not stop: %+v", name, snapshot.Health)
		}
	}
	if strings.Contains(logs.String(), cfg.IdentityKeyPath) || strings.Contains(logs.String(), "PRIVATE KEY") {
		t.Fatalf("logs exposed private-key material: %s", logs.String())
	}
}

func testConfig(root string) config.Config {
	return config.Config{
		ControlPlaneEndpoint:  "127.0.0.1:7443",
		DeviceChannelEndpoint: "127.0.0.1:7444",
		DatabasePath:          filepath.Join(root, "state", "edge.db"),
		SpoolDirectory:        filepath.Join(root, "spool"),
		IdentityCertPath:      filepath.Join(root, "identity", "device.crt"),
		IdentityKeyPath:       filepath.Join(root, "identity", "device.key"),
		ShutdownSeconds:       1,
		LogLevel:              "info",
	}
}

func hasHealth(conditions []storage.HealthCondition, name, status string) bool {
	for _, condition := range conditions {
		if condition.Name == name && condition.Status == status {
			return true
		}
	}
	return false
}

func writeAppIdentity(t *testing.T, certPath, keyPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(certPath), 0o700); err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: "guardian-edge-app-test"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		URIs:         []*url.URL{{Scheme: "urn", Opaque: "guardian:device:0198f7c4-7b30-7f11-8a44-111111111111"}},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
}
