---
id: WCX-02
phase: 2
wave: foundation
title: Generated OpenAPI types, API transport, and error taxonomy
status: draft
risk: high
components:
  - web-console
decision_refs:
  - WC-D02
  - WC-D03
  - WC-D04
  - WC-D29
  - RE-10
  - CP-04
  - TS-06
  - SA-11
remediates:
  - P1-W11 GAP-3
acceptance_refs:
  - WCX-000 section 3.1
  - RE-10 no duplicated API DTO semantics
  - Phase 1 browser onboarding skeleton E2E must pass unchanged
depends_on:
  - WCX-01
allowed_paths:
  - "apps/web-console/src/**"
  - "apps/web-console/package.json"
  - "apps/web-console/tsconfig.app.json"
  - "apps/web-console/vite.config.ts"
  - "package.json"
  - "package-lock.json"
  - "tools/check-generated.mjs"
  - "Taskfile.yml"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-02.md"
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

# WCX-02 — Generated OpenAPI types, API transport, and error taxonomy

## 1. Purpose

Close the `RE-10` drift by generating Web Console DTO types from the approved
OpenAPI contract, and give the console one transport layer with a normalized
error taxonomy and stable server-state conventions.

## 2. Why now

`apps/web-console/src/api/types.ts` hand-duplicates eight OpenAPI schemas,
which `RE-10` prohibits. `ApiError` discards the response body, so validation,
conflict, rate-limit, and degraded outcomes are indistinguishable and each
route invents its own message. Phase 2 decoy management and Phase 3 incidents
multiply both problems. Query keys are free-form string literals, which will
not survive the Phase 3 invalidation graph.

## 3. Inputs and decisions

- `WC-D02` — `openapi-typescript` type generation only, committed under
  `src/generated/`, with a CI freshness check. No runtime client dependency is
  added; the existing thin `request()` wrapper is retained.
- `WC-D03` option A — a frontend `toConsoleError()` taxonomy over the current
  `{"status": "<slug>"}` body. Option B, the backend body extension, is change
  proposal `0004` and is **out of scope here**.
- `WC-D04` — typed query-key factories, `queryOptions()` helpers, feature-local
  invalidation, cursor pagination through `useInfiniteQuery`.
- `RE-10`, `CP-04`, `TS-06` — REST/OpenAPI contract, no DTO duplication.
- Remediates `P1-W11 GAP-3`.

## 4. Dependencies

`WCX-01` must be accepted. The generated directory and the transport layer are
written into the boundary layout it establishes.

## 5. Scope

1. Generate TypeScript types from `openapi/guardian.yaml` into
   `src/generated/openapi.ts` and commit them.
2. Add a generated-freshness check to CI.
3. Delete `src/api/types.ts` and re-point every consumer at derived types.
4. Move the transport into `src/shared/api/` and remove the inert `Origin`
   header assignment.
5. Add the `ConsoleError` taxonomy and `toConsoleError()`.
6. Add query-key factories, `queryOptions()` helpers, and the cursor
   pagination convention.
7. Split `api/client.ts` into feature-local API modules.

## 6. Non-goals

- No change to `openapi/guardian.yaml` or any Control Plane behaviour.
- No runtime OpenAPI client library, no `openapi-fetch`, no Orval.
- No generated React Query hooks; hooks are hand-written against the helpers.
- No backend error-body change; that is change proposal `0004`.
- No new endpoint consumption. Endpoints not called today stay uncalled;
  `WCX-09` adds them.
- No UI, text, styling, or accessibility change.

## 7. Allowed paths

Only the paths in frontmatter. `openapi/**` is forbidden: this package
consumes the contract and must never edit it. `tests/e2e/**` is forbidden so
that an unchanged browser suite proves behaviour preservation.

## 8. Security constraints

1. The security-relevant behaviour of `request()` must be preserved exactly:
   `credentials: 'include'`, `cache: 'no-store'`, the `X-CSRF-Token` header on
   mutations only, the `If-Match` revision header, and the
   `guardian:unauthorized` event on an unexpected 401. A test asserts each.
2. Remove `headers.set('Origin', ...)`. It is a forbidden fetch header that
   browsers ignore; the backend check relies on the browser-supplied header.
   The removal must be accompanied by a code comment recording that origin
   validation is a Control Plane responsibility, so the line is not
   reintroduced.
3. `ConsoleError` must separate the operator-facing message from diagnostic
   detail. The operator-facing message is drawn from a fixed table and must
   never interpolate a raw backend string, a submitted credential, a
   requested path, or a response body fragment.
4. An unrecognised backend status slug maps to the generic unexpected-error
   entry. It must never be rendered verbatim.
