---
id: WCX-000
title: Web Console extended architecture and decision record
status: owner-decided
decided_on: 2026-09-03
decided_by: Product Owner
supersedes: none
components:
  - web-console
---

# WCX-000 — Web Console extended architecture and decision record

## 1. Purpose and authority

This record closes the material Web Console architecture, UI/UX, security,
quality, and developer-experience decisions that were open after `P1-W11`
delivered the Phase 1 console shell. It exists so that implementation packages
`WCX-01` through `WCX-21` can be executed without re-deriving architecture.

This record is **subordinate** to the approved planning documents. Where it
appears to conflict with `0-planning-documents/**`, the planning documents win
and this record must be corrected. It changes no approved product scope,
roadmap phase ownership, contract, or security boundary on its own; the two
items that would do so are routed to change proposals (§6).

- Decisions `WC-D01` through `WC-D32` were researched and recommended on
  2026-09-03 and accepted by the Product Owner on 2026-09-03.
- `WC-D31`'s open sub-question (phase placement) was decided by the Product
  Owner on 2026-09-03: **Phase 2 start**.

## 2. Reconfirmed approved baseline — not reopened

The following remain APPROVED and were not reconsidered. Any future proposal to
change one is a change proposal, not a frontend decision.

| Decision | Content |
|---|---|
| TS-02 | React + TypeScript |
| TS-03 | Vite; SSR framework is not the default |
| DT-07, CP-08 | Authenticated static SPA served from the Control Plane origin |
| CP-04, TS-06 | REST/JSON with OpenAPI for the web/public API |
| RE-10 | Generated typed client or thin typed wrapper; no DTO duplication |
| TS-04 | TanStack Query-class server state; React primitives for local state; no Redux-like global store without proven cross-cutting need |
| TS-05 | Explicit React Router-class route tree |
| IA-02, IA-05 | Local authentication; server-side session with HttpOnly cookie |
| IA-04 | TOTP for the privileged operator |
| AUTH-04 | OIDC/SSO is not an MVP baseline |
| IA-06, AUTH-05 | Fine-grained configurable RBAC is not an MVP baseline |
| UX-01 | Incident-first primary screen; health/coverage secondary |
| EV-04, SRC-07 | Evidence and provenance-aware wording precede inference |
| SEC-07, SEC-08 | Attacker-controlled data is untrusted; hostile HTML/JS/ANSI/Markdown must not execute |
| UX-08 | No advanced topology map |
| W11-C2-A | Radix Primitives with CSS Modules and CSS variables; no Redux-like store, SSR, unsafe HTML, or inline-script requirement |

Web Console product correctness depends on no external operations tooling.

## 3. Closed decision register

Status column: `ACCEPTED` means the recommended direction was accepted by the
Product Owner on 2026-09-03.

### 3.1 Architecture and contract foundation

| ID | Decision | Outcome | Status | Package |
|---|---|---|---|---|
| WC-D01 | Module boundaries | Feature-sliced `src/features/**`, `src/shared/**`, `src/app/**`; public API only via `index.ts`; deep imports blocked by lint | ACCEPTED | WCX-01 |
| WC-D02 | API client generation | `openapi-typescript` type generation only, committed under `src/generated/`, with a CI freshness check; the existing thin `request()` wrapper is retained; no runtime client dependency | ACCEPTED | WCX-02 |
| WC-D03 | Error contract | Frontend `toConsoleError()` taxonomy now; backend body extension routed to change proposal `0004` | ACCEPTED | WCX-02, CP-0004 |
| WC-D04 | Server-state conventions | Typed query-key factories per feature, `queryOptions()` helpers, feature-local invalidation, cursor pagination via `useInfiniteQuery` | ACCEPTED | WCX-02 |
| WC-D05 | Data freshness transport | Centralised polling `freshnessPolicy` now; SSE is not adopted; a measured trigger condition is recorded for a future change proposal | ACCEPTED | WCX-07 |
| WC-D06 | Router mode and splitting | Keep the explicit route tree without router loaders/actions; route-level `lazy()` splitting; upgrade React Router 7 to 8 as maintenance | ACCEPTED | WCX-07 |
| WC-D07 | Authorization seam | Single `useCapability(action)` seam returning `true` for the owner; controls are disabled with a reason, never hidden; no role model is implemented | ACCEPTED | WCX-01 |
| WC-D08 | Re-authentication UX | Add `POST /v1/auth/csrf` for CSRF-proof re-issue without MFA, plus step-up reauthentication for sensitive actions; routed to change proposal `0003` | ACCEPTED | CP-0003, WCX-09 |

