# ADR 0011: Adapter-based decoy integration

- Status: Accepted
- Decision refs: DR-06, DR-07, DR-08
- Source: Step 4 system architecture and technology decisions

## Decision

Integrate mature upstream decoys selectively through adapters. Cowrie is the
P0 SSH medium-interaction candidate; OpenCanary remains a candidate
low-interaction pack/reference, not the product core.

## Consequences

Upstream capability is reusable without making product truth, evidence, or
lifecycle semantics depend on upstream architecture.
