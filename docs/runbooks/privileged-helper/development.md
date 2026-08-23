# Privileged helper development runbook

## Build and verify

From the repository root:

```bash
task privileged:generated
task privileged:security
GOWORK=off go -C apps/edge-agent test ./...
```

`task privileged:security` runs ordinary abuse tests, systemd profile checks,
and a network-disabled root container with only `CHOWN`, `SETUID`, and `SETGID`
capabilities. That lab proves an authorized UID/GID can call `GetStatus`, wrong
UID and GID peers fail before dispatch and are audited, and a decoy identity
cannot open the production-mode socket.

## Reference installation

Build without embedding credentials:

```bash
CGO_ENABLED=0 GOWORK=off go -C apps/edge-agent build -trimpath -o /tmp/guardian-edge-privd ./cmd/edge-privd
```

Install the binary as
`/usr/libexec/guardian-edge/guardian-edge-privd`, owner `root:root`, mode
`0755`. Install the sysusers, tmpfiles, helper service, and main service files
from `deploy/edge-agent/`, then run:

```bash
systemd-sysusers
systemd-tmpfiles --create
systemd-analyze verify guardian-edge-privd.service guardian-edge.service
systemctl daemon-reload
systemctl enable --now guardian-edge-privd.service guardian-edge.service
```

Expected socket metadata:

```text
/run/guardian-edge-privd                    root:guardian-edge 0750
/run/guardian-edge-privd/guardian-edge-privd.sock root:guardian-edge 0660
```

## P1 operation state

The installed P1 unit has no allowlist arguments and an empty capability
bounding set. `GetStatus` is reachable, while every host-mutating request is
denied by the argument allowlist or returns
`phase-2-adapter-not-implemented`. Do not grant `CAP_NET_ADMIN`,
`CAP_SYS_ADMIN`, a runtime socket, or a raw policy input through a local
override. Those changes require their later approved work package and security
review.

For isolated contract development, root-controlled repeatable flags exist for
exact values only:

```text
--allow-interface guardian0
--allow-address-range 192.0.2.0/24
--allow-namespace guardian-decoy-a
--allow-workload guardian-workload-a
```

No positional argument is accepted. These flags authorize validation; the P1
adapter still performs no host mutation.

## Diagnosis and recovery

1. Inspect `systemctl status guardian-edge-privd.service` and the Edge
   `privileged-helper` health reason.
2. Verify directory/socket owner, group, type, and mode. Never replace the
   socket with a symlink or regular file; startup fails closed.
3. Read structured `guardian-edge-privd` journal events. They contain only
   redacted fingerprints and peer metadata.
4. Restart the helper. The Edge client retries automatically and returns to
   `healthy/reachable` after a successful typed status probe.
5. If a stale socket is owned by the wrong identity or is not a socket, stop
   and investigate. The helper deliberately refuses to remove it.

The helper has no persistent data. Its bounded idempotency cache is rebuilt on
restart; convergent adapters are responsible for safe retries.
