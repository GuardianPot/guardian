---
id: WCX-10
phase: 2
wave: foundation
title: Navigation information architecture and responsive correction
status: draft
risk: medium
components:
  - web-console
decision_refs:
  - WC-D14
  - WC-D04
  - WC-D06
  - WC-D07
  - UX-01
  - UX-07
  - ENV-01
  - ENV-02
remediates:
  - P1-W11 GAP-1
acceptance_refs:
  - WCX-000 section 3.3
  - UX-01 incident-first primary screen
depends_on:
  - WCX-04
integration_dependencies:
  - WCX-07
allowed_paths:
  - "apps/web-console/src/**"
  - "tests/e2e/web-console/**"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-10.md"
forbidden_paths:
  - "apps/control-plane/**"
  - "apps/edge-agent/**"
  - "openapi/**"
  - "proto/**"
  - "0-planning-documents/**"
requires_owner_review: true
requires_security_review: false
---

# WCX-10 — Navigation information architecture and responsive correction

## 1. Purpose

Restructure navigation around the approved incident-first model, introduce the
environment scope selector, and fix the responsive defect that hides the
sign-out control on narrow viewports.

## 2. Why now

`UX-01` makes an incident-first dashboard the primary screen. The console's
current root is environment-scoped, so Phase 3 would either bolt incidents
into an environment subtree or restructure navigation while the dashboard is
being built. Deciding the shell now means Phase 3 adds a screen rather than
rebuilding the frame. The responsive defect is separate but lands here because
it is a defect in the same shell.

## 3. Inputs and decisions

- `WC-D14` — global incident-first root; environment is a scope selector
  carried in a URL query parameter, keeping shared links deterministic.
- `WC-D04` — scope participates in query keys.
- `WC-D06` — explicit route tree; feature-level chunks.
- `WC-D07` — capability-gated controls are disabled with a reason, not hidden.
- `UX-01`, `UX-07`, `ENV-01`, `ENV-02`.
- Remediates `P1-W11 GAP-1`.

## 4. Dependencies

`WCX-04` for the shell components and states. `WCX-07` is an integration
dependency because chunk boundaries and the navigation structure must agree.

## 5. Scope

1. Introduce the route tree of 9.1 with a placeholder home that Phase 3
   replaces.
2. Implement the environment scope selector and its URL parameter.
3. Rebuild the application shell with a responsive navigation pattern that
   never hides operator controls.
4. Fix `GAP-1`.
5. Migrate existing routes to the new tree with redirects from old paths.
6. Add browser scenarios for narrow-viewport operability and scope
   persistence.

## 6. Non-goals

- No incident, decoy, or notification screen. The home route renders a
  placeholder that states Phase 3 will populate it and links to environments
  and devices; it must not fabricate incident content or counts.
- No new capability, endpoint, or data.
- No navigation entry for a screen that does not exist.
- No change to authentication, session, or capability semantics.
- No topology map. `UX-08` forbids it.

## 7. Allowed paths

Only the paths in frontmatter. This package owns `tests/e2e/web-console/**`
for its scenarios.

## 8. Security constraints

1. Route guards remain presentation only. `RequireAuth` continues to gate
   rendering, and every protected read remains authorised server-side. A test
   asserts that removing the guard would not expose data, by confirming each
   protected endpoint returns `401` without a session.
2. The environment scope parameter is untrusted input. It is validated against
   the UUID pattern before use, never interpolated into markup, and never used
   to construct a request path without validation. An invalid or unknown value
   renders `not-found` or `denied` as the backend reports, never a silent
   fallback to another environment.
3. Falling back to a different environment when the requested one is denied is
   forbidden. An operator must never believe they are looking at environment A
   while seeing environment B.
4. No navigation label, breadcrumb, or document title may contain
   backend-supplied text unless rendered through the untrusted components.
5. Old-path redirects must not create an open redirect. Only same-origin,
   statically known paths are redirect targets; no redirect target is read
   from a query parameter.

