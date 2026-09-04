---
id: WCX-15
phase: 5
wave: capability
title: Security hardening, theme completion, regression suites
status: draft
risk: high
components:
  - web-console
  - control-plane
decision_refs:
  - WC-D25
  - WC-D12
  - WC-D26
  - WC-D28
  - WC-D30
  - SA-11
  - SEC-08
  - SEC-09
  - OPS-04
  - PERF-07
  - PERF-08
acceptance_refs:
  - Phase 5 exit gate items for security review, performance benchmark, and full browser E2E
  - P5-W13 full browser E2E
  - P5-W14 security review
depends_on:
  - WCX-14
  - WCX-19
integration_dependencies:
  - P5-W9
  - P5-W13
  - P5-W14
allowed_paths:
  - "apps/web-console/src/**"
  - "apps/web-console/index.html"
  - "apps/web-console/package.json"
  - "apps/web-console/test/**"
  - "apps/web-console/vite.config.ts"
  - "package-lock.json"
  - "tests/e2e/web-console/**"
  - "apps/control-plane/internal/api/server.go"
  - "apps/control-plane/internal/api/server_test.go"
  - "apps/control-plane/internal/api/web_console.go"
  - "apps/control-plane/internal/api/web_console_test.go"
  - "security/wcx-15-web-console-hardening-review.md"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-15.md"
forbidden_paths:
  - "apps/edge-agent/**"
  - "openapi/**"
  - "proto/**"
  - "apps/control-plane/internal/auth/**"
  - "apps/control-plane/internal/storage/**"
  - "apps/control-plane/internal/devicechannel/**"
  - "0-planning-documents/**"
requires_owner_review: true
requires_security_review: true
---

# WCX-15 — Security hardening, theme completion, regression suites

## 1. Purpose

Close the Web Console for MVP release: complete the browser security header
set including Trusted Types, add system-preference light theme, add the
user-triggered diagnostic report, and stand up the visual and performance
regression suites.

## 2. Why now

Phase 5 is the pilot hardening and release phase. `P5-W14` requires a manual
security review covering web session, CSP, CSRF, and hostile rendering;
`P5-W13` requires the full browser path; `P5-W9` requires the performance
benchmark. The console-side work those gates depend on is collected here.
Trusted Types and visual regression were deliberately deferred to this point
so they land against a stable component and severity system rather than a
moving one.

## 3. Inputs and decisions

- `WC-D25` — the remaining headers: explicit `object-src 'none'`,
  Cross-Origin-Opener-Policy, Cross-Origin-Resource-Policy, and
  `require-trusted-types-for 'script'` with a Trusted Types policy.
- `WC-D12` — system-preference light theme, no manual toggle, no persistence.
- `WC-D26` — visual regression, deferred from `WCX-06`, lands here.
- `WC-D28` — user-triggered diagnostic report copied to the clipboard under a
  strict field allowlist; no automatic telemetry and no third-party service.
- `WC-D30` — performance regression enforcement matures into the versioned
  benchmark.
- `SA-11`, `SEC-08`, `SEC-09`, `OPS-04`, `PERF-07`, `PERF-08`.

## 4. Dependencies

`WCX-14` and `WCX-19` must be accepted, so that hardening, visual regression,
and the full browser path cover every delivered surface including onboarding,
corrections, notification channels, updates, and operational health. `P5-W9`,
`P5-W13`, and `P5-W14` are integration dependencies; this package supplies the
console-side evidence those gates consume.

## 5. Scope

1. Complete the security header set and adopt Trusted Types.
2. Add the light theme under `prefers-color-scheme`.
3. Build the user-triggered diagnostic report.
4. Stand up visual regression.
5. Stand up the versioned performance benchmark and its regression gate.
6. Complete the console-side security review for `P5-W14`.

## 6. Non-goals

- No new product capability, screen, or endpoint.
- No manual theme toggle and no theme persistence. `WC-D12` decided
  system-preference only.
- No automatic error telemetry, no beacon, no third-party monitoring service,
  and no new backend endpoint for diagnostics.
