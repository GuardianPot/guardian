# Canonical device and telemetry contracts

Status: Accepted for the Phase 0 skeleton

## Versioning

- Protobuf packages, JSON Schema IDs, and API paths carry an explicit major
  version (`guardian.device.v1`, `guardian.telemetry.v1`, and `/v1/`).
- Additive fields may be introduced inside a development major version only
  after owner review. A semantic or security-breaking shape change creates the
  next major version and updates all development consumers together.
- There is no implied compatibility promise between development snapshots;
  the checked-in major version is the traceable contract boundary.
- `event_id` is stable across retries and replay. `observed_at` is an RFC 3339
  timestamp, and device identifiers use the same restricted identifier grammar
  across JSON and protobuf consumers.

## Scope boundary

Phase 0 defines only identity and telemetry transport envelopes. Device
capabilities, commands, product measurements, AI decisions, alert taxonomies,
and Phase 1–5 workflows remain outside this skeleton until their evidence and
acceptance criteria exist.

## Traceability

- Device gRPC, Protobuf, and mTLS: [ADR 0005](../adr/0005-device-grpc-protobuf-mtls.md).
- REST/OpenAPI boundary: [ADR 0006](../adr/0006-rest-openapi-api.md).
- Durable event identity and replay: [ADR 0008](../adr/0008-edge-sqlite-wal.md)
  and [P0-W8](../work-packages/phase-0/P0-W8.md).
- Device identity and mTLS lifecycle: [ADR 0013](../adr/0013-product-device-pki.md)
  and [P0-W9](../work-packages/phase-0/P0-W9.md).
- Contract decision record: [ADR 0017](../adr/0017-versioned-device-telemetry-contracts.md).
