# Change proposal 0008: lightweight delivery process

- Status: APPROVED
- Owner decision: `@sinanganiz` approved on 2026-09-05.
- Affected decision IDs: `BR-09`, `BR-22`, `WP-12`, `PM-05`, `PM-19`, `PM-24`,
  `AP-01`, `AP-02`, and the change-proposal lifecycle in
  `docs/change-proposals/README.md`
- Affected acceptance criteria: none
- Work package: `WCX-000`

## Problem and context

The Step 7 governance model was written for a team. This project is one
Product Owner directing one agent, and several rules cost a round trip each
without adding review. Observed in one week of `WCX` delivery: five pull
requests for one planning package, a full options-and-impact change proposal
for decisions the owner had already made verbally, and a merge sequence that
required the owner to run a command and report back before work could
continue.

## Options

1. **Keep Step 7 as written.** Maximum traceability, and the friction is real
   and repeated.
2. **Cut the steps that duplicate an owner decision already recorded
   elsewhere, and automate the merge handoff.** Keep every gate that catches
   something a human would otherwise miss.
3. **Suspend the work-package process during Phase 2.** Fastest, and it loses
   the specification discipline that keeps the agent from inventing scope.

## Recommendation

**Option 2.** The changes, each with what it replaces:

1. **Auto-merge enabled** (`BR-22` amended). The owner approves once; GitHub
   merges when `PR checks` is green. Replaces: agent writes a merge command,
   owner runs it, owner reports back.
2. **Branch protection without strict up-to-date** (`BR-09` amended to
   non-strict). Replaces: rebasing a branch after every unrelated merge to
   `main`, which cost one pull request outright.
3. **`DONE` is merged plus green `PR checks` plus the owner saying accepted**
   (`WP-12`, `PM-19`, `PM-24` simplified). Replaces: a separate
   `AC-VALIDATION` state and a distinct acceptance record.
4. **Project fields suspended** (`PM-05` suspended, not deleted). Eleven
   fields on twenty-one issues answer questions a single owner already knows.
   They return when more than one work stream runs in parallel.
5. **Owner-directed decisions use a short record.** When the owner directs a
   change, it is recorded as a dated decision with its rationale and its
   accepted risk, not a full options-impact-rollout proposal. The full format
   stays required when the **agent** proposes changing an approved decision,
   because that is where the owner needs the alternatives laid out.
6. **`requires_security_review` only where a trust boundary moves.** It was
   set on fourteen of twenty-one `WCX` packages; it belongs on the ones that
   touch authentication, secrets, hostile content, privileged surfaces, or the
   Control Plane.
7. **Repository settings may be changed on a written owner directive**
   (`AP-01`, `AP-02` amended), recorded here or in a later proposal. The agent
   still cannot merge, release, or touch secrets.

Unchanged and deliberately kept: `PR checks` on every pull request, the
fourteen mandatory work-package sections (`WP-05`), CODEOWNERS protection of
`security/`, `docs/adr/`, `docs/phase-gates/`, and agent instruction files, and
`AP-08`, which keeps merge authority away from the agent.

## Impact

- **Product scope:** none.
- **Architecture:** none.
- **Contracts and data:** none.
- **Security and trust boundary:** `AP-08` still denies merge. Settings changes
  are now possible on a written directive, which is a real widening of agent
  authority and the reason it is written down here rather than assumed.
  Auto-merge cannot merge a red pull request, because `PR checks` is a
  required check.
- **Operations and release:** the owner performs one approval per pull request
  instead of an approval plus a merge command plus a report.

## Rollout and failure behavior

Applied on 2026-09-05 by the Product Owner's directive:

- repository `allow_auto_merge` set to true;
- branch protection created on `main` with required check `PR checks`,
  `strict` false, linear history required, force pushes and deletions blocked,
  no required approving reviews, and admin enforcement off so the owner keeps
  the `BR-19` emergency bypass.

Reverting is deleting the branch protection and setting `allow_auto_merge`
back to false.

**The accepted risk:** with no required approving review, a pull request can
merge on a green check without a human reading it. That is the intended trade
for a solo project, and it is bounded by `AP-08`: the agent cannot approve or
merge, so a human still starts every merge.

`BR-15` CODEOWNER review is **not** enforced by this configuration. It remains
a manual practice. Turning it on requires a required approving review count of
at least one, which reintroduces a per-pull-request approval step; the owner
can enable it at any time.

## Owner decision record

- Decision: APPROVED
- Decided by: `@sinanganiz`
- Date: 2026-09-05
- Rationale: the process was written for a team and this is one owner and one
  agent. The steps removed duplicated a decision already recorded elsewhere or
  existed to coordinate people who are not present. The gates that catch real
  defects were kept.