- No change to Control Plane authentication, session, storage, or device
  channel. The permitted backend surface is the HTTP header middleware and the
  static asset handler only.
- No external penetration test. `P5-W14` records it as approved before wider
  beta, not as a first-pilot blocker.
- No `Strict-Transport-Security` `preload` directive; that remains a separate
  owner decision because it is difficult to reverse.

## 7. Allowed paths

Only the paths in frontmatter. The Control Plane surface stays narrowed to the
header middleware and the static handler.

## 8. Security constraints

1. Trusted Types must be enforced, not merely reported. `require-trusted-types-for
   'script'` with a named policy, and a `trusted-types` directive listing only
   that policy. The console must require no policy at all if it creates no
   sink; if a dependency requires one, the policy is minimal, named, and
   documented.
2. Adopting Trusted Types must not require loosening any existing directive.
   `script-src 'self'`, the absence of `unsafe-inline` and `unsafe-eval`,
   `frame-ancestors 'none'`, and `default-src 'none'` all remain.
3. The diagnostic report is an allowlist, never a redaction pass. Only the
   fields in 9.3 may appear. Anything not on the list is absent by
   construction, so a new field cannot leak by being forgotten.
4. The diagnostic report must never contain session or CSRF values, TOTP or
   recovery codes, bootstrap or enrollment material, passwords, incident or
   evidence content, AI output, notification content, device or zone names,
   full URLs with identifiers, or any exception message.
5. The report is produced only on explicit operator action and is written only
   to the clipboard. It is never transmitted, never stored, and never written
   to a file by the console.
6. Visual regression baselines must never be captured while any one-time
   secret, credential attempt, or captured evidence payload is on screen.
   Baselines use fixture data from the `WCX-06` corpus and the component
   workbench, never live incident data.
7. The light theme must satisfy the same contrast requirements as dark, proven
   by the same automated check, and must preserve palette disjointness from
   `WCX-03`.
8. Production source maps remain forbidden.

## 9. Implementation requirements

### 9.1 Security headers and Trusted Types

Added to the existing `securityHeaders` middleware:

| Header or directive | Value |
|---|---|
| CSP `object-src` | `'none'`, stated explicitly rather than inherited |
| CSP `require-trusted-types-for` | `'script'` |
| CSP `trusted-types` | the minimal named policy set, or none if no sink exists |
| `Cross-Origin-Opener-Policy` | `same-origin` |
| `Cross-Origin-Resource-Policy` | `same-origin` |

Requirements:

1. No existing header or directive value is weakened.
2. Static asset responses keep the immutable cache policy; `index.html` and
   client-route fallbacks keep `no-store`; API and secret responses keep
   `no-store`. Unknown `/v1` paths keep returning JSON `404`.
3. Handler tests assert every header value and assert that the previously
   established headers are unchanged.
4. A browser test confirms the console operates correctly with Trusted Types
   enforced in a browser that supports it, and degrades to existing behaviour
   in one that does not.

### 9.2 Light theme

1. `semantic.css` gains a `@media (prefers-color-scheme: light)` block
   containing only token reassignments, as `WCX-03` structured it. No
   component or layout file changes.
2. `index.html` `color-scheme` becomes `light dark`.
3. No toggle, no persistence, no storage access of any kind.
4. The `WCX-03` contrast test runs over both themes; both must pass.
5. The `WCX-03` disjointness test runs over both themes.
6. Every status glyph, severity ramp, and focus ring is verified in light.

### 9.3 Diagnostic report

An explicit `Copy diagnostic report` control in the account area. It composes
a plain-text report from this exact allowlist and nothing else:

| Field | Source |
|---|---|
| Console version and build identifier | build-time constant |
| Browser name and version | user agent, parsed to name and major version only |
| Viewport size and device pixel ratio | measured |
| Reduced-motion and colour-scheme preference | media queries |
| Current route pattern | the route pattern, never the resolved path or its identifiers |
| Last error kind and `messageKey` | `ConsoleError` fields only |
| Last error HTTP status and backend status slug | `ConsoleError` fields only |
| Last error timestamp | UTC ISO-8601 |
| Last five freshness-class refresh outcomes | class name and outcome only |
| Whether a session is present and whether a CSRF proof is held | booleans only |