## 9. Implementation requirements

### 9.1 Route tree

```text
/login                                  unauthenticated
/                                       home (Phase 3 dashboard; placeholder now)
/environments                           environment list
/environments/:environmentId            environment detail
/environments/:environmentId/devices/:deviceId
                                        device detail
/account                                sessions and password (WCX-09)
/__components                           development only (WCX-06)
*                                       not found
```

Scope: `?env=<uuid>` is read by the shell and applied to scope-aware queries.
It is absent when a screen already carries the environment in its path.

Redirects: the previous default redirect to `/environments` is replaced by
`/`. Any previously bookmarkable path continues to resolve; a test enumerates
the old paths and asserts each still lands on the correct screen.

Primary navigation entries, in order: `Home`, `Environments`, `Account`. An
entry is added only when its screen exists. Phase 3 adds `Incidents` content
to `Home`; Phase 2 capability work adds `Decoys`.

### 9.2 Environment scope selector

1. Rendered in the shell header, listing environments the operator can read.
2. Selecting an environment sets `?env=` on the current route and refetches
   scope-aware queries.
3. The selection is not persisted anywhere. On a fresh load with no parameter,
   no environment is selected and scope-aware surfaces render a state that
   asks for a selection, never a silently chosen default.
4. When exactly one environment exists, it is preselected on load and the
   parameter is written to the URL so the link remains deterministic.
5. The selector is disabled with a reason while the environment list is
   loading, denied, or empty.

### 9.3 Responsive shell

1. A single shell layout with a defined breakpoint at 900 pixels.
2. Above the breakpoint: persistent sidebar navigation and a visible operator
   block containing the signed-in identity, sign-out, and the
   re-authentication affordance.
3. At or below the breakpoint: navigation collapses into a disclosure control,
   and the operator block moves into that disclosure. **No operator control is
   removed at any viewport width.** This is the `GAP-1` fix and is the single
   most important requirement in this package.
4. The disclosure is a button with `aria-expanded` and `aria-controls`,
   operable by keyboard, closing on escape and on route change, returning
   focus to its trigger.
5. The read-only session banner remains visible at every width.
6. Minimum supported width is 320 pixels with no horizontal page scroll. Wide
   content scrolls inside its own container.

### 9.4 UI/UX requirements

1. The home placeholder states plainly that the incident dashboard arrives in
   a later phase and offers the two working entry points. It must not display
   an empty incident list, a zero count, or anything that could read as "no
   incidents detected".
2. Breadcrumbs on nested routes give an explicit path back to the environment
   and to the list.
3. The active navigation entry is indicated by more than colour.
4. Scope changes do not lose the operator's position on the current screen
   where the screen remains valid.

### 9.5 Accessibility requirements

1. Navigation is a `nav` landmark with an accessible name; the shell has one
   `main` landmark.
2. The skip link remains the first focusable element and targets `main`.
3. Route change focus, announcement, and document title from `WCX-05` continue
   to work in the new tree; the placeholder home participates.
4. The disclosure control meets the pattern in 9.3.
5. The scope selector is a labelled control whose current value is announced
   on change, once.
6. Every navigation entry and every breadcrumb is reachable by keyboard at
   every supported width.
7. Axe reports no serious or critical issue on every route at both a wide and
   a narrow viewport.

### 9.6 API and data contracts

No contract change. The scope selector consumes the existing environment list.
Scope-aware query keys gain the environment identifier as a scope segment,
following the `WCX-02` key shape.

### 9.7 Error and failure behaviour

1. An unknown or malformed `env` parameter renders `not-found`; a denied one
   renders `denied`. Neither falls back to another environment.
2. A failed environment list disables the selector with a reason and leaves
   non-scoped screens usable.
3. An unknown route renders a not-found screen with navigation intact, not a
   redirect that hides the mistake.
4. A route chunk failure renders the route boundary fallback from `WCX-04`.

### 9.8 Internationalisation and theme

