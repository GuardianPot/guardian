package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/deception"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ deception.Repository = (*Store)(nil)

// withAuditedDecoyMutation commits a decoy write only when its audit event is
// valid and appended in the same transaction. It mirrors withAuditedMutation
// but hands the callback the pgx transaction directly, because the decoy
// domain is expressed in hand-written SQL rather than generated queries.
func (s *Store) withAuditedDecoyMutation(
	ctx context.Context,
	mutation func(context.Context, pgx.Tx) (audit.Event, error),
) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin decoy transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	event, err := mutation(ctx, tx)
	if err != nil {
		return err
	}
	if _, err := (auditQueryAppender{queries: dbgen.New(tx)}).Append(ctx, event); err != nil {
		return fmt.Errorf("append decoy audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit decoy mutation: %w", err)
	}
	return nil
}

// decoyColumns is the desired object. Observed state is never selected here:
// the two are joined by the caller into separate response fields, never into
// one row that could be read as a single truth.
const decoyColumns = `
    d.decoy_id::text, d.environment_id::text, d.zone_id::text, d.display_name,
    d.decoy_family, d.persona, d.interaction_level, host(d.address),
    d.pack, d.pack_version, COALESCE(d.pack_digest, ''), d.desired_state,
    d.revision, d.created_at, d.updated_at`

