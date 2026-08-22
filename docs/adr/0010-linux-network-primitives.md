# ADR 0010: Linux network primitives

- Status: Accepted
- Decision refs: NW-01, NW-02, SP-01
- Source: Step 4 system architecture and technology decisions

## Decision

Use Linux network namespaces/netlink and nftables for network presence,
secondary-IP placement, routing, and default-deny egress policy.

## Consequences

The network behavior remains inspectable and uses maintained kernel primitives;
P0-W5 must prove conflict, reconcile, cleanup, and egress behavior.