All navigation labels, breadcrumb text, placeholder copy, and the disclosure
control name enter the `WCX-08` catalogue. Environment display names are
untrusted values and render through the `WCX-06` components inside the
selector and breadcrumbs.

### 9.9 Performance

The shell stays in the entry chunk. The scope selector must not fetch the
environment list more than once per freshness interval, shared across every
consumer through a single `queryOptions()` helper. Initial authenticated load
must remain within the `WCX-07` budget.

### 9.10 Observability

None.

### 9.11 Documentation

Add a `Navigation and scope` section to
`docs/runbooks/web-console/development.md` covering the route tree, the scope
parameter, the redirect table, the breakpoint behaviour, and the rule that no
operator control is hidden at any width.

## 10. Required tests

### 10.1 Unit and component

1. Every old path resolves to the correct new screen.
2. `?env=` is validated; malformed renders `not-found`, denied renders
   `denied`, and neither substitutes another environment.
3. With no parameter and multiple environments, no environment is auto-chosen.
4. With exactly one environment, it is preselected and written to the URL.
5. The selector is disabled with a reason while loading, denied, or empty.
6. At 320, 375, 900, and 1440 pixels, the sign-out control, the identity
   block, and the re-authentication affordance are all present and reachable.
7. The disclosure sets `aria-expanded`, closes on escape and on navigation,
   and returns focus to its trigger.
8. The read-only banner is present at every tested width.
9. The home placeholder renders no incident count, no empty incident list, and
   no phrase implying absence of detections. Asserted with explicit negative
   assertions.
10. No redirect target is derived from a query parameter.
11. Scope changes update scope-aware query keys and trigger exactly one
    refetch per affected query.

### 10.2 Browser and E2E scenarios

Added to `tests/e2e/web-console/`:

1. Narrow-viewport operability: sign in at 375 pixels, open navigation, reach
   every entry, and sign out successfully. This scenario is the acceptance
   proof for `GAP-1`.
2. Keyboard-only traversal of the shell at a narrow and a wide viewport.
3. Deterministic sharing: copy the URL with `?env=`, open it in a second
   context after signing in, and land on the same scoped view.
4. Denied scope: request an environment the session cannot read and confirm
   the denied state without any fallback.
5. Axe scan of every route at both viewports.
6. No horizontal page scroll at 320 pixels on every route.

### 10.3 Regression

The existing onboarding suite passes with only path updates where the route
tree moved, and with no assertion weakened.

## 11. Acceptance criteria and Definition of Done

1. The route tree in 9.1 exists, old paths redirect correctly, and no
   navigation entry points at a non-existent screen.
2. The scope selector works, never auto-selects among several environments,
   and never falls back on denial.
3. No operator control is hidden at any viewport width from 320 pixels
   upward, proven by browser test.
4. The home placeholder cannot be read as a statement about incidents.
5. Route focus, announcement, and title behaviour still hold.
6. `task web:check` and `task web:e2e` pass within the `WCX-07` budget.

## 12. Evidence required

- Browser recordings or step logs for the narrow-viewport sign-out scenario in
  all three engines.
- Screenshots at 320, 375, 900, and 1440 pixels for each route.
- Axe reports per route and viewport.
- Redirect table with test output.
- Bundle report showing the shell remained in the entry chunk.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- the incident-first root cannot be introduced without a placeholder that
  risks reading as a statement about detections;
- the scope parameter proves insufficient once environment-level authorization
  is considered, which would reopen `WC-D14`;
- the responsive fix cannot preserve every operator control without a layout
  change beyond the shell;
- an old path cannot be preserved without an ambiguous redirect;
- Phase 3 dashboard requirements appear to demand a different shell.

## 14. Deliverables

The new route tree with redirects, the environment scope selector with strict
validation and no silent fallback, the responsive shell with the `GAP-1` fix,
the home placeholder, breadcrumbs, the browser scenarios including
narrow-viewport sign-out, and the navigation runbook section.
