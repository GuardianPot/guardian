package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/config"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
)

func TestCollectAndWriteAreReadOnlyBoundedAndRedacted(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	marker := "PRIVATE_KEY_SHOULD_NEVER_APPEAR"
	cfg := config.Config{
		ControlPlaneEndpoint: "guardian.example:443",
		DatabasePath:         filepath.Join(root, "edge.db"),
		SpoolDirectory:       filepath.Join(root, "spool"),
		IdentityCertPath:     filepath.Join(root, marker+".crt"),
		IdentityKeyPath:      filepath.Join(root, marker+".key"),
		ShutdownSeconds:      15,
		LogLevel:             "info",
	}
	store, err := storage.Open(ctx, storage.Options{DatabasePath: cfg.DatabasePath, SpoolDirectory: cfg.SpoolDirectory})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetIdentity(ctx, storage.IdentityRecord{
		CertificateSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		NotBefore:         time.Now().Add(-time.Hour),
		NotAfter:          time.Now().Add(time.Hour),
		EnrollmentStatus:  "enrolled",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetHealth(ctx, storage.HealthCondition{Name: "device-channel", Status: "degraded", ReasonCode: "not-implemented"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	report, err := Collect(ctx, cfg, "test-version", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Write(&output, report, "json"); err != nil {
		t.Fatal(err)
	}
	if output.Len() > maxOutputBytes || strings.Contains(output.String(), marker) || strings.Contains(output.String(), cfg.DatabasePath) {
		t.Fatalf("unsafe diagnostics output: %s", output.String())
	}
	var decoded Report
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("machine-readable output: %v", err)
	}
	if decoded.Version != "test-version" || decoded.State.Identity == nil || decoded.State.Identity.EnrollmentStatus != "enrolled" {
		t.Fatalf("decoded report = %+v", decoded)
	}

	output.Reset()
	if err := Write(&output, report, "text"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "health.device-channel=degraded") || strings.Contains(output.String(), marker) {
		t.Fatalf("text diagnostics output: %s", output.String())
	}
}

func TestWriteRejectsUnknownFormat(t *testing.T) {
	if err := Write(new(bytes.Buffer), Report{}, "yaml"); err == nil {
		t.Fatal("unknown diagnostics format unexpectedly accepted")
	}
}
