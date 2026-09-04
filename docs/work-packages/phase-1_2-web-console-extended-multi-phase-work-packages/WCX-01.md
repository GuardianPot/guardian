---
id: WCX-01
phase: 2
wave: foundation
title: Module boundaries, capability seam, lint and typecheck hardening
status: approved-for-implementation
risk: medium
components:
  - web-console
decision_refs:
  - WC-D01
  - WC-D07
  - WC-D29
  - TS-02
  - TS-04
  - IA-06
  - CP-02
remediates:
  - P1-W11 GAP-5
acceptance_refs:
  - WCX-000 section 3.1
  - Phase 1 browser onboarding skeleton E2E must pass unchanged
depends_on:
  - P1-W11
allowed_paths:
  - "apps/web-console/src/**"
  - "apps/web-console/eslint.config.js"
  - "apps/web-console/package.json"
  - "apps/web-console/tsconfig.json"
  - "apps/web-console/tsconfig.app.json"
  - "apps/web-console/tsconfig.node.json"
  - "apps/web-console/vite.config.ts"
  - "package-lock.json"
  - "docs/engineering/web-console-dependency-policy.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-01.md"
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

# WCX-01 — Module boundaries, capability seam, lint and typecheck hardening

## 1. Purpose

Establish the module boundary model, the permission-aware seam, and the static
analysis gates that every later Web Console package depends on, without
changing any observable console behaviour.

## 2. Why now

`P1-W11` delivered a flat technical layout of four routes. Phase 2 adds decoy
management, Phase 3 adds incidents, journey, and evidence, and Phase 4 adds AI
and notifications. Deciding module ownership once, and enforcing it
mechanically, prevents `components/` and `api/client.ts` from becoming
cross-cutting collection points. The capability seam must exist before
`WCX-09` introduces the first security-sensitive mutation controls.

## 3. Inputs and decisions

- `WC-D01` — feature-sliced boundaries with lint-enforced public module APIs.
- `WC-D07` — a single `useCapability` seam; no role model is implemented.
- `WC-D29` — dependency admission bar recorded as repository policy.
- `IA-06` — the internal authorization model may carry a hook distinguishing
  authenticated operator from owner or security-sensitive actions.
- `TS-04` — server state stays in TanStack Query; local state stays in React
  primitives; no Redux-like global store is introduced.
- Remediates `P1-W11 GAP-5`.

## 4. Dependencies

`P1-W11` must be merged. No other package may start until this one is
accepted, because every later package writes into the layout defined here.

## 5. Scope

1. Introduce the directory layout in section 9.1 and move existing source into
   it without behaviour change.
2. Add path aliases and wire them into TypeScript, Vite, and Vitest.
3. Add the `useCapability` seam and route the existing `canMutate` gate
   through it.
4. Add import-boundary, React Hooks, and type-aware ESLint rules.
5. Typecheck the previously unchecked configuration files.
6. Record the console dependency admission policy.

## 6. Non-goals

- No new product capability, route, screen, or API call.
- No visual change. No CSS is rewritten; only import specifiers move.
- No role model, permission backend, or authorization semantics.
- No `eslint-plugin-jsx-a11y`; that rule set belongs to `WCX-05`.
- No design-system, component, token, or error-taxonomy work.
- No dependency additions other than the ESLint plugins named in 9.1.

## 7. Allowed paths

Only the paths in frontmatter. Control Plane, Edge Agent, contract, and test
directories are forbidden. `tests/e2e/web-console/**` is deliberately
forbidden: the existing browser suite must pass unchanged as proof that this
package preserved behaviour.

## 8. Security constraints

1. No change to authentication, session, CSRF, or secret-lifetime behaviour.
   Moving `AuthContext` between directories must not alter when the CSRF proof
   is created, cleared, or read.
2. The capability seam is **never** a security authority. Its module
   documentation must state that authorization is enforced by the Control
   Plane and that the seam only decides presentation.
3. A denied capability disables a control and renders a reason. It must not
   hide the control, because a missing control reads as a broken product to a
   generalist operator.
4. No secret, session, or CSRF value may be routed through the seam.
5. `src/shared/testing/**` must remain excluded from the production build; a
   test asserts this.

