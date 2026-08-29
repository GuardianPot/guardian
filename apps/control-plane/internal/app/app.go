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
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicepki"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/health"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/jobs"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/lifecycle"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/reconciliation"
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
	secrets, err := secretstore.LoadLocal(cfg.MasterKeyFile)
	if err != nil {
		return fmt.Errorf("load Control Plane SecretStore: %w", err)
	}
	authService, err := auth.NewService(store, secrets, auth.DefaultArgon2Params, cfg.PublicOrigin)
	if err != nil {
		return fmt.Errorf("build local authentication service: %w", err)
	}
	environmentService, err := environment.NewService(store)
	if err != nil {
		return fmt.Errorf("build environment service: %w", err)
	}
	healthService, err := health.NewService(store)
	if err != nil {
		return fmt.Errorf("build health service: %w", err)
	}
	deviceInventoryService, err := devices.NewInventoryService(store)
	if err != nil {
		return fmt.Errorf("build device inventory service: %w", err)
	}
	serverOptions := []api.Option{
		api.WithAuthService(authService),
		api.WithEnvironmentService(environmentService),
		api.WithHealthService(healthService),
		api.WithDeviceInventoryService(deviceInventoryService),
		api.WithWebConsoleDirectory(cfg.WebConsoleDirectory),
		api.WithEnvironmentAuthorizer(api.EnvironmentAuthorizerFunc(func(
			ctx context.Context, sessionToken, csrf, origin string, mutation bool,
		) (string, error) {
			if mutation {
				session, err := authService.AuthorizeMutation(ctx, sessionToken, csrf, origin)
				return session.UserID, err
			}
			session, err := authService.AuthorizeRead(ctx, sessionToken)
			return session.UserID, err
		})),
		api.WithAuditAuthorizer(api.AuditAuthorizerFunc(func(ctx context.Context, sessionToken string) error {
			_, err := authService.AuthorizeRead(ctx, sessionToken)
			return err
		})),
		api.WithDeviceAdminAuthorizer(api.DeviceAdminAuthorizerFunc(func(
			ctx context.Context, sessionToken, _ string, csrf, origin string, mutation bool,
		) (string, error) {
			if mutation {
				session, err := authService.AuthorizeMutation(ctx, sessionToken, csrf, origin)
				return session.UserID, err
			}
			session, err := authService.AuthorizeRead(ctx, sessionToken)
			return session.UserID, err
		})),
	}
	var channelServer *devicechannel.Server
	if cfg.EnrollmentEnabled() {
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
		if cfg.DeviceChannelEnabled() {
			reconciliationService, err := reconciliation.NewService(store)
			if err != nil {
				return fmt.Errorf("build reconciliation service: %w", err)
			}
			reconciliationHandler, err := reconciliation.NewChannelHandler(reconciliationService)
			if err != nil {
				return fmt.Errorf("build reconciliation channel handler: %w", err)
			}
			healthHandler, err := devicechannel.NewHealthChannelHandler(healthService)
			if err != nil {
				return fmt.Errorf("build health channel handler: %w", err)
			}
			channelServer, err = devicechannel.NewServer(devicechannel.Config{
				Address: cfg.DeviceChannelAddress, TLSCertificateFile: cfg.TLSCertificateFile,
				TLSPrivateKeyFile: cfg.TLSPrivateKeyFile, DeviceCAPEM: authority.CertificatePEM(),
				Verifier: deviceService, Reconciliation: reconciliationHandler, Health: healthHandler, Logger: logger,
			})
			if err != nil {
				return fmt.Errorf("build device channel server: %w", err)
			}
		}
	}
	components, err := lifecycle.New(
		auth.New(),
		environment.New(),
		devices.New(),
		reconciliation.New(),
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
	if channelServer != nil {
		if err := channelServer.Start(); err != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
			defer cancel()
			return errors.Join(fmt.Errorf("start device channel server: %w", err), server.Shutdown(shutdownCtx), components.Stop(shutdownCtx))
		}
		logger.InfoContext(ctx, "Device channel started", "device_channel_address", channelServer.Address())
	}
	logger.InfoContext(ctx, "Control Plane started", "http_address", server.Address())

	var runErr error
	var channelErrors <-chan error
	if channelServer != nil {
		channelErrors = channelServer.Errors()
	}
	select {
	case <-ctx.Done():
		logger.Info("Control Plane shutdown requested")
	case serveErr := <-server.Errors():
		if serveErr != nil {
			runErr = fmt.Errorf("serve HTTP: %w", serveErr)
		} else {
			runErr = errors.New("HTTP server stopped unexpectedly")
		}
	case serveErr := <-channelErrors:
		if serveErr != nil {
			runErr = fmt.Errorf("serve device channel: %w", serveErr)
		} else {
			runErr = errors.New("device channel server stopped unexpectedly")
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	var channelShutdownErr error
	if channelServer != nil {
		channelShutdownErr = channelServer.Shutdown(shutdownCtx)
	}
	componentErr := components.Stop(shutdownCtx)
	return errors.Join(runErr, channelShutdownErr, shutdownErr, componentErr)
}
