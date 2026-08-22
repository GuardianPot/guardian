# ADR 0013: Product-specific device X.509 CA

- Status: Accepted
- Decision refs: SA-02, SA-03, SA-05, SP-07
- Source: Step 4 system architecture and technology decisions

## Decision

Use a product-specific X.509 device CA with standard Go crypto/TLS
primitives. Store Edge private keys in OS-protected files as the baseline;
TPM-backed storage is future hardening, not a current requirement.

## Consequences

Enrollment, rotation, revocation, and replay resistance are product-owned and
can be proven in a disposable test CA without production trust material.
