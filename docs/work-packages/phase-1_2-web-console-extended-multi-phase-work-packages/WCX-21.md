---
id: WCX-21
phase: 2
wave: capability
title: Retention configuration and purge visibility
status: approved-for-implementation
risk: high
components:
  - web-console
  - control-plane
decision_refs:
  - DATA-01
  - DATA-06
  - DATA-03
  - DATA-04
  - DATA-05
  - CS-06
  - EV-05
  - AUTH-06
  - OPS-03
  - WC-D15
  - WC-D16
acceptance_refs:
  - DATA-01 data-class-specific configurable retention
  - DATA-06 environment-level retention and purge
depends_on:
  - WCX-11
  - WCX-08
change_proposal_refs:
  - CP-0006
allowed_paths:
  - "apps/web-console/src/**"
  - "tests/e2e/web-console/**"
  - "security/wcx-21-retention-review.md"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-21.md"
  - "apps/control-plane/internal/retention/**"
  - "apps/control-plane/internal/api/retention.go"
  - "apps/control-plane/internal/api/retention_test.go"
  - "apps/control-plane/internal/app/app.go"
  - "apps/control-plane/internal/storage/retention.go"
  - "apps/control-plane/internal/storage/retention_test.go"
  - "apps/control-plane/internal/storage/queries/retention.sql"
  - "apps/control-plane/internal/storage/migrations/**"
  - "openapi/guardian.yaml"
forbidden_paths:
  - "apps/edge-agent/**"
  - "proto/**"
  - "0-planning-documents/**"
requires_owner_review: true
requires_security_review: true
---

# WCX-21 — Retention configuration and purge visibility

## 1. Purpose

Give the operator a surface to see and configure data-class retention and to
observe purge activity, so that `DATA-01`'s configurable values and
`DATA-06`'s environment-level purge are reachable from the product rather than
only from deployment configuration.

## 2. Why now

The Product Owner decided on 2026-09-04 that retention values are
operator-configurable and that the surface is delivered now rather than
deferred. The reason this matters early is that retention governs data the
product begins collecting in Phase 2: normalized events and evidence, raw
transcripts, and quarantined hostile files all accumulate from the first
deployed decoy. Deferring the surface to a later phase means collecting data
for two phases under a retention policy the operator cannot see or change.

## 3. Inputs and decisions

- `DATA-01` — data-class-specific default policy with **configurable values**
  across six classes: normalized events and evidence, incidents, AI outputs,
  raw transcripts, hostile files, and audit logs. Exact default day counts are
  explicitly deferred by `DATA-01` to a final MVP specification owner
  decision.
- `DATA-06` — environment-level retention and purge job. Individual evidence
  editing or deletion from the incident UI is **not allowed**.
- `CS-06` — suppression retains evidence; nothing is silently deleted.
- `EV-05` — central append-oriented audit and provenance with no silent edit.
- `OPS-03` — staleness indicators.
- `AUTH-06` — configuration changes are audited.

## 4. Dependencies and backend ownership

`WCX-11` for the form stack and `WCX-08` for wording and timestamps.

**This package owns its own backend**, assigned by change proposal
[`0006`](../../change-proposals/0006-retention-configuration-ownership.md),
approved 2026-09-04. No roadmap package owned retention configuration
endpoints; `P5-W10` exercises retention and purge only as a PostgreSQL storage
benchmark. Rather than expand an approved phase, `0006` gives this package a
narrowly enumerated Control Plane surface, following the pattern approved for
`P1-W11` under `W11-C8-A` and reused in `WCX-06` and `WCX-09`.

The backend scope is exactly:

- a `retention` module holding the policy domain and its bounds;
- endpoints reading the effective policy per data class, including default,
  permitted bounds, and whether the class is operator-modifiable;
- an endpoint writing an operator-chosen value within bounds;
- purge execution state and outcome, reported from the existing jobs module;
- one forward-only migration and its `sqlc` queries;
- the corresponding `openapi/guardian.yaml` resource.

No existing domain, handler, migration, or query may change behaviour. Audit,
auth, environment, device, health, and reconciliation code is untouched.

## 5. Scope

1. A retention surface listing all six `DATA-01` classes with effective value,
   default, permitted bounds, and last change.
2. Editing a class value where the contract permits it.
3. Purge visibility: last run, scope, outcome, and next scheduled run as the
   backend reports them.
4. Class availability handling for classes whose data does not exist yet.
5. Audit presentation of retention changes.

## 6. Non-goals

- No individual evidence, incident, or audit record deletion from any surface.
  `DATA-06` forbids it and `EV-05` requires append-oriented audit.
