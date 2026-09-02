# P1-W9 platform health security review

- Review date: 2026-08-29
- Work package: P1-W9
- Decisions: EN-08, CP-06, OB-01, OB-02, SA-14, W9-C1-A through W9-C15-A
- Scope: Edge evidence, durable report/ACK, authenticated ingest, projection,
  environment aggregate, read API, and fixed read-only runtime probe

## Trust-boundary conclusion

Device identity is supplied only by the P1-W5 mTLS certificate verifier; a
health frame contains no device identity field. The Control Plane re-verifies
certificate eligibility before ingest, validates the complete 16 KiB report,
rate-limits per device, and applies it in a serializable active-device
transaction. A replaced stream cannot mark its replacement disconnected.

The owner read API has no health mutator. TLS and the P1-W2 server-side session
authorizer run before a projection read, errors are sanitized, responses are
`no-store`, and bounded messages are JSON text rather than markup. Pending,
disabled, and revoked inventory devices are excluded from environment
operational aggregation.

## Edge durability and measurement controls

- SQLite schema 3 stores exactly eight canonical conditions, a decimal uint64
  next sequence, one exact pending payload, and matching ACK metadata. Report
  state and conditions commit together before channel enqueue.
- An unacknowledged report is replayed rather than overwritten. Only the exact
  report ID and sequence clear it; conflicting ACKs preserve pending truth.
- Certificate, reconciliation, database read/write/quick-check, spool usage and
  filesystem free space, Linux `adjtimex`, helper, and runtime evidence use
  closed states/reasons. Raw database, syscall, gRPC, certificate, and runtime
  errors never enter a condition message.
- `spool_capacity_bytes` is required and bounded from 64 MiB through 1 TiB.
  Unavailable clock or filesystem measurements are `Unknown`, never healthy.

## Privileged runtime probe

The main Edge service has `/run/containerd` marked inaccessible and imports no
containerd package. The helper alone imports pinned
`github.com/containerd/containerd/api` v1.11.1 and calls only the Version RPC
at the compile-time `/run/containerd/containerd.sock` path. The new
`GetRuntimeStatus` request is empty. Its response is a bounded reachability enum
and stable reason; no caller-selected path, arbitrary request, command,
lifecycle operation, container identity, version string, or other runtime
metadata crosses the helper boundary.

The helper retains AF_UNIX-only/private-network sandboxing, an empty capability
set, peer-credential authentication, message/concurrency limits, method
allowlisting, audit, and deadline enforcement. P1-W9 grants no runtime mutation
authority.

## Threat review

| Threat | Control and evidence |
|---|---|
| False green before evidence | Complete unknown initial set; all eight conditions block green; stale receive time and explicit disconnect override. |
| Cross-device overwrite | mTLS identity binding, active-device row lock, monotonic sequence, exact duplicate and conflict handling. |
| Replacement-session race | Registry removal returns current ownership; only current-session closure calls the durable disconnect handler. |
| Crash between creation and send | SQLite report and conditions persist before enqueue; restart replays the stored canonical payload. |
| ACK spoof or reordering | ACK kind, report ID, and sequence must match the one pending row. |
| Storage or disk failure hidden | Read/write/quick-check and two independent spool thresholds; unavailable evidence is Unknown. |
| Runtime privilege expansion | Parameterless fixed-path Version-only helper method; main unit path denial; no lifecycle API or metadata response. |
| Hostile UI content | UTF-8/byte/secret-fragment validation, JSON encoding, no-store response; P1-W11 must render as text. |
| Aggregate nondeterminism | Active-only input, fixed False/Unknown/True precedence, canonical condition order, lowest-UUID tie break. |

## Evidence commands

```text
task health:contracts
task privileged:generated
task health:integration
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
task validate
```

The isolated integration fixture stops and restarts an exact containerd Version
service at the fixed Unix path, stops and restarts the helper client fixture,
reopens SQLite with an unacknowledged report, verifies mismatch/ACK behavior,
exercises PostgreSQL disconnect/recovery, repeats race-enabled tests, and scans
the health boundary for credential-shaped material.

## Hostile-message and CI evidence amendment — 2026-09-03

Owner-authorized closure of the two remaining P1-W9 evidence gaps. No decision,
contract, threshold, boundary, or acceptance criterion changed.

- **End-to-end hostile-message evidence.** The test-only publisher can now
  replace exactly one canonical condition, so a bounded, secret-free markup
  message travels the real mTLS device channel, Control Plane validation and
  persistence, the authorized read API, and the browser. Chromium, Firefox, and
  WebKit each assert the message appears as literal text inside the condition
  list, that the list contains no `img`, `svg`, `script`, `iframe`, `object`, or
  `embed` element, that the aggregate attributes the block to the spool
  condition with reason `capacity_critical`, and that no dialog was raised. A
  status change carries a fresh transition time, so the backend's monotonic
  transition-history validation stays unweakened.
- **API escaping evidence.** `TestHealthReadEncodesHostileMessagesAsInertJSONText`
  proves both read endpoints emit no raw `<img`, `<script`, `<svg`, or `">`
  sequence and that decoding the response returns the message byte-for-byte, so
  escaping is lossless rather than stripping or entity rewriting.
- **CI wiring.** `task health:integration` now runs in the quality workflow, so
  the durability, ACK/replay, disconnect, helper/runtime stop-restart, and
  race evidence produces a CI link as `§12` requires instead of local-only runs.
  Its committed credential-shaped scan moved from `rg` to the identical POSIX
  `grep -E` pattern so any CI image can run it, and a scanner that cannot
  complete is now a failure rather than a silent pass.

## Known limitations and residual risk

- Non-Linux builds report clock and filesystem measurements as unavailable;
  the supported Debian runtime exercises the Linux probes.
- When the privileged helper is absent, runtime reachability is Unknown because
  bypassing the helper would violate the privilege boundary.
- An accepted exact duplicate after reconnect remains idempotent and does not
  rewrite a prior disconnect override. ACK permits the next newer full report,
  which clears it; channel-state dirtiness schedules that report immediately.
- P1-W11 owns browser route composition and screenshot evidence; both are now
  merged, and the hostile-message browser evidence above closes the remaining
  UI gap. Backend acceptance still must not be presented as that UI evidence.
- The pinned `govulncheck` scan found no called vulnerabilities in either Go
  application. The unchanged Control Plane dependency graph retains one
  module-only warning, `GO-2026-5932`, for the unmaintained
  `golang.org/x/crypto/openpgp` package; Guardian neither imports nor calls that
  package. The Edge dependency graph, including the new containerd API module,
  reported no vulnerability.

Product Owner application/security acceptance remains mandatory before merge.
