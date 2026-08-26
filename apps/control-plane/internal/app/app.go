// Package app composes the Control Plane process without hiding module or
// shutdown order.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/api"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/auth"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/config"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/deception"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicepki"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/health"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/jobs"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/lifecycle"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/secretstore"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage"
)

// Run opens the already-migrated database, starts modules in dependency order,
// serves health endpoints, and drains everything on cancellation.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	store, err := storage.Open(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		return fmt.Errorf("open Control Plane store: %w", err)
	}
	defer store.Close()
	if err := store.Ready(ctx); err != nil {
		return fmt.Errorf("Control Plane database is not ready; run migrate explicitly: %w", err)
	}
	serverOptions := []api.Option{}
	if cfg.EnrollmentEnabled() {
		secrets, err := secretstore.LoadLocal(cfg.MasterKeyFile)
		if err != nil {
			return fmt.Errorf("load Control Plane SecretStore: %w", err)
		}
		caMaterial, err := store.DeviceCAMaterial(ctx)
		if err != nil {
			return fmt.Errorf("load device CA material: %w", err)
		}
		authority, err := devicepki.LoadProductAuthority(caMaterial, secrets, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("open device CA: %w", err)
		}
		deviceService, err := devices.NewService(store, authority)
		if err != nil {
			return fmt.Errorf("build device enrollment service: %w", err)
		}
		serverOptions = append(serverOptions,
			api.WithDeviceService(deviceService),
			api.WithTLSFiles(cfg.TLSCertificateFile, cfg.TLSPrivateKeyFile, authority.CertificatePEM()),
		)
	}
	components, err := lifecycle.New(
		auth.New(),
		environment.New(),
		devices.New(),
		deception.New(),
		health.New(),
		audit.New(),
		jobs.New(),
		api.NewModule(),
	)
	if err != nil {
		return fmt.Errorf("build module graph: %w", err)
	}
	if err := components.Start(ctx); err != nil {
		return err
	}

	server := api.NewServer(cfg.HTTPAddress, store, logger, serverOptions...)
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
