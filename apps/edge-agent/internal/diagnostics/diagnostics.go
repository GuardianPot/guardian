// Package diagnostics produces bounded, read-only, redaction-safe local output.
package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/config"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
)

const maxOutputBytes = 64 << 10

// ConfigSummary intentionally omits database, spool, certificate, and key paths.
type ConfigSummary struct {
	ControlPlaneEndpoint string `json:"control_plane_endpoint"`
	IdentityConfigured   bool   `json:"identity_configured"`
	StorageConfigured    bool   `json:"storage_configured"`
}

// Report is the bounded machine-readable support view.
type Report struct {
	Version     string           `json:"version"`
	GeneratedAt time.Time        `json:"generated_at"`
	Config      ConfigSummary    `json:"config"`
	State       storage.Snapshot `json:"state"`
}

// Collect opens SQLite read-only and never creates or repairs state.
func Collect(ctx context.Context, cfg config.Config, version string, now time.Time) (Report, error) {
	store, err := storage.OpenReadOnly(ctx, storage.Options{
		DatabasePath: cfg.DatabasePath, SpoolDirectory: cfg.SpoolDirectory,
	})
	if err != nil {
		return Report{}, err
	}
	defer store.Close()
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		return Report{}, err
	}
	return Report{
		Version:     version,
		GeneratedAt: now.UTC(),
		Config: ConfigSummary{
			ControlPlaneEndpoint: cfg.ControlPlaneEndpoint,
			IdentityConfigured:   cfg.IdentityCertPath != "" && cfg.IdentityKeyPath != "",
			StorageConfigured:    cfg.DatabasePath != "" && cfg.SpoolDirectory != "",
		},
		State: snapshot,
	}, nil
}

// Write renders either JSON or a concise text status and enforces a hard output
// bound before writing anything to the caller.
func Write(writer io.Writer, report Report, format string) error {
	var buffer bytes.Buffer
	switch format {
	case "json":
		encoder := json.NewEncoder(&buffer)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return fmt.Errorf("encode Edge diagnostics: %w", err)
		}
	case "text":
		enrollment := "unavailable"
		if report.State.Identity != nil {
			enrollment = report.State.Identity.EnrollmentStatus
		}
		if _, err := fmt.Fprintf(&buffer,
			"version=%s schema=%d enrollment=%s endpoint=%s queue_pending=%d queue_inflight=%d spool_objects=%d spool_bytes=%d\n",
			report.Version,
			report.State.SchemaVersion,
			enrollment,
			report.Config.ControlPlaneEndpoint,
			report.State.Queue.Pending,
			report.State.Queue.Inflight,
			report.State.Spool.Objects,
			report.State.Spool.Bytes,
		); err != nil {
			return err
		}
		for _, condition := range report.State.Health {
			if _, err := fmt.Fprintf(&buffer, "health.%s=%s reason=%s\n", condition.Name, condition.Status, condition.ReasonCode); err != nil {
				return err
			}
		}
	default:
		return errors.New("diagnostic format must be text or json")
	}
	if buffer.Len() > maxOutputBytes {
		return fmt.Errorf("diagnostic output exceeds %d bytes", maxOutputBytes)
	}
	_, err := writer.Write(buffer.Bytes())
	return err
}