func (s *Store) ListDecoys(ctx context.Context, environmentID string, limit int32) ([]deception.View, error) {
	rows, err := s.pool.Query(ctx, `
SELECT`+decoyColumns+`
FROM guardian_deception.decoys d
JOIN guardian_environment.environments e ON e.environment_id = d.environment_id
WHERE d.environment_id = $1
  AND d.desired_state <> 'removed'
  AND e.organization_id = (SELECT organization_id FROM guardian_environment.organizations WHERE singleton = true)
ORDER BY d.name_key, d.decoy_id
LIMIT $2`, environmentID, limit)
	if err != nil {
		return nil, fmt.Errorf("list decoys: %w", err)
	}
	decoys, err := scanDecoys(rows)
	if err != nil {
		return nil, err
	}
	if len(decoys) == 0 {
		// Distinguish an empty environment from one that does not exist: the
		// console must not render "no decoys" for an identity that is not there.
		var exists bool
		if err := s.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1 FROM guardian_environment.environments
    WHERE environment_id = $1
      AND organization_id = (SELECT organization_id FROM guardian_environment.organizations WHERE singleton = true)
)`, environmentID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("validate decoy environment scope: %w", err)
		}
		if !exists {
			return nil, deception.ErrNotFound
		}
		return []deception.View{}, nil
	}
	identifiers := make([]string, 0, len(decoys))
	for _, decoy := range decoys {
		identifiers = append(identifiers, decoy.DecoyID)
	}
	observed, err := s.observations(ctx, identifiers)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	views := make([]deception.View, 0, len(decoys))
	for _, decoy := range decoys {
		views = append(views, deception.View{Decoy: decoy, Observed: observationFor(observed, decoy.DecoyID, now)})
	}
	return views, nil
}

func (s *Store) Decoy(ctx context.Context, environmentID, decoyID string) (deception.View, error) {
	rows, err := s.pool.Query(ctx, `
SELECT`+decoyColumns+`
FROM guardian_deception.decoys d
JOIN guardian_environment.environments e ON e.environment_id = d.environment_id
WHERE d.environment_id = $1
  AND d.decoy_id = $2
  AND d.desired_state <> 'removed'
  AND e.organization_id = (SELECT organization_id FROM guardian_environment.organizations WHERE singleton = true)`,
		environmentID, decoyID)
	if err != nil {
		return deception.View{}, fmt.Errorf("load decoy: %w", err)
	}
	decoys, err := scanDecoys(rows)
	if err != nil {
		return deception.View{}, err
	}
	if len(decoys) != 1 {
		return deception.View{}, deception.ErrNotFound
	}
	observed, err := s.observations(ctx, []string{decoyID})
	if err != nil {
		return deception.View{}, err
	}
	return deception.View{
		Decoy:    decoys[0],
		Observed: observationFor(observed, decoyID, time.Now().UTC()),
	}, nil
}

func (s *Store) CreateDecoy(
	ctx context.Context,
	environmentID string,
	write deception.Write,
	mutation deception.Mutation,
) (deception.Decoy, error) {
	var result deception.Decoy
	err := s.withAuditedDecoyMutation(ctx, func(ctx context.Context, tx pgx.Tx) (audit.Event, error) {
		if err := assertAddressInZone(ctx, tx, environmentID, write.ZoneID, write.Address); err != nil {
			return audit.Event{}, err
		}
		// The device channel can carry 64 decoys per environment. Refusing the
		// 65th here means the operator learns at the moment they ask, rather
		// than by noticing later that a decoy never reached an Edge.
		var active int
		if err := tx.QueryRow(ctx, `
SELECT count(*) FROM guardian_deception.decoys
WHERE environment_id = $1 AND desired_state <> 'removed'`, environmentID).Scan(&active); err != nil {
			return audit.Event{}, fmt.Errorf("count active decoys: %w", err)
		}
		if active >= deception.MaxDecoysPerDevice {
			return audit.Event{}, deception.ErrDecoyBudgetExhausted
		}
		rows, err := tx.Query(ctx, `
INSERT INTO guardian_deception.decoys (
    environment_id, zone_id, display_name, name_key, decoy_family, persona,
    interaction_level, address, pack, pack_version, pack_digest, desired_state
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::inet, $9, $10, NULLIF($11, ''), 'deployed')
RETURNING`+returningDecoyColumns,
			environmentID, write.ZoneID, write.Name.DisplayName, write.Name.NameKey,
			string(write.Family), string(write.Persona), string(write.InteractionLevel),
			write.Address, write.Pack, write.PackVersion, write.PackDigest)
		if err != nil {
			return audit.Event{}, mapDecoyWriteError(err)
		}
		created, err := scanDecoys(rows)
		if err != nil {
			return audit.Event{}, mapDecoyWriteError(err)
		}
		if len(created) != 1 {
			return audit.Event{}, deception.ErrNotFound
		}
		result = created[0]
		after, err := decoySnapshot(result)
		if err != nil {
			return audit.Event{}, err
		}
		return decoyAuditEvent(audit.ActionDecoyCreated, mutation, result.DecoyID, nil, &after), nil
	})
	if err != nil {
		return deception.Decoy{}, err
	}
	return result, nil
}

func (s *Store) UpdateDecoy(
	ctx context.Context,
	environmentID, decoyID string,
	write deception.Write,
	expectedRevision int64,
	mutation deception.Mutation,
) (deception.Decoy, error) {
	var result deception.Decoy
	err := s.withAuditedDecoyMutation(ctx, func(ctx context.Context, tx pgx.Tx) (audit.Event, error) {
		before, err := lockDecoy(ctx, tx, environmentID, decoyID, expectedRevision)
		if err != nil {
			return audit.Event{}, err
		}
		if err := assertAddressInZone(ctx, tx, environmentID, write.ZoneID, write.Address); err != nil {
			return audit.Event{}, err
		}
		rows, err := tx.Query(ctx, `
UPDATE guardian_deception.decoys AS d
SET zone_id = $3, display_name = $4, name_key = $5, decoy_family = $6, persona = $7,
    interaction_level = $8, address = $9::inet, pack = $10, pack_version = $11,
    pack_digest = NULLIF($12, ''), revision = revision + 1, updated_at = clock_timestamp()
WHERE d.environment_id = $1 AND d.decoy_id = $2
RETURNING`+returningDecoyColumns,
			environmentID, decoyID, write.ZoneID, write.Name.DisplayName, write.Name.NameKey,
			string(write.Family), string(write.Persona), string(write.InteractionLevel),
			write.Address, write.Pack, write.PackVersion, write.PackDigest)
		if err != nil {
			return audit.Event{}, mapDecoyWriteError(err)
		}
		updated, err := scanDecoys(rows)
		if err != nil {
			return audit.Event{}, mapDecoyWriteError(err)
		}
		if len(updated) != 1 {
			return audit.Event{}, deception.ErrNotFound
		}
		result = updated[0]
		beforeSnapshot, err := decoySnapshot(before)
		if err != nil {
			return audit.Event{}, err
		}
		afterSnapshot, err := decoySnapshot(result)
		if err != nil {
			return audit.Event{}, err
		}
		return decoyAuditEvent(audit.ActionDecoyUpdated, mutation, result.DecoyID, &beforeSnapshot, &afterSnapshot), nil
	})
	if err != nil {
		return deception.Decoy{}, err
	}
	return result, nil
}

func (s *Store) SetDecoyDesiredState(
	ctx context.Context,
	environmentID, decoyID string,
	state deception.DesiredState,
	expectedRevision int64,
	mutation deception.Mutation,
) (deception.Decoy, error) {
	action := audit.ActionDecoyEnabled
	if state == deception.DesiredDisabled {
		action = audit.ActionDecoyDisabled
	}
	if state != deception.DesiredDeployed && state != deception.DesiredDisabled {
		return deception.Decoy{}, deception.ErrInvalidInput
	}
	return s.transitionDecoy(ctx, environmentID, decoyID, state, action, expectedRevision, mutation)
}

// RemoveDecoy retires the decoy from the active list. The row survives so that
// its identity stays resolvable, and its audit trail is untouched: removal is
// not deletion of history.
func (s *Store) RemoveDecoy(
	ctx context.Context,
	environmentID, decoyID string,
	expectedRevision int64,
	mutation deception.Mutation,
) (deception.Decoy, error) {
	return s.transitionDecoy(
		ctx, environmentID, decoyID, deception.DesiredRemoved,
		audit.ActionDecoyRemoved, expectedRevision, mutation,
	)
}

func (s *Store) transitionDecoy(
	ctx context.Context,
	environmentID, decoyID string,
	state deception.DesiredState,
	action audit.Action,
	expectedRevision int64,
	mutation deception.Mutation,
) (deception.Decoy, error) {
	var result deception.Decoy
	err := s.withAuditedDecoyMutation(ctx, func(ctx context.Context, tx pgx.Tx) (audit.Event, error) {
		before, err := lockDecoy(ctx, tx, environmentID, decoyID, expectedRevision)
		if err != nil {
			return audit.Event{}, err
		}
		rows, err := tx.Query(ctx, `
UPDATE guardian_deception.decoys AS d
SET desired_state = $3, revision = revision + 1, updated_at = clock_timestamp()
WHERE d.environment_id = $1 AND d.decoy_id = $2
RETURNING`+returningDecoyColumns, environmentID, decoyID, string(state))
		if err != nil {
			return audit.Event{}, mapDecoyWriteError(err)
		}
		updated, err := scanDecoys(rows)
		if err != nil {
			return audit.Event{}, mapDecoyWriteError(err)
		}
		if len(updated) != 1 {
			return audit.Event{}, deception.ErrNotFound
		}
		result = updated[0]
		beforeSnapshot, err := decoySnapshot(before)
		if err != nil {
			return audit.Event{}, err
		}
		afterSnapshot, err := decoySnapshot(result)
		if err != nil {
			return audit.Event{}, err
		}
		return decoyAuditEvent(action, mutation, result.DecoyID, &beforeSnapshot, &afterSnapshot), nil
	})
	if err != nil {
		return deception.Decoy{}, err
	}
	return result, nil
}

// RecordObservations writes observed decoy truth. It is reached only from the
// device channel, and it refuses observations for decoys the reporting device's
// environment does not own, so one Edge cannot report on another's decoys.
func (s *Store) RecordObservations(ctx context.Context, report deception.Report) error {
	if err := deception.ValidateReport(report); err != nil {
		return err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin decoy observation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var environmentID string
	if err := tx.QueryRow(ctx, `
SELECT environment_id::text
FROM guardian_devices.devices
WHERE device_id = $1 AND state = 'active'`, report.DeviceID).Scan(&environmentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return deception.ErrNotFound
		}
		return fmt.Errorf("resolve reporting device environment: %w", err)
	}
	for _, observation := range report.Observations {
		var owned bool
		if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1 FROM guardian_deception.decoys
    WHERE decoy_id = $1 AND environment_id = $2 AND desired_state <> 'removed'
)`, observation.DecoyID, environmentID).Scan(&owned); err != nil {
			return fmt.Errorf("verify observed decoy scope: %w", err)
		}
		if !owned {
			// Silently ignoring is the wrong shape here: a device reporting on a
			// decoy it does not own is a contract violation, not a race.
			return deception.ErrNotFound
		}
		if err := writeObservation(ctx, tx, report, observation); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit decoy observations: %w", err)
	}
	return nil
}