### 3.2 Design system and visual language

| ID | Decision | Outcome | Status | Package |
|---|---|---|---|---|
| WC-D09 | Design system | Continue Radix Primitives + CSS Modules; consolidate to the single `radix-ui` package; headless TanStack Table/Virtual are permitted and are not a second design system | ACCEPTED | WCX-04 |
| WC-D10 | Design tokens | Two-layer primitive/semantic tokens owned by one file; components may use semantic tokens only; enforced by lint | ACCEPTED | WCX-03 |
| WC-D11 | Severity encoding | Brand/chrome, severity, and health palettes are disjoint; confidence is never colour-coded; every severity or status indicator carries icon + text + colour | ACCEPTED | WCX-03 |
| WC-D12 | Theme model | Dark-only through Phase 4; system-preference light support in Phase 5; no manual toggle and no browser storage for theme | ACCEPTED | WCX-03, WCX-15 |
| WC-D13 | Motion policy | Bounded motion budget; severity, confidence, and health are never encoded by motion; timelines never auto-play | ACCEPTED | WCX-03 |

### 3.3 UX architecture

| ID | Decision | Outcome | Status | Package |
|---|---|---|---|---|
| WC-D14 | Information architecture | Global incident-first root; environment is a scope selector carried in a URL query parameter | ACCEPTED | WCX-10 |
| WC-D15 | Data-state matrix | Eight canonical states: loading, empty, unknown, stale, partial, degraded, denied, error; no state may render as healthy | ACCEPTED | WCX-04 |
| WC-D16 | Destructive actions | Three levels: reversible, destructive-recoverable, irreversible-security; L3 requires typed confirmation and step-up reauthentication | ACCEPTED | WCX-04, WCX-09 |
| WC-D17 | Feedback surfaces | Inline, banner, and toast with fixed semantics; errors are never toast-only; long-running work is shown on the object itself | ACCEPTED | WCX-04 |
| WC-D18 | Time presentation | Local time with a visible zone abbreviation, UTC ISO-8601 on detail, seconds in evidence views, relative time only alongside absolute, degraded `clock_quality` marks affected timestamps | ACCEPTED | WCX-08 |
| WC-D19 | Large collections | Headless TanStack Table and TanStack Virtual, cursor pagination, filter state in URL query parameters, virtualisation above 200 rendered rows | ACCEPTED | WCX-12 |
| WC-D20 | Journey rendering | Semantic HTML and CSS timeline with no charting library; every visual has an accessible text or table equivalent | ACCEPTED | WCX-13 |

### 3.4 Forms, internationalisation, accessibility

| ID | Decision | Outcome | Status | Package |
|---|---|---|---|---|
| WC-D21 | Form stack | React Hook Form with a Standard Schema validator (Zod-class); schemas derive from generated OpenAPI types; client validation is UX only, the backend remains authoritative | ACCEPTED | WCX-11 |
| WC-D22 | Internationalisation | English only, i18n-ready: all operator-facing text lives in one catalogue, no literal strings in components, backend-originated text is never translated | ACCEPTED | WCX-08 |
| WC-D23 | Accessibility | WCAG 2.2 Level AA, enforced by `eslint-plugin-jsx-a11y`, component-level axe assertions, and full-page browser axe scans | ACCEPTED | WCX-05 |

### 3.5 Security

| ID | Decision | Outcome | Status | Package |
|---|---|---|---|---|
| WC-D24 | Hostile-content renderer | Canonical untrusted-text contract: no raw HTML anywhere, ANSI and control characters shown as visible escapes rather than interpreted, bidi and zero-width characters made visible, attacker-supplied URLs never clickable, filenames never used as paths or download targets, bounded length with a truncation indicator, clipboard only on explicit action | ACCEPTED | WCX-06 |
| WC-D25 | Browser security headers | `Strict-Transport-Security` and `Permissions-Policy` now; explicit `object-src 'none'`, COOP, CORP, and Trusted Types in Phase 5 | ACCEPTED | WCX-06, WCX-15 |

