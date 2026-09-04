---
id: WCX-07
phase: 2
wave: foundation
title: Freshness policy, code splitting, performance budgets
status: approved-for-implementation
risk: medium
components:
  - web-console
decision_refs:
  - WC-D05
  - WC-D06
  - WC-D30
  - WC-D15
  - TS-05
  - PERF-05
  - PERF-07
  - PERF-08
  - OPS-03
acceptance_refs:
  - WCX-000 section 3.1 and 3.6
  - PERF-05 incident visibility 5 s p95 excluding AI
  - PERF-07 usable page interaction under 2 s
depends_on:
  - WCX-02
integration_dependencies:
  - WCX-04
allowed_paths:
  - "apps/web-console/src/**"
  - "apps/web-console/test/**"
  - "apps/web-console/package.json"
  - "apps/web-console/vite.config.ts"
  - "package-lock.json"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-07.md"
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

# WCX-07 — Freshness policy, code splitting, performance budgets

## 1. Purpose

Replace scattered polling intervals with one declarative freshness policy,
introduce route-level code splitting, and put the three-dimensional
performance budget under CI enforcement.

## 2. Why now

Four queries carry a hard-coded five-second interval today, and nothing stops
the next screen from inventing a fifth value. Phase 3 must satisfy `PERF-05`,
incident visibility within five seconds at the ninety-fifth percentile, on a
console that will also poll health, decoys, and notifications. The transport
decision was made deliberately: polling now, with a recorded measured trigger
for reconsidering a server-driven channel. That policy must be a single object
so the trigger can be evaluated against real data rather than against
scattered constants.

## 3. Inputs and decisions

- `WC-D05` — centralised polling `freshnessPolicy`; SSE is not adopted; a
  measured trigger condition is recorded for a future change proposal.
- `WC-D06` — keep the explicit route tree without router loaders or actions;
  add route-level `lazy()` splitting; upgrade React Router 7 to 8 as
  maintenance.
- `WC-D30` — three-dimensional budget: initial route bundle, total bundle, and
  runtime interaction; regression above twenty percent requires owner review,
  matching `PERF-08`.
- `WC-D15` — a read older than its policy allows renders `stale`.
- `OPS-03` — staleness is an explicit product concern.

## 4. Dependencies

`WCX-02` is a hard dependency; the policy wraps the query layer it defines.
`WCX-04` is an integration dependency, because the `stale` state component
must exist for the age threshold to be renderable.

## 5. Scope

1. Define resource freshness classes and the `freshnessPolicy` module.
2. Remove every hard-coded `refetchInterval` and `staleTime` from call sites.
3. Add visibility, focus, and online awareness.
4. Add route-level code splitting with defined chunk boundaries.
5. Upgrade React Router from 7 to 8.
6. Implement the three-dimensional budget check in CI.
7. Record the measured trigger condition for the transport reconsideration.

## 6. Non-goals

- No SSE, WebSocket, long polling, or any server-push mechanism.
- No router loaders or actions; server state stays in TanStack Query.
- No new screen, capability, or endpoint.
- No backend change of any kind, including no `ETag` support work. Conditional
  requests are prepared for but not required, because adding them to the
  contract is a backend decision outside this package.
- No runtime telemetry. Budget measurement happens in CI, not in production.

## 7. Allowed paths

Only the paths in frontmatter. `tests/e2e/**` is forbidden; browser-level
budget scenarios belong to `WCX-15`.

## 8. Security constraints

1. Polling must stop entirely when no session is present. A background
   interval that keeps requesting after sign-out or expiry is forbidden, and a
   test asserts no request is issued after the unauthorized event fires.
2. A `401` during background refresh must follow the existing expiry path
   exactly: clear the in-memory CSRF proof, clear non-session query state, and
   return the operator to sign-in. Background refresh must not create a second
   path to session handling.
3. No query key, cache entry, or chunk name may contain a secret, a token, or
   any value excluded from persistence by the secret-lifetime rules.
4. A lazily loaded chunk must not be fetchable in a way that reveals
   authenticated screen names to an unauthenticated visitor beyond what the
   static asset listing already reveals. Chunk names use route identifiers,
   never environment, device, or incident identifiers.
5. Reducing polling frequency must never make a degraded or failing state
   appear healthy for longer than the policy declares; the `stale` treatment
   is mandatory once the declared age is exceeded.

## 9. Implementation requirements

### 9.1 Freshness policy

