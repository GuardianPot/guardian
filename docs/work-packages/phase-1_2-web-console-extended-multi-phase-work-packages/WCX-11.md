---
id: WCX-11
phase: 2
wave: capability
title: Form and validation stack with decoy management UI
status: approved-for-implementation
risk: high
components:
  - web-console
decision_refs:
  - WC-D21
  - WC-D03
  - WC-D16
  - WC-D15
  - UX-06
  - ON-05
  - DC-11
  - DC-12
  - OPS-02
  - SEC-06
  - SEC-08
blocked_by_change_proposal:
  - CP-0004
acceptance_refs:
  - WCX-000 section 3.4
  - UX-06 decoy management minimum
  - AC-SMB-002 emulated Windows persona disclosure
  - SEC-06 revoked device decoys marked unmanaged
  - Phase 2 exit gate "4 decoy families shown in Web Console"
depends_on:
  - WCX-02
  - WCX-04
  - WCX-08
  - P2-W15
integration_dependencies:
  - CP-0004 approved by the Product Owner
allowed_paths:
  - "apps/web-console/src/**"
  - "apps/web-console/package.json"
  - "package-lock.json"
  - "tests/e2e/web-console/**"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-11.md"
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

# WCX-11 — Form and validation stack with decoy management UI

## 1. Purpose

Introduce the console's form and validation stack and prove it on the first
screen that genuinely needs it: decoy management, which is also the Phase 2
exit-gate requirement that four decoy families are visible in the console.

## 2. Why now

Every form in the console today is one or two uncontrolled fields with HTML
attribute validation copied by hand from the OpenAPI constraints. Decoy
configuration introduces persona, type, zone, address, and lifecycle fields
with interdependent validity. Building that screen without a form stack means
building the stack inside the screen.

## 3. Inputs and decisions

- `WC-D21` — React Hook Form with a Standard Schema validator; schemas derive
  from generated OpenAPI types; client validation is UX only and the backend
  remains authoritative.
- `WC-D03` — field-level error display depends on change proposal `0004`;
  without it, form-level errors remain the ceiling.
- `WC-D16` — confirmation levels apply to decoy removal and disabling.
- `WC-D15` — decoy status surfaces use the canonical data states.
- `UX-06` — approved decoy management minimum.
- `OPS-02`, `DC-11`, `DC-12`, `ON-05`, `SEC-08`.

## 4. Dependencies

`WCX-02`, `WCX-04`, and `WCX-08`. **`P2-W15` and the Phase 2 decoy backend
must be accepted first**; this package consumes a contract it does not create.
Change proposal `0004` is an integration dependency for field-level errors
only; without it the package still ships with form-level errors.

## 5. Scope

1. Add the form stack and the schema-derivation helper.
2. Migrate existing forms — login, environment create and rename, zone create,
   enrollment-token create — onto it without changing behaviour.
3. Add the unsaved-changes guard and dirty-state handling.
4. Build the decoy list satisfying `UX-06`.
5. Build the decoy detail and configuration form.
6. Build the deploy, remove, enable, and disable flows with correct
   confirmation levels and desired-versus-observed reporting.

## 6. Non-goals

- No incident, evidence, journey, AI, or notification capability.
- No decoy backend, adapter, pack, or runtime work.
- No contract change; `openapi/**` is forbidden here.
- No AI-assisted placement or persona generation. `ON-04` approved
  deterministic rules and `AIM-06` places persona generation outside the MVP.
- No topology map or graphical placement editor.
- No custom detection-rule editor.
- No field-level error display if change proposal `0004` is unapproved.

## 7. Allowed paths

Only the paths in frontmatter. `decoys/**` is forbidden: this package renders
decoy state and never defines decoy behaviour.

## 8. Security constraints

1. Client-side validation is a convenience only. Every constraint is enforced
   by the Control Plane, and a rejected submission must be rendered truthfully
   even when the client believed the input valid. A test submits a
   client-valid, server-invalid payload and asserts the rejection is shown.
2. Validation schemas derive from generated types. Hand-copying a pattern,
   length, or range from the contract is forbidden, because a drifted copy
   silently permits invalid input.
3. Decoy personas, banners, hostnames, and any operator-authored decoy text
   are shown to attackers by design. The console must warn, at the point of
   entry, that these values are attacker-visible, and must never suggest
   placing real credentials or real hostnames in them.
4. Decoy-originated and device-originated values, including observed status
   messages, last-interaction summaries, and version strings, are untrusted
   and render through the `WCX-06` components.
5. No synthetic credential, honey token, or decoy secret value may be
   displayed after its intended one-time workflow, persisted to browser
   storage, placed in a URL, or captured in a screenshot, trace, or video. The
   one-time handling rules established for enrollment secrets apply unchanged.
6. Form state must never be autosaved to browser storage. An unsaved-changes
   guard warns; it does not persist.
7. A decoy shown as active must reflect the backend's observed status. Desired
   configuration is never rendered as observed reality, and a decoy whose
   observation is missing renders `unknown`, never healthy.
