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

## Visual system

Two token layers and one rule: colour is never the only channel.

- `@shared/theme/primitives.css` holds raw values — neutrals, hues, spacing,
  type, radii, elevation, durations. Nothing outside the theme directory may
  reference a primitive.
- `@shared/theme/semantic.css` is the only file that reads a primitive. It
  assigns meaning: surface, line, text, brand, health, severity, config,
  device, confidence, interaction, notice, spacing, motion. `WCX-15` adds the
  light theme by reassigning these same names and nothing else, which is why
  the block is a flat list of assignments with no rules in it.
- Components use semantic names only. `test/check-theme-tokens.mjs` fails the
  lint on any colour literal or raw duration in `src/**/*.css` outside the
  theme directory. Layout geometry is deliberately out of scope: it does not
  change with the theme.

Colour carries meaning in exactly three disjoint groups, asserted by
`tokens.test.ts`:

| Group | Hues | Meaning |
|---|---|---|
| brand | azure | identity only, never a status |
| health | green, red, slate | reported truth; unknown is a category, not a midpoint |
| severity | the five-step `--color-sev-*` ramp | ordered impact |

Device inventory state and configuration completeness are **neutral greys on
purpose**. Inventory is not health and a complete configuration is not
protection; giving either a green would assert something the system does not
know. They are distinguished by glyph and text instead.

`@shared/theme/statusEncoding` is the only place a status string becomes a
visual. Every lookup is total — an unrecognised value, including a prototype
key, resolves to `UNKNOWN_ENCODING`, never to healthy or complete. Confidence
returns filled steps and a label with **no tone and no glyph** (`WC-D11`);
it is never coloured like severity.

Motion is capped at `--motion-standard` (200 ms) and never encodes status.
`prefers-reduced-motion: reduce` collapses every duration to `--motion-none`.

Two rules exist because both were broken once, in the same defect:

- **Every tint is derived, never copied.** A translucent surface is
  `color-mix(in srgb, var(--color-x) 12%, transparent)`, not a hand-written
  `rgb()` with the same numbers. The copies drifted the moment a primitive
  changed, and a badge then composited its foreground over a background from
  an older palette.
- **Tone rules are compound**: `.statusBadge.healthTrue`, never `.healthTrue`.
  A single class is specificity 0,1,0 and loses to any `.container element`
  rule that sets a colour. `.panelHeading > span { color: var(--text-muted) }`
  won that way and repainted the health badge grey, dropping it to 4.22.

Contrast is asserted against **what the browser paints**, not what the token
holds. A badge composites its tone at 12% over the panel beneath it; measuring
the tone against the bare panel token reports a contrast no operator sees.
Severity high and critical measured 4.96 and 5.05 that way and were really
4.30 and 4.34.

The unit table is necessary and not sufficient: it cannot see which rule
actually wins the cascade. The axe scan in the browser flow is the authority,
and it runs in `full.yml`, not on a pull request. A change to tokens or to
badge CSS should be dispatched against `full.yml` on its branch before merge.

CSS size, measured minified per file (`WCX-03` section 9.10 asks for before
and after):

| File | Before | After |
|---|---:|---:|
| `theme/primitives.css` | — | 1541 |
| `theme/semantic.css` | — | 3588 |
| `styles/global.css` | 1094 | 1090 |
| `styles/app.module.css` | 8465 | 10412 |
| **bundle** | **10952** | **19500** |

The bundle exceeds the sum of its parts because CSS Modules rewrites each class
to a hashed name, and the compound tone selectors carry two of them.

The package estimated roughly 3 KiB and expected removing duplicated literals
to offset most of it. That was wrong in both directions and the real number is
7509 bytes. Tokenising *costs* bytes at the use site as well as the definition
site — `var(--health-true)` is longer than the `#7cf2bd` it replaced — so
there is no offset to collect, and two deliberate layers mean 139 custom
properties are declared before a single rule is written. The remaining growth
is eleven new class rules in `app.module.css` for the severity tones and the
status glyph. The 32 KiB budget holds with 13.5 KiB of headroom, and the cost
buys the light theme in `WCX-15` as a reassignment rather than a rewrite.

## Shared components and data states

Every screen composes from `@shared/ui`. The layer exists because `P1-W11`
wrote its truth semantics inline in two routes and they had already started to
drift: a 404 on health was rendered as unavailable in one place and as a
bespoke sentence in another.

Three rules hold across the whole layer:

- **No component takes a `className`.** `WCX-04` section 9.5 permits one
  documented layout slot; this layer offers none. A screen positions a
  component with its own container element, so it can never restyle a status
  indicator into something that reads differently from what the backend said.
  `controls.test.tsx` asserts no module declares a `className` prop.
