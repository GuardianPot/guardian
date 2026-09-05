# Web Console development and evidence

## Runtime boundary

P1-W11 is a static React/TypeScript/Vite application served from the existing
Control Plane HTTPS origin. The production image uses Node 24 only in a build
stage and contains no Node runtime, web development server, or second session
mechanism. `GUARDIAN_WEB_CONSOLE_DIR` selects an absolute, clean directory of
already-built assets. The reviewed container sets it to
`/usr/share/guardian/web-console`.

The server returns hashed `assets/` with a one-year immutable cache policy and
returns `index.html` or client-route fallbacks with `Cache-Control: no-store`.
Unknown `/v1` requests always remain JSON 404 responses. The CSP allows only
self-hosted scripts, styles, images, fonts, forms, and connections; it contains
neither `unsafe-inline` nor `unsafe-eval`.

## Local build

From the repository root:

```text
npm ci
task web:check
task container:check
```

For a local API process, build the SPA and point the Control Plane at its
output before `serve`:

```text
npm run build --workspace @guardianpot/web-console
export GUARDIAN_WEB_CONSOLE_DIR="$(pwd)/apps/web-console/dist"
```

All authentication, database, public-origin, and TLS settings from the auth,
environment, enrollment, device-channel, and health runbooks remain required.
The initial owner bootstrap stays a CLI/API ceremony; it is deliberately not a
public Web Console route.

## Operator path

1. Sign in with the local owner password and either a TOTP code or an unused
   recovery code.
2. Create an environment. This writes configuration only and performs no
   network discovery or mutation.
3. Add canonical RFC1918 zones and, when needed, rename the environment through
   the current strong revision.
4. Create one 15-minute Edge enrollment secret. Transfer it directly to the
   intended Edge and dismiss the dialog. It is not copied to a URL, query
   cache, browser storage, log, trace, video, or screenshot.
5. Wait for the device inventory record to become `active`. Inventory state is
   displayed separately from health.
6. Open the device view and inspect all eight P1-W9 conditions. `False` and
   `Unknown` are distinct blocking outcomes; source device IDs and backend
   reason codes remain visible as escaped text.

A hard reload can restore read-only access from the HttpOnly session cookie,
but the memory-only CSRF proof is intentionally gone. Re-authenticate before a
mutation or logout. A denied or expired session clears non-session query state
and returns the operator to sign-in.

## Browser evidence

Install the pinned browser engines once, then run the disposable fixture on a
Linux host or CI runner:

```text
npx playwright install --with-deps chromium firefox webkit
task web:e2e
```

The fixture creates a unique PostgreSQL database volume, temporary master key,
server TLS identity, product device CA, owner, and recovery codes. Chromium,
Firefox, and WebKit each use the real HTTPS APIs, the real Edge enrollment
binary, and the authenticated device channel. The test-only protocol publisher
submits valid all-true health evidence only inside that disposable environment;
it is not linked into or reachable from the production artifact.

Each browser proves login, environment and zone creation, hard-reload
reauthentication, CSRF denial, invalid enrollment-token denial, one-time secret
dismissal, active inventory, all eight healthy conditions, disconnect
degradation, reconnect recovery, reduced motion, an axe serious/critical scan,
empty browser storage, and expired/missing-cookie handling.

Trace, video, and automatic screenshots stay disabled. Exactly one explicit
post-dismissal screenshot per browser is allowed. CI retains those three PNGs
for seven days; no text, JSON, ZIP, trace, or video artifact is accepted.

Windows cannot provide the Unix owner-only master-key mode enforced by the
Control Plane. Run the complete fixture on Linux/CI; do not weaken the
master-key permission check for local convenience.

## Failure handling

- A missing or inaccessible asset directory returns a generic unavailable
  response and never leaks a filesystem path.
- A 401 from authenticated API use expires the UI session view. Reauthenticate;
  do not reconstruct a CSRF value.
- A refused sign-out is reported as failed and the session remains active. Retry
  or revoke the session through the Control Plane; do not assume it ended.
- A health 404 or fetch error is rendered as unavailable, never healthy.
- A pending or active inventory record without a health projection remains
  explicitly unreported.
- Never attach DevTools storage dumps, network archives, HAR files, traces,
  videos, or screenshots captured while a one-time secret is visible.

## Module layout and boundaries

`WCX-01` established a feature-sliced source tree. The layout is:

```text
apps/web-console/src/
  main.tsx      Vite entry; the only file outside a layer
  app/          router, application shell, providers
  features/
    auth/       session context, capability seam, sign-in
    environments/
    devices/
    health/
  shared/
    api/        transport and DTO types
    auth/       capability types and the pure resolver
    forms/      typed form-field readers
    ui/         shared presentation components
    styles/     CSS Modules and global styles
    theme/      design tokens (WCX-03)
    text/       operator text catalogue (WCX-08)
    hooks/
    testing/    test-only helpers, never imported by production code
  generated/    generated OpenAPI types (WCX-02)
```

Path aliases resolve identically in TypeScript, Vite, and Vitest:

| Alias | Target |
|---|---|
| `@app/*` | `src/app/*` |
| `@features/*` | `src/features/*` |
| `@shared/*` | `src/shared/*` |
| `@generated/*` | `src/generated/*` |

Import rules, enforced by `no-restricted-imports` in `eslint.config.js`:

1. `app` may import feature public APIs and `shared`.
2. A feature may import `shared`, `generated`, and its own internals.
3. A feature may import another feature only through that feature's `index.ts`,
   and only for a pair listed in `eslint.config.js` with its reason. The
   approved pairs are `environments -> auth`, `environments -> health`, and
   `devices -> health`.
