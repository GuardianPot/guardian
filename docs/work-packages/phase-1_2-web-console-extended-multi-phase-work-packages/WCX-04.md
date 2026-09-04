---
id: WCX-04
phase: 2
wave: foundation
title: Shared component layer, data-state matrix, error boundary
status: approved-for-implementation
risk: high
components:
  - web-console
decision_refs:
  - WC-D09
  - WC-D15
  - WC-D16
  - WC-D17
  - WC-D03
  - W11-C2-A
  - OPS-03
  - SEC-08
remediates:
  - P1-W11 GAP-2
acceptance_refs:
  - WCX-000 section 3.3
  - OPS-03 staleness
  - Phase 1 browser onboarding skeleton E2E must pass
depends_on:
  - WCX-02
  - WCX-03
allowed_paths:
  - "apps/web-console/src/**"
  - "apps/web-console/package.json"
  - "package-lock.json"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-04.md"
forbidden_paths:
  - "apps/control-plane/**"
  - "apps/edge-agent/**"
  - "openapi/**"
  - "proto/**"
  - "tests/**"
  - "0-planning-documents/**"
requires_owner_review: true
requires_security_review: true
---

# WCX-04 — Shared component layer, data-state matrix, error boundary

## 1. Purpose

Build the shared component layer that every later screen composes from: the
eight canonical data states, the confirmation levels for destructive and
privileged actions, the three feedback surfaces, and the error boundaries that
stop an unexpected exception from blanking the console.

## 2. Why now

`P1-W11` proved the truth semantics by writing them inline: health 404 is
rendered as unavailable rather than healthy, and inventory state is kept apart
from health. Those rules currently live in route code and will drift the
moment Phase 2 and Phase 3 add screens. There is also no error boundary at
all, so any unexpected exception blanks the whole console — unacceptable in a
security product.

## 3. Inputs and decisions

- `WC-D09` — Radix Primitives with CSS Modules remain the design system;
  consolidate to the single `radix-ui` package. Headless utilities are
  permitted and are not a second design system.
- `WC-D15` — eight canonical data states; no state may render as healthy.
- `WC-D16` — three confirmation levels for mutations.
- `WC-D17` — inline, banner, and toast with fixed semantics; errors are never
  toast-only; long-running work is shown on the object itself.
- `WC-D03` — components consume `ConsoleError`, never HTTP status codes.
- `OPS-03` — staleness is an explicit product concern.
- `SEC-08` — hostile content must not execute.

## 4. Dependencies

`WCX-02` for the error taxonomy and query conventions, `WCX-03` for semantic
tokens and the status encoding contract.

## 5. Scope

1. Consolidate `@radix-ui/react-dialog` and `@radix-ui/react-label` into the
   single `radix-ui` package.
2. Implement the eight data-state components and the `DataBoundary` helper
   that maps a query result to the correct state.
3. Implement root and route-level error boundaries.
4. Implement the three confirmation levels.
5. Implement the three feedback surfaces and the pending-on-object pattern.
6. Implement the base control set the later phases need: `Button`,
   `TextField`, `Panel`, `StatusBadge`, `ConfidenceMeter`, `Dialog`,
   `Banner`, `Toast`, `DescriptionList`, `Skeleton`.
7. Migrate the four existing routes onto the shared components without
   changing what they display.

## 6. Non-goals

- No new route, screen, endpoint, or capability.
- No table, pagination, virtualisation, or filtering. Those are `WCX-12`.
- No form library and no schema validation. Those are `WCX-11`.
- No timeline, chart, or journey rendering. Those are `WCX-13`.
- No text extraction into a catalogue. That is `WCX-08`.
- No `jsx-a11y` rule activation. That is `WCX-05`.
- No new design system, no Tailwind, no MUI, no shadcn adoption.

## 7. Allowed paths

Only the paths in frontmatter. `tests/e2e/**` is forbidden; the existing
browser suite must pass unchanged where it asserts current behaviour, and any
scenario that genuinely needs a new selector is added by `WCX-06`.

## 8. Security constraints

1. No component renders HTML from data. `dangerouslySetInnerHTML` remains
   absent and is forbidden by lint.
2. No data state may present missing, denied, degraded, or stale data as
   healthy, complete, or successful. This is the single most important rule in
   this package and is asserted per state.
3. `denied` must be distinguishable from `empty`. An authorization failure
   that renders as an empty list would tell an operator that no devices exist
   when in fact access was refused.
4. `unknown` must be distinguishable from `empty` and from `False`. Absence of
   a health projection is not a healthy signal and not a failing signal.
5. Toast must never be the only surface carrying an error, because a toast can
   expire unseen.
6. Confirmation level 3 controls must remain disabled until their capability
   allows them, and must never be hidden.
7. No component may place any value into browser storage. A test asserts that
   after exercising every component, all storage areas are empty.
8. Error boundary fallbacks must render a fixed message from the catalogue.
   They must never render an exception message, a stack, a component stack, or
   any request or response content.

