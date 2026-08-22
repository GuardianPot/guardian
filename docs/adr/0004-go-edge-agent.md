# ADR 0004: Go Edge Agent and privilege boundary

- Status: Accepted
- Decision refs: EN-01, EN-02, SA-01
- Source: Step 4 system architecture and technology decisions

## Decision

Implement the Edge Agent in Go 1.27.0. Keep privileged host/network/runtime
operations behind a narrow explicit boundary; do not grant decoy processes
runtime sockets or broad host authority.

## Consequences

Edge implementation can control Linux primitives while keeping attacker-facing
decoys constrained and reviewable.
