// Package app composes the Control Plane process without hiding module or
// shutdown order.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/config"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/database"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/httpapi"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/lifecycle"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/modules/api"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/modules/audit"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/modules/auth"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/modules/deception"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/modules/devices"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/modules/environment"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/modules/health"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/modules/jobs"
)

// Run opens the already-migrated database, starts modules in dependency order,
// serves health endpoints, and drains everything on cancellation.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	store, err := database.Open(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		return fmt.Errorf("open Control Plane store: %w", err)
	}
	defer store.Close()
	if err := store.Ready(ctx); err != nil {
		return fmt.Errorf("Control Plane database is not ready; run migrate explicitly: %w", err)
	}

	components, err := lifecycle.New(
		auth.New(),
		environment.New(),
		devices.New(),
		deception.New(),
		health.New(),
		audit.New(),
		jobs.New(),
		api.New(),
	)
	if err != nil {
		return fmt.Errorf("build module graph: %w", err)
	}
	if err := components.Start(ctx); err != nil {
		return err
	}

	server := httpapi.New(cfg.HTTPAddress, store, logger)
	if err := server.Start(); err != nil {
		return errors.Join(fmt.Errorf("start HTTP server: %w", err), components.Stop(context.Background()))
	}
	logger.InfoContext(ctx, "Control Plane started", "http_address", server.Address())

	var runErr error
	select {
	case <-ctx.Done():
		logger.Info("Control Plane shutdown requested")
	case serveErr := <-server.Errors():
		if serveErr != nil {
			runErr = fmt.Errorf("serve HTTP: %w", serveErr)
		} else {
			runErr = errors.New("HTTP server stopped unexpectedly")
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	componentErr := components.Stop(shutdownCtx)
	return errors.Join(runErr, shutdownErr, componentErr)
}