One module, `@shared/api/freshness.ts`, defining resource classes and their
parameters. No component or feature may pass `refetchInterval` or `staleTime`
directly; a lint rule forbids those options outside this module.

| Class | Refetch interval | Stale after | Applies to |
|---|---|---|---|
| `critical` | 5 s | 15 s | incident lists and incident detail from Phase 3, notification counts from Phase 4 |
| `operational` | 10 s | 30 s | health projections, device inventory, decoy runtime status |
| `configuration` | 60 s | 5 min | environments, zones, settings |
| `static` | none | never | organization singleton, enumerations |
| `once` | none | never | one-time reads such as an enrollment secret creation result |

Behaviour requirements:

1. `refetchIntervalInBackground` is false for every class. A hidden tab issues
   no interval refetch.
2. On tab visibility change to visible, and on window focus, every `critical`
   and `operational` query refetches immediately.
3. On regaining network connectivity, all non-`static` queries refetch.
4. When a query's data age exceeds the class `stale after` value, the
   consuming surface renders `stale` with the observed age, even when the last
   fetch succeeded.
5. Every query carries the `AbortSignal` from TanStack Query.
6. Requests are prepared for conditional revalidation: the transport passes
   through an `If-None-Match` value when one is supplied and treats `304` as a
   successful unchanged response. Nothing supplies one yet; this is inert
   until a backend decision adds `ETag` to list endpoints.
7. The policy module exports the measured trigger from 9.7 as data, so the
   Phase 5 benchmark can assert against it rather than against prose.

Existing call sites in the device, environment, and health queries adopt
`operational`; environment and zone lists adopt `configuration`; the session
query adopts a dedicated `session` class preserving today's 30-second
staleness and no interval.

### 9.2 Code splitting

Chunk boundaries, exactly:

| Chunk | Contents |
|---|---|
| `entry` | application shell, providers, router, error boundaries, shared transport |
| `login` | the unauthenticated route and everything only it needs |
| `feature-<name>` | one chunk per feature under `src/features/` |
| `vendor-react` | React and React DOM |
| `vendor-query` | TanStack Query |
| `vendor-ui` | Radix |

Rules:

1. Routes are loaded with `React.lazy` and rendered inside a `Suspense`
   boundary whose fallback is the `loading` state component.
2. The login route must not pull any authenticated feature chunk.
3. A navigation must not require more than one feature chunk.
4. Chunk file names remain content-hashed and carry no resource identifier.
5. `Suspense` fallbacks must not cause a layout shift that moves the focused
   element, so route-change focus from `WCX-05` stays correct.

### 9.3 Router upgrade

Upgrade `react-router-dom` from 7 to 8. The upgrade is behaviour-preserving:
the route tree, its paths, and its guards stay identical. If any API used by
the console changed, the change is described in the evidence and covered by a
test. If the upgrade cannot be made behaviour-preserving, stop and escalate
rather than adapting the route architecture.

### 9.4 Performance budgets

Three dimensions, all enforced by `npm run bundle:check` extended for this
purpose and run inside `task web:check`:

| Dimension | Budget | Measured as |
|---|---|---|
| Initial login load | 120 KiB gzipped | `entry` plus `login` plus vendor chunks they require |
| Initial authenticated load | 200 KiB gzipped | `entry` plus the default route's feature chunk plus required vendor chunks |
| Total | 450 KiB uncompressed JavaScript, 32 KiB CSS | all emitted assets, preserving the current limit |
| Runtime interaction | recorded baseline only in this package | measured in `WCX-12` where a data-dense screen exists |

Additional rules:

1. Production source maps remain forbidden; the existing check is preserved.
2. The check prints every chunk with its raw and gzipped size, so growth is
   attributable.
3. A regression above twenty percent in any enforced dimension fails the build
   and requires owner review, matching `PERF-08`.

### 9.5 UI/UX requirements

1. No visible change to what data is shown. Refresh cadence changes for some
   resources; the information does not.
2. A `stale` surface states the observation age in operator terms and offers a
   manual refresh.
3. Route transitions show the `loading` state, not a blank region.
4. Background refresh must never move focus, reorder a list the operator is
   reading, or reset a scroll position.

### 9.6 Accessibility requirements

1. `Suspense` fallbacks use the `loading` state component and its `status`
   role, so a route transition is announced once.
2. A background refresh that changes nothing produces no announcement.
3. Reduced-motion behaviour is unchanged; no skeleton animation is added by
   this package beyond what `WCX-04` defines.

### 9.7 Measured trigger for transport reconsideration

