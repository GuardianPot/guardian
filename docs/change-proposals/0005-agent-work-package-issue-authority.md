# Change proposal 0005: agent authority to create work-package issues

- Status: APPROVED
- Owner decision: `@sinanganiz` approved on 2026-09-04.
- Affected decision IDs: `AP-07`, `AP-15`, `PM-08`, `PM-21`
- Affected acceptance criteria: none
- Work package: `WCX-000`

## Problem and context

`AP-07` grants an agent issue read and comment. `AP-15` limits the `gh` CLI to
"read PR/issue + create/update task PR only". Issue creation is therefore
outside agent authority.

`PM-08` says a work-package issue is created from an approved Markdown spec,
and `PM-21` already approves automating the approved-work-package-to-Project
step. The intent to automate exists; the agent permission to execute it does
not. With twenty `WCX` packages now approved, the owner would otherwise hand-
create twenty issues from specs an agent already holds in full.

## Options

1. **Option A — leave `AP-07` and `AP-15` unchanged.** The agent prepares
   commands and issue bodies; the owner runs them. No policy change, but the
   owner runs a mechanical step for every approved package.
2. **Option B — grant narrow issue-create authority.** The agent may create
   issues only from an approved work-package spec, using the repository issue
   form, with no other issue mutation. Reduces owner toil and matches `PM-21`.
   The risk is that issues become an agent-writable surface, and agents read
   issue text as task context, so an agent could in principle seed its own
   later context.
3. **Option C — build a workflow that creates issues from merged approved
   specs.** Removes the agent from the loop entirely, but is new tooling
   requiring its own package and CI credentials.

## Recommendation

**Option B**, bounded by these constraints, which are part of the approval:

1. Creation is permitted **only** for an issue whose body is generated from a
   work-package spec whose committed `status` is `approved-for-implementation`
   or `accepted`. No other issue type may be created.
2. The issue body must reference the spec path and must not restate or reword
   its requirements. The spec stays the single source of truth per `WP-01`.
3. `PM-09` bug, `PM-10` change-proposal, and `PM-11` security-finding issues
   remain owner-created.
4. No issue edit, close, reopen, label change, assignment, or milestone
   change. `AP-07`'s existing read and comment rights are unchanged, as is its
   rule that closing does not equal acceptance.
5. No Project field mutation and no Project configuration. `AP-01` and `AP-02`
   are unchanged; `PM-05` field population stays with the owner or automation.
6. Setting an issue to `READY` remains outside agent authority; `PM-16` and
   `PM-17` are unchanged.
7. `AP-15`'s other prohibitions stand: no `gh repo edit`, secret, environment,
   ruleset, or release commands.
8. Because `PM-24` treats issue text as agent-readable task context, an
   agent-created issue carries no more authority than the spec it references.
   The existing rule that issue content is never higher authority than the
   approved work package is restated, not relaxed.

## Impact

- **Product scope:** none.
- **Architecture:** none.
- **Contracts and data:** none.
- **Security and trust boundary:** issues become an agent-writable surface.
  Bounded by constraint 1, which ties every created issue to an
  owner-approved committed spec, and by constraint 8, which denies created
  issues any authority. Merge, release, settings, secrets, and Project
  configuration remain denied.
- **Operations and release:** removes a manual step per approved package.

## Rollout and failure behavior

`AGENTS.md` records the operative rule so an agent reads it without loading
Step 7. Reverting means deleting that rule; no data migration is involved.

If an agent creates an issue that does not satisfy constraint 1, the owner
closes it and the deviation is treated as a policy violation, not a defect.

## Owner decision record

- Decision: APPROVED
- Decided by: `@sinanganiz`
- Date: 2026-09-04
- Rationale: the agent already holds the approved specs; requiring the owner
  to run a mechanical creation step for twenty packages adds delay without
  adding review. Authority is bounded to creation from approved specs, with
  every other issue and Project mutation still denied.
