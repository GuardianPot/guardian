# Privileged helper development runbook

## Build and verify

From the repository root:

```bash
task privileged:generated
task privileged:security
task privileged:netlink
GOWORK=off go -C apps/edge-agent test ./...
```

`task privileged:security` runs ordinary abuse tests, systemd profile checks,
and a network-disabled root container with only `CHOWN`, `SETUID`, and `SETGID`
capabilities. That lab proves an authorized UID/GID can call `GetStatus`, wrong
UID and GID peers fail before dispatch and are audited, and a decoy identity
cannot open the production-mode socket.

`task privileged:netlink` exercises both host adapters against a real kernel in a
container holding `CAP_NET_ADMIN` and nothing else, in its own throwaway network
namespace.

For addresses it proves one is added, observed, removed, that repeating either
is reported as no change, and that an address the adapter did not label is
refused in both directions and survives untouched.

For egress it proves the thing that matters: a datagram sent from a decoy source
address is refused by the kernel with `EPERM` once the policy is applied, while
the same datagram to the same destination from the host's own address still
goes. It also deletes the table behind the adapter's back to confirm a removed
policy reads as unconverged rather than as already applied.

Both labs run in the `full` workflow through `task go:check`; neither runs in the
fast lane.

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

`P2-W1` and `P2-W2` are implemented, and both fit inside the one capability the
unit already grants.

| Operation | State | Notes |
|---|---|---|
| `EnsureAddress` | implemented | Adds and removes IPv4 addresses over `NETLINK_ROUTE` |
| `ApplyNftablesPolicy` | implemented | Default-deny decoy egress over `NETLINK_NETFILTER` |
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

### The decoy egress policy

`ApplyNftablesPolicy` installs `AC-SEC-003`: a decoy cannot open an outbound
connection. The ruleset lives in the host's own `ip` table `guardian_decoy` and
is keyed on the `--allow-address-range` prefixes — the same set that decides
which addresses may be placed, so the two cannot drift apart. Inspect it with:

```bash
nft list table ip guardian_decoy
```

Two base chains, `guardian_forward` and `guardian_output`, send traffic from a
decoy source to `guardian_egress`, which accepts an established or related reply
and drops everything else. Both base chains carry policy `accept` on purpose: a
`drop` policy in a Guardian table would drop the host's own traffic, so the
denial is in the rules and scoped to decoy sources.

Every rule carries a marker naming the policy version and a digest of the ranges
it was built from, which is how the helper tells "already applied" from "the
table was flushed" and from "the configured ranges changed". The result is read
back from the kernel before the helper reports it as applied.

The ruleset does not survive a reboot — nftables state is kernel state. It is
reinstalled by the next reconcile pass, and a decoy must not be started before
that pass completes.

### Workload definitions

`ReconcileContainer` names a workload; it cannot describe one. What a workload
id means on this host is a root-owned file, and a container can exist only if
the file is installed *and* the id is allowlisted with `--allow-workload`:

```text
/etc/guardian-edge/workloads/<workload-id>.json   root:root 0644
```

```json
{
  "schema": "guardian.workload.v1",
  "workload_id": "guardian-workload-ssh-a",
  "pack": "ssh-cowrie",
  "pack_version": "0.1.0",
  "image": {
    "repository": "registry.example.internal/guardian/ssh-cowrie",
    "digest": "sha256:<64 hex characters>"
  },
  "ports": [{ "port": 22, "protocol": "tcp" }],
  "privileges": { "capabilities": ["NET_BIND_SERVICE"] },
  "resources": { "cpu_millicores": 500, "memory_mib": 256, "pids": 128 },
  "user": { "uid": 10001, "gid": 10001 }
}
```

Every field is required. The image is identified by digest only — there is no
tag form. `NET_BIND_SERVICE` is the only grantable capability, the uid and gid
must not be 0, and an unknown field is refused rather than ignored. The file
must be a regular file: a symlink is refused, not followed.

The container lifecycle that consumes these definitions is not implemented yet;
see `docs/work-packages/phase-2/P2-W3.md`. Installing a definition today changes
nothing on the host.

### If a capability is reported unsupported

`GetStatus` reports each operation with a reason:

| Reason | Operation | Meaning |
|---|---|---|
| `netlink-address-adapter` | address | Working |
| `nftables-egress-adapter` | egress | Working |
| `no-decoy-ranges-configured` | egress | No `--allow-address-range`, so the policy would protect nothing |
| `no-cap-net-admin` | both | The unit's bounding set does not include `CAP_NET_ADMIN` |
| `netlink-unavailable` | both | `RestrictAddressFamilies` omits `AF_NETLINK`, or `PrivateNetwork=yes` |
| `nftables-unavailable` | egress | The kernel has no `nf_tables` subsystem |

Only `no-decoy-ranges-configured` is an argument problem. The rest are the
service profile or the kernel, not the binary; check the unit before anything
else.

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