- **No component writes to browser storage.** `storage.test.tsx` renders every
  component, interacts with the ones that respond to interaction, and asserts
  that `localStorage`, `sessionStorage`, and IndexedDB are both empty *and*
  never written to — a value written and deleted would pass an emptiness check
  alone.
- **No component renders HTML from data.** `dangerouslySetInnerHTML` is absent
  and asserted absent. `WCX-06` adds the full untrusted-text contract; this is
  the floor it builds on.

### The eight data states

Each is a component under `@shared/ui/state/`. The rule that governs all of
them: **no state may read as healthy, complete, or successful.** The word
"healthy" appears in exactly one string in the layer, inside the sentence that
denies it, and `states.test.tsx` asserts that negatively for all eight.

| State | Meaning | What it must say |
|---|---|---|
| `loading` | first load in flight | the activity, announced as `status` |
| `empty` | the backend confirmed zero items | what the collection is, and the action that creates the first item when the operator holds the capability |
| `unknown` | no observation exists | that this is neither a healthy nor a failing signal, plus what would produce an observation |
| `stale` | last good data, refresh not current | the data, its observation age, and why the refresh is not current |
| `partial` | some sources answered, some did not | the data that arrived, a list of what did not, and a retry |
| `degraded` | an upstream dependency is impaired | which dependency, what still answers, what does not |
| `denied` | authorization refused | that access was refused — and nothing about what exists |
| `error` | unexpected failure | a fixed message, a retry when retryable, no diagnostic detail |

Three distinctions carry the security weight, and each has its own test:

- `denied` is not `empty`. A refusal rendered as an empty list tells an
  operator that no devices exist when access was in fact refused.
- `unknown` is neither `empty` nor `False`. Absence of a health projection is
  not a negative observation and not a positive one.
- `denied` names nothing. It renders no count, no identifier, and no
  collection name, because the refusal withheld exactly that.

### The mapping

`resolveDataState` is pure and exhaustive over `ConsoleErrorKind`, so adding a
kind to the taxonomy fails typecheck rather than falling through to `error`.
`DataBoundary` renders whichever state it returns; the screen supplies only the
words.

| Condition | State |
|---|---|
| `isPending` and no cached data | `loading` |
| success, collection is empty | `empty` |
| success, projection absent where the domain defines absence | `unknown` |
| `forbidden`, `unauthenticated`, or `reauthentication-required` | `denied` |
| `not-found` on an observation-shaped resource | `unknown` |
| `unavailable` or `timeout`, with cached data | `stale` |
| `unavailable` or `timeout`, without cached data | `degraded` |
| `network`, with cached data | `stale` |
| `network`, without cached data | `error` |
| any other error | `error` |
| success, but observed longer ago than the freshness policy allows | `stale` |

Two deliberate readings of that table:

- `reauthentication-required` is a third authorization refusal, so it joins
  `denied` rather than the catch-all. Falling through to `error` would render
  a refusal as an unexplained failure. A read carries no CSRF proof, so the
  transport classifies a read's 401 as `unauthenticated` and this kind is
  currently unreachable from a read.
- Freshness is checked **before** the success-shaped states. `empty` asserts
  that the backend confirmed zero items; a read past its freshness policy
  confirmed nothing about now, so an empty collection past its policy is
  `stale`, not a confirmed zero.

The threshold is `FRESHNESS_LIMIT_MS` in `@shared/ui/state/freshness.ts`, one
constant beside the rule that reads it. `WCX-07` replaces it with the real
per-read policy.

Which reads it applies to is deliberately opt-in, by passing `observedAt` to
`DataBoundary`. Today that is the environment and device health projections,
which carry a real observation time in `received_at` — `OPS-03` is about
exactly those. A list read has no observation time of its own, so it is not
subject to the interim threshold; deciding that it should be, and at what age,
is `WCX-07`'s policy call rather than a number guessed at a call site. The
stale state still shows an age for those reads, taken from when the query
cache last accepted the data.

### Error boundaries

`P1-W11` GAP-2: no boundary existed, so any unexpected exception blanked the
console. A blank page in a security product is indistinguishable from "nothing
is wrong".

- The **root** boundary wraps the router in `main.tsx`. Its fallback names the
  product, states a fixed failure, and offers a reload. It renders inside
  `#root`, so the document language and the theme stylesheet stay mounted.
- The **route** boundary wraps the shell's `<Outlet>`, so a failing screen
  leaves navigation and sign-out mounted. It resets when the location changes,
  so moving away from a broken screen recovers without a reload. `/login` sits
  outside the shell and carries its own.

Neither fallback renders anything derived from the caught error: no message, no
stack, no component stack, no request or response content. The error goes to
`recordRenderError`, a module-level variable that dies with the tab and exists
so `WCX-15` can build the user-triggered diagnostic report on an interface that
already exists. Nothing is transmitted, logged, or stored.

### Confirmation levels