func writeObservation(ctx context.Context, tx pgx.Tx, report deception.Report, observation deception.Observation) error {
	var desiredRevision any
	if observation.DesiredRevision != nil {
		desiredRevision = int64(*observation.DesiredRevision)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO guardian_deception.decoy_observed_state (
    decoy_id, reporting_device_id, report_id, observed_state,
    last_interaction_at, desired_revision, observed_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (decoy_id) DO UPDATE SET
    reporting_device_id = excluded.reporting_device_id,
    report_id = excluded.report_id,
    observed_state = excluded.observed_state,
    last_interaction_at = excluded.last_interaction_at,
    desired_revision = excluded.desired_revision,
    observed_at = excluded.observed_at,
    reported_at = clock_timestamp()
WHERE excluded.observed_at >= guardian_deception.decoy_observed_state.observed_at`,
		observation.DecoyID, report.DeviceID, report.ReportID, string(observation.State),
		observation.LastInteractionAt, desiredRevision, report.ObservedAt); err != nil {
		return fmt.Errorf("project observed decoy state: %w", err)
	}
	for _, condition := range observation.Conditions {
		var observedRevision any
		if condition.ObservedRevision != nil {
			observedRevision = fmt.Sprintf("%d", *condition.ObservedRevision)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO guardian_deception.decoy_conditions (
    decoy_id, condition_type, status, reason_code, message,
    observed_revision, last_transition_time
) VALUES ($1, $2, $3, $4, $5, $6::numeric, $7)
ON CONFLICT (decoy_id, condition_type) DO UPDATE SET
    status = excluded.status,
    reason_code = excluded.reason_code,
    message = excluded.message,
    observed_revision = excluded.observed_revision,
    last_transition_time = excluded.last_transition_time`,
			observation.DecoyID, string(condition.Type), string(condition.Status),
			condition.Reason, condition.Message, observedRevision, condition.LastTransitionTime); err != nil {
			return fmt.Errorf("project observed decoy condition: %w", err)
		}
	}
	return nil
}