## 9. Implementation requirements

### 9.1 Technical requirements

Directory layout, exactly:

```text
apps/web-console/src/
  app/          router, providers, application entry, root boundaries
  features/
    auth/       session, login, reauthentication
    environments/
    devices/
    health/
  shared/
    api/        transport, error taxonomy, query client configuration
    auth/       capability seam
    ui/         design-system components (populated by WCX-04)
    theme/      tokens (populated by WCX-03)
    text/       catalogue (populated by WCX-08)
    hooks/
    testing/    test-only helpers, never imported by production code
  generated/    generated OpenAPI types (populated by WCX-02)
```

Each `features/<name>/` exposes exactly one `index.ts`. A feature's internal
files are not importable from outside that feature.

Path aliases, declared identically in `tsconfig.app.json`, `vite.config.ts`
`resolve.alias`, and the Vitest configuration:

| Alias | Target |
|---|---|
| `@app/*` | `src/app/*` |
| `@features/*` | `src/features/*` |
| `@shared/*` | `src/shared/*` |
| `@generated/*` | `src/generated/*` |

Import rules, enforced by lint and not by convention:

1. `app/**` may import `features/*` public APIs and `shared/**`.
2. `features/<a>/**` may import `shared/**`, `generated/**`, and its own
   internals.
3. A feature may import another feature only through that feature's `index.ts`,
   and only for pairs listed in the ESLint configuration. The initial allowed
   list is empty; adding a pair requires a comment naming the reason.
4. `shared/**` may import `shared/**` and `generated/**` only. It may never
   import `features/**` or `app/**`.
5. `generated/**` imports nothing from the application.
6. Relative imports that escape a feature (`../../features/...`) are errors.

Capability seam, in `src/shared/auth/capability.ts`:

```ts
export type Capability =
  | 'environment.create'
  | 'environment.update'
  | 'zone.create'
  | 'zone.update'
  | 'zone.delete'
  | 'device.enroll'
  | 'device.disable'
  | 'device.revoke'
  | 'session.revoke'
  | 'account.password';

export type CapabilityDenial =
  | 'reauthentication-required'
  | 'not-permitted';

export type CapabilityDecision =
  | { allowed: true }
  | { allowed: false; denial: CapabilityDenial };

export function useCapability(capability: Capability): CapabilityDecision;
```

Phase 2 behaviour: every capability returns `{ allowed: true }` when the
session holds a CSRF proof, and
`{ allowed: false, denial: 'reauthentication-required' }` otherwise. The
`not-permitted` variant is defined but unreachable in this phase and must be
exhaustively handled by callers.

Existing `auth.canMutate` call sites in `Shell`, `EnvironmentsPage`,
`EnvironmentPage`, and `DevicePage` route through `useCapability` with the
capability matching the control. `canMutate` is removed from the public
`AuthContext` value.

ESLint additions:

1. `eslint-plugin-react-hooks` recommended rules, as errors.
2. `typescript-eslint` type-aware configuration
   (`recommendedTypeChecked`), with `projectService` enabled.
3. An import-boundary rule set implementing rules 1 to 6 above, using
   `eslint-plugin-boundaries` or `import/no-restricted-paths`.
4. `no-restricted-imports` forbidding `@shared/testing` outside test files.

TypeScript:

1. Add `noUncheckedIndexedAccess`, `noImplicitOverride`, and
   `exactOptionalPropertyTypes` to `tsconfig.app.json`.
2. The `typecheck` script becomes `tsc -b`, so `tsconfig.node.json` is
   actually built and `vite.config.ts` is typechecked.
3. Add `test/check-bundle.mjs` to a checked project or convert it to
   TypeScript; it must not remain unchecked.

Dependency policy document `docs/engineering/web-console-dependency-policy.md`
records the `WC-D29` admission bar: purpose, decision reference, gzipped size
and budget effect, licence, install-script status, transitive dependency
count, and maintenance evidence, all recorded in the introducing pull request.
Licences are limited to MIT, Apache-2.0, and BSD for production dependencies;
MPL-2.0 is permitted for development-only dependencies.

### 9.2 UI/UX requirements

