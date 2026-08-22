# ADR 0003: Go control-plane runtime

- Status: Accepted
- Decision refs: CP-01, TS-01
- Source: Step 4 system architecture and technology decisions

## Decision

Implement the Control Plane in Go 1.27.0 as a compiled, typed, concurrent
service with explicit package boundaries.

## Consequences

The service has a small runtime footprint and predictable deployment. New
runtime changes require a new ADR or change proposal.
