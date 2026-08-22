# Guardian

Guardian is a private, security-focused deception platform under the
`GuardianPot` organization.

## Repository status

The repository currently contains the approved product, architecture, MVP,
roadmap, and engineering-governance baseline. Implementation starts with
Phase 0 after repository bootstrap acceptance is complete.

- Product and engineering source of truth: [`0-planning-documents/`](0-planning-documents/)
- Roadmap: [`0-planning-documents/step-6-roadmap/`](0-planning-documents/step-6-roadmap/)
- Repository and agent workflow: [`0-planning-documents/step-7-repository-and-ai-agent-workflow/`](0-planning-documents/step-7-repository-and-ai-agent-workflow/)
- ADR index: [`docs/adr/`](docs/adr/)

## Current execution rule

All implementation work is performed through a scoped work package, a short-
lived branch, a pull request, CI, and owner review. Agents may propose and
implement changes, but they cannot merge, bypass protections, change
repository settings, or access production secrets.
