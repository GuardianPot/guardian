//go:build integration

package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/database/dbgen"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/database/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestFreshMigrationIsIdempotentAndRestartPreservesState(t *testing.T) {
	ctx := context.Background()
	databaseURL := createTestDatabase(t)

	first, err := Migrate(ctx, databaseURL)
	if err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if first.Applied != 1 || first.Version != migrations.LatestVersion {
		t.Fatalf("first Migrate() = %+v", first)
	}
	second, err := Migrate(ctx, databaseURL)
	if err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
	if second.Applied != 0 || second.Version != migrations.LatestVersion {
		t.Fatalf("second Migrate() = %+v, want idempotent no-op", second)
	}

	store, err := Open(ctx, databaseURL, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatalf("Ready() error = %v", err)
	}

	commitState := dbgen.PutServiceStateParams{StateKey: "service-instance", StateValue: []byte(`{"generation":1}`)}
	if err := store.WithTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable}, func(queries *dbgen.Queries) error {
		return queries.PutServiceState(ctx, commitState)
	}); err != nil {
		t.Fatalf("committed WithTx() error = %v", err)
	}

	rollbackErr := errors.New("deliberate rollback")
	err = store.WithTx(ctx, pgx.TxOptions{}, func(queries *dbgen.Queries) error {
		if err := queries.PutServiceState(ctx, dbgen.PutServiceStateParams{
			StateKey:   "rolled-back",
			StateValue: []byte(`{"must":"disappear"}`),
		}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("rollback WithTx() error = %v", err)
	}
	if _, err := store.queries.GetServiceState(ctx, "rolled-back"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("rolled-back state error = %v, want pgx.ErrNoRows", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.queries.CreateJob(ctx, dbgen.CreateJobParams{
		JobID:       "durable-job-1",
		JobType:     "reconcile",
		Payload:     []byte(`{"revision":7}`),
		AvailableAt: pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	auditRecord, err := store.queries.AppendAuditRecord(ctx, dbgen.AppendAuditRecordParams{
		ActorType:  "system",
		ActorID:    "control-plane",
		Action:     "service.started",
		ObjectType: "service",
		ObjectID:   "control-plane",
		Metadata:   []byte(`{"source":"integration-test"}`),
		TraceID:    pgtype.Text{String: "trace-integration", Valid: true},
	})
	if err != nil || auditRecord.Sequence != 1 {
		t.Fatalf("AppendAuditRecord() = (%+v, %v)", auditRecord, err)
	}
	store.Close()

	restarted, err := Open(ctx, databaseURL, 4)
	if err != nil {
		t.Fatalf("restart Open() error = %v", err)
	}
	defer restarted.Close()
	state, err := restarted.queries.GetServiceState(ctx, commitState.StateKey)
	if err != nil || !equalJSON(state.StateValue, commitState.StateValue) {
		t.Fatalf("restarted state = (%+v, %v)", state, err)
	}
	job, err := restarted.queries.GetJob(ctx, "durable-job-1")
	if err != nil || job.Status != "queued" || job.Attempts != 0 || !equalJSON(job.Payload, []byte(`{"revision":7}`)) {
		t.Fatalf("restarted job = (%+v, %v)", job, err)
	}
}

func TestOpenDoesNotApplyMigrations(t *testing.T) {
	ctx := context.Background()
	databaseURL := createTestDatabase(t)

	store, err := Open(ctx, databaseURL, 2)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if err := store.Ready(ctx); !errors.Is(err, ErrSchemaNotReady) {
		t.Fatalf("Ready() error = %v, want ErrSchemaNotReady", err)
	}

	for _, relation := range []string{
		"public.goose_db_version",
		"guardian_system.service_state",
	} {
		var found sql.NullString
		if err := store.pool.QueryRow(ctx, `SELECT to_regclass($1)::text`, relation).Scan(&found); err != nil {
			t.Fatalf("look up relation %q: %v", relation, err)
		}
		if found.Valid {
			t.Fatalf("Open() unexpectedly created relation %q", relation)
		}
	}
}

func equalJSON(left, right []byte) bool {
	var leftValue any
	var rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func TestFailedMigrationDoesNotAdvanceOrPartiallyApply(t *testing.T) {
	ctx := context.Background()
	databaseURL := createTestDatabase(t)
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	broken := fstest.MapFS{
		"00001_valid.sql":   {Data: []byte("-- +goose Up\nCREATE TABLE migration_one (id integer PRIMARY KEY);\n")},
		"00002_invalid.sql": {Data: []byte("-- +goose Up\nCREATE TABLE must_rollback (id integer);\nSELECT guardian_deliberate_failure();\n")},
	}
	if _, err := applyMigrations(ctx, db, broken); err == nil {
		t.Fatal("applyMigrations() unexpectedly accepted an invalid migration")
	}

	var version int64
	if err := db.QueryRowContext(ctx, `SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = true`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("migration version = %d, want last successful version 1", version)
	}
	var relation sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('public.must_rollback')`).Scan(&relation); err != nil {
		t.Fatal(err)
	}
	if relation.Valid {
		t.Fatal("failed migration left a partial table behind")
	}
}

func createTestDatabase(t *testing.T) string {
	t.Helper()
	base := os.Getenv("GUARDIAN_TEST_DATABASE_URL")
	if base == "" {
		t.Fatal("GUARDIAN_TEST_DATABASE_URL is required for integration tests")
	}
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	databaseName := fmt.Sprintf("guardian_test_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(context.Background(), base)
	if err != nil {
		t.Fatalf("connect to test PostgreSQL: %v", err)
	}
	if _, err := admin.Exec(context.Background(), "CREATE DATABASE "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
		admin.Close(context.Background())
		t.Fatalf("create isolated test database: %v", err)
	}
	admin.Close(context.Background())

	testURL := *parsed
	testURL.Path = "/" + databaseName
	t.Cleanup(func() {
		admin, err := pgx.Connect(context.Background(), base)
		if err != nil {
			t.Errorf("reconnect for database cleanup: %v", err)
			return
		}
		defer admin.Close(context.Background())
		_, err = admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{databaseName}.Sanitize()+" WITH (FORCE)")
		if err != nil && !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("drop isolated test database: %v", err)
		}
	})
	return testURL.String()
}