// DesiredDecoys projects the deployable decoys of one environment for the
// device channel. A removed decoy is absent rather than present as a tombstone,
// and a disabled one is still transmitted so the Edge can stop it.
func (s *Store) desiredDecoys(ctx context.Context, tx pgx.Tx, environmentID string) ([]decoyDesired, error) {
	rows, err := tx.Query(ctx, `
SELECT decoy_id::text, zone_id::text, display_name, decoy_family, persona,
       interaction_level, host(address), pack, pack_version,
       COALESCE(pack_digest, ''), desired_state, revision
FROM guardian_deception.decoys
WHERE environment_id = $1 AND desired_state <> 'removed'
ORDER BY decoy_id
LIMIT $2`, environmentID, deception.MaxDecoysPerDevice+1)
	if err != nil {
		return nil, fmt.Errorf("read desired decoys: %w", err)
	}
	defer rows.Close()
	desired := []decoyDesired{}
	for rows.Next() {
		var entry decoyDesired
		var revision int64
		if err := rows.Scan(&entry.DecoyID, &entry.ZoneID, &entry.DisplayName, &entry.Family,
			&entry.Persona, &entry.InteractionLevel, &entry.Address, &entry.Pack,
			&entry.PackVersion, &entry.PackDigest, &entry.DesiredState, &revision); err != nil {
			return nil, fmt.Errorf("scan desired decoy: %w", err)
		}
		entry.SourceRevision = uint64(revision)
		desired = append(desired, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate desired decoys: %w", err)
	}
	// Truncating here would silently drop a decoy an operator asked for, with
	// no signal anywhere. CreateDecoy refuses the 65th decoy so this cannot
	// normally happen; if it does, the snapshot fails loudly instead.
	if len(desired) > deception.MaxDecoysPerDevice {
		return nil, fmt.Errorf(
			"%w: environment holds more than the %d decoys the device channel can carry",
			deception.ErrDecoyBudgetExhausted, deception.MaxDecoysPerDevice,
		)
	}
	return desired, nil
}

type decoyDesired struct {
	DecoyID          string
	ZoneID           string
	DisplayName      string
	Family           string
	Persona          string
	InteractionLevel string
	Address          string
	Pack             string
	PackVersion      string
	PackDigest       string
	DesiredState     string
	SourceRevision   uint64
}

// observations reads observed state for the given decoys and applies SEC-06:
// a decoy whose reporting device has been disabled or revoked reads as
// unmanaged. Its last-known conditions are preserved, because the decoy may
// still be running; what Guardian has lost is the ability to manage it.
func (s *Store) observations(ctx context.Context, decoyIDs []string) (map[string]deception.Observation, error) {
	result := make(map[string]deception.Observation, len(decoyIDs))
	rows, err := s.pool.Query(ctx, `
SELECT o.decoy_id::text, o.reporting_device_id::text, o.observed_state,
       o.last_interaction_at, o.desired_revision, o.reported_at, dev.state
FROM guardian_deception.decoy_observed_state o
JOIN guardian_devices.devices dev ON dev.device_id = o.reporting_device_id
WHERE o.decoy_id = ANY(SELECT unnest($1::text[])::uuid)`, decoyIDs)
	if err != nil {
		return nil, fmt.Errorf("read observed decoy state: %w", err)
	}
	for rows.Next() {
		var (
			observation     deception.Observation
			lastInteraction *time.Time
			desiredRevision *int64
			reportedAt      time.Time
			state           string
			deviceState     string
		)
		if err := rows.Scan(&observation.DecoyID, &observation.ReportingDeviceID, &state,
			&lastInteraction, &desiredRevision, &reportedAt, &deviceState); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan observed decoy state: %w", err)
		}
		observation.State = deception.ObservedLifecycle(state)
		if deviceState == "disabled" || deviceState == "revoked" {
			observation.State = deception.ObservedUnmanaged
		}
		reported := reportedAt.UTC()
		observation.ReportedAt = &reported
		if lastInteraction != nil {
			value := lastInteraction.UTC()
			observation.LastInteractionAt = &value
		}
		if desiredRevision != nil {
			value := uint64(*desiredRevision)
			observation.DesiredRevision = &value
		}
		result[observation.DecoyID] = observation
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate observed decoy state: %w", err)
	}
	rows.Close()
	if len(result) == 0 {
		return result, nil
	}
	return s.attachConditions(ctx, result)
}

