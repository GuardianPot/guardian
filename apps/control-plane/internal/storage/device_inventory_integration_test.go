//go:build integration

package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
)

func TestDeviceInventoryIsBoundedScopedAndIncludesOnlyActiveCertificateExpiry(t *testing.T) {
	ctx := context.Background()
	databaseURL := createTestDatabase(t)
	if _, err := Migrate(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, databaseURL, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	environmentService, err := environment.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	configured, err := environmentService.CreateEnvironment(ctx, "Inventory integration", environment.Mutation{ActorID: "owner-1"})
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)
	expiresAt := createdAt.Add(24 * time.Hour)
	deviceIDs := []string{
		"018f1f7e-6d31-7cc5-8db8-17547f78e6c1",
		"018f1f7e-6d31-7cc5-8db8-17547f78e6c2",
	}
	for index, deviceID := range deviceIDs {
		state := devices.DevicePending
		if index == 1 {
			state = devices.DeviceActive
		}
		if _, err := store.pool.Exec(ctx, `
INSERT INTO guardian_devices.devices (device_id, environment_id, display_name, state, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)`, deviceID, configured.EnvironmentID, "edge-inventory-"+string(rune('a'+index)), state, createdAt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.pool.Exec(ctx, `
INSERT INTO guardian_devices.certificates (
  serial, device_id, fingerprint_sha256, certificate_pem, not_before, not_after, state, created_at
) VALUES ('01', $1, $2, $3, $4, $5, 'active', $4)`,
		deviceIDs[1], "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", []byte("certificate"), createdAt, expiresAt); err != nil {
		t.Fatal(err)
	}

	items, err := store.ListInventoryDevices(ctx, configured.EnvironmentID, 200)
	if err != nil || len(items) != 2 || items[0].DeviceID != deviceIDs[0] || items[1].DeviceID != deviceIDs[1] {
		t.Fatalf("inventory list = (%+v, %v)", items, err)
	}
	if items[0].ActiveCertificateExpiresAt != nil || items[1].ActiveCertificateExpiresAt == nil || !items[1].ActiveCertificateExpiresAt.Equal(expiresAt) {
		t.Fatalf("inventory certificate expiries = (%v, %v)", items[0].ActiveCertificateExpiresAt, items[1].ActiveCertificateExpiresAt)
	}
	item, err := store.InventoryDevice(ctx, configured.EnvironmentID, deviceIDs[1])
	if err != nil || item.State != devices.DeviceActive || item.DisplayName != "edge-inventory-b" {
		t.Fatalf("inventory item = (%+v, %v)", item, err)
	}
	if _, err := store.InventoryDevice(ctx, "018f1f7e-6d31-7cc5-8db8-17547f78e6cf", deviceIDs[1]); !errors.Is(err, devices.ErrNotFound) {
		t.Fatalf("cross-environment inventory error = %v", err)
	}
	if _, err := store.ListInventoryDevices(ctx, "018f1f7e-6d31-7cc5-8db8-17547f78e6cf", 200); !errors.Is(err, devices.ErrNotFound) {
		t.Fatalf("missing environment inventory error = %v", err)
	}
	if _, err := store.ListInventoryDevices(ctx, configured.EnvironmentID, 201); !errors.Is(err, devices.ErrInvalidInput) {
		t.Fatalf("unbounded inventory error = %v", err)
	}
}