Requirements:

1. The report is assembled by an allowlist builder; no object is spread and no
   value is copied wholesale.
2. Before copying, the report is shown to the operator in full so they can see
   exactly what will be shared.
3. A test enumerates the produced fields and fails if any field outside the
   allowlist appears.
4. A test seeds session values, CSRF proofs, secrets, incident content, and
   evidence payloads into the running application and asserts none appears in
   the report.
5. Copy failure is reported inline and never silently appears to succeed.

### 9.4 Visual regression

1. Baselines are captured from the `WCX-06` component workbench and from
   fixture-driven route renders, never from live data.
2. Coverage: every shared component state from `WCX-04`, every severity,
   confidence, health, and device-state value from `WCX-03`, and the primary
   routes, each in light and dark, at 320, 375, 900, and 1440 pixels.
3. Runs with reduced motion forced and with animations disabled so results are
   deterministic.
4. A difference above a recorded threshold fails and requires an explicit
   baseline update in the pull request, with the visual change described.
5. Baselines are committed and reviewed like code.

### 9.5 Performance benchmark and regression gate

1. The `WCX-07` load budgets and the `WCX-12` interaction budget become a
   single versioned benchmark run in CI and before release.
2. Measurements: initial login load, initial authenticated load, total bundle,
   time to interactive on the reference dataset, incident-list interaction
   latency at 200 rows, incident-detail time to interactive with a large
   journey and evidence set, and the interaction-to-visibility latency feeding
   `PERF-05`.
3. Results are versioned per `PERF-08`. A regression above twenty percent in
   any measure fails and requires owner review.
4. The `WCX-07` recorded transport trigger is evaluated against the benchmark
   results. If it fires, a change proposal for a server-driven invalidation
   channel is opened rather than the threshold being adjusted.

### 9.6 UI/UX requirements

1. The light theme must be a faithful counterpart, not a lighter tint. Severity
   ordering, health distinctness, and the confidence indicator must read
   equally well in both.
2. The diagnostic report control is discoverable in the account area and
   explains what it collects before it collects it.
3. No visual change to dark theme beyond what a light-theme token
   restructuring unavoidably requires; any such change is listed in the
   evidence.

### 9.7 Accessibility requirements

1. Contrast passes in both themes for every defined pairing.
2. The diagnostic report preview is a readable region with an accessible name;
   the copy control states what is copied.
3. Copy success and failure are announced once.
4. Visual regression coverage includes focus-visible states so a focus ring
   regression is caught.
5. The full `P5-W13` browser path passes an axe scan at every step.

### 9.8 API and data contracts

No contract change. The only backend change is response headers.

### 9.9 Error and failure behaviour

1. If Trusted Types enforcement breaks a dependency, the correct response is
   to replace or fix the dependency, not to relax the directive. If neither is
   possible, stop and escalate.
2. A clipboard failure reports inline.
3. A light-theme contrast failure fails the build; it is not waived.
4. A benchmark regression fails the build and requires owner review.

### 9.10 Internationalisation and theme

The diagnostic report is fixed English technical output and is deliberately
not part of the operator catalogue, because it is a support artefact rather
than product language. Every other new string enters the `WCX-08` catalogue.

### 9.11 Observability

The diagnostic report is the console's only diagnostic surface, and it is
manual, local, and allowlisted. No automatic telemetry exists anywhere in the
console.

### 9.12 Documentation

Add `Security headers`, `Theme`, `Diagnostics`, and `Regression suites`
sections to `docs/runbooks/web-console/development.md`. Record
`security/wcx-15-web-console-hardening-review.md` covering trust boundaries,
web session, CSP, CSRF, hostile rendering, prompt-injection rendering, secret
lifetime, dependency and licence review, and residual risk, in the established
format, as the console-side input to `P5-W14`.

