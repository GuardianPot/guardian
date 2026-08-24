# P1-W9 health contract and domain shell

This first P1-W9 delivery defines health truth without pretending that the
later runtime seams exist. It contains no fake backend, persistence adapter,
device-channel transport, route composition, or mock-green UI path.

## Canonical condition order

Every report contains exactly one of each condition in this order:

1. `edge_connected`
2. `device_certificate_ready`
3. `config_converged`
4. `local_database_healthy`
5. `spool_healthy`
6. `clock_quality`
7. `container_runtime_reachable`
8. `privileged_helper_reachable`

`False` takes aggregate precedence over `Unknown`; `Unknown` takes precedence
over `True`. All eight conditions block a green aggregate. Before fresh Control
Plane evidence exists, connectivity is `Unknown/heartbeat_stale`; the other
conditions are `Unknown/not_observed`.

## Transition and report rules

- A status or reason change advances `last_transition_time`.
- A message or observed-revision refresh preserves transition time.
- Reports are full snapshots, not deltas, and are bounded to 16 KiB.
- Report IDs are canonical lowercase UUIDv7 values. Sequence starts at one and
  increases durably per device.
- Exact retries are idempotent. A conflicting same sequence or an older
  sequence is rejected without replacing accepted truth.
- Control Plane receive time drives the 90-second stale transition. Edge clock
  time never keeps a stale connection green.
- Health message text is bounded, screened for secret-like values, and must be
  rendered as plain escaped text.

## Phase 1 thresholds

| Condition | Healthy | False | Unknown |
|---|---|---|---|
| Certificate | More than 10 days remain and no failure evidence | Final 10-day window, expired, revoked, or rotation failed | Clock unreliable or not observed |
| Clock | Synchronized and absolute offset is at most 5 seconds | Unsynchronized or offset exceeds 5 seconds | Measurement unavailable or not observed |
| Spool | Below 80% configured use and above 10% filesystem free | Warning at 80% or 10% free; critical at 95% or 5% free | Measurement unavailable or not observed |
| Local database | Read, write, and integrity evidence succeed | One of those checks fails | Not observed |
| Runtime/helper | Typed probe succeeds | One fresh failed probe; timeout is 2 seconds | Not observed |

One fresh successful probe recovers only its own condition.

## Read API security boundary

The contract exposes only:

- `GET /v1/environments/{environmentId}/health`
- `GET /v1/devices/{deviceId}/health`

Both operations require the Guardian cookie-session security scheme and remain
default-denied until P1-W2 supplies authorization. Environment scope is checked
before a projection read. Successful responses require
`Cache-Control: no-store`. No health mutation endpoint exists.

## Validation

Run the focused contract and domain gate:

```text
task health:contracts
```

The full repository acceptance gate remains `task validate`. P1-W9 final
acceptance additionally waits for real P1-W4, P1-W5, P1-W6, and P1-W11
integration and its approved failure-injection/browser evidence.
