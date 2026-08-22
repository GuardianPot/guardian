# ADR 0002: Modular product architecture

- Status: Accepted
- Decision refs: SD-01, SD-02, RE-01
- Source: Step 4 system architecture and technology decisions

## Decision

Use a modular product architecture with distributed components only where the
security or deployment boundary requires it: Control Plane, Web Console, Edge
Agent, and isolated decoy/runtime boundaries.

## Consequences

The product avoids premature microservice breadth while preserving explicit
trust boundaries and independently testable components.
