# Phase 2 work packages

Phase 2 turns the Edge into a working deception sensor. The approved catalog
is [`03_Phase_2_Deception_Runtime_Networking_and_Telemetry.md`](../../../0-planning-documents/step-6-roadmap/03_Phase_2_Deception_Runtime_Networking_and_Telemetry.md),
which the Product Owner approved as FINAL. That roadmap names fifteen
workstreams, `P2-W1` through `P2-W15`.

This directory holds the ones written up as implementable work packages. A
roadmap entry is a paragraph of intent; a work package is what an agent can
execute against, with allowed paths, security constraints, and acceptance
criteria. Entries are written when they are about to be worked on, not in
advance.

| Package | Status |
|---|---|
| `P2-W15` — Decoy domain, lifecycle contract, and management surface | delivered 2026-09-08 |
| `P2-W4` — Decoy manifest schema | draft, implemented on `main` 2026-09-08 |
| `P2-W1` — Routed presence driver | draft, driver, conflict probe, and proxy-ARP adapter on `main` 2026-09-13 |
| `P2-W2` — nftables egress policy | draft, implemented on `main`; zone forwarding rule added 2026-09-13 |
| `P2-W3` — containerd production runtime manager | draft, implemented on `main` with ADR 0019 network attachment 2026-09-13 |
| `P2-W10` — Canonical event and evidence envelope | draft, implemented on `main` 2026-09-08 |
| `P2-W11` — Edge normalization adapters | draft, implemented on `main` 2026-09-08 |
| `P2-W9` — Synthetic credential domain | draft, domain on `main` 2026-09-10; decoy delivery blocked |

Every other roadmap workstream is unwritten.

`P2-W4` was written and implemented in the same pass, in the order the Product
Owner set. Its `status` stays `draft` because promoting it is the owner's
call, not an agent's; the code, the schema, the manifests, and the security
review are on `main` for that decision to be made against.

`WCX-11` is unblocked: every `UX-06` field now exists in the contract, and
`openapi/guardian.yaml` carries the decoy paths the console reads. What it will
render today is a list of decoys whose observed state is `unknown`, because
`P2-W3` has not supplied a container runtime. That is the intended output, and
the console must render it as unknown rather than as anything more reassuring.

## How P2-W15 relates to WCX-11

The roadmap's one-line summary of `P2-W15` is "Decoy management UI", which
reads as a console package. It is not one, and `WCX-11` settles it: that
package's section 4 states that `P2-W15` and the Phase 2 decoy backend must be
accepted first, because `WCX-11` "consumes a contract it does not create".

So the split is:

- **`P2-W15`** creates the decoy — the domain, the contract, the transport,
  and the observed-state reporting. It is forbidden from touching
  `apps/web-console/src/**`.
- **`WCX-11`** renders it, with the form and validation stack, hostile-content
  rendering, and confirmation levels. It is forbidden from touching
  `openapi/**` and the Control Plane.

Neither duplicates the other, and the Phase 2 exit gate — four decoy families
visible in the console — needs both plus the runtime packages `P2-W3` and
`P2-W5` through `P2-W8`.

## Lifecycle

Phase 2 is not delivered. Delivery process is in `AGENTS.md`: a package is
implemented directly on `main` once the owner approves it, and its status here
is not changed by an agent without that approval.
