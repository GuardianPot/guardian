---
id: WCX-12
phase: 3
wave: capability
title: Incident-first dashboard, tables, pagination, virtualisation
status: draft
risk: high
components:
  - web-console
decision_refs:
  - WC-D19
  - WC-D30
  - WC-D14
  - WC-D11
  - WC-D15
  - UX-01
  - UX-02
  - UX-07
  - CS-01
  - CS-02
  - CS-09
  - SRC-01
  - SRC-07
  - PERF-05
  - PERF-07
acceptance_refs:
  - AC-UX-001
  - Phase 3 exit gate "incident-first UX passes"
  - PERF-05 incident visibility 5 s p95 excluding AI
depends_on:
  - WCX-07
  - WCX-10
  - P3-W13
integration_dependencies:
  - P3-W12
allowed_paths:
  - "apps/web-console/src/**"
  - "apps/web-console/package.json"
  - "apps/web-console/test/**"
  - "package-lock.json"
  - "tests/e2e/web-console/**"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-12.md"
forbidden_paths:
  - "apps/control-plane/**"
  - "apps/edge-agent/**"
  - "openapi/**"
  - "proto/**"
  - "ai/**"
  - "0-planning-documents/**"
requires_owner_review: true
requires_security_review: true
---

# WCX-12 — Incident-first dashboard, tables, pagination, virtualisation

## 1. Purpose

Replace the `WCX-10` home placeholder with the approved incident-first
dashboard, and build the table, filtering, pagination, and virtualisation
foundation that the incident list and later evidence views require.

## 2. Why now

`UX-01` makes this the product's primary screen and `AC-UX-001` is a Phase 3
exit criterion. It is also the first screen where the console must satisfy a
measured latency target, `PERF-05`, and the first with enough rows to need a
real table implementation.

## 3. Inputs and decisions

- `WC-D19` — headless TanStack Table and TanStack Virtual, cursor pagination,
  filter state in URL query parameters, virtualisation above 200 rendered
  rows.
- `WC-D30` — the runtime interaction budget is measured here for the first
  time.
- `WC-D11` — severity uses an ordered ramp with a non-colour channel;
  confidence is never colour-coded.
- `WC-D14` — this screen is the application root.
- `UX-02` — the approved minimum column set.
- `UX-07` — health and coverage remain a secondary panel.
- `CS-09` — deduplicated bursts preserve count and time range.
- `SRC-01`, `SRC-07` — observed source is guaranteed; wording stays
  provenance-aware.

## 4. Dependencies

`WCX-07` for freshness and budgets, `WCX-10` for the root route and scope.
**`P3-W13` and the Phase 3 incident backend must be accepted first.** `P3-W12`
deduplication is an integration dependency because count and time-range
rendering depend on it.

## 5. Scope

1. Add the headless table and virtualisation libraries and build the shared
   `DataTable` primitives on top of them.
2. Implement filtering, sorting, and cursor pagination with URL-carried state.
3. Build the incident list satisfying `UX-02`.
4. Build the dashboard with incidents primary and health secondary.
5. Implement the severity and confidence presentation contract from `WCX-03`.
6. Add the runtime interaction budget measurement.

## 6. Non-goals

- No incident detail, journey, or evidence explorer. Those are `WCX-13`.
- No AI content, notification centre, or disposition workflow. Those are
  `WCX-14`.
- No SIEM-style query language. `UX-05` approved a scoped evidence table with
  basic filters only.
- No incident merge or split UI. `WCX-17` delivers it against `P3-W8`.
- No expected-source or suppression management. `WCX-17` delivers it against
  `P3-W11`.
- No topology map or graphical relationship view.
- No client-side correlation, scoring, severity computation, or
  deduplication. The console displays the backend's values and never derives
  them.

## 7. Allowed paths

Only the paths in frontmatter. `ai/**` is forbidden; the dashboard shows no AI
content in this package.

## 8. Security constraints

1. Every incident field that originates from attacker behaviour — title
   fragments, observed source values, decoy names, behaviour summaries — is
   untrusted and renders through the `WCX-06` components.
2. Severity, confidence, status, and counts are backend values. The console
   must never compute, adjust, round, or infer them. A test asserts that a
   rendered severity equals the backend value for every level.
3. Confidence must never be presented as certainty. `SRC-07` wording applies:
   the source column is labelled `Observed source`, and any inferred attribute
   is labelled as inferred.
4. An empty incident list is a strong claim. It may be rendered only on a
   confirmed successful empty response, and must state the scope and time
   window it covers. A denied, degraded, stale, or failed read must never
   render as an empty list, because that would tell an operator there are no
   detections when the truth is that the console does not know.
5. A stale list must show its observation age. Silently showing an old list
   during a backend outage would understate active detections.
6. Filter values entered by the operator are placed in the URL. They must be
   validated before use, must never be interpolated into markup, and must not
   include anything drawn from attacker-supplied content without going through
   the untrusted components when displayed back.
