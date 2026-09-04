---
id: WCX-20
phase: 2
wave: capability
title: Synthetic honey credential workflow
status: draft
risk: high
components:
  - web-console
decision_refs:
  - DC-09
  - DC-10
  - CS-03
  - DATA-02
  - SEC-02
  - AUTH-06
  - WC-D16
  - WC-D24
  - W11-C3-A
acceptance_refs:
  - DC-09 bounded MVP synthetic credential capability
  - P2-W9 "normal UI does not reveal reusable secret after intended workflow"
  - Phase 2 exit gate "synthetic credential works"
depends_on:
  - WCX-11
  - P2-W9
allowed_paths:
  - "apps/web-console/src/**"
  - "tests/e2e/web-console/**"
  - "security/wcx-20-honey-credential-review.md"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-20.md"
forbidden_paths:
  - "apps/control-plane/**"
  - "apps/edge-agent/**"
  - "openapi/**"
  - "proto/**"
  - "decoys/**"
  - "0-planning-documents/**"
requires_owner_review: true
requires_security_review: true
---

# WCX-20 — Synthetic honey credential workflow

## 1. Purpose

Deliver the console surface for the approved bounded synthetic credential
capability: generating a decoy-only credential, associating it with a decoy,
revealing it exactly once for manual placement, and observing its trigger
lifecycle.

## 2. Why now

`DC-09` approves synthetic honey credentials as a bounded MVP capability and
`P2-W9` builds the domain. The Phase 2 exit gate requires that the synthetic
credential works. The workflow is explicitly operator-driven — `DC-09` states
the operator places the credential manually in a lab or pilot location — so
without a console surface the capability cannot be exercised. `WCX-11` covers
decoy management and deliberately excluded this workflow, leaving it without a
home.

`CS-03` makes use of a valid synthetic credential the strongest confidence
contributor in the product, so the operator must be able to create one and
understand what its use will mean.

## 3. Inputs and decisions

- `DC-09` — bounded MVP: the product generates a decoy-only credential; it
  authenticates only to the selected decoy or is recognised by that decoy as a
  high-confidence marker; it carries no production privilege; the operator
  places it manually; its use raises incident confidence. Automated placement
  by an endpoint agent is outside the MVP.
- `DC-10` — honey files, documents, and tokens are **not** in the MVP.
- `CS-03` — valid synthetic credential use is a very strong confidence factor.
- `DATA-02` — real captured passwords are never rendered in plaintext.
- `P2-W9` — normal UI does not reveal a reusable secret after the intended
  workflow.
- `W11-C3-A` — the established one-time secret handling rules.

## 4. Dependencies

`WCX-11` for the decoy feature and lifecycle patterns. **`P2-W9` must be
accepted first.**

## 5. Scope

1. Credential list showing non-secret metadata and lifecycle state.
2. Creation with decoy association and one-time reveal.
3. The manual placement guidance surface.
4. Trigger and use history.
5. Revocation and rotation of a credential.

## 6. Non-goals

- No honey files, documents, or tokens. `DC-10` excludes them from the MVP.
- No automated placement, endpoint agent integration, or file distribution.
  `DC-09` places automated placement outside the MVP.
- No credential re-display after the one-time reveal, and no recovery path
  that would show it again.
- No import of an operator-supplied credential. The product generates it.
- No production credential handling of any kind.
- No incident, evidence, or confidence rendering beyond linking to the
  incident a trigger produced. `WCX-12` and `WCX-13` own those.

## 7. Allowed paths

Only the paths in frontmatter.

## 8. Security constraints

1. **The credential secret is one-time material.** It is displayed exactly
   once at creation or rotation under the `W11-C3-A` rules already proven for
   the enrollment secret: it bypasses the query cache, lives only in
   route-local state, leaves the DOM on dismissal, route exit, and unload, and
   never enters browser storage, a URL, a log, an analytics sink, a trace, a
   video, or a screenshot.
2. After the reveal the console shows only non-secret metadata: identifier,
   associated decoy, username or principal if the contract exposes it as
   non-secret, creation time, state, and last trigger time. No control may
   re-reveal the secret, and a source-level check asserts no read path exists.
