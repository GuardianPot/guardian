# Privileged helper development runbook

## Build and verify

From the repository root:

```bash
task privileged:generated
task privileged:security
task privileged:netlink
task privileged:containerd
GOWORK=off go -C apps/edge-agent test ./...
```

`task privileged:containerd` runs pinned, checksum-verified containerd 2.3.5 and
runc 1.5.1 in a throwaway privileged container, pulls busybox by digest, and
runs the decoy lifecycle against them. A probe running as the decoy reports its
own isolation by exit code. It needs network access and an amd64 host.

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
`0755`. Build the decoy network holder the same way and install it beside the
helper; the helper refuses to run one that is not root-owned or is writable by
anyone else:

```bash
CGO_ENABLED=0 GOWORK=off go -C apps/edge-agent build -trimpath -o /tmp/guardian-netholder ./cmd/guardian-netholder
install -o root -g root -m 0755 /tmp/guardian-netholder /usr/libexec/guardian-edge/guardian-netholder
```

 Install the sysusers, tmpfiles, helper service, and main service files
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

`P2-W1`, `P2-W2`, and `P2-W3` are implemented, including decoy network
attachment (ADR 0019), and none of them needed a capability beyond the one the
unit grants.

| Operation | State | Notes |
|---|---|---|
| `EnsureAddress` | implemented | Adds and removes a decoy address's proxy-ARP entry over `NETLINK_ROUTE` |
| `ApplyNftablesPolicy` | implemented | Default-deny decoy egress and zone forwarding restriction over `NETLINK_NETFILTER` |
| `ReconcileContainer` | implemented | Decoy and network-holder containers through containerd 2.x, and their host side |
| `EnsureNetworkNamespace` | `phase-2-adapter-not-implemented` | Would need `CAP_SYS_ADMIN`; a decoy's namespace belongs to its holder instead (ADR 0019) |

The unit's `CapabilityBoundingSet=CAP_NET_ADMIN` is the whole of the helper's
own privilege. It is not the whole of what the helper can cause: the containerd
socket is a root-equivalent API, so a decoy is contained by the spec the helper
sends, not by the helper's capabilities.
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

Allowlist only zone interfaces the host does not already route for. Once the
egress policy is applied, traffic forwarded in from an allowlisted interface is
dropped unless it is addressed to a decoy range.

### What Guardian puts on the host is marked

A decoy's address lives inside its holder's namespace, not on the host.
`EnsureAddress` adds a proxy-ARP entry for it on the zone interface, and the
runtime adds a veth and a `/32` route. Each is marked where the kernel keeps it:

```bash
ip neigh show proxy            # entries with "proto 71" are Guardian's
ip route show proto 71         # one /32 per running decoy
ip -d link show | grep -B1 'alias guardian-decoy'
```

The marks are also the ownership check. A proxy entry without protocol 71, or
an address a host interface holds, is refused in both directions with
`address-held-by-host`. A link named like a decoy veth without Guardian's alias
is refused with `interface-held-by-host`. Guardian adopts and removes only what
it placed.

The kernel answers a proxy entry only on an interface that forwards. Forwarding
on the zone interface is enabled when the first decoy on it is attached and
turned off again when the last goes, if it was off before; the record of that is
a file per interface in `/run/guardian-edge-privd/forwarding`. The proxy delay on
the interface is set to zero.

### The decoy egress policy

`ApplyNftablesPolicy` installs `AC-SEC-003`: a decoy cannot open an outbound
connection. The ruleset lives in the host's own `ip` table `guardian_decoy` and
is keyed on the `--allow-address-range` prefixes — the same set that decides
which addresses may be placed, so the two cannot drift apart. Inspect it with:

```bash
nft list table ip guardian_decoy
```

Two base chains, `guardian_forward` and `guardian_output`, send traffic from a
decoy source — and, in `guardian_forward`, anything arriving from a `gdn*` decoy
veth — to `guardian_egress`, which accepts an established or related reply and
drops everything else. `guardian_forward` also sends traffic arriving on each
`--allow-interface` to `guardian_zone`, which lets it through only to a decoy
range. Both base chains carry policy `accept` on purpose: a
`drop` policy in a Guardian table would drop the host's own traffic, so the
denial is in the rules and scoped to decoy sources.

Every rule carries a marker naming the policy version and a digest of the ranges
and interfaces it was built from, which is how the helper tells "already applied" from "the
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
  "user": { "uid": 10001, "gid": 10001 },
  "network": { "interface": "guardian0", "address": "192.0.2.40" }
}
```

Every field is required. The image is identified by digest only — there is no
tag form. `NET_BIND_SERVICE` is the only grantable capability, the uid and gid
must not be 0, and an unknown field is refused rather than ignored. The file
must be a regular file: a symlink is refused, not followed.

`network` is where the decoy answers: one IPv4 address in canonical form, on one
zone interface. It must be the address the Control Plane assigned the decoy, and
the helper refuses it with `workload-network-not-allowlisted` unless the
interface is an `--allow-interface` and the address is inside an
`--allow-address-range`.

### Decoy containers

The helper needs containerd 2.x with its Transfer service (the default in 2.x)
at `/run/containerd/containerd.sock`. Everything it creates lives in the
containerd namespace `guardian-decoy`, and nothing else on the host should use
that namespace:

```bash
ctr -n guardian-decoy containers ls
ctr -n guardian-decoy tasks ls
```

A running state is refused with `egress-policy-not-applied` until the egress
policy is installed on the current boot, so after a reboot apply the policy
before asking for decoys. Stopped and absent work regardless, including for a
workload whose definition has been removed.

Images are fetched by digest from the definition's repository without
credentials; a registry that requires authentication fails the pull and the
decoy does not start.

Every decoy has a network holder, the container `<workload-id>.holder`, which
owns the decoy's network namespace and gives it `eth0` with its `/32`. Holder and
decoy restart together: a holder that dies is replaced on the next pass, and
its decoy is rebuilt into the new namespace (`holder-replaced`). Stopping ends
the decoy, removes its veth, then ends the holder; removing also deletes both
containers.

| Refusal | Meaning |
|---|---|
| `workload-not-installed` | No definition file for this id |
| `workload-definition-invalid` | The definition failed validation |
| `workload-network-not-allowlisted` | The definition's interface or address is outside the helper's allowlist |
| `egress-policy-not-applied` | The default-deny policy is not installed right now |
| `holder-binary-untrusted` | The holder binary is missing, not root-owned, or writable by others |
| `image-digest-mismatch` | The image record does not point at the pinned digest |
| `image-platform-unavailable` | The image has no manifest for this host's platform |
| `holder-start-failed` | The runtime could not start the holder |
| `container-start-failed` | The runtime could not start the decoy; nothing was left behind |
| `holder-changed-before-start` | The holder changed while the decoy was joining it; retried on the next pass |
| `interface-not-found` | The definition's zone interface does not exist |
| `interface-held-by-host` | A link with the decoy veth's name is not Guardian's |
| `decoy-address-in-use` | Another workload's decoy is attached at this address |
| `route-held-by-host` | A route to the decoy's address exists that is not Guardian's |
| `network-attach-failed` | The kernel refused a host-side change; see the helper's journal |
| `runtime-unreachable` | containerd did not answer on its socket |

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
| `containerd-runtime-adapter` | containers | Working |
| `no-workloads-allowlisted` | containers | No `--allow-workload`, so there is nothing to run |

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