8. Per `SEC-06`, when a device is disabled or revoked its decoys may continue
   running last-known-good configuration while the Control Plane can no longer
   manage them. Those decoys must be marked **unmanaged** and their observed
   state marked offline or unknown, and they must stay visible in that state
   until they are explicitly removed or the device is re-enrolled. Hiding them
   or continuing to show them as managed would misrepresent what the product
   controls.
9. Per `AC-SMB-002`, an emulated persona must be labelled as emulated. The
   console presents a Windows-flavoured decoy as a Windows-flavoured persona
   and must never state or imply that a real Windows host exists. The same
   rule applies to every persona in the `DC-11` curated set.

## 9. Implementation requirements

### 9.1 Form stack

1. Add React Hook Form and a Standard Schema validator, both pinned exactly
   and recorded under the `WCX-01` dependency policy.
2. A shared `useConsoleForm()` wrapper binds the resolver, wires field errors
   to the `WCX-04` `TextField` contract, and maps a submission `ConsoleError`
   to form-level or field-level presentation.
3. Schema derivation: a helper builds validators from the generated OpenAPI
   schema constraints. Where a constraint cannot be derived, the schema
   references the generated type and a comment records why, and a test asserts
   the derived constraint matches the contract.
4. Submission is always pessimistic. No form applies an optimistic update.
5. Dirty state is tracked; a navigation away from a dirty form prompts, and
   the prompt itself never persists the form.
6. A submitting form disables its controls, keeps its accessible name stable,
   and cannot be double-submitted.

### 9.2 Error presentation

Without change proposal `0004`: a rejected submission renders a form-level
error derived from the `ConsoleError` taxonomy, plus, where the taxonomy
identifies a `validation` kind, a generic statement of which form section is
at fault. No field is marked invalid on the basis of a guess.

With `0004` approved: the backend supplies `field_errors`, and each is
attached to its control through the `TextField` contract with `aria-invalid`
and `aria-describedby`. Unknown field names are surfaced as form-level errors
rather than dropped.

### 9.3 Decoy list

Satisfying `UX-06` and the `P2-W15` requirement, each row shows: display name,
type and persona, address and zone, health, version, last interaction time,
and desired-versus-observed state. Row actions: open detail, enable, disable,
remove.

Rules:

1. Health and desired-versus-observed are separate columns. A converged
   desired state is not a health claim.
2. `last interaction` uses the `WCX-08` `Timestamp` with `second` precision
   and renders `unknown` when no interaction has been observed. It must never
   render as `never` in a way that implies confirmed absence when the
   observation itself is missing.
3. The list uses `operational` freshness from `WCX-07`.
4. The reference environment caps at 32 decoys, so no virtualisation is
   required; the list must still use the `WCX-04` table primitives so `WCX-12`
   can extend rather than replace it.

### 9.4 Decoy detail and configuration

Sections: identity and persona, placement, runtime state, version and update
state, and recent interaction summary. Configuration fields follow the Phase 2
contract exactly; the console adds no field the contract does not define and
omits none it does.

`ON-05` allows the operator to override decoy address and persona. Those
fields carry the attacker-visibility warning from 8.3 inline, at the field,
not only in a page-level note.

### 9.5 Lifecycle flows and confirmation levels

| Action | Level | Notes |
|---|---|---|
| Enable | L1 | reversible |
| Disable | L1 | reversible; state change is reported, not assumed |
| Update configuration | L1 | pessimistic submit with conflict handling |
| Deploy | L2 | names the zone and address that will be occupied |
| Remove | L2 | names the decoy and states that observed history is retained |

Convergence: `DC-12` and the Phase 2 target of sixty seconds for a cached
artefact mean deployment is long-running. Per `WC-D17` the pending state lives
on the decoy row and the detail header, showing the desired revision, the last
observed revision, and the elapsed time. No blocking modal is used. Exceeding
the expected convergence window renders `degraded` with the reason the backend
reports, never a timeout invented by the console.

### 9.6 UI/UX requirements

1. Configuration changes state plainly that they take effect only after the
   Edge converges, and the UI shows when that happened.
2. Removing a decoy states that evidence and incidents already recorded are
   retained; `CS-06` forbids silent deletion and the wording must reflect it.
3. A decoy in a failed runtime state shows the backend reason and the
   operator's next action.
4. The list is usable at 320 pixels, with columns collapsing into a stacked
   row rather than scrolling the page horizontally.

### 9.7 Accessibility requirements

1. The decoy list is a semantic table with a caption, header cells, and a
   row-scoped accessible name for every action.
2. Every form control has a persistent label; errors are associated and
   announced once.
3. The attacker-visibility warning is programmatically associated with its
   field, not a visual-only adornment.
4. Submission progress is announced once on completion, not on each poll.
5. The unsaved-changes prompt is a dialog meeting the `WCX-04` contract.
6. Axe reports no serious or critical issue on every new screen.

### 9.8 API and data contracts

Consumed from the Phase 2 decoy contract as delivered by `P2-W15`. This
package does not define it. If a field required by `UX-06` is absent from the
contract, stop and escalate rather than deriving or inferring it in the
console.

### 9.9 Error and failure behaviour

1. A configuration conflict renders `conflict` with the current server value
   offered for reload; no silent overwrite.
