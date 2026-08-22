# ADR 0009: containerd and runc decoy runtime

- Status: Accepted
- Decision refs: DR-02, DR-04, AC-SEC-002
- Source: Step 4 system architecture and technology decisions

## Decision

Use containerd 2.x with an OCI runtime such as runc for decoy lifecycle. The
Edge Agent owns an explicit lifecycle boundary; decoys do not receive the
container runtime socket.

## Consequences

Decoys use a standard OCI ecosystem while lifecycle, resource, capability,
health, and cleanup controls remain product-owned.
