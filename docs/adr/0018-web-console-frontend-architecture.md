# ADR 0018: Web Console frontend architecture

- Status: Proposed
- Date: 2026-09-03
- Decision refs: WC-D01, WC-D02, WC-D05, WC-D06, WC-D09, WC-D24
- Approved baseline refs: TS-02, TS-03, TS-04, TS-05, DT-07, CP-04, CP-08,
  RE-10, SEC-08, SA-11
- Acceptance refs: WCX-000 master decision record; PERF-05; PERF-07; SEC-08

## Context

`P1-W11` delivered the Phase 1 Web Console shell: four routes, a hand-written
API wrapper, hand-declared DTO types, two Radix primitives, and one CSS
module. The approved baseline fixes the framework, build tool, server-state
library, routing class, transport style, and static-SPA deployment, but leaves
open how the console is structured internally as Phase 2 adds decoy
management, Phase 3 adds incidents, journey, and evidence, and Phase 4 adds AI
and notification surfaces.

Six of those open questions are architecture rather than implementation
detail, because they determine module ownership, contract coupling, the
network model, the delivery model, the component substrate, and the rendering
trust boundary. They are recorded here so later packages inherit them instead
of re-deriving them.

## Options considered

1. **Defer structure until Phase 3.** Lowest immediate cost; every later
   package re-decides ownership and coupling, and the Product Owner arbitrates
   repeatedly.
2. **Adopt a broader framework.** A server-rendered or full-stack framework
   would supply routing, data loading, and structure. Rejected: `DT-07` and
   `CP-08` approved an authenticated static SPA and explicitly excluded SSR as
   the default.
3. **Record a bounded set of frontend architecture decisions now, inside the
   approved baseline.** Selected.

## Decision

### Module boundaries — WC-D01

Feature-sliced structure: `src/app/`, `src/features/<domain>/`,
`src/shared/`, `src/generated/`. A feature exposes exactly one public
`index.ts`. `shared` may never import a feature. Feature-to-feature imports
require an explicitly listed pair. Enforced by lint, mirroring the module
boundary discipline already enforced in the Control Plane under `CP-02` and
ADR 0002.

### API contract coupling — WC-D02

Types are generated from `openapi/guardian.yaml` with `openapi-typescript`,
committed under `src/generated/`, and verified fresh in CI. No runtime OpenAPI
client library is adopted; the existing thin `request()` wrapper is retained.
This closes the `RE-10` drift introduced by hand-declared DTO types while
adding zero runtime dependency and leaving the reviewed security behaviour of
the transport untouched.

### Data freshness transport — WC-D05

Polling with a single declarative freshness policy. No SSE and no WebSocket.
The console is a browser client of a stateful, non-horizontally-scaled Control
Plane (`CP-06`, `CP-07`); adding long-lived server-push connections is a real
operational cost that no measurement currently justifies. A measured trigger is
recorded as data: if the reference benchmark shows incident visibility above
the `PERF-05` threshold with the tightest polling class active, a change
proposal for server-driven invalidation is opened rather than the threshold
being adjusted.

### Routing and delivery — WC-D06

The explicit route tree is retained without router loaders or actions, keeping
TanStack Query the single server-state authority under `TS-04`. Route-level
lazy loading introduces feature-aligned chunks with an isolated login chunk.
React Router moves from 7 to 8 as behaviour-preserving maintenance under the
repository version policy.

### Component substrate — WC-D09

Radix Primitives with CSS Modules and CSS variables continue as the single
design system, consolidated onto the `radix-ui` package. MUI and
shadcn/Tailwind were both considered. MUI's default runtime styling conflicts
with the approved CSP prohibition on `unsafe-inline`, and its zero-runtime
alternative is not in active development; shadcn/ui would preserve the Radix
accessibility base but replace the approved CSS Modules model. The decisive
factor is that this product's severity, confidence, and uncertainty encoding is
a differentiator, and inheriting a general-purpose design system's colour and
emphasis semantics would surrender control precisely there. Headless utilities
for tables and virtualisation are permitted; they carry no visual semantics and
are not a second design system.

### Rendering trust boundary — WC-D24

A canonical untrusted-content contract. Attacker-influenced values render only
through dedicated components that escape control and ANSI sequences rather
than interpreting them, make bidirectional and zero-width characters visible,
never emit a URL-bearing attribute, never treat a filename as a path or a
download target, and bound length with an explicit truncation indicator. The
boundary is compiler-enforced through a branded untrusted string type applied
at the API layer.

## Consequences

Later packages inherit ownership, coupling, network, delivery, component, and
rendering decisions rather than arbitrating them. The costs are explicit:
feature-sliced structure adds ceremony while the console is small; retaining
Radix means building tables, comboboxes, and date controls in-house during
Phase 3; and polling accepts a bounded visibility delay and a request-rate
floor that must be re-examined against real measurement in Phase 5.

Follow-up work is specified in `WCX-01` through `WCX-15`. Two accepted
decisions outside this ADR alter approved contracts and are routed to change
proposals `0003` and `0004`.

## Security and failure behaviour

Trust boundaries are unchanged. Authorization remains a Control Plane
responsibility; the console's capability seam is presentation only and is
documented as such. Session, CSRF, and secret-lifetime semantics established
by `P1-W11` are preserved by every decision here. The rendering trust boundary
strengthens `SEC-08` from a principle into a compiler- and test-enforced
contract, and extends it to visual forgery classes that text escaping alone
does not address. Generated types add no runtime code and therefore no runtime
attack surface. Chunk names carry route identifiers only, never resource
identifiers.

Failure behaviour: a failed chunk load renders a route boundary fallback
rather than blanking the console; a failed refresh renders explicit staleness
or degradation and never a healthy or empty appearance; an unrecognised
backend status maps to a generic unexpected error and is never rendered
verbatim.

## Evidence

- `docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/00-master-decision-record.md`
- `WCX-01`, `WCX-02`, `WCX-06`, `WCX-07`, `WCX-09` package specifications
- `docs/change-proposals/0003-web-console-session-csrf-reissue.md`
- `docs/change-proposals/0004-web-console-error-contract.md`
