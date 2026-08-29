-- +goose Up

CREATE SCHEMA guardian_reconciliation;

CREATE TABLE guardian_reconciliation.desired_state_revisions (
    device_id uuid NOT NULL REFERENCES guardian_devices.devices(device_id),
    revision bigint NOT NULL CHECK (revision > 0),
    message_id uuid NOT NULL UNIQUE CHECK ((uuid_extract_version(message_id) = 7) IS TRUE),
    content_sha256 bytea NOT NULL CHECK (octet_length(content_sha256) = 32),
    payload jsonb NOT NULL CHECK (pg_column_size(payload) BETWEEN 2 AND 1048576),
    acknowledged_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (device_id, revision)
);

CREATE INDEX desired_state_latest_idx
    ON guardian_reconciliation.desired_state_revisions (device_id, revision DESC);

CREATE TABLE guardian_reconciliation.observed_messages (
    device_id uuid NOT NULL REFERENCES guardian_devices.devices(device_id),
    message_id uuid NOT NULL CHECK ((uuid_extract_version(message_id) = 7) IS TRUE),
    received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (device_id, message_id)
);

CREATE TABLE guardian_reconciliation.observed_state (
    device_id uuid PRIMARY KEY REFERENCES guardian_devices.devices(device_id),
    message_id uuid NOT NULL CHECK ((uuid_extract_version(message_id) = 7) IS TRUE),
    desired_revision bigint NOT NULL CHECK (desired_revision > 0),
    observed_revision bigint NOT NULL CHECK (observed_revision >= 0),
    last_good_revision bigint NOT NULL CHECK (last_good_revision >= 0),
    condition_status text NOT NULL
        CHECK (condition_status IN ('pending', 'converged', 'retrying', 'failed')),
    reason_code text NOT NULL CHECK (reason_code ~ '^[a-z0-9_.-]{1,64}$'),
    attempt_count integer NOT NULL CHECK (attempt_count BETWEEN 0 AND 6),
    retry_at timestamptz,
    last_transition_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (observed_revision <= desired_revision),
    CHECK (last_good_revision <= observed_revision),
    CHECK ((condition_status = 'retrying') = (retry_at IS NOT NULL)),
    CHECK (condition_status <> 'converged' OR (
        observed_revision = desired_revision AND last_good_revision = observed_revision
    ))
);

-- Desired payloads contain bounded configuration metadata only. Secrets,
-- credentials, certificates, artifacts, and raw error text are prohibited by
-- the domain validator and are not represented by the schema.
