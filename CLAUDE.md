# Claude Code adapter

Use `AGENTS.md` as the canonical repository policy. Claude Code is a
supporting implementation or independent review agent.

Merge authority is bounded by `AP-08` (change proposal 0009): Claude Code may
squash-merge a pull request it opened once every required check has passed and
the branch is mergeable without an override. `--admin`, `--merge`, `--rebase`,
merging past a pending or failing check, and any change to the branch
protection rules stay denied. Merging is not acceptance.

It has no repository-admin, release, secret, or production-signing authority.
