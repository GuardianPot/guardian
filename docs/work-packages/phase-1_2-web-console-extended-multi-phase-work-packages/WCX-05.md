---
id: WCX-05
phase: 2
wave: foundation
title: Accessibility baseline and enforcement
status: draft
risk: medium
components:
  - web-console
decision_refs:
  - WC-D23
  - WC-D11
  - WC-D13
  - WC-D17
remediates:
  - P1-W11 GAP-4
  - P1-W11 GAP-5 (jsx-a11y)
acceptance_refs:
  - WCX-000 section 3.4
  - WCAG 2.2 Level AA
depends_on:
  - WCX-04
allowed_paths:
  - "apps/web-console/src/**"
  - "apps/web-console/eslint.config.js"
  - "apps/web-console/index.html"
  - "apps/web-console/package.json"
  - "package-lock.json"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-05.md"
forbidden_paths:
  - "apps/control-plane/**"
  - "apps/edge-agent/**"
  - "openapi/**"
  - "proto/**"
  - "tests/**"
  - "0-planning-documents/**"
requires_owner_review: true
requires_security_review: false
---

# WCX-05 — Accessibility baseline and enforcement

## 1. Purpose

Set `WCAG 2.2 Level AA` as the enforced conformance target and add the three
enforcement layers that keep it from regressing, together with the route-level
focus, announcement, and document-title behaviour the console currently lacks.

## 2. Why now

`P1-W11` produced good practice without a defined target: a skip link,
landmarks, labels, error association, and reduced-motion handling exist, but
route changes move no focus, announce nothing, and never change the document
title. The only automated check is a full-page browser scan, which catches
regressions late. Every later package adds screens; the target and the gates
must exist before they do.

## 3. Inputs and decisions

- `WC-D23` — `WCAG 2.2 Level AA`, enforced by `eslint-plugin-jsx-a11y`,
  component-level axe assertions, and full-page browser scans.
- `WC-D11` — colour is never the only channel.
- `WC-D13` — reduced motion removes animation.
- `WC-D17` — feedback surfaces carry defined roles.
- Remediates `P1-W11 GAP-4` and the `jsx-a11y` portion of `GAP-5`.

## 4. Dependencies

`WCX-04` must be accepted; enforcement targets the shared component layer, and
fixing components individually before they are shared would be wasted work.

## 5. Scope

1. Activate `eslint-plugin-jsx-a11y` as errors.
2. Add component-level axe assertions and a shared test helper.
3. Implement route-change focus management, announcement, and document title.
4. Correct any conformance defect the new gates reveal in existing code.
5. Add a keyboard-only browser scenario to the component test layer.
6. Record the conformance target and its exceptions.

## 6. Non-goals

- No new screen, route, capability, or endpoint.
- No visual redesign. Fixes are limited to semantics, focus, naming, and
  announcement; a fix that requires a visual change is escalated.
- No browser E2E file changes; `tests/**` is forbidden here. Browser-level
  keyboard and axe scenarios are added by `WCX-06`, which owns that directory.
- No screen-reader vendor certification. Automated conformance plus documented
  manual checks is the bar.

## 7. Allowed paths

Only the paths in frontmatter.

## 8. Security constraints

1. An accessible name, description, or live-region announcement must never
   include backend or attacker-supplied content beyond what is already
   rendered as inert text. Announcements are composed from catalogue keys plus
   already-escaped values.
2. A live region must never announce a one-time enrollment secret, a
   credential, or any value the secret-lifetime rules exclude from persistence
   surfaces. A test asserts that the secret dialog's announcement contains no
   secret material.
3. Focus management must not move focus into a dialog that renders secret
   material without the operator having invoked it.
4. Disabled controls keep their reason associated, so an operator using a
   screen reader learns that reauthentication is required rather than
   encountering a silently inert control.

## 9. Implementation requirements

### 9.1 Conformance target

`WCAG 2.2 Level AA` for every operator-facing surface. Recorded exceptions
must be listed in the runbook with a reason and a review date; the initial
list must be empty or justified in the evidence.

### 9.2 Route-change behaviour

On every completed navigation:

1. Set `document.title` to `<screen name> — Guardian Console`. Screen names
   come from the route definition, not from backend data.
2. Move focus to the screen's `h1`, which carries `tabIndex={-1}`. If the
   screen is still loading, focus the main landmark and move focus to the `h1`
   when it appears.
3. Announce the screen name through a single polite live region owned by the
   application shell. The region is not re-created per navigation.
4. Do not scroll-restore in a way that hides the focused heading behind a
   sticky element.

The existing skip link continues to target the main landmark and must remain
the first focusable element.

### 9.3 Static enforcement

Activate `eslint-plugin-jsx-a11y` with its recommended rule set as errors, plus
explicit activation of rules for label association, interactive element roles,
redundant roles, autofocus prohibition, and no positive `tabIndex`. Rule
suppressions require an inline comment naming the reason; a repository check
counts suppressions and fails if any lacks a reason.

