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
- P0-W1 through P0-W10 have accepted Phase 0 fixture evidence, merged PRs,
  and closed issues.
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

The current authorized execution batch is complete. All P0-W1 through P0-W10
packages have evidence; the Phase 0 gate remains `IN REVIEW — NOT APPROVED`
until the Product Owner records the human gate decision.
