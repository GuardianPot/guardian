# Repository bootstrap status

## Completed

- Private `GuardianPot/guardian` repository exists with `main` as default.
- Squash-only merge, merge-commit disabled, rebase disabled, and automatic
  head-branch deletion are enabled.
- Governance files, CODEOWNERS, agent adapters, PR/issue templates, ADR
  system, Phase 0 package specifications, and CI skeleton are committed.
- Dependabot alerts and automated security fixes are enabled.
- P0-W1 through P0-W10 issues exist and are linked to the `Guardian Delivery`
  organization Project with Phase, Component, Risk, Work package, Blocked by,
  and Acceptance status fields.
- The latest quality workflow completed successfully.

## Accepted current-plan limitation

The current GitHub plan does not permit private-repository Rulesets or branch
protection through the available API. Product Owner decision: do not upgrade
the plan. Therefore `main` protection is not claimed as active. CI, CODEOWNERS,
repository policy, owner-managed merge authority, and documented workflow
controls remain active where the current plan supports them.

This is an operational limitation, not an open decision. If the plan changes
later, Rulesets and protection acceptance tests must be enabled before a
production or pilot release.

## Next execution state

P0-W1 is the only READY package. P0-W2 through P0-W10 remain blocked by their
documented dependencies. No Phase 0 package is complete until its evidence is
reviewed and accepted by `@sinanganiz`.
