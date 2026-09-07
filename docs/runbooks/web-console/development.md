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

## Accessibility

The conformance target is **WCAG 2.2 Level AA** for every operator-facing
surface (`WC-D23`). It is a target with teeth: three layers enforce it, and a
regression fails the build rather than waiting for someone to notice.

`P1-W11` produced good practice without a target — a skip link, landmarks,
labels, error association, reduced motion — but route changes moved no focus,
announced nothing, and never changed the document title (GAP-4), and the only
automated check was a full-page browser scan that runs nightly.

### Three enforcement layers

| Layer | What it sees | Where it runs |
|---|---|---|
| `eslint-plugin-jsx-a11y` | what is wrong in the source | `npm run lint`, every commit |
| axe on rendered components | what is only wrong once rendered — a name that resolves to nothing, a description pointing at an absent id | `npm run test`, every commit |
| axe on the full page in a real browser | what is only wrong once the cascade resolves — contrast, focus visibility, stacking | `full.yml`, nightly |

Each catches what the one before it cannot. The first two are fast enough to
run on every change; the third is the only one that sees a real rendering
engine, which is why `color-contrast` is disabled in the component layer: jsdom
resolves no cascade and no composited background, so it would measure a colour
the operator never sees. Contrast is covered instead by the token table in
`@shared/theme/tokens.ts` and by the browser scan.

The component layer fails on `serious` and `critical`. `moderate` and `minor`
are collected and printed rather than failed on — a component rendered outside
a page legitimately has no landmark to sit in — so a new one is visible without
turning every best-practice heuristic into a build break.

A `jsx-a11y` suppression must state its reason after ESLint's `--` separator.
`a11yLint.test.ts` fails the suite on one that does not, and also asserts that
the rules are configured as errors *and* that they actually fire — a rule that
is configured but silently unloaded looks exactly like a codebase with no
violations.

### The route-change contract

On every completed navigation, `RouteAnnouncer` does three things:

1. sets `document.title` to `<screen name> — Guardian Console`;
2. moves focus to the screen's `h1`, which carries `tabIndex={-1}`. When the
   screen is still loading there is no heading yet, so focus lands on the
   `main` landmark and follows the heading in when it renders;
3. announces the screen name through one polite live region that lives in
   `AppLayout` and is never re-created.

Two details that are easy to get wrong and are asserted:

- **Screen names come from the route definition's `handle`, never from backend
  data.** An environment's title is "Environment — Guardian Console", not the
  environment's display name. A title travels outside the document — into a
  browser tab, a window list, a bookmark — and section 8.1 keeps
  attacker-supplied content off every such surface.
- **Focus is claimed, never stolen.** If the operator moved focus themselves
  while a screen loaded, the late-arriving heading does not take it back.

There is no `ScrollRestoration`, and the sidebar is `position: static` at the
width where it becomes a horizontal bar, so nothing sticky can cover the
focused heading.

### Manual checks

Automated conformance is the floor, not the ceiling. Before releasing a screen,
walk it once by keyboard alone and confirm:

- every control is reachable in an order that matches the visual layout;
- the focus ring is visible on every control, including on dark surfaces;
- a dialog traps focus, closes on escape, and returns focus to whatever opened
  it;
- a disabled control announces why it is disabled;
- no announcement repeats on a polled refresh that changed nothing;
- the screen is usable at 200% zoom and at a 320-pixel width.

### Exceptions register

| Exception | Reason | Owner | Review |
|---|---|---|---|
| The sign-in failure message does not name which credential was wrong. | Deliberate. Naming the failing field would tell an attacker whether a username exists. The message still names the correction, which is what section 9.6.5 asks for. | Product | Standing |

`keyboard.test.tsx` keeps the register honest: it parses the stylesheet, finds
every region a breakpoint hides, and fails on anything not on an exhaustive
allowed list. Adding a new entry to that list is a deliberate act that has to
be justified here.

### GAP-1, closed

`P1-W11` GAP-1 — the operator block, holding sign-out and the re-authentication
link, was `display: none` below 900 pixels — was briefly recorded here as an
exception owned by `WCX-10`. It is now fixed: the block becomes a wrapping row
in the horizontal bar instead of disappearing from it.

It was pulled forward because it blocked two packages rather than one.
`WCX-05` cannot claim its own acceptance criterion 5 with it open, and
`WCX-06`'s browser keyboard traversal reaches sign-out at a narrow viewport,
so both would have had to record the same defect and move on.

**What `WCX-10` still owns.** Its section 9.3 specifies a disclosure control
that navigation and the operator block collapse into below the breakpoint, with
`aria-expanded`, `aria-controls`, escape-to-close, and focus return. None of
that is delivered here. What is delivered is the invariant that section calls
its single most important requirement — *no operator control is removed at any
viewport width* — which is the defect, as distinct from the design that
replaces it. `WCX-10` restyles the same markup; it no longer has to un-break
it first.

