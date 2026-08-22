# P0-W6 decoy runtime security review

- Work package: P0-W6
- Decision baseline: ADR 0009
- Review status: READY FOR OWNER ACCEPTANCE

## Review result

The fixture keeps lifecycle authority outside the decoy. The decoy runs with
the containerd-managed `runc` runtime, an exact image digest, no network, no
bind mounts, no runtime sockets, a read-only root, bounded resources, dropped
capabilities, and `no-new-privileges`.

The failure-injection path kills the process and then removes the fixture. Two
complete cycles prove that cleanup and recreation are idempotent.

## Remaining boundary

The fixture validates the runtime boundary only. Product Edge Agent lifecycle
code and decoy adapters remain later scoped work; this review does not approve
attacker-facing decoy behavior.
