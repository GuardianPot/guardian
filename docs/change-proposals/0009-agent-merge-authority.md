# Change proposal 0009: agent merge authority

- Status: APPROVED
- Owner decision: `@sinanganiz` approved on 2026-09-05.
- Affected decision IDs: `AP-08`, `AP-14`, `BR-15`, `CI-26`
- Affected acceptance criteria: none. Every gate that guarded a merge before
  still guards it; this changes who presses the button once they are green.
- Work package: `WCX-000`

## Problem and context

`AP-08` denied merge authority to the agent outright, and `AGENTS.md` carried
the matching prohibition — "merge or bypass a pull request". The tool layer
enforced it independently: `.claude/settings.local.json` denied
`gh pr merge` in both shells, so the command could not be issued even when
directed.

Change proposal `0008` cut the delivery process to a single reviewer and
`0007` cut pull-request CI to under two minutes. What remains between a green
check and a merged commit is a manual hand-off: the agent reports the run is
green, the Product Owner returns to the terminal and pastes a command that has
no decision left in it. On 2026-09-05 the Product Owner directed that this
hand-off be removed.

The prohibition was written when the agent's output was unproven and branch
protection did not exist. Both conditions have changed. Since `0008` the
repository has required the `PR checks` status, linear history, and a pull
request for every change to `main`, with force-push and deletion blocked.

## Options

1. **Keep `AP-08` as written.** No new authority. Every package ends with the
   agent idle, waiting for a paste. The rule reads as a safety control but no
   longer buys a decision, because the owner does not re-review at the merge
   step — they merge on the strength of the green check they were shown.
2. **Grant merge authority, bounded by branch protection.** The agent may
   squash-merge a pull request it opened, once every required check has passed
   and the branch is mergeable. Direct pushes to `main`, administrator
   overrides, and any change to the protection rules themselves stay denied.
3. **Grant merge authority with no bound.** Allow `--admin`, allow merging
   while checks are pending. Removes the control entirely.

## Recommendation

**Option 2.** The safety property worth keeping is not "a human presses
merge"; it is "nothing reaches `main` that has not passed every required
check". Branch protection enforces that at the server for every non-admin
actor, and `AP-08` was a second, weaker copy of the same guarantee, paid for
with a hand-off per package.

**One gap, stated rather than glossed:** `main` is currently configured with
`enforce_admins: false`, and the agent operates with the owner's token, which
is an administrator. The server would therefore accept an administrator
override. Nothing in the tool layer can reliably prevent that either — a deny
pattern on `gh pr merge --admin` is defeated by flag ordering, and the same
merge is reachable through `gh api`. The bound in this proposal is therefore
a rule the agent follows, not a control the server enforces, until the owner
sets `enforce_admins: true`. That change is recommended and is not made here,
because it constrains the owner as well as the agent and is theirs to choose.

`AP-08` becomes:

> **ALLOW**, bounded. The agent may squash-merge a pull request it opened when
> every required check has passed and the branch is mergeable without an
> override. `--admin`, `--merge`, `--rebase`, force-merge of a pending or
> failing check, and any change to the branch protection rules stay **DENY**.
> Merging does not constitute acceptance: the work package's acceptance
> evidence is still recorded and still approved by the owner.

The last sentence is the part that matters. Merge and acceptance were already
separate under `AP-07` for issues; this keeps them separate for pull requests.
A merged commit is not an accepted work package.

## Impact

- **Product scope:** none.
- **Architecture:** none.
- **Contracts and data:** none.
- **Security and trust boundary:** unchanged in substance. Every required
  check still runs and still blocks. The bypass paths — direct push,
  administrator override, protection-rule edits, force push — remain denied in
  `AGENTS.md`, in `AP-14`, and in the tool-level deny list, which keeps
  `gh repo edit`, `gh ruleset`, `gh secret`, and `git push --force`. What the
  agent gains is the ability to act on a result it is already permitted to
  read.
- **Operations and release:** `AP-09` through `AP-11` are untouched. Release
  creation, production deployment, and signing keys stay denied. Merge
  authority does not extend to a tag or a release.

## Rollout and failure behavior

Three coordinated edits, all in this change:

1. `0-planning-documents/step-7-.../03_AI_Coding_Agent_Strategy_and_Permissions.md`
   — `AP-08` restated as above.
2. `AGENTS.md` — the "merge or bypass a pull request" bullet narrowed to
   "bypass branch protection, merge a pull request whose required checks have
   not passed, or use an administrator override to merge".
3. `.claude/settings.local.json` — the four `gh pr merge` deny entries
   replaced by a narrower pair denying `gh pr merge --admin`. Every other deny
   entry stays, including `gh repo edit`, `gh ruleset`, `gh secret`, and
   `git push --force`. The `--admin` pattern is a tripwire, not a control: see
   the gap noted under Recommendation.

Reverting is restoring those three files.

**The accepted risk, stated plainly:** a defect that `PR checks` does not
catch now reaches `main` without a second person having looked at the diff at
the moment of merge. Under `0007` that class already includes Go integration,
container, contract, and browser regressions, which are caught on the push to
`main` or nightly. `0008` had already reduced the merge step to a formality
for a solo project; this records that formality accurately rather than
pretending a control exists.

The correction, if defects start reaching `main`, is to require a review from
`CODEOWNERS` in branch protection — `BR-15`, which is specified and currently
left off. That is a stronger control than `AP-08` was, because it blocks the
merge at the server rather than relying on the agent obeying a rule.

## Owner decision record

- Decision: APPROVED
- Decided by: `@sinanganiz`
- Date: 2026-09-05
- Rationale: branch protection already enforces what `AP-08` was protecting,
  and the manual merge step had become a hand-off with no decision in it. The
  bypass paths stay denied, and merging remains separate from acceptance.
