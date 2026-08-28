package storage

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ environment.Repository = (*Store)(nil)

func (s *Store) Organization(ctx context.Context) (environment.Organization, error) {
	row, err := s.queries.GetOrganizationSingleton(ctx)
	if err != nil {
		return environment.Organization{}, fmt.Errorf("load organization singleton: %w", err)
	}
	return environment.Organization{
		OrganizationID: row.OrganizationID,
		CreatedAt:      row.CreatedAt.Time.UTC(),
	}, nil
}

func (s *Store) ListEnvironments(ctx context.Context, limit int32) ([]environment.Environment, error) {
	rows, err := s.queries.ListEnvironments(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	items := make([]environment.Environment, 0, len(rows))
	for _, row := range rows {
		items = append(items, environmentFromListRow(row))
	}
	return items, nil
}

func (s *Store) Environment(ctx context.Context, environmentID string) (environment.Environment, error) {
	id, err := parseUUID(environmentID)
	if err != nil {
		return environment.Environment{}, environment.ErrNotFound
	}
	row, err := s.queries.GetEnvironment(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return environment.Environment{}, environment.ErrNotFound
	}
	if err != nil {
		return environment.Environment{}, fmt.Errorf("load environment: %w", err)
	}
	return environmentFromGetRow(row), nil
}

func (s *Store) CreateEnvironment(
	ctx context.Context,
	name environment.NormalizedName,
	mutation environment.Mutation,
) (environment.Environment, error) {
	var result environment.Environment
	_, err := s.withAuditedMutation(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(
		ctx context.Context,
		queries *dbgen.Queries,
	) (audit.Event, error) {
		row, err := queries.CreateEnvironment(ctx, dbgen.CreateEnvironmentParams{
			DisplayName: name.DisplayName,
			NameKey:     name.NameKey,
		})
		if err != nil {
			return audit.Event{}, mapEnvironmentWriteError(err, false)
		}
		result = environment.Environment{
			EnvironmentID:  row.EnvironmentID,
			OrganizationID: row.OrganizationID,
			DisplayName:    row.DisplayName,
			Revision:       row.Revision,
			Status:         environment.StatusNeedsZones,
			CreatedAt:      row.CreatedAt.Time.UTC(),
			UpdatedAt:      row.UpdatedAt.Time.UTC(),
		}
		after, err := environmentSnapshot(result)
		if err != nil {
			return audit.Event{}, err
		}
		return audit.Event{
			OccurredAt: mutation.OccurredAt,
			Actor:      audit.Actor{Type: audit.ActorTypeUser, ID: mutation.ActorID},
			Action:     audit.ActionEnvironmentCreated,
			Object: audit.ObjectRef{
				Type: audit.ObjectTypeEnvironment,
				ID:   result.EnvironmentID,
			},
			CorrelationID: result.EnvironmentID,
			RequestID:     mutation.RequestID,
			After:         &after,
		}, nil
	})
	if err != nil {
		return environment.Environment{}, err
	}
	return result, nil
}

func (s *Store) UpdateEnvironment(
	ctx context.Context,
	environmentID string,
	name environment.NormalizedName,
	expectedRevision int64,
	mutation environment.Mutation,
) (environment.Environment, error) {
	id, err := parseUUID(environmentID)
	if err != nil {
		return environment.Environment{}, environment.ErrNotFound
	}
	var result environment.Environment
	_, err = s.withAuditedMutation(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(
		ctx context.Context,
		queries *dbgen.Queries,
	) (audit.Event, error) {
		beforeRow, err := queries.GetEnvironmentForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return audit.Event{}, environment.ErrNotFound
		}
		if err != nil {
			return audit.Event{}, fmt.Errorf("lock environment: %w", err)
		}
		if beforeRow.Revision != expectedRevision {
			return audit.Event{}, environment.ErrPreconditionFailed
		}
		beforeProjection, err := queries.GetEnvironment(ctx, id)
		if err != nil {
			return audit.Event{}, fmt.Errorf("load environment snapshot: %w", err)
		}
		before := environmentFromGetRow(beforeProjection)
		if _, err := queries.UpdateEnvironment(ctx, dbgen.UpdateEnvironmentParams{
			EnvironmentID: id,
			DisplayName:   name.DisplayName,
			NameKey:       name.NameKey,
		}); err != nil {
			return audit.Event{}, mapEnvironmentWriteError(err, false)
		}
		current, err := queries.GetEnvironment(ctx, id)
		if err != nil {
			return audit.Event{}, fmt.Errorf("reload updated environment: %w", err)
		}
		result = environmentFromGetRow(current)
		beforeSnapshot, err := environmentSnapshot(before)
		if err != nil {
			return audit.Event{}, err
		}
		afterSnapshot, err := environmentSnapshot(result)
		if err != nil {
			return audit.Event{}, err
		}
		return audit.Event{
			OccurredAt: mutation.OccurredAt,
			Actor:      audit.Actor{Type: audit.ActorTypeUser, ID: mutation.ActorID},
			Action:     audit.ActionEnvironmentUpdated,
			Object: audit.ObjectRef{
				Type: audit.ObjectTypeEnvironment,
				ID:   result.EnvironmentID,
			},
			CorrelationID: result.EnvironmentID,
			RequestID:     mutation.RequestID,
			Before:        &beforeSnapshot,
			After:         &afterSnapshot,
		}, nil
	})
	if err != nil {
		return environment.Environment{}, err
	}
	return result, nil
}

