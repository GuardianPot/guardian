---
id: WCX-13
phase: 3
wave: capability
title: Incident detail, attacker journey, evidence explorer
status: draft
risk: high
components:
  - web-console
decision_refs:
  - WC-D20
  - WC-D24
  - WC-D18
  - WC-D31
  - UX-03
  - UX-04
  - UX-05
  - COR-04
  - COR-05
  - EV-01
  - EV-02
  - EV-04
  - SRC-01
  - SRC-07
  - DATA-02
  - DATA-03
  - DATA-04
  - DATA-05
  - AUTH-06
  - SEC-08
acceptance_refs:
  - AC-UX-002
  - AC-UX-003
  - AC-UX-004
  - AC-UX-005
  - Phase 3 exit gate "attacker journey readable" and "observed/inferred distinction exists"
depends_on:
  - WCX-12
  - P3-W14
  - P3-W15
allowed_paths:
  - "apps/web-console/src/**"
  - "tests/e2e/web-console/**"
  - "security/wcx-13-evidence-rendering-review.md"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-13.md"
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

# WCX-13 — Incident detail, attacker journey, evidence explorer

## 1. Purpose

Deliver the screen that carries the product's core promise: a readable
attacker journey with grounded evidence, an explicit separation of what was
observed from what was inferred, and an audit trail — all rendered from
attacker-influenced data that must never execute or deceive.

## 2. Why now

`P3-W14` and `P3-W15` produce incident detail, journey, and evidence data, and
`AC-UX-002` through `AC-UX-005` are Phase 3 exit criteria. This is also the
highest-risk rendering surface in the product: HTTP bodies, SSH transcripts,
filenames, and captured credential attempts all arrive here.

## 3. Inputs and decisions

- `WC-D20` — semantic HTML and CSS timeline, no charting library, every visual
  paired with an accessible text or table equivalent.
- `WC-D24` — the untrusted rendering contract from `WCX-06` governs every
  evidence surface.
- `WC-D18` — second precision, visible timezone, reachable UTC, and
  clock-quality marking.
- `WC-D31` — the organization-level audit viewer is delivered in Phase 3.
- `UX-03` — nine mandatory detail sections. `UX-04` — MITRE labels secondary.
  `UX-05` — incident-scoped evidence table with basic filters, no query
  language.
- `COR-04`, `COR-05` — timeline first with a simple relationship summary.
- `EV-01`, `EV-02`, `EV-04` — the full chain, minimum canonical fields, and
  provenance in the UI.

## 4. Dependencies

`WCX-12` for tables, filters, and pagination. **`P3-W14` and `P3-W15` must be
accepted first.**

## 5. Scope

1. Incident detail with all nine `UX-03` sections, sections 6 and 7 as
   explicit Phase 4 placeholders.
2. The attacker journey timeline and relationship summary.
3. The incident-scoped evidence explorer with the `UX-05` filters.
4. Evidence detail rendering for every captured content class.
5. The observed-versus-inferred presentation contract.
6. The incident audit and history section, and the organization-level audit
   viewer.

## 6. Non-goals

- No AI content generation, retrieval, or display beyond a pending or
  unavailable placeholder. `WCX-14` delivers it.
- No guidance content beyond a placeholder. `WCX-14` delivers it.
- No disposition workflow, suppression approval, or notification
  acknowledgement. `WCX-14` delivers them. Phase 3 status transitions exposed
  by `P3-W14` are displayed and operable; the `CS-07` benign-disposition
  workflow is not.
- No SIEM query language, saved searches, or cross-incident search.
- No evidence file download or blob viewing. Whether and how raw artefacts can
  be exported is a security decision that is not yet made; the console shows
  metadata and bounded inline content only.
- No graphical node-and-edge journey graph. `COR-05` approved timeline first
  and `UX-08` forbids a topology map.
- No incident merge or split UI, and no expected-source or suppression rule
  management. `WCX-17` delivers both against `P3-W8` and `P3-W11`, reusing the
  evidence rendering and audit surfaces built here.

## 7. Allowed paths

Only the paths in frontmatter. `ai/**` is forbidden.

## 8. Security constraints

1. Every evidence value is attacker-influenced and renders through the
   `WCX-06` untrusted components. This applies to HTTP request bodies and
   headers, SSH transcripts and commands, usernames and password attempts,
   filenames, user agents, banners, and any decoy-reported string.
