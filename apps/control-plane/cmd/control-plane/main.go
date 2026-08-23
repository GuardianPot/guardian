package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/app"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/config"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/database"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "guardian-control-plane: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, lookup config.LookupEnv, stdout, stderr io.Writer) error {
	command, cfg, err := config.Load(args, lookup)
	if err != nil {
		return err
	}
	switch command {
	case config.CommandVersion:
		_, err := fmt.Fprintln(stdout, version)
		return err
	case config.CommandMigrate:
		report, err := database.Migrate(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "migration complete: applied=%d version=%d\n", report.Applied, report.Version)
		return err
	case config.CommandServe:
		return app.Run(ctx, cfg, newLogger(stderr, cfg.LogLevel))
	default:
		return fmt.Errorf("unsupported command %q", command)
	}
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