### Cost

The component suite runs 253 tests across 29 files in about 8 seconds wall
clock on a development machine. The ceiling is 60 seconds; past that, split the
axe assertions into their own project rather than dropping them.

The bundle grew by 2 020 bytes of JavaScript and 444 of CSS — the live region,
the announcer, and the route handles. `jsx-a11y` and `axe-core` are
development-only and ship nothing. JavaScript stands at 403 914 bytes against
a 460 800 budget, CSS at 24 011 against 32 768.

`eslint-plugin-jsx-a11y@6.10.2` declares a peer range of `eslint ^3 || … || ^9`
and this repository is on ESLint 10. The plugin works — `a11yLint.test.ts`
proves the rules fire — but the stale metadata means a *new* `npm install` in
this workspace needs `--legacy-peer-deps`. `npm ci` is unaffected, so CI is
unaffected. Drop the flag once upstream ships a release that lists ESLint 10.

## Hostile content rendering

Guardian is a deception product. A large share of what this console displays
was written by whoever attacked the network. React's text escaping is necessary
and not sufficient: it stops a captured string *executing*, and does nothing
about the attacks that forge what an operator sees.

Three that survive escaping intact:

| Attack | What the operator sees without the contract |
|---|---|
| right-to-left override | `report<RLO>gnp.exe` displays as `reportexe.png` — an executable reading as an image |
| ANSI sequence | a transcript repainted so a denial reads as a grant |
| zero-width characters | `pass<ZWSP>word` invisible to a reader and to a search |

### Untrusted values are not strings

`Untrusted` is an opaque type. At runtime it is the string it always was; to
the compiler it is not a string at all, so it cannot reach JSX, an attribute,
or a plain string. The only way to display one is `UntrustedText` (short
single-line values) or `UntrustedBlock` (multi-line captured payloads).

```ts
const name = untrusted(raw.display_name);
<span>{name}</span>                 // typecheck error
<UntrustedText value={name} />      // the only way through
```

Routing is a typecheck failure rather than a review comment.
`untrusted.test.tsx` proves each refusal with `@ts-expect-error`, so if the
brand ever stops working those turn from passing assertions into build errors.
String coercion — `'x' + value` and `` `${value}` `` — is not a TypeScript
error, because both produce a string; it is caught by the type-aware lint rules
`restrict-template-expressions`, `restrict-plus-operands`, and
`no-base-to-string`, and the same test asserts those are configured as errors.

`reveal()` is the single escape hatch. The renderer needs it, and the clipboard
needs it to copy the original value. Nothing else should call it — the one
other legitimate use in the console is the environment rename field, where the
operator is editing the value and React sets an input value as a property
rather than as parsed markup.

### Where the boundary sits

Once per feature API module, in `@shared/api/taint`. What is marked:

- health condition `reason` and `message` — a compromised or emulated Edge
  writes these directly;
- device and source identifiers inside a projection — device-supplied strings,
  not identifiers Guardian issued;
- display names — the console cannot distinguish an operator's typing from an
  attacker with a stolen session, so it does not try.

What is **not** marked: values the backend validates into a shape it issued —
UUIDs, CIDRs, timestamps, enum states. Marking those would make the brand mean
"string" rather than "untrusted".

The `taint*` functions rebuild the object rather than casting it. A cast would
keep compiling when the contract gains a new free-text field; rebuilding means
the new field arrives unmarked and someone has to decide which side of the
boundary it belongs on.

### What the renderer does

In order: control characters, then directional formatting, then invisible
characters, then the length bound. Each becomes visible escaped source —
`\x1b`, `\u202e` — carrying an accessible description so a screen-reader user
learns a character was present rather than hearing nothing.

Two deliberate non-behaviours:

- **Nothing is normalised.** Normalising would change the evidence an operator
  is reading.
- **Nothing is removed.** A removed character is a character the operator never
  learns was there.

Bounds are 512 code points for a single-line value and 65 536 for a block.
Section 9.1.5 of `WCX-06` states the block bound as 64 KiB; it is applied in
code points so a truncation can never split a character and invent a
replacement glyph that was not in the evidence. Truncation always states the
original length.

`UntrustedBlock` keeps newline and tab, because they are structure an operator
reads. `UntrustedText` escapes them: a single-line value containing a newline is
not a single line, and would break the layout it was placed in.

### The permanent corpus

`@shared/hostile/corpus` holds 24 inert fixtures (`RE-12`). Every invisible
character is built with `String.fromCodePoint` rather than typed, and a test
asserts the file's own source contains no literal invisible byte — a corpus
nobody can review is not evidence of anything.

**The file only grows.** Every rendering defect found from here on adds a
fixture, so the same defect cannot return unnoticed. Removing an entry means
deciding a class of hostile content no longer needs proving, which is an owner
decision rather than a cleanup.

It lives in `shared/` rather than `shared/testing/` because the
development-only workbench renders it, and a workbench is not a test.
`check-bundle.mjs` asserts no production chunk contains a fixture id.