7. No incident content may be written to browser storage, including filter
   presets. URL state is the only persistence.

## 9. Implementation requirements

### 9.1 Table primitives

Built on headless libraries; the visual layer stays in the `WCX-03` and
`WCX-04` system, so this is not a second design system.

1. `DataTable` supports column definitions, sorting, row actions, an empty
   state drawn from the `WCX-04` matrix, and a row-level pending treatment.
2. Virtualisation activates above 200 rendered rows and is off below it. Both
   paths are tested, because the accessible structure must be identical.
3. Column visibility is configurable per table and its state is carried in the
   URL.
4. Every table is a semantic table. Virtualisation must not break row and
   column semantics; if the chosen approach cannot preserve them, stop and
   escalate rather than shipping a non-semantic grid.

### 9.2 Filtering, sorting, pagination

1. Filter, sort, and column state live in URL query parameters so a triage
   view is shareable and survives reload and browser navigation.
2. Parameters are validated on read; an invalid parameter is ignored and the
   applied filter set is shown explicitly so the operator always knows what is
   filtered.
3. Pagination uses the backend cursor convention through `useInfiniteQuery`,
   per `WCX-02`. Page size is an explicit constant.
4. Sorting is server-side where the contract supports it. Client-side sorting
   across a paginated set is forbidden, because sorting one page and calling
   it the ranking would be misleading.
5. A filtered empty result states which filters produced it and offers to
   clear them, so it is never confused with an unfiltered empty result.

### 9.3 Incident list columns

Satisfying `UX-02` exactly: title, status, severity, confidence, observed
source, first seen, last seen, affected decoy count, primary behaviour or
category, and notification state.

Rules:

1. Severity uses the ordered ramp plus shape and text from `WCX-03`.
2. Confidence uses the non-colour `ConfidenceMeter` and is visually distinct
   from severity, so the two axes cannot be read as one.
3. First seen and last seen use the `WCX-08` `Timestamp` at `second`
   precision, with the timezone visible and the UTC value reachable.
4. For a deduplicated incident, the occurrence count and the time range are
   shown together per `CS-09`; a count is never shown without its range.
5. Observed source uses provenance-aware wording and renders through the
   untrusted components.
6. Notification state reflects `P4-*` delivery once it exists; until then it
   renders `unknown` rather than implying that nothing was sent.
7. The default ordering and the default time window are explicit and visible.

### 9.4 Dashboard composition

1. Open incidents are the primary region and occupy the top of the screen.
2. Health and coverage form a clearly secondary region, per `UX-07`, reusing
   the existing health projection presentation.
3. The dashboard shows no aggregate that could be mistaken for a safety
   verdict. A count of zero open incidents is rendered with its scope and
   window and with an explicit statement that it describes detections, not
   safety.
4. The dashboard uses the `critical` freshness class from `WCX-07`.

### 9.5 UI/UX requirements

1. A generalist operator must be able to answer "what needs attention now"
   without configuring anything.
2. Row density is comfortable by default; a compact option is permitted but
   the default must not compromise the target-size guidance in `WCX-05`.
3. New rows arriving on refresh must not reorder or shift content under the
   operator's pointer or focus; new items are indicated and applied on an
   explicit affordance if the operator is actively scrolled into the list.
4. At 320 pixels the table collapses to stacked rows preserving all `UX-02`
   fields; no field is dropped at any width.

### 9.6 Accessibility requirements

1. Semantic table structure with a caption, header cells, and correct scope,
   in both virtualised and non-virtualised paths.
2. Sortable headers expose `aria-sort` and are keyboard operable.
3. Filter controls are labelled; the applied filter set is announced once on
   change.
4. Loading additional pages announces once on completion, not per row.
5. Severity and confidence are never conveyed by colour alone.
6. Virtualised content exposes the total row count so a screen-reader user
   knows the list size.
7. Axe reports no serious or critical issue at both viewports.

### 9.7 API and data contracts

Consumed from the Phase 3 incident contract delivered by `P3-W13`. This
package defines no contract. Cursor pagination follows the existing
`next_cursor` convention already used by audit events. If a `UX-02` column is
absent from the contract, stop and escalate.

### 9.8 Error and failure behaviour

1. Denied renders `denied`, never an empty list.
2. Degraded or failed reads without cached data render `degraded`, never an
   empty list.
3. Reads with cached data render `stale` with the observation age.
4. A failed page load leaves earlier pages visible and offers retry, and never
   silently truncates the list.
5. A partially failed dashboard renders `partial`, naming what could not be
   loaded.

### 9.9 Internationalisation and theme

All labels, filter names, and empty-state copy enter the `WCX-08` catalogue.
The zero-incident wording is security-critical and is reviewed as such.
Severity tokens come from `WCX-03`; no new colour is introduced.

### 9.10 Performance

