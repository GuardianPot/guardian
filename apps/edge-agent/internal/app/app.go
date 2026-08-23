// Package app composes the unprivileged Edge daemon without hiding lifecycle
// order or secure-identity failure.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/components"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/config"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/lifecycle"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
	"go.opentelemetry.io/otel"
)

const tracerName = "github.com/GuardianPot/guardian/apps/edge-agent"

// Run validates the protected identity before creating state, starts the fixed
// component graph, and drains it on cancellation.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) (runErr error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "edge-agent.run")
	defer span.End()

	metadata, err := identity.Load(cfg.IdentityCertPath, cfg.IdentityKeyPath, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("load secure Edge identity: %w", err)
	}
	store, err := storage.Open(ctx, storage.Options{
		DatabasePath: cfg.DatabasePath, SpoolDirectory: cfg.SpoolDirectory,
	})
	if err != nil {
		return fmt.Errorf("open Edge state: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close Edge state: %w", err))
		}
	}()
	if err := store.SetIdentity(ctx, storage.IdentityRecord{
		CertificateSHA256: metadata.CertificateSHA256,
		NotBefore:         metadata.NotBefore,
		NotAfter:          metadata.NotAfter,
		EnrollmentStatus:  "enrolled",
	}); err != nil {
		return fmt.Errorf("record Edge identity state: %w", err)
	}
	if err := store.SetHealth(ctx, storage.HealthCondition{Name: "process", Status: "unknown", ReasonCode: "starting"}); err != nil {
		return fmt.Errorf("record Edge startup state: %w", err)
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("read Edge identity state: %w", err)
	}
	enrollmentStatus := "unavailable"
	if snapshot.Identity != nil {
		enrollmentStatus = snapshot.Identity.EnrollmentStatus
	}

	graph := components.NewFoundation(store, metadata, enrollmentStatus)
	manager, err := lifecycle.New(graph.Ordered()...)
	if err != nil {
		return fmt.Errorf("build Edge component graph: %w", err)
	}
	if err := manager.Start(ctx); err != nil {
		_ = store.SetHealth(context.Background(), storage.HealthCondition{Name: "process", Status: "failed", ReasonCode: "component-start-failed"})
		return err
	}
	if err := store.SetHealth(ctx, storage.HealthCondition{Name: "process", Status: "healthy", ReasonCode: "running"}); err != nil {
		return errors.Join(err, manager.Stop(context.Background()))
	}
	logger.InfoContext(ctx, "Edge daemon started",
		"control_plane_endpoint", cfg.ControlPlaneEndpoint,
		"certificate_sha256", metadata.CertificateSHA256,
	)

	<-ctx.Done()
	logger.Info("Edge daemon shutdown requested")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout())
	defer cancel()
	stopErr := manager.Stop(shutdownCtx)
	healthErr := store.SetHealth(shutdownCtx, storage.HealthCondition{Name: "process", Status: "stopped", ReasonCode: "shutdown"})
	return errors.Join(stopErr, healthErr)
}
