---
id: WCX-06
phase: 2
wave: foundation
title: Test tooling, hostile-content contract, component workbench
status: draft
risk: high
components:
  - web-console
  - control-plane
decision_refs:
  - WC-D24
  - WC-D25
  - WC-D26
  - WC-D27
  - SEC-07
  - SEC-08
  - SA-11
  - RE-11
  - RE-12
  - DATA-02
  - DATA-03
  - DATA-04
  - DATA-05
acceptance_refs:
  - WCX-000 section 3.5 and 3.6
  - SEC-08 hostile UI rendering release blocker
  - RE-12 permanent security regression fixtures
depends_on:
  - WCX-01
integration_dependencies:
  - WCX-04
allowed_paths:
  - "apps/web-console/src/**"
  - "apps/web-console/package.json"
  - "apps/web-console/vite.config.ts"
  - "apps/web-console/test/**"
  - "package-lock.json"
  - "tests/e2e/web-console/**"
  - "security/wcx-06-hostile-content-review.md"
  - "apps/control-plane/internal/api/server.go"
  - "apps/control-plane/internal/api/server_test.go"
  - "apps/control-plane/internal/api/web_console_test.go"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-06.md"
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

# WCX-06 — Test tooling, hostile-content contract, component workbench

## 1. Purpose

Turn `SEC-08` from a principle into an enforced, testable rendering contract;
bind network mocks to the generated OpenAPI contract; add the two browser
security headers that are missing today; and give the console an isolated
component environment without adding a heavyweight toolchain.

## 2. Why now

Phase 2 and Phase 3 introduce the content classes that actually carry attacker
influence: raw HTTP request bodies, SSH transcripts with ANSI escapes,
attacker-supplied URLs, filenames, and captured credential attempts. Today's
protection is React text escaping plus ad-hoc tests. React escaping stops
script execution but does not stop visual forgery through bidirectional
control characters, ANSI sequences, or zero-width characters. The contract
must exist before the first screen renders that content.

## 3. Inputs and decisions

- `WC-D24` — canonical untrusted-content rendering contract.
- `WC-D25` option B — add `Strict-Transport-Security` and `Permissions-Policy`
  now; the remaining headers and Trusted Types are `WCX-15`.
- `WC-D26` — migrate network mocking to MSW typed against generated OpenAPI
  types with `onUnhandledRequest: 'error'`; component-level axe was added by
  `WCX-05`; visual regression is deferred to Phase 5.
- `WC-D27` — a repository-local, development-only component route; no
  Storybook.
- `SEC-07`, `SEC-08`, `SA-11`, `RE-11`, `RE-12`.
- `DATA-02` to `DATA-05` define the captured content classes.

## 4. Dependencies

`WCX-01` is a hard dependency for the boundary layout. `WCX-04` is an
integration dependency: the workbench renders shared components, so the
workbench route is completed after `WCX-04` merges. A sequential pull-request
chain is permitted for this package.

## 5. Scope

1. Implement the untrusted-content rendering contract and its components.
2. Build the permanent hostile-content fixture corpus.
3. Migrate network mocking from the hand-written stub to MSW typed against
   generated types.
4. Add `Strict-Transport-Security` and `Permissions-Policy` to the Control
   Plane response headers.
5. Add browser-level keyboard traversal and expanded axe scenarios.
6. Build the development-only component workbench route with a build test
   proving it never ships.
7. Record the security review.

## 6. Non-goals

- No visual regression testing. That is `WCX-15`.
- No Trusted Types, COOP, CORP, or `object-src` change. Those are `WCX-15`.
- No Storybook, no second design system, no new UI capability.
- No change to Control Plane authentication, session, storage, device channel,
  or any endpoint behaviour. The only permitted backend change is adding two
  response headers in the existing `securityHeaders` middleware.
- No change to `openapi/guardian.yaml`.
- No evidence download, blob viewing, or file handling implementation. This
  package defines the rules that forbid unsafe forms of them; the screens
  arrive in `WCX-13`.