### Forbidden APIs

`test/check-unsafe-dom.mjs` runs on lint and forbids
`dangerouslySetInnerHTML`, `innerHTML`, `outerHTML`, `insertAdjacentHTML`,
`document.write`, `eval`, `new Function`, and string-bodied timers across the
whole console. `WCX-06` section 8.2 permits no suppression, which is why this
is a repository check rather than an ESLint rule a directive could switch off
line by line.

### The component workbench

`/__components` in a development build. Not Storybook: one file, no second
toolchain, and it renders the same components from the same source, so it
cannot drift from what ships. It exists because some states are hard to reach
in the running product — a `partial` read needs one source to fail while
another succeeds — and a state nobody can look at is a state nobody checks.

Three independent proofs that it never ships:

1. `router.tsx` mounts it behind `import.meta.env.DEV`, which Vite replaces
   with `false` so Rollup drops the dynamic import and everything it reaches;
2. `check-bundle.mjs` fails on its marker or any corpus fixture id in a
   production chunk;
3. a browser scenario asks a production build for `/__components` and asserts
   it receives the ordinary SPA shell.

Its styles are in `workbench.module.css`, not `app.module.css`, because CSS
Modules does not tree-shake unused class rules: two classes in the shared
stylesheet shipped 299 bytes in every production build. The measured CSS size
is now byte-for-byte what it was before the workbench existed.

### Network mocking

MSW, with handlers keyed `METHOD /path` exactly as the old `stubFetch` was, so
the migration moved the mechanism and left every assertion where it was.
`onUnhandledRequest: 'error'` carries forward the guarantee the stub gave by
answering 404, and makes it louder: an unmocked call throws rather than
returning something plausible.

**One finding worth remembering.** Under jsdom's `Request` class, MSW's
interceptor rebuilt every intercepted request and dropped every header the
console had set. The `X-CSRF-Token` and `If-Match` assertions would have passed
against a console that sent neither — a security assertion silently turning
into a no-op. `src/shared/testing/setup.ts` now hands the environment Node's
`Request`, `Response`, and `Headers`, which is what MSW targets. `fetch` is
left alone; overriding only the three classes keeps the blast radius as small
as the problem.

If a header assertion ever starts passing suspiciously easily, check that
setup file first.

### Browser security headers

`Permissions-Policy` denies 26 features the console never uses, on every
scheme. `Strict-Transport-Security` pins HTTPS for a year including subdomains,
**only over TLS** — a development listener that pinned a plain-HTTP host would
lock a developer out of their own machine for a year. `preload` is deliberately
absent: it is effectively irreversible for months and is an owner decision, not
a middleware default.

## Freshness and performance

### One policy, no interval literals

Four queries used to carry a hard-coded five-second interval, and nothing
stopped the next screen inventing a fifth value. That is not untidiness.
`WC-D05` chose polling over a server-driven channel **deliberately** and
recorded a measured condition for reconsidering it — and a trigger cannot be
evaluated against constants scattered through feature modules.

So a resource declares a class. It never declares an interval.

| Class | Refetch | Stale after | Applies to |
|---|---|---|---|
| `critical` | 5 s | 15 s | incident lists and detail (Phase 3), notification counts (Phase 4) |
| `operational` | 10 s | 30 s | health projections, device inventory, decoy runtime status |
| `configuration` | 60 s | 5 min | environments, zones, settings |
| `static` | none | never | organization singleton, enumerations |
| `once` | none | never | one-time reads such as an enrollment secret |
| `session` | none | 30 s | the session probe, preserving the Phase 1 cadence |

`@shared/api/freshness.ts` owns what a class means; `test/check-freshness.mjs`
fails the lint if `refetchInterval`, `staleTime`, `refetchIntervalInBackground`,
`refetchOnWindowFocus`, or `refetchOnReconnect` appears anywhere else.

The same object supplies the **staleness threshold** the `stale` data state
renders against, so a cadence change cannot leave the staleness treatment
behind. `WCX-04` shipped that threshold as one interim constant; it is now
per class.

Behaviour that follows from the class:

- a hidden tab issues no interval refetch at all;
- returning to the tab, or focusing the window, refetches a polled class
  immediately rather than waiting out the remainder of an interval;
- regaining connectivity refetches everything except `static`, including
  classes that do not poll — a read taken before an outage is not evidence of
  anything after it;
- retry backoff grows but never exceeds eight intervals, so a failing resource
  keeps being retried and keeps updating its observed age.

### Polling stops with the session

`permitPolling` is a gate the session opens and closes. Signing out already
removes non-session queries, which stops their intervals, but an interval
firing between removal and the next tick would still reach the Control Plane
on behalf of an operator who has left. The gate is consulted by TanStack after
every fetch, so a session that ends between ticks stops the next one.

A background `401` follows the same expiry path a foreground one does. There is
no second route into session handling.