2. No evidence surface may emit a link, an image, a media element, an iframe,
   an object, or any URL-bearing attribute from data. An observed URL is text.
3. No evidence artefact is downloadable in this package. No `download`
   attribute, no blob URL, no object URL, and no file-saving affordance may
   exist.
4. Captured credential material, including `DATA-02` password attempts, is
   displayed only where the approved retention decisions permit, is clearly
   labelled as attacker-supplied, and is never copied automatically, never
   placed in a URL, and never included in any screenshot taken by the test
   suite.
5. Bounded rendering is mandatory. Every payload is truncated at the `WCX-06`
   bounds with an explicit indicator; an unbounded transcript must never be
   injected into the DOM.
6. The observed-versus-inferred distinction is a security control, not a
   stylistic choice. An inferred attribute rendered as observed would cause a
   wrong operator action. Every field carries its provenance and a test
   asserts it.
7. Audit records are append-oriented truth. The console renders them read-only
   and offers no edit or delete affordance, consistent with `EV-05` and
   `AUTH-06`.
8. No incident, evidence, or audit content may be written to browser storage.
9. The evidence explorer's filter values are validated before use and are
   never interpolated into markup.

## 9. Implementation requirements

### 9.1 Incident detail sections

All nine `UX-03` sections, in this order, each independently linkable within
the page:

1. **What happened** — a plain-language summary composed from backend fields,
   never from a client-side narrative generator.
2. **Confidence and severity** — the two axes presented separately per
   `WCX-03`, each with the backend's stated reasons per `CS-03`.
3. **Source context** — provenance-aware per `SRC-07`: `Observed source` for
   the guaranteed field, and every other attribute labelled with how it was
   obtained, for example `supplied during authentication` or
   `probable, last observed`.
4. **Attacker journey** — the timeline of 9.2.
5. **Evidence** — the explorer of 9.3.
6. **AI explanation** — a placeholder stating that AI explanation arrives in a
   later phase. It must not imply that an explanation was attempted and
   failed, and must not occupy visual prominence over evidence, per `EV-04`.
7. **Recommended next actions** — a placeholder with the same rules.
8. **Status and disposition** — current status, the transitions `P3-W14`
   exposes, and the history of transitions. Disposition workflow is Phase 4.
9. **Audit and history** — the incident's audit records per 9.5.

Sections 6 and 7 render as `unknown`-class placeholders from the `WCX-04`
matrix, never as empty or as absence of risk.

### 9.2 Attacker journey timeline

1. An ordered semantic list rendered with CSS, no charting library.
2. Each step shows: time at second precision with visible timezone,
   observed source, decoy and zone, protocol, action, and a link to the
   underlying evidence.
3. Irregular time gaps are handled by showing the elapsed interval between
   consecutive steps as text. The visual axis is not proportionally scaled,
   because a proportional axis over irregular intervals would either compress
   bursts into illegibility or imply precision the data does not carry. The
   elapsed-interval text is the authoritative reading.
4. A burst that was deduplicated per `CS-09` renders as one step carrying its
   count and time range, expandable to the individual observations.
5. The timeline never auto-plays, animates, or reorders. `WC-D13` applies.
6. A relationship summary states, in text, how the steps were correlated:
   which dimensions matched per `COR-01`, and within which window per
   `COR-03`. Correlation is backend truth; the console never re-derives it.
7. Every visual element has an accessible text equivalent; the timeline is
   also available as a table view toggled by an explicit control, and both
   views carry identical content.

### 9.3 Evidence explorer

Per `UX-05`, an incident-scoped table with basic filters and no query
language. Filters: time range, protocol, decoy, action, observed source, and
evidence identifier. Built on the `WCX-12` table primitives with URL-carried
state, cursor pagination, and virtualisation above the threshold.

Columns include the `EV-02` minimum canonical fields. Selecting a row opens
evidence detail.

### 9.4 Evidence detail rendering

Per content class, all through the `WCX-06` untrusted components:

| Class | Rendering |
|---|---|
| Connection or banner probe | metadata table only |
| HTTP request | method, path, headers, and body as bounded `UntrustedBlock`; no link is emitted from any URL |
| SSH transcript and commands | bounded `UntrustedBlock` with ANSI escaped, not interpreted |
| Credential attempt | labelled as attacker-supplied, subject to the approved retention decision, never auto-copied |
| Filename or path | `UntrustedText`; never used as a path, a download name, or a request segment |
| Synthetic credential use | labelled explicitly as a honey credential, with the confidence contribution stated per `CS-03` |

