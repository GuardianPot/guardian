---
id: WCX-09
phase: 2
wave: capability
title: Operator completeness and sensitive-action reauthentication
status: approved-for-implementation
risk: high
components:
  - web-console
  - control-plane
decision_refs:
  - WC-D31
  - WC-D08
  - WC-D16
  - WC-D07
  - W11-C3-A
  - IA-04
  - IA-05
  - IA-06
  - AUTH-01
  - AUTH-02
  - AUTH-06
  - SA-11
  - SEC-06
blocked_by_change_proposal:
  - CP-0003
acceptance_refs:
  - WCX-000 section 3.7
  - Phase 1 gate exit evidence "Edge enrollment, rotation, and revocation"
  - SEC-06 revoked Edge
depends_on:
  - WCX-04
  - WCX-06
  - WCX-08
integration_dependencies:
  - CP-0003 approved by the Product Owner
allowed_paths:
  - "apps/web-console/src/**"
  - "apps/web-console/package.json"
  - "package-lock.json"
  - "tests/e2e/web-console/**"
  - "security/wcx-09-operator-completeness-review.md"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-09.md"
allowed_paths_gated_on_cp_0003:
  - "openapi/guardian.yaml"
  - "apps/control-plane/internal/api/authentication.go"
  - "apps/control-plane/internal/api/server.go"
  - "apps/control-plane/internal/api/authentication_test.go"
  - "apps/control-plane/internal/api/server_test.go"
  - "apps/control-plane/internal/auth/service.go"
  - "apps/control-plane/internal/auth/service_test.go"
forbidden_paths:
  - "apps/edge-agent/**"
  - "proto/**"
  - "apps/control-plane/internal/storage/**"
  - "apps/control-plane/internal/devicechannel/**"
  - "apps/control-plane/internal/devicepki/**"
  - "0-planning-documents/**"
requires_owner_review: true
requires_security_review: true
---

# WCX-09 — Operator completeness and sensitive-action reauthentication

## 1. Purpose

Surface the security-critical Control Plane capabilities that already exist
but have no console interface, and add the step-up reauthentication that makes
them safe to expose.

## 2. Why now

Phase 1 shipped enrollment-token revocation, device disable and revoke, zone
update and delete, session listing and revocation, and password change as
working, authorised, audited API operations. None of them is reachable from
the console. An operator who suspects an Edge is compromised cannot revoke it
from the product. The Product Owner placed this work at Phase 2 start on
2026-09-03 and confirmed that the Phase 1 gate may close on capability
evidence.

## 3. Inputs and decisions

- `WC-D31` — add operator completeness covering device disable and revoke,
  enrollment-token list and revoke, zone edit and delete, session list and
  revoke, and password change. Organization-level audit viewing is deferred to
  Phase 3.
- `WC-D08` — add CSRF-proof re-issue without full MFA, and require step-up
  reauthentication for sensitive actions. Routed through change proposal
  `0003`.
- `WC-D16` — three confirmation levels; these screens are the first real users
  of level 3.
- `WC-D07` — controls are disabled with a reason, never hidden.
- `AUTH-06` — every one of these operations is already audited server-side.

## 4. Dependencies

`WCX-04` for confirmation levels and data states, `WCX-06` for untrusted
rendering of device and zone names inside confirmations, `WCX-08` for wording
and timestamps. **Change proposal `0003` must be recorded as `APPROVED` by the
Product Owner before any work begins on the reauthentication path or on the
gated backend paths.** The read-only and level-1 and level-2 parts of this
package may proceed without it; the level-3 actions may not.

## 5. Scope

1. Device lifecycle controls: disable, re-enable where the API allows, revoke,
   and re-enroll, with correct confirmation levels.
2. Enrollment-token management: list active tokens with expiry and revoke.
3. Zone management: rename, edit CIDR where the API allows, and delete.
4. Session management: list sessions with creation, last-seen, and expiry, and
   revoke a session other than the current one.
5. Password change.
6. Step-up reauthentication and CSRF-proof re-issue, per change proposal
   `0003`.
7. Extension of the confirmation action-to-level table.

## 6. Non-goals

- No organization-level audit event viewer. `WC-D31` defers it to Phase 3.
- No bulk device operations and no automatic re-enrollment. Re-enrollment is
  operator-initiated, one device at a time.
- No role model, no user management, no second account, no OIDC.
- No bootstrap flow. Initial owner bootstrap remains a deliberate out-of-band
  ceremony and must not become a console route.
- No recovery-code regeneration or display.
- No change to session lifetime, idle or absolute expiry, throttling, TOTP, or
  revocation semantics.
- No decoy, incident, or AI capability.

## 7. Allowed paths

