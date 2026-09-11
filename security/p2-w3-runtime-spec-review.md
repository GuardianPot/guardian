# P2-W3 decoy runtime spec security review

- Review date: 2026-09-11
- Work package: P2-W3 (first slice: workload definition and OCI spec)
- Decisions: DR-02, DR-04, SP-02, ADR 0009
- Acceptance: AC-SEC-001, AC-SEC-002 in the structural direction
- Scope: `apps/edge-agent/internal/privileged/workload.go` and
  `containerspec.go`. No container is created by anything in this slice.

## What this slice decides

Every other isolation statement in the repository is about what cannot be
*asked for*. The OCI spec is the one place that says what a decoy is *granted*,
so it is where `AC-SEC-001` and `AC-SEC-002` are actually decided.

## Trust boundary

The runtime detail comes from a root-owned file, not from any request. The Edge
Agent sends only a workload id. A compromised Edge Agent can choose among
installed, allowlisted workloads and their run state; it cannot introduce an
image, capability, mount, or user.

Inside the file, the controls are aimed at an operator's mistake rather than an
attacker, since only root can write it: a tag instead of a digest, a second
capability, uid 0, a missing resource bound, or an unknown field that its author
believed was a restriction. Each is refused, and each refusal is tested.

The loader takes the workload id from the Edge, so the id is validated against
the allowlist pattern before it becomes part of a path, the file is checked with
`Lstat`, and the opened descriptor is compared with the checked one. The first
version used `Stat`, which follows symlinks; the symlink test caught it before
it was committed.

## The spec starts from nothing

The spec is a small hand-written struct rather than the full runtime-spec
types. A builder that starts from a complete struct grants whatever it forgets to
clear; this one cannot express a host namespace path, a privileged flag, a
device allowance, or a bind mount, because none of those fields exists in it.

Asserted against the serialised JSON — what containerd would receive:

- no mention of the runtime socket, Docker's socket, or any Guardian state or
  configuration path;
- every mount sourced from a kernel filesystem, none from a host path, none a
  bind;
- read-only root; bounded tmpfs; `noexec` on the areas a decoy writes;
- capability sets exactly the grant, and empty without one; `noNewPrivileges`;
- the workload's non-root uid and gid, never the image's;
- all six namespaces new;
- memory with no swap beyond it, CPU quota, pids, deny-all devices.

The image supplies only its program: arguments, environment, and working
directory, bounded, NUL-free, and with an absolute, clean working directory.
That runs inside the boundary above and cannot widen it.

## Seccomp is a blocklist

The profile allows by default and denies about fifty syscalls that load
kernel code, trace other processes, manipulate mounts and namespaces, or reach
escalation-prone kernel interfaces (`bpf`, `io_uring`, `userfaultfd`, keyrings).
With empty capabilities and `noNewPrivileges` most of them already fail; the
profile stops them before they reach the kernel implementation.

This is weaker than the default-deny allowlist Docker and containerd ship, and
it is named as a blocklist in the code, the tests, and here. It should not be
read as equivalent. Cowrie runs attacker input, so `P2-W5` should not ship on it
alone; adopting a maintained allowlist is recorded as an owner decision in
`P2-W3` section 7.

## Privilege

This slice changes no privilege and no service directive. The analysis for the
rest of the package: the container lifecycle needs no new capability, because
containerd does the privileged work and the helper already reaches its socket.
The capability that would grow is `CAP_SYS_ADMIN`, for creating network
namespaces in `EnsureNetworkNamespace`, and the profile has no room for it. That
is the network-attachment decision in `P2-W3` section 7.

## Not covered

The runtime direction of `AC-SEC-002` — a running decoy failing to reach the
socket — needs a real container and belongs to the lifecycle slice. So do
restart behaviour, partial-create cleanup, and the ordering requirement from
`P2-W2` that no decoy starts before the egress policy is applied on the current
boot.

## Conclusion

No privilege, dependency, contract, or execution surface changed. What a decoy
receives is fixed by a spec that cannot express the grants `AC-SEC-001` and
`AC-SEC-002` forbid, tested at the level a runtime reads. Two known weaknesses
are named rather than hidden: a blocklist seccomp profile, and a network
attachment design that is not yet decided.