Recorded as data in the policy module and asserted by the Phase 5 benchmark:

> If, on the `P5-W9` reference environment, the measured time from a decoy
> interaction to its visibility in the console exceeds 5 seconds at the
> ninety-fifth percentile with the `critical` class active, or if the
> aggregate console request rate on a three-tab operator session exceeds a
> recorded ceiling, then a change proposal for a server-driven invalidation
> channel is opened. Until one of those is measured, polling stands.

### 9.8 API and data contracts

No contract change. The transport gains inert `If-None-Match` and `304`
handling. The set of endpoints called is unchanged.

### 9.9 Error and failure behaviour

1. A failed background refresh with cached data renders `stale`, never
   `error`, and never silently keeps showing fresh-looking data.
2. A failed background refresh without cached data renders `degraded`.
3. Repeated failures apply exponential backoff up to the class interval
   multiplied by eight, then hold. Backoff never silences the `stale` or
   `degraded` treatment.
4. A failed chunk load renders the route boundary fallback with a reload
   action; it must not blank the console.

### 9.10 Internationalisation and theme

Staleness and refresh wording are catalogue-ready constants. Age formatting
uses the canonical rules that `WCX-08` defines; until then it uses a shared
helper that `WCX-08` replaces.

### 9.11 Observability

None in production. Budget measurement is a CI artefact.

### 9.12 Documentation

Add a `Freshness and performance` section to
`docs/runbooks/web-console/development.md` containing the class table, the
chunk table, the budget table, and the recorded trigger.

## 10. Required tests

### 10.1 Unit and component

1. No source file outside the policy module passes `refetchInterval` or
   `staleTime`; a lint fixture proves the rule fails.
2. Each resource class applies its declared interval and staleness.
3. A hidden document issues no interval refetch; becoming visible triggers an
   immediate refetch for `critical` and `operational`.
4. Regaining connectivity refetches non-`static` queries.
5. Exceeding the class staleness renders `stale` with the observed age even
   after a successful fetch.
6. No request is issued after the unauthorized event fires.
7. A background `401` follows the single existing expiry path.
8. A failed refresh with cached data renders `stale`; without cached data it
   renders `degraded`.
9. Backoff grows and caps as specified and does not suppress the staleness
   treatment.
10. A failed lazy chunk load renders the route fallback and keeps navigation.
11. The router upgrade preserves every route path, redirect, and guard.

### 10.2 Build

1. The extended bundle check reports per-chunk raw and gzipped sizes.
2. Initial login and initial authenticated budgets hold.
3. The login chunk graph contains no authenticated feature chunk.
4. No chunk name contains a resource identifier.
5. No production source map is emitted.

### 10.3 Browser and E2E scenarios

The existing onboarding suite passes unchanged in all three browsers,
including the health degradation and reconnect steps, which exercise the new
`operational` cadence. No new browser scenario is added here.

## 11. Acceptance criteria and Definition of Done

1. One freshness policy governs every query, with no interval literal at any
   call site.
2. Hidden tabs poll nothing; visibility, focus, and reconnection refetch
   correctly.
3. Staleness renders explicitly once a class threshold is exceeded.
4. Route-level splitting exists with the declared chunk boundaries and the
   login chunk is isolated.
5. React Router 8 is in place with behaviour preserved.
6. The three-dimensional budget is enforced in CI with per-chunk reporting.
7. The measured transport trigger is recorded as data.
8. `task web:check` and `task web:e2e` pass.

## 12. Evidence required

- Per-chunk size report, before and after.
- Initial login and initial authenticated load measurements.
- A recording or log demonstrating that a hidden tab issues no requests.
- Router upgrade notes listing any changed API and its covering test.
- Freshness class assignment table for every existing query.
- Lint output for the interval-literal fixture.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- the React Router 8 upgrade cannot preserve the route architecture;
- an initial-load budget cannot be met without changing the design system or
  dropping a dependency chosen by an accepted decision;
- meeting `PERF-05` appears impossible with the `critical` class, which would
  trigger the recorded transport change proposal immediately rather than at
  Phase 5;
- polling cadence would require a backend change such as `ETag` support;
- splitting changes what an operator sees during navigation.

## 14. Deliverables

The freshness policy module with class assignments and the recorded transport
trigger, removal of every interval literal, visibility, focus, and
connectivity awareness, route-level code splitting with defined chunks, the
React Router 8 upgrade, the extended three-dimensional bundle check, the test
suite above, and the runbook section.
