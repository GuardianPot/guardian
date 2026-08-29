package storage

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
)

const schemaSQL = `
CREATE TABLE identity_metadata (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    certificate_sha256 TEXT NOT NULL CHECK (length(certificate_sha256) = 64),
    certificate_not_before INTEGER NOT NULL,
    certificate_not_after INTEGER NOT NULL,
    enrollment_status TEXT NOT NULL CHECK (enrollment_status IN ('enrolled', 'revoked', 'unavailable')),
    updated_at INTEGER NOT NULL
);

CREATE TABLE state_revisions (
    object_kind TEXT PRIMARY KEY,
    desired_revision INTEGER NOT NULL CHECK (desired_revision >= 0),
    observed_revision INTEGER NOT NULL CHECK (observed_revision >= 0),
    last_good_revision INTEGER NOT NULL CHECK (last_good_revision >= 0),
    condition_code TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE retry_metadata (
    operation_id TEXT PRIMARY KEY,
    attempts INTEGER NOT NULL CHECK (attempts >= 0),
    next_attempt_at INTEGER NOT NULL,
    last_error_code TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE component_health (
    name TEXT PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('healthy', 'degraded', 'failed', 'stopped', 'unknown')),
    reason_code TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE health_report_state (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    next_sequence TEXT NOT NULL CHECK (length(next_sequence) BETWEEN 1 AND 20),
    pending_report_id TEXT CHECK (pending_report_id IS NULL OR length(pending_report_id) = 36),
    pending_sequence TEXT CHECK (pending_sequence IS NULL OR length(pending_sequence) BETWEEN 1 AND 20),
    pending_payload BLOB CHECK (pending_payload IS NULL OR length(pending_payload) BETWEEN 1 AND 16384),
    pending_created_at INTEGER,
    acknowledged_report_id TEXT CHECK (acknowledged_report_id IS NULL OR length(acknowledged_report_id) = 36),
    acknowledged_sequence TEXT CHECK (acknowledged_sequence IS NULL OR length(acknowledged_sequence) BETWEEN 1 AND 20),
    acknowledged_at INTEGER,
    updated_at INTEGER NOT NULL,
    CHECK ((pending_report_id IS NULL) = (pending_sequence IS NULL)),
    CHECK ((pending_report_id IS NULL) = (pending_payload IS NULL)),
    CHECK ((pending_report_id IS NULL) = (pending_created_at IS NULL)),
    CHECK ((acknowledged_report_id IS NULL) = (acknowledged_sequence IS NULL)),
    CHECK ((acknowledged_report_id IS NULL) = (acknowledged_at IS NULL))
);

INSERT INTO health_report_state (singleton, next_sequence, updated_at)
VALUES (1, '1', 0);

CREATE TABLE canonical_health_conditions (
    condition_type TEXT PRIMARY KEY CHECK (condition_type IN (
        'edge_connected', 'device_certificate_ready', 'config_converged',
        'local_database_healthy', 'spool_healthy', 'clock_quality',
        'container_runtime_reachable', 'privileged_helper_reachable'
    )),
    status TEXT NOT NULL CHECK (status IN ('True', 'False', 'Unknown')),
    reason_code TEXT NOT NULL CHECK (length(reason_code) BETWEEN 1 AND 64),
    message TEXT NOT NULL CHECK (length(CAST(message AS BLOB)) <= 512),
    observed_revision TEXT CHECK (observed_revision IS NULL OR length(observed_revision) BETWEEN 1 AND 20),
    last_transition_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE reconciliation_state (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    desired_message_id TEXT NOT NULL CHECK (length(desired_message_id) = 36),
    desired_revision INTEGER NOT NULL CHECK (desired_revision > 0),
    observed_revision INTEGER NOT NULL CHECK (observed_revision >= 0),
    last_good_revision INTEGER NOT NULL CHECK (last_good_revision >= 0),
    desired_digest BLOB NOT NULL CHECK (length(desired_digest) = 32),
    desired_payload BLOB NOT NULL CHECK (length(desired_payload) BETWEEN 1 AND 1048576),
    last_good_digest BLOB,
    last_good_payload BLOB,
    condition_status TEXT NOT NULL CHECK (condition_status IN ('pending', 'converged', 'retrying', 'failed')),
    reason_code TEXT NOT NULL CHECK (length(reason_code) BETWEEN 1 AND 64),
    attempt_count INTEGER NOT NULL CHECK (attempt_count BETWEEN 0 AND 6),
    retry_at INTEGER,
    observed_message_id TEXT NOT NULL CHECK (length(observed_message_id) = 36),
    observed_pending INTEGER NOT NULL CHECK (observed_pending IN (0, 1)),
    last_transition_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK (observed_revision <= desired_revision),
    CHECK (last_good_revision <= observed_revision),
    CHECK (
        (last_good_revision = 0 AND last_good_digest IS NULL AND last_good_payload IS NULL)
        OR
        (last_good_revision > 0 AND length(last_good_digest) = 32 AND length(last_good_payload) BETWEEN 1 AND 1048576)
    ),
    CHECK ((condition_status = 'retrying') = (retry_at IS NOT NULL)),
    CHECK (condition_status <> 'converged' OR (
        observed_revision = desired_revision AND last_good_revision = observed_revision
    ))
);

CREATE TABLE spool_objects (
    digest TEXT PRIMARY KEY CHECK (length(digest) = 64),
    relative_path TEXT NOT NULL UNIQUE,
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    state TEXT NOT NULL CHECK (state IN ('available', 'quarantined')),
    created_at INTEGER NOT NULL
);

CREATE TABLE durable_events (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    payload_digest TEXT NOT NULL REFERENCES spool_objects(digest),
    payload_size INTEGER NOT NULL CHECK (payload_size >= 0),
    state TEXT NOT NULL CHECK (state IN ('pending', 'inflight', 'delivered')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at INTEGER NOT NULL,
    lease_until INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    delivered_at INTEGER
);

CREATE INDEX durable_events_available_idx
    ON durable_events (state, available_at, sequence);
CREATE INDEX retry_metadata_due_idx
    ON retry_metadata (next_attempt_at, operation_id);
`