1. `PERF-05` — the time from a decoy interaction to its visibility in this
   list must be within 5 seconds at the ninety-fifth percentile on the
   reference environment, excluding AI. The `critical` freshness class governs
   this; the measurement is recorded and compared against the `WCX-07`
   trigger.
2. `PERF-07` — the list must be interactive within 2 seconds on the reference
   dataset over a normal broadband path.
3. The runtime interaction budget from `WC-D30` is established here: with 200
   rendered rows, sorting, filtering, and scrolling must stay within a
   recorded interaction-latency ceiling, measured in CI and versioned.
4. The incident feature is its own chunk, and the initial authenticated load
   budget from `WCX-07` still holds.
5. Background refresh must not re-render the entire table; row identity is
   stable by incident identifier.

### 9.11 Observability

None in the console.

### 9.12 Documentation

Add an `Incident dashboard and tables` section to
`docs/runbooks/web-console/development.md` covering the table primitives, the
URL state contract, the virtualisation threshold, the column set, and the
zero-incident wording rule.

## 10. Required tests

### 10.1 Unit and component

1. Every `UX-02` column renders, with severity and confidence visually and
   programmatically distinct.
2. Rendered severity, confidence, status, and counts equal the backend values
   for every level; nothing is computed client-side.
3. A deduplicated incident shows count and time range together; a count never
   appears alone.
4. Denied, degraded, stale, and failed reads never render as an empty list.
   Asserted with explicit negative assertions on the empty-state text.
5. A confirmed empty result states its scope and window and does not imply
   safety.
6. A filtered empty result names the filters and offers to clear them.
7. URL state round-trips: applying filters, sorting, and column visibility,
   then reloading, reproduces the same view.
8. An invalid URL parameter is ignored and the applied filter set is shown.
9. Client-side sorting across pages is impossible; a fixture proves the
   guard.
10. Virtualised and non-virtualised paths produce identical accessible
    structure and identical row content.
11. Hostile incident titles, source values, decoy names, and behaviour
    summaries render inert.
12. Notification state renders `unknown` before Phase 4 rather than implying
    no notification was sent.
13. Background refresh preserves scroll position and focus and does not
    reorder rows under the operator.
14. No incident content reaches browser storage.

### 10.2 Browser and E2E scenarios

1. Run the Phase 3 North-Star deterministic scenario and confirm the resulting
   incident appears in the list with correct severity, confidence, source,
   decoy count, and time range.
2. Measure interaction-to-visibility latency across repeated runs and record
   the ninety-fifth percentile against `PERF-05`.
3. Filter, sort, paginate, copy the URL, and reopen it in a second context to
   confirm the identical view.
4. Force an authorisation denial and confirm `denied`, not empty.
5. Force a backend outage and confirm `stale` with age, then `degraded`.
6. Narrow-viewport rendering at 375 and 320 pixels with all fields present.
7. Axe scan at both viewports, virtualised and non-virtualised.

### 10.3 Performance

CI records initial load, time to interactive on the reference dataset, and the
interaction-latency measurements for sort, filter, and scroll at 200 rows.
Results are versioned; a regression above twenty percent fails and requires
owner review per `PERF-08`.

## 11. Acceptance criteria and Definition of Done

1. The dashboard is the application root, incidents primary and health
   secondary, satisfying `AC-UX-001`.
2. Every `UX-02` column is present and truthful; nothing is computed
   client-side.
3. No failure, denial, staleness, or degradation renders as an empty list.
4. Filter, sort, and pagination state is shareable through the URL and
   nothing is stored in the browser.
5. Virtualisation activates above 200 rows without changing semantics.
6. `PERF-05` and `PERF-07` measurements are recorded and met, or the `WCX-07`
   transport trigger is raised with evidence.
7. The runtime interaction budget is established and enforced.
8. `task web:check` and `task web:e2e` pass within all budgets.

## 12. Evidence required

- North-Star scenario browser evidence in all three engines.
- Latency measurements with the ninety-fifth percentile against `PERF-05`.
- Time-to-interactive measurement against `PERF-07`.
- Interaction-latency baseline at 200 rows.
- Negative-assertion test output proving no failure renders as empty.
- URL state round-trip evidence.
- Axe reports, screenshots at four viewport widths.
- Dependency admission records for the table and virtualisation libraries.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- the Phase 3 contract lacks a `UX-02` column;
- virtualisation cannot preserve semantic table structure;
- `PERF-05` cannot be met with the `critical` polling class, which raises the
  recorded `WC-D05` transport change proposal;
- server-side sorting is unavailable, since client-side sorting across pages
  is forbidden;
- an empty-list rendering cannot be proven distinct from every failure mode;
- the dashboard would need to display an aggregate that reads as a safety
  verdict.

## 14. Deliverables

Shared table primitives with virtualisation, URL-carried filter, sort, and
column state, cursor pagination, the incident list satisfying `UX-02`, the
incident-first dashboard with secondary health, the severity and confidence
presentation, the performance measurements and enforced interaction budget,
the browser scenarios, and the runbook section.