### The recorded transport trigger

`TRANSPORT_RECONSIDERATION_TRIGGER` is data, not prose, so the `P5-W9`
benchmark can assert against it:

> On the `P5-W9` reference environment, if the measured time from a decoy
> interaction to console visibility exceeds **5 seconds at p95** with the
> `critical` class active, or if the aggregate request rate on a three-tab
> operator session exceeds **180 per minute**, a change proposal for a
> server-driven invalidation channel is opened. Until one of those is
> *measured*, polling stands.

### Chunks

| Chunk | Contents |
|---|---|
| `index` | Vite bootstrap, ~1 KB |
| `entry` | shell, providers, router, error boundaries, shared transport, shared UI |
| `login` | the sign-in screen |
| `feature-<name>` | one per feature under `src/features/` |
| `vendor-react` | React and React DOM |
| `vendor-query` | TanStack Query |

Three things about this table are worth knowing before changing it.

**The split point lives in each feature, not in the router.** A feature barrel
is statically imported by the shell and by other features, so a dynamic
`import('@features/...')` from `router.tsx` moves nothing — Rollup says as
much. Each feature therefore exports its own `lazy` route component, which
keeps `WCX-01`'s rule that a feature is entered only through its public API
while still giving the bundler a real boundary.

**`sideEffects` matters.** Without `"sideEffects": ["*.css"]` in
`package.json`, importing one control from the `@shared/ui` barrel keeps the
whole layer. Adding it cut the largest chunk by a third.

**The modal stack is deliberately outside `entry`.** Radix's dialog brings a
focus trap, a dismissable layer, a portal, and scroll locking. The sign-in
screen opens no dialog, and shipping all of that to an unauthenticated visitor
is what pushed the initial login load over its budget. It now travels with the
first feature that opens one.

**`vendor-ui` is not emitted.** Radix's module ids never reach `manualChunks`
under Vite 8's rolldown, so the bundler places it with its consumer instead.
The outcome is better than the declared table: Radix ships only with the screen
that opens a dialog, rather than as a chunk every page fetches. Per-chunk
reporting still gives the attribution the table was for.

### Budgets

`npm run bundle:check` enforces three dimensions and prints every chunk raw and
gzipped, so growth is attributable to a chunk rather than to "the bundle".

| Dimension | Measured | Budget |
|---|---:|---:|
| Initial login load, gzipped | 112 920 | 122 880 |
| Initial authenticated load, gzipped | 127 640 | 204 800 |
| Total JavaScript, raw | 410 508 | 460 800 |
| Total CSS, raw | 25 036 | 32 768 |

A regression above twenty percent against the **committed baseline** in
`check-bundle.mjs` fails the build and requires owner review (`WC-D30`,
`PERF-08`). The baseline is a number in that file, not the previous run: a
change can stay inside a budget while doubling a figure, and only a committed
baseline catches that. Update it deliberately, in the same commit as the change
that moved it, with the reason in the message.

The check also asserts that the login chunk pulls no authenticated feature
chunk, that no chunk name carries a UUID, and that no production source map is
emitted.

Runtime interaction is not measured here. `WCX-12` measures it, where a
data-dense screen exists to measure.

### The router

`react-router-dom` was replaced by `react-router@8`. In v7 the DOM package
became a thin re-export of the core, and only the core has an 8.x line —
`react-router-dom` stops at 7.18.3. The upgrade was import-path-only:
typecheck clean, the whole suite unchanged, and every API the console uses
present with the same signature.

## Text catalogue and wording rules

Every word an operator reads lives in `src/shared/text/catalogue.ts`. That is
not tidiness. This product's differentiator is its wording — `SRC-07` requires
provenance-aware language and `EV-04` requires evidence to precede inference —
and wording spread across forty components cannot be reviewed as a whole. In
one file it can be read in one sitting, which is the only way those two rules
get checked at all.

### Reading and writing text

```tsx
import { plural, t, tx } from '@shared/text';

t('environments.heading')                      // 'Environments'
t('environments.total', { count: 12 })         // '12 total'
plural('environments.zoneCount', 1)            // '1 zone'
tx('common.sessionReadOnlyFull', {             // a sentence around a control
  reauthenticate: <Link to="/login">{t('common.reauthenticate')}</Link>,
})
```

`t` returns a string and cannot emit markup: a value containing `<img …>`
becomes those characters. `tx` returns nodes, and still parses nothing — the
nodes are React elements the caller supplied, never a string to be
interpreted.

Both are typechecked against the catalogue entry itself. An unknown key does
not compile, and neither does a call that omits a placeholder the entry needs,
because the required values are derived from the string with a template-literal
type. Adding `{count}` to an entry immediately fails every call site that does
not supply it. `typeFixtures.test.ts` compiles fixtures of each failure through
the TypeScript API, so the guarantee is proved rather than assumed.

### Key naming

