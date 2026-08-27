//go:build integration

package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage/dbgen"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestAuditedMutationCommitsOrRollsBackAtomically(t *testing.T) {
	ctx := context.Background()
	store := openAuditTestStore(t, 4)

	before := mustAuditSnapshot(t, map[string]any{
		"enabled":  false,
		"password": "must-not-survive",
	})
	after := mustAuditSnapshot(t, map[string]any{
		"enabled": true,
		"token":   "must-not-survive",
	})
	committedState := dbgen.PutServiceStateParams{
		StateKey:   "audited-commit",
		StateValue: []byte(`{"enabled":true}`),
	}
	record, err := store.withAuditedMutation(ctx, pgx.TxOptions{}, func(ctx context.Context, queries *dbgen.Queries) (audit.Event, error) {
		if err := queries.PutServiceState(ctx, committedState); err != nil {
			return audit.Event{}, err
		}
		return audit.Event{
			OccurredAt:    time.Now().UTC(),
			Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "control-plane"},
			Action:        audit.ActionSecuritySettingChanged,
			Object:        audit.ObjectRef{Type: audit.ObjectTypeSecuritySetting, ID: committedState.StateKey},
			CorrelationID: "atomic-commit",
			RequestID:     "request-atomic-commit",
			Before:        &before,
			After:         &after,
		}, nil
	})
	if err != nil {
		t.Fatalf("withAuditedMutation(commit) error = %v", err)
	}
	if _, err := store.queries.GetServiceState(ctx, committedState.StateKey); err != nil {
		t.Fatalf("committed mutation is missing: %v", err)
	}
	if record.Sequence != 1 || record.Event.CorrelationID != "atomic-commit" {
		t.Fatalf("committed audit record = %+v", record)
	}
	for name, snapshot := range map[string]*audit.Snapshot{"before": record.Event.Before, "after": record.Event.After} {
		if snapshot == nil {
			t.Fatalf("%s snapshot is nil", name)
		}
		encoded := string(snapshot.Bytes())
		if strings.Contains(encoded, "must-not-survive") || !strings.Contains(encoded, audit.RedactedSnapshotValue) {
			t.Fatalf("%s snapshot was not safely redacted: %s", name, encoded)
		}
	}

	rolledBackState := dbgen.PutServiceStateParams{
		StateKey:   "audited-rollback",
		StateValue: []byte(`{"must":"rollback"}`),
	}
	_, err = store.withAuditedMutation(ctx, pgx.TxOptions{}, func(ctx context.Context, queries *dbgen.Queries) (audit.Event, error) {
		if err := queries.PutServiceState(ctx, rolledBackState); err != nil {
			return audit.Event{}, err
		}
		return audit.Event{
			OccurredAt:    time.Now().UTC(),
			Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "control-plane"},
			Action:        audit.Action("security.action.deleted"),
			Object:        audit.ObjectRef{Type: audit.ObjectTypeSecurityAction, ID: rolledBackState.StateKey},
			CorrelationID: "atomic-rollback",
		}, nil
	})
	if !errors.Is(err, audit.ErrInvalidEvent) {
		t.Fatalf("withAuditedMutation(rollback) error = %v, want ErrInvalidEvent", err)
	}
	if _, err := store.queries.GetServiceState(ctx, rolledBackState.StateKey); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("rolled-back mutation lookup error = %v, want pgx.ErrNoRows", err)
	}

	oversizedState := dbgen.PutServiceStateParams{
		StateKey:   "snapshot-rollback",
		StateValue: []byte(`{"must":"rollback"}`),
	}
	_, err = store.withAuditedMutation(ctx, pgx.TxOptions{}, func(ctx context.Context, queries *dbgen.Queries) (audit.Event, error) {
		if err := queries.PutServiceState(ctx, oversizedState); err != nil {
			return audit.Event{}, err
		}
		_, err := audit.NewSnapshot(map[string]any{"message": strings.Repeat("x", audit.MaxSnapshotStringBytes+1)})
		if err != nil {
			return audit.Event{}, err
		}
		return audit.Event{}, errors.New("oversized snapshot unexpectedly passed validation")
	})
	if !errors.Is(err, audit.ErrSnapshotStringTooLong) {
		t.Fatalf("withAuditedMutation(oversized snapshot) error = %v, want ErrSnapshotStringTooLong", err)
	}
	if _, err := store.queries.GetServiceState(ctx, oversizedState.StateKey); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("oversized-snapshot mutation lookup error = %v, want pgx.ErrNoRows", err)
	}
	anchor, err := store.queries.GetAuditAnchor(ctx)
	if err != nil || anchor != 1 {
		t.Fatalf("audit anchor after rollback = (%d, %v), want 1", anchor, err)
	}
}

