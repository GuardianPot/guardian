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
- `DesiredStateSnapshot` and `ObservedState` are revision-only P1-W5 transport
  envelopes. P1-W6 owns their typed objects, persistence, reconciliation, and
  process-restart idempotency.
- Retriable state frames use UUIDv7 message IDs. A receiver acknowledges only
  after its configured domain handler accepts the frame. Exact duplicates in
  the active session are acknowledged without invoking the handler again.
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