Console, browser test, security review, runbook, and package paths are always
allowed. The `allowed_paths_gated_on_cp_0003` set becomes writable **only**
after the Product Owner records `APPROVED` on change proposal `0003`, and only
for the CSRF re-issue endpoint and its step-up semantics. Storage, device
channel, and device PKI remain forbidden throughout.

## 8. Security constraints

1. The console never becomes an authorization authority. Every control's
   availability mirrors, and never substitutes for, Control Plane enforcement.
   A disabled control is a presentation choice; a rejected request is the
   truth.
2. Step-up reauthentication must require the operator's password and a fresh
   MFA proof. It must not accept a cached proof, must not be satisfiable by
   the session cookie alone, and must not extend the absolute session lifetime.
3. The CSRF re-issue endpoint must require a valid, unexpired session cookie,
   must be rate-limited on the same persistent throttle the login path uses,
   must be audited, and must not extend the absolute session lifetime. It
   returns only a new CSRF proof and never any credential material.
4. The re-issued CSRF proof is held in memory exactly as today. It is never
   written to any storage, URL, cache, log, or artefact.
5. Level-3 confirmation requires the operator to type the exact object name.
   The typed value is compared to the object's canonical name and is never
   sent to the backend.
6. Revoking the current session must sign the operator out immediately and
   clear all non-session query state. Revoking another session must not
   disturb the current one. The current session must be clearly labelled and
   must not be revocable through the other-session control.
7. Password change must invalidate other sessions according to existing
   backend behaviour; the console reflects whatever the backend does and never
   claims an outcome the response did not confirm.
8. Device and zone names are attacker-influenceable and are rendered through
   the untrusted components from `WCX-06`, including inside confirmation
   dialogs and typed-confirmation prompts.
9. No enrollment token value is ever listed. The token list shows identifiers,
   device names, creation and expiry times, and state only. The one-time
   secret remains visible exactly once at creation, under existing rules.
10. A revoked device must never be presented as reachable or healthy.
    `SEC-06` behaviour is reflected, not reinterpreted. It may be presented as
    re-enrollable, because `SEC-06` names re-enrollment as a resolution state.
11. The re-enrollment token is one-time material and follows the enrollment
    secret rules from `W11-C3-A` without exception: it bypasses the query
    cache, lives only in route-local state, leaves the DOM on dismissal, route
    exit, and unload, and never enters browser storage, a URL, a log, a trace,
    a video, or a screenshot. No control may re-display it, and a source-level
    check asserts no read path exists.
12. Re-enrollment must not silently restore a revoked device. Issuing the
    token changes nothing about the device's current state; the device becomes
    active only when it completes enrollment and the backend reports it. The
    console must render the intermediate state truthfully and must never show
    a device as recovered because a token was issued.

## 9. Implementation requirements

### 9.1 Confirmation level assignment

Added to the `WCX-04` table:

| Action | Level | Rationale |
|---|---|---|
| Zone rename | L1 | reversible configuration edit |
| Device disable | L2 | reversible by re-enable, but interrupts operation |
| Enrollment-token revoke | L2 | destructive but re-issuable |
| Zone delete | L2 | destructive; may orphan configuration |
| Device revoke | L3 | irreversible trust decision |
| Device re-enroll | L3 | re-establishes device trust and issues one-time bootstrap material |
| Session revoke | L3 | immediate access removal |
| Password change | L3 | credential change |

Device re-enrollment is level 3 by Product Owner decision on 2026-09-04. It is
the inverse of revocation and re-establishes the trust that revocation
removed, so it carries the same gate: typed confirmation of the device name
plus step-up reauthentication.

### 9.2 Step-up reauthentication

A shared `useStepUp()` interface implementing the seam `WCX-04` defined.

1. Invoking a level-3 action opens a reauthentication dialog before the
   confirmation, not after.
2. The dialog collects password and one MFA proof, using the same TOTP and
   recovery-code selection the login screen offers.
3. On success the console holds a short-lived step-up marker in memory,
   scoped to a single action and cleared immediately after that action
   completes or fails. It is never reused for a second action.
4. On failure the dialog reports a generic denial that never reflects a
   submitted value, and the underlying action is not attempted.
5. Rate limiting is enforced server-side; the console renders the
   `rate-limited` error from the `WCX-02` taxonomy with a wait indication and
   never retries automatically.

### 9.3 CSRF-proof re-issue

Per change proposal `0003`:

1. On a reload-restored read-only session, the console offers a single
   explicit `Restore write access` action rather than requiring full sign-in.
2. That action calls the re-issue endpoint. On success the console holds the
   new proof in memory and enables level-1 and level-2 controls.
3. Level-3 controls remain gated by step-up reauthentication regardless of
   proof freshness.
4. If change proposal `0003` is not approved, this section is not implemented
   and the current full-sign-in behaviour stands unchanged; the rest of the
   package still delivers with level-3 actions omitted.

