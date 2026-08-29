# Device channel v1

P1-W5 defines one authenticated bidirectional gRPC stream:

```text
guardian.device.v1.DeviceChannelService.Connect
Edge ConnectRequest stream <-> Control Plane ConnectResponse stream
```

The wire schema is authoritative in
`proto/guardian/device/v1/channel.proto`. Device identity never appears in a
frame; the Control Plane derives it exclusively from the verified product
device certificate URI SAN.

## Negotiation

The first Edge frame must be `EdgeHello`. The first Control Plane frame is
`ProtocolSelection`. Phase 1 selects major `1`, minor `0`. A different major or
a client minor newer than the server receives an unaccepted selection with
reason `protocol_incompatible`, followed by gRPC `FailedPrecondition`. The Edge
records this as degraded channel health and retries with bounded backoff.

During development there is no compatibility adapter. A breaking update needs
Product Owner review and an atomic update of every repository consumer. Minor
support-window behavior is added when a second minor generation exists.

## Frames and delivery

- `Heartbeat` is emitted every 30 seconds. The Control Plane uses its receive
  time and closes a stream after 90 seconds without a valid heartbeat.
- `DesiredStateSnapshot` is a complete P1-W6 device-scoped snapshot. It carries
  the authenticated device/environment binding, zone metadata ordered by zone
  UUID, and at most 64 non-operational placeholder-decoy records. It never
  carries credentials, artifacts, commands, firewall rules, or real decoy
  lifecycle instructions.
- The Control Plane assigns an immutable per-device monotonic revision and a
  UUIDv7 message ID. A deterministic content digest excludes that transport
  envelope, so unchanged content reuses the current revision while a later
  complete snapshot may intentionally return to earlier content as a new
  revision.
- `ObservedState` reports desired, applied, and last-known-good revisions plus
  one `pending`, `converged`, `retrying`, or `failed` condition. Retry output
  includes a bounded attempt count and next retry time; other conditions must
  not include a retry time.
- Retriable state frames use UUIDv7 message IDs. A receiver acknowledges only
  after its configured domain handler accepts the frame. Exact duplicates in
  the active session are acknowledged without invoking the handler again.
- Desired and observed acknowledgements are independent. The Edge retains the
  same pending observed message across reconnects until its matching
  acknowledgement arrives. P1-W6 reconciliation and P1-W9 health use separate
  handlers; an unavailable handler returns `Unimplemented` and does not ACK.
- `HealthReport` retains the P1-W9 full-snapshot contract. P1-W9 owns durable
  projection and aggregate truth; P1-W5 enforces transport bounds and quota.

## Security and resource limits

- TLS 1.3 minimum; per-device X.509 client authentication; server-name
  verification against the Edge operating-system trust store.
- P1-W4 active certificate serial and durable device state are checked before
  negotiation, every 30 seconds, and before state-bearing frames.
- One active stream per device. A newer fully authenticated stream atomically
  cancels and replaces an older stream.
- 1 MiB maximum gRPC message, 16 KiB metadata/header budget, and per-direction
  queue limits of 64 frames and 4 MiB.
- Health reports are at most 16 KiB and use a per-device token bucket of six
  reports per minute with burst three.
- Queue saturation returns explicit backpressure. Credentials, certificate
  bodies, payloads, and arbitrary metadata are never logged or attached to
  traces. Only W3C trace context propagates.
- Desired content is rejected before apply when the authenticated device
  binding differs, the object type is unsupported, an identifier/CIDR/text
  bound fails, or canonical ordering is absent. Validation has no privileged
  or attacker-facing side effect and failure preserves last-known-good state.

## P1-W6 delivery and reconciliation

The current complete snapshot is generated and queued after every successful
connection or reconnect. Delivery is at least once; correctness does not rely
on connection-local duplicate memory.

| Input | Durable Edge result |
|---|---|
| Same revision and digest | No reapply; republish the stable observed result. |
| Same revision, different digest | Terminal `revision_conflict`; preserve candidate and last-known-good payloads. |
| Older or out-of-order revision | Do not replace current truth; republish current observed result. |
| Higher/skipped revision | Accept the complete snapshot atomically and reconcile it. |
| Malformed, unsupported, or identity-mismatched snapshot | Terminal bounded reason; do not apply and do not erase last-known-good state. |
| Retryable apply failure | Persist attempts and retry after 1, 2, 4, 8, then 16 seconds; a sixth application failure ends as `retry_exhausted`. |

The Edge commits candidate, condition, retry metadata, last-known-good payload,
and pending observed output in SQLite before reporting it. Restart resumes that
durable row. Disconnect causes no destructive action. This Phase 1 skeleton
applies metadata only; it cannot invoke the privileged helper or manage a real
decoy.

## gRPC failure contract

| Code | Meaning |
|---|---|
| `Unauthenticated` | Certificate is missing, invalid, expired, disabled, revoked, or no longer eligible. |
| `InvalidArgument` | Hello order, timestamp, state, acknowledgement, health report, or frame shape is invalid. |
| `FailedPrecondition` | Protocol generation is incompatible. |
| `ResourceExhausted` | Message, queue, or health-report rate bound is exceeded. |
| `DeadlineExceeded` | Hello or heartbeat deadline expired. |
| `Aborted` | A newer authenticated stream replaced this device session. |
| `Unimplemented` | A later-package domain handler is not installed; the frame is not acknowledged. |

Errors remain generic at the wire boundary. They do not disclose database,
certificate, key, token, filesystem, or internal handler details.
