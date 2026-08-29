# P1-W6 desired-state reconciliation security review

## Review scope

This review covers the versioned desired/observed contract, PostgreSQL desired
publication and observed projection, Edge SQLite schema 2, the unprivileged
reconciliation loop, real P1-W5 mTLS delivery, retry/restart behavior, and
blocking evidence authorized by W6-C1-A through W6-C8-A.

## Trust boundaries

- P1-W5 authenticates the Edge certificate and supplies the device identity;
  a frame cannot assert or override that identity.
- The Control Plane derives complete desired content only from the active
  device/environment and ordered zone records. PostgreSQL assigns immutable
  per-device revisions and UUIDv7 message identities under a device-row lock.
- Edge treats every remote snapshot as untrusted until its device binding,
  supported fields, UUIDs, canonical private IPv4 CIDRs, ordering, text, and
  cardinality bounds pass.
- Candidate acceptance, condition transition, retry state, last-known-good
  selection, and pending observed output are one SQLite transaction.
- P1-W6 has no privileged-helper reference, shell execution, artifact fetch,
  network/firewall mutation, production credential, or real decoy operation.

## Threat and containment review

| Threat | Control and evidence |
|---|---|
| Cross-device or forged desired state | TLS-authenticated server stream plus required snapshot device binding; mismatch is terminal and preserves last-known-good. |
| Mutable revision or equivocation | Immutable `(device_id, revision)` PostgreSQL key, unique message ID, deterministic digest, and terminal same-revision conflict. |
| Replay, reordering, or skipped delivery | Durable monotonic revision comparison; duplicate/stale no-op, complete higher revision accepted, stable observed replay until ACK. |
| Malformed or future object smuggling | Protobuf unknown-field rejection, JSON unknown-field rejection, explicit v1 object set, 1 MiB channel/storage bound, and cardinality/text/CIDR validation. |
| Destructive invalid update | Validation precedes apply; terminal failures cannot replace last-known-good payload or report convergence. |
| Retry amplification | One initial application plus at most five persisted retries on the fixed 1/2/4/8/16-second schedule, then terminal `retry_exhausted`. |
| Crash or partial state | SQLite transaction and write-barrier injection tests; restart reads the single durable row and resumes pending/retry output. |
| Silent loss between domain handlers | Reconciliation and P1-W9 health handlers/ACK handlers are separate; missing ownership returns `Unimplemented`. |
| Secret or payload disclosure | Desired schema has no secret fields; audit and diagnostics store bounded metadata, and safe snapshots omit payload/digest. Source/evidence secret scan is blocking. |
| Unauthorized side effect | Default applier validates and records metadata only; no privileged or attacker-facing operation exists in this package. |

## Persistence and audit

PostgreSQL migration 6 stores bounded JSON desired snapshots, SHA-256 content
digests, ACK timestamps, deduplicated observed message IDs, and the current
observed projection. Publication appends `desired_state.revision.published`
with device/environment identity, revision, digest, and object counts; it does
not include the snapshot payload. Edge diagnostics expose only redaction-safe
revision and condition summaries.

The Edge development schema update is forward-only. A schema-1 database fails
closed and uses the documented P1-W7 reset/reseed recovery path; P1-W6 does not
silently migrate, discard, or reinterpret existing local state.

## Dependency and vulnerability review

- P1-W6 changes no Go module, npm manifest, lockfile, container base, or runtime
  dependency. Existing dependency and license policy therefore remains
  unchanged; `npm run dependencies:check` passed.
- `govulncheck v1.7.0` against both complete runtime module trees reported zero
  symbol- and imported-package-level vulnerabilities. The Control Plane module
  graph still contains `GO-2026-5932` for the unmaintained
  `golang.org/x/crypto/openpgp` package, but Guardian does not import or call
  that package and no fixed module version exists. This known non-callable
  upstream finding is retained rather than described as zero module findings.
- Protobuf and sqlc outputs reproduced byte-for-byte from the pinned repository
  tools. P1-W6 introduces no custom cryptography; SHA-256 is used only for
  deterministic content identity and integrity comparison.

## Evidence commands

```text
task privileged:generated
task reconciliation:integration
task validate
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
```

Final CI links, commit SHA, test results, dependency/license evidence, and any
remaining platform limitation are recorded in the PR and issue. Product Owner
application/security acceptance remains mandatory before merge.

## Known limitations and residual risk

- A connected Edge receives environment changes on its next reconnect rather
  than by an in-session publication trigger. The content remains durable and
  converges after reconnect; continuous push is deferred.
- Placeholder-decoy metadata proves the contract and loop only. It deliberately
  cannot prove real decoy containment or lifecycle behavior.
- SHA-256 detects content equality/corruption; authenticity comes from the
  mutually authenticated TLS channel and repository-controlled producer, not
  from a payload signature.
