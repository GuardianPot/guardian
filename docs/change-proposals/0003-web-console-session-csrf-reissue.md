# Change proposal 0003: Web Console CSRF re-issue and sensitive-action step-up

- Status: APPROVED
- Owner decision: `@sinanganiz` approved on 2026-09-04.
- Affected decision IDs: `W11-C3-A`, `IA-04`, `IA-05`, `SA-11`, `WC-D08`
- Affected acceptance criteria: `P1-W11` acceptance "TOTP and one-time secrets
  are handled without browser persistence leakage"; the Phase 1 browser
  onboarding hard-reload reauthentication step
- Work package: `WCX-09`

## Problem and context

`W11-C3-A` binds the console to a deliberate rule: the synchronizer CSRF proof
lives only in React memory, so a hard reload restores a read-only session and
any mutation or logout requires full reauthentication — username, password,
and a fresh MFA proof.

In Phase 1 that rule costs almost nothing. The console exposes three
mutations: create environment, rename environment, create zone, plus
enrollment-token creation and logout. An operator reloads rarely.

Phases 2 through 4 change the arithmetic. `WCX-09` adds device disable and
revoke, enrollment-token revoke, zone edit and delete, session revoke, and
password change. `WCX-11` adds decoy deploy, remove, enable, disable, and
configuration update. `WCX-14` adds disposition, suppression approval, and
notification acknowledgement. An operator triaging an incident will reload,
open a second tab, or return to a suspended tab many times in a working
session, and each of those currently costs a full password and MFA entry.

The predictable failure is behavioural rather than technical: an operator who
finds reloading expensive stops reloading. In a product whose value is timely
detection, training the operator to avoid refreshing works directly against
product correctness. `OPS-03` and the `WCX-04` staleness treatment mitigate
stale reads but do not remove the incentive.

There is a second, independent gap. The console today has no concept of
step-up reauthentication. Once a CSRF proof exists, every mutation is equally
available. `WCX-09` introduces genuinely irreversible actions — device revoke,
session revoke, password change — that deserve a stronger gate than a routine
configuration edit, and no such gate exists.

`IA-06` and `AUTH-05` keep fine-grained RBAC out of the MVP, so authorization
cannot supply that gradient. Reauthentication is the available mechanism.

## Options

1. **Option A — preserve `W11-C3-A` unchanged.** Every reload continues to
   require full reauthentication before any mutation or logout.

   Advantages: no change to an approved implementation contract; the strongest
   available guarantee that a mutation follows a recent, complete
   authentication; nothing to review, test, or roll back.

   Disadvantages: friction grows with every capability package and is worst
   exactly during incident triage, when reloading matters most; no gradient
   between a routine edit and an irreversible trust decision, so
   `W11-C3-A` protects a zone rename exactly as strongly as a device
   revocation; the deterrent against reloading is a real product risk.

2. **Option B — add CSRF re-issue plus step-up reauthentication.** Add
   `POST /v1/auth/csrf`, which accepts a valid, unexpired session cookie and
   returns a new synchronizer CSRF proof without requiring MFA. Separately,
   require a fresh password and MFA proof immediately before each
   irreversible-security action, scoped single-use to that action.

   Advantages: removes friction from routine work while adding a gate that
   does not exist today for the actions that most deserve one; the security
   posture on irreversible actions strictly improves; the console can drop the
   pattern where a rename and a revocation are equally guarded.

   Disadvantages: under an assumed cross-site scripting foothold, an attacker
   who can run script in the console origin can call the re-issue endpoint and
   obtain a proof, so the CSRF proof stops functioning as an incidental second
   barrier. This is a genuine reduction, and it must be accepted knowingly
   rather than reasoned away. It is bounded by the approved content-security
   policy — `script-src 'self'`, no `unsafe-inline`, no `unsafe-eval`, no raw
   HTML rendering — and by the `WCX-06` untrusted-content contract, under
   which such a foothold already implies a systemic failure. It is also a new
   authenticated endpoint, and therefore new surface requiring rate limiting,
   auditing, and review.

3. **Option C — time-boxed re-issue.** As Option B, but the re-issue endpoint
   accepts a session only within a fixed window after its last successful MFA,
   for example sixty minutes, and requires full reauthentication afterwards.

   Advantages: bounds how long a session can keep renewing write access
   without a fresh MFA proof.

   Disadvantages: unpredictable behaviour from the operator's point of view —
   the same action sometimes prompts and sometimes does not, with no visible
   reason — which is a poor property for a security tool; adds a second
   expiry concept alongside the existing 15-minute idle and 8-hour absolute
   limits; the added value over Option B is small once step-up already guards
   the irreversible actions.

