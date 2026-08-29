package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/config"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	if err := run(context.Background(), []string{"version"}, new(bytes.Buffer), &stdout, new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != version {
		t.Fatalf("version output = %q", stdout.String())
	}
}

func TestRunRequiresConfig(t *testing.T) {
	if err := run(context.Background(), []string{"serve"}, new(bytes.Buffer), new(bytes.Buffer), new(bytes.Buffer)); err == nil {
		t.Fatal("serve without config unexpectedly succeeded")
	}
}

func TestStatusIsReadOnlyAndDoesNotCreateMissingDatabase(t *testing.T) {
	root := t.TempDir()
	cfg, configPath := writeMainConfig(t, root)
	err := run(context.Background(), []string{"status", "--config", configPath}, new(bytes.Buffer), new(bytes.Buffer), new(bytes.Buffer))
	if err == nil {
		t.Fatal("status unexpectedly succeeded without state")
	}
	if _, statErr := os.Stat(cfg.DatabasePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("read-only status created database: %v", statErr)
	}
}

func TestRecoverDBRequiresExplicitConfirmation(t *testing.T) {
	root := t.TempDir()
	cfg, configPath := writeMainConfig(t, root)
	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.DatabasePath, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := run(context.Background(), []string{"recover-db", "--config", configPath}, new(bytes.Buffer), new(bytes.Buffer), new(bytes.Buffer))
	if !errors.Is(err, storage.ErrRecoveryConfirmationRequired) {
		t.Fatalf("unconfirmed recover-db error = %v", err)
	}
	var stdout bytes.Buffer
	err = run(context.Background(), []string{
		"recover-db", "--config", configPath, "--confirm-reset-development-data",
	}, new(bytes.Buffer), &stdout, new(bytes.Buffer))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "development database recovered") {
		t.Fatalf("recovery output = %q", stdout.String())
	}
}

func writeMainConfig(t *testing.T, root string) (config.Config, string) {
	t.Helper()
	cfg := config.Config{
		ControlPlaneEndpoint: "127.0.0.1:7443",
		DatabasePath:         filepath.Join(root, "state", "edge.db"),
		SpoolDirectory:       filepath.Join(root, "spool"),
		SpoolCapacityBytes:   1 << 30,
		IdentityCertPath:     filepath.Join(root, "identity", "device.crt"),
		IdentityKeyPath:      filepath.Join(root, "identity", "device.key"),
		ShutdownSeconds:      1,
		LogLevel:             "info",
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "edge.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return cfg, path
}