### 9.4 Screens and controls

Device detail gains a lifecycle section showing current inventory state, the
available transitions, and, for each unavailable transition, the reason it is
unavailable. Re-enrollment appears there as a recovery path for a device whose
certificate expired or whose revocation was later cleared, and states that its
decoys remain unmanaged until re-enrollment completes, matching the `SEC-06`
wording that `WCX-11` renders. Environment detail gains an enrollment-token panel listing active
tokens with device name, creation, expiry, and state, each with a revoke
control. Zone rows gain edit and delete controls. A new account area exposes
session list and password change.

Every control obeys `WCX-07` freshness classes: token and session lists are
`operational`, zones are `configuration`.

### 9.5 UI/UX requirements

1. Every destructive control states its effect in the operator's terms before
   it is used, not only in the confirmation.
2. A revoked device remains visible in inventory with its state and the time
   of revocation. Removing it from view would hide history.
3. The current session is labelled `This session` and its revoke control is
   replaced by the existing sign-out.
4. An expired enrollment token is shown as expired rather than removed, so an
   operator can see that a handoff window closed.
5. A failed destructive action states that nothing changed. It must never
   leave the operator unsure whether the action took effect.
6. Password change reports exactly what the backend confirmed, including
   whether other sessions ended.

### 9.6 Accessibility requirements

1. The reauthentication and confirmation dialogs meet the `WCX-04` dialog
   contract: focus trap, escape, return focus, accessible name and
   description.
2. Focus lands on cancel, never on the destructive confirm control.
3. The typed-confirmation field has a label stating exactly what to type and
   an error that names the mismatch without echoing the typed value.
4. Completion of a destructive action is announced once through the polite
   live region.
5. Lists of tokens and sessions are semantic tables with a caption and header
   cells; every row action has an accessible name that identifies its row.

### 9.7 API and data contracts

Consumed as they exist today:

| Operation | Endpoint |
|---|---|
| Disable device | `POST /v1/environments/{environmentId}/devices/{deviceId}/disable` |
| Revoke device | `POST /v1/environments/{environmentId}/devices/{deviceId}/revoke` |
| List enrollment tokens | `GET /v1/environments/{environmentId}/enrollment-tokens` |
| Revoke enrollment token | `DELETE /v1/environments/{environmentId}/enrollment-tokens/{tokenId}` |
| Update zone | `PATCH /v1/environments/{environmentId}/zones/{zoneId}` |
| Delete zone | `DELETE /v1/environments/{environmentId}/zones/{zoneId}` |
| List sessions | `GET /v1/auth/sessions` |
| Revoke session | `DELETE /v1/auth/sessions/{sessionId}` |
| Change password | `POST /v1/auth/password` |
| Re-enroll device | `POST /v1/environments/{environmentId}/devices/{deviceId}/re-enrollment-token` |

The only contract addition is the CSRF re-issue endpoint from change proposal
`0003`. No existing operation's request, response, or semantics may change.
Revisioned updates continue to send `If-Match`.

### 9.8 Error and failure behaviour

1. A revision conflict on zone update renders the `conflict` state with an
   explicit statement that another change occurred, and offers to reload the
   current value. It must never silently overwrite.
2. A `403` renders `denied`, distinct from `not-found`.
3. A destructive action that fails leaves the object's displayed state
   unchanged and states that nothing changed.
4. No optimistic update is applied to any destructive action. The displayed
   state changes only after the backend confirms.
5. Revoking the current session, then receiving success, immediately follows
   the existing expiry path.

### 9.9 Internationalisation and theme

All new text enters the `WCX-08` catalogue. Device and zone names are
untrusted values and are never treated as catalogue content. Destructive
styling uses the semantic action tokens from `WCX-03`, not severity tokens.

### 9.10 Performance

Token and session lists are bounded by the existing API limits and need no
virtualisation. These screens must not push the authenticated initial-load
budget beyond the `WCX-07` limit; the account area is its own feature chunk.

### 9.11 Observability

None in the console. Every operation here is already audited server-side under
`AUTH-06`; the console adds no client-side record.

### 9.12 Documentation

Add an `Operator lifecycle actions` section to
`docs/runbooks/web-console/development.md` covering the confirmation levels,
the step-up flow, the restore-write-access flow, and the recovery path when a
destructive action fails. Record
`security/wcx-09-operator-completeness-review.md`.

## 10. Required tests

### 10.1 Unit and component

1. Each action maps to its assigned confirmation level; a table-driven test
   covers every row.
2. A level-3 action cannot proceed without a successful step-up, proven by
   asserting no request is issued when step-up is cancelled or fails.
3. The step-up marker is single-use: a second level-3 action requires a second
   step-up.