func (s *Store) ListZones(ctx context.Context, environmentID string, limit int32) ([]environment.Zone, error) {
	environmentUUID, err := parseUUID(environmentID)
	if err != nil {
		return nil, environment.ErrNotFound
	}
	if _, err := s.queries.GetEnvironment(ctx, environmentUUID); errors.Is(err, pgx.ErrNoRows) {
		return nil, environment.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("validate environment scope: %w", err)
	}
	rows, err := s.queries.ListZones(ctx, dbgen.ListZonesParams{EnvironmentID: environmentUUID, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("list environment zones: %w", err)
	}
	items := make([]environment.Zone, 0, len(rows))
	for _, row := range rows {
		items = append(items, zoneFromListRow(row))
	}
	return items, nil
}

func (s *Store) Zone(ctx context.Context, environmentID, zoneID string) (environment.Zone, error) {
	environmentUUID, err := parseUUID(environmentID)
	if err != nil {
		return environment.Zone{}, environment.ErrNotFound
	}
	zoneUUID, err := parseUUID(zoneID)
	if err != nil {
		return environment.Zone{}, environment.ErrNotFound
	}
	row, err := s.queries.GetZone(ctx, dbgen.GetZoneParams{EnvironmentID: environmentUUID, ZoneID: zoneUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return environment.Zone{}, environment.ErrNotFound
	}
	if err != nil {
		return environment.Zone{}, fmt.Errorf("load environment zone: %w", err)
	}
	return zoneFromGetRow(row), nil
}

func (s *Store) CreateZone(
	ctx context.Context,
	environmentID string,
	name environment.NormalizedName,
	cidr string,
	mutation environment.Mutation,
) (environment.Zone, error) {
	environmentUUID, prefix, err := environmentStorageInput(environmentID, cidr)
	if err != nil {
		return environment.Zone{}, err
	}
	var result environment.Zone
	_, err = s.withAuditedMutation(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(
		ctx context.Context,
		queries *dbgen.Queries,
	) (audit.Event, error) {
		if err := lockEnvironmentZones(ctx, queries, environmentID, environmentUUID); err != nil {
			return audit.Event{}, err
		}
		overlaps, err := queries.ZoneOverlapExists(ctx, dbgen.ZoneOverlapExistsParams{
			EnvironmentID: environmentUUID,
			Network:       prefix,
			ExcludeZoneID: pgtype.UUID{},
		})
		if err != nil {
			return audit.Event{}, fmt.Errorf("check zone overlap: %w", err)
		}
		if overlaps {
			return audit.Event{}, environment.ErrCIDRConflict
		}
		row, err := queries.CreateZone(ctx, dbgen.CreateZoneParams{
			EnvironmentID: environmentUUID,
			DisplayName:   name.DisplayName,
			NameKey:       name.NameKey,
			Network:       prefix,
		})
		if err != nil {
			return audit.Event{}, mapEnvironmentWriteError(err, true)
		}
		if err := queries.BumpEnvironmentRevision(ctx, environmentUUID); err != nil {
			return audit.Event{}, fmt.Errorf("advance environment revision: %w", err)
		}
		result = zoneFromCreateRow(row)
		after, err := zoneSnapshot(result)
		if err != nil {
			return audit.Event{}, err
		}
		return zoneAuditEvent(audit.ActionZoneCreated, mutation, result.ZoneID, nil, &after), nil
	})
	if err != nil {
		return environment.Zone{}, err
	}
	return result, nil
}

func (s *Store) UpdateZone(
	ctx context.Context,
	environmentID, zoneID string,
	name environment.NormalizedName,
	cidr string,
	expectedRevision int64,
	mutation environment.Mutation,
) (environment.Zone, error) {
	environmentUUID, prefix, err := environmentStorageInput(environmentID, cidr)
	if err != nil {
		return environment.Zone{}, err
	}
	zoneUUID, err := parseUUID(zoneID)
	if err != nil {
		return environment.Zone{}, environment.ErrNotFound
	}
	var result environment.Zone
	_, err = s.withAuditedMutation(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(
		ctx context.Context,
		queries *dbgen.Queries,
	) (audit.Event, error) {
		if err := lockEnvironmentZones(ctx, queries, environmentID, environmentUUID); err != nil {
			return audit.Event{}, err
		}
		beforeRow, err := queries.GetZoneForUpdate(ctx, dbgen.GetZoneForUpdateParams{
			EnvironmentID: environmentUUID,
			ZoneID:        zoneUUID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return audit.Event{}, environment.ErrNotFound
		}
		if err != nil {
			return audit.Event{}, fmt.Errorf("lock environment zone: %w", err)
		}
		if beforeRow.Revision != expectedRevision {
			return audit.Event{}, environment.ErrPreconditionFailed
		}
		overlaps, err := queries.ZoneOverlapExists(ctx, dbgen.ZoneOverlapExistsParams{
			EnvironmentID: environmentUUID,
			Network:       prefix,
			ExcludeZoneID: zoneUUID,
		})
		if err != nil {
			return audit.Event{}, fmt.Errorf("check updated zone overlap: %w", err)
		}
		if overlaps {
			return audit.Event{}, environment.ErrCIDRConflict
		}
		before := environment.Zone{
			ZoneID:        beforeRow.ZoneID,
			EnvironmentID: beforeRow.EnvironmentID,
			DisplayName:   beforeRow.DisplayName,
			CIDR:          beforeRow.Cidr,
			Revision:      beforeRow.Revision,
			CreatedAt:     beforeRow.CreatedAt.Time.UTC(),
			UpdatedAt:     beforeRow.UpdatedAt.Time.UTC(),
		}
		row, err := queries.UpdateZone(ctx, dbgen.UpdateZoneParams{
			DisplayName:   name.DisplayName,
			NameKey:       name.NameKey,
			Network:       prefix,
			EnvironmentID: environmentUUID,
			ZoneID:        zoneUUID,
		})
		if err != nil {
			return audit.Event{}, mapEnvironmentWriteError(err, true)
		}
		if err := queries.BumpEnvironmentRevision(ctx, environmentUUID); err != nil {
			return audit.Event{}, fmt.Errorf("advance environment revision: %w", err)
		}
		result = zoneFromUpdateRow(row)
		beforeSnapshot, err := zoneSnapshot(before)
		if err != nil {
			return audit.Event{}, err
		}
		afterSnapshot, err := zoneSnapshot(result)
		if err != nil {
			return audit.Event{}, err
		}
		return zoneAuditEvent(
			audit.ActionZoneUpdated,
			mutation,
			result.ZoneID,
			&beforeSnapshot,
			&afterSnapshot,
		), nil
	})
	if err != nil {
		return environment.Zone{}, err
	}
	return result, nil
}

func (s *Store) RemoveZone(
	ctx context.Context,
	environmentID, zoneID string,
	expectedRevision int64,
	mutation environment.Mutation,
) error {
	environmentUUID, err := parseUUID(environmentID)
	if err != nil {
		return environment.ErrNotFound
	}
	zoneUUID, err := parseUUID(zoneID)
	if err != nil {
		return environment.ErrNotFound
	}
	_, err = s.withAuditedMutation(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(
		ctx context.Context,
		queries *dbgen.Queries,
	) (audit.Event, error) {
		if err := lockEnvironmentZones(ctx, queries, environmentID, environmentUUID); err != nil {
			return audit.Event{}, err
		}
		beforeRow, err := queries.GetZoneForUpdate(ctx, dbgen.GetZoneForUpdateParams{
			EnvironmentID: environmentUUID,
			ZoneID:        zoneUUID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return audit.Event{}, environment.ErrNotFound
		}
		if err != nil {
			return audit.Event{}, fmt.Errorf("lock environment zone for removal: %w", err)
		}
		if beforeRow.Revision != expectedRevision {
			return audit.Event{}, environment.ErrPreconditionFailed
		}
		before := environment.Zone{
			ZoneID:        beforeRow.ZoneID,
			EnvironmentID: beforeRow.EnvironmentID,
			DisplayName:   beforeRow.DisplayName,
			CIDR:          beforeRow.Cidr,
			Revision:      beforeRow.Revision,
			CreatedAt:     beforeRow.CreatedAt.Time.UTC(),
			UpdatedAt:     beforeRow.UpdatedAt.Time.UTC(),
		}
		rows, err := queries.DeleteZone(ctx, dbgen.DeleteZoneParams{EnvironmentID: environmentUUID, ZoneID: zoneUUID})
		if err != nil {
			return audit.Event{}, fmt.Errorf("remove environment zone: %w", err)
		}
		if rows != 1 {
			return audit.Event{}, environment.ErrNotFound
		}
		if err := queries.BumpEnvironmentRevision(ctx, environmentUUID); err != nil {
			return audit.Event{}, fmt.Errorf("advance environment revision: %w", err)
		}
		beforeSnapshot, err := zoneSnapshot(before)
		if err != nil {
			return audit.Event{}, err
		}
		return zoneAuditEvent(audit.ActionZoneRemoved, mutation, before.ZoneID, &beforeSnapshot, nil), nil
	})
	return err
}

func lockEnvironmentZones(
	ctx context.Context,
	queries *dbgen.Queries,
	environmentID string,
	environmentUUID pgtype.UUID,
) error {
	if err := queries.LockEnvironmentZoneSet(ctx, environmentID); err != nil {
		return fmt.Errorf("lock environment zone set: %w", err)
	}
	if _, err := queries.GetEnvironmentForUpdate(ctx, environmentUUID); errors.Is(err, pgx.ErrNoRows) {
		return environment.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("lock environment: %w", err)
	}
	return nil
}

func environmentStorageInput(environmentID, cidr string) (pgtype.UUID, netip.Prefix, error) {
	environmentUUID, err := parseUUID(environmentID)
	if err != nil {
		return pgtype.UUID{}, netip.Prefix{}, environment.ErrNotFound
	}
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return pgtype.UUID{}, netip.Prefix{}, environment.ErrInvalidInput
	}
	return environmentUUID, prefix, nil
}

func mapEnvironmentWriteError(err error, zone bool) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	if pgErr.Code == "23505" {
		if zone && pgErr.ConstraintName == "zones_environment_id_network_key" {
			return environment.ErrCIDRConflict
		}
		return environment.ErrNameConflict
	}
	if pgErr.Code == "23503" {
		return environment.ErrNotFound
	}
	return err
}

func environmentFromListRow(row dbgen.ListEnvironmentsRow) environment.Environment {
	return environment.Environment{
		EnvironmentID:  row.EnvironmentID,
		OrganizationID: row.OrganizationID,
		DisplayName:    row.DisplayName,
		Revision:       row.Revision,
		ZoneCount:      row.ZoneCount,
		Status:         environment.Status(row.Status),
		CreatedAt:      row.CreatedAt.Time.UTC(),
		UpdatedAt:      row.UpdatedAt.Time.UTC(),
	}
}

func environmentFromGetRow(row dbgen.GetEnvironmentRow) environment.Environment {
	return environment.Environment{
		EnvironmentID:  row.EnvironmentID,
		OrganizationID: row.OrganizationID,
		DisplayName:    row.DisplayName,
		Revision:       row.Revision,
		ZoneCount:      row.ZoneCount,
		Status:         environment.Status(row.Status),
		CreatedAt:      row.CreatedAt.Time.UTC(),
		UpdatedAt:      row.UpdatedAt.Time.UTC(),
	}
}

func zoneFromListRow(row dbgen.ListZonesRow) environment.Zone {
	return environment.Zone{
		ZoneID:        row.ZoneID,
		EnvironmentID: row.EnvironmentID,
		DisplayName:   row.DisplayName,
		CIDR:          row.Cidr,
		Revision:      row.Revision,
		CreatedAt:     row.CreatedAt.Time.UTC(),
		UpdatedAt:     row.UpdatedAt.Time.UTC(),
	}
}

func zoneFromGetRow(row dbgen.GetZoneRow) environment.Zone {
	return environment.Zone{
		ZoneID:        row.ZoneID,
		EnvironmentID: row.EnvironmentID,
		DisplayName:   row.DisplayName,
		CIDR:          row.Cidr,
		Revision:      row.Revision,
		CreatedAt:     row.CreatedAt.Time.UTC(),
		UpdatedAt:     row.UpdatedAt.Time.UTC(),
	}
}

func zoneFromCreateRow(row dbgen.CreateZoneRow) environment.Zone {
	return environment.Zone{
		ZoneID:        row.ZoneID,
		EnvironmentID: row.EnvironmentID,
		DisplayName:   row.DisplayName,
		CIDR:          row.Cidr,
		Revision:      row.Revision,
		CreatedAt:     row.CreatedAt.Time.UTC(),
		UpdatedAt:     row.UpdatedAt.Time.UTC(),
	}
}

func zoneFromUpdateRow(row dbgen.UpdateZoneRow) environment.Zone {
	return environment.Zone{
		ZoneID:        row.ZoneID,
		EnvironmentID: row.EnvironmentID,
		DisplayName:   row.DisplayName,
		CIDR:          row.Cidr,
		Revision:      row.Revision,
		CreatedAt:     row.CreatedAt.Time.UTC(),
		UpdatedAt:     row.UpdatedAt.Time.UTC(),
	}
}

func environmentSnapshot(value environment.Environment) (audit.Snapshot, error) {
	return audit.NewSnapshot(map[string]any{
		"environment_id":  value.EnvironmentID,
		"organization_id": value.OrganizationID,
		"display_name":    value.DisplayName,
		"revision":        value.Revision,
		"zone_count":      value.ZoneCount,
		"status":          value.Status,
	})
}

func zoneSnapshot(value environment.Zone) (audit.Snapshot, error) {
	return audit.NewSnapshot(map[string]any{
		"zone_id":        value.ZoneID,
		"environment_id": value.EnvironmentID,
		"display_name":   value.DisplayName,
		"cidr":           value.CIDR,
		"revision":       value.Revision,
	})
}

func zoneAuditEvent(
	action audit.Action,
	mutation environment.Mutation,
	zoneID string,
	before, after *audit.Snapshot,
) audit.Event {
	return audit.Event{
		OccurredAt:    mutation.OccurredAt,
		Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: mutation.ActorID},
		Action:        action,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeZone, ID: zoneID},
		CorrelationID: zoneID,
		RequestID:     mutation.RequestID,
		Before:        before,
		After:         after,
	}
}