2. A deploy that the Edge rejects renders the backend reason on the decoy, not
   as a generic failure.
3. A decoy whose observation is stale beyond its freshness class renders
   `stale` with the observation age.
4. A missing observation renders `unknown`.
5. A failed enable or disable leaves the displayed state unchanged and states
   that nothing changed.

### 9.10 Internationalisation and theme

All new text enters the `WCX-08` catalogue, including the attacker-visibility
warning, which must be reviewed as security-critical wording. Decoy health
uses the health palette; decoy severity concepts do not exist yet and no
severity token may be used here.

### 9.11 Performance

The decoy feature is its own chunk. The form stack is loaded only by routes
that use forms, so the login chunk must not include it; a build assertion
checks this. The authenticated initial-load budget from `WCX-07` holds.

### 9.12 Observability

None in the console.

### 9.13 Documentation

Add `Forms and validation` and `Decoy management` sections to
`docs/runbooks/web-console/development.md`, including the schema-derivation
rule, the attacker-visibility warning requirement, and the convergence
reporting model.

## 10. Required tests

### 10.1 Unit and component

1. A client-valid, server-invalid submission renders the rejection truthfully.
2. Derived schema constraints match the generated contract; a drift fixture
   fails the test.
3. No form applies an optimistic update; displayed state changes only after a
   confirmed response.
4. Dirty state triggers the guard; the guard persists nothing, verified by
   asserting empty browser storage afterwards.
5. Double submission is impossible.
6. Every `UX-06` column is present and health is separate from
   desired-versus-observed.
7. A missing interaction observation renders `unknown`, not `never`.
8. A missing decoy observation renders `unknown`, never healthy.
9. A stale observation renders `stale` with its age.
10. Hostile decoy names, personas, versions, and observed messages render
    inert.
15. A decoy belonging to a disabled or revoked device renders as unmanaged
    with an offline or unknown observed state, stays visible, and is never
    shown as managed or healthy, satisfying `SEC-06`.
16. Every persona is labelled as emulated; no surface states or implies a real
    Windows, database, or application host exists, satisfying `AC-SMB-002`.
11. The attacker-visibility warning is associated with its fields.
12. No decoy secret or synthetic credential appears after its one-time
    workflow, and browser storage stays empty.
13. Confirmation levels match the table in 9.5.
14. With `0004` unapproved, no field is marked invalid on a guess; with it
    approved, each returned field error attaches to its control and unknown
    field names surface at form level.

### 10.2 Browser and E2E scenarios

1. Deploy one decoy of each of the four Phase 2 families, observe convergence
   reporting from desired to observed, and confirm all four appear in the
   list. This is the Phase 2 exit-gate evidence.
2. Disable and re-enable a decoy and confirm the observed state follows.
3. Remove a decoy and confirm previously recorded evidence is retained.
4. Break convergence deliberately and confirm the console renders `degraded`
   with the backend reason rather than a fabricated timeout.
5. Submit an invalid configuration and confirm the rejection is truthful.
6. Hostile persona and banner values render inert in list and detail.
7. Narrow-viewport operability at 375 pixels.
8. Axe scan of the list, detail, and each dialog.

Secret-bearing steps keep traces, video, and screenshots disabled.

## 11. Acceptance criteria and Definition of Done

1. The form stack exists, all existing forms are migrated with unchanged
   behaviour, and no constraint is hand-copied from the contract.
2. Client validation never suppresses a truthful backend rejection.
3. The decoy list satisfies every `UX-06` column with health separate from
   convergence.
4. Deploy, remove, enable, and disable work with correct confirmation levels
   and truthful long-running reporting.
5. Four decoy families are visible in the console, satisfying the Phase 2 exit
   gate.
6. Attacker-visible fields carry an associated warning.
7. Hostile decoy content is inert everywhere.
8. `task web:check` and `task web:e2e` pass within budget.

## 12. Evidence required

- Browser evidence of four decoy families deployed, converged, and listed, in
  all three engines.
- Convergence timeline showing desired revision, observed revision, and
  elapsed time.
- Degraded-convergence evidence with the backend reason displayed.
- Schema-derivation drift test output.
- Hostile decoy content rendering report.
- Storage-empty assertions.
- Axe reports and narrow-viewport screenshots.
- Dependency admission records for the form and validation libraries.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- the Phase 2 decoy contract lacks a field that `UX-06` requires;
- a validation constraint cannot be derived and hand-copying appears to be the
  only option;
- distinguishing desired from observed state is impossible from the contract;
- convergence failure cannot be attributed to a backend-supplied reason;
- an attacker-visible field cannot be identified from the contract, so the
  warning cannot be placed accurately;
- change proposal `0004` is unapproved and form-level errors prove
  insufficient for a multi-field decoy configuration.

## 14. Deliverables

The form and validation stack with schema derivation and the unsaved-changes
guard, migrated existing forms, the decoy list satisfying `UX-06`, decoy
detail and configuration with attacker-visibility warnings, the lifecycle
flows with confirmation levels and truthful convergence reporting, the browser
scenarios including four-family evidence, and the two runbook sections.
