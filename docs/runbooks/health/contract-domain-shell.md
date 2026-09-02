# P1-W9 platform health operations

P1-W9 carries measured Edge truth through the authenticated device channel,
durable PostgreSQL projection, and owner-authorized read API. There is no fake
backend or mock-green production path.

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
- Edge SQLite schema 3 stores all eight conditions, the next sequence, and one
  exact pending report. It persists before enqueue, replays the pending report
  after restart, and clears it only for the matching report ID and sequence.
- PostgreSQL migration 7 applies ordering and transition validation inside a
  serializable device transaction. Only a closure from the current channel
  session sets `False/channel_disconnected`; replacement-session cleanup is a
  no-op.

## Phase 1 thresholds

| Condition | Healthy | False | Unknown |
|---|---|---|---|
| Certificate | More than 10 days remain and no failure evidence | Final 10-day window, expired, revoked, or rotation failed | Clock unreliable or not observed |
| Clock | Synchronized and absolute offset is at most 5 seconds | Unsynchronized or offset exceeds 5 seconds | Measurement unavailable or not observed |
| Spool | Below 80% configured use and above 10% filesystem free | Warning at 80% or 10% free; critical at 95% or 5% free | Measurement unavailable or not observed |
| Local database | Read, write, and integrity evidence succeed | One of those checks fails | Not observed |
| Runtime/helper | Typed probe succeeds | One fresh failed probe; timeout is 2 seconds | Not observed |

One fresh successful probe recovers only its own condition.

`spool_capacity_bytes` is required and must be between 64 MiB and 1 TiB. The
reference profile uses 1 GiB. Capacity is evaluated against both durable spool
bytes and filesystem free space.

## Runtime privilege boundary

The unprivileged `guardian-edge` unit cannot see `/run/containerd`. Only
`guardian-edge-privd` performs the parameterless `GetRuntimeStatus` operation.
The helper calls containerd's Version RPC at the fixed
`/run/containerd/containerd.sock` path with a two-second deadline and returns
only `reachable`, `probe-failed`, or `probe-timeout`. It accepts no caller path,
command, runtime object, or lifecycle request and returns no runtime metadata.

## Read API security boundary

The contract exposes only:

- `GET /v1/environments/{environmentId}/health`
- `GET /v1/devices/{deviceId}/health`

Both operations require the Guardian cookie-session security scheme and remain
default-denied without the P1-W2 owner-session authorizer. Environment scope is
checked before a projection read. Successful responses require
`Cache-Control: no-store`. No health mutation endpoint exists.

Environment health includes only active devices. Severity is `False`, then
`Unknown`, then `True`; equal-severity attribution uses the lowest device UUID.
Pending, disabled, and revoked devices remain inventory records but cannot make
the operational aggregate healthier or less healthy. Zero active devices is
explicitly `Unknown/no_active_devices`.

## Failure injection and recovery

Run only on an authorized development host:

1. Stop `guardian-edge-privd`. The next publishable snapshot must show
   `privileged_helper_reachable=False`; runtime becomes `Unknown` because the
   trusted probe boundary is unavailable. Restart the helper and require one
   fresh success before helper health recovers.
2. With the helper running, stop containerd. The helper remains reachable and
   `container_runtime_reachable=False/probe_failed` (or `probe_timeout` at the
   deadline). Restart containerd and require a fresh Version RPC success.
3. Interrupt the device channel. The Control Plane must mark only the current
   session `False/channel_disconnected`. Reconnect and accept a newer full
   report to clear it.
4. Leave a report unacknowledged, stop Edge, and restart it. Verify the report
   ID, sequence, and canonical payload are identical before ACK. A mismatched
   ACK must not clear it.
5. Simulate read, write, and quick-check failures only in the disposable
   storage fixture. Never corrupt or fill a production database to prove the
   condition.

For a development schema-2 database, use the explicit reset/reseed recovery in
`docs/runbooks/edge-agent/development.md`; schema 3 adds no compatibility shim.

## Validation

Run the focused contract and domain gate:

```text
task health:contracts
task health:integration
```

The full repository acceptance gate remains `task validate`. P1-W9 backend and
failure evidence is produced by `task health:integration`, which the quality
workflow now runs so the evidence carries a CI link.

Browser escaping and no-fake-green evidence runs with the P1-W11 route shell:

```text
task web:e2e
```

Each of Chromium, Firefox, and WebKit publishes one bounded, secret-free markup
message on `spool_healthy=False/capacity_critical` through the real device
channel and requires the console to render it as literal text, with no element
created inside the condition list and no dialog raised. Because that report
changes a condition status, it carries a fresh transition time; reusing the
earlier timestamp is correctly rejected as a non-monotonic history rewrite.