## Recommendation

**Option B**, subject to the constraints below. These are part of the
recommendation, not implementation detail; approving Option B without them
would not carry the reasoning above.

1. The re-issue endpoint requires a valid, unexpired, unrevoked session
   cookie and nothing else. It returns only a new CSRF proof and never any
   credential, token, or recovery material.
2. It does **not** extend the absolute session lifetime and does not reset the
   idle timer beyond the normal last-seen update that any authorised request
   already performs. The approved 15-minute idle and 8-hour absolute limits
   are unchanged.
3. It is rate limited on the same persistent account and source throttle the
   login path uses, and it emits an audit event under `AUTH-06`.
4. The re-issued proof is held in memory exactly as today. Browser storage,
   URLs, query caches, logs, analytics, screenshots, traces, and videos remain
   excluded, so the `W11-C3-A` secret-lifetime rules survive intact.
5. Step-up reauthentication requires the operator's password and a fresh MFA
   proof — TOTP or an unused recovery code. It cannot be satisfied by the
   session cookie alone, cannot reuse a cached proof, and does not extend the
   absolute session lifetime.
6. The step-up result is single-use and scoped to one action. It is cleared as
   soon as that action completes or fails, and a second irreversible action
   requires a second step-up.
7. The irreversible-security action set is explicit and closed: device revoke,
   session revoke, and password change. Adding an action to that set is a
   package-level decision recorded in the `WCX-04` confirmation table, not an
   implementation judgement.

Evidence required before approval can be considered discharged: endpoint tests
for session validity, expiry, and revocation; proof that the absolute lifetime
is unchanged; rate-limit evidence; audit-record evidence; a browser test
showing that a level-3 action is unreachable without a successful step-up; and
confirmation that a cancelled or failed step-up issues no request.

## Impact

- **Product scope:** no new product capability. Routine mutation work after a
  reload becomes possible without full sign-in; irreversible actions become
  harder than they are today. The console's set of operations is unchanged.
- **Architecture:** one additive REST endpoint on the existing authenticated
  session surface. No new session mechanism, credential store, token type, or
  transport. `IA-05` server-side sessions with an HttpOnly cookie are
  unchanged.
- **Contracts and data:** `openapi/guardian.yaml` gains one path. No existing
  operation's request, response, or semantics change. No schema is modified.
  No database migration is required beyond whatever the audit vocabulary needs
  for the new event type.
- **Security and trust boundary:** the trust boundary is unchanged —
  authorization remains entirely a Control Plane responsibility. The change
  trades one incidental barrier under an assumed script-execution foothold for
  an explicit, currently absent gate on irreversible actions. Both sides of
  that trade are stated plainly in Option B and neither should be discounted.
- **Operations and release:** no deployment topology, configuration, or
  runbook change beyond documenting the two new flows. No new dependency.

## Rollout and failure behavior

The repository development compatibility policy permits forward-only change,
so no compatibility shim is required. The endpoint is purely additive: an
older console build never calls it, and the existing full-sign-in path
continues to work unchanged.

`WCX-09` is written so that this proposal is not a hard blocker for the whole
package. If the proposal is declined, `WCX-09` still delivers the read-only
screens and the level-1 and level-2 actions, and the level-3 actions are
omitted rather than shipped without a step-up gate. If the proposal is
approved but the endpoint later proves problematic, removing it returns the
console to today's behaviour with no data migration and no contract cleanup
beyond deleting one path.

Failure behavior of the flows themselves: a failed re-issue leaves the session
read-only and offers full sign-in; a failed or cancelled step-up issues no
request and leaves the target object unchanged; a rate-limited re-issue or
step-up renders the `rate-limited` error from the `WCX-02` taxonomy with a
wait indication and never retries automatically.

## Owner decision record

- Decision: APPROVED
- Decided by: `@sinanganiz`
- Date: 2026-09-04
- Rationale: Option B approved: CSRF re-issue endpoint plus single-use step-up reauthentication for device revoke, device re-enroll, session revoke, and password change, subject to every constraint in the Recommendation section.

Context for the decision: the Product Owner accepted `WC-D08` Option B in the
Web Console decision register on 2026-09-03. That acceptance closed the
architecture question. It does not by itself record the change-proposal
approval, because `W11-C3-A` is an approved implementation contract and only
`@sinanganiz` can record `OWNER DECISION` and `APPROVED` under the change
proposal lifecycle. This record remains `PENDING` until that entry is made.