## 7. Allowed paths

Only the paths in frontmatter. The Control Plane surface is deliberately
narrowed to the HTTP header middleware and its tests, following the narrow
integration-path pattern approved for `P1-W11`. Any other backend need stops
and escalates.

## 8. Security constraints

1. Attacker-controlled telemetry is never trusted UI content. No component may
   render it as markup, as a URL target, as a filename, as a style value, or
   as a class name.
2. `dangerouslySetInnerHTML`, `document.write`, `innerHTML`, `outerHTML`,
   `insertAdjacentHTML`, and `eval` are forbidden by lint across the whole
   console, with no suppression permitted.
3. The header addition must not weaken any existing header. `frame-ancestors`
   remains `'none'`, the CSP remains free of `unsafe-inline` and
   `unsafe-eval`, and API and secret responses remain `no-store`.
4. MSW must never be reachable from a production build. A build test asserts
   no production module resolves the MSW package or the handlers directory.
5. The workbench route must not exist in a production build, must never call a
   real endpoint, and must never render real or realistic secret material. It
   renders fixture data only.
6. Fixtures must not contain real credentials, real hostnames belonging to
   third parties, or working exploit payloads directed at any external system.
   They are inert strings used to prove the renderer neutralises them.

## 9. Implementation requirements

### 9.1 Untrusted-content rendering contract

Two components in `@shared/ui/untrusted/`:

`UntrustedText` for short single-line values such as display names, reasons,
usernames, filenames, and source identifiers. `UntrustedBlock` for multi-line
captured payloads such as HTTP bodies and terminal transcripts.

Both apply the following transformations before rendering, in order:

1. **Control characters.** All C0 and C1 control characters except newline and
   tab in `UntrustedBlock`, and all of them in `UntrustedText`, are replaced
   by a visible escape representation, for example `\x1b`. ANSI escape
   sequences are shown as their escaped source. They are never interpreted and
   never converted into styling.
2. **Bidirectional and directional formatting.** `U+202A` to `U+202E`,
   `U+2066` to `U+2069`, and `U+200F`, `U+200E` are replaced by a visible
   marker so a right-to-left override cannot reorder displayed text.
3. **Zero-width and invisible characters.** `U+200B`, `U+200C`, `U+200D`,
   `U+2060`, `U+FEFF` are replaced by a visible marker.
4. **Unicode normalisation is not applied.** The value is displayed as
   received; normalising could change evidence.
5. **Length bound.** `UntrustedText` truncates at 512 code points and
   `UntrustedBlock` at 64 KiB, each rendering an explicit truncation indicator
   that states the original length. Truncation never occurs silently.
6. **No linkification.** A value that looks like a URL is rendered as text.
   `UntrustedText` and `UntrustedBlock` never emit an anchor, and never set
   `href`, `src`, `srcset`, `action`, `formaction`, `poster`, `data`, or any
   `on*` attribute from data.
7. **No download affordance.** A filename is never used as a `download`
   attribute value, a path segment, or a request path. Copying is available
   only through an explicit operator action.
8. **Clipboard.** Copy is an explicit button, never automatic, and copies the
   original untransformed value. The button states that the copied value is
   attacker-supplied.

Any surface that displays backend or attacker-originated content must use
these components. A lint rule forbids rendering a value typed as untrusted
outside them; the domain types carry a branded `Untrusted<string>` type
introduced here so the compiler enforces routing.

### 9.2 Hostile fixture corpus

A permanent corpus under `@shared/testing/hostile/`, satisfying `RE-12`. It
must include at least: script and event-handler injection attempts, nested
markup, a `javascript:` URL, a `data:` URL, an ANSI colour sequence, an ANSI
cursor-control sequence, a terminal title-setting sequence, a
right-to-left override that reverses a plausible filename, zero-width
characters splitting a keyword, a very long single-token string, a string
containing null bytes, a Markdown link and image, a filename containing path
traversal segments and a trailing space, a filename with a double extension,
and a credential-looking string.