3. The reveal surface must state plainly, before revealing, that the
   credential grants no production access, that it authenticates only to the
   associated decoy, that it is intended for placement where an attacker would
   plausibly find it, and that its use will raise incident confidence. An
   operator who misunderstands this could place it somewhere harmful.
4. The console must never suggest or accept a real production system,
   username, or password as part of this workflow. No field accepts an
   operator-supplied secret.
5. `DATA-02` applies to the trigger history: an attacker's supplied value at
   use time is never rendered in plaintext. The console shows a match or
   no-match result and the evidence reference, never the submitted secret.
6. Placement guidance is instructional text only. The console performs no file
   write, no download, no clipboard write without an explicit operator action,
   and no transfer to any host.
7. Revocation is a level-2 confirmation stating that previously recorded
   evidence and incidents are retained and that future use will no longer
   authenticate.
8. Credential metadata is untrusted where it can carry operator-authored or
   decoy-reported text and renders through the `WCX-06` components.

## 9. Implementation requirements

### 9.1 Credential list

Per credential: identifier, associated decoy and its zone, principal or
username as the contract exposes it, state, creation time, last trigger time,
and trigger count. State values come from the backend and render through the
`WCX-04` matrix; a credential whose state cannot be determined renders
`unknown`, never active.

Freshness follows the `operational` class from `WCX-07`.

### 9.2 Creation and one-time reveal

1. Creation is a form using the `WCX-11` stack, selecting the decoy to
   associate and any contract-defined attributes. No secret field exists.
2. On success the secret is revealed in a dialog that reuses the
   enrollment-secret pattern, including the explicit acknowledgement control.
3. The dialog carries the statements required by 8.3 above the secret, not
   below it.
4. Dismissal removes the secret from the DOM and from component state, and a
   test asserts it is unrecoverable afterwards.
5. Rotation, where the contract supports it, follows the identical path and
   states that the previous secret stops authenticating.

### 9.3 Manual placement guidance

A short, catalogue-sourced instructional surface accompanying the reveal,
covering where a credential is plausibly discovered in a lab or pilot
environment and what must not be done with it. It is text only; it performs no
action and offers no automated placement.

### 9.4 Trigger and use history

1. Per credential: each recorded use with time at second precision, observed
   source, the decoy that recognised it, the match result, and a link to the
   incident or evidence it contributed to.
2. The confidence contribution is stated in `CS-03` terms as the backend
   reports it; the console never computes it.
3. An empty history renders `empty` only on a confirmed successful response,
   and never on a failed or denied read, so an operator is never told a
   credential was unused when the console simply does not know.

### 9.5 UI/UX requirements

1. The workflow reads as a deliberate security action, not a routine setting.
2. The list makes it obvious which credentials have been triggered.
3. A credential associated with a removed or revoked decoy is shown with that
   condition, not silently hidden.
4. At 320 pixels the list and history stack without dropping the trigger time
   or count.

### 9.6 Accessibility requirements

1. The reveal dialog meets the `WCX-04` contract, and its announcement
   contains no secret material.
2. The statements required by 8.3 are part of the dialog's accessible
   description.
3. The copy control has an accessible name identifying what is copied and that
   it is shown once.
4. Lists are semantic tables with captions and row-scoped action names.
5. Axe reports no serious or critical issue at both viewports.

### 9.7 API and data contracts

Consumed from `P2-W9`. This package defines no contract. If the contract
exposes a path that returns a previously created secret, stop and escalate:
the console must not surface it, and its existence is a security finding for
the backend package.

### 9.8 Error and failure behaviour

1. A failed creation reveals nothing and states that no credential was
   created.
2. A failed rotation states that the previous secret remains valid, because an
   operator who believes rotation succeeded will replace the placed credential
   and lose the trap.
3. A failed revocation leaves the credential active and says so.
4. An unavailable trigger history renders `unknown`, never an empty history.
5. No action is optimistic.

### 9.9 Internationalisation and theme