- No manual purge trigger. Purge is a backend job; the console shows its
  state. A console-initiated bulk deletion is not an approved capability.
- No retention default values invented by this package. `DATA-01` defers the
  exact day counts to an owner decision and CP-0006 constraint 5 keeps them
  deferred; the console reads defaults from the backend and hardcodes nothing.
- No legal, compliance, or regulatory guidance text. The console states what
  the configuration does, not whether it satisfies any external obligation.
- No per-incident or per-evidence retention override.
- No export or archive capability.

## 7. Allowed paths

Only the paths in frontmatter.

## 8. Security constraints

1. **Retention is a destructive configuration.** Lowering a value causes data
   to be deleted permanently on the next purge. Every reduction is a level-2
   confirmation that states the class, the old and new values, and that data
   older than the new value will be permanently removed and cannot be
   recovered. A reduction must never be applied without that statement.
2. Raising a value is level 1, because it destroys nothing.
3. The confirmation for a reduction must state, where the backend supplies it,
   how much data the change will make eligible for deletion. If the backend
   cannot supply that estimate, the confirmation says so explicitly rather
   than implying the impact is small.
4. **Audit log retention is special.** `EV-05` requires append-oriented audit
   and `AUTH-06` requires audit records that cannot be edited through the
   product API. If the contract exposes audit retention as reducible, the
   console renders the reduction with an explicit warning that it shortens the
   product's own accountability record, and the change is level 3 requiring
   step-up reauthentication. If the contract does not permit reducing it, the
   console shows the value as fixed and offers no control.
5. No surface anywhere may offer deletion of an individual evidence record,
   incident, or audit entry. A source-level check asserts no such request is
   constructible.
6. The console never computes, estimates, or predicts a retention outcome. All
   values, bounds, estimates, and purge results are backend truth.
7. A class whose current value cannot be read renders `unknown` and offers no
   edit control. It must never render a default as if it were the effective
   value, because an operator would then believe retention is configured when
   the console does not know.
8. Retention changes are audited server-side and appear in the organization
   audit viewer from `WCX-13`.

## 9. Implementation requirements

### 9.1 Retention surface

A table of all six `DATA-01` classes, each showing: class name in operator
terms, effective retention value, backend-supplied default, permitted bounds,
whether it is operator-modifiable, when it last changed, and who changed it.

Rules:

1. All six classes are always listed. A class whose data does not exist in the
   current phase is listed with an explicit not-yet-collected state rather
   than hidden, so the operator sees the complete policy surface.
2. Effective value, default, and bounds all come from the backend. No value is
   hardcoded in the console.
3. A non-modifiable class shows its value with the reason it is fixed.
4. Values render through the `WCX-08` duration formatting so a day count is
   never ambiguous.

### 9.2 Editing

1. Editing uses the `WCX-11` form stack with bounds derived from the contract,
   never hand-copied.
2. A value outside the permitted bounds fails with the backend reason.
3. Reduction follows 8.1 through 8.4; increase is level 1.
4. Submission is pessimistic. The displayed value changes only after the
   backend confirms.
5. A rejected change leaves the previous value displayed and states that
   nothing changed.

### 9.3 Purge visibility

1. Last purge run: start time, scope, per-class counts if the backend supplies
   them, and outcome.
2. Next scheduled run where the backend reports one.
3. A failed purge renders `degraded` with the backend reason. A failed purge
   means data is being retained beyond policy, which the surface states
   plainly rather than treating as a minor operational event.
4. Purge state uses the `configuration` freshness class from `WCX-07`.

### 9.4 UI/UX requirements

1. The surface leads with what is retained and for how long, not with the
   controls, so the operator reads the current policy before changing it.
2. Reduction confirmations are unambiguous about permanence.
3. The relationship to suppression is stated once: suppression changes
   notification, retention changes storage, and neither hides evidence from
   the operator while it is retained.
4. At 320 pixels the class table stacks without dropping bounds or last-change
   information.

### 9.5 Accessibility requirements

1. The class table is semantic with a caption, header cells, and row-scoped
   action names.
2. Confirmation dialogs meet the `WCX-04` contract, with the permanence
   statement in the accessible description.
3. Bounds and validation errors are associated with their control.
4. A completed change announces once.
5. Axe reports no serious or critical issue at both viewports.

### 9.6 API and data contracts

Defined by this package under CP-0006 and added to `openapi/guardian.yaml`. If the assigned contract cannot express
per-class bounds, modifiability, or purge outcome, stop and escalate.

### 9.7 Error and failure behaviour

