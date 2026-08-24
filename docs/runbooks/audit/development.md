# Audit development and operations runbook

## Contract

P1-W10 stores first-class audit events in PostgreSQL 18. Each event has a
durable UUIDv7 `event_id`, caller-supplied occurrence time, database-recorded
time, actor/action/object identity, required correlation identity, optional
request identity, and optional bounded before/after snapshots. The internal
identity sequence exists only for deterministic keyset pagination.

Both timestamps must be finite UTC instants strictly after the Go zero time once
normalized to PostgreSQL's microsecond precision (the minimum is zero plus one
microsecond), and before UTC year 10000. The storage pool decodes PostgreSQL
`timestamptz` in UTC so a valid instant near a year boundary cannot become
non-serializable in the host timezone.

The action and object vocabulary is closed to the approved Phase 1 auth,
device, environment, desired-state, and denied-security operations. Adding a
new vocabulary value requires the owner-reviewed package that owns that
product behavior; do not insert arbitrary strings to bypass validation.

## Repeatable acceptance fixture

From the repository root, run:

```bash
bash tests/integration/audit/run.sh
```

The fixture starts a uniquely named disposable PostgreSQL 18 Compose project
with random owner credentials used only inside the fixture. Its integration
tests also assume a constrained runtime role to prove allowed appends and
denied generated-column overrides. The fixture exercises fresh migration,
append and rollback atomically, database mutation denial, deterministic
pagination, restart persistence, concurrent appends, snapshot redaction and
bounds, and HTTP authorization behavior. Cleanup removes only that named
disposable project and its volume.

The complete repository gate remains:

```bash
task validate
```

## Runtime database role recipe

Provision roles with an authorized database administrator. The serving process
must not connect as the database owner, a superuser, or a role allowed to
alter/disable triggers. The following audit-specific grants assume an existing
login/group role named `guardian_control_plane_runtime`; authentication and
password provisioning are deployment responsibilities and are intentionally
not shown:

```sql
REVOKE ALL ON SCHEMA guardian_audit FROM PUBLIC;
REVOKE ALL ON TABLE guardian_audit.records FROM PUBLIC;
REVOKE ALL ON SEQUENCE guardian_audit.records_sequence_seq FROM PUBLIC;
REVOKE ALL ON SCHEMA guardian_audit
    FROM guardian_control_plane_runtime;
REVOKE ALL ON TABLE guardian_audit.records
    FROM guardian_control_plane_runtime;
REVOKE ALL ON SEQUENCE guardian_audit.records_sequence_seq
    FROM guardian_control_plane_runtime;

GRANT USAGE ON SCHEMA guardian_audit
    TO guardian_control_plane_runtime;
GRANT SELECT ON TABLE guardian_audit.records
    TO guardian_control_plane_runtime;
GRANT INSERT (
    occurred_at,
    actor_type,
    actor_id,
    action,
    object_type,
    object_id,
    correlation_id,
    request_id,
    before_snapshot,
    after_snapshot
) ON TABLE guardian_audit.records
    TO guardian_control_plane_runtime;
GRANT USAGE ON SEQUENCE guardian_audit.records_sequence_seq
    TO guardian_control_plane_runtime;

REVOKE UPDATE, DELETE, TRUNCATE ON TABLE guardian_audit.records
    FROM guardian_control_plane_runtime;
```

Verify the effective table privileges before serving:

```sql
SELECT privilege_type
FROM information_schema.role_table_grants
WHERE grantee = 'guardian_control_plane_runtime'
  AND table_schema = 'guardian_audit'
  AND table_name = 'records'
ORDER BY privilege_type;
```

The table-level result contains only `SELECT`; `INSERT` is deliberately a
column-level grant. Verify that exact insert allowlist separately:

```sql
SELECT column_name
FROM information_schema.role_column_grants
WHERE grantee = 'guardian_control_plane_runtime'
  AND table_schema = 'guardian_audit'
  AND table_name = 'records'
  AND privilege_type = 'INSERT'
ORDER BY column_name;
```

The expected columns are `action`, `actor_id`, `actor_type`, `after_snapshot`,
`before_snapshot`, `correlation_id`, `object_id`, `object_type`, `occurred_at`,
and `request_id`. In particular, the role cannot supply `sequence`, `event_id`,
or `recorded_at`, including through `INSERT ... OVERRIDING SYSTEM VALUE`; those
remain database generated. The database trigger also rejects `UPDATE`,
`DELETE`, and `TRUNCATE`, but this is defense in depth, not a cryptographic
ledger: a database administrator or table owner can alter schema and trigger
state. Guardian makes no WORM/notarization claim in Phase 1.

Verify the three denied columns against the effective role, including inherited
grants, rather than relying only on grant catalogs:

```sql
SELECT column_name,
       has_column_privilege(
           'guardian_control_plane_runtime',
           'guardian_audit.records',
           column_name,
           'INSERT'
       ) AS may_insert
FROM (VALUES ('event_id'), ('recorded_at'), ('sequence')) AS denied(column_name)
ORDER BY column_name;
```

