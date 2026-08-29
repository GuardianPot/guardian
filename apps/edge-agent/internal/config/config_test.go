package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseRequiresExplicitCommandAndAbsoluteConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"missing command", nil},
		{"unknown command", []string{"start"}},
		{"missing config", []string{"serve"}},
		{"relative config", []string{"serve", "--config", "edge.json"}},
		{"invalid format", []string{"status", "--config", filepath.Join(t.TempDir(), "edge.json"), "--format", "yaml"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(test.args); err == nil {
				t.Fatalf("Parse(%q) unexpectedly succeeded", test.args)
			}
		})
	}
}

func TestParseCommands(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "edge.json")
	invocation, err := Parse([]string{"diagnostics", "--config", configPath, "--format", "json"})
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Command != CommandDiagnostics || invocation.ConfigPath != configPath || invocation.Format != "json" {
		t.Fatalf("Parse() = %+v", invocation)
	}

	recoverInvocation, err := Parse([]string{"recover-db", "--config", configPath, "--confirm-reset-development-data"})
	if err != nil {
		t.Fatal(err)
	}
	if !recoverInvocation.ConfirmDevelopmentDB {
		t.Fatal("recovery confirmation was not recorded")
	}

	versionInvocation, err := Parse([]string{"version"})
	if err != nil || versionInvocation.Command != CommandVersion {
		t.Fatalf("version Parse() = (%+v, %v)", versionInvocation, err)
	}
}

func TestLoadStrictConfigurationAndDefaults(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "edge.json")
	writeConfig(t, path, `{
  "control_plane_endpoint": "guardian.example:443",
  "device_channel_endpoint": "devices.guardian.example:443",
  "database_path": "`+filepath.Join(root, "edge.db")+`",
  "spool_directory": "`+filepath.Join(root, "spool")+`",
  "spool_capacity_bytes": 1073741824,
  "identity_certificate_path": "`+filepath.Join(root, "identity.crt")+`",
  "identity_private_key_path": "`+filepath.Join(root, "identity.key")+`"
}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ShutdownTimeout() != 15*time.Second || cfg.LogLevel != "info" || cfg.DeviceChannelEndpoint != "devices.guardian.example:443" {
		t.Fatalf("defaults = timeout %s level %q", cfg.ShutdownTimeout(), cfg.LogLevel)
	}
}

func TestLoadRejectsMissingUnknownOversizedAndWritableConfiguration(t *testing.T) {
	root := t.TempDir()
	if _, err := Load(filepath.Join(root, "missing.json")); err == nil {
		t.Fatal("missing configuration unexpectedly loaded")
	}

	unknownPath := filepath.Join(root, "unknown.json")
	writeConfig(t, unknownPath, `{"unexpected":true}`)
	if _, err := Load(unknownPath); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}

	oversizedPath := filepath.Join(root, "oversized.json")
	writeConfig(t, oversizedPath, `{"padding":"`+strings.Repeat("x", maxConfigBytes)+`"}`)
	if _, err := Load(oversizedPath); err == nil {
		t.Fatal("oversized configuration unexpectedly loaded")
	}

	writablePath := filepath.Join(root, "writable.json")
	writeConfig(t, writablePath, `{}`)
	if err := os.Chmod(writablePath, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(writablePath); err == nil || !strings.Contains(err.Error(), "world-writable") {
		t.Fatalf("writable configuration error = %v", err)
	}

	trailingPath := filepath.Join(root, "trailing.json")
	writeConfig(t, trailingPath, `{} {}`)
	if _, err := Load(trailingPath); err == nil {
		t.Fatal("trailing JSON unexpectedly accepted")
	}

	whitespacePath := filepath.Join(root, "whitespace.json")
	writeConfig(t, whitespacePath, `{}`+strings.Repeat(" ", maxConfigBytes))
	if _, err := Load(whitespacePath); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized trailing whitespace error = %v", err)
	}
}

func TestValidateRejectsUnsafeConfiguration(t *testing.T) {
	root := t.TempDir()
	valid := Config{
		ControlPlaneEndpoint:  "guardian.example:443",
		DeviceChannelEndpoint: "devices.guardian.example:443",
		DatabasePath:          filepath.Join(root, "edge.db"),
		SpoolDirectory:        filepath.Join(root, "spool"),
		SpoolCapacityBytes:    1 << 30,
		IdentityCertPath:      filepath.Join(root, "device.crt"),
		IdentityKeyPath:       filepath.Join(root, "device.key"),
		ShutdownSeconds:       15,
		LogLevel:              "info",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid configuration: %v", err)
	}

	tests := []Config{
		func() Config { c := valid; c.ControlPlaneEndpoint = "http://guardian"; return c }(),
		func() Config { c := valid; c.DeviceChannelEndpoint = "https://guardian"; return c }(),
		func() Config { c := valid; c.DatabasePath = "edge.db"; return c }(),
		func() Config { c := valid; c.SpoolCapacityBytes = MinimumSpoolCapacity - 1; return c }(),
		func() Config { c := valid; c.SpoolCapacityBytes = MaximumSpoolCapacity + 1; return c }(),
		func() Config { c := valid; c.IdentityKeyPath = c.IdentityCertPath; return c }(),
		func() Config { c := valid; c.ShutdownSeconds = 121; return c }(),
		func() Config { c := valid; c.LogLevel = "trace"; return c }(),
	}
	for index, cfg := range tests {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("unsafe configuration %d unexpectedly validated", index)
		}
	}
}

func writeConfig(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("test configuration permissions = %o", info.Mode().Perm())
	}
}

func TestEnsureJSONEOFPropagatesMalformedTrailingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malformed.json")
	writeConfig(t, path, `{}`+string([]byte{0}))
	_, err := Load(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("malformed trailing data error = %v", err)
	}
}