1. An unreadable class value renders `unknown` with no edit control.
2. A rejected change leaves the previous value and states that nothing
   changed.
3. An unavailable purge history renders `unknown`, never an empty history,
   because an empty history would imply purge has never needed to run.
4. A failed purge renders `degraded` and states that data is retained beyond
   policy.
5. No change is optimistic.

### 9.8 Internationalisation and theme

All class names, permanence statements, bounds wording, and purge wording
enter the `WCX-08` catalogue and are reviewed as security-critical language.
Retention state uses the health palette for degraded purge, never a severity
token; retention is an operational condition, not an incident severity.

### 9.9 Performance

The surface is small and needs no virtualisation. It shares the account and
settings chunk with `WCX-09`.

### 9.10 Observability

None in the console. Retention values and purge counts are excluded from the
`WCX-15` diagnostic report.

### 9.11 Documentation

Add a `Retention and purge` section to
`docs/runbooks/web-console/development.md` covering the six classes, the
reduction confirmation rule, the audit-retention special case, and the rule
that the console hardcodes no default. Record
`security/wcx-21-retention-review.md`.

## 10. Required tests

### 10.1 Unit and component

1. All six `DATA-01` classes are always listed, including classes whose data
   does not yet exist, which render a not-yet-collected state.
2. No default, bound, or effective value is hardcoded; a fixture with
   different backend values renders them.
3. A reduction is level 2 and its confirmation states the class, both values,
   and permanence; an increase is level 1.
4. When the backend supplies no impact estimate, the confirmation says so
   rather than omitting the impact.
5. Audit-log retention reduction, where permitted, is level 3 with step-up and
   carries the accountability warning; where not permitted, no control exists.
6. No surface can construct a request that deletes an individual evidence
   record, incident, or audit entry.
7. An unreadable class value renders `unknown` with no edit control and never
   shows the default as effective.
8. A rejected change leaves the previous value and states nothing changed.
9. An unavailable purge history renders `unknown`, never empty.
10. A failed purge renders `degraded` and states data is retained beyond
    policy.
11. No change is optimistic.
12. No retention content reaches browser storage.

### 10.2 Browser and E2E scenarios

1. Read the effective policy for every class against the real backend.
2. Increase a value and confirm the level-1 path.
3. Reduce a value, confirm the permanence statement, apply it, and confirm the
   backend reflects the change and the audit record exists.
4. Attempt a value outside the permitted bounds and confirm truthful
   rejection.
5. Force a purge failure and confirm the degraded state states that data is
   retained beyond policy.
6. Confirm no deletion affordance exists on any incident, evidence, or audit
   surface.
7. Narrow-viewport rendering at 375 and 320 pixels.
8. Axe scan of the surface and its dialogs.

## 11. Acceptance criteria and Definition of Done

1. All six `DATA-01` classes are visible with backend-supplied effective
   values, defaults, and bounds, and nothing is hardcoded.
2. Configurable classes are editable within bounds; reductions carry the
   permanence confirmation.
3. Audit retention is handled per 8.4.
4. No individual record deletion is possible anywhere.
5. Purge state is visible and a failed purge is reported as data retained
   beyond policy.
6. Retention changes appear in the audit viewer.
7. `task web:check` and `task web:e2e` pass within budget.
8. The security review is recorded.

## 12. Evidence required

- Browser evidence of reading, increasing, and reducing a value in all three
  engines, with the permanence confirmation captured.
- Audit records for each retention change with actor, time, and before-and-
  after values.
- Purge failure evidence with the degraded wording.
- Source-level proof that no individual-record deletion request is
  constructible.
- Fixture evidence proving no default is hardcoded.
- Axe reports and viewport screenshots.
- `security/wcx-21-retention-review.md`.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- the retention contract cannot express per-class bounds or modifiability;
- purge outcome cannot be reported per class;
- the contract permits reducing audit retention without a distinguishable
  control, so the `EV-05` accountability concern cannot be surfaced;
- the contract exposes an individual-record deletion operation, which
  `DATA-06` forbids surfacing;
- the backend cannot supply an impact estimate and the Product Owner considers
  a reduction confirmation without one insufficient;
- the exact default day counts that `DATA-01` deferred are still undecided at
  implementation time, since the console displays but does not define them.

## 14. Deliverables

The six-class retention surface with backend-supplied values, defaults, and
bounds, editing with level-appropriate confirmations and the permanence
statement, the audit-retention special case, purge visibility including
truthful failure reporting, audit presentation of changes, the browser
scenarios, the security review, and the runbook section.