## 9. Implementation requirements

### 9.1 Data-state matrix

Eight states, each a distinct component under `@shared/ui/state/`:

| State | Meaning | Required content |
|---|---|---|
| `loading` | first load in flight | status role, described activity |
| `empty` | backend confirmed zero items | what the collection is, and the action that creates the first item when the operator has the capability |
| `unknown` | no observation exists; absence is not a negative observation | explicit statement that this is not a healthy signal, plus what would produce an observation |
| `stale` | last successful data shown, refresh failing or paused | the data, plus the age of the observation and the reason refresh is not current |
| `partial` | some sources succeeded and some failed | the successful data, an explicit list of what could not be loaded, and a retry action |
| `degraded` | an upstream dependency is impaired | which dependency, what still works, and what does not |
| `denied` | authorization refused | that access was refused, distinct from absence, with no detail about what exists |
| `error` | unexpected failure | a fixed message, a retry action when the error is retryable, and no diagnostic detail |

`DataBoundary` maps a TanStack Query result plus a `ConsoleError` to exactly
one state, with an exhaustive switch over `ConsoleErrorKind`:

| Condition | State |
|---|---|
| `isPending` and no cached data | `loading` |
| success and empty collection | `empty` |
| success and `null` projection where the domain defines absence | `unknown` |
| error is `forbidden` or `unauthenticated` | `denied` |
| error is `not-found` on an observation-shaped resource | `unknown` |
| error is `unavailable` or `timeout` with cached data | `stale` |
| error is `unavailable` or `timeout` without cached data | `degraded` |
| error is `network` with cached data | `stale` |
| error is `network` without cached data | `error` |
| any other error | `error` |

A read that is fresh but older than its freshness policy allows is rendered
`stale` even on success. `WCX-07` supplies the age threshold; until then the
threshold is a constant beside the policy.

### 9.2 Error boundaries

1. A root boundary wraps the router. Its fallback renders the product name, a
   fixed failure message, and a reload action. It never unmounts the document
   language or theme.
2. A route boundary wraps each routed element so a failure in one screen does
   not remove navigation. The operator must still be able to sign out.
3. A boundary reset occurs on navigation, so moving away from a broken screen
   recovers without a full reload.
4. Boundaries capture nothing to any sink in this package. `WCX-15` adds the
   user-triggered diagnostic report; the boundary exposes an internal hook for
   it and stores the last error in memory only.

### 9.3 Confirmation levels

| Level | Applies to | Interaction |
|---|---|---|
| L1 reversible | enable, disable where re-enable is immediate, disposition changes | no confirmation; result feedback and an undo affordance where the backend supports it |
| L2 destructive-recoverable | zone delete, enrollment-token revoke | modal confirmation; the confirm button names the effect, not `OK`; the affected object is named in the dialog |
| L3 irreversible-security | device revoke, session revoke, password change | modal confirmation, the operator types the exact object name to enable the confirm control, and step-up reauthentication is required |

Rules:

1. The action-to-level mapping is a table in code, not a per-call-site
   judgement. `WCX-09` and `WCX-11` extend the table; neither invents a level.
2. A confirmation dialog states the effect in the operator's terms and, for
   L3, states explicitly that the action cannot be undone.
3. The confirm control is destructive-styled and is never the default focused
   control; focus lands on cancel.
4. Step-up reauthentication is invoked through an interface defined here and
   implemented by `WCX-09` once change proposal `0003` is approved. Until
   then, L3 actions are unreachable because no screen exposes them.

### 9.4 Feedback surfaces

| Surface | Use | Lifetime | Role |
|---|---|---|---|
| inline | field and form-scoped validation and results | until the form changes | `alert` for errors, none for success |
| banner | page or scope-level persistent condition, such as read-only session, degraded dependency, stale data | until the condition clears | `status` for informational, `alert` for blocking |
| toast | short confirmation of a completed action | dismissible, at least 6 seconds, pauses on hover and focus | `status` |

Errors always appear inline or in a banner. A toast may accompany them but
never replaces them. Long-running work is shown on the object it affects: the
object's own row or panel carries a pending treatment, its age, and the
reason. No blocking progress modal is permitted.

### 9.5 Base component set

`Button` with `primary`, `secondary`, `destructive`, and `quiet` variants, a
`pending` state that keeps its accessible name stable, and a disabled state
that renders a reason. `TextField` with a label, description, error, and a
`aria-describedby` wiring that is not optional. `Panel` with heading level
control so a screen keeps a valid heading order. `StatusBadge` implementing
the `WCX-03` encoding contract for health, severity, device state, and
configuration. `ConfidenceMeter` implementing the non-colour confidence
indicator. `Dialog` over Radix with focus trap, return focus, and escape
handling. `Banner`, `Toast`, `DescriptionList`, and `Skeleton`.

Every component takes no `className` from outside except a documented layout
slot, so a screen cannot restyle a status indicator.

### 9.6 UI/UX requirements