func TestAuditTableRejectsMutationAndInvalidStoredContracts(t *testing.T) {
	ctx := context.Background()
	store := openAuditTestStore(t, 4)
	record := appendAuditEvent(t, store, audit.ActionLoginFailed, audit.ObjectTypeUser, "unknown-user", "mutation-guard")

	var generatedUUIDVersion int16
	if err := store.pool.QueryRow(ctx, `
SELECT uuid_extract_version(event_id)
FROM guardian_audit.records
WHERE sequence = $1`, record.Sequence).Scan(&generatedUUIDVersion); err != nil {
		t.Fatalf("inspect generated event ID: %v", err)
	}
	if record.Sequence <= 0 || generatedUUIDVersion != 7 {
		t.Fatalf("generated identity = (sequence %d, UUID version %d), want positive UUIDv7", record.Sequence, generatedUUIDVersion)
	}

	for name, statement := range map[string]string{
		"update":   "UPDATE guardian_audit.records SET request_id = 'changed'",
		"delete":   "DELETE FROM guardian_audit.records",
		"truncate": "TRUNCATE guardian_audit.records",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := store.pool.Exec(ctx, statement)
			assertPostgreSQLErrorCode(t, err, "55000")
		})
	}

	_, err := store.pool.Exec(ctx, `
INSERT INTO guardian_audit.records (
    occurred_at, actor_type, actor_id, action, object_type, object_id, correlation_id
) VALUES (now(), 'system', 'control-plane', 'auth.logout', 'user', 'user-1', 'invalid-pair')`)
	assertPostgreSQLErrorCode(t, err, "23514")

	invalidSnapshots := []struct {
		name string
		raw  string
	}{
		{name: "missing-schema", raw: `{"data":{}}`},
		{name: "null-schema", raw: `{"schema":null,"data":{}}`},
		{name: "missing-data", raw: `{"schema":"guardian.audit.snapshot.v1"}`},
		{name: "null-data", raw: `{"schema":"guardian.audit.snapshot.v1","data":null}`},
		{name: "array-data", raw: `{"schema":"guardian.audit.snapshot.v1","data":[]}`},
	}
	for _, column := range []string{"before_snapshot", "after_snapshot"} {
		for _, invalid := range invalidSnapshots {
			t.Run(column+"/"+invalid.name, func(t *testing.T) {
				statement := fmt.Sprintf(`
INSERT INTO guardian_audit.records (
    occurred_at, actor_type, actor_id, action, object_type, object_id,
    correlation_id, %s
) VALUES (
    now(), 'system', 'control-plane', 'security.action.denied',
    'security_action', 'unsafe-snapshot', 'invalid-snapshot', $1::json
)`, column)
				_, err := store.pool.Exec(ctx, statement, invalid.raw)
				assertPostgreSQLErrorCode(t, err, "23514")
			})
		}
	}

	for name, statement := range map[string]string{
		"non-positive-sequence": `
INSERT INTO guardian_audit.records (
    sequence, occurred_at, actor_type, actor_id, action, object_type, object_id, correlation_id
) OVERRIDING SYSTEM VALUE VALUES (
    0, now(), 'system', 'control-plane', 'security.action.denied',
    'security_action', 'forged-sequence', 'forged-sequence'
)`,
		"uuid-v4": `
INSERT INTO guardian_audit.records (
    event_id, occurred_at, actor_type, actor_id, action, object_type, object_id, correlation_id
) VALUES (
    '00000000-0000-4000-8000-000000000000', now(), 'system', 'control-plane',
    'security.action.denied', 'security_action', 'forged-event-id', 'forged-event-id'
)`,
		"uuid-v7-non-rfc-variant": `
INSERT INTO guardian_audit.records (
    event_id, occurred_at, actor_type, actor_id, action, object_type, object_id, correlation_id
) VALUES (
    '00000000-0000-7000-0000-000000000000', now(), 'system', 'control-plane',
    'security.action.denied', 'security_action', 'forged-event-id', 'forged-event-id'
)`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := store.pool.Exec(ctx, statement)
			assertPostgreSQLErrorCode(t, err, "23514")
		})
	}

	_, err = store.pool.Exec(ctx, `
INSERT INTO guardian_audit.records (
    occurred_at, actor_type, actor_id, action, object_type, object_id, correlation_id
) VALUES (
    now(), 'system', repeat('a', 257), 'security.action.denied',
    'security_action', 'bounded-identity', 'identity-bound'
)`)
	assertPostgreSQLErrorCode(t, err, "23514")

	anchor, err := store.queries.GetAuditAnchor(ctx)
	if err != nil || anchor != 1 {
		t.Fatalf("audit anchor after rejected writes = (%d, %v), want 1", anchor, err)
	}
}