5. No response body, request body, or header value may be logged, stored, or
   attached to any cache key.
6. Generated types are types only. The generation step must not emit runtime
   code, and a test asserts the generated module contributes zero bytes to the
   production bundle.

## 9. Implementation requirements

### 9.1 Technical requirements

Generation:

1. Add `openapi-typescript` as a development dependency, pinned exactly.
2. Add script `generate:api` producing `src/generated/openapi.ts` from
   `openapi/guardian.yaml`.
3. Commit the generated file. Extend `tools/check-generated.mjs` so that a
   drifted or missing generated file fails CI: regenerate into a temporary
   path and compare byte-for-byte.
4. Add the generation and freshness steps to `task web:check`.
5. Exclude `src/generated/**` from lint formatting rules but keep it
   typechecked.

Derived domain types replace `src/api/types.ts`:

```ts
import type { components } from '@generated/openapi';

export type Environment = components['schemas']['Environment'];
export type Zone = components['schemas']['Zone'];
export type DeviceInventory = components['schemas']['DeviceInventory'];
export type HealthView = components['schemas']['HealthView'];
export type HealthCondition = components['schemas']['HealthCondition'];
export type AuthSession = components['schemas']['AuthSession'];
export type EnrollmentTokenSecret =
  components['schemas']['EnrollmentTokenSecret'];
```

No field, union, or enum may be re-declared by hand. Where the console needs a
narrower view model, it is derived by `Pick`, `Omit`, or a mapping function,
never by re-typing.

Transport, in `src/shared/api/transport.ts`: the current `request()` moves
unchanged except for the `Origin` removal and the error path, which now
constructs a `ConsoleError`.

Error taxonomy, in `src/shared/api/error.ts`:

```ts
export type ConsoleErrorKind =
  | 'unauthenticated'
  | 'reauthentication-required'
  | 'forbidden'
  | 'not-found'
  | 'validation'
  | 'conflict'
  | 'rate-limited'
  | 'unavailable'
  | 'timeout'
  | 'network'
  | 'unexpected';

export type ConsoleError = {
  kind: ConsoleErrorKind;
  /** Stable catalogue key for the operator-facing message. */
  messageKey: string;
  /** Backend status slug, retained for diagnostics only. Never rendered. */
  statusSlug?: string;
  httpStatus?: number;
  retryable: boolean;
};

export function toConsoleError(input: unknown): ConsoleError;
```

Mapping rules, exhaustive and testable:

| Condition | Kind | Retryable |
|---|---|---|
| HTTP 401 with no active session | `unauthenticated` | no |
| HTTP 401 on a mutation while a session read still succeeds | `reauthentication-required` | no |
| HTTP 403 | `forbidden` | no |
| HTTP 404 | `not-found` | no |
| HTTP 400 or 422 | `validation` | no |
| HTTP 409 or 412 | `conflict` | no |
| HTTP 429 | `rate-limited` | yes |
| HTTP 500, 502, 503, 504 | `unavailable` | yes |
| `AbortError` from a timeout | `timeout` | yes |
| `TypeError` from `fetch` | `network` | yes |
| anything else | `unexpected` | no |

`messageKey` values are catalogue keys, not sentences; `WCX-08` supplies the
text. Until then a temporary English map lives beside the taxonomy and is
removed by `WCX-08`.

Server-state conventions, in `src/shared/api/query.ts` and per feature:

1. Query keys come from a per-feature factory. The key shape is
   `[feature, resource, ...scopeIds, params?]`. No component constructs a key
   literal.
2. Each read is exposed as a `queryOptions()` helper so a component, a
   prefetch, and an invalidation all reference one definition.
3. Invalidation lives in the feature's API module next to the mutation that
   causes it, never inline in a component.
4. Any list endpoint that the contract paginates with `next_cursor` is
   consumed through `useInfiniteQuery`. `limit` is an explicit parameter with
   a named default constant; no literal `200` appears in a call site.
5. Mutations do not retry. Reads retry only when the `ConsoleError` is
   `retryable`, at most twice, with exponential backoff.
6. Every request carries an `AbortSignal` from TanStack Query so navigation
   cancels in-flight reads.

Feature API modules replace the single `api` object:
`@features/auth/api`, `@features/environments/api`, `@features/devices/api`,
`@features/health/api`. Each exports its query-key factory, its
`queryOptions()` helpers, and its mutation functions.

### 9.2 UI/UX requirements

No visible change in this package. Existing route-local message strings stay
byte-identical; they are produced from the taxonomy's temporary map rather
than from inline `catch` blocks. Message text changes belong to `WCX-04` and
`WCX-08`.

### 9.3 Accessibility requirements

