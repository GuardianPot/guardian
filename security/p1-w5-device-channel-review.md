# P1-W5 device channel security review

## Review scope

This review covers the dedicated Control Plane gRPC listener, outbound Edge
client, `guardian.device.v1` channel contract, lifecycle wiring, resource
bounds, deployment profile, and blocking tests authorized by W5-C1-A through
W5-C7-A.

## Trust boundaries

- Device authentication is TLS 1.3 mTLS with the P1-W4 product device CA.
- The verified certificate URI SAN is the sole device identity source.
- P1-W4 durable active-serial/device-state checks run before frames, on a
  30-second active timer, and before state-bearing ingest.
- Server authentication uses hostname verification and the Edge host trust
  store. No plaintext, insecure-skip-verify, bearer, WebSocket, polling, or
  Control-Plane-initiated Edge connection fallback exists.
- P1-W6/P1-W9 handlers are explicit seams. Missing handlers fail without
  acknowledgement and cannot manufacture durable or healthy state.

## Abuse and containment review

| Threat | Control and evidence |
|---|---|
| Missing, foreign, expired, disabled, or revoked certificate | TLS chain validation plus durable verifier; unit and PostgreSQL-backed active-revocation tests. |
| Identity mismatch or client-asserted identity | No identity field exists in channel frames; P1-W4 validates the single device URI SAN. |
| Duplicate/reconnect race | One active registry entry per device; a newer authenticated session cancels the old generation. |
| Malformed or oversized Protobuf | Typed generated codec and 1 MiB peer limits; invalid frames return stable gRPC errors. |
| Metadata or trace abuse | 16 KiB header budget and OpenTelemetry W3C trace-context handler; payload/credential attributes are absent. |
| Memory growth and slow consumer | 64-frame plus 4 MiB queues, non-blocking enqueue, explicit `ResourceExhausted`, and race tests. |
| Health flood | 16 KiB report validation and persistent-across-reconnect per-device token bucket, six/minute burst three. |
| Stale or half-open connection | 30-second application heartbeat, 90-second server receive-time stale threshold, bounded gRPC cancellation. |
| Reconnect storm | Cryptographic full-jitter exponential delay from one to 60 seconds, reset only after 90 seconds stable. |
| Credential leakage | Generic wire errors and logs; source/evidence secret-shaped scan; no key/certificate body logging. |

## Dependency and cryptography review

- Standard Go TLS/X.509 and maintained gRPC/Protobuf libraries are used; there
  is no custom cryptography.
- Runtime dependencies are pinned in both Go modules. Implementation resolved
  gRPC `v1.83.2`, Protobuf `v1.36.12`, OpenTelemetry `v1.46.0`, and otelgrpc
  `v0.71.0`. `go list -m -u` reported no newer versions for these four modules.
- `govulncheck v1.7.0` reported no symbol- or imported-package-level
  vulnerabilities in either runtime. The Control Plane module graph retains
  `GO-2026-5932` against the unmaintained `golang.org/x/crypto/openpgp`
  package, but Guardian does not import that package and there is no fixed
  module version; this non-callable upstream advisory remains recorded rather
  than being represented as zero module-level findings.
- Module license files identify gRPC, OpenTelemetry, and otelgrpc as Apache-2.0
  and Protobuf as BSD-3-Clause; all are allowed dependency classes.
- Generated code comes only from pinned Go tool directives through Buf; no
  unpinned generator binary from `PATH` is used.

## Evidence commands

```text
buf lint
task privileged:generated
task device-channel:integration
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
task validate
```

Final CI links, commit SHA, test counts, and any remaining platform limitation
are recorded in the PR and issue evidence. Product Owner/CODEOWNER application
and security acceptance remains mandatory before merge.