func TestAuditRuntimeRoleUsesOnlyNarrowInsertAndReadPrivileges(t *testing.T) {
	ctx := context.Background()
	store := openAuditTestStore(t, 4)
	roleName := fmt.Sprintf("guardian_audit_runtime_%d", time.Now().UnixNano())
	roleIdentifier := pgx.Identifier{roleName}.Sanitize()
	if _, err := store.pool.Exec(ctx, "CREATE ROLE "+roleIdentifier+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"); err != nil {
		t.Fatalf("create restricted runtime role: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := store.pool.Exec(cleanupContext, "DROP OWNED BY "+roleIdentifier); err != nil {
			t.Errorf("drop runtime role privileges: %v", err)
			return
		}
		if _, err := store.pool.Exec(cleanupContext, "DROP ROLE "+roleIdentifier); err != nil {
			t.Errorf("drop runtime role: %v", err)
		}
	})

	if _, err := store.pool.Exec(ctx, `
REVOKE ALL ON SCHEMA guardian_audit FROM PUBLIC;
REVOKE ALL ON TABLE guardian_audit.records FROM PUBLIC;
REVOKE ALL ON SEQUENCE guardian_audit.records_sequence_seq FROM PUBLIC;
REVOKE ALL ON SCHEMA guardian_audit FROM `+roleIdentifier+`;
REVOKE ALL ON TABLE guardian_audit.records FROM `+roleIdentifier+`;
REVOKE ALL ON SEQUENCE guardian_audit.records_sequence_seq FROM `+roleIdentifier+`;
GRANT USAGE ON SCHEMA guardian_audit TO `+roleIdentifier+`;
GRANT SELECT ON TABLE guardian_audit.records TO `+roleIdentifier+`;
GRANT INSERT (
    occurred_at, actor_type, actor_id, action, object_type, object_id,
    correlation_id, request_id, before_snapshot, after_snapshot
) ON TABLE guardian_audit.records TO `+roleIdentifier+`;
GRANT USAGE ON SEQUENCE guardian_audit.records_sequence_seq TO `+roleIdentifier+`;`); err != nil {
		t.Fatalf("grant restricted runtime privileges: %v", err)
	}

	var schemaUsage, tableSelect, sequenceUsage bool
	var schemaCreate, tableWideInsert, tableUpdate, tableDelete, tableTruncate bool
	var sequenceSelect, sequenceUpdate bool
	if err := store.pool.QueryRow(ctx, `
SELECT
    has_schema_privilege($1::name, 'guardian_audit', 'USAGE'),
    has_table_privilege($1::name, 'guardian_audit.records', 'SELECT'),
    has_sequence_privilege($1::name, 'guardian_audit.records_sequence_seq', 'USAGE'),
    has_schema_privilege($1::name, 'guardian_audit', 'CREATE'),
    has_table_privilege($1::name, 'guardian_audit.records', 'INSERT'),
    has_table_privilege($1::name, 'guardian_audit.records', 'UPDATE'),
    has_table_privilege($1::name, 'guardian_audit.records', 'DELETE'),
    has_table_privilege($1::name, 'guardian_audit.records', 'TRUNCATE'),
    has_sequence_privilege($1::name, 'guardian_audit.records_sequence_seq', 'SELECT'),
    has_sequence_privilege($1::name, 'guardian_audit.records_sequence_seq', 'UPDATE')`, roleName).Scan(
		&schemaUsage,
		&tableSelect,
		&sequenceUsage,
		&schemaCreate,
		&tableWideInsert,
		&tableUpdate,
		&tableDelete,
		&tableTruncate,
		&sequenceSelect,
		&sequenceUpdate,
	); err != nil {
		t.Fatalf("inspect restricted runtime privileges: %v", err)
	}
	if !schemaUsage || !tableSelect || !sequenceUsage {
		t.Fatalf(
			"required runtime privileges = schema usage %t, table select %t, sequence usage %t; want all true",
			schemaUsage,
			tableSelect,
			sequenceUsage,
		)
	}
	if schemaCreate || tableWideInsert || tableUpdate || tableDelete || tableTruncate || sequenceSelect || sequenceUpdate {
		t.Fatalf(
			"forbidden runtime privileges = schema create %t, table insert/update/delete/truncate %t/%t/%t/%t, sequence select/update %t/%t; want all false",
			schemaCreate,
			tableWideInsert,
			tableUpdate,
			tableDelete,
			tableTruncate,
			sequenceSelect,
			sequenceUpdate,
		)
	}
	var membershipCount int
	if err := store.pool.QueryRow(ctx, `
SELECT count(*)
FROM pg_auth_members
WHERE member = (SELECT oid FROM pg_roles WHERE rolname = $1)`, roleName).Scan(&membershipCount); err != nil {
		t.Fatalf("inspect restricted runtime memberships: %v", err)
	}
	if membershipCount != 0 {
		t.Fatalf("restricted runtime parent membership count = %d, want 0", membershipCount)
	}

	beginAsRuntime := func(t *testing.T) pgx.Tx {
		t.Helper()
		tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if err != nil {
			t.Fatalf("begin runtime transaction: %v", err)
		}
		if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+roleIdentifier); err != nil {
			_ = tx.Rollback(context.Background())
			t.Fatalf("set restricted runtime role: %v", err)
		}
		return tx
	}

	appendTransaction := beginAsRuntime(t)
	record, err := (auditQueryAppender{queries: dbgen.New(appendTransaction)}).Append(ctx, audit.Event{
		OccurredAt:    time.Now().UTC(),
		Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "control-plane"},
		Action:        audit.ActionSecurityActionDenied,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeSecurityAction, ID: "runtime-role"},
		CorrelationID: "runtime-role",
	})
	if err != nil {
		_ = appendTransaction.Rollback(context.Background())
		t.Fatalf("restricted runtime append: %v", err)
	}
	if err := appendTransaction.Commit(ctx); err != nil {
		t.Fatalf("commit restricted runtime append: %v", err)
	}
	if record.Sequence != 1 {
		t.Fatalf("restricted runtime sequence = %d, want 1", record.Sequence)
	}

	readTransaction := beginAsRuntime(t)
	readQueries := dbgen.New(readTransaction)
	if err := readQueries.AcquireAuditAnchorLock(ctx); err != nil {
		_ = readTransaction.Rollback(context.Background())
		t.Fatalf("restricted runtime anchor lock: %v", err)
	}
	anchor, err := readQueries.GetAuditAnchor(ctx)
	if err != nil {
		_ = readTransaction.Rollback(context.Background())
		t.Fatalf("restricted runtime anchor: %v", err)
	}
	rows, err := readQueries.ListAuditRecords(ctx, dbgen.ListAuditRecordsParams{AnchorSequence: anchor, PageSize: 2})
	if err != nil {
		_ = readTransaction.Rollback(context.Background())
		t.Fatalf("restricted runtime list: %v", err)
	}
	if err := readTransaction.Commit(ctx); err != nil {
		t.Fatalf("commit restricted runtime read: %v", err)
	}
	if len(rows) != 1 || rows[0].EventID != record.EventID {
		t.Fatalf("restricted runtime rows = %+v, want event %s", rows, record.EventID)
	}

	protectedColumns := []struct {
		name  string
		value string
	}{
		{name: "sequence", value: "42"},
		{name: "event_id", value: "uuidv7()"},
		{name: "recorded_at", value: "clock_timestamp()"},
	}
	for _, protected := range protectedColumns {
		t.Run("forged-"+protected.name, func(t *testing.T) {
			tx := beginAsRuntime(t)
			statement := fmt.Sprintf(`
INSERT INTO guardian_audit.records (
    %s, occurred_at, actor_type, actor_id, action, object_type, object_id, correlation_id
) OVERRIDING SYSTEM VALUE VALUES (
    %s, now(), 'system', 'control-plane', 'security.action.denied',
    'security_action', 'runtime-forgery', 'runtime-forgery'
)`, protected.name, protected.value)
			_, err := tx.Exec(ctx, statement)
			assertPostgreSQLErrorCode(t, err, "42501")
			_ = tx.Rollback(context.Background())
		})
	}

	updateTransaction := beginAsRuntime(t)
	_, err = updateTransaction.Exec(ctx, "UPDATE guardian_audit.records SET request_id = 'forged'")
	assertPostgreSQLErrorCode(t, err, "42501")
	_ = updateTransaction.Rollback(context.Background())

	anchor, err = store.queries.GetAuditAnchor(ctx)
	if err != nil || anchor != 1 {
		t.Fatalf("anchor after restricted runtime forgery attempts = (%d, %v), want 1", anchor, err)
	}
}

