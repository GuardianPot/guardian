package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/app"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/config"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/diagnostics"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/enrollment"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
)

const fixtureCrashExitCode = 42

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "guardian-edge: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 3 && args[0] == "--w8-fixture" {
		return runWALFixture(args[1], args[2])
	}
	invocation, err := config.Parse(args)
	if err != nil {
		return err
	}
	if invocation.Command == config.CommandVersion {
		_, err := fmt.Fprintln(stdout, version)
		return err
	}
	cfg, err := config.Load(invocation.ConfigPath)
	if err != nil {
		return err
	}

	switch invocation.Command {
	case config.CommandServe:
		return app.Run(ctx, cfg, newLogger(stderr, cfg.LogLevel))
	case config.CommandEnroll:
		token, err := readEnrollmentToken(stdin)
		if err != nil {
			return err
		}
		defer clear(token)
		result, err := (&enrollment.Client{}).Enroll(
			ctx, cfg.ControlPlaneEndpoint, token, cfg.IdentityCertPath, cfg.IdentityKeyPath,
		)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "Edge enrolled: device_id=%s certificate_serial=%s\n", result.DeviceID, result.Serial)
		return err
	case config.CommandStatus, config.CommandDiagnostics:
		report, err := diagnostics.Collect(ctx, cfg, version, time.Now())
		if err != nil {
			return err
		}
		return diagnostics.Write(stdout, report, invocation.Format)
	case config.CommandRecoverDB:
		report, err := storage.RecoverDevelopmentDatabase(ctx, storage.Options{
			DatabasePath: cfg.DatabasePath, SpoolDirectory: cfg.SpoolDirectory,
		}, invocation.ConfirmDevelopmentDB, time.Now())
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "development database recovered: quarantined=%d database=%s\n", len(report.QuarantinedPaths), report.DatabasePath); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported command %q", invocation.Command)
	}
}

func readEnrollmentToken(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, enrollment.ErrInvalidToken
	}
	token, err := io.ReadAll(io.LimitReader(reader, 66))
	if err != nil || len(token) > 65 {
		clear(token)
		return nil, enrollment.ErrInvalidToken
	}
	token = bytes.TrimSuffix(token, []byte("\n"))
	token = bytes.TrimSuffix(token, []byte("\r"))
	if len(token) == 0 || bytes.IndexAny(token, " \t\r\n") >= 0 {
		clear(token)
		return nil, enrollment.ErrInvalidToken
	}
	return token, nil
}

func newLogger(writer io.Writer, configured string) *slog.Logger {
	level := slog.LevelInfo
	switch configured {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level}))
}

func runWALFixture(mode, dbPath string) error {
	ctx := context.Background()
	store, err := storage.Open(ctx, storage.Options{
		DatabasePath: dbPath, SpoolDirectory: dbPath + ".spool",
	})
	if err != nil {
		return err
	}
	defer store.Close()

	const eventID = "w8-crash-fixture-event"
	switch mode {
	case "crash":
		if _, err := store.Enqueue(ctx, eventID, []byte("fixture durable payload")); err != nil {
			return fmt.Errorf("seed fixture event: %w", err)
		}
		event, ok, err := store.Claim(ctx, time.Now(), 100*time.Millisecond)
		if err != nil {
			return fmt.Errorf("claim fixture event: %w", err)
		}
		if !ok || event.ID != eventID || event.Attempts != 1 {
			return fmt.Errorf("unexpected crash claim: event=%+v available=%t", event, ok)
		}
		fmt.Printf("fixture claimed %s attempt=%d; simulating abrupt process death\n", event.ID, event.Attempts)
		os.Exit(fixtureCrashExitCode)
	case "recover":
		event, ok, err := store.Claim(ctx, time.Now(), time.Second)
		if err != nil {
			return fmt.Errorf("claim replayed fixture event: %w", err)
		}
		if !ok {
			return errors.New("replayed fixture event was not available")
		}
		if event.ID != eventID || event.Attempts != 2 {
			return fmt.Errorf("unexpected replayed event: %+v", event)
		}
		if err := store.Ack(ctx, event.ID, event.Attempts, time.Now()); err != nil {
			return fmt.Errorf("ack replayed fixture event: %w", err)
		}
		stats, err := store.Stats(ctx)
		if err != nil {
			return fmt.Errorf("read fixture stats: %w", err)
		}
		if stats.Delivered != 1 || stats.Pending != 0 || stats.Inflight != 0 {
			return fmt.Errorf("unexpected final fixture stats: %+v", stats)
		}
		fmt.Printf("fixture replayed %s attempt=%d and delivered exactly once\n", event.ID, event.Attempts)
		return nil
	default:
		return fmt.Errorf("unknown W8 fixture mode %q", mode)
	}
	return nil
}
