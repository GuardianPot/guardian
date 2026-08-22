# ADR 0017: Versioned device and telemetry contracts

- Status: Accepted
- Date: 2026-08-22
- Decision refs: P0-W10, CP-04, CM-06
- Acceptance refs: P0-W1, P0-W8, P0-W9

## Context

The repository needs one traceable contract boundary for device identity and
telemetry without prematurely modeling product capabilities.

## Decision

Keep a minimal v1 contract in three synchronized surfaces: versioned
Protobuf packages for the device plane, JSON Schemas for payload validation,
and the `/v1/telemetry` OpenAPI endpoint. The v1 envelope carries stable event
and device identifiers, a schema version, an RFC 3339 observation time, and an
opaque payload.

## Consequences

The Phase 0 contract is small enough to validate and evolve with spike
evidence. Product-specific fields remain intentionally unspecified. Breaking
development changes are allowed after owner review and require an explicit
major-version boundary.

## Security and failure behavior

Contracts do not grant device trust. mTLS identity, certificate lifecycle, and
revocation remain governed by ADR 0013 and P0-W9. Stable event IDs preserve
replay and duplicate-prevention behavior validated by P0-W8.

## Evidence

- `proto/guardian/device/v1/device.proto`
- `proto/guardian/telemetry/v1/telemetry.proto`
- `schemas/device/v1/device-identity.schema.json`
- `schemas/telemetry/v1/telemetry-envelope.schema.json`
- `openapi/guardian.yaml`
- `tools/check-canonical-contracts.mjs`