Every fixture is exercised against `UntrustedText`, `UntrustedBlock`, and every
surface that displays backend content. Every future rendering defect adds a
permanent fixture here.

### 9.3 Network mocking migration

1. Add MSW as a development dependency.
2. Handlers live in `@shared/testing/msw/` and are typed against
   `@generated/openapi`, so a contract change that invalidates a mock fails
   typecheck rather than silently passing.
3. `onUnhandledRequest: 'error'` is mandatory, preserving the existing
   guarantee that an unstubbed call can never be mistaken for an authorised or
   healthy outcome.
4. Existing assertions in the migrated tests are preserved; only the mock
   mechanism changes. Header assertions on `X-CSRF-Token` and `If-Match` must
   survive the migration.
5. The previous `stubFetch` harness is removed once every test is migrated.

### 9.4 Browser security headers

In the existing Control Plane `securityHeaders` middleware, add:

- `Strict-Transport-Security: max-age=31536000; includeSubDomains`
- `Permissions-Policy` denying every powerful feature the console does not
  use, at minimum camera, microphone, geolocation, payment, USB, serial,
  Bluetooth, midi, display capture, idle detection, and local fonts.

Requirements:

1. No existing header value changes.
2. `Strict-Transport-Security` is emitted only when the request was served
   over TLS, so a plain-HTTP development listener cannot pin a browser.
3. `preload` is not included; adding it is a separate owner decision because
   it is difficult to reverse.
4. Handler tests assert the presence and exact value of both headers and
   assert that the CSP, referrer policy, and nosniff headers are unchanged.

### 9.5 Component workbench

A route mounted only when the build is a development build, at
`/__components`. It renders every shared component in every state defined by
`WCX-04`, every status value defined by `WCX-03`, and the hostile fixture
corpus. It uses fixture data exclusively and performs no network call.

Exclusion is proven three ways: the route module is imported behind a
build-time condition so it is tree-shaken; the bundle check asserts the
workbench module name appears in no production chunk; and a browser test
against a production build asserts `/__components` returns the SPA fallback
without workbench content.

### 9.6 UI/UX requirements

Escaped content must remain readable. The visible escape representation uses a
distinct, non-alarming treatment drawn from `WCX-03` tokens, is explained once
per block by a short legend, and does not use a severity colour. A truncation
indicator states the original size and offers the copy action for the full
original value.

### 9.7 Accessibility requirements

1. Escape markers carry an accessible description so a screen-reader user
   learns that a control character was present rather than hearing nothing.
2. The copy control has a clear accessible name that includes what is copied
   and that the value is untrusted.
3. `UntrustedBlock` is a scrollable region with a keyboard-reachable scroll
   container and an accessible name.
4. The workbench route passes the axe helper for every rendered component.

### 9.8 API and data contracts

No contract change. The branded `Untrusted<string>` type is applied at the
boundary in the feature API modules created by `WCX-02`, marking every field
whose value originates from a device, a decoy, or captured telemetry.

### 9.9 Error and failure behaviour

1. A value that cannot be transformed, for example due to an unpaired
   surrogate, renders the truncation and escape treatment rather than throwing.
2. A rendering failure inside an untrusted component is caught by the route
   error boundary from `WCX-04` and never blanks the console.
3. A failed clipboard write reports failure inline; it never silently appears
   to succeed.

### 9.10 Internationalisation and theme

Legend text, truncation text, and copy-control names are catalogue-ready
constants. The escape treatment uses semantic tokens only.

### 9.11 Performance

`UntrustedBlock` must render a 64 KiB payload without blocking interaction
beyond the `WCX-07` budget. Transformation runs once per value and is
memoised. MSW, the fixtures, and the workbench are development-only and
contribute zero production bytes, asserted by the bundle check.

### 9.12 Observability

None. Untrusted content is never logged, never sent anywhere, and never
included in any diagnostic surface, including the `WCX-15` report.

