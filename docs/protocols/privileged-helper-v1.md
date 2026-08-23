# Privileged helper v1 protocol

## Boundary

`guardian.privileged.v1.PrivilegedHelperService` is a local-only gRPC service
over `/run/guardian-edge-privd/guardian-edge-privd.sock`. The socket directory
is `root:guardian-edge` `0750`; the socket is `root:guardian-edge` `0660`.
There is no TCP listener, reflection service, plaintext fallback, shell method,
filesystem API, raw nftables input, executable argument, or runtime-socket
delegation.

The server accepts a connection only when Linux `SO_PEERCRED` returns a live
PID greater than one and the exact dedicated `guardian-edge` UID and primary
GID. The PID is pinned with `pidfd_open`; all real/effective/saved/filesystem
UID and GID slots in `/proc/<pid>/status` must match the kernel credentials.
Authentication failure occurs before protobuf dispatch and is audited.

## Methods

| RPC | Typed input | P1 production result |
|---|---|---|
| `GetStatus` | empty request | API version plus four capability states |
| `EnsureAddress` | request ID, exact interface, bounded IP prefix, present/absent | `UNSUPPORTED` after validation |
| `ApplyNftablesPolicy` | request ID, exact namespace, `DEFAULT_DENY_EGRESS` profile | `UNSUPPORTED` after validation |
| `ReconcileContainer` | request ID, exact workload ID, running/stopped/absent | `UNSUPPORTED` after validation |
| `EnsureNetworkNamespace` | request ID, exact namespace, present/absent | `UNSUPPORTED` after validation |

Root-controlled startup allowlists are exact. Empty allowlists deny all
mutating request arguments. Interface names are bounded to Linux's 15-byte
name limit and a conservative character set. Namespace/workload IDs require a
`guardian-` prefix. Address requests must be contained by a canonical
allowlisted prefix and cannot be unspecified, loopback, multicast, or
link-local. IPv4-mapped IPv6 forms are rejected so one address has only one
accepted representation. Each startup allowlist is limited to 256 entries.

## Failure and retry semantics

- Maximum received/sent protobuf size: 16 KiB.
- Maximum simultaneously accepted client connections: 16.
- Maximum concurrent RPC streams per connection: 32.
- Server operation deadline: five seconds or the caller's earlier deadline.
- Edge client probe/call deadline: two seconds; reachability probe every five
  seconds.
- A request ID is required for every mutating RPC. Identical concurrent or
  repeated requests execute once and reuse the result. Reusing an ID with a
  different deterministic request fingerprint fails with `AlreadyExists`.
- The cache is process-local, bounded to 1,024 results, and expires entries
  after 15 minutes. Later adapters must remain convergent and idempotent across
  helper restarts; the cache is not a durable authority.
- Helper loss changes Edge health to `degraded` with a bounded reason such as
  `socket-missing`, `socket-verification-failed`, `rpc-timeout`, or
  `rpc-unavailable`. It does not crash the Edge daemon.

## Audit contract

Every authenticated, rejected, unknown, malformed, oversized, cancelled, and
completed RPC produces structured metadata. Events include method, request ID,
SHA-256 request fingerprint, outcome, stable reason code, and peer PID/UID/GID.
Payloads, addresses, policy contents, secrets, and credential material are not
logged. Only syntactically valid bounded RPC methods and request IDs are
recorded verbatim; invalid values are omitted or normalized to `unknown` and
remain represented only by the fingerprint.

The canonical schema is
`proto/guardian/privileged/v1/privileged.proto`; committed Go output is
regenerated through the pinned Go tools in `buf.gen.yaml`.
