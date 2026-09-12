# P2-W3 decoy runtime security review

- Review dates: 2026-09-11 (workload definition and spec), 2026-09-11
  (lifecycle, seccomp allowlist, lab)
- Work package: P2-W3
- Decisions: DR-02, DR-04, SP-02, ADR 0009
- Acceptance: AC-SEC-001, AC-SEC-002, crash/restart, no runtime socket mount
- Scope: `apps/edge-agent/internal/privileged` — `workload.go`,
  `containerspec.go`, `seccomp.go`, `containerd_client.go`,
  `container_runtime.go`, and the vendored `seccompprofile/`

## The finding to read first: the containerd socket is root

The helper's capability bounding set is `CAP_NET_ADMIN`, and `systemd-analyze`
scores the unit at 1.8. Neither number describes what the helper can cause.
The containerd API has no authorisation beyond the socket's permissions: anyone
who can send it a spec can run anything, as root, with any capability and any
mount. The helper can reach that socket — it could since `P1-W8`, when the
runtime probe first connected — so its effective privilege is root, mediated by
containerd.

What contains a decoy is therefore not the helper's capabilities but what this
code can send containerd, which is only the output of `BuildContainerSpec`. That
is why the spec is hand-written and minimal, why the Edge Agent can name a
workload but never describe one, and why the tests check the serialised spec
rather than the struct. It is also why a bug in this package is a root bug, and
why every new file that imports containerd has to be added by name to the
module's boundary test.

## Trust boundary

The runtime detail comes from a root-owned definition, never from a request. A
compromised Edge Agent can choose among installed, allowlisted workloads and
their run state. It cannot introduce an image, capability, mount, or user.

The definition's controls are aimed at an operator's mistake: a tag instead of
a digest, a second capability, uid 0, a missing resource bound, an unknown field
its author believed was a restriction. The loader validates the id before it
becomes a path, uses `Lstat`, and compares the opened descriptor with the
checked file; the first version used `Stat`, which follows symlinks, and a test
caught it.

## What a decoy is granted

The spec starts from nothing. It cannot express a namespace path, a privileged
flag, a device allowance, or a bind mount, because those fields do not exist in
it. A read-only root, bounded tmpfs, all six namespaces new, capabilities
exactly the grant, `no_new_privs`, the workload's non-root user, memory with no
swap beyond it, CPU and pid limits, deny-all devices.

Two properties were wrong until the lab ran them, and both are now fixed and
asserted:

- The root snapshot was a read-only view, which stops runc creating mount
  points the image does not ship; no decoy could have started. It is now a
  writable snapshot with the spec's read-only remount on top, which is what the
  decoy sees and what the probe checks.
- The telemetry tmpfs was root-owned, so a non-root decoy could not have written
  the events its adapter reads. It now belongs to the workload's uid and stays
  `noexec`.

## Seccomp is the upstream allowlist

`github.com/moby/profiles/seccomp` v0.2.3 — the file Docker Engine 29.8.0
vendors — is embedded unmodified, pinned by SHA-256, with its Apache-2.0
licence beside it. Guardian maintains no syscall policy of its own. It
maintains the translation from Docker's conditional format, and that
translation is tested in both directions: a capability-gated allowance appears
when its capability is granted and not otherwise, an architecture-gated one
only on that architecture, a kernel-gated one only on a kernel known to be new
enough. An unknown kernel satisfies no minimum, and an unmapped architecture is
refused rather than run unfiltered. The profile format is decoded strictly, so
an upstream change to it is noticed at update time.

One difference from the blocklist it replaced is worth recording: upstream
allows `ptrace` and `process_vm_*` from kernel 4.8. A decoy is alone in its pid
namespace and has no `CAP_SYS_PTRACE`, so it can trace only its own processes.
That is Docker's judgement and it is adopted as such.

The lab confirms the filter is in force from inside the decoy.

## Runtime behaviour

- **Namespace scoping.** Every containerd call carries the `guardian-decoy`
  namespace. The helper cannot list, inspect, or remove another tenant's
  containers, and the lab confirms a decoy does not exist when looked up from
  containerd's default namespace.
- **Egress ordering.** A running state is refused unless the default-deny
  policy for exactly the configured ranges is installed, read from the kernel at
  that moment. The refusal happens before containerd is contacted.
- **Stopping never depends on the definition.** An operator can always stop or
  remove a decoy, including one whose definition has been uninstalled.
- **No runtime text crosses the RPC.** containerd and runc errors name sockets,
  paths, and digests; they become closed reason codes. The lab sees the raw
  error through a test-only hook that nothing reachable over the RPC can set.
- **A long call, bounded.** `ReconcileContainer` may hold a request slot for up
  to three minutes because a first pass can fetch an image. Only allowlisted
  workloads reach it, the helper's concurrency bound still applies, and every
  other privileged call keeps five seconds.
