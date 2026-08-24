-- +goose Up

-- Phase 1 is still development-only. Replace the foundation audit stub rather
-- than preserving or backfilling its pre-contract rows.
DROP TABLE guardian_audit.records;

CREATE TABLE guardian_audit.records (
    sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY
        CHECK (sequence > 0),
    event_id uuid NOT NULL DEFAULT uuidv7() UNIQUE
        CHECK ((uuid_extract_version(event_id) = 7) IS TRUE),
    occurred_at timestamptz NOT NULL CHECK ((
        isfinite(occurred_at)
        AND occurred_at > TIMESTAMPTZ '0001-01-01 00:00:00+00'
        AND occurred_at < TIMESTAMPTZ '10000-01-01 00:00:00+00'
    ) IS TRUE),
    recorded_at timestamptz NOT NULL DEFAULT clock_timestamp() CHECK ((
        isfinite(recorded_at)
        AND recorded_at > TIMESTAMPTZ '0001-01-01 00:00:00+00'
        AND recorded_at < TIMESTAMPTZ '10000-01-01 00:00:00+00'
    ) IS TRUE),
    actor_type text NOT NULL CHECK (actor_type IN ('system', 'user', 'device')),
    actor_id text NOT NULL CHECK (octet_length(actor_id) BETWEEN 1 AND 256),
    action text NOT NULL,
    object_type text NOT NULL,
    object_id text NOT NULL CHECK (octet_length(object_id) BETWEEN 1 AND 256),
    correlation_id text NOT NULL CHECK (octet_length(correlation_id) BETWEEN 1 AND 128),
    request_id text,
    -- json preserves the domain's compact canonical envelope bytes, allowing
    -- the database to enforce the exact approved 16 KiB encoded limit. jsonb
    -- textual reserialization can add whitespace at this boundary.
    before_snapshot json,
    after_snapshot json,
    CHECK (request_id IS NULL OR octet_length(request_id) BETWEEN 1 AND 128),
    CHECK (
        before_snapshot IS NULL
        OR (
            json_typeof(before_snapshot) = 'object'
            AND before_snapshot->>'schema' = 'guardian.audit.snapshot.v1'
            AND json_typeof(before_snapshot->'data') = 'object'
            AND octet_length(before_snapshot::text) <= 16384
        ) IS TRUE
    ),
    CHECK (
        after_snapshot IS NULL
        OR (
            json_typeof(after_snapshot) = 'object'
            AND after_snapshot->>'schema' = 'guardian.audit.snapshot.v1'
            AND json_typeof(after_snapshot->'data') = 'object'
            AND octet_length(after_snapshot::text) <= 16384
        ) IS TRUE
    ),
    CONSTRAINT audit_action_object_pair CHECK ((action, object_type) IN (
        ('auth.bootstrap_token.created', 'bootstrap_token'),
        ('auth.bootstrap.succeeded', 'user'),
        ('auth.bootstrap.failed', 'bootstrap_token'),
        ('auth.login.succeeded', 'user'),
        ('auth.login.failed', 'user'),
        ('auth.logout', 'session'),
        ('auth.password.changed', 'user'),
        ('auth.mfa.enrolled', 'user'),
        ('auth.recovery_code.used', 'user'),
        ('auth.session.revoked', 'session'),
        ('auth.security_setting.changed', 'security_setting'),
        ('device.enrollment_token.created', 'enrollment_token'),
        ('device.enrollment_token.revoked', 'enrollment_token'),
        ('device.enrollment.succeeded', 'device'),
        ('device.enrollment.failed', 'device'),
        ('device.certificate.issued', 'device_certificate'),
        ('device.certificate.rotated', 'device_certificate'),
        ('device.disabled', 'device'),
        ('device.revoked', 'device'),
        ('environment.created', 'environment'),
        ('environment.updated', 'environment'),
        ('zone.created', 'zone'),
        ('zone.updated', 'zone'),
        ('zone.removed', 'zone'),
        ('desired_state.revision.published', 'desired_state_revision'),
        ('security.action.denied', 'security_action')
    ))
);

CREATE INDEX audit_records_action_sequence_idx
    ON guardian_audit.records (action, sequence DESC);

CREATE INDEX audit_records_correlation_sequence_idx
    ON guardian_audit.records (correlation_id, sequence DESC);

CREATE INDEX audit_records_object_sequence_idx
    ON guardian_audit.records (object_type, object_id, sequence DESC);

-- +goose StatementBegin
CREATE FUNCTION guardian_audit.reject_record_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION USING
        ERRCODE = '55000',
        MESSAGE = 'guardian audit records are append-only';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_records_append_only
BEFORE UPDATE OR DELETE OR TRUNCATE ON guardian_audit.records
FOR EACH STATEMENT
EXECUTE FUNCTION guardian_audit.reject_record_mutation();

-- This development migration is intentionally forward-only. Recovery is code
-- rollback followed by an explicit database reset and reseed.