### 9.13 Documentation

Add a `Hostile content rendering` section to
`docs/runbooks/web-console/development.md` covering the contract, the branded
type, the fixture corpus, and the rule that every rendering defect adds a
permanent fixture. Record `security/wcx-06-hostile-content-review.md` in the
established review format.

## 10. Required tests

### 10.1 Unit and component

1. Every fixture in the corpus renders as inert text through both untrusted
   components, creating no element, no attribute, and no URL.
2. ANSI sequences appear as escaped source and produce no styling.
3. A right-to-left override does not reorder the displayed filename.
4. Zero-width characters are visible in the output.
5. Truncation triggers at the defined bounds, states the original length, and
   never occurs silently.
6. No untrusted component emits an anchor or any URL-bearing attribute.
7. Copy copies the original value and reports failure when the clipboard is
   unavailable.
8. A branded untrusted value rendered outside the untrusted components fails
   typecheck; a fixture proves it.
9. MSW rejects an unhandled request, and the migrated tests preserve their
   original assertions including the CSRF and `If-Match` header checks.
10. A production build contains no MSW module, no fixture module, and no
    workbench module.

### 10.2 Backend

1. Handler tests assert `Strict-Transport-Security` and `Permissions-Policy`
   values on a TLS request.
2. A plain-HTTP request emits no `Strict-Transport-Security`.
3. Existing CSP, referrer-policy, and nosniff assertions still pass unchanged.
4. `go -C apps/control-plane test ./...` passes.

### 10.3 Browser and E2E scenarios

Added to `tests/e2e/web-console/`:

1. A hostile-content scenario that seeds attacker-shaped display names through
   the real API and asserts no dialog, navigation, network request, or console
   error results, in all three browsers.
2. A keyboard-only traversal of the onboarding path, reaching every control
   including sign-out at a narrow and a wide viewport.
3. An expanded axe scan covering the login, environments, environment, and
   device routes.
4. A production-build assertion that `/__components` returns the SPA fallback
   and no workbench content.
5. Header assertions for `Strict-Transport-Security` and `Permissions-Policy`
   on a real response.

Secret-bearing steps keep traces, video, and screenshots disabled, matching
the existing configuration.

## 11. Acceptance criteria and Definition of Done

1. The untrusted rendering contract exists, is compiler-enforced through the
   branded type, and neutralises every fixture in the corpus.
2. The permanent fixture corpus exists and is exercised by component and
   browser tests.
3. MSW replaces the hand-written stub with `onUnhandledRequest: 'error'` and
   no assertion was weakened.
4. Both new response headers are present with correct TLS conditioning, and no
   existing header changed.
5. The workbench exists in development and is provably absent from production
   by three independent checks.
6. `task web:check`, `task web:e2e`, and the Control Plane test suite pass.
7. The security review document is recorded.

## 12. Evidence required

- Fixture-by-fixture rendering test report.
- Browser hostile-content run for all three engines, with no console error and
  no network request triggered by fixture content.
- MSW migration diff summary showing preserved assertions.
- Response header capture from a real TLS request and from a plain-HTTP
  request.
- Production bundle inspection proving the absence of MSW, fixtures, and
  workbench.
- `security/wcx-06-hostile-content-review.md`.
- Dependency admission records for MSW.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- neutralising a content class would destroy evidence an operator needs, which
  would reopen `WC-D24`;
- the branded untrusted type cannot be applied without changing the generated
  types or the contract;
- `Strict-Transport-Security` would break an approved deployment topology;
- a Control Plane change beyond the two headers appears necessary;
- MSW cannot preserve an existing security assertion;
- the workbench cannot be proven absent from the production build.

## 14. Deliverables

The untrusted rendering components with the branded type and lint enforcement,
the permanent hostile fixture corpus, the MSW migration with preserved
assertions, the two Control Plane response headers with tests, the
development-only component workbench with triple exclusion proof, the new
browser scenarios, the security review, and the runbook section.