4. The typed-confirmation control enables only on an exact match and never
   transmits the typed value.
5. A failed destructive action leaves displayed state unchanged and says so.
6. A revision conflict renders `conflict`, not `validation`, and offers
   reload.
7. Revoking another session leaves the current session intact; the current
   session exposes no revoke control.
8. Revoking the current session follows the existing expiry path exactly.
9. Hostile device and zone names render inert inside confirmation dialogs and
   typed-confirmation prompts.
10. No enrollment token value appears anywhere in the token list DOM.
11. After exercising every flow, all browser storage areas are empty.
12. A revoked device is shown with its state and revocation time and is never
    described as reachable or healthy.
13. Re-enrollment requires typed confirmation and step-up; no request is
    issued when either is cancelled.
14. The re-enrollment token is shown once, leaves the DOM on dismissal, route
    exit, and unload, and reaches no query cache or browser storage; a
    source-level check asserts no re-display path exists.
15. Issuing a re-enrollment token does not change the displayed device state;
    the device becomes active only after the backend reports completed
    enrollment.

### 10.2 Backend, only if change proposal `0003` is approved

1. The re-issue endpoint requires a valid session cookie and rejects an
   expired, revoked, or absent one.
2. It does not extend the absolute session lifetime.
3. It is rate-limited on the existing persistent throttle.
4. It emits an audit event.
5. It returns no credential material other than the new CSRF proof.
6. Existing login, logout, session, revoke, and password tests pass unchanged.

### 10.3 Browser and E2E scenarios

1. Create an enrollment token, list it, revoke it, and confirm the device
   cannot then enrol with it.
2. Enrol a device, disable it, observe the state change, revoke it, and
   confirm the device channel is refused, exercising `SEC-06`.
2a. Re-enroll the revoked device: complete step-up and typed confirmation,
   receive the one-time token, dismiss it, run the real Edge enrollment with
   it, and confirm the device returns to active and its decoys stop being
   marked unmanaged. Confirm the token appears in no storage or artefact.
3. Rename and delete a zone, including a concurrent-modification conflict.
4. List sessions from two browser contexts, revoke the other one, and confirm
   the revoked context is signed out while the current one continues.
5. Change the password and confirm the reported session outcome.
6. Reload to a read-only session, use `Restore write access`, perform a
   level-2 action, then attempt a level-3 action and confirm step-up is
   required.
7. A denied step-up leaves the object unchanged.
8. Axe scan of every new screen and dialog.

Secret-bearing steps keep traces, video, and screenshots disabled. Screenshots
are taken only after explicit dismissal, matching existing practice.

## 11. Acceptance criteria and Definition of Done

1. Every operation in 9.7 is reachable, correctly confirmed, and correctly
   audited server-side.
2. Level-3 actions require step-up reauthentication and single-use marking.
3. Restore-write-access works when change proposal `0003` is approved, and is
   absent when it is not, with the package still delivering the rest.
4. No destructive action is optimistic; failures state that nothing changed.
5. Conflicts are distinguishable from validation failures.
6. Revoked devices and expired tokens remain visible with truthful state, and
   a revoked device can be recovered through re-enrollment under level-3
   confirmation without the token ever persisting.
7. Hostile names are inert everywhere, including confirmations.
8. `task web:check`, `task web:e2e`, the Control Plane test suite, and the
   auth integration suite pass.
9. The security review is recorded.

## 12. Evidence required

- Confirmation-level table test output.
- Browser evidence for each scenario in 10.3 across all three engines, with
  secrets dismissed before any capture.
- Proof that a revoked device is refused on the device channel.
- Audit records showing actor, time, object, and before-and-after references
  for each destructive action.
- Storage-empty assertions.
- If `0003` is approved: endpoint tests, rate-limit evidence, audit evidence,
  and confirmation that absolute session lifetime is unchanged.
- `security/wcx-09-operator-completeness-review.md`.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- change proposal `0003` is not approved and the resulting operator friction
  appears to make level-3 actions unusable in practice;
- the re-enrollment endpoint returns anything beyond a one-time token, or
  exposes a path that re-reads an issued token;
- issuing a re-enrollment token changes device state server-side, which would
  make the console unable to report the intermediate state truthfully;
- a destructive operation's backend semantics differ from what the contract
  documents;
- a backend change beyond the CSRF re-issue endpoint appears necessary;
- password change or session revocation behaviour cannot be reported
  truthfully from the available response;
- exposing any of these controls would require weakening the CSRF, session,
  or throttling model.

## 14. Deliverables

Device lifecycle, enrollment-token, zone, session, and password screens with
correct confirmation levels, the step-up reauthentication flow, the
restore-write-access flow when approved, the extended confirmation table, the
backend CSRF re-issue endpoint when approved, the browser scenarios, the
security review, and the runbook section.