### 3.6 Quality, developer experience, observability

| ID | Decision | Outcome | Status | Package |
|---|---|---|---|---|
| WC-D26 | Test tooling | Migrate network mocking to MSW with handlers typed against generated OpenAPI types and `onUnhandledRequest: 'error'`; add component-level axe; visual regression deferred to Phase 5 | ACCEPTED | WCX-06, WCX-15 |
| WC-D27 | Component workbench | Repository-local development-only component route; no Storybook; a build test proves it never reaches the production bundle | ACCEPTED | WCX-06 |
| WC-D28 | Frontend observability | User-triggered diagnostic report copied to the clipboard under a strict field allowlist; no automatic telemetry and no third-party service | ACCEPTED | WCX-15 |
| WC-D29 | Dependency admission | Every new console dependency records purpose, gzipped size, licence, install-script status, transitive count, and maintenance evidence in its pull request | ACCEPTED | WCX-01 |
| WC-D30 | Performance budgets | Three-dimensional budget: initial route bundle, total bundle, and runtime interaction; regression above twenty percent requires owner review | ACCEPTED | WCX-07, WCX-12 |

### 3.7 Scope

| ID | Decision | Outcome | Status | Package |
|---|---|---|---|---|
| WC-D31 | Phase 1 UI gap | Add an operator-completeness package covering device disable/revoke, enrollment-token list/revoke, zone edit/delete, session list/revoke, and password change. **Placed at Phase 2 start.** The Phase 1 gate may close on capability evidence. An organization-level audit viewer is deferred to Phase 3 | ACCEPTED | WCX-09 |
| WC-D32 | Package decomposition | Twenty-one packages, `WCX-01` to `WCX-21`, with phase labels in §5 | ACCEPTED | this record |

### 3.8 Coverage corrections — 2026-09-04

A post-authoring coverage audit against the approved decision set found five
operator-facing surfaces that the first decomposition omitted. They are
approved capabilities with no console home, not new scope, and they are added
as `WCX-16` through `WCX-19`. No roadmap capability is pulled earlier than its
approved phase.

| Omitted surface | Approved basis | Now covered by |
|---|---|---|
| `ON-01` steps 7, 8, 9, 11, and 12: consent-gated placement validation, deterministic placement proposal with override, deployment approval, coverage verification, environment readiness | ON-01..ON-06, OPS-02, AC-ON-005, AC-ON-006 | WCX-16 |
| Expected-source and scoped suppression management | CS-05, CS-06, AC-CF-003, AC-CF-004, P3-W11 | WCX-17 |
| Incident merge and split | COR-06, AC-INC-004, P3-W8 | WCX-17 |
| Notification channel configuration and escalation contacts | NT-02, NT-03, NT-06, NT-07, AC-NT-002, AC-NT-003, P4-W11, P4-W12 | WCX-18 |
| Update and rollback control, the full `UX-07` health minimum including queue and explicit loss state and AI provider status, and the diagnostics bundle entry point | UP-01..UP-04, UX-07, OPS-04, P5-W3, P5-W4, P5-W8, P5-W11 | WCX-19 |
| Synthetic honey credential generation, one-time reveal, manual placement, and trigger history | DC-09, CS-03, DATA-02, P2-W9 | WCX-20 |
| Retention configuration and purge visibility | DATA-01, DATA-06, EV-05 | WCX-21, backend assigned by CP-0006 |
| Device re-enrollment recovery path | SEC-06 | WCX-09, level 3 |
| Decoys of a disabled or revoked device marked unmanaged and kept visible | SEC-06 | WCX-11 section 8.8 |
| Emulated persona disclosure, no claim of a real Windows host | AC-SMB-002, DC-11 | WCX-11 section 8.9 |

A second sweep on 2026-09-04 produced `WCX-20` and the two `WCX-11`
additions above. `DC-10` honey files, documents, and tokens, `DET-04` custom
detection rules, `COR-07` historical reprocessing, `NT-04` native Slack and
Teams integrations, `UP-05` offline update bundles, `UP-06` staged fleet
rollout, and `UX-08` advanced topology map were each confirmed as approved
exclusions and correctly carry no console surface.

