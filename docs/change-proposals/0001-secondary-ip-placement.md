# Change proposal 0001: routed secondary-IP placement

- Status: APPROVED
- Work package: P0-W5
- Affected decision: ADR 0010 / `NW-01`, `NW-02`, `SP-01`
- Owner decision: `@sinanganiz` approved the recommended option during the
  Phase 0 execution review on 2026-08-22.

## Problem

The edge must present decoy addresses in a routed zone without attaching
decoy lifecycle or host networking privileges to the attacker-facing zone.
Placement must be conflict-detectable, idempotently reconciled, removable, and
unable to create an egress path.

## Options considered

1. Bind the secondary address to the attacker-facing interface. This makes the
   address a local presence in the untrusted zone and blurs the edge/decoy
   boundary.
2. Bind a `/32` secondary address to the edge's routed decoy-side interface.
   The edge owns the local address while the attacker reaches it through the
   existing routed path. This is the recommended option.
3. Add the address to the Docker host or bridge. This couples product network
   truth to host plumbing and bypasses the edge lifecycle boundary.

## Decision

Use option 2. The first lab identity is `172.30.20.99/32`, placed on the edge
interface whose connected route is `172.30.20.0/24`. Placement uses Linux
netlink through `ip`, checks for a duplicate with an ARP probe before adding,
uses `ip address replace` for reconciliation, and removes the address during
cleanup.

The edge nftables policy is default-deny for forwarding and output. Only the
approved zone-to-zone route and internal management/zone destinations are
allowed; there is no default route or masquerading from lab services.

## Acceptance evidence

`tools/lab-secondary-ip.ps1` and `tools/lab-secondary-ip.sh` reset the
disposable lab and prove placement, routed reachability, idempotent reconcile,
duplicate detection, cleanup, and egress denial.

## Consequences

The edge owns the secondary-IP lifecycle and must reconcile it after restart.
The address is intentionally a development lab identity, not a production
allocation. Future address allocation must remain within the routed zone and
must retain the same conflict, cleanup, and egress tests.
