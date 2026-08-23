# P1-W8 privileged-helper security review

- Review date: 2026-08-23
- Work package: P1-W8
- Decisions: EN-02, EN-03, EN-04, SA-10, SEC-04, CP-0002, F2-A
- Scope: typed local privilege boundary; Phase 2 host adapters remain deferred

## Trust boundary conclusion

The unprivileged Edge daemon receives no root identity, Linux capability,
container runtime socket, command execution primitive, or raw ruleset API. The
root-owned helper has one fixed AF_UNIX endpoint. Filesystem permissions block
decoy identities before connect; `SO_PEERCRED`, pidfd liveness, and complete
UID/GID-slot checks authenticate the dedicated Edge identity before dispatch.

The RPC schema has four typed mutating operations and a status probe. Request
values pass strict lexical, enum, size, exact-name, and network-prefix
allowlists. Unknown methods/fields, malformed semantics, oversized payloads,
request-ID conflicts, command-injection strings, and traversal strings fail
closed. Startup allowlists, active connections, concurrent streams, message
sizes, operation durations, and idempotency state all have explicit bounds.
Audit records are metadata-only; untrusted method and request-ID values must
pass bounded lexical validation before they can appear verbatim.

## P1 capability conclusion

The production adapter reports every host operation as unsupported. The
systemd unit has an empty capability and ambient-capability set, private
networking, AF_UNIX-only sockets, read-only system protection, device denial,
namespace restriction, and the `@system-service` syscall filter. Offline
`systemd-analyze security` reports `1.1 OK`. P1 therefore establishes and tests
the privilege boundary without performing network, nftables, namespace, or
container mutation.

## Abuse and failure evidence

- Authorized real UID/GID over a Unix socket: pass.
- Wrong UID and wrong primary GID: rejected before RPC dispatch and audited.
- Invalid PID plus synthetic UID/GID/PID policy cases: rejected.
- Production-mode socket permissions deny a decoy UID/GID at connect: pass in
  a network-disabled root container.
- Shell/child-process primitive scan and forbidden RPC field scan: pass.
- Interface command injection, namespace traversal, unknown protobuf field,
  unknown RPC, oversized message, and non-allowlisted resource: rejected.
- Concurrent identical retry executes the adapter once; conflicting reuse is
  rejected.
- Cancellation and bounded deadline are propagated and audited.
- Helper stop/restart changes Edge health between precise degraded and healthy
  states without a daemon component-start failure.
- Generated-code freshness, Go vet/unit tests, race tests, module verification,
  vulnerability analysis, and full repository CI are acceptance gates.

Local validation on 2026-08-23 passed `task validate`,
`task privileged:security`, the full Edge race suite, `govulncheck v1.7.0`,
and `go-licenses v2.0.1`. The hardened unit scored `1.1 OK` in offline
`systemd-analyze security`. Two clean static helper builds were byte-identical
at SHA-256
`5ad5e492e91fa4097a847754ab59dcdda2aed52fdc777f03afcd8cc43eed8b05`.

## Residual risks and later gates

- gRPC bytes are not cryptographically encrypted because transport is a local
  kernel Unix socket. Authentication and access control rely on Linux peer
  credentials, a dedicated UID/GID, and root-owned filesystem metadata.
- A process already executing as the dedicated `guardian-edge` identity is in
  the trusted client principal. Request allowlists and typed methods limit the
  effect of compromise; this package does not claim per-thread or per-binary
  attestation.
- Idempotency cache state is intentionally not durable. Later adapters must
  reconcile desired state safely after helper restart.
- Phase 2 must independently review each Linux/runtime adapter and any proposed
  capability addition. `CAP_SYS_ADMIN`, broad root APIs, shell execution, raw
  nftables scripts, and runtime-socket delegation remain prohibited.

No unresolved P1-W8 security-boundary decision remains in this implementation.
