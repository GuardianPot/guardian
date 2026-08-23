-- +goose Up

CREATE SCHEMA guardian_system;
CREATE SCHEMA guardian_auth;
CREATE SCHEMA guardian_environment;
CREATE SCHEMA guardian_devices;
CREATE SCHEMA guardian_deception;
CREATE SCHEMA guardian_health;
CREATE SCHEMA guardian_audit;
CREATE SCHEMA guardian_jobs;
CREATE SCHEMA guardian_api;

CREATE TABLE guardian_system.service_state (
    state_key text PRIMARY KEY,
    state_value jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE guardian_jobs.jobs (
    job_id text PRIMARY KEY,
    job_type text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    lease_owner text,
    lease_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((status = 'running') = (lease_owner IS NOT NULL AND lease_until IS NOT NULL))
);

CREATE INDEX jobs_available_idx
    ON guardian_jobs.jobs (status, available_at, created_at);

CREATE TABLE guardian_audit.records (
    sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor_type text NOT NULL,
    actor_id text NOT NULL,
    action text NOT NULL,
    object_type text NOT NULL,
    object_id text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    trace_id text
);

CREATE INDEX audit_records_occurred_idx
    ON guardian_audit.records (occurred_at, sequence);

-- This development migration is intentionally forward-only. Recovery is code
-- rollback followed by an explicit database reset and reseed.
