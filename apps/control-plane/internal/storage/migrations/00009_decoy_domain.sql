-- +goose Up

-- P2-W15. The decoy stops being a placeholder.
--
-- The load-bearing decision in this schema is that desired and observed state
-- are two tables, not two column groups in one. An operator asked for
-- something; a device reported something. A single row that carried both would
-- eventually be read as "this decoy is deployed" when all anyone knows is that
-- someone once asked for it, and a deception product that misreports its own
-- coverage is worse than one that admits it does not know.
--
-- guardian_deception is created by 00001_foundation, which reserved a schema
-- per domain up front. This migration fills it in rather than creating it.

-- The composite key a decoy needs so its zone cannot belong to a different
-- environment than the decoy does. zones already has an equivalent index; this
-- one is unique so it can carry a foreign key.
ALTER TABLE guardian_environment.zones
    ADD CONSTRAINT zones_environment_zone_key UNIQUE (environment_id, zone_id);

CREATE TABLE guardian_deception.decoys (
    decoy_id uuid PRIMARY KEY DEFAULT uuidv7()
        CHECK ((uuid_extract_version(decoy_id) = 7) IS TRUE),
    environment_id uuid NOT NULL
        REFERENCES guardian_environment.environments(environment_id),
    zone_id uuid NOT NULL,
    display_name text NOT NULL CHECK (octet_length(display_name) BETWEEN 1 AND 512),
    name_key text NOT NULL CHECK (octet_length(name_key) BETWEEN 1 AND 1024),
    -- DC-01: four families, closed.
    decoy_family text NOT NULL CHECK (decoy_family IN ('ssh', 'http', 'postgres', 'smb')),
    -- DC-11: a curated persona set, never free text.
    persona text NOT NULL CHECK (persona IN (
        'linux_admin_server', 'internal_admin_web_app',
        'database_server', 'windows_file_service_host'
    )),
    interaction_level text NOT NULL CHECK (interaction_level IN ('low', 'medium')),
    -- INT-01 pairs interaction level to family. SSH is medium, HTTP and SMB are
    -- low, and a database surface may be either.
    CHECK (
        (decoy_family = 'ssh' AND interaction_level = 'medium')
        OR (decoy_family IN ('http', 'smb') AND interaction_level = 'low')
        OR decoy_family = 'postgres'
    ),
    address inet NOT NULL CHECK (
        pg_catalog.family(address) = 4
        AND masklen(address) = 32
        AND (
            address <<= '10.0.0.0/8'::inet
            OR address <<= '172.16.0.0/12'::inet
            OR address <<= '192.168.0.0/16'::inet
        )
    ),
    -- DC-12: the pack triple. It is opaque here; P2-W4 owns what a manifest
    -- contains. pack_digest is NULL until a manifest exists to hash, because a
    -- fabricated digest asserts an artifact identity nothing has verified.
    pack text NOT NULL CHECK (pack ~ '^[a-z][a-z0-9-]{0,63}$'),
    pack_version text NOT NULL CHECK (pack_version ~ '^[0-9]{1,6}\.[0-9]{1,6}\.[0-9]{1,6}$'),
    pack_digest text CHECK (pack_digest IS NULL OR pack_digest ~ '^sha256:[0-9a-f]{64}$'),
    desired_state text NOT NULL CHECK (desired_state IN ('deployed', 'disabled', 'removed')),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (created_at <= updated_at),
    FOREIGN KEY (environment_id, zone_id)
        REFERENCES guardian_environment.zones(environment_id, zone_id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT
);

-- A removed decoy keeps its row so its identity, and therefore every audit
-- record and interaction attributed to it, stays resolvable. It leaves the
-- active list, so it also releases its name and address for reuse.
CREATE UNIQUE INDEX decoys_active_name_key
    ON guardian_deception.decoys (environment_id, name_key)
    WHERE desired_state <> 'removed';

CREATE UNIQUE INDEX decoys_active_address_key
    ON guardian_deception.decoys (environment_id, address)
    WHERE desired_state <> 'removed';

CREATE INDEX decoys_environment_idx
    ON guardian_deception.decoys (environment_id, decoy_id);

CREATE INDEX decoys_zone_idx
    ON guardian_deception.decoys (zone_id);

-- Observed state. A missing row means unknown, and the read path materializes
-- that rather than defaulting a column. There is deliberately no default row
-- created alongside a decoy: an empty table is the truthful representation of
-- "nothing has reported".
CREATE TABLE guardian_deception.decoy_observed_state (
    decoy_id uuid PRIMARY KEY
        REFERENCES guardian_deception.decoys(decoy_id) ON DELETE CASCADE,
    reporting_device_id uuid NOT NULL REFERENCES guardian_devices.devices(device_id),
    report_id uuid NOT NULL CHECK ((uuid_extract_version(report_id) = 7) IS TRUE),
    -- 'unmanaged' is absent here on purpose. It is projected at read time from
    -- device state per SEC-06, never stored, because it is a statement about
    -- Guardian's ability to manage the decoy rather than about the decoy.
    observed_state text NOT NULL
        CHECK (observed_state IN ('unknown', 'deployed', 'degraded', 'absent')),
    last_interaction_at timestamptz,
    desired_revision bigint CHECK (desired_revision IS NULL OR desired_revision > 0),
    observed_at timestamptz NOT NULL,
    reported_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (last_interaction_at IS NULL OR last_interaction_at <= observed_at)
);

CREATE TABLE guardian_deception.decoy_conditions (
    decoy_id uuid NOT NULL
        REFERENCES guardian_deception.decoy_observed_state(decoy_id) ON DELETE CASCADE,
    condition_type text NOT NULL CHECK (condition_type IN (
        'runtime_healthy', 'address_applied', 'port_responding',
        'telemetry_reporting', 'policy_applied', 'version_matches_desired'
    )),
    status text NOT NULL CHECK (status IN ('True', 'False', 'Unknown')),
    reason_code text NOT NULL
        CHECK (octet_length(reason_code) BETWEEN 1 AND 64 AND reason_code ~ '^[a-z][a-z0-9_]{0,63}$'),
    message text NOT NULL CHECK (octet_length(message) <= 512),
    observed_revision numeric(20, 0)
        CHECK (observed_revision IS NULL OR observed_revision BETWEEN 0 AND 18446744073709551615),
    last_transition_time timestamptz NOT NULL,
    PRIMARY KEY (decoy_id, condition_type)
);

-- The audit vocabulary gains five decoy pairs. The constraint is a CHECK
-- rather than a lookup table, so extending it means replacing it; every
-- existing pair is carried over verbatim.
ALTER TABLE guardian_audit.records DROP CONSTRAINT audit_action_object_pair;

ALTER TABLE guardian_audit.records ADD CONSTRAINT audit_action_object_pair CHECK ((action, object_type) IN (
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
    ('auth.csrf.reissued', 'session'),
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
    -- The new pairs. Every decoy lifecycle transition is auditable, and the
    -- audit trail outlives the decoy leaving the active list.
    ('decoy.created', 'decoy'),
    ('decoy.updated', 'decoy'),
    ('decoy.removed', 'decoy'),
    ('decoy.enabled', 'decoy'),
    ('decoy.disabled', 'decoy'),
    ('desired_state.revision.published', 'desired_state_revision'),
    ('security.action.denied', 'security_action')
));

-- This development migration is forward-only. Recovery is code rollback
-- followed by an explicit database reset and reseed.