func TestAuditTimestampBoundsRejectUnreadableRows(t *testing.T) {
	ctx := context.Background()
	store := openAuditTestStore(t, 4)
	validTimes := []time.Time{
		time.Date(1, time.January, 1, 0, 0, 0, 1000, time.UTC),
		time.Date(9999, time.December, 31, 23, 59, 59, 999999000, time.UTC),
	}
	for index, occurredAt := range validTimes {
		record, err := store.Append(ctx, audit.Event{
			OccurredAt:    occurredAt,
			Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "control-plane"},
			Action:        audit.ActionSecurityActionDenied,
			Object:        audit.ObjectRef{Type: audit.ObjectTypeSecurityAction, ID: fmt.Sprintf("timestamp-boundary-%d", index)},
			CorrelationID: fmt.Sprintf("timestamp-boundary-%d", index),
		})
		if err != nil {
			t.Fatalf("Append(valid timestamp %s) error = %v", occurredAt.Format(time.RFC3339Nano), err)
		}
		if !record.Event.OccurredAt.Equal(occurredAt) {
			t.Fatalf("stored timestamp = %s, want %s", record.Event.OccurredAt, occurredAt)
		}
	}

	invalidTimes := []struct {
		name  string
		value string
	}{
		{name: "positive-infinity", value: "infinity"},
		{name: "negative-infinity", value: "-infinity"},
		{name: "go-zero", value: "0001-01-01 00:00:00+00"},
		{name: "bc", value: "0001-01-01 00:00:00 BC"},
		{name: "year-10000", value: "10000-01-01 00:00:00+00"},
	}
	for _, column := range []string{"occurred_at", "recorded_at"} {
		for _, invalid := range invalidTimes {
			t.Run(column+"/"+invalid.name, func(t *testing.T) {
				columns := "occurred_at, actor_type, actor_id, action, object_type, object_id, correlation_id"
				values := "$1::timestamptz, 'system', 'control-plane', 'security.action.denied', 'security_action', 'timestamp-poison', 'timestamp-poison'"
				if column == "recorded_at" {
					columns = "recorded_at, " + columns
					values = "$1::timestamptz, now(), 'system', 'control-plane', 'security.action.denied', 'security_action', 'timestamp-poison', 'timestamp-poison'"
				}
				statement := fmt.Sprintf("INSERT INTO guardian_audit.records (%s) VALUES (%s)", columns, values)
				_, err := store.pool.Exec(ctx, statement, invalid.value)
				assertPostgreSQLErrorCode(t, err, "23514")
			})
		}
	}

	anchor, err := store.queries.GetAuditAnchor(ctx)
	if err != nil || anchor != int64(len(validTimes)) {
		t.Fatalf("anchor after timestamp poison attempts = (%d, %v), want %d", anchor, err, len(validTimes))
	}
}

