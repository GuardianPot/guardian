# Guardian

Guardian is a private, security-focused deception platform under the
`GuardianPot` organization.

## Repository status

Phase 0 and Phase 1 are delivered. Phase 2 — the Web Console extended packages
`WCX-01` onward — is in progress.

- Product and engineering source of truth: [`0-planning-documents/`](0-planning-documents/)
- Roadmap: [`0-planning-documents/step-6-roadmap/`](0-planning-documents/step-6-roadmap/)
- ADR index: [`docs/adr/`](docs/adr/)
- Where things are written down: [`docs/engineering/context-map.md`](docs/engineering/context-map.md)
- Work packages: [`docs/work-packages/`](docs/work-packages/)

## How work gets delivered

One owner, one agent, direct to `main`. Run `task check` before committing;
it is the same lane CI runs. `full.yml` covers integration, containers, and
browser end-to-end nightly and on demand — locally that is `task validate`.

The whole process is [`AGENTS.md`](AGENTS.md), and it is one page.