func (s *Store) attachConditions(
	ctx context.Context,
	observations map[string]deception.Observation,
) (map[string]deception.Observation, error) {
	identifiers := make([]string, 0, len(observations))
	for decoyID := range observations {
		identifiers = append(identifiers, decoyID)
	}
	rows, err := s.pool.Query(ctx, `
SELECT decoy_id::text, condition_type, status, reason_code, message,
       observed_revision::text, last_transition_time
FROM guardian_deception.decoy_conditions
WHERE decoy_id = ANY(SELECT unnest($1::text[])::uuid)`, identifiers)
	if err != nil {
		return nil, fmt.Errorf("read observed decoy conditions: %w", err)
	}
	defer rows.Close()
	collected := make(map[string]map[deception.ConditionType]deception.Condition, len(observations))
	for rows.Next() {
		var (
			decoyID          string
			condition        deception.Condition
			conditionType    string
			status           string
			observedRevision *string
		)
		if err := rows.Scan(&decoyID, &conditionType, &status, &condition.Reason,
			&condition.Message, &observedRevision, &condition.LastTransitionTime); err != nil {
			return nil, fmt.Errorf("scan observed decoy condition: %w", err)
		}
		condition.Type = deception.ConditionType(conditionType)
		condition.Status = deception.Status(status)
		condition.LastTransitionTime = condition.LastTransitionTime.UTC()
		if observedRevision != nil {
			var value uint64
			if _, err := fmt.Sscanf(*observedRevision, "%d", &value); err == nil {
				condition.ObservedRevision = &value
			}
		}
		if collected[decoyID] == nil {
			collected[decoyID] = make(map[deception.ConditionType]deception.Condition, len(deception.ConditionTypes()))
		}
		collected[decoyID][condition.Type] = condition
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate observed decoy conditions: %w", err)
	}
	for decoyID, observation := range observations {
		// Any dimension the device did not report stays Unknown. A missing
		// condition is never completed with a favourable value.
		transitionAt := time.Now().UTC()
		if observation.ReportedAt != nil {
			transitionAt = *observation.ReportedAt
		}
		fallback := deception.UnreportedObservation(decoyID, transitionAt)
		conditions := make([]deception.Condition, 0, len(fallback.Conditions))
		for index, conditionType := range deception.ConditionTypes() {
			if condition, ok := collected[decoyID][conditionType]; ok {
				conditions = append(conditions, condition)
				continue
			}
			conditions = append(conditions, fallback.Conditions[index])
		}
		observation.Conditions = conditions
		observations[decoyID] = observation
	}
	return observations, nil
}

