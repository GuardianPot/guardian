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