The action-to-level mapping is a table in `@shared/ui/confirm/levels.ts`, not a
per-call-site judgement. `WCX-09` and `WCX-11` extend it; neither invents a
level.

| Level | Applies to | Interaction |
|---|---|---|
| 1 reversible | enable, disable, disposition changes | no confirmation; result feedback, undo where the backend supports it |
| 2 destructive-recoverable | zone delete, enrollment-token revoke | modal; the confirm button names the effect, never `OK`; the object is named |
| 3 irreversible-security | device revoke, session revoke, password change | modal; the operator types the exact object name to enable confirm; step-up reauthentication |

Focus lands on cancel, never on the destructive control. Radix would otherwise
focus whichever control comes first in the DOM.

Step-up reauthentication is an **interface only** here. The single
implementation is `stepUpUnavailable`, which refuses, so a level 3 confirmation
cannot complete. That is the intended state: no screen exposes a level 3 action
in this package, and `WCX-09` supplies the implementation against approved
change proposal `0003`.

Focus return is handled in `@shared/ui/controls/Dialog.tsx` rather than by
Radix. Radix returns focus to its own `Dialog.Trigger`, and every dialog in
this console opens from state — a one-time secret appears when a mutation
resolves, not when a button is pressed — so there is no trigger to return to.
The invoking control is captured in `onOpenAutoFocus`, while focus is still on
it.

### Feedback surfaces

| Surface | Use | Lifetime | Role |
|---|---|---|---|
| inline | field and form-scoped validation and results | until the form changes | `alert` for errors, none for success |
| banner | page or scope-level persistent condition | until the condition clears | `status` informational, `alert` blocking |
| toast | short confirmation of a completed action | dismissible, at least 6 s, pauses on hover and focus | `status` |

**An error can never be toast-only.** That is enforced structurally rather than
by convention: `Toast` has no error tone and no way to add one. `show` takes
text and nothing else, a `ToastMessage` carries no severity, and the module
contains no `alert` role. Errors go to `InlineMessage` or `Banner`, both of
which persist.

The toast region is `pointer-events: none`, and only the dismiss control takes
pointer events, so a floating confirmation can never intercept a click on the
content it covers — including in the browser suite.

Long-running work uses `PendingOnObject`: the treatment goes on the object's
own row or panel with its age and reason. No blocking progress modal is
permitted, and the component offers no way to build one.

### Radix consolidation

`@radix-ui/react-dialog` and `@radix-ui/react-label` are replaced by the single
`radix-ui` package (`WC-D09`). `WCX-04` expected the dependency graph to
shrink. **It grew**, and the honest number is worth recording: the meta-package
vendors every primitive, so the lockfile went from 17 `@radix-ui` entries and
330 packages to 61 and 378. What that buys is one version to bump instead of
one per primitive, and later packages adding a popover or a tooltip add no new
dependency. The shipped bundle is tree-shaken and only pays for what is
imported.

### Budgets after this package

| Measure | Before | After | Change | Budget |
|---|---:|---:|---:|---:|
| JavaScript | 384 692 | 401 894 | +17 202 (+4.5%) | 460 800 |
| CSS | 19 500 | 23 567 | +4 067 (+20.9%) | 32 768 |

Both hold: 57.5 KiB of JavaScript headroom and 9.0 KiB of CSS headroom.

The CSS figure needs an owner's eye. `WC-D30` sets twenty percent as the point
at which a regression requires owner review, and CSS crossed it at 20.9%. The
growth is the shared layer's own rules — the state block, the two banner
tones, the toast region, the confidence meter, the description list, and the
two new button variants — not a change to anything that already existed.
`WCX-04` section 9.11 sets the binding constraint as the absolute 32 KiB cap,
which holds, so this is recorded rather than escalated.

## Continuous integration

Two workflows, one job each, no conditions.

| Workflow | Trigger | Contents |
|---|---|---|
| `checks.yml` | push to `main`, every pull request | Markdown format, contract layout, generated freshness, dependency policy, workflow SHA pins, secret scan, the full Web Console gate, and Go vet, unit tests, and formatting |
| `full.yml` | nightly 03:00 UTC, manual dispatch | everything above plus Go integration and race suites, both PostgreSQL integrations, contract tooling, container smoke build, Cowrie fixture, buf breaking checks, and the three-engine browser flow |

`checks.yml` is exactly `task check`, so running that locally before committing
makes the remote run redundant — it exists to catch the times you skipped it,
not to be waited on. `full.yml` needs Docker, browser engines, and generation
tooling; its local equivalent is `task validate`.

Run the browser flow locally against a subset with `GUARDIAN_E2E_PROJECTS`:

```text
GUARDIAN_E2E_PROJECTS=chromium task web:e2e:run
```

`task web:e2e` runs the full web gate first; `task web:e2e:run` runs only the
flow. The runner asserts one post-dismissal screenshot per selected engine.