Every evidence record shows its identifier, its canonical timestamps, and its
provenance, satisfying `EV-04`.

MITRE ATT&CK labels are secondary and collapsed by default per `UX-04`, with a
human-readable behaviour description shown first.

### 9.5 Audit surfaces

1. **Incident audit** — the incident's own records, read-only, with actor,
   time, object, and before-and-after references.
2. **Organization audit viewer** — a dedicated screen over
   `GET /v1/audit/events`, using the existing cursor pagination and the
   `WCX-12` table primitives, with filters the contract supports.
3. Both are read-only. No edit, delete, or export affordance exists.
4. Actor and object values that can carry operator-authored or
   attacker-influenced text render through the untrusted components.

### 9.6 UI/UX requirements

1. Evidence is visually primary; the AI and guidance placeholders are visually
   subordinate, per `EV-04`.
2. A generalist operator must be able to read the journey top to bottom and
   understand the sequence without opening evidence detail.
3. Every claim in the summary is traceable to a step or an evidence record;
   nothing is asserted without a path to its basis.
4. Long transcripts are collapsed by default with an explicit expand control
   stating the payload size.
5. At 320 pixels the timeline stacks and the evidence table collapses to
   stacked rows; no field is dropped.

### 9.7 Accessibility requirements

1. The timeline is an ordered list; the table view is a semantic table. Both
   are keyboard navigable and expose the same content.
2. Each timeline step has an accessible name identifying its time, source, and
   action.
3. Expand and collapse controls expose `aria-expanded` and `aria-controls`.
4. `UntrustedBlock` regions are keyboard-scrollable with an accessible name.
5. Section navigation within the page uses real headings in order, with no
   skipped level.
6. The observed-versus-inferred distinction is conveyed textually, never by
   colour or position alone.
7. Axe reports no serious or critical issue at both viewports.

### 9.8 API and data contracts

Consumed from `P3-W14` and `P3-W15`, plus the existing audit endpoint. No
contract is defined here. If a `UX-03` section, a `UX-05` filter, or an
`EV-02` canonical field is unavailable from the contract, stop and escalate.

### 9.9 Error and failure behaviour

1. A failed evidence read renders `degraded` inside the evidence section while
   the rest of the incident remains readable, using `partial` at the page
   level to name what could not be loaded.
2. A missing journey renders `unknown`, never an empty timeline, because an
   empty timeline would imply no attacker activity was correlated.
3. A denied incident renders `denied`, distinct from `not-found`.
4. An evidence record that cannot be rendered safely renders its metadata and
   an explicit statement that the payload could not be displayed. It is never
   skipped silently, because a silently dropped evidence record is missing
   evidence.
5. Degraded `clock_quality` on a source marks the affected timestamps per
   `WCX-08`.

### 9.10 Internationalisation and theme

All section headings, labels, provenance phrases, and placeholder copy enter
the `WCX-08` catalogue. Provenance wording is security-critical and reviewed
as such. Backend reason codes, condition types, MITRE identifiers, and all
evidence content are never translated. Severity and confidence use `WCX-03`
tokens; evidence escape markers use the neutral treatment from `WCX-06`.

### 9.11 Performance

1. Incident detail must be interactive within 2 seconds on the reference
   dataset per `PERF-07`.
2. A journey with several hundred steps and an evidence set at the `WCX-06`
   bound must stay within the `WCX-12` interaction budget.
3. Evidence payload transformation is memoised per record and never repeated
   on re-render.
4. Collapsed transcripts are not parsed or transformed until expanded.
5. The incident-detail feature shares the incident chunk from `WCX-12`.

### 9.12 Observability

None in the console. Evidence content is excluded from every diagnostic
surface, including the `WCX-15` report.

### 9.13 Documentation

Add an `Incident detail, journey, and evidence` section to
`docs/runbooks/web-console/development.md` covering the nine sections, the
timeline reading model, the evidence rendering table, the observed-versus-
inferred contract, and the no-download rule. Record
`security/wcx-13-evidence-rendering-review.md`.

## 10. Required tests

### 10.1 Unit and component