func TestAuditSnapshotNearEncodedLimitRoundTripsExactly(t *testing.T) {
	store := openAuditTestStore(t, 4)
	projection := make(map[string]any)
	var snapshot audit.Snapshot
	for index := 0; index < audit.MaxSnapshotMembers; index++ {
		projection[fmt.Sprintf("field_%02d", index)] = strings.Repeat("x", audit.MaxSnapshotStringBytes)
		candidate, err := audit.NewSnapshot(projection)
		if errors.Is(err, audit.ErrSnapshotTooLarge) {
			delete(projection, fmt.Sprintf("field_%02d", index))
			break
		}
		if err != nil {
			t.Fatalf("NewSnapshot(boundary candidate %d) error = %v", index, err)
		}
		snapshot = candidate
	}
	if size := len(snapshot.Bytes()); size < 15*1024 || size > audit.MaxSnapshotBytes {
		t.Fatalf("boundary snapshot size = %d, want 15 KiB..%d", size, audit.MaxSnapshotBytes)
	}

	record, err := store.Append(context.Background(), audit.Event{
		OccurredAt:    time.Now().UTC(),
		Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "control-plane"},
		Action:        audit.ActionSecurityActionDenied,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeSecurityAction, ID: "snapshot-boundary"},
		CorrelationID: "snapshot-boundary",
		Before:        &snapshot,
	})
	if err != nil {
		t.Fatalf("Append(boundary snapshot) error = %v", err)
	}
	if record.Event.Before == nil || string(record.Event.Before.Bytes()) != string(snapshot.Bytes()) {
		t.Fatalf("stored boundary snapshot did not preserve canonical bytes")
	}
}