4. `shared` may never import `features` or `app`.
5. Deep imports into another feature and relative imports that escape a
   directory are always errors.
6. `@shared/testing` is unreachable from production modules.

`npm run lint` runs ESLint and then `test/check-boundaries.mjs`, which lints
the fixtures under `src/**/__boundary__/` and fails if any boundary violation
stops being an error. Those fixtures are excluded from the normal lint run and
are never imported by the application.

## Capability seam

`useCapability(capability)` from `@features/auth` decides whether a control
renders as available. It is **presentation only**. The Control Plane enforces
every authorization decision, and a rejected request must still be rendered
truthfully.

A denied capability disables its control and shows the reason. It must never
hide the control: a missing control reads as a broken product, while a
disabled one with a reason tells the operator what to do.

Phase 2 has one local owner and no role model, so every capability resolves
from whether the in-memory CSRF proof exists. The `not-permitted` denial is
declared for a future role model and is unreachable today; callers handle it
exhaustively so a later role model needs no call-site change.

## Static analysis

- `npm run typecheck` runs `tsc -b`, covering `src`, `vite.config.ts`,
  `eslint.config.js`, and `test/*.mjs`.
- `strict` is extended with `noUncheckedIndexedAccess`, `noImplicitOverride`,
  and `exactOptionalPropertyTypes`.
- ESLint runs `typescript-eslint` type-checked rules plus
  `eslint-plugin-react-hooks`.
- `npm run bundle:check` enforces the size budget, forbids production source
  maps, and fails if test-only code reaches a production chunk.

New dependencies follow
[`docs/engineering/web-console-dependency-policy.md`](../../engineering/web-console-dependency-policy.md).

## Generated API types

`WCX-02` derives every DTO from the approved OpenAPI contract, satisfying
`RE-10`. Nothing under `src/shared/api/types.ts` re-declares a field, union, or
enum; a schema removed or renamed in `openapi/guardian.yaml` fails typecheck
rather than drifting silently.

Regenerate after any contract change:

```text
npm run generate:api -w @guardianpot/web-console
```

`task web:check` runs `npm run generated:check` first. It regenerates into a
temporary path, compares byte-for-byte against the committed file, and also
fails if the generated module ever declares runtime code. Generated types emit
nothing, so they add no production bytes.

The generator runs from the workspace directory because the repository-root
`redocly.yaml` declares a named API that `openapi-typescript` would otherwise
require an output key for.

## API layer

One transport, one error taxonomy, per-feature API modules.

- `@shared/api/transport` owns the only `fetch` call. Its security behaviour is
  fixed: `credentials: 'include'`, `cache: 'no-store'`, the CSRF proof on
  mutations only, `If-Match` on revisioned updates, and the
  `guardian:unauthorized` event on an unexpected 401. It never sets `Origin`;
  that is a forbidden fetch header the browser supplies and the Control Plane
  validates.
- `@shared/api/error` classifies every failure into a `ConsoleError`.
  Components consume the classification and never inspect an HTTP status. The
  operator-facing message comes from a fixed table keyed by `messageKey`; a
  backend `status` slug is retained for diagnostics and is never rendered, so
  a hostile slug cannot reach the DOM.

| Condition | Kind | Retryable |
|---|---|---|
| 401 without an active mutation proof | `unauthenticated` | no |
| 401 on a request that carried a proof | `reauthentication-required` | no |
| 403 | `forbidden` | no |
| 404 | `not-found` | no |
| 400, 422 | `validation` | no |
| 409, 412 | `conflict` | no |
| 429 | `rate-limited` | yes |
| 500 to 504 | `unavailable` | yes |
| `AbortError` | `timeout` | yes |
| `TypeError` from `fetch` | `network` | yes |
| anything else | `unexpected` | no |

- Each feature owns an `api.ts` exporting its query-key factory, its
  `queryOptions()` helpers, and its mutations. Keys follow
  `[feature, resource, ...scopeIds]`; no component builds one. Invalidation
  lives beside the mutation that causes it.
- Reads retry at most twice and only for a retryable classification; mutations
  never retry. Every read carries the query's `AbortSignal`.
- A 304 is a success with no body, handled before the error branch because
  `Response.ok` is false for it.

`WCX-07` replaces the per-query `refetchInterval` values with the central
freshness policy; `WCX-08` moves `messageKey` text into the operator catalogue.

## Continuous integration scope

Change proposal `0007` split the single `quality` job into selective jobs.

| Job | When it runs |
|---|---|
| Fast gate | every pull request and push, unconditionally |
| Web Console | the diff touches `apps/web-console/`, `tests/e2e/web-console/`, or `openapi/` |
| Go and integration | the diff touches Control Plane, Edge Agent, `pkg/`, or an integration suite |
| Contracts and container | the diff touches `proto/`, `openapi/`, `schemas/`, buf config, `deploy/`, `decoys/`, or a Dockerfile |
| Browser E2E | with the Web Console job; Chromium only on a pull request |

Every job runs regardless of paths on a push to `main`, on the nightly
schedule, and on manual dispatch, which is also when the browser flow runs in
all three engines. Selection is computed by `.github/scripts/changed-areas.sh`
from `git diff` alone, and fails safe: a change to `package.json`,
`package-lock.json`, `Taskfile.yml`, `go.work`, or anything under
`.github/workflows/` or `.github/scripts/` runs every area.

Run the browser flow locally against a subset with `GUARDIAN_E2E_PROJECTS`:

```text
GUARDIAN_E2E_PROJECTS=chromium task web:e2e
```

The runner asserts one post-dismissal screenshot per selected engine.