## 3.9 Device re-enrollment — decided 2026-09-04

`WC-D31`'s accepted list of operator-completeness screens did not include
`POST /v1/environments/{environmentId}/devices/{deviceId}/re-enrollment-token`,
so `WCX-09` records it as a non-goal.

The second sweep found that `SEC-06` describes the recovery path in its own
approved text: decoys of a disabled or revoked device stay marked unmanaged
"until explicitly removed **or re-enrolled**". That makes re-enrollment a
state the approved decisions already anticipate an operator reaching, and the
console currently offers no path to it. A device whose certificate expired, or
one revoked during an investigation that was later cleared, cannot be restored
from the product.

**Product Owner decision, 2026-09-04:** device re-enrollment is added to
`WCX-09`'s scope at **confirmation level 3**. It is the inverse of revocation
and re-establishes the trust revocation removed, so it carries the same gate as
revoke: typed confirmation of the device name plus step-up reauthentication.

Consequences recorded in `WCX-09`:

- `POST /v1/environments/{environmentId}/devices/{deviceId}/re-enrollment-token`
  is consumed, and re-enrollment appears in the device lifecycle section;
- the issued token is one-time material under the `W11-C3-A` rules, with no
  re-display path;
- issuing a token changes no device state; the device becomes active only when
  the backend reports completed enrollment, so the console never shows a
  device as recovered because a token was issued;
- a browser scenario re-enrolls a revoked device end to end and confirms its
  decoys stop being marked unmanaged, closing the `SEC-06` recovery path that
  `WCX-11` renders.

## 3.10 Retention configuration — decided 2026-09-04

`DATA-01` approves data-class-specific retention with **configurable values**
across six classes, and defers the exact default day counts to a final MVP
specification owner decision. `DATA-06` approves an environment-level
retention and purge job and forbids individual evidence deletion from the
incident UI.

No roadmap package owns a retention configuration surface. `P5-W10` exercises
retention and purge behaviour as a storage benchmark, not as an operator
screen. This planning set therefore does **not** contain a retention
configuration package, because inventing one would assign a console surface
to a capability whose ownership and defaults the roadmap has not settled.

**Product Owner decision, 2026-09-04:** retention values are
operator-configurable and the surface is delivered **now**, as `WCX-21` at
Phase 2, rather than deferred. The reason it belongs early is that retention
governs data the product starts collecting in Phase 2 — normalized events and
evidence, raw transcripts, and quarantined hostile files — so deferring the
surface means collecting under a policy the operator cannot see or change.

Two things this decision does **not** settle, and which `WCX-21` therefore
does not invent:

1. **The exact default day counts per class.** `DATA-01` explicitly defers
   them to a final MVP specification owner decision. `WCX-21` reads defaults,
   bounds, and effective values from the backend and hardcodes none.
2. **The backend owner.** No roadmap package in Phase 1 through Phase 5
   delivers retention configuration endpoints; `P5-W10` exercises retention
   and purge only as a storage benchmark. `WCX-21` therefore carries
   `blocked_by_missing_backend_owner: true` and is not promotable until the
   Product Owner assigns a backend package that owns reading the effective
   policy per class, writing an operator value within bounds, and reporting
   purge execution and outcome. That assignment is a roadmap decision this
   planning set cannot make.

## 4. Recorded implementation defects in delivered `P1-W11` output

These are corrections to already-delivered Phase 1 console code, not new
capability. Each is assigned to a package below. The Product Owner may require
`GAP-1` and `GAP-2` before Phase 1 gate closure; neither is required by this
record to close the gate.