Every `may_insert` value must be `false`. The serving login must not inherit or
be able to assume another role that restores broader schema, table, or sequence
privileges. The acceptance fixture also executes denied override statements as
the constrained role; repeat that negative test after deployment grants change.

Audit the remaining effective privilege and membership surface:

```sql
SELECT
    has_schema_privilege(
        'guardian_control_plane_runtime', 'guardian_audit', 'CREATE'
    ) AS schema_create,
    has_table_privilege(
        'guardian_control_plane_runtime', 'guardian_audit.records', 'INSERT'
    ) AS table_wide_insert,
    has_table_privilege(
        'guardian_control_plane_runtime', 'guardian_audit.records', 'UPDATE'
    ) AS table_update,
    has_table_privilege(
        'guardian_control_plane_runtime', 'guardian_audit.records', 'DELETE'
    ) AS table_delete,
    has_table_privilege(
        'guardian_control_plane_runtime', 'guardian_audit.records', 'TRUNCATE'
    ) AS table_truncate,
    has_sequence_privilege(
        'guardian_control_plane_runtime',
        'guardian_audit.records_sequence_seq',
        'SELECT'
    ) AS sequence_select,
    has_sequence_privilege(
        'guardian_control_plane_runtime',
        'guardian_audit.records_sequence_seq',
        'UPDATE'
    ) AS sequence_update;

SELECT parent.rolname AS parent_role,
       membership.inherit_option,
       membership.set_option,
       membership.admin_option
FROM pg_auth_members AS membership
JOIN pg_roles AS member ON member.oid = membership.member
JOIN pg_roles AS parent ON parent.oid = membership.roleid
WHERE member.rolname = 'guardian_control_plane_runtime'
ORDER BY parent.rolname;
```

Every privilege boolean must be `false`, and the membership result must be
empty unless each parent role has been separately reviewed as no broader than
this recipe. Run the same membership audit for the actual serving login; it
must not have an alternate path to broader Guardian database privileges.

## Safe snapshot construction

Callers must build an explicit safe projection through the audit snapshot
constructor. Never pass an HTTP request, domain aggregate, database row, or
credential structure wholesale. The constructor recursively replaces known
secret fields and rejects unsupported or oversized content; it never silently
truncates a snapshot. A rejection must abort the containing domain mutation so
the mutation cannot commit without its required audit event.

Each `guardian.audit.snapshot.v1` envelope is limited to 16 KiB, depth 6, 256
nodes, 64 object members or array elements, 64 bytes per key, and 512 bytes per
string. Passwords, session/bootstrap/enrollment/recovery tokens, TOTP material,
private keys, authorization/cookie values, and equivalent known secret fields
must not be stored or logged.

PostgreSQL stores the constructor's canonical envelope as `json` so its
encoded-byte check is exact; snapshot contents are not a query surface. Action,
object, correlation, and sequence access use typed indexed columns. Timestamps
are typed record fields but are intentionally not Phase 1 API filters.

A representative API record has this shape; IDs and timestamps below are
illustrative, and the internal sequence is intentionally absent:

```json
{
  "event_id": "0198dc8c-c600-7000-8000-000000000001",
  "occurred_at": "2026-08-24T12:00:00Z",
  "recorded_at": "2026-08-24T12:00:00.010Z",
  "actor": { "type": "user", "id": "user-1" },
  "action": "auth.security_setting.changed",
  "object": { "type": "security_setting", "id": "mfa-policy" },
  "correlation_id": "correlation-1",
  "request_id": "request-1",
  "before": {
    "schema": "guardian.audit.snapshot.v1",
    "data": { "enabled": false, "session_token": "[REDACTED]" }
  },
  "after": {
    "schema": "guardian.audit.snapshot.v1",
    "data": { "enabled": true }
  }
}
```

## Read API

`GET /v1/audit/events` uses an opaque versioned Base64URL cursor, newest-first
keyset pagination, a default page size of 50, and a maximum of 200. The first
page fixes a sequence anchor so later concurrent appends cannot shift the
remaining traversal. Supported filters are exact action and correlation ID,
plus object type and object ID supplied together. Clients must treat cursors as
opaque and start a new traversal when changing filters.

The storage adapter takes a shared transaction advisory lock before allocating
an append sequence and holds it through commit. First-page anchor acquisition
briefly takes the matching exclusive lock, so it waits for every lower allocated
sequence to commit or roll back before reading the anchor. Do not bypass the
typed append query or change this lock ordering; either change requires a new
owner/security review and the blocked-precommit pagination regression.

Audit reads use the Secure, HttpOnly, SameSite=Strict
`__Host-guardian_session` cookie contract. Until P1-W2 supplies the session
authorizer, the operation fails closed with `401` before executing any audit
query. The collection has no product update/delete operation; unsupported
methods return `405`.

## Recovery and retention boundary

P1-W10 has no retention-delete workflow. Do not delete audit rows to repair a
development fixture. Development migrations are forward-only; if an
incompatible development database must be recovered, follow the explicit
named-project reset procedure in the Control Plane development runbook and
reseed development data. Never apply that reset procedure to pilot or
production data.