func TestAppendProtocolLockPrecedesIdentityAllocation(t *testing.T) {
	store := openAuditTestStore(t, 6)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	anchorTransaction, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatalf("BeginTx(anchor blocker) error = %v", err)
	}
	defer func() { _ = anchorTransaction.Rollback(context.Background()) }()
	if err := dbgen.New(anchorTransaction).AcquireAuditAnchorLock(ctx); err != nil {
		t.Fatalf("AcquireAuditAnchorLock(blocker) error = %v", err)
	}

	var initialLastValue int64
	var initialIsCalled bool
	if err := store.pool.QueryRow(ctx, `
SELECT last_value, is_called
FROM guardian_audit.records_sequence_seq`).Scan(&initialLastValue, &initialIsCalled); err != nil {
		t.Fatalf("inspect initial identity sequence: %v", err)
	}
	if initialIsCalled {
		t.Fatal("fresh audit identity sequence is unexpectedly already called")
	}

	type appendResult struct {
		record audit.Record
		err    error
	}
	const appendCount = 2
	results := make(chan appendResult, appendCount)
	for index := 0; index < appendCount; index++ {
		go func(index int) {
			record, err := store.Append(ctx, audit.Event{
				OccurredAt:    time.Now().UTC(),
				Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "control-plane"},
				Action:        audit.ActionSecurityActionDenied,
				Object:        audit.ObjectRef{Type: audit.ObjectTypeSecurityAction, ID: fmt.Sprintf("blocked-append-%d", index)},
				CorrelationID: fmt.Sprintf("blocked-append-%d", index),
			})
			results <- appendResult{record: record, err: err}
		}(index)
	}

	for {
		waiterCount, err := auditAdvisoryWaiterCount(ctx, store, "ShareLock")
		if err != nil {
			t.Fatalf("inspect append advisory-lock waiters: %v", err)
		}
		if waiterCount == appendCount {
			break
		}
		select {
		case completed := <-results:
			t.Fatalf("Append escaped exclusive protocol lock: record=%+v err=%v", completed.record, completed.err)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for append advisory locks: %v", ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}

	var blockedLastValue int64
	var blockedIsCalled bool
	if err := store.pool.QueryRow(ctx, `
SELECT last_value, is_called
FROM guardian_audit.records_sequence_seq`).Scan(&blockedLastValue, &blockedIsCalled); err != nil {
		t.Fatalf("inspect blocked identity sequence: %v", err)
	}
	if blockedLastValue != initialLastValue || blockedIsCalled != initialIsCalled {
		t.Fatalf(
			"identity advanced before protocol lock: before=(%d,%t) blocked=(%d,%t)",
			initialLastValue,
			initialIsCalled,
			blockedLastValue,
			blockedIsCalled,
		)
	}

	if err := anchorTransaction.Commit(ctx); err != nil {
		t.Fatalf("Commit(anchor blocker) error = %v", err)
	}
	sequences := make(map[int64]struct{}, appendCount)
	for index := 0; index < appendCount; index++ {
		select {
		case completed := <-results:
			if completed.err != nil {
				t.Fatalf("Append(after anchor release) error = %v", completed.err)
			}
			sequences[completed.record.Sequence] = struct{}{}
		case <-ctx.Done():
			t.Fatalf("Append did not resume after anchor release: %v", ctx.Err())
		}
	}
	for _, expected := range []int64{1, 2} {
		if _, found := sequences[expected]; !found {
			t.Fatalf("committed sequences = %v, missing %d", sequences, expected)
		}
	}

	var finalLastValue int64
	var finalIsCalled bool
	if err := store.pool.QueryRow(ctx, `
SELECT last_value, is_called
FROM guardian_audit.records_sequence_seq`).Scan(&finalLastValue, &finalIsCalled); err != nil {
		t.Fatalf("inspect final identity sequence: %v", err)
	}
	if finalLastValue != appendCount || !finalIsCalled {
		t.Fatalf("final identity sequence = (%d,%t), want (%d,true)", finalLastValue, finalIsCalled, appendCount)
	}
}

func TestFirstPageAnchorWaitsForPrecommitAppend(t *testing.T) {
	store := openAuditTestStore(t, 6)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	appendTransaction, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatalf("BeginTx(append) error = %v", err)
	}
	defer func() { _ = appendTransaction.Rollback(context.Background()) }()

	lateRecord, err := (auditQueryAppender{queries: dbgen.New(appendTransaction)}).Append(ctx, audit.Event{
		OccurredAt:    time.Now().UTC(),
		Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "control-plane"},
		Action:        audit.ActionSecurityActionDenied,
		Object:        audit.ObjectRef{Type: audit.ObjectTypeSecurityAction, ID: "late-precommit"},
		CorrelationID: "late-precommit",
	})
	if err != nil {
		t.Fatalf("Append(precommit) error = %v", err)
	}
	if lateRecord.Sequence != 1 {
		t.Fatalf("precommit sequence = %d, want 1", lateRecord.Sequence)
	}

	type listResult struct {
		page audit.Page
		err  error
	}
	result := make(chan listResult, 1)
	go func() {
		page, err := store.List(ctx, audit.ListQuery{Limit: 10})
		result <- listResult{page: page, err: err}
	}()

	waiting := false
	for !waiting {
		select {
		case completed := <-result:
			t.Fatalf("first-page List returned before precommit append: page=%+v err=%v", completed.page, completed.err)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for anchor advisory lock: %v", ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}

		waiterCount, err := auditAdvisoryWaiterCount(ctx, store, "ExclusiveLock")
		if err != nil {
			t.Fatalf("inspect advisory-lock waiters: %v", err)
		}
		waiting = waiterCount > 0
	}

	select {
	case completed := <-result:
		t.Fatalf("first-page List escaped observed advisory wait: page=%+v err=%v", completed.page, completed.err)
	default:
	}
	if err := appendTransaction.Commit(ctx); err != nil {
		t.Fatalf("Commit(precommit append) error = %v", err)
	}

	select {
	case completed := <-result:
		if completed.err != nil {
			t.Fatalf("List(after commit) error = %v", completed.err)
		}
		assertAuditSequences(t, completed.page.Records, lateRecord.Sequence)
		if completed.page.Records[0].EventID != lateRecord.EventID {
			t.Fatalf("anchored event ID = %q, want %q", completed.page.Records[0].EventID, lateRecord.EventID)
		}
	case <-ctx.Done():
		t.Fatalf("List did not resume after append commit: %v", ctx.Err())
	}
}

