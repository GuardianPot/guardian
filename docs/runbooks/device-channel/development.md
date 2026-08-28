# Device channel development runbook

## Configuration

The Control Plane listens on a dedicated non-privileged address configured by
`GUARDIAN_DEVICE_CHANNEL_ADDRESS` (default `127.0.0.1:8443`). It reuses the
configured server certificate/key and the product device CA initialized by
P1-W4. Production ingress maps this listener to a dedicated TCP/443-compatible
endpoint; the development listener must not run as root merely to bind port 443.

The Edge configuration uses separate endpoints:

```json
{
  "control_plane_endpoint": "guardian.example:443",
  "device_channel_endpoint": "devices.guardian.example:443"
}
```

The first endpoint owns HTTPS enrollment/rotation. The second is the outbound
gRPC/mTLS channel. The channel server certificate must match its endpoint host
and chain to the Edge operating-system trust store. The product device private
key remains an owner-only, regular, non-symlink file.

## Validation

Run the blocking package evidence:

```bash
task device-channel:integration
```

This starts disposable PostgreSQL 18, uses the real P1-W4 enrollment and durable
certificate verifier, negotiates a real TLS 1.3 gRPC stream, revokes the active
device, and verifies immediate fail-closed termination. It also repeats Control
Plane and Edge channel tests under the race detector.

The complete repository gate remains:

```bash
task validate
```

## Failure injection

- Stop and restart the Control Plane listener. Edge must enter bounded
  reconnect backoff and negotiate a new single active stream when it returns.
- Block the channel port for longer than 90 seconds. Control Plane must mark the
  heartbeat stale and close the stream; recovery needs fresh negotiation.
- Start two clients with the same protected identity. The newer authenticated
  stream must replace the older one with `Aborted`.
- Revoke or disable the device while connected. The next eligibility check must
  close the stream with `Unauthenticated`; retry must not re-establish it.
- Send an incompatible major, malformed frame, oversized frame, or a fourth
  immediate health report. Each must receive its documented bounded failure
  without either process crashing.

## Safe diagnostics

Allowed evidence includes listener address, TLS version, generic gRPC status,
device UUID, reconnect reason code, and bounded queue counters. Do not capture
private keys, certificate bodies, enrollment/session tokens, database URLs,
payloads, or unrestricted gRPC metadata. If packet evidence is required, record
only listener/profile metadata and encrypted-flow properties.

The Edge local `device-channel` component is truthful but not the final P1-W9
projection. P1-W6, P1-W9, and P1-W11 retain their durable state, health, and UI
acceptance gates.
