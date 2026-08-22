# ADR 0006: REST/OpenAPI user and integration API

- Status: Accepted
- Decision refs: CP-04, TS-06
- Source: Step 4 system architecture and technology decisions

## Decision

Use REST/JSON with an OpenAPI description for user-facing and integration API
surfaces. Generate typed clients or use a thin typed wrapper rather than
duplicating DTO definitions.

## Consequences

External integrations get a conventional HTTP contract while the device plane
remains separate and strongly typed.