The four existing routes are migrated to the shared components and must
continue to display the same information, including the separation of
inventory state from health, the eight health conditions rendered as text, the
blocking condition and its source device, and the read-only session banner.
Where a screen previously rendered a bespoke message for missing health, it
now renders the `unknown` state with equivalent meaning.

### 9.7 Accessibility requirements

1. Every state component announces correctly: `loading` and informational
   banners use `status`, blocking errors use `alert`.
2. Dialog focus is trapped, escape closes, focus returns to the invoking
   control, and the dialog has an accessible name and description.
3. `Panel` heading levels are explicit so no screen skips a level.
4. Toast is reachable by keyboard and does not steal focus.
5. Disabled controls remain focusable-by-description: the reason is associated
   with the control via `aria-describedby`, so a screen-reader user learns why
   it is disabled.
6. The existing axe scan must continue to report no serious or critical issue.

### 9.8 API and data contracts

None changed. `DataBoundary` consumes the `ConsoleError` shape from `WCX-02`.
No new endpoint is called.

### 9.9 Error and failure behaviour

Defined by 9.1 and 9.2. Additionally: a component that receives an unexpected
union value renders the unknown treatment; a component that receives no data
where data is required renders `unknown`, never an empty success.

### 9.10 Internationalisation and theme

All new operator-facing strings are declared as catalogue-ready constants in
one module per component so `WCX-08` can move them without renaming. All
colours come from `WCX-03` semantic tokens.

### 9.11 Performance

Component code must not push the bundle past 450 KiB JavaScript or 32 KiB CSS.
Radix consolidation is expected to reduce, not increase, the dependency graph;
report the change. No component may subscribe to a global interval.

### 9.12 Observability

The root boundary retains the last error in memory behind an interface for
`WCX-15`. Nothing is transmitted, logged, or stored.

### 9.13 Documentation

Add a `Shared components and data states` section to
`docs/runbooks/web-console/development.md` containing the state matrix, the
mapping table, the confirmation levels, and the feedback-surface rules.

## 10. Required tests

### 10.1 Unit and component

1. Every row of the state mapping table is asserted.
2. `denied` never renders as `empty`; `unknown` never renders as healthy,
   empty, or failing. Asserted with explicit negative assertions on the
   rendered text.
3. A stale read renders the data plus its observation age and reason.
4. A partial result lists what failed and offers retry.
5. The root boundary catches a thrown render error and renders the fixed
   fallback; the thrown message never appears in the DOM.
6. The route boundary keeps navigation and sign-out reachable during a screen
   failure.
7. Navigating away from a failed screen resets the boundary.
8. L2 confirmation names the object and the effect; L3 requires the typed name
   before enabling confirm, and confirm is not initially focused.
9. Toast never carries an error alone: a test asserts that for every error
   path an inline or banner surface is present.
10. After exercising every component, `localStorage`, `sessionStorage`, and
    IndexedDB are empty.
11. Hostile strings supplied as display names, reasons, health messages, and
    object names in confirmations render as inert text and create no markup.
12. The four migrated routes render the same information as before, asserted
    against the existing route tests with assertions preserved.

### 10.2 Browser and E2E scenarios

The existing onboarding suite passes in Chromium, Firefox, and WebKit,
including the read-only reload gate, the CSRF denial, the one-time secret
dismissal, the eight healthy conditions, the degradation step, and the axe
scan.

## 11. Acceptance criteria and Definition of Done

1. All eight states exist, `DataBoundary` maps to them exhaustively, and no
   state renders as healthy.
2. Root and route error boundaries exist; a screen failure never blanks the
   console and never leaks an exception message.
3. The three confirmation levels exist with a code-level action mapping table.
4. The three feedback surfaces exist with the stated semantics, and no error
   path is toast-only.
5. The base component set exists and the four existing routes are migrated
   with no information loss.
6. `radix-ui` replaces the two individual Radix packages.
7. `task web:check` and `task web:e2e` pass within budget.

## 12. Evidence required

- State-matrix test report covering all eight states and every mapping row.
- Error-boundary evidence showing a caught failure with navigation intact and
  no exception text in the DOM.
- Screenshots of each state for one representative surface, with any secret
  dismissed first.
- Storage-empty assertion output.
- Hostile-string component test output.
- Bundle size and dependency-count change from the Radix consolidation.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- a real backend response cannot be classified into exactly one of the eight
  states without inventing a semantic;
- distinguishing `denied` from `empty` is impossible for an endpoint, which
  would make change proposal `0004` blocking;
- an existing route cannot be migrated without changing what an operator sees;
- the confirmation levels require a backend capability that does not exist;
- the bundle or CSS budget cannot be met.

## 14. Deliverables

The eight data-state components with `DataBoundary`, root and route error
boundaries, the three confirmation levels with their action mapping, the three
feedback surfaces and the pending-on-object pattern, the base component set,
the Radix consolidation, the migrated existing routes, the full test suite
above, and the runbook section.
