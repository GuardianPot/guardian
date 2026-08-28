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
- The SSH fixture client captures stdout and stderr in independent buffers.
  This prevents the SSH package's concurrent stream-copy goroutines from
  racing on one non-thread-safe `bytes.Buffer`.
- A concurrent regression test runs under Go's race detector on Linux, and the
  fixture completes five successful interactive sessions before event
  normalization.

## Known limitations

- This is a disposable upstream-behavior and containment spike, not the final
  Edge orchestration implementation.
- The test client uses an intentionally disposable password inside the fixture
  only; no production or user credential is involved.
- Full product telemetry ingestion, file quarantine, and independent external
  penetration testing remain later-phase work.
- Separate stream buffers do not preserve a total ordering between stdout and
  stderr. The fixture only requires both streams to be captured without shared
  mutable state; no canonical event contract depends on cross-stream ordering.

## Evidence commands

```text
task cowrie:adapter
bash tools/cowrie-fixture.sh
GOWORK=off go -C tools/cowrie-client test -race ./...
task validate
```
