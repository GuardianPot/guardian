# Edge Agent development runbook

This runbook covers the P1-W7 unprivileged daemon foundation. Enrollment,
device-channel traffic, desired-state reconciliation, privileged mutations, and
full platform health are implemented by later work packages.

## Approved boundaries

- Decisions: EN-01, EN-02, EN-03, EN-05, EN-06, EN-08, EN-12, SA-05, and
  OB-01.
- Acceptance: Phase 1 P1-W7 acceptance.
- Main process: native Go 1.27.0 systemd service under the dedicated
  `guardian-edge` no-login/no-home account.
- Local metadata: SQLite WAL with synchronous `FULL` and a bounded busy timeout.
- Payloads: content-addressed files under the protected spool; SQLite contains
  digests, sizes, state, leases, and retry metadata, not large payload bodies.
- Privileged work: no root requirement, capability, shell API, container-runtime
  socket, or network mutation exists in P1-W7.

## Local build and test

From the repository root in Linux/WSL:

```bash
GOWORK=off go -C apps/edge-agent test ./...
bash tests/integration/edge-agent/run.sh
```

The integration fixture builds a disposable binary and identity, then validates
missing configuration, secure-identity refusal, startup, JSON diagnostics,
SIGTERM shutdown, restart, WAL replay, corruption, explicit recovery,
permissions, service directives, and secret redaction. It only uses a temporary
directory and removes that exact directory on exit.

## Configuration and permissions

Use `deploy/edge-agent/config.example.json` as the non-secret configuration
shape. Unknown fields and files over 64 KiB are rejected. The configuration must
be an absolute, regular, non-symlink file and must not be group/world writable.

Reference permissions:

| Path | Owner | Mode |
|---|---|---:|
| `/etc/guardian-edge` | `root:guardian-edge` | `0750` |
| `/etc/guardian-edge/config.json` | `root:guardian-edge` | `0640` |
| `/var/lib/guardian-edge` | `guardian-edge:guardian-edge` | `0700` |
| Device certificate | `guardian-edge:guardian-edge` | `0644` or stricter |
| Device private key | `guardian-edge:guardian-edge` | `0600` |
| SQLite DB/WAL/SHM | `guardian-edge:guardian-edge` | `0600` |
| Spool directories / objects | `guardian-edge:guardian-edge` | `0700` / `0600` |

The daemon validates a matching, currently valid certificate/key pair before it
creates or opens local state. Missing, expired, mismatched, symlinked, or broadly
readable key material yields a named startup failure. It never attempts
plaintext, unauthenticated, or implicit enrollment.

## Commands

```text
guardian-edge serve --config /etc/guardian-edge/config.json
guardian-edge status --config /etc/guardian-edge/config.json
guardian-edge status --config /etc/guardian-edge/config.json --format json
guardian-edge diagnostics --config /etc/guardian-edge/config.json
guardian-edge diagnostics --config /etc/guardian-edge/config.json --format text
guardian-edge version
```

`status` and `diagnostics` open the database read-only. They never create,
migrate, repair, or reset it. Output is capped at 64 KiB and database result sets
at 64 records per category. Paths, key bytes, enrollment tokens, raw remote
errors, and payload bodies are omitted. A representative JSON report is in
`docs/runbooks/edge-agent/diagnostics-sample.json`.

## Lifecycle and persisted conditions

Startup order is explicit:

1. enrollment/validated identity boundary;
2. telemetry spool;
3. device channel;
4. reconciler;
5. privileged-helper client;
6. health reporter.

Shutdown uses the reverse order under the configured timeout. P1-W7 reports
future channel, reconciler, and helper capabilities as `degraded` with reason
`not-implemented`; it does not pretend those later-package features work.
Process and component states are persisted as bounded reason codes.

Desired, observed, and last-known-good revisions; retry attempts and next time;
health conditions; identity certificate metadata; spool metadata; and event
leases survive restart. Expired leases replay with a monotonically increasing
attempt number, and a stale worker cannot acknowledge a newer lease.
Revocation is sticky for the same certificate fingerprint across restart and is
reported as degraded; only a genuinely new certificate establishes a new local
identity record. P1-W7 opens no device channel, so it cannot bypass that state.

## Corruption and explicit development recovery

A failed SQLite `quick_check`, `SQLITE_CORRUPT`, or `SQLITE_NOTADB` result enters
the named `edge database is corrupt` state. An unknown/unversioned development
schema with tables enters `edge database schema is incompatible`. Normal open,
status, and diagnostics do not modify or reset either condition.

Development recovery is intentionally forward-only and discards active local
metadata after quarantining the exact DB and any WAL/SHM sidecars:

```bash
guardian-edge recover-db \
  --config /etc/guardian-edge/config.json \
  --confirm-reset-development-data
```

The command refuses to run without the confirmation flag and refuses to reset a
healthy DB. Quarantined files use the suffix
`.corrupt-YYYYMMDDTHHMMSS.NNNNNNNNNZ`; a fresh current-schema DB is then
initialized. The content-addressed spool is not silently deleted. This is the
approved development recovery path, not a production data-preservation layer.

## Named operational failures

| Condition | Meaning | Operator action |
|---|---|---|
| `secure device identity is unavailable` | Identity files are absent | Provision through the approved enrollment flow; do not bypass TLS. |
| `device identity permissions are unsafe` | Key/cert path or mode violates the protected-file baseline | Correct ownership/mode and retry. |
| `edge database is busy` | Lock contention exceeded the bounded timeout | Inspect competing processes; do not run two writers. |
| `edge storage is full` | SQLite or filesystem reported no capacity | Free capacity and inspect spool/health thresholds. |
| `edge spool object is unavailable` | CAS content is missing, symlinked, wrong-sized, or hash-invalid | Preserve evidence and investigate filesystem integrity. |
| `edge database is corrupt` | SQLite integrity failed | Capture diagnostics, stop the daemon, then use explicit development recovery if authorized. |

## systemd review

Install the files described in `deploy/edge-agent/README.md`, then run:

```bash
systemd-analyze verify /etc/systemd/system/guardian-edge.service
systemd-analyze security guardian-edge.service
systemctl show guardian-edge.service \
  -p User -p Group -p NoNewPrivileges -p CapabilityBoundingSet
```

The reference unit intentionally retains outbound IPv4/IPv6 and Unix-socket
families for the future mTLS channel and typed local helper. It grants no
capability and no writable path outside systemd-managed Edge state/runtime
directories.

## Known limitations

- No real enrollment, device channel, reconciler, privileged helper, update
  system, decoy lifecycle, or production packaging matrix is included.
- OpenTelemetry spans use the vendor-neutral API with the default no-op provider
  until the later observability package supplies SDK/export configuration.
- Spool capacity policy and garbage collection are later-package work; P1-W7
  limits each normalized event payload to 1 MiB and exposes aggregate size.
- P1-W7 accepts only the exact current development schema. Older development DBs
  require the explicit quarantine/reset command; no compatibility layer exists.