func TestAuditKeysetPaginationUsesStableAnchorAndExactFilters(t *testing.T) {
	store := openAuditTestStore(t, 4)

	appendAuditEvent(t, store, audit.ActionEnvironmentUpdated, audit.ObjectTypeEnvironment, "environment-a", "correlation-a")
	appendAuditEvent(t, store, audit.ActionZoneUpdated, audit.ObjectTypeZone, "zone-a", "correlation-a")
	appendAuditEvent(t, store, audit.ActionEnvironmentUpdated, audit.ObjectTypeEnvironment, "environment-a", "correlation-target")
	appendAuditEvent(t, store, audit.ActionEnvironmentUpdated, audit.ObjectTypeEnvironment, "environment-b", "correlation-target")
	appendAuditEvent(t, store, audit.ActionSecurityActionDenied, audit.ObjectTypeSecurityAction, "policy-a", "correlation-other")

	ctx := context.Background()
	first, err := store.List(ctx, audit.ListQuery{Limit: 2})
	if err != nil {
		t.Fatalf("List(first) error = %v", err)
	}
	assertAuditSequences(t, first.Records, 5, 4)
	firstCursor, err := audit.ParseCursor(first.NextCursor)
	if err != nil || firstCursor.AnchorSequence != 5 || firstCursor.PositionSequence != 4 {
		t.Fatalf("first cursor = (%+v, %v)", firstCursor, err)
	}

	appendAuditEvent(t, store, audit.ActionEnvironmentUpdated, audit.ObjectTypeEnvironment, "environment-new", "correlation-new")

	second, err := store.List(ctx, audit.ListQuery{Limit: 2, Cursor: firstCursor})
	if err != nil {
		t.Fatalf("List(second) error = %v", err)
	}
	assertAuditSequences(t, second.Records, 3, 2)
	secondCursor, err := audit.ParseCursor(second.NextCursor)
	if err != nil || secondCursor.AnchorSequence != 5 || secondCursor.PositionSequence != 2 {
		t.Fatalf("second cursor = (%+v, %v)", secondCursor, err)
	}

	third, err := store.List(ctx, audit.ListQuery{Limit: 2, Cursor: secondCursor})
	if err != nil {
		t.Fatalf("List(third) error = %v", err)
	}
	assertAuditSequences(t, third.Records, 1)
	if third.NextCursor != "" {
		t.Fatalf("third next cursor = %q, want empty", third.NextCursor)
	}

	current, err := store.List(ctx, audit.ListQuery{Limit: 2})
	if err != nil {
		t.Fatalf("List(current) error = %v", err)
	}
	assertAuditSequences(t, current.Records, 6, 5)

	actionPage, err := store.List(ctx, audit.ListQuery{Limit: 20, Action: audit.ActionEnvironmentUpdated})
	if err != nil {
		t.Fatalf("List(action filter) error = %v", err)
	}
	assertAuditSequences(t, actionPage.Records, 6, 4, 3, 1)

	correlationPage, err := store.List(ctx, audit.ListQuery{Limit: 20, CorrelationID: "correlation-target"})
	if err != nil {
		t.Fatalf("List(correlation filter) error = %v", err)
	}
	assertAuditSequences(t, correlationPage.Records, 4, 3)

	objectPage, err := store.List(ctx, audit.ListQuery{
		Limit:      20,
		ObjectType: audit.ObjectTypeEnvironment,
		ObjectID:   "environment-a",
	})
	if err != nil {
		t.Fatalf("List(object filter) error = %v", err)
	}
	assertAuditSequences(t, objectPage.Records, 3, 1)
}

