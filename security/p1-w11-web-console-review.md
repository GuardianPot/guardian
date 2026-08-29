# P1-W11 Web Console security review

## Review state

Implementation review complete; Product Owner application/security evidence
acceptance and human-controlled PR merge remain required.

Work package: `P1-W11`. Decisions: DT-07, CP-04, CP-08, TS-02 through TS-05,
SA-11, SEC-08, and Product Owner approval of W11-C1-A through W11-C8-A on
2026-08-29. Acceptance reference: Phase 1 browser onboarding skeleton E2E.

## Security boundaries

- The browser receives only an HttpOnly, Secure, SameSite=Strict session cookie
  and a synchronizer CSRF proof. The proof exists in React memory only.
- Reloaded sessions are read-only until fresh MFA reauthentication returns a
  new CSRF proof. Logout is also a protected mutation.
- Enrollment secrets bypass TanStack Query and live only in route-local React
  state. Dismissal removes their DOM/state reference; route exit and unload
  destroy the owning component/page.
- No application code uses localStorage, sessionStorage, IndexedDB, raw HTML,
  inline scripts, `unsafe-inline`, `unsafe-eval`, analytics, or client logs.
- Device inventory is a separate authorized projection containing only the
  approved identity, display, state, timestamp, and optional active-certificate
  expiry fields. Authorization runs before storage; lists are fixed at 200 and
  ordered by device ID.
- Health UI consumes only the P1-W9 backend projection and renders eight
  textual conditions. Inventory, process presence, or channel presence cannot
  manufacture a healthy status.
- The final image is Distroless/nonroot. Node 24 and package tooling exist only
  in the digest-pinned build stage.

## Dependency and license review

All direct npm versions are exact and the complete dependency graph is locked.
The direct-package lock metadata reports only MIT, Apache-2.0, and MPL-2.0
licenses: the sole MPL-2.0 direct dependency is the test-only accessibility
adapter `@axe-core/playwright`. Production and complete `npm audit` runs report
zero known vulnerabilities, and `npm install-scripts ls` reports no unreviewed
install scripts. No browser dependency is loaded from a CDN at runtime.

## Abuse and failure cases reviewed

| Case | Control and evidence |
|---|---|
| Hostile display names or health messages | React text rendering plus JSON HTML escaping; component test verifies markup is not created. |
| Session expiry or missing cookie | Authenticated 401 clears session/query state; browser test clears the cookie and observes sign-in. |
| CSRF replay or forged value | Mutations send the memory-only proof and exact browser origin; real browser test confirms a forged proof receives 401. |
| Hard reload | Cookie restores reads only; all mutation controls remain disabled until MFA reauthentication. |
| Secret in cache/storage/artifact | Secret call bypasses query/mutation caches; unit and browser tests assert DOM removal and empty storage; traces/videos/automatic screenshots are off. |
| API route confused with SPA route | Unknown `/v1` receives JSON 404 and `no-store`; handler test proves no index fallback. |
| Stale or invented health | Missing projection is unavailable; `False` and `Unknown` remain blocking; disconnect/reconnect is exercised over the real device channel. |
| Cross-environment inventory read | Environment and device IDs are jointly scoped in SQL; integration test proves a mismatched environment returns not found. |
| Unbounded inventory | Service fixes the list limit at 200; storage rejects larger bounds. |
| Active-certificate disclosure | Only expiry is projected; serial, fingerprint, PEM, key, and history are excluded. |
| Dependency compromise surface | Direct versions and lockfile are exact; install scripts are denied unless reviewed; npm production and complete audits report zero vulnerabilities. |
| Bundle/runtime expansion | Bundle check caps JS at 450 KiB and CSS at 32 KiB with no source maps; container smoke verifies no Node runtime is required. |

## Evidence commands

```text
task web:check
task web:e2e
npm audit --omit=dev
npm audit
npm run openapi:check
go -C apps/control-plane test ./...
go -C apps/control-plane test -tags=integration -run TestDeviceInventoryIsBoundedScopedAndIncludesOnlyActiveCertificateExpiry ./internal/storage
task container:check
task policy
task contracts
```

The Linux CI browser job runs Chromium, Firefox, and WebKit sequentially against
disposable real services. Its only retained artifacts are three screenshots
taken after enrollment-secret dismissal. The test-only health publisher is
outside every production build and can affect only the disposable test Control
Plane to which its one-time device identity belongs.

## Known limitations and residual risk

- Phase 1 has one local owner and no role editor, OIDC, or delegated access.
- Bootstrap provisioning and recovery-code custody remain an out-of-band owner
  ceremony; the SPA does not manage them.
- Browser refresh requires MFA before any mutation by design.
- The E2E all-true health publisher is a test-only protocol client, not proof
  that arbitrary production hosts satisfy runtime/helper prerequisites.
- Current private-repository plan limits still prevent repository rulesets or
  classic branch protection. CI success, this review, owner acceptance, and PR
  merge remain distinct gates.
