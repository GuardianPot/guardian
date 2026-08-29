# Desired-state reconciliation development runbook

## Scope and expected state

P1-W6 publishes one complete metadata-only snapshot when an authenticated Edge
connects or reconnects. PostgreSQL owns immutable desired revisions and the
latest observed projection. Edge SQLite schema version 2 owns the durable
candidate, last-known-good snapshot, bounded retry state, and pending observed
message. No P1-W6 path executes a command, mutates networking, downloads an
artifact, or starts/stops a decoy.

The redaction-safe Edge diagnostic snapshot may expose only revision numbers,
condition/reason codes, attempt count, retry time, and whether observed output
is pending. It must not expose the desired or last-known-good payload or digest.

## Validation

Run the blocking package evidence from the repository root:

```bash
task reconciliation:integration
```

The task starts disposable PostgreSQL 18, exercises migration 6 and immutable
publication, drives a real TLS 1.3 mTLS Control Plane-to-Edge convergence,
verifies acknowledgement durability, and repeats the reconciliation packages
under the race detector. It also parses the committed JSON schema and scans the
changed source/evidence paths for credential-shaped material.

The complete repository gate remains:

```bash
task validate
```

## Safe diagnosis

Use the Edge health snapshot and these bounded fields: desired revision,
observed revision, last-good revision, condition status, reason code, attempt
count, retry time, and observed-pending flag. On the Control Plane, use revision,
message UUID, acknowledgement timestamp, condition status, and audit event
metadata. Never print snapshot payloads, database URLs, private keys,
certificates, enrollment/session tokens, or unrestricted gRPC metadata.

Common reason codes are:

| Reason | Meaning and action |
|---|---|
| `applied` | The metadata snapshot converged and became last-known-good. |
| `identity_mismatch` | Stop; verify device/environment assignment and certificate identity. |
| `invalid_snapshot` | Stop; validate identifiers, private canonical CIDRs, ordering, and bounds. |
| `unsupported_object` | Stop; deploy atomically compatible Control Plane and Edge consumers. |
| `revision_conflict` | Stop publication and investigate immutable-revision corruption or a non-canonical producer. |
| `apply_failed` | Wait for the persisted bounded retry schedule; inspect only coded local diagnostics. |
| `retry_exhausted` | The sixth application attempt failed after all five scheduled retries; resolve the local cause and publish a later complete revision. |

## Recovery and failure injection

- Restart the Edge while `observed_pending` is true. The same observed message
  must be republished and cleared only by its matching ACK.
- Restart during `retrying`. Attempts and `retry_at` must survive; a retry must
  not run before its persisted time.
- Disconnect longer than the retry window. Local reconciliation may continue,
  but last-known-good state must remain and no offline teardown is allowed.
- Replay the same desired frame. Apply count must not increase and the observed
  identity/output must remain stable.
- Inject a write-barrier failure before candidate commit. Reopen SQLite and
  verify that no partial candidate, retry, or revision projection exists.
- Corrupt or replace SQLite, or present schema version 1. Startup must fail
  closed through the P1-W7 storage/recovery boundary.

This development repository intentionally has no in-place schema-1-to-schema-2
migration. After preserving any approved non-secret diagnostic evidence, stop
the Edge and use the P1-W7 explicit recovery command:

```bash
guardian-edge recover-db \
  --config /etc/guardian-edge/config.json \
  --confirm-reset-development-data
```

The command quarantines the exact database and WAL/SHM sidecars before creating
schema 2; it does not silently delete the content-addressed spool. Do not apply
ad hoc SQL repair or use this development reset for production-like data.

## Known Phase 1 limits

The Control Plane publishes the current complete snapshot on connection or
reconnect; it does not yet push an environment edit into an already-connected
session. Reconnect is the supported development trigger. Placeholder-decoy
objects are metadata only. Operator UI, real decoy lifecycle, privileged
networking, and multi-Edge orchestration remain outside P1-W6.