Keys describe **meaning, not wording**. A key has to survive a rewrite of the
sentence it names.

```text
devices.enrollment.secretShownOnce      correct
devices.enrollment.enterThisOnTheHost   wrong — names this draft's phrasing
```

Flat dotted keys in eleven namespaces: `common`, `auth`, `environments`,
`environment`, `devices`, `health`, `states`, `confirm`, `untrusted`, `time`,
`errors`. `catalogue.test.tsx` fails on a twelfth, so a new namespace is a
decision recorded here rather than a key that drifted in.

Two namespaces are keyed by something outside the catalogue and cannot be
renamed freely: `errors.*` is keyed by the `messageKey` values `WCX-02`
reserved, and `health.condition.*` by the backend condition types. Both are
read with a template key, so the whole sub-namespace has to stay in step with
the contract.

### The rules the catalogue exists to hold

1. **Provenance-aware phrasing.** `Observed source`, `Reported by`, `Last
   observed`, `Supplied during authentication` — never an assertion of
   identity.
2. **Absence is never phrased as health.** `No health projection has been
   reported` is correct; `All good` is not. `catalogue.test.tsx` fails on a
   list of phrases in that family.
3. **Inference is labelled** as probable, inferred, or suggested.
4. **Configuration completeness and health never share a word.**
5. **Error text states what happened and the next action** — never a
   diagnostic detail, never a submitted value.
6. **Destructive confirmations name the object and the irreversible effect.**

### What may not go in it

No credential, token, bootstrap value, or recovery code, including as an
example. `catalogue.test.tsx` scans every entry for eight secret *shapes* —
JWT, AWS key id, PEM header, long base64 and hex runs, grouped recovery codes,
credential-carrying URLs, and `token:`-style assignments — rather than for a
wordlist, because whoever adds an example credential will not name it
`password`. A planted fixture proves the scan is not vacuous.

No backend or device text either. A device-supplied reason, a condition type,
a status slug: those are data, rendered through the untrusted components from
the hostile-content contract, never a catalogue key and never translated.

No entry is composed from another. A sentence assembled from fragments at a
call site cannot be reviewed as a sentence and cannot later be translated —
which is why the read-only banner is one entry with a `{reauthenticate}`
placeholder rather than the three fragments it used to be.

### The lint rule

`guardian/no-literal-text`, defined in `apps/web-console/eslint.config.js`,
fails the build on operator-facing text written at a call site. It fires on
JSX text, on a string literal in a child expression, and on a string-valued
attribute.

Attributes are **default-deny**: every name not in `TECHNICAL_ATTRIBUTES` is
treated as operator-facing. That list holds CSS classes, element ids, URLs,
form mechanics, ARIA attributes whose values come from a closed enumeration,
and this repository's own design-system variants. A new prop is a violation
until someone adds it, which is the right way round — the alternative, a
denylist of the four attributes the specification names, would miss
`heading="Environments"` entirely.

Three exemptions, all fixture material rather than operator surfaces: tests,
`shared/testing/**`, and `app/workbench/**`, the development-only gallery that
cannot reach a production build. `literalText.test.ts` asserts the rule is
configured as an error on a real component file and lints eight fixtures
through the plugin imported from the config itself, so the rule proved is the
rule the build runs.

### Size

The catalogue is bundled with the entry chunk and must stay under 12 KiB
uncompressed, measured as `JSON.stringify(CATALOGUE)`. At the end of `WCX-08`
it is **12,196 bytes across 208 entries**, 92 bytes inside the budget.

That is tight, and the way to make room is not to shorten sentences. Two
things do work: `catalogue.test.tsx` fails on any entry no component reads, so
wording left behind by a deleted screen cannot accumulate; and where several
keys name the same thing in the same words — five copies of `The Control
Plane` — they collapse to one key, which is also one place to edit when the
thing is renamed. Two keys that merely *happen* to share wording today stay
separate: a heading and a button are free to diverge, and meaning-keyed names
are the whole point.

## Time presentation

`Timestamp` from `@shared/ui` renders every instant in the console. Its one
rule, seen from several sides: never claim more about an instant than the data
supports.

```tsx
<Timestamp value={health.received_at} precision="second" uncertainClock={degraded} />
<Timestamp value={zone.updated_at} />
```

| Prop | Values | Default |
|---|---|---|
| `value` | ISO-8601 from the backend, or `null`/`undefined` | required |
| `precision` | `minute`, `second` | `minute` |
| `mode` | `absolute`, `absoluteWithRelative` | `absolute` |
| `uncertainClock` | the source device reported degraded `clock_quality` | `false` |

### Precision

`second` is **mandatory** where ordering is the thing being established, and
`minute` is permitted elsewhere.

