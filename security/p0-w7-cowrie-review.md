# P0-W7 Cowrie adapter security review

Status: Accepted for Phase 0 fixture scope

## Reviewed controls

- Official Cowrie `v3.0.12` OCI image is referenced by immutable digest and its
  embedded upstream revision label is checked.
- The disposable container uses an internal Docker network, so the Cowrie
  process has no default route to arbitrary external destinations.
- Root filesystem is read-only; only bounded tmpfs paths and the image's
  disposable anonymous state volumes are writable.
- No host bind mounts, Docker/containerd sockets, privileged mode, or retained
  production credentials are used.
- All Linux capabilities are dropped and `no-new-privileges` is enabled.
- CPU, memory, PID, and writable-state bounds are verified by the fixture.
- Failed and successful authentication events are normalized into an allowlist
  shape; the raw Cowrie password field is deliberately discarded.
- Attacker command content is classified as untrusted data and cannot become an
  executable or instruction field in canonical telemetry.
- Malformed raw event lines are reported and skipped without discarding valid
  evidence or crashing the proof harness.

## Known limitations

- This is a disposable upstream-behavior and containment spike, not the final
  Edge orchestration implementation.
- The test client uses an intentionally disposable password inside the fixture
  only; no production or user credential is involved.
- Full product telemetry ingestion, file quarantine, and independent external
  penetration testing remain later-phase work.

## Evidence commands

```text
task cowrie:adapter
task validate
```
