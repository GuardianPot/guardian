package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func lookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func TestLoadServeDefaultsAndOverrides(t *testing.T) {
	masterKey := filepath.Join(t.TempDir(), "guardian-master-key")
	tlsCertificate := filepath.Join(t.TempDir(), "server.crt")
	tlsKey := filepath.Join(t.TempDir(), "server.key")
	command, cfg, err := Load([]string{"serve", "--http-address", "127.0.0.1:9090"}, lookup(map[string]string{
		"GUARDIAN_DATABASE_URL":       "postgres://user:secret@db/guardian",
		"GUARDIAN_SHUTDOWN_TIMEOUT":   "20s",
		"GUARDIAN_DATABASE_MAX_CONNS": "12",
		"GUARDIAN_LOG_LEVEL":          "debug",
		"GUARDIAN_MASTER_KEY_FILE":    masterKey,
		"GUARDIAN_PUBLIC_ORIGIN":      "https://guardian.example.test",
		"GUARDIAN_TLS_CERT_FILE":      tlsCertificate,
		"GUARDIAN_TLS_KEY_FILE":       tlsKey,
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if command != CommandServe || cfg.HTTPAddress != "127.0.0.1:9090" || cfg.DeviceChannelAddress != "127.0.0.1:8443" || cfg.ShutdownTimeout != 20*time.Second || cfg.DatabaseMaxConns != 12 || cfg.LogLevel != "debug" {
		t.Fatalf("Load() = (%q, %+v), unexpected configuration", command, cfg)
	}
}

func TestLoadRejectsMissingDatabaseWithoutLeakingConfiguredValue(t *testing.T) {
	_, _, err := Load([]string{"serve"}, lookup(nil))
	if err == nil || !strings.Contains(err.Error(), "GUARDIAN_DATABASE_URL is required") {
		t.Fatalf("Load() error = %v, want missing database error", err)
	}

	secret := "super-secret-password"
	masterKey := filepath.Join(t.TempDir(), "guardian-master-key")
	tlsCertificate := filepath.Join(t.TempDir(), "server.crt")
	tlsKey := filepath.Join(t.TempDir(), "server.key")
	_, _, err = Load([]string{"serve"}, lookup(map[string]string{
		"GUARDIAN_DATABASE_URL":       "postgres://guardian:" + secret + "@db/guardian",
		"GUARDIAN_DATABASE_MAX_CONNS": "0",
		"GUARDIAN_MASTER_KEY_FILE":    masterKey,
		"GUARDIAN_TLS_CERT_FILE":      tlsCertificate,
		"GUARDIAN_TLS_KEY_FILE":       tlsKey,
	}))
	if err == nil {
		t.Fatal("Load() unexpectedly accepted zero database connections")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("configuration error leaked database credential")
	}
}

func TestLoadRequiresExplicitKnownCommand(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"version", "extra"}} {
		if _, _, err := Load(args, lookup(nil)); err == nil {
			t.Fatalf("Load(%v) unexpectedly succeeded", args)
		}
	}
}

func TestServeRequiresAuthAndAllowsEnrollmentTLSDisabled(t *testing.T) {
	masterKey := filepath.Join(t.TempDir(), "master.key")
	command, cfg, err := Load([]string{"serve"}, lookup(map[string]string{
		"GUARDIAN_DATABASE_URL":    "postgres://guardian:secret@db/guardian",
		"GUARDIAN_MASTER_KEY_FILE": masterKey,
		"GUARDIAN_PUBLIC_ORIGIN":   "https://guardian.example.test",
	}))
	if err != nil || command != CommandServe || cfg.EnrollmentEnabled() {
		t.Fatalf("auth-only serve load = %q %+v %v", command, cfg, err)
	}
	_, _, err = Load([]string{"serve"}, lookup(map[string]string{
		"GUARDIAN_DATABASE_URL":    "postgres://guardian:secret@db/guardian",
		"GUARDIAN_MASTER_KEY_FILE": masterKey,
		"GUARDIAN_PUBLIC_ORIGIN":   "http://guardian.example.test",
	}))
	if err == nil || !strings.Contains(err.Error(), "GUARDIAN_PUBLIC_ORIGIN") {
		t.Fatalf("invalid public origin error = %v", err)
	}
	_, _, err = Load([]string{"serve"}, lookup(map[string]string{
		"GUARDIAN_DATABASE_URL":    "postgres://guardian:secret@db/guardian",
		"GUARDIAN_MASTER_KEY_FILE": masterKey,
		"GUARDIAN_PUBLIC_ORIGIN":   "https://guardian.example.test",
		"GUARDIAN_TLS_CERT_FILE":   filepath.Join(t.TempDir(), "server.crt"),
	}))
	if err == nil || !strings.Contains(err.Error(), "must be configured together") {
		t.Fatalf("partial TLS bundle error = %v", err)
	}
}

func TestCreateBootstrapTokenRequiresAbsoluteMasterKey(t *testing.T) {
	command, cfg, err := Load([]string{"create-bootstrap-token"}, lookup(map[string]string{
		"GUARDIAN_DATABASE_URL":    "postgres://guardian:secret@db/guardian",
		"GUARDIAN_MASTER_KEY_FILE": filepath.Join(t.TempDir(), "master.key"),
	}))
	if err != nil || command != CommandCreateBootstrapToken || cfg.MasterKeyFile == "" {
		t.Fatalf("Load(create-bootstrap-token) = %q %+v %v", command, cfg, err)
	}
}

func TestServeFailsClosedWithoutAuthRuntimeSettings(t *testing.T) {
	base := map[string]string{"GUARDIAN_DATABASE_URL": "postgres://guardian:secret@db/guardian"}
	if _, _, err := Load([]string{"serve"}, lookup(base)); err == nil || !strings.Contains(err.Error(), "GUARDIAN_MASTER_KEY_FILE") {
		t.Fatalf("missing master-key error = %v", err)
	}
	base["GUARDIAN_MASTER_KEY_FILE"] = filepath.Join(t.TempDir(), "master.key")
	if _, _, err := Load([]string{"serve"}, lookup(base)); err == nil || !strings.Contains(err.Error(), "GUARDIAN_PUBLIC_ORIGIN") {
		t.Fatalf("missing public-origin error = %v", err)
	}
}

func TestServeValidatesOptionalWebConsoleDirectory(t *testing.T) {
	base := map[string]string{
		"GUARDIAN_DATABASE_URL":    "postgres://guardian:secret@db/guardian",
		"GUARDIAN_MASTER_KEY_FILE": filepath.Join(t.TempDir(), "master.key"),
		"GUARDIAN_PUBLIC_ORIGIN":   "https://guardian.example.test",
		"GUARDIAN_WEB_CONSOLE_DIR": "relative/web-console",
	}
	if _, _, err := Load([]string{"serve"}, lookup(base)); err == nil || !strings.Contains(err.Error(), "GUARDIAN_WEB_CONSOLE_DIR") {
		t.Fatalf("relative web-console directory error = %v", err)
	}
	base["GUARDIAN_WEB_CONSOLE_DIR"] = filepath.Join(t.TempDir(), "web-console")
	if err := os.Mkdir(base["GUARDIAN_WEB_CONSOLE_DIR"], 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base["GUARDIAN_WEB_CONSOLE_DIR"], "index.html"), []byte("web console"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, cfg, err := Load([]string{"serve"}, lookup(base))
	if err != nil || cfg.WebConsoleDirectory != base["GUARDIAN_WEB_CONSOLE_DIR"] {
		t.Fatalf("absolute web-console directory = (%q, %v)", cfg.WebConsoleDirectory, err)
	}
}