| Surface | Precision | Why |
|---|---|---|
| Health projection `received_at` | `second` | A health projection is evidence and its ordering is what an operator reasons about |
| Enrollment secret expiry | `second` | A 15-minute secret is not actionable to the minute |
| Attacker journey and audit entries | `second` | Ordering *is* the finding |
| Health condition transitions | `second` | Same |
| Zone and environment `updated_at` | `minute` | Configuration edits, not evidence |
| Device last-seen in a list | `minute` | Scanned, not correlated |

### What it will not do

**Never a bare local time.** The zone abbreviation is always visible, so a
timestamp copied into an incident report still says which clock it was on.
`Intl.DateTimeFormat` rejects `timeZoneName` alongside `dateStyle`/`timeStyle`,
so the format is spelled out component by component — that combination throws
at render time, in every browser, and is worth knowing before reaching for the
shorter form.

**Never a fabricated instant.** An absent, empty, or unparseable value renders
`No timestamp was recorded` — never a default date, never the epoch, never now.
Parsing is strict ISO-8601 with a time of day, because `Date.parse` is
permissive by design: it reads `'0'` as the first of January 2000, and a
best-effort parse here is exactly the fabricated precision the rule forbids.

**Never a relative time alone.** `15 minutes ago` cannot go in a report and
cannot be compared against a device log, so it appears only beside the absolute
value, in `absoluteWithRelative` mode. It recomputes once a minute and
announces nothing when it does: no live region, no `role="status"`. The
interval exists only in that mode and is cleared on unmount — the shared
layer's no-polling rule exempts this module on the strength of the two tests
that assert exactly those two properties.

**Never a silent bad clock.** When the source device's `clock_quality`
condition is not `True`, the timestamp carries a visible marker and an
accessible description saying so. `Unknown` counts as degraded: a device that
has not reported its clock quality has not established that its clock is good.
The marker uses `--text-muted` and a dashed `--line-strong` border — a neutral
token, never a severity one. A bad clock is a caveat about evidence quality,
not an incident, and colouring it as one would be a false signal.

The absolute UTC instant is always retrievable: it is in the accessible
description, not only in the `title`, because `title` is mouse-only. Age
formatting (`formatAge`) lives in this module too, so the `stale` state and a
relative timestamp cannot disagree about how long ago something was.

## Navigation and scope

The console's root is the incident dashboard, because `UX-01` says it is. The
dashboard does not exist yet — Phase 3 builds it — so `/` renders a
placeholder. Deciding the shell now means Phase 3 adds a screen instead of
rebuilding the frame around one.

### The route tree

| Path | Screen | Chunk |
|---|---|---|
| `/login` | Sign in | `login` |
| `/` | Home placeholder | `home` |
| `/environments` | Environment list | `feature-environments` |
| `/environments/:environmentId` | Environment detail | `feature-environments` |
| `/environments/:environmentId/devices/:deviceId` | Device detail | `feature-devices` |
| `/account` | Sessions and password | `feature-account` |
| `/__components` | Component workbench | development only |
| `*` | Not found | `home` |

**No path moved in `WCX-10`.** Two behaviours changed: `/` was a redirect to
`/environments` and is now a screen, and an unknown path was a redirect to
`/environments` and is now a not-found screen.

That second change is the one to understand. Redirecting an unknown address
hides the mistake: an operator who followed a stale link lands on a working
screen and concludes the link was right. In a console where the address
carries the scope, silently arriving somewhere else is the same class of error
as falling back to a different environment. So the address bar keeps what was
asked for, the screen says it does not resolve, and navigation stays mounted.

The catch-all sits *inside* `RequireAuth`, so a signed-out visitor still
reaches sign-in rather than a not-found page that tells them nothing.

A navigation entry exists only when its screen does. `Home`, `Environments`,
`Account` — that is the list, and `navigation.test.tsx` fails if a catalogue
label like `Incidents` or `Decoys` appears before its screen.

### The `?env=` scope parameter

`WC-D14` puts the environment scope in the URL rather than in memory or
storage, so a link pasted into an incident channel resolves to the same view
for whoever opens it.

It is also untrusted input — anyone can type it, and it arrives from a link
the console has never authenticated. `src/app/scope.ts` holds three rules and
all three are about refusing to guess:

1. It is validated against the UUID pattern **before** use, so a malformed
   value never reaches a request path.
2. An unknown or denied environment renders `not-found` or `denied` as the
   Control Plane reports it.
3. **There is no fallback.** Not to the first environment, not to the only
   environment, not to the last one viewed.

Rule 3 is the one worth being stubborn about. An operator who believes they
are looking at environment A while seeing environment B will act on the wrong
network, and nothing on the screen would tell them.

The selector follows from it. With several environments and no parameter,
nothing is selected — picking the first would put an operator in front of a
network they did not ask for, and every screen after that would look correct.
With exactly one environment there is no ambiguity, so it is preselected *and
written to the URL*: a link that resolves differently depending on how many
environments the reader can see is not a deterministic link.

While the list is loading, refused, or empty, the selector is disabled with
the reason, never hidden (`WC-D07`).