// observationFor returns the stored observation, or the honest default for a
// decoy nothing has reported on: unknown, with every dimension Unknown.
func observationFor(
	observations map[string]deception.Observation,
	decoyID string,
	now time.Time,
) deception.Observation {
	if observation, ok := observations[decoyID]; ok {
		return observation
	}
	return deception.UnreportedObservation(decoyID, now)
}

const returningDecoyColumns = `
    decoy_id::text, environment_id::text, zone_id::text, display_name,
    decoy_family, persona, interaction_level, host(address),
    pack, pack_version, COALESCE(pack_digest, ''), desired_state,
    revision, created_at, updated_at`

func scanDecoys(rows pgx.Rows) ([]deception.Decoy, error) {
	defer rows.Close()
	decoys := []deception.Decoy{}
	for rows.Next() {
		var (
			decoy            deception.Decoy
			family           string
			persona          string
			interactionLevel string
			desiredState     string
		)
		if err := rows.Scan(&decoy.DecoyID, &decoy.EnvironmentID, &decoy.ZoneID, &decoy.DisplayName,
			&family, &persona, &interactionLevel, &decoy.Address, &decoy.Pack, &decoy.PackVersion,
			&decoy.PackDigest, &desiredState, &decoy.Revision, &decoy.CreatedAt, &decoy.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan decoy: %w", err)
		}
		decoy.Family = deception.Family(family)
		decoy.Persona = deception.Persona(persona)
		decoy.InteractionLevel = deception.InteractionLevel(interactionLevel)
		decoy.DesiredState = deception.DesiredState(desiredState)
		decoy.CreatedAt = decoy.CreatedAt.UTC()
		decoy.UpdatedAt = decoy.UpdatedAt.UTC()
		decoys = append(decoys, decoy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate decoys: %w", err)
	}
	return decoys, nil
}

func lockDecoy(
	ctx context.Context,
	tx pgx.Tx,
	environmentID, decoyID string,
	expectedRevision int64,
) (deception.Decoy, error) {
	rows, err := tx.Query(ctx, `
SELECT`+decoyColumns+`
FROM guardian_deception.decoys d
JOIN guardian_environment.environments e ON e.environment_id = d.environment_id
WHERE d.environment_id = $1
  AND d.decoy_id = $2
  AND e.organization_id = (SELECT organization_id FROM guardian_environment.organizations WHERE singleton = true)
FOR UPDATE OF d`, environmentID, decoyID)
	if err != nil {
		return deception.Decoy{}, fmt.Errorf("lock decoy: %w", err)
	}
	locked, err := scanDecoys(rows)
	if err != nil {
		return deception.Decoy{}, err
	}
	if len(locked) != 1 {
		return deception.Decoy{}, deception.ErrNotFound
	}
	// A removed decoy is gone from the operator's view, so a write against it
	// is a 404 rather than a stale-revision conflict.
	if locked[0].DesiredState == deception.DesiredRemoved {
		return deception.Decoy{}, deception.ErrNotFound
	}
	if locked[0].Revision != expectedRevision {
		return deception.Decoy{}, deception.ErrPreconditionFailed
	}
	return locked[0], nil
}

// assertAddressInZone is the section 8.7 placement check. The zone's prefix
// lives in the database, so the containment test happens here, in the same
// transaction as the write it guards.
func assertAddressInZone(ctx context.Context, tx pgx.Tx, environmentID, zoneID, address string) error {
	var cidr string
	err := tx.QueryRow(ctx, `
SELECT network::text
FROM guardian_environment.zones
WHERE environment_id = $1 AND zone_id = $2`, environmentID, zoneID).Scan(&cidr)
	if errors.Is(err, pgx.ErrNoRows) {
		return deception.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("load decoy zone prefix: %w", err)
	}
	return deception.AddressWithinZone(address, cidr)
}

func mapDecoyWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23505":
		if pgErr.ConstraintName == "decoys_active_address_key" {
			return deception.ErrAddressConflict
		}
		return deception.ErrNameConflict
	case "23503":
		return deception.ErrNotFound
	case "23514":
		return deception.ErrInvalidInput
	default:
		return err
	}
}

func decoySnapshot(value deception.Decoy) (audit.Snapshot, error) {
	return audit.NewSnapshot(map[string]any{
		"decoy_id":          value.DecoyID,
		"environment_id":    value.EnvironmentID,
		"zone_id":           value.ZoneID,
		"display_name":      value.DisplayName,
		"family":            value.Family,
		"persona":           value.Persona,
		"interaction_level": value.InteractionLevel,
		"address":           value.Address,
		"pack":              value.Pack,
		"pack_version":      value.PackVersion,
		"desired_state":     value.DesiredState,
		"revision":          value.Revision,
	})
}

func decoyAuditEvent(
	action audit.Action,
	mutation deception.Mutation,
	decoyID string,
	before, after *audit.Snapshot,
) audit.Event {
	return audit.Event{
		OccurredAt:    mutation.OccurredAt,
		Actor:         audit.Actor{Type: audit.ActorTypeUser, ID: mutation.ActorID},
		Action:        action,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeDecoy, ID: decoyID},
		CorrelationID: decoyID,
		RequestID:     mutation.RequestID,
		Before:        before,
		After:         after,
	}
}