- **Digests end to end.** containerd verifies the pull against the pinned
  digest; the helper refuses an image record pointing anywhere else and hashes
  every manifest and config blob it reads itself.

## Network attachment (ADR 0019)

Reviewed 2026-09-13. Scope adds `holderspec.go`, `holdernet_linux.go`, the
`EnsureAddress` rewrite in `hostadapter_linux.go`, the zone rules in
`nftables_linux.go`, `internal/netholder`, `internal/rtnetlink`, and
`cmd/guardian-netholder`.

**No new privilege.** The helper's unit is unchanged: every host-side operation
— creating a veth, moving its peer into a namespace by pid, a route, a proxy
neighbour, per-interface forwarding and proxy_arp through `IFLA_INET_CONF`, the
proxy delay through `RTM_SETNEIGHTBL` — runs under `CAP_NET_ADMIN`, and the
netlink lab proves each one in a container holding nothing else. The one file
the helper now writes, a forwarding record per zone interface, is in its
`RuntimeDirectory`.

**Where the decoy's address comes from.** The owner decided on 2026-09-13 that
it is root's: the workload definition carries `network.interface` and
`network.address`, and the helper refuses a definition whose interface or /32 is
not in its own allowlist with `workload-network-not-allowlisted`. The RPC is
unchanged. A compromised Edge Agent still chooses only which installed workload
runs; it cannot point one at an address.

**The holder.** A Guardian container, not attacker-facing, and the only one with
a host-sourced mount: its own static binary, bind-mounted `ro,nosuid,nodev` onto
an empty root. It runs as uid 65534 with `CAP_NET_ADMIN` in every set,
`no_new_privs`, every namespace new, the upstream seccomp allowlist, and tight
memory, CPU, and pid limits. The helper refuses to run it unless the binary is a
regular, root-owned file writable by no one else (`holder-binary-untrusted`),
because runc executes it with a capability. Its only input is the name of the
interface the helper moves in, parsed to exactly one form; it opens no socket
but netlink and takes no arguments.

**The join is the risk, and it is bounded three ways.** A decoy's spec may carry
one namespace path, `/proc/<pid>/ns/net`, matched against a pattern that admits
nothing else and not pid 1. The pid is the holder task containerd just reported.
And it is re-checked between the task's create and start: if the holder is not
the same running task, the pid could belong to a host process by now, so the
decoy is deleted before its program runs (`holder-changed-before-start`). A
holder's containerd id ends in `.holder`, a form no workload id can take, so no
allowlisted workload can be another's holder.

**Ownership on the host.** Nothing unmarked is changed. The veth carries an
alias naming workload, zone, and holder pid; routes and proxy entries carry
protocol 71. A proxy entry without it, or an address any host interface holds,
is `address-held-by-host` in both directions. A link with a decoy veth's name but
no Guardian alias is `interface-held-by-host`; one belonging to another workload
is `decoy-address-in-use`.

**Forwarding.** ADR 0019's accepted consequence is that the Edge routes for
decoys. It is enabled on a zone interface only after the egress policy is
confirmed on this boot, recorded before it is changed, and restored when the
last Guardian veth on that interface goes. The policy gains two rules: forwarded
traffic arriving on an allowlisted interface is dropped unless addressed to a
decoy range, and anything arriving from a decoy veth goes through the egress
decision whatever its source address — a second route to the same drop that
does not rely on a decoy being unable to change its source. The containerd lab
shows a zone host reaching the decoy and failing to reach a production host
behind the Edge, with the Edge itself reaching production as the control.

**Residual risk.** An allowlisted interface on which the host already routes for
someone else loses that routing when the policy is applied; the runbook makes
this a precondition. Holder and decoy share a network namespace by design; the
holder is Guardian's code and the decoy cannot reach its processes, which are in
another pid namespace.

## Dependencies

No new direct dependency. Using `containerd/api/types` brings
`opencontainers/image-spec` and `opencontainers/go-digest` into `go.sum` as
indirect modules; `containerd/api` already required both, and both are type
definitions. The vendored seccomp profile is data, not code.

## Not covered

- **IPv6 decoys.** Proxy ARP, the veth naming, and the policy are IPv4 only.
- **Image provenance beyond the digest.** Signature enforcement is Phase 5.
  Until then an operator who pins a digest is trusting whoever published it.
- **Private registries.** The pull carries no credentials, so a registry
  requiring authentication fails the pull and the decoy does not start.
- **Pack images.** None exists yet; the lab uses busybox.

## Conclusion

The decoy container is contained by construction, proven from inside a running
one, and now reachable on the address root assigned it and on nothing else. The
helper's own privilege did not change on paper and should not be read as low in
fact: through containerd it is root, and the spec builder — including the one
namespace path it may now write — is the control that matters.
