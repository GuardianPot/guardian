// Package config owns Edge daemon command and file configuration parsing.
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Command is an explicitly selected Edge operation.
type Command string

const (
	CommandServe       Command = "serve"
	CommandEnroll      Command = "enroll"
	CommandStatus      Command = "status"
	CommandDiagnostics Command = "diagnostics"
	CommandRecoverDB   Command = "recover-db"
	CommandVersion     Command = "version"
)

const (
	defaultShutdownSeconds = 15
	defaultLogLevel        = "info"
	maxConfigBytes         = 64 << 10
)

// Invocation contains command-line choices. Secrets are deliberately absent.
type Invocation struct {
	Command              Command
	ConfigPath           string
	Format               string
	ConfirmDevelopmentDB bool
}

// Config contains non-secret Edge runtime settings. Enrollment tokens and
// private-key material are intentionally not accepted in this file.
type Config struct {
	ControlPlaneEndpoint  string `json:"control_plane_endpoint"`
	DeviceChannelEndpoint string `json:"device_channel_endpoint,omitempty"`
	DatabasePath          string `json:"database_path"`
	SpoolDirectory        string `json:"spool_directory"`
	IdentityCertPath      string `json:"identity_certificate_path"`
	IdentityKeyPath       string `json:"identity_private_key_path"`
	ShutdownSeconds       int    `json:"shutdown_timeout_seconds,omitempty"`
	LogLevel              string `json:"log_level,omitempty"`
}

// ShutdownTimeout returns the validated shutdown window.
func (c Config) ShutdownTimeout() time.Duration {
	return time.Duration(c.ShutdownSeconds) * time.Second
}

// Parse parses one explicit Edge command without reading configuration files.
func Parse(args []string) (Invocation, error) {
	if len(args) == 0 {
		return Invocation{}, errors.New("command is required: serve, enroll, status, diagnostics, recover-db, or version")
	}

	command := Command(args[0])
	if command == CommandVersion {
		if len(args) != 1 {
			return Invocation{}, errors.New("version does not accept arguments")
		}
		return Invocation{Command: command}, nil
	}

	switch command {
	case CommandServe, CommandEnroll, CommandStatus, CommandDiagnostics, CommandRecoverDB:
	default:
		return Invocation{}, fmt.Errorf("unknown command %q", args[0])
	}

	invocation := Invocation{Command: command, Format: "text"}
	if command == CommandDiagnostics {
		invocation.Format = "json"
	}
	flags := flag.NewFlagSet(string(command), flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&invocation.ConfigPath, "config", "", "absolute path to the Edge configuration file")
	if command == CommandStatus || command == CommandDiagnostics {
		flags.StringVar(&invocation.Format, "format", invocation.Format, "output format: text or json")
	}
	if command == CommandRecoverDB {
		flags.BoolVar(&invocation.ConfirmDevelopmentDB, "confirm-reset-development-data", false, "confirm destructive development database recovery")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return Invocation{}, fmt.Errorf("parse %s options: %w", command, err)
	}
	if flags.NArg() != 0 {
		return Invocation{}, fmt.Errorf("unexpected %s arguments", command)
	}
	if invocation.ConfigPath == "" {
		return Invocation{}, errors.New("--config is required")
	}
	if !filepath.IsAbs(invocation.ConfigPath) {
		return Invocation{}, errors.New("--config must be an absolute path")
	}
	if invocation.Format != "text" && invocation.Format != "json" {
		return Invocation{}, errors.New("--format must be text or json")
	}
	return invocation, nil
}

// Load reads a bounded, strict JSON configuration file and validates every
// startup invariant. Group/world-writable configuration is rejected.
func Load(path string) (Config, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Config{}, fmt.Errorf("read Edge configuration metadata: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return Config{}, errors.New("Edge configuration must be a regular non-symlink file")
	}
	if info.Mode().Perm()&0o022 != 0 {
		return Config{}, errors.New("Edge configuration must not be group- or world-writable")
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open Edge configuration: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return Config{}, fmt.Errorf("inspect opened Edge configuration: %w", err)
	}
	if !os.SameFile(info, openedInfo) {
		return Config{}, errors.New("Edge configuration changed while opening")
	}

	limited := &io.LimitedReader{R: file, N: maxConfigBytes + 1}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode Edge configuration: %w", err)
	}
	if limited.N == 0 {
		return Config{}, fmt.Errorf("Edge configuration exceeds %d bytes", maxConfigBytes)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Config{}, err
	}
	if limited.N == 0 {
		return Config{}, fmt.Errorf("Edge configuration exceeds %d bytes", maxConfigBytes)
	}
	if cfg.ShutdownSeconds == 0 {
		cfg.ShutdownSeconds = defaultShutdownSeconds
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = defaultLogLevel
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate enforces secure transport and filesystem invariants without reading
// or rendering credential material.
func (c Config) Validate() error {
	host, portText, err := net.SplitHostPort(c.ControlPlaneEndpoint)
	if err != nil || !validEndpointHost(host) {
		return errors.New("control_plane_endpoint must be a host:port endpoint")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("control_plane_endpoint port must be between 1 and 65535")
	}
	if c.DeviceChannelEndpoint != "" {
		channelHost, channelPortText, err := net.SplitHostPort(c.DeviceChannelEndpoint)
		if err != nil || !validEndpointHost(channelHost) {
			return errors.New("device_channel_endpoint must be a host:port endpoint")
		}
		channelPort, err := strconv.Atoi(channelPortText)
		if err != nil || channelPort < 1 || channelPort > 65535 {
			return errors.New("device_channel_endpoint port must be between 1 and 65535")
		}
	}
	paths := []struct {
		name  string
		value string
	}{
		{"database_path", c.DatabasePath},
		{"spool_directory", c.SpoolDirectory},
		{"identity_certificate_path", c.IdentityCertPath},
		{"identity_private_key_path", c.IdentityKeyPath},
	}
	for _, item := range paths {
		if item.value == "" || !filepath.IsAbs(item.value) {
			return fmt.Errorf("%s must be an absolute path", item.name)
		}
		if filepath.Clean(item.value) != item.value {
			return fmt.Errorf("%s must be a clean path", item.name)
		}
	}
	if c.IdentityCertPath == c.IdentityKeyPath {
		return errors.New("identity certificate and private key paths must differ")
	}
	if c.SpoolDirectory == string(filepath.Separator) {
		return errors.New("spool_directory must not be the filesystem root")
	}
	if c.ShutdownSeconds < 1 || c.ShutdownSeconds > 120 {
		return errors.New("shutdown_timeout_seconds must be between 1 and 120")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("log_level must be debug, info, warn, or error")
	}
	return nil
}

func validEndpointHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) < 1 || len(host) > 253 || strings.TrimSpace(host) != host || strings.ContainsAny(host, " \t\r\n") {
		return false
	}
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character >= 'a' && character <= 'z') ||
				(character >= 'A' && character <= 'Z') ||
				(character >= '0' && character <= '9') || character == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing Edge configuration data: %w", err)
	}
	return errors.New("Edge configuration must contain exactly one JSON object")
}