Scope is absent when a screen already carries the environment in its path.
Today the placeholder is its only consumer; Phase 3's incident surfaces are
what it is really for, and their query keys take the environment as a scope
segment following the `WCX-02` key shape.

### The breakpoint and the disclosure

One number: **900 pixels**, in `useNarrowViewport.ts` and in the stylesheet's
media query, each naming the other. Two that drift apart give a width at which
the disclosure believes it is closed while the layout has already expanded,
and at that width the operator block sits inside a panel nothing can open.

Above it: a persistent sidebar with navigation, the scope selector, and the
operator block. At or below it: all three move into a disclosure — a real
button with `aria-expanded` and `aria-controls`, closing on escape and on
navigation and returning focus to its trigger.

**No operator control is removed at any width, from 320 pixels upward.** This
is the single most important rule in the shell and it is here because it was
broken: `P1-W11` gave the operator block `display: none` below 900 pixels, so
an operator who suspected a stolen session could not sign out from the device
in their hand. `WCX-05` closed the defect by making the block wrap; `WCX-10`
replaced that with the disclosure.

The suite that shipped the original defect passed, because nothing in it had a
width. jsdom has no layout and no `matchMedia`, so the shell reads the
viewport through `matchMedia` and `shared/testing/viewport.ts` answers from a
width the test sets. `shell.test.tsx` asserts the invariant at 320, 375, 900,
and 1440; the browser suite proves it again at 375 and 320 by keyboard, with
sign-out reached through the disclosure and no horizontal page scroll.

If `matchMedia` is missing the shell treats the viewport as **wide**. That is
the fail-safe direction on purpose: wide renders every operator control
unconditionally, so an environment the console cannot measure gets the layout
that hides nothing.

### The home placeholder

A blank incident dashboard is not neutral. An operator who glances at one and
comes away believing Guardian looked and found nothing is worse off than one
who never opened it — that is the exact failure this product exists to
prevent, reproduced in its own shell.

So the placeholder says what it is: the dashboard is not built, the absence of
content is a fact about the console rather than about the network, and here
are the two screens that work. `navigation.test.tsx` asserts the negative
directly — no count, no empty list, no "all clear", no "secure", no list or
table element at all.

If you are the one adding the real dashboard, that test is the contract: it
should be deleted deliberately, not edited into passing.

## Operator lifecycle actions

Device disable and revoke, re-enrollment, enrollment-token revocation, zone
edit and delete, session revocation, and password change. Every one of these
was already a working, authorised, audited Control Plane operation before the
console exposed it; `AUTH-06` covers the audit trail and nothing here adds to
it. What the console adds is the gate.

### The confirmation levels

`src/shared/ui/confirm/levels.ts` is the table, and it is the only place a
level is decided. A call site names an action, never a level, so a screen
cannot downgrade a device revoke to a "are you sure?" prompt.

| Action | Level | Why |
|---|---|---|
| `zone.rename` | 1 | Reversible configuration edit |
| `device.enable` | 1 | Reversible, and immediate |
| `device.disable` | 2 | Reversible by re-enrollment, but it stops a live Edge opening new sessions |
| `enrollment.revoke` | 2 | Destructive, but a new token can be issued |
| `zone.delete` | 2 | Destructive; may orphan configuration |
| `device.revoke` | 3 | Irreversible trust decision |
| `device.reenroll` | 3 | Re-establishes the trust revocation removed |
| `session.revoke` | 3 | Immediate access removal |
| `account.password` | 3 | Credential change |

Level 1 renders no dialog at all — passing a level 1 action to
`ConfirmationDialog` returns `null` on purpose, so a screen that adds a modal
to a reversible action gets no modal rather than a wrong one. Level 2 is a
modal whose confirm button names the effect. Level 3 adds a typed confirmation
of the object's name and step-up reauthentication.

`device.enable` has no screen. The contract has no re-enable endpoint, so a
control would fail; `DEVICE_TRANSITIONS` records that the path from `disabled`
back to `active` does not exist and the device screen says so instead.

### Step-up reauthentication

```tsx
const stepUp = useStepUp();
// ...
<ConfirmationDialog action="device.revoke" objectName={name} open stepUp={stepUp} … />
{stepUp.element}   // render once, near the action it guards
```

The prompt opens **before** the confirmation, not after. Asking an operator to
type a device name and only then telling them to reauthenticate wastes the
typing and presents the confirmation as if the action were already authorised.

It asks for the password and a fresh MFA proof every time, through the same
TOTP-or-recovery control the sign-in screen offers — a step-up that quietly
accepted only TOTP would lock out an operator holding recovery codes precisely
because they lost the authenticator. It cannot be satisfied by the session
cookie, and there is no cached proof to accept.

The mark is single-use. `request(action)` reauthenticates and marks one action;
`consume(action)` spends it and clears it. A second irreversible action finds
nothing to spend and asks again. Nothing else can read the mark: it is a ref
inside the hook with no accessor. If you are adding a level 3 action, you get
this by passing `stepUp` and doing nothing else.

