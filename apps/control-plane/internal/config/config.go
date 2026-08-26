// Package config owns Control Plane process configuration.
package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Command is an explicitly selected Control Plane operation.
type Command string

const (
	CommandServe                Command = "serve"
	CommandMigrate              Command = "migrate"
	CommandInitDeviceCA         Command = "init-device-ca"
	CommandCreateBootstrapToken Command = "create-bootstrap-token"
	CommandVersion              Command = "version"
)

const (
	defaultHTTPAddress    = "127.0.0.1:8080"
	defaultShutdown       = 15 * time.Second
	defaultDatabaseConns  = int32(10)
	defaultLogLevel       = "info"
	minimumShutdownWindow = time.Second
)

// Config contains runtime settings. DatabaseURL is sensitive and must never be
// rendered through logs or diagnostics.
type Config struct {
	HTTPAddress        string
	DatabaseURL        string
	ShutdownTimeout    time.Duration
	DatabaseMaxConns   int32
	LogLevel           string
	MasterKeyFile      string
	PublicOrigin       string
	TLSCertificateFile string
	TLSPrivateKeyFile  string
}

// LookupEnv matches os.LookupEnv and keeps parsing deterministic in tests.
type LookupEnv func(string) (string, bool)

// Load parses one explicit command and its environment-backed configuration.
// Database credentials are accepted only through GUARDIAN_DATABASE_URL so they
// do not appear in the process argument list.
func Load(args []string, lookup LookupEnv) (Command, Config, error) {
	if len(args) == 0 {
		return "", Config{}, errors.New("command is required: serve, migrate, init-device-ca, create-bootstrap-token, or version")
	}

	command := Command(args[0])
	switch command {
	case CommandVersion:
		if len(args) != 1 {
			return "", Config{}, errors.New("version does not accept arguments")
		}
		return command, Config{}, nil
	case CommandServe, CommandMigrate, CommandInitDeviceCA, CommandCreateBootstrapToken:
	default:
		return "", Config{}, fmt.Errorf("unknown command %q", args[0])
	}

	cfg := Config{
		HTTPAddress:        envOr(lookup, "GUARDIAN_HTTP_ADDRESS", defaultHTTPAddress),
		DatabaseURL:        envOr(lookup, "GUARDIAN_DATABASE_URL", ""),
		ShutdownTimeout:    defaultShutdown,
		DatabaseMaxConns:   defaultDatabaseConns,
		LogLevel:           envOr(lookup, "GUARDIAN_LOG_LEVEL", defaultLogLevel),
		MasterKeyFile:      envOr(lookup, "GUARDIAN_MASTER_KEY_FILE", ""),
		PublicOrigin:       envOr(lookup, "GUARDIAN_PUBLIC_ORIGIN", ""),
		TLSCertificateFile: envOr(lookup, "GUARDIAN_TLS_CERT_FILE", ""),
		TLSPrivateKeyFile:  envOr(lookup, "GUARDIAN_TLS_KEY_FILE", ""),
	}
	if value, ok := lookup("GUARDIAN_SHUTDOWN_TIMEOUT"); ok && value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return "", Config{}, fmt.Errorf("parse GUARDIAN_SHUTDOWN_TIMEOUT: %w", err)
		}
		cfg.ShutdownTimeout = duration
	}
	if value, ok := lookup("GUARDIAN_DATABASE_MAX_CONNS"); ok && value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return "", Config{}, fmt.Errorf("parse GUARDIAN_DATABASE_MAX_CONNS: %w", err)
		}
		cfg.DatabaseMaxConns = int32(parsed)
	}

	flags := flag.NewFlagSet(string(command), flag.ContinueOnError)
	flags.SetOutput(new(discardWriter))
	if command == CommandServe {
		flags.StringVar(&cfg.HTTPAddress, "http-address", cfg.HTTPAddress, "HTTP listen address")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return "", Config{}, fmt.Errorf("parse %s options: %w", command, err)
	}
	if flags.NArg() != 0 {
		return "", Config{}, fmt.Errorf("unexpected %s arguments", command)
	}
	if err := cfg.Validate(command); err != nil {
		return "", Config{}, err
	}
	return command, cfg, nil
}

// Validate enforces startup invariants without rendering sensitive values.
func (c Config) Validate(command Command) error {
	if command != CommandServe && command != CommandMigrate && command != CommandInitDeviceCA && command != CommandCreateBootstrapToken {
		return nil
	}
	if c.DatabaseURL == "" {
		return errors.New("GUARDIAN_DATABASE_URL is required")
	}
	if c.DatabaseMaxConns < 1 {
		return errors.New("GUARDIAN_DATABASE_MAX_CONNS must be positive")
	}
	if c.ShutdownTimeout < minimumShutdownWindow {
		return fmt.Errorf("GUARDIAN_SHUTDOWN_TIMEOUT must be at least %s", minimumShutdownWindow)
	}
	if command == CommandServe && c.HTTPAddress == "" {
		return errors.New("HTTP address must not be empty")
	}
	if command == CommandInitDeviceCA || command == CommandCreateBootstrapToken || command == CommandServe {
		if c.MasterKeyFile == "" {
			return errors.New("GUARDIAN_MASTER_KEY_FILE is required")
		}
		if !filepath.IsAbs(c.MasterKeyFile) || filepath.Clean(c.MasterKeyFile) != c.MasterKeyFile {
			return errors.New("GUARDIAN_MASTER_KEY_FILE must be an absolute clean path")
		}
	}
	if command == CommandServe {
		if !validPublicOrigin(c.PublicOrigin) {
			return errors.New("GUARDIAN_PUBLIC_ORIGIN must be an HTTPS origin without a path, query, fragment, or trailing slash")
		}
		tlsConfigured := c.TLSCertificateFile != "" || c.TLSPrivateKeyFile != ""
		if tlsConfigured && !c.EnrollmentEnabled() {
			return errors.New("GUARDIAN_TLS_CERT_FILE and GUARDIAN_TLS_KEY_FILE must be configured together")
		}
		if !tlsConfigured {
			return validateLogLevel(c.LogLevel)
		}
		for name, path := range map[string]string{
			"GUARDIAN_TLS_CERT_FILE": c.TLSCertificateFile,
			"GUARDIAN_TLS_KEY_FILE":  c.TLSPrivateKeyFile,
		} {
			if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
				return fmt.Errorf("%s must be an absolute clean path", name)
			}
		}
		if c.TLSCertificateFile == c.TLSPrivateKeyFile {
			return errors.New("TLS certificate and private key paths must differ")
		}
	}
	return validateLogLevel(c.LogLevel)
}

// EnrollmentEnabled reports whether the fail-closed direct-TLS/device
// enrollment bundle was configured completely. Authentication always requires
// the master key; direct TLS additionally requires both certificate paths.
func (c Config) EnrollmentEnabled() bool {
	return c.MasterKeyFile != "" && c.TLSCertificateFile != "" && c.TLSPrivateKeyFile != ""
}

func validPublicOrigin(value string) bool {
	if len(value) < len("https://a") || len(value) > 2048 || !strings.HasPrefix(value, "https://") || strings.HasSuffix(value, "/") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil &&
		parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validateLogLevel(value string) error {
	switch value {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("GUARDIAN_LOG_LEVEL must be debug, info, warn, or error")
	}
	return nil
}

func envOr(lookup LookupEnv, key, fallback string) string {
	if value, ok := lookup(key); ok {
		return value
	}
	return fallback
}

type discardWriter struct{}

func (*discardWriter) Write(p []byte) (int, error) { return len(p), nil }
