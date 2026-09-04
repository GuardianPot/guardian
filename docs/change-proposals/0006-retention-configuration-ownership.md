# Change proposal 0006: retention configuration backend ownership

- Status: APPROVED
- Owner decision: `@sinanganiz` approved on 2026-09-04.
- Affected decision IDs: `DATA-01`, `DATA-06`, `EV-05`, `CP-04`
- Affected acceptance criteria: `DATA-01` configurable retention; `DATA-06`
  environment-level retention and purge
- Work package: `WCX-21`

## Problem and context

`DATA-01` approves data-class-specific retention with configurable values
across six classes and `DATA-06` approves an environment-level retention and
purge job. Both are MVP scope. Neither has a backend owner: no package in
Phase 1 through Phase 5 delivers retention configuration endpoints, and
`P5-W10` exercises retention only as a PostgreSQL storage benchmark.

`WCX-21` specifies the console surface but cannot start against a contract
that nobody owns.

## Options

1. **Option A — add a new Phase 2 backend package, `P2-W16`.** Clean
   separation of backend from console. But Phase 2 is APPROVED/FINAL with
   `P2-W1` through `P2-W15`, so adding a package expands an approved phase's
   scope and its exit gate.
2. **Option B — give `WCX-21` narrow backend allowed paths.** The package owns
   the retention domain, its endpoints, its migration, and the console
   surface as one reviewable outcome. This follows the precedent already
   approved for `P1-W11` under `W11-C8-A`, and reused in `WCX-06` and
   `WCX-09`, where a console package received a narrowly enumerated Control
   Plane surface. No approved phase gains a package.
3. **Option C — defer retention configuration past MVP.** Contradicts
   `DATA-01` and `DATA-06`, which are approved MVP scope.

## Recommendation

**Option B**, bounded by these constraints, which are part of the approval:

1. `WCX-21` gains exactly these additional allowed paths, and no others:
   `apps/control-plane/internal/retention/**` (new module),
   `apps/control-plane/internal/api/retention.go` and its test,
   `apps/control-plane/internal/app/app.go` for module wiring,
   `apps/control-plane/internal/storage/retention.go` and its test,
   `apps/control-plane/internal/storage/queries/retention.sql`,
   one new file under `apps/control-plane/internal/storage/migrations/`, and
   `openapi/guardian.yaml`.
2. No existing domain, handler, migration, or query may change behaviour. The
   package adds; it does not modify audit, auth, environment, device, health,
   or reconciliation code.
3. The purge job itself is scheduled work under the existing jobs module. It
   must not delete audit records in a way that breaks `EV-05` append-oriented
   provenance; if audit retention is reducible at all, the contract marks it
   distinctly so the console can apply the level-3 treatment `WCX-21` section
   8.4 requires.
4. The contract must expose, per class, the effective value, the default,
   permitted bounds, and whether the class is operator-modifiable, plus purge
   execution state and outcome. Without those fields `WCX-21` cannot render
   truthfully.
5. Exact default day counts remain deferred by `DATA-01`. This proposal does
   not set them. The backend carries a documented default per class that the
   owner confirms before MVP release; the console never defines one.
6. `DATA-06` stands: no endpoint may delete an individual evidence record,
   incident, or audit entry, and none is surfaced.

`WCX-21` moves from `draft` to `approved-for-implementation` on this approval,
and its `blocked_by_missing_backend_owner` flag is removed.

## Impact

- **Product scope:** none added. This assigns ownership for capability that
  `DATA-01` and `DATA-06` already approved.
- **Architecture:** one new Control Plane module following `CP-02` module
  boundaries and ADR 0002.
- **Contracts and data:** `openapi/guardian.yaml` gains a retention resource.
  One forward-only migration adds retention policy storage.
- **Security and trust boundary:** unchanged. Retention is destructive
  configuration, so `WCX-21`'s confirmation levels and the `EV-05` audit
  constraint govern it. The package adds no deletion endpoint for individual
  records.
- **Operations and release:** purge becomes an operator-visible scheduled job.

## Rollout and failure behavior

Development migrations are forward-only per repository policy. If retention
storage must change shape later, the recovery path is reset and reseed of
development data.

If the backend cannot supply per-class bounds or modifiability,
`WCX-21` section 13 already requires the implementer to stop and escalate
rather than render a value the console cannot qualify.

## Owner decision record

- Decision: APPROVED
- Decided by: `@sinanganiz`
- Date: 2026-09-04
- Rationale: retention configuration is approved MVP scope with no owner.
  Option B assigns it without expanding an approved phase, and reuses the
  narrow-backend-path pattern already approved for `P1-W11`. Keeping the
  endpoint and its only consumer in one package makes the destructive
  semantics reviewable as a whole.