func initializeSchema(ctx context.Context, db *sql.DB) error {
	version, err := readSchemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if version == currentSchemaVersion {
		return verifyRequiredTables(ctx, db)
	}
	if version != 0 {
		return fmt.Errorf("%w: found version %d, expected %d", ErrSchemaIncompatible, version, currentSchemaVersion)
	}
	count, err := userTableCount(ctx, db)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("%w: unversioned development database contains tables", ErrSchemaIncompatible)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return classifyError("begin edge schema transaction", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, schemaSQL); err != nil {
		return classifyError("create edge schema", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", currentSchemaVersion)); err != nil {
		return classifyError("record edge schema version", err)
	}
	if err := tx.Commit(); err != nil {
		return classifyError("commit edge schema", err)
	}
	return verifyRequiredTables(ctx, db)
}

func verifySchema(ctx context.Context, db *sql.DB) error {
	version, err := readSchemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if version != currentSchemaVersion {
		return fmt.Errorf("%w: found version %d, expected %d", ErrSchemaIncompatible, version, currentSchemaVersion)
	}
	return verifyRequiredTables(ctx, db)
}

func readSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, classifyError("read edge schema version", err)
	}
	return version, nil
}

func userTableCount(ctx context.Context, db *sql.DB) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM sqlite_master
WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&count); err != nil {
		return 0, classifyError("count edge database tables", err)
	}
	return count, nil
}

func verifyRequiredTables(ctx context.Context, db *sql.DB) error {
	required := map[string][]string{
		"identity_metadata":           {"singleton", "certificate_sha256", "certificate_not_before", "certificate_not_after", "enrollment_status", "updated_at"},
		"state_revisions":             {"object_kind", "desired_revision", "observed_revision", "last_good_revision", "condition_code", "updated_at"},
		"retry_metadata":              {"operation_id", "attempts", "next_attempt_at", "last_error_code", "updated_at"},
		"component_health":            {"name", "status", "reason_code", "updated_at"},
		"health_report_state":         {"singleton", "next_sequence", "pending_report_id", "pending_sequence", "pending_payload", "pending_created_at", "acknowledged_report_id", "acknowledged_sequence", "acknowledged_at", "updated_at"},
		"canonical_health_conditions": {"condition_type", "status", "reason_code", "message", "observed_revision", "last_transition_at", "updated_at"},
		"reconciliation_state":        {"singleton", "desired_message_id", "desired_revision", "observed_revision", "last_good_revision", "desired_digest", "desired_payload", "last_good_digest", "last_good_payload", "condition_status", "reason_code", "attempt_count", "retry_at", "observed_message_id", "observed_pending", "last_transition_at", "updated_at"},
		"spool_objects":               {"digest", "relative_path", "size_bytes", "state", "created_at"},
		"durable_events":              {"sequence", "event_id", "payload_digest", "payload_size", "state", "attempts", "available_at", "lease_until", "created_at", "updated_at", "delivered_at"},
	}
	for table, expectedColumns := range required {
		rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
		if err != nil {
			return classifyError("verify edge schema table", err)
		}
		columns := []string{}
		for rows.Next() {
			var columnID, notNull, primaryKey int
			var name, dataType string
			var defaultValue any
			if err := rows.Scan(&columnID, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
				rows.Close()
				return classifyError("scan edge schema table", err)
			}
			columns = append(columns, name)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return classifyError("iterate edge schema table", err)
		}
		rows.Close()
		if !slices.Equal(columns, expectedColumns) {
			return fmt.Errorf("%w: table %s columns do not match version %d", ErrSchemaIncompatible, table, currentSchemaVersion)
		}
	}
	for _, index := range []string{"durable_events_available_idx", "retry_metadata_due_idx"} {
		var count int
		if err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", index).Scan(&count); err != nil {
			return classifyError("verify edge schema index", err)
		}
		if count != 1 {
			return fmt.Errorf("%w: required index %s is missing", ErrSchemaIncompatible, index)
		}
	}
	return nil
}