No regression. Existing `role="alert"` and `role="status"` placement is
unchanged.

### 9.4 API and data contracts

Consumed only, never modified. The set of endpoints called by the console is
identical before and after: session, login, logout, environments list, get,
create, update, zones list and create, devices list and get, enrollment-token
create, environment health, and device health.

### 9.5 Error and failure behaviour

1. Every failed request produces a `ConsoleError`; no raw `Error`, `Response`,
   or backend body reaches a component.
2. A `reauthentication-required` error clears the in-memory CSRF proof exactly
   as today and dispatches the existing unauthorized event.
3. A `conflict` error from a revision mismatch is distinguishable from a
   `validation` error. The environment rename path must be able to tell the
   operator that a concurrent change occurred rather than that the name is
   invalid.
4. Failures never fall back to optimistic or cached success. A stale cached
   value may be displayed only when explicitly marked stale, which `WCX-04`
   renders.

### 9.6 Internationalisation and theme

`messageKey` values are chosen now so that `WCX-08` can move them into the
catalogue without renaming. No theme impact.

### 9.7 Performance

Generated types add zero runtime bytes. The bundle must not grow by more than
1 percent. Splitting the client into feature modules must not duplicate the
transport; a test asserts a single transport module in the build output.

### 9.8 Observability

None yet. `ConsoleError` reserves `statusSlug` and `httpStatus` for the
`WCX-15` diagnostic report, but nothing reads them in this package.

### 9.9 Documentation

Update `docs/runbooks/web-console/development.md` with the generation command,
the freshness check, the error taxonomy table, and the query-key convention.

## 10. Required tests

### 10.1 Unit and component

1. Every row of the error mapping table is asserted, including the
   unrecognised-slug fallback to `unexpected`.
2. A hostile backend status slug, for example one containing markup or ANSI
   escapes, never appears in the rendered output.
3. `request()` preserves `credentials: 'include'`, `cache: 'no-store'`, the
   CSRF header on mutations only, and `If-Match` on revisioned updates.
4. No request sets an `Origin` header.
5. An unexpected 401 dispatches `guardian:unauthorized` and clears the CSRF
   proof; an expected 401 on the session probe does not.
6. Query-key factories produce stable keys, and a mutation's invalidation
   targets the keys its own module declares.
7. Cursor pagination requests a second page using the returned `next_cursor`
   and stops when it is absent.
8. Reads retry only for retryable kinds; mutations never retry.
9. A build test asserts `src/generated/**` contributes no runtime bytes.

### 10.2 Contract

1. A test proves every derived domain type resolves from `components` in the
   generated module, so a removed or renamed schema fails typecheck.
2. The generated-freshness check fails when `openapi/guardian.yaml` is edited
   without regeneration. This is verified with a temporary fixture, not by
   editing the real contract.

### 10.3 Browser and E2E scenarios

The existing onboarding suite passes unchanged in Chromium, Firefox, and
WebKit, including the CSRF-denial and expired-session steps, proving that
transport behaviour was preserved.

## 11. Acceptance criteria and Definition of Done

1. `src/api/types.ts` no longer exists and no OpenAPI schema is hand-declared.
2. `src/generated/openapi.ts` is committed and CI fails on drift.
3. All requests flow through one transport module with unchanged security
   behaviour and no `Origin` header.
4. `toConsoleError()` covers every row of the mapping table and no component
   inspects an HTTP status directly.
5. Every query uses a factory key and a `queryOptions()` helper; no literal
   key and no literal `limit` remain.
6. `task web:check` and `task web:e2e` pass.
7. Bundle growth is at most 1 percent.

## 12. Evidence required

- Generation command output and the committed generated file diff.
- CI output showing the freshness check failing on a drift fixture and passing
  on the committed state.
- `task web:check` and `task web:e2e` results.
- A table mapping every previously inline `catch` message to its new
  `messageKey`, proving no operator-facing text changed.
- Bundle size before and after.
- Dependency admission record for `openapi-typescript`.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- the OpenAPI contract cannot express a type the console needs, which would
  require a contract change rather than a console change;
- preserving an existing operator-facing message requires interpolating a
  backend-supplied string;
- distinguishing `validation` from `conflict` proves impossible from the
  current `{"status"}` body, which would make change proposal `0004` a
  blocking dependency rather than a follow-up;
- the generated module cannot be produced without runtime code;
- any security behaviour of `request()` would change.

## 14. Deliverables

Committed generated OpenAPI types with a CI freshness gate, a single shared
transport module, the `ConsoleError` taxonomy with an exhaustive mapping,
per-feature API modules with typed query keys and `queryOptions()` helpers,
the cursor pagination convention, tests for all of the above, and the updated
development runbook.
