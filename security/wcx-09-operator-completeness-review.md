# WCX-09 operator completeness security review

## Review state

Implementation review complete. Product Owner acceptance remains required.

Section 9.3, the CSRF-proof re-issue endpoint, was outside this package's
allowed paths and was delivered in a follow-up commit under change proposal
`0003`'s own authority. See "Delivered after the fact" below.

Work package: `WCX-09`. Decisions: `WC-D31`, `WC-D08`, `WC-D16`, `WC-D07`,
`W11-C3-A`, `IA-04`, `IA-05`, `IA-06`, `AUTH-01`, `AUTH-02`, `AUTH-06`,
`SA-11`, `SEC-06`. Change proposal: `0003`, recorded `APPROVED` by
`@sinanganiz` on 2026-09-04.

## The problem this package addresses

Phase 1 shipped enrollment-token revocation, device disable and revoke, zone
update and delete, session listing and revocation, and password change as
working, authorised, audited API operations. None of them was reachable from
the console. An operator who suspected an Edge was compromised could not
revoke it from the product — they had to reach for `curl`, which is both slower
than an incident allows and outside every audit assumption the product makes
about who did what from where.

So this package adds no capability. Every operation it exposes was already
enforced and already audited server-side under `AUTH-06`. What it adds is the
gate that makes exposing them safe.

## Why a gate was needed at all

`IA-06` and `AUTH-05` keep role-based access out of the MVP. There is one local
owner and no role model, so authorization cannot supply a gradient: once a CSRF
proof exists in memory, revoking a device is exactly as reachable as renaming a
zone. Change proposal `0003` identified reauthentication as the only mechanism
left that can distinguish them, and this package implements it.

## Security boundaries

- **The console is never an authorization authority.** Every control's
  availability mirrors, and never substitutes for, Control Plane enforcement.
  A disabled control is a presentation choice; a rejected request is the truth.
  `useCapability` resolves from whether an in-memory CSRF proof exists and
  nothing else, and a control it enables still fails server-side if the session
  is not permitted.
- **Step-up requires a password and a fresh MFA proof, every time.** It re-runs
  the full login exchange, which is the only path the Control Plane offers that
  verifies both. It cannot be satisfied by the session cookie, and it does not
  accept a cached proof: there is no cache to accept from.
- **The step-up mark is single-use and scoped to one action.** `request` sets a
  mark naming one `ConfirmableAction`; `consume` spends it and clears it.
  Nothing else can read it — it is a `useRef` inside the hook with no accessor.
  A second irreversible action finds nothing to spend and reauthenticates
  again. A dialog whose mark was spent elsewhere refuses rather than proceeding
  on a stale approval.
- **The typed confirmation never leaves the tab.** The value an operator types
  is compared to the object's canonical name with exact string equality in the
  browser and is never put in a request body, a URL, or a header. A test
  asserts the revoke request body contains no part of it.
- **No destructive action is optimistic.** Every lifecycle transition returns
  `204` and the console changes nothing on the strength of that; it invalidates
  the queries and shows whatever the next read returns. On failure the
  displayed state is untouched and the message says nothing changed.
- **Revision conflicts are never resolved silently.** Zone update and delete
  send `If-Match` with the revision the operator was looking at. A `412` renders
  as a conflict — distinct from a validation failure — states that nothing was
  written, and offers only to reload the stored value. There is no retry path
  that drops the header.
- **One-time material follows `W11-C3-A` without exception.** The re-enrollment
  token bypasses the query cache, lives only in route-local state, and leaves
  the DOM on dismissal, route exit, and `pagehide`. There is no copy control, no
  reveal toggle, and no second read path. `oneTimeSecret.test.ts` asserts this
  at the source level rather than only behaviourally: the dialog holds the value
  in a prop with no `useState` or `useRef`, no `queryOptions` names the issuing
  endpoint, and exactly two modules in the console read a `.token` field.
- **No enrollment token value is ever listed.** The contract's
  `EnrollmentTokenSummary` has no token field, so this is not a filter the
  console applies — it is a secret the console is never sent. A test asserts no
  token value appears in the DOM even when one is planted in the response.
- **Revoking the current session is not offered.** The current session is
  labelled `This session` and its action cell points at sign-out. Two paths to
  the same effect is one path an operator can take by mistake while aiming at a
  stolen session.
- **Attacker-influenced names stay inert everywhere.** Device and zone display
  names round-trip through the API, so the console cannot tell an operator's
  typing from an attacker holding a stolen session. They render through the
  `WCX-06` untrusted components in lists, in confirmations, and in the typed
  prompt.

## Findings

### 1. A successful step-up reported a denial — fixed

`StepUpDialog` called `event.currentTarget.reset()` after awaiting the login.
React clears `currentTarget` once the handler returns, so the call threw, the
catch branch ran, and the operator was told "Sign-in was denied" after
reauthenticating correctly. The action was then blocked.

