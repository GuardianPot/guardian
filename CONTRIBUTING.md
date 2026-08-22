# Contributing to Guardian

## Before changing code

Start from a READY work-package issue. Read its scope, allowed paths,
acceptance criteria, dependencies, and required evidence. If the requested
change conflicts with an approved decision, stop and open a change proposal.

## Pull requests

Use a short-lived branch and open a pull request linked to exactly one primary
work package. Draft PRs are welcome during long work. Mark a PR ready only
after the required checks and evidence are complete.

All PRs must pass CI. Changes to workflows, agent instructions, contracts,
security-sensitive code, architecture/product documents, and dependency or
runtime authority require `@sinanganiz` CODEOWNER review.

Only squash merging is enabled. Agents and automation do not merge PRs.

## Local validation

At minimum run:

```text
git diff --check
npm run format:check
```

If a command is not yet available in the current bootstrap stage, report that
fact explicitly rather than claiming it passed.
