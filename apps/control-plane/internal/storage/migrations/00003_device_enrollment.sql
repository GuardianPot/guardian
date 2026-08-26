-- +goose Up

CREATE TABLE guardian_devices.certificate_authority (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    certificate_pem bytea NOT NULL CHECK (octet_length(certificate_pem) BETWEEN 1 AND 16384),
    private_key_envelope bytea NOT NULL CHECK (octet_length(private_key_envelope) BETWEEN 32 AND 16384),
    not_after timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE guardian_devices.devices (
    device_id uuid PRIMARY KEY CHECK ((uuid_extract_version(device_id) = 7) IS TRUE),
    environment_id uuid NOT NULL,
    display_name text NOT NULL CHECK (octet_length(display_name) BETWEEN 1 AND 128),
    state text NOT NULL CHECK (state IN ('pending', 'active', 'disabled', 'revoked')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (created_at <= updated_at)
);

CREATE INDEX devices_environment_idx
    ON guardian_devices.devices (environment_id, device_id);

CREATE TABLE guardian_devices.enrollment_tokens (
    token_id uuid PRIMARY KEY CHECK ((uuid_extract_version(token_id) = 7) IS TRUE),
    device_id uuid NOT NULL REFERENCES guardian_devices.devices(device_id),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL,
    CHECK (created_at < expires_at),
    CHECK (consumed_at IS NULL OR revoked_at IS NULL)
);

CREATE INDEX enrollment_tokens_device_idx
    ON guardian_devices.enrollment_tokens (device_id, created_at DESC);

CREATE TABLE guardian_devices.certificates (
    serial text PRIMARY KEY CHECK (serial ~ '^[0-9a-f]{1,32}$'),
    device_id uuid NOT NULL REFERENCES guardian_devices.devices(device_id),
    fingerprint_sha256 text NOT NULL UNIQUE CHECK (fingerprint_sha256 ~ '^[0-9a-f]{64}$'),
    certificate_pem bytea NOT NULL CHECK (octet_length(certificate_pem) BETWEEN 1 AND 32768),
    not_before timestamptz NOT NULL,
    not_after timestamptz NOT NULL,
    state text NOT NULL CHECK (state IN ('active', 'revoked')),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL,
    CHECK (not_before < not_after),
    CHECK ((state = 'revoked') = (revoked_at IS NOT NULL))
);

CREATE UNIQUE INDEX certificates_one_active_per_device
    ON guardian_devices.certificates (device_id)
    WHERE state = 'active';

CREATE INDEX certificates_device_history_idx
    ON guardian_devices.certificates (device_id, created_at DESC);

CREATE TABLE guardian_devices.enrollment_throttles (
    scope_hash bytea PRIMARY KEY CHECK (octet_length(scope_hash) = 32),
    window_started_at timestamptz NOT NULL,
    failures integer NOT NULL CHECK (failures BETWEEN 0 AND 1000000),
    blocked_until timestamptz,
    updated_at timestamptz NOT NULL
);

-- Development migration recovery is reset/reseed. No plaintext enrollment
-- token, Edge key, or CA private key is stored by this schema.