func TestAuditConcurrentAppendIsUniqueAndDeterministicallyOrdered(t *testing.T) {
	store := openAuditTestStore(t, 16)
	const appendCount = 32

	var wait sync.WaitGroup
	errorsFound := make(chan error, appendCount)
	for i := 0; i < appendCount; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, err := store.Append(context.Background(), audit.Event{
				OccurredAt: time.Now().UTC(),
				Actor:      audit.Actor{Type: audit.ActorTypeSystem, ID: "control-plane"},
				Action:     audit.ActionSecurityActionDenied,
				Object: audit.ObjectRef{
					Type: audit.ObjectTypeSecurityAction,
					ID:   fmt.Sprintf("concurrent-policy-%02d", index),
				},
				CorrelationID: fmt.Sprintf("concurrent-%02d", index),
			})
			if err != nil {
				errorsFound <- err
			}
		}(i)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("concurrent Append() error = %v", err)
	}
	if t.Failed() {
		return
	}

	page, err := store.List(context.Background(), audit.ListQuery{Limit: appendCount})
	if err != nil {
		t.Fatalf("List(concurrent records) error = %v", err)
	}
	if len(page.Records) != appendCount {
		t.Fatalf("concurrent record count = %d, want %d", len(page.Records), appendCount)
	}
	seenEventIDs := make(map[string]struct{}, appendCount)
	seenSequences := make(map[int64]struct{}, appendCount)
	for index, record := range page.Records {
		if err := record.Validate(); err != nil {
			t.Fatalf("record %d validation error = %v", index, err)
		}
		if _, duplicate := seenEventIDs[record.EventID]; duplicate {
			t.Fatalf("duplicate event ID %q", record.EventID)
		}
		seenEventIDs[record.EventID] = struct{}{}
		if _, duplicate := seenSequences[record.Sequence]; duplicate {
			t.Fatalf("duplicate sequence %d", record.Sequence)
		}
		seenSequences[record.Sequence] = struct{}{}
		if index > 0 && page.Records[index-1].Sequence <= record.Sequence {
			t.Fatalf("records are not newest-first: %d then %d", page.Records[index-1].Sequence, record.Sequence)
		}
	}
}

func openAuditTestStore(t *testing.T, maximumConnections int32) *Store {
	t.Helper()
	ctx := context.Background()
	databaseURL := createTestDatabase(t)
	report, err := Migrate(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if report.Applied != int(migrations.LatestVersion) || report.Version != migrations.LatestVersion {
		t.Fatalf("Migrate() = %+v, want %d migrations at version %d", report, migrations.LatestVersion, migrations.LatestVersion)
	}
	store, err := Open(ctx, databaseURL, maximumConnections)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(store.Close)
	return store
}

func appendAuditEvent(
	t *testing.T,
	store *Store,
	action audit.Action,
	objectType audit.ObjectType,
	objectID string,
	correlationID string,
) audit.Record {
	t.Helper()
	record, err := store.Append(context.Background(), audit.Event{
		OccurredAt:    time.Now().UTC(),
		Actor:         audit.Actor{Type: audit.ActorTypeSystem, ID: "control-plane"},
		Action:        action,
		Object:        audit.ObjectRef{Type: objectType, ID: objectID},
		CorrelationID: correlationID,
	})
	if err != nil {
		t.Fatalf("Append(%s) error = %v", action, err)
	}
	return record
}

func mustAuditSnapshot(t *testing.T, projection map[string]any) audit.Snapshot {
	t.Helper()
	snapshot, err := audit.NewSnapshot(projection)
	if err != nil {
		t.Fatalf("NewSnapshot() error = %v", err)
	}
	return snapshot
}

func assertPostgreSQLErrorCode(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil {
		t.Fatalf("database operation unexpectedly succeeded; want PostgreSQL code %s", expected)
	}
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != expected {
		t.Fatalf("database error = %v, want PostgreSQL code %s", err, expected)
	}
}

func assertAuditSequences(t *testing.T, records []audit.Record, expected ...int64) {
	t.Helper()
	if len(records) != len(expected) {
		t.Fatalf("record count = %d, want %d: %+v", len(records), len(expected), records)
	}
	for index, sequence := range expected {
		if records[index].Sequence != sequence {
			t.Fatalf("record %d sequence = %d, want %d", index, records[index].Sequence, sequence)
		}
	}
}

func auditAdvisoryWaiterCount(ctx context.Context, store *Store, mode string) (int, error) {
	var waiterCount int
	err := store.pool.QueryRow(ctx, `
SELECT count(*)
FROM pg_locks
WHERE locktype = 'advisory'
  AND database = (SELECT oid FROM pg_database WHERE datname = current_database())
  AND classid = 1196769618
  AND objid = 1
  AND objsubid = 2
  AND mode = $1
  AND granted = false`, mode).Scan(&waiterCount)
	return waiterCount, err
}
