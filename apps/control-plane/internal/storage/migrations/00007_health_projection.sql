-- +goose Up

CREATE TABLE guardian_health.device_projections (
    device_id uuid PRIMARY KEY REFERENCES guardian_devices.devices(device_id),
    environment_id uuid NOT NULL REFERENCES guardian_environment.environments(environment_id),
    report_id uuid CHECK (report_id IS NULL OR (uuid_extract_version(report_id) = 7) IS TRUE),
    sequence numeric(20, 0) NOT NULL DEFAULT 0 CHECK (sequence BETWEEN 0 AND 18446744073709551615),
    observed_at timestamptz,
    received_at timestamptz NOT NULL,
    disconnected_at timestamptz,
    report_payload bytea CHECK (report_payload IS NULL OR octet_length(report_payload) BETWEEN 1 AND 16384),
    CHECK ((report_id IS NULL) = (observed_at IS NULL)),
    CHECK ((report_id IS NULL) = (report_payload IS NULL)),
    CHECK ((report_id IS NULL AND sequence = 0) OR (report_id IS NOT NULL AND sequence > 0)),
    CHECK (disconnected_at IS NULL OR disconnected_at >= received_at)
);

CREATE INDEX health_projections_environment_idx
    ON guardian_health.device_projections (environment_id, device_id);

CREATE TABLE guardian_health.device_conditions (
    device_id uuid NOT NULL REFERENCES guardian_health.device_projections(device_id) ON DELETE CASCADE,
    condition_type text NOT NULL CHECK (condition_type IN (
        'edge_connected', 'device_certificate_ready', 'config_converged',
        'local_database_healthy', 'spool_healthy', 'clock_quality',
        'container_runtime_reachable', 'privileged_helper_reachable'
    )),
    status text NOT NULL CHECK (status IN ('True', 'False', 'Unknown')),
    reason_code text NOT NULL CHECK (octet_length(reason_code) BETWEEN 1 AND 64 AND reason_code ~ '^[a-z][a-z0-9_]{0,63}$'),
    message text NOT NULL CHECK (octet_length(message) <= 512),
    observed_revision numeric(20, 0) CHECK (observed_revision BETWEEN 0 AND 18446744073709551615),
    last_transition_time timestamptz NOT NULL,
    PRIMARY KEY (device_id, condition_type)
);

-- Development recovery from schema 6 is reset/reseed. Health persistence is
-- forward-only and deliberately has no compatibility trigger or shadow table.
