# ADR 0001: Monorepo and agent governance

- Status: Accepted
- Decision refs: GOV-02, GOV-03, AG-01, AP-04, WP-01
- Source: Step 7 repository and AI-agent workflow specification

## Decision

Use one private product monorepo. Agents work from scoped packages and may
propose changes through PRs, but owner authority remains required for merge,
architecture, security, and release decisions.

## Consequences

Product and engineering context remain versioned together. Agent instructions,
workflow files, contracts, and security paths receive owner review.
