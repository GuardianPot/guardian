# Architecture Decision Records

ADRs record material architecture decisions and their lifecycle. They do not
silently replace approved product, MVP, roadmap, or governance decisions.

## Status lifecycle

`Proposed → Accepted → Superseded` (or `Rejected` when a proposal is closed
without adoption).

## Creating an ADR

1. Identify the affected approved decision and acceptance criteria.
2. Describe context, considered options, decision, consequences, security
   impact, and migration/rollback implications.
3. Open a change-proposal issue if the proposal changes approved scope,
   architecture, contracts, or security boundaries.
4. Obtain `@sinanganiz` review before changing status to `Accepted`.

## Naming

Use `NNNN-short-kebab-case.md`, starting at `0001`.

Template: [`0000-template.md`](0000-template.md)

## Imported approved decision records

The following records normalize the high-impact decisions already approved in
Step 4. They are durable traceability records; they do not silently change
the Step 2–6 source documents.

| ADR | Decision |
|---|---|
| [0001](0001-repository-and-agent-governance.md) | Monorepo and agent governance |
| [0002](0002-modular-product-architecture.md) | Modular product architecture |
| [0003](0003-go-control-plane.md) | Go control-plane runtime |
| [0004](0004-go-edge-agent.md) | Go Edge agent and privilege boundary |
| [0005](0005-device-grpc-protobuf-mtls.md) | Device gRPC/Protobuf/mTLS plane |
| [0006](0006-rest-openapi-api.md) | REST/OpenAPI user API |
| [0007](0007-central-postgresql.md) | PostgreSQL central persistence |
| [0008](0008-edge-sqlite-wal.md) | SQLite WAL edge durability |
| [0009](0009-containerd-runc-runtime.md) | containerd + runc runtime |
| [0010](0010-linux-network-primitives.md) | Linux netns/netlink/nftables |
| [0011](0011-adapter-based-decoy-integration.md) | Adapter-based decoy integration |
| [0012](0012-ai-provider-abstraction.md) | Provider-abstracted AI boundary |
| [0013](0013-product-device-pki.md) | Product-specific device X.509 CA |
| [0014](0014-opentelemetry-observability.md) | OpenTelemetry instrumentation |
| [0015](0015-reproducible-artifacts.md) | Reproducible artifacts and provenance |

## Change proposals

Material changes use the versioned templates in
[`docs/change-proposals/`](../change-proposals/). An agent may draft a
proposal, but only `@sinanganiz` can accept it.