No visual or interaction change. The rendered DOM, text, and control
enabled/disabled states must be identical before and after this package, with
one exception: a disabled control whose capability denial is
`reauthentication-required` renders the existing re-authentication wording
through the seam rather than through an inline `canMutate` check.

### 9.3 Accessibility requirements

No regression. Existing skip link, landmark, label, and error-association
behaviour is preserved byte-for-byte in the rendered output. The existing
Playwright axe scan must continue to report no serious or critical issue.

### 9.4 API and data contracts

None changed. No request path, method, header, or body is altered. The
`generated/` directory is created empty with a placeholder that `WCX-02`
replaces.

### 9.5 Error and failure behaviour

Unchanged. The `guardian:unauthorized` event, the 401 handling, and the query
cache clearing behaviour move directory but keep identical semantics, proven
by the existing `AuthContext` tests passing without modification to their
assertions.

### 9.6 Internationalisation and theme

None. Text remains where it is; `WCX-08` extracts it. Tokens remain where they
are; `WCX-03` restructures them.

### 9.7 Performance

The production bundle must not grow. The existing budget of 450 KiB JavaScript
and 32 KiB CSS remains, and the measured bundle size after this package must
be within 2 percent of the size before it. All added tooling is
development-only.

### 9.8 Observability

None. No logging, telemetry, or diagnostic surface is added.

### 9.9 Documentation

Update `docs/runbooks/web-console/development.md` with the directory layout,
the alias table, and the import rules. Add
`docs/engineering/web-console-dependency-policy.md`.

## 10. Required tests

### 10.1 Unit and component

1. Every existing Vitest test passes with assertions unchanged; only import
   specifiers may be edited.
2. `useCapability` returns `allowed: true` for each capability when a CSRF
   proof exists.
3. `useCapability` returns `denial: 'reauthentication-required'` for each
   capability when the CSRF proof is absent, including the reload-restored
   read-only session.
4. A control denied by the seam is rendered, disabled, and accompanied by its
   reason. A test asserts the control is present in the DOM, so hiding it is a
   failure.
5. A build test asserts no production module resolves `@shared/testing`.

### 10.2 Static analysis

1. `npm run lint` passes with `--max-warnings=0`.
2. A fixture-based test proves each import rule fails: a file importing a
   feature internal from another feature, a `shared` file importing a feature,
   and a relative import escaping a feature each produce a lint error.
3. `npm run typecheck` runs `tsc -b` and covers `vite.config.ts`.

### 10.3 Browser and E2E scenarios

The existing `tests/e2e/web-console/onboarding.spec.ts` suite passes unchanged
in Chromium, Firefox, and WebKit. No new scenario is added by this package.
Because `tests/**` is forbidden here, an unchanged suite passing is the proof
of behaviour preservation.

## 11. Acceptance criteria and Definition of Done

1. The directory layout, aliases, and import rules in 9.1 exist and are
   enforced by a failing lint on violation.
2. `useCapability` exists, all former `canMutate` call sites route through it,
   and `canMutate` is no longer exported.
3. React Hooks rules, type-aware rules, and the stricter compiler options are
   active and the workspace is clean under them.
4. `tsc -b` typechecks the configuration project.
5. The dependency policy document exists.
6. `task web:check` and `task web:e2e` pass.
7. Bundle size is within 2 percent of the pre-package measurement.
8. No product behaviour, text, route, or API call changed.

## 12. Evidence required

- `task web:check` output including the bundle line, before and after.
- `task web:e2e` result for all three browsers.
- Lint output for the three boundary-violation fixtures showing failure.
- A diff summary showing that no `.css` file and no user-visible string
  changed.
- Dependency admission table for each added ESLint plugin.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- preserving behaviour requires changing authentication, session, CSRF, or
  secret-lifetime semantics;
- a boundary rule cannot be expressed without granting a feature-to-feature
  import that is not obviously justified;
- the stricter compiler options reveal a defect whose correct fix changes
  user-visible behaviour;
- an existing test's assertion, not merely its imports, must change;
- the bundle grows by more than 2 percent.

## 14. Deliverables

Restructured console source under the new layout, path aliases wired into all
three toolchains, the capability seam with tests, hardened ESLint and
TypeScript configuration with boundary-violation fixtures, the dependency
policy document, and the updated development runbook.