| ID | Defect | Evidence | Package |
|---|---|---|---|
| GAP-1 | The operator block containing sign-out and re-authenticate is hidden below 900 pixels, so an operator cannot end a session on a narrow viewport | `apps/web-console/src/styles/app.module.css` media query at 900 pixels | WCX-10 |
| GAP-2 | No React error boundary exists; an unexpected exception blanks the whole console | absence across `apps/web-console/src` | WCX-04 |
| GAP-3 | The client sets a forbidden `Origin` request header that browsers ignore; the backend check relies on the browser-supplied header, so this is inert but misleading code | `apps/web-console/src/api/client.ts` | WCX-02 |
| GAP-4 | Route changes do not move focus, do not announce, and never change the document title | `apps/web-console/src/App.tsx` and route components | WCX-05 |
| GAP-5 | ESLint runs without `eslint-plugin-react-hooks`, without `jsx-a11y`, and without type-aware rules; `vite.config.ts` and `test/check-bundle.mjs` are never typechecked | `apps/web-console/eslint.config.js`, `apps/web-console/package.json` | WCX-01 (hooks, type-aware, config typecheck); WCX-05 (`jsx-a11y`) |
| GAP-6 | `formatTime` and `formatExpiry` duplicate one function; dynamic CSS-module class lookup is untyped | `HealthPanel.tsx`, `SecretDialog.tsx` | WCX-08 |

## 5. Package to phase mapping

`wave: foundation` packages are behaviour-preserving structural and defect
remediation work. They add no product capability and pull no roadmap capability
earlier, following the precedent set by `P1-G1`.

| Package | Phase | Wave | Title |
|---|---|---|---|
| WCX-01 | 2 | foundation | Module boundaries, capability seam, lint and typecheck hardening |
| WCX-02 | 2 | foundation | Generated OpenAPI types, API transport, and error taxonomy |
| WCX-03 | 2 | foundation | Design tokens, severity semantics, theme and motion policy |
| WCX-04 | 2 | foundation | Shared component layer, data-state matrix, error boundary |
| WCX-05 | 2 | foundation | Accessibility baseline and enforcement |
| WCX-06 | 2 | foundation | Test tooling, hostile-content contract, component workbench |
| WCX-07 | 2 | foundation | Freshness policy, code splitting, performance budgets |
| WCX-08 | 2 | foundation | Text catalogue and canonical timestamp presentation |
| WCX-09 | 2 | capability | Operator completeness and sensitive-action reauthentication |
| WCX-10 | 2 | foundation | Navigation information architecture and responsive correction |
| WCX-11 | 2 | capability | Form and validation stack with decoy management UI |
| WCX-12 | 3 | capability | Incident-first dashboard, tables, pagination, virtualisation |
| WCX-13 | 3 | capability | Incident detail, attacker journey, evidence explorer |
| WCX-14 | 4 | capability | AI explanation, notification centre, disposition |
| WCX-15 | 5 | capability | Security hardening, theme completion, regression suites |
| WCX-16 | 2 | capability | Guided onboarding, placement validation, coverage verification |
| WCX-17 | 3 | capability | Expected-source management and incident merge and split |
| WCX-18 | 4 | capability | Notification channel configuration and escalation contacts |
| WCX-19 | 5 | capability | Update and rollback, operational health, diagnostics bundle |
| WCX-20 | 2 | capability | Synthetic honey credential workflow |
| WCX-21 | 2 | capability | Retention configuration and purge visibility |

`WCX-15` runs last within Phase 5, after `WCX-19`, so its hardening,
regression, and full-browser evidence covers every delivered surface.

## 6. Routed change proposals

Two accepted decisions alter an approved contract or an approved
implementation contract and therefore cannot be executed on this record alone.

| Proposal | Decision | Was blocking | Status |
|---|---|---|---|
| [`0003`](../../change-proposals/0003-web-console-session-csrf-reissue.md) | WC-D08 | `WCX-09` sensitive-action reauthentication | APPROVED 2026-09-04 |
| [`0004`](../../change-proposals/0004-web-console-error-contract.md) | WC-D03 option B | `WCX-11` field-level validation display | APPROVED 2026-09-04 |
| [`0005`](../../change-proposals/0005-agent-work-package-issue-authority.md) | AP-07, AP-15 amendment | work-package issue creation | APPROVED 2026-09-04 |
| [`0006`](../../change-proposals/0006-retention-configuration-ownership.md) | DATA-01, DATA-06 ownership | `WCX-21` | APPROVED 2026-09-04 |

Both proposals were approved subject to every constraint in their
Recommendation sections; those constraints are binding on `WCX-09` and
`WCX-11`.