### 9.4 Component-level enforcement

Add an axe assertion helper used by every shared component test and by every
route test. It fails on any violation at `serious` or `critical` impact, and
reports `moderate` and `minor` without failing so that regressions are
visible.

### 9.5 Keyboard operability

Every interactive element is reachable and operable by keyboard alone, in a
logical order. Specific requirements:

1. The sign-out control is reachable at every viewport width. This is verified
   here even though the responsive defect itself is fixed in `WCX-10`.
2. Dialogs trap focus, close on escape, and return focus to the invoking
   control.
3. The segmented MFA method control is operable with arrow keys and exposes
   its pressed state.
4. No element uses a positive `tabIndex`.
5. Focus is always visible; the focus ring meets the contrast requirement set
   in `WCX-03`.

### 9.6 Forms and errors

1. Every input has a programmatically associated label.
2. Every error is associated with its control through `aria-describedby` and
   marked with `aria-invalid`.
3. Form-level errors use `alert`; success uses `status`.
4. Required fields are marked programmatically, not only visually.
5. An error message identifies the field and the correction, not just that
   something failed.

### 9.7 Dynamic content

1. The polite live region announces screen changes and completed actions.
2. An assertive region is used only for blocking errors, and at most one
   exists.
3. Content that updates on a refresh interval must not announce on every
   refresh; only a change in state announces.

### 9.8 API and data contracts

None.

### 9.9 Error and failure behaviour

If the live region or focus target is missing, the screen must still be
operable. A missing `h1` is a defect that fails the component test rather than
a silent runtime fallback.

### 9.10 Internationalisation and theme

Screen names and announcements are catalogue-ready constants for `WCX-08`. The
document language stays `en` until the catalogue supports another locale.

### 9.11 Performance

The live region and focus management add negligible cost. `jsx-a11y` and the
axe helper are development-only. Component test runtime may grow; keep the
suite under a documented ceiling and report it.

### 9.12 Observability

None.

### 9.13 Documentation

Add an `Accessibility` section to `docs/runbooks/web-console/development.md`
covering the target, the three enforcement layers, the route-change contract,
the manual check list, and the exceptions register.

## 10. Required tests

### 10.1 Unit and component

1. Every shared component and every route passes the axe helper with no
   serious or critical violation.
2. Navigation sets the document title, moves focus to the `h1`, and announces
   once.
3. Navigation to a still-loading screen focuses the main landmark and then the
   `h1` when it renders.
4. The skip link is the first focusable element and moves focus to main.
5. A keyboard-only traversal of each existing route reaches every interactive
   control, including sign-out, at both a wide and a narrow viewport.
6. Dialog focus trap, escape, and focus return are asserted.
7. Every input has an associated label; every error is associated and marks
   `aria-invalid`.
8. A disabled control exposes its reason through `aria-describedby`.
9. The secret dialog announcement contains no secret material.
10. A refresh that changes nothing produces no announcement; a state change
    produces exactly one.

### 10.2 Static analysis

`npm run lint` passes with `jsx-a11y` active and `--max-warnings=0`. A fixture
containing an unlabelled input and a positive `tabIndex` fails lint. Every
suppression carries a reason.

### 10.3 Browser and E2E scenarios

No new browser scenario is added here because `tests/**` is forbidden. The
existing suite's axe scan and reduced-motion step must continue to pass.
`WCX-06` adds the browser-level keyboard traversal and the expanded axe
scenarios.

## 11. Acceptance criteria and Definition of Done

1. `WCAG 2.2 Level AA` is the recorded target with a justified exceptions
   register.
2. `jsx-a11y` is active as errors and the workspace is clean.
3. The axe helper is used by every shared component and route test.
4. Route changes set the title, move focus, and announce exactly once.
5. Every interactive control is keyboard reachable at both tested viewports,
   including sign-out.
6. `task web:check` and `task web:e2e` pass.

## 12. Evidence required

- Lint output with `jsx-a11y` active, plus the failing fixture output.
- Axe report per component and per route, including moderate and minor
  findings that were reported but not blocking.
- Keyboard traversal test output for both viewports.
- The exceptions register, empty or justified.
- Component test suite runtime before and after.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- a `WCAG 2.2 Level AA` criterion cannot be met without a visual or layout
  change, since visual change is a non-goal here;
- the target-size or dragging criteria conflict with a data-dense layout
  planned for `WCX-12`;
- an existing behaviour must change to become conformant;
- a required announcement would necessarily include backend-supplied text.

## 14. Deliverables

The recorded conformance target and exceptions register, `jsx-a11y`
enforcement, the shared axe assertion helper wired into every component and
route test, route-change focus, announcement, and title behaviour, keyboard
operability fixes, and the accessibility runbook section.