**Known limitation.** Step-up runs the login exchange, so the Control Plane
issues a new session and the absolute lifetime restarts. `WCX-09` section 8.2
asks that it not, but the contract offers no endpoint that verifies a password
and a fresh MFA proof without creating a session. Recorded in
`security/wcx-09-operator-completeness-review.md` with the follow-up.

### Restore write access

`W11-C3-A` keeps the synchronizer proof in browser memory, so a reload leaves
a valid session that cannot change anything. `Restore write access` on the
read-only banner exchanges the surviving cookie for a new proof through
`POST /v1/auth/csrf`, which change proposal `0003` approved.

It is the one request in the console that deliberately carries **no** CSRF
token, because not having one is the situation it exists to resolve. The
Control Plane requires the session cookie and an exact origin match instead,
and the cookie is `SameSite=Strict`, so a cross-site page never sends it.

What it restores is level 1 and level 2. **Level 3 stays gated behind step-up
whatever the proof's age**, and that separation is the whole shape of the
change proposal: a fresh proof says the browser has a session, not that the
operator is still the one holding it.

Three refusals, deliberately different:

| Response | What the console does |
|---|---|
| `401` | Ends the session and goes to sign-in. The cookie was not valid after all, and leaving an operator on a read-only banner for a session that no longer exists tells them something untrue |
| `429` | Names the wait, stays read-only, and never retries |
| anything else | Says write access could not be restored and offers full sign-in |

Full sign-in stays on the banner beside the cheap path, because a refused
re-issue must leave a way forward rather than a dead end.

The endpoint does not extend the absolute session lifetime. `ReissueCSRF` in
`internal/storage/auth.go` writes `csrf_hash` and nothing else — an
`expires_at` written there would turn a re-issue into a session extension,
which is the thing the proposal was approved on the condition of not doing.
It is rate limited on the same persistent throttle the login path uses, and it
emits an `auth.csrf.reissued` audit event against the session.

**The trade this makes is stated in the proposal and is not reasoned away**:
under an assumed script-execution foothold in the console origin, an attacker
can call this and obtain a proof, so the CSRF token stops being an incidental
second barrier. That is bounded by the approved CSP and by the `WCX-06`
untrusted-content contract, under which such a foothold already implies a
systemic failure — and it buys a step-up gate on irreversible actions that did
not exist at all before.

### When a destructive action fails

Nothing is optimistic. A transition returns `204`, the console invalidates the
queries, and what an operator sees next is whatever the Control Plane returns —
never a state the console assumed. On failure the displayed state is left
exactly as it was and the message says so, because an operator unsure whether a
revoke landed will either repeat it or trust a device they meant to remove.

Three failure shapes, deliberately worded apart:

| What happened | What the operator reads |
|---|---|
| The request failed | "… did not complete. Nothing changed on this device." |
| A revision conflict (`409`/`412`) | "Another change reached this … first, so nothing was written." plus a reload control |
| A rejected value (`400`) | The policy that was violated, and that nothing changed |

A conflict is not a validation failure and must never read like one: the value
was right, someone else was faster. Presenting it as invalid input sends the
operator to fix the wrong thing. Zone edit and delete both send `If-Match` with
the revision the operator was looking at, and there is no retry path that drops
the header — that retry would overwrite a change the console never showed
anyone.

### One-time material

Enrollment secrets and re-enrollment tokens both go through
`OneTimeSecretDialog` in `@shared/ui`. One implementation, because the rules it
holds are the kind only ever broken by a copy drifting:

- the value is a prop, never state in the dialog and never in a query cache.
  The caller holds it in route-local state, so route exit destroys it;
- dismissal unmounts the dialog, so the value leaves the DOM;
- `pagehide` dismisses it too, so a closed or reloaded tab leaves nothing in a
  restored page or a back-forward cache entry;
- there is no copy control, no reveal toggle, and no second read path.

`oneTimeSecret.test.tsx` asserts that last point at the source level, not just
behaviourally: the dialog contains no `useState` or `useRef`, no `queryOptions`
names an issuing endpoint, and exactly two modules in the console read a
`.token` field. Adding a third is a deliberate act that fails a test first.

Enrollment token *lists* never carry a value. That is not a filter — the
contract's `EnrollmentTokenSummary` has no token field, so the console is never
sent one.

### Sessions

The current session is labelled `This session` and has no revoke control; its
action cell points at sign-out. Two paths to the same effect is one an operator
can take by mistake while aiming at a stolen session. Revoked sessions stay
listed, because a session that ended is exactly the row an intrusion
investigation needs.

Password change revokes every session for the owner server-side and returns
fresh credentials, which the console installs immediately — the proof it held a
moment earlier is dead. It does **not** say how many sessions ended, because
the response does not say: it points at the session list instead. If you are
tempted to add a count here, the response still will not contain one.

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
