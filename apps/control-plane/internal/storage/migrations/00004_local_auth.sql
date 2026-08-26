-- +goose Up

CREATE TABLE guardian_auth.bootstrap_tokens (
    token_id uuid PRIMARY KEY CHECK ((uuid_extract_version(token_id) = 7) IS TRUE),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL,
    CHECK (created_at < expires_at),
    CHECK (consumed_at IS NULL OR revoked_at IS NULL)
);

CREATE UNIQUE INDEX bootstrap_tokens_one_active
    ON guardian_auth.bootstrap_tokens ((true))
    WHERE consumed_at IS NULL AND revoked_at IS NULL;

CREATE TABLE guardian_auth.users (
    user_id uuid PRIMARY KEY CHECK ((uuid_extract_version(user_id) = 7) IS TRUE),
    username text NOT NULL UNIQUE CHECK (
        username = lower(username)
        AND username ~ '^[a-z0-9._-]{3,64}$'
    ),
    role text NOT NULL CHECK (role = 'owner'),
    status text NOT NULL CHECK (status IN ('pending_mfa', 'active')),
    password_phc text NOT NULL CHECK (
        octet_length(password_phc) BETWEEN 80 AND 512
        AND password_phc LIKE '$argon2id$v=19$%'
    ),
    totp_seed_envelope bytea NOT NULL CHECK (octet_length(totp_seed_envelope) BETWEEN 48 AND 512),
    last_totp_counter bigint NOT NULL DEFAULT -1 CHECK (last_totp_counter >= -1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (created_at <= updated_at)
);

CREATE UNIQUE INDEX users_single_owner ON guardian_auth.users ((role));

CREATE TABLE guardian_auth.recovery_codes (
    user_id uuid NOT NULL REFERENCES guardian_auth.users(user_id) ON DELETE RESTRICT,
    code_hash bytea NOT NULL UNIQUE CHECK (octet_length(code_hash) = 32),
    consumed_at timestamptz,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, code_hash)
);

CREATE TABLE guardian_auth.sessions (
    session_id uuid PRIMARY KEY CHECK ((uuid_extract_version(session_id) = 7) IS TRUE),
    user_id uuid NOT NULL REFERENCES guardian_auth.users(user_id) ON DELETE RESTRICT,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    csrf_hash bytea NOT NULL CHECK (octet_length(csrf_hash) = 32),
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    revocation_reason text CHECK (
        revocation_reason IS NULL OR revocation_reason IN (
            'logout', 'owner_revoked', 'password_changed', 'recovery_login', 'expired'
        )
    ),
    CHECK (created_at <= last_seen_at AND last_seen_at <= expires_at),
    CHECK ((revoked_at IS NULL) = (revocation_reason IS NULL))
);

CREATE INDEX sessions_user_history_idx
    ON guardian_auth.sessions (user_id, created_at DESC, session_id DESC);

CREATE TABLE guardian_auth.authentication_throttles (
    scope_hash bytea PRIMARY KEY CHECK (octet_length(scope_hash) = 32),
    window_started_at timestamptz NOT NULL,
    failures integer NOT NULL CHECK (failures BETWEEN 0 AND 1000000),
    blocked_until timestamptz,
    updated_at timestamptz NOT NULL
);

-- Development recovery is forward-only reset/reseed. Passwords, bootstrap
-- tokens, session/CSRF secrets, recovery codes, TOTP plaintext, and the
-- SecretStore master key are never stored by this schema.