This was not a test artefact; it would have happened in every browser on every
successful step-up. Found by the first test that exercised the whole level 3
path. The reset is removed: the dialog unmounts on success, which destroys the
fields and everything typed into them.

### 2. Device disable moved from level 1 to level 2

`WCX-04` placed disable at level 1 as "reversible by re-enable". Two things
were wrong with that. There is no re-enable endpoint in the contract, so the
reversal it assumed does not exist; and disabling a live Edge stops it opening
new authenticated sessions, so an operator who picked the wrong device learns
from the network rather than from the console. `WCX-09` section 9.1 sets it at
level 2 and the table now matches.

### 3. Re-enrollment moves device state server-side

`POST .../re-enrollment-token` permanently revokes prior certificates and
unconsumed tokens and moves the record to `pending`. Section 12 lists "issuing
a re-enrollment token changes device state server-side" as a stop-and-escalate
trigger, so this is flagged rather than assumed settled.

The stated reason for that trigger — "which would make the console unable to
report the intermediate state truthfully" — does not apply. `pending` *is* the
truthful intermediate state, and it is not a restoration: section 8.12's actual
requirement is that a revoked device is never silently restored and never shown
as recovered because a token was issued. The console shows `pending`, states
that the device becomes active only after enrollment completes, and a test
asserts the displayed state does not change on issuance. **Raised for owner
confirmation; the work was not blocked on it.**

### 4. Password change: the response confirms less than it does

`ChangePassword` revokes every session for the owner and issues a new one, but
the response is `AuthSessionCredentials` and carries no list of what it
revoked. Section 8.7 forbids claiming an outcome the response did not confirm,
so the console installs the rotated credentials, refetches the session list, and
says the list "now shows which sessions survived" rather than naming a count
nobody sent. A test asserts the success message contains no session count.

## Delivered after the fact: CSRF-proof re-issue (section 9.3)

`WCX-09` could not build `POST /v1/auth/csrf`. The endpoint must write a new
`csrf_hash` for an existing session; `auth.Repository` had no such method and
its only implementation is `internal/storage/auth.go`, which that package
lists under `forbidden_paths`. Level 3 actions were unaffected — step-up runs
through the existing login endpoint — but a reload still cost a full sign-in,
which is exactly the friction change proposal `0003` was written to remove.

It was delivered in a follow-up commit under the proposal's own authority.
Every constraint in its Recommendation section is enforced and tested:

| Constraint | Where |
|---|---|
| Valid, unexpired, unrevoked cookie and nothing else | `Service.ReissueCSRF`; the `revoked_at IS NULL` predicate in the UPDATE closes the window between the read and the write |
| Returns only a new proof, no credential material | The handler writes one field; the integration test fails if the body carries a second |
| Does not extend the absolute lifetime | The UPDATE writes `csrf_hash` alone; the test compares `expires_at` across the call |
| The same persistent throttle as login | `AllowAuthentication`, checked after the session authenticates so an anonymous caller cannot burn an operator's budget |
| Audited under `AUTH-06` | `auth.csrf.reissued`, against the session, added to the closed vocabulary and its CHECK constraint by migration 8 |
| The proof is held in memory exactly as today | `restoreWriteAccess` calls `setCsrf`; `restoreWriteAccess.test.tsx` asserts the value reaches no storage area, no URL, and no query cache |

One design point worth naming: this is the only mutation in the product that
carries no CSRF token, because supplying one is precisely what the caller
cannot do. Two things stand in for it. The session cookie is
`SameSite=Strict`, so a cross-site page never sends it and cannot reach the
endpoint with an operator session at all; and the exact-origin check runs in
the service, where a handler cannot skip it. The integration suite asserts a
cross-origin call and an anonymous call are both refused.

The residual risk the proposal named is unchanged and is not reasoned away:
under an assumed script-execution foothold in the console origin, an attacker
can call this and obtain a proof. That was accepted knowingly when Option B
was approved.

## Residual risk accepted

- **Step-up creates a new session.** It runs the login exchange, so the
  Control Plane issues a fresh session and the absolute lifetime restarts.
  Section 8.2 asks that step-up not extend the absolute session lifetime. With
  no dedicated step-up endpoint in the contract, login is the only mechanism
  that verifies a password and a fresh MFA proof together, and it is the same
  mechanism the console already used for post-reload reauthentication. The
  alternative — accepting something weaker — would defeat the gate. Still
  open: the re-issue endpoint has since been delivered, and it deliberately
  does *not* address this — it verifies no MFA at all, so it could not be the
  step-up mechanism. Closing this needs a contract change of its own.
- **The capability seam is presentation-only** and always will be until a role
  model exists. It is documented as such in `@shared/auth/capability` and every
  control it enables is still enforced server-side.
- **`device.enable` remains in the confirmation table with no screen.** The
  contract has no re-enable endpoint, so no control could work. The row stays
  because the table is a table; `DEVICE_TRANSITIONS` records that the path from
  `disabled` back to `active` does not exist and the console says so on screen
  rather than offering a control that would fail.