`WC-D03` option A is not blocked and is implemented in `WCX-02`.

## 7. Architecture record

Decisions `WC-D01`, `WC-D02`, `WC-D05`, `WC-D06`, `WC-D09`, and `WC-D24` are
recorded durably in
[ADR 0018](../../adr/0018-web-console-frontend-architecture.md), accepted by
`@sinanganiz` on 2026-09-04.

## 8. Traceability to approved acceptance criteria

| Approved reference | Where it is satisfied |
|---|---|
| UX-01 incident-first home | WCX-10 information architecture, WCX-12 dashboard |
| UX-02 incident list columns | WCX-12 |
| UX-03 incident detail sections | WCX-13, WCX-14 |
| UX-04 MITRE labels as secondary detail | WCX-13 |
| UX-05 incident-scoped evidence explorer | WCX-13 |
| UX-06 decoy management minimum | WCX-11 |
| UX-07 environment and Edge health UX, all seven elements | WCX-19, with coverage from WCX-16 and channel health from WCX-18 |
| ON-01 twelve-step onboarding flow | WCX-09, WCX-10, WCX-11, WCX-16 |
| ON-02, ON-03 bounded, consent-gated probing | WCX-16 |
| ON-04, ON-05 deterministic placement proposal and override | WCX-16 |
| ON-06, AC-ON-005, AC-ON-006 coverage verification | WCX-16 |
| CS-05, CS-06, AC-CF-003, AC-CF-004 expected sources and suppression | WCX-17 |
| COR-06, AC-INC-004 manual merge and split | WCX-17 |
| NT-02, NT-03, NT-07, AC-NT-002, AC-NT-003 channels and contacts | WCX-18 |
| UP-01..UP-04, AC-UP-001..004 update, verification, rollback | WCX-19 |
| OPS-04 diagnostics bundle | WCX-19 |
| P5-W8 explicit evidence-loss state | WCX-19 |
| CS-01, CS-02 confidence and severity separation | WCX-03, WCX-12 |
| SRC-07 provenance-aware wording | WCX-08 text catalogue, WCX-13 |
| SEC-08 hostile UI rendering release blocker | WCX-06 |
| SA-11 browser session, CSP, CSRF | WCX-06, WCX-15 |
| PERF-05 incident visibility 5 s p95 excluding AI | WCX-07 freshness policy, WCX-12 |
| PERF-07 usable page interaction under 2 s | WCX-07 and WCX-12 budgets under WC-D30 |
| RE-10 no DTO duplication | WCX-02 |
| RE-11 testing pyramid | WCX-06 |
| RE-12 permanent security regression fixtures | WCX-06 |
| AIM-08 AI unavailable behaviour | WCX-14 |
| NT-01, NT-06 in-product notification and acknowledgement | WCX-14 |
| OPS-03 staleness | WCX-04 data-state matrix, WCX-07 |

## 9. Standing constraints for every package in this set

1. No package may introduce SSR, a server runtime for the console, a
   Redux-like global store, raw HTML rendering, an inline-script requirement,
   `unsafe-inline`, `unsafe-eval`, or a CDN-loaded runtime dependency.
2. No package may place session, CSRF, TOTP, recovery, bootstrap, enrollment,
   or private-key material in browser storage, URLs, query caches, logs,
   analytics, screenshots, traces, or videos.
3. No package may render backend or attacker-originated content as trusted
   markup, and none may make a health, coverage, or severity signal appear
   healthier than the backend reported.
4. Test doubles remain test-only and can never produce production green state.
5. Every new dependency satisfies `WC-D29`.
6. A package that needs a decision not recorded here stops and escalates
   rather than choosing silently.

## 10. Status

- **Decision status:** CLOSED — 0 open material owner decisions as of
  2026-09-03.
- **Implementation authority:** granted for `WCX-01` through `WCX-20` by
  Product Owner promotion on 2026-09-04. All twenty-one are promoted; CP-0006 assigned
  the retention backend. Promotion is not `READY`; `PM-16`
  dependency gating still applies per package.
- **Change control:** Any material modification of this record requires an
  explicit change proposal and Product Owner approval.
