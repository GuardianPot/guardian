# ADR 0005: Device gRPC, Protobuf, and mTLS plane

- Status: Accepted
- Decision refs: CP-04, CM-06, TP-01, SA-04
- Source: Step 4 system architecture and technology decisions

## Decision

Use versioned Protobuf contracts over gRPC for the device management channel,
with TLS 1.3 preferred and mTLS device authentication.

## Consequences

Contracts are typed and versioned. During development, breaking changes are
allowed after owner review and all development consumers are updated together.
