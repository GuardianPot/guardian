package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
	"github.com/jackc/pgx/v5"
)

var _ devices.InventoryRepository = (*Store)(nil)

func (s *Store) ListInventoryDevices(ctx context.Context, environmentID string, limit int32) ([]devices.InventoryDevice, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (
SELECT 1 FROM guardian_environment.environments WHERE environment_id = $1
)`, environmentID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("verify device inventory environment: %w", err)
	}
	if !exists {
		return nil, devices.ErrNotFound
	}
	if limit < 1 || limit > 200 {
		return nil, devices.ErrInvalidInput
	}
	rows, err := s.pool.Query(ctx, `
SELECT d.device_id::text, d.environment_id::text, d.display_name, d.state,
       d.created_at, d.updated_at, c.not_after
FROM guardian_devices.devices d
LEFT JOIN guardian_devices.certificates c
  ON c.device_id = d.device_id AND c.state = 'active'
WHERE d.environment_id = $1
ORDER BY d.device_id
LIMIT $2`, environmentID, limit)
	if err != nil {
		return nil, fmt.Errorf("list device inventory: %w", err)
	}
	defer rows.Close()
	result := make([]devices.InventoryDevice, 0)
	for rows.Next() {
		var item devices.InventoryDevice
		if err := rows.Scan(
			&item.DeviceID, &item.EnvironmentID, &item.DisplayName, &item.State,
			&item.CreatedAt, &item.UpdatedAt, &item.ActiveCertificateExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan device inventory: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate device inventory: %w", err)
	}
	return result, nil
}

func (s *Store) InventoryDevice(ctx context.Context, environmentID, deviceID string) (devices.InventoryDevice, error) {
	var item devices.InventoryDevice
	err := s.pool.QueryRow(ctx, `
SELECT d.device_id::text, d.environment_id::text, d.display_name, d.state,
       d.created_at, d.updated_at, c.not_after
FROM guardian_devices.devices d
LEFT JOIN guardian_devices.certificates c
  ON c.device_id = d.device_id AND c.state = 'active'
WHERE d.environment_id = $1 AND d.device_id = $2`, environmentID, deviceID).Scan(
		&item.DeviceID, &item.EnvironmentID, &item.DisplayName, &item.State,
		&item.CreatedAt, &item.UpdatedAt, &item.ActiveCertificateExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return devices.InventoryDevice{}, devices.ErrNotFound
	}
	if err != nil {
		return devices.InventoryDevice{}, fmt.Errorf("read device inventory item: %w", err)
	}
	return item, nil
}