All wording, in particular the 8.3 statements, the placement guidance, and the
revocation and rotation wording, enters the `WCX-08` catalogue and is reviewed
as security-critical language. A triggered credential is an operational state
and uses the health palette, never a severity token; severity belongs to the
incident its use produced.

### 9.10 Performance

Lists are small and need no virtualisation but use the `WCX-12` table
primitives. The feature shares the decoy chunk from `WCX-11`.

### 9.11 Observability

None. Credential identifiers and secrets are excluded from the `WCX-15`
diagnostic report.

### 9.12 Documentation

Add a `Synthetic honey credentials` section to
`docs/runbooks/web-console/development.md` covering the one-time model, the
required statements, the manual placement workflow, and the rule that a
secret is never recoverable after dismissal. Record
`security/wcx-20-honey-credential-review.md`.

## 10. Required tests

### 10.1 Unit and component

1. The secret is shown once, leaves the DOM on dismissal, route exit, and
   unload, and never reaches the query cache or any browser storage area.
2. No control re-reveals a secret; a source-level check asserts no read path.
3. The required statements appear above the secret in the reveal dialog and in
   its accessible description.
4. No form field accepts an operator-supplied secret.
5. A failed creation reveals nothing; a failed rotation states the previous
   secret remains valid; a failed revocation states the credential is still
   active.
6. Trigger history never renders an attacker-submitted secret; only a match
   result and an evidence reference appear.
7. An unavailable history renders `unknown`, never empty.
8. A credential on a removed or revoked decoy shows that condition.
9. Hostile credential metadata and decoy names render inert.
10. After exercising the full workflow, all browser storage areas are empty.

### 10.2 Browser and E2E scenarios

1. Create a credential, dismiss the reveal, and confirm it cannot be
   recovered anywhere in the console.
2. Use the credential against its associated decoy over the real pipeline and
   confirm the trigger appears with a link to the resulting evidence, and that
   confidence rises as the backend reports, producing Phase 2 exit-gate
   evidence.
3. Attempt the credential against a different decoy and confirm the result is
   reported as the backend reports it.
4. Revoke the credential and confirm previously recorded evidence remains.
5. Confirm no browser storage, trace, video, or screenshot contains the
   secret at any point.
6. Narrow-viewport rendering at 375 and 320 pixels.
7. Axe scan of every new screen and dialog.

Secret-bearing steps keep traces, video, and screenshots disabled. Screenshots
are taken only after explicit dismissal.

## 11. Acceptance criteria and Definition of Done

1. A credential can be created, associated with a decoy, revealed once, and
   placed manually.
2. The secret is unrecoverable after dismissal and appears in no storage,
   cache, or artefact.
3. The required statements are shown before the secret.
4. Trigger history links use to evidence without rendering any submitted
   secret.
5. Rotation and revocation report failure truthfully.
6. The Phase 2 exit-gate synthetic credential evidence is produced.
7. `task web:check` and `task web:e2e` pass within budget.
8. The security review is recorded.

## 12. Evidence required

- Browser evidence of the full workflow in all three engines, with the secret
  dismissed before any capture.
- Storage, cache, and artefact scans proving the secret does not persist.
- Source-level proof that no re-reveal path exists.
- Trigger evidence linking credential use to the resulting incident evidence.
- Axe reports and viewport screenshots.
- `security/wcx-20-honey-credential-review.md`.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- the contract exposes an endpoint that returns a previously created secret;
- the credential cannot be proven decoy-only from the contract, so the console
  cannot truthfully state that it grants no production access;
- the trigger history exposes an attacker-submitted secret, which `DATA-02`
  forbids rendering;
- rotation semantics do not define the previous secret's validity, so the
  console cannot state the placement implication truthfully;
- an operator workflow appears to require automated placement, which `DC-09`
  excludes.

## 14. Deliverables

The credential list with non-secret metadata, creation with decoy association
and one-time reveal under the established secret rules, the manual placement
guidance surface, trigger and use history linked to evidence, rotation and
revocation with truthful failure reporting, the browser scenarios producing
Phase 2 synthetic credential evidence, the security review, and the runbook
section.
