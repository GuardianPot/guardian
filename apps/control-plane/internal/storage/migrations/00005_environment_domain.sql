-- +goose Up

CREATE TABLE guardian_environment.organizations (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    organization_id uuid NOT NULL UNIQUE DEFAULT uuidv7()
        CHECK ((uuid_extract_version(organization_id) = 7) IS TRUE),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

INSERT INTO guardian_environment.organizations (singleton) VALUES (true);

-- +goose StatementBegin
CREATE FUNCTION guardian_environment.reject_organization_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'the Phase 1 organization singleton is immutable'
        USING ERRCODE = '55000';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER organization_immutable
BEFORE UPDATE OR DELETE ON guardian_environment.organizations
FOR EACH ROW EXECUTE FUNCTION guardian_environment.reject_organization_mutation();

CREATE TABLE guardian_environment.environments (
    environment_id uuid PRIMARY KEY DEFAULT uuidv7()
        CHECK ((uuid_extract_version(environment_id) = 7) IS TRUE),
    organization_id uuid NOT NULL
        REFERENCES guardian_environment.organizations(organization_id),
    display_name text NOT NULL CHECK (octet_length(display_name) BETWEEN 1 AND 512),
    name_key text NOT NULL CHECK (octet_length(name_key) BETWEEN 1 AND 1024),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (created_at <= updated_at),
    UNIQUE (organization_id, name_key)
);

CREATE TABLE guardian_environment.zones (
    zone_id uuid PRIMARY KEY DEFAULT uuidv7()
        CHECK ((uuid_extract_version(zone_id) = 7) IS TRUE),
    environment_id uuid NOT NULL
        REFERENCES guardian_environment.environments(environment_id),
    display_name text NOT NULL CHECK (octet_length(display_name) BETWEEN 1 AND 512),
    name_key text NOT NULL CHECK (octet_length(name_key) BETWEEN 1 AND 1024),
    network cidr NOT NULL CHECK (
        family(network) = 4
        AND masklen(network) > 0
        AND (
            network <<= '10.0.0.0/8'::cidr
            OR network <<= '172.16.0.0/12'::cidr
            OR network <<= '192.168.0.0/16'::cidr
        )
    ),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (created_at <= updated_at),
    UNIQUE (environment_id, name_key),
    UNIQUE (environment_id, network)
);

CREATE INDEX zones_environment_idx
    ON guardian_environment.zones (environment_id, zone_id);

ALTER TABLE guardian_devices.devices
    ADD CONSTRAINT devices_environment_fk
    FOREIGN KEY (environment_id)
    REFERENCES guardian_environment.environments(environment_id)
    ON UPDATE RESTRICT
    ON DELETE RESTRICT;