1. All nine `UX-03` sections render; sections 6 and 7 render as placeholders
   that do not imply absence of risk or a failed attempt.
2. Every field carries its provenance; an inferred attribute never renders
   with observed wording. Table-driven across every attribute the contract
   defines.
3. Confidence and severity remain separate and neither is computed
   client-side.
4. The timeline renders elapsed intervals as text and applies no proportional
   scaling.
5. A deduplicated burst renders as one step with count and range and expands
   to individual observations.
6. The table view of the timeline contains identical content to the list view.
7. Every evidence class renders through the untrusted components; the full
   `WCX-06` hostile corpus is exercised against every evidence surface.
8. No evidence surface emits an anchor, image, media element, iframe, object,
   or any URL-bearing attribute.
9. No download affordance, `download` attribute, blob URL, or object URL
   exists anywhere in the feature. Asserted by a source-level check and a
   rendered-DOM assertion.
10. Payloads truncate at the bounds with an explicit indicator.
11. A missing journey renders `unknown`, not an empty timeline.
12. An unrenderable evidence record shows metadata and an explicit statement,
    and is never skipped.
13. Audit surfaces expose no edit, delete, or export control.
14. Degraded clock quality marks the affected timestamps.
15. No incident, evidence, or audit content reaches browser storage.

### 10.2 Browser and E2E scenarios

1. Run the Phase 3 North-Star scenario end to end and confirm the journey
   reads correctly: SMB probe, HTTP admin discovery, SSH login attempts,
   synthetic credential use, SSH commands, and PostgreSQL authentication, in
   order, with correct sources and evidence links.
2. Confirm the observed-versus-inferred distinction is visible for every
   attributed field.
3. Seed hostile content into every evidence class through the real pipeline
   and confirm no execution, no navigation, no network request, and no console
   error in all three browsers.
4. Confirm a right-to-left override in a captured filename does not reorder
   the displayed value.
5. Filter, paginate, and deep-link the evidence explorer.
6. Open the organization audit viewer, paginate through the cursor, and
   confirm read-only presentation.
7. Force an evidence-read failure and confirm the incident remains readable
   with `partial` naming the failure.
8. Narrow-viewport rendering at 375 and 320 pixels.
9. Axe scan of detail, timeline, table view, evidence detail, and audit
   viewer.

Screenshots are taken only where no captured credential material is on screen.
Traces and video remain disabled for any step that renders credential content.

## 11. Acceptance criteria and Definition of Done

1. All nine `UX-03` sections exist, satisfying `AC-UX-002` through
   `AC-UX-005`.
2. The journey is readable without opening evidence, and its correlation basis
   is stated in text.
3. Observed and inferred are distinguishable for every attributed field.
4. Every evidence class renders inertly against the full hostile corpus in all
   three browsers.
5. No download or blob affordance exists anywhere.
6. The evidence explorer satisfies `UX-05` with no query language.
7. Incident audit and the organization audit viewer are read-only and
   paginated.
8. `task web:check` and `task web:e2e` pass within the `WCX-12` budgets.
9. The evidence rendering security review is recorded.

## 12. Evidence required

- North-Star scenario browser evidence in all three engines with the journey
  rendered.
- Hostile-corpus rendering report per evidence class.
- Right-to-left override screenshot showing the filename unreordered.
- Source-level and DOM-level proof that no download affordance exists.
- Provenance table showing every attributed field and its rendered wording.
- Partial-failure evidence.
- Axe reports and viewport screenshots.
- `security/wcx-13-evidence-rendering-review.md`.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- the contract cannot distinguish an observed attribute from an inferred one;
- a `UX-03` section, a `UX-05` filter, or an `EV-02` field is unavailable;
- an evidence class cannot be rendered safely within the `WCX-06` contract and
  would need an exception;
- operators appear to require raw artefact download, which is an unmade
  security decision;
- a proportional time axis appears necessary for the journey to be readable,
  which reopens `WC-D20`;
- retention decisions make it unclear whether a captured credential may be
  displayed at all.

## 14. Deliverables

Incident detail with all nine sections, the attacker journey timeline with its
table equivalent and correlation summary, the incident-scoped evidence
explorer with `UX-05` filters, safe evidence rendering for every captured
content class, the observed-versus-inferred contract, incident audit and the
organization audit viewer, the browser scenarios including the North-Star
journey and the hostile corpus, the security review, and the runbook section.