## 10. Required tests

### 10.1 Unit and component

1. The diagnostic report contains exactly the allowlisted fields and nothing
   else.
2. Seeded session, CSRF, secret, incident, evidence, and AI values never
   appear in the report.
3. The report shows the operator its content before copying; copy failure is
   reported.
4. Contrast and disjointness pass in both themes for every pairing.
5. No storage access occurs when the colour scheme changes.
6. Every status glyph and severity step is distinguishable in light theme.

### 10.2 Backend

1. Handler tests assert every new header and directive value.
2. Previously established headers, cache policies, and the `/v1` `404`
   behaviour are unchanged.
3. `go -C apps/control-plane test ./...` passes.

### 10.3 Browser and E2E scenarios

1. The full `P5-W13` path in Chromium, Firefox, and WebKit: login with TOTP,
   environment, Edge enrolment, zones, decoy deployment, test attack, incident,
   journey, AI, notification, and disposition.
2. The same path repeated in light theme.
3. Trusted Types enforced: the full path completes with no policy violation
   reported in a supporting browser.
4. The `WCX-06` hostile corpus and the `WCX-14` prompt-injection corpus both
   re-run against the final build as release-gate evidence for `SEC-08` and
   `SEC-09`.
5. Storage remains empty across the entire path.
6. Axe scan at every step, both themes, at a narrow and a wide viewport.
7. Diagnostic report generated and inspected for disallowed content.

Secret-bearing steps keep traces, video, and automatic screenshots disabled.
Only explicit post-dismissal screenshots are retained.

### 10.4 Regression suites

Visual regression across the coverage in 9.4, and the performance benchmark
of 9.5 with versioned results.

## 11. Acceptance criteria and Definition of Done

1. The complete header set including enforced Trusted Types is present, with
   no existing directive weakened.
2. Light theme works under system preference, with no toggle and no storage,
   and passes contrast and disjointness in both themes.
3. The diagnostic report is allowlist-built, operator-triggered,
   clipboard-only, and provably free of excluded content.
4. Visual regression is running with committed baselines from fixture data.
5. The performance benchmark is versioned and gates regressions above twenty
   percent.
6. The full `P5-W13` path passes in three browsers in both themes, with the
   hostile and prompt-injection corpora re-run against the final build.
7. The console-side security review is recorded for `P5-W14`.

## 12. Evidence required

- Response header capture from a real TLS request.
- Trusted Types enforcement evidence with no violation across the full path.
- Contrast reports for both themes.
- Diagnostic report sample plus the seeded-secret exclusion test output.
- Visual regression baseline set and a demonstration diff.
- Versioned benchmark results with comparison against the previous run and
  against `PERF-05` and `PERF-07`.
- Full browser path evidence in three engines and two themes.
- Hostile and prompt-injection corpus results against the final build.
- Storage-empty assertions across the full path.
- Dependency and licence review for the final dependency set.
- `security/wcx-15-web-console-hardening-review.md`.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- Trusted Types cannot be enforced without weakening a directive or without a
  dependency change that exceeds this package;
- a light-theme contrast requirement cannot be met without changing the
  severity or health palette agreed in `WCX-03`;
- the diagnostic allowlist proves insufficient for real pilot support, which
  would reopen `WC-D28`;
- the benchmark fires the `WCX-07` transport trigger, which requires a change
  proposal rather than a threshold adjustment;
- a `PERF-05` or `PERF-07` target cannot be met within the approved
  architecture;
- the security review surfaces a finding that cannot be closed inside this
  package.

## 14. Deliverables

The completed security header set with enforced Trusted Types, the
system-preference light theme with dual-theme contrast verification, the
allowlisted operator-triggered diagnostic report, the visual regression suite
with committed fixture-based baselines, the versioned performance benchmark
and regression gate, the full browser path evidence in three engines and two
themes, the re-run hostile and prompt-injection corpora, the console-side
security review for `P5-W14`, and the four runbook sections.
