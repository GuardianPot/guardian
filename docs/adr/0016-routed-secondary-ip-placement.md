# ADR 0016: Routed secondary-IP placement

- Status: Accepted
- Decision refs: `NW-01`, `NW-02`, `SP-01`
- Source: P0-W5 spike and change proposal 0001
- Owner approval: `@sinanganiz`, 2026-08-22

## Decision

Bind routed decoy secondary addresses as `/32` identities on the edge
interface serving the decoy-side zone. Detect conflicts before placement,
reconcile with netlink, remove on cleanup, and enforce default-deny nftables
forward/output policy with no lab default route or masquerading.

## Consequences

The edge owns the address lifecycle and must reconcile it after restart. The
placement remains isolated from the attacker-facing L2 segment and is
validated by the P0-W5 disposable lab spike.
