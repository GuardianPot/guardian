# Privileged helper development runbook

## Build and verify

From the repository root:

```bash
task privileged:generated
task privileged:security
task presence:netlink
GOWORK=off go -C apps/edge-agent test ./...
```

`task privileged:security` runs ordinary abuse tests, systemd profile checks,
and a network-disabled root container with only `CHOWN`, `SETUID`, and `SETGID`
capabilities. That lab proves an authorized UID/GID can call `GetStatus`, wrong
UID and GID peers fail before dispatch and are audited, and a decoy identity
cannot open the production-mode socket.

`task presence:netlink` exercises the address adapter against a real kernel in a
container holding `CAP_NET_ADMIN` and nothing else, in its own throwaway network
namespace. It proves an address is added, observed, removed, that repeating
either is reported as no change, and that an address the adapter did not label
is refused in both directions and survives untouched. Both labs run in the
`full` workflow through `task go:check`; neither runs in the fast lane.

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

## What the installed helper can do

`P2-W1` gave the helper one host-mutating capability and nothing else.

| Operation | State | Notes |
|---|---|---|
| `EnsureAddress` | implemented | Adds and removes IPv4 addresses over `NETLINK_ROUTE` |
| `ApplyNftablesPolicy` | `phase-2-adapter-not-implemented` | `P2-W2` |
| `ReconcileContainer` | `phase-2-adapter-not-implemented` | `P2-W3` |
| `EnsureNetworkNamespace` | `phase-2-adapter-not-implemented` | `P2-W3` |

The unit's `CapabilityBoundingSet=CAP_NET_ADMIN` is the whole of its privilege.
Do not add `CAP_SYS_ADMIN`, `CAP_NET_RAW`, a runtime socket, or a raw policy
input through a local override; each belongs to a later work package with its
own security review, and `task privileged:security` fails if the unit grants
more than the one capability.

### Installing it changes nothing on its own

The helper mutates nothing until the operator names what it may touch. With no
arguments, every `EnsureAddress` is refused with `interface-not-allowlisted` or
`address-prefix-not-allowlisted` before the adapter is reached. The flags are
root-controlled, repeatable, and match exact values only:

```text
--allow-interface guardian0
--allow-address-range 192.0.2.0/24
--allow-namespace guardian-decoy-a
--allow-workload guardian-workload-a
```

No positional argument is accepted. Put only decoy ranges in
`--allow-address-range`: an allowlisted range containing a production host's
address is the one configuration mistake this design cannot protect against.

### Addresses Guardian placed are labelled

Every address the adapter adds carries the IPv4 label `<interface>:gdn`, so
`ip -4 addr show` distinguishes them by eye:

```bash
ip -4 -o addr show label '*:gdn'
```

The label is also the adapter's ownership check. An address on an allowlisted
interface that does not carry it is refused in both directions with
`address-held-by-host` — Guardian will neither adopt nor remove an address it
did not place. An interface whose name is longer than 11 characters cannot
carry a label that fits the kernel's 15-character limit, and address operations
on it are refused with `interface-name-too-long-to-label` rather than performed
unlabelled.

### If the capability is reported unsupported

`GetStatus` reports `PRIVILEGED_OPERATION_ADDRESS` with a reason:

| Reason | Meaning |
|---|---|
| `netlink-address-adapter` | Working |
| `no-cap-net-admin` | The unit's bounding set does not include `CAP_NET_ADMIN` |
| `netlink-unavailable` | `RestrictAddressFamilies` omits `AF_NETLINK`, or `PrivateNetwork=yes` |

All three are the service profile, not the binary. Check the unit before
anything else.

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
