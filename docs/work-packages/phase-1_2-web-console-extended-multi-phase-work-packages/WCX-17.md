---
id: WCX-17
phase: 3
wave: capability
title: Expected-source management and incident merge and split
status: draft
risk: high
components:
  - web-console
decision_refs:
  - CS-05
  - CS-06
  - CS-07
  - CS-08
  - CS-09
  - COR-06
  - COR-07
  - SRC-01
  - SRC-07
  - AUTH-06
  - WC-D16
  - WC-D24
acceptance_refs:
  - AC-CF-003
  - AC-CF-004
  - AC-INC-004
  - Phase 3 exit gate "known scanner suppression passes" and "merge/split passes"
depends_on:
  - WCX-13
  - P3-W8
  - P3-W11
allowed_paths:
  - "apps/web-console/src/**"
  - "tests/e2e/web-console/**"
  - "security/wcx-17-suppression-review.md"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-17.md"
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

# WCX-17 — Expected-source management and incident merge and split

## 1. Purpose

Deliver the two Phase 3 operator corrections that no other package covers:
managing expected sources and scoped suppression, and manually merging or
splitting incidents when correlation was wrong.

## 2. Why now

`P3-W11` and `P3-W8` are Phase 3 backend packages whose entire value is
operator-facing, and both are named in the Phase 3 exit gate. `CS-05` makes
expected-source definition core, and `COR-06` makes basic merge and split core
because wrong correlation is a trust problem: an operator who cannot correct a
mis-grouped incident stops believing the grouping. `WCX-12` and `WCX-13`
deliberately excluded both, leaving `AC-CF-003`, `AC-CF-004`, and `AC-INC-004`
without a console home.

## 3. Inputs and decisions

- `CS-05` — the operator may define a source IP or CIDR as a known scanner or
  expected automation.
- `CS-06` — suppression retains event and evidence; it downgrades or
  suppresses notification according to a scoped rule. Silent deletion is
  forbidden.
- `CS-08` — no autonomous learning. The system may propose a scoped
  suppression; the operator approves it.
- `COR-06` — basic manual merge and split is core; advanced bulk tooling is
  not.
- `COR-07` — historical reprocessing has no UI.
- `AUTH-06` — both operations are audited server-side.
- `SRC-07` — provenance-aware wording throughout.

## 4. Dependencies

`WCX-13` for incident detail, evidence rendering, and the audit surface.
**`P3-W8` and `P3-W11` must be accepted first.**

## 5. Scope

1. Expected-source entries: list, create, edit, and remove IP or CIDR entries.
2. Scoped suppression policy management and its notification eligibility flag.
3. The effect surface showing what a rule currently affects.
4. Incident merge with evidence preservation.
5. Incident split by evidence subset or source sequence.
6. Correction history and audit presentation for both.

## 6. Non-goals

- No autonomous or automatic suppression. `CS-08` requires explicit operator
  approval for every rule.
- No evidence deletion, editing, or purging from any surface. `CS-06` and
  `DATA-06` forbid it.
- No historical reprocessing UI. `COR-07` decided against it.
- No bulk merge, bulk split, or batch correction tooling. `COR-06` limits this
  to basic capability.
- No detection-rule editor. `DET-04` places custom rules outside the MVP.
- No confidence or severity override. Those remain engine outputs; the
  operator influences them only through expected-source context.
- No AI-proposed suppression. Suggestions originating from Phase 4 are handled
  in `WCX-14`; this package covers the Phase 3 rule surface they approve into.

## 7. Allowed paths

Only the paths in frontmatter.

## 8. Security constraints

1. **Suppression never deletes.** Every surface that creates, edits, or
   applies a rule must state that events and evidence are retained and that
   only notification eligibility changes. Wording implying removal is a
   defect, because an operator who believes suppression deletes data will use
   it to hide findings.
2. A suppression rule reduces what an operator is told. Its scope must be
   stated exactly before approval: which source range, which decoys or zones,
   which behaviours, and for how long or under what condition, as the backend
   defines them. A rule whose scope cannot be stated precisely must not be
   approvable.
3. Expected-source entries are operator-authored but describe attacker-facing
   address space. Their display names and notes are untrusted content and
   render through the `WCX-06` components.
4. An expected-source entry must never be presented as making a source safe.
   Per `CS-03` it reduces suspicion confidence; the console states that effect
   and never implies the source is trusted or excluded from evidence.
5. Merge and split alter the operator's picture of an attack. Both are
   level-2 confirmations naming the affected incidents and stating that
   evidence identifiers are preserved.
6. Split must never orphan evidence. The console must show, before
   confirmation, exactly which evidence moves and which remains, and must
   refuse to submit a split whose preview does not account for every selected
   record.
7. Neither operation may be optimistic. Displayed grouping changes only after
   the backend confirms.
8. No rule, entry, or correction content reaches browser storage.

## 9. Implementation requirements

### 9.1 Expected-source management

A dedicated screen listing entries with: source IP or CIDR, classification as
known scanner or expected automation, scope, the notification eligibility
flag, who created it, when, and its current effect.

1. Create and edit use the `WCX-11` form stack with backend-derived
   validation; CIDR validation is never hand-copied from the contract.
2. Overlapping or conflicting entries fail with the backend reason.
3. Removing an entry is a level-2 confirmation stating that historical
   evidence and incidents are unaffected.
4. Each entry shows its effect in operator terms: what confidence contribution
   it reduces and what notification behaviour it changes, drawn from backend
   fields and never inferred by the console.

### 9.2 Scoped suppression policy

1. A rule is always attached to a stated scope. The console renders the scope
   the backend defines and offers no way to create an unscoped rule.
2. The approval dialog states scope, retention of evidence, notification
   downgrade rather than deletion, and duration or condition.
3. Rules are listed with their current state, their scope, and their last
   effect, and can be revoked. Revocation is a level-1 action because it
   restores notification rather than removing it.
4. Where Phase 4 later proposes a rule, `WCX-14` routes the approval into this
   same surface so there is exactly one rule model in the product.

### 9.3 Effect surface

For each entry and rule, show what it currently affects: the count of
incidents whose notification eligibility it changed and a link to them. This
exists so a suppression cannot quietly hide a growing volume of activity. The
count is backend-supplied and renders `unknown` when unavailable, never zero.

### 9.4 Incident merge

1. Initiated from incident detail. The operator selects one or more target
   incidents.
2. A preview states which incidents combine, which identifier survives, and
   that every evidence identifier is preserved.
3. A level-2 confirmation names the incidents.
4. After success the surviving incident shows a correction entry in its
   history; the merged identifiers remain resolvable so an existing link or
   notification reference does not become a dead end.

### 9.5 Incident split

1. Initiated from incident detail by selecting an evidence subset or a source
   sequence, per `P3-W8`.
2. A preview lists exactly which evidence records move to the new incident and
   which remain, with a count for each and an explicit total reconciliation.
3. The console refuses to submit when the preview does not account for every
   selected record.
4. A level-2 confirmation names both resulting incidents.
5. After success both incidents show a correction entry in their history.

### 9.6 Correction history

Merge, split, expected-source changes, and suppression approvals appear in the
incident audit section from `WCX-13` and in the organization audit viewer,
read-only, with actor, time, object, and before-and-after references per
`AUTH-06`.

### 9.7 UI/UX requirements

1. Correction actions are discoverable from incident detail but are not
   adjacent to disposition controls, so a correction is not confused with a
   disposition.
2. Every preview is reviewable before confirmation; no correction is a single
   click.
3. The suppression surface leads with what is retained, not with what is
   suppressed.
4. At 320 pixels previews and lists stack without dropping counts.

### 9.8 Accessibility requirements

1. Entry, rule, and preview lists are semantic tables with captions and
   row-scoped action names.
2. Evidence selection for split is keyboard operable with a stated selection
   count announced on change.
3. Confirmation dialogs meet the `WCX-04` contract with scope in the
   accessible description.
4. Completion announces once.
5. Axe reports no serious or critical issue at both viewports.

### 9.9 API and data contracts

Consumed from `P3-W8` and `P3-W11`. This package defines no contract. If the
contract cannot express a rule's exact scope, or cannot supply a split preview
that reconciles every evidence record, stop and escalate.

### 9.10 Error and failure behaviour

1. A rejected entry or rule leaves the list unchanged with the backend reason.
2. A failed merge or split leaves both incidents unchanged and states that
   nothing changed.
3. A conflict, for example an incident modified concurrently, renders
   `conflict` and offers reload; no silent overwrite.
4. An unavailable effect count renders `unknown`, never zero.
5. A partially applied correction is impossible by contract; if the backend
   reports one, the console renders `degraded` and states that the grouping
   may be inconsistent rather than showing a confident result.

### 9.11 Internationalisation and theme

All suppression, retention, and correction wording enters the `WCX-08`
catalogue and is reviewed as security-critical language, in particular every
statement about what suppression does and does not remove. Suppressed
incidents must not be styled with a lowered severity token; severity is
unchanged by suppression and only notification eligibility differs.

### 9.12 Performance

Entry and rule lists are small and need no virtualisation but use the `WCX-12`
table primitives. Split previews over a large evidence set use the same
virtualisation threshold. The feature shares the incident chunk.

### 9.13 Observability

None in the console.

### 9.14 Documentation

Add an `Expected sources, suppression, and corrections` section to
`docs/runbooks/web-console/development.md` covering the retention wording
rule, the scope requirement, the split reconciliation rule, and the
correction-versus-disposition distinction. Record
`security/wcx-17-suppression-review.md`.

## 10. Required tests

### 10.1 Unit and component

1. Every suppression surface states evidence retention and notification
   downgrade; a negative assertion proves no deletion wording appears.
2. A rule cannot be approved without a stated scope; an unscoped fixture is
   not submittable.
3. An expected-source entry never renders as making a source safe or trusted.
4. Removing an entry states that historical data is unaffected.
5. The effect count renders `unknown` when unavailable, never zero.
6. A split preview reconciles every selected evidence record; a fixture with
   an unaccounted record blocks submission.
7. Merge preview names the surviving identifier and states evidence
   preservation.
8. Merged incident identifiers remain resolvable after merge.
9. Neither merge nor split is optimistic; displayed grouping changes only
   after confirmation.
10. A failed correction leaves both incidents unchanged and says so.
11. A concurrent modification renders `conflict`, not `validation`.
12. Hostile entry names and notes render inert.
13. Suppressed incidents retain their severity token.
14. No rule, entry, or correction content reaches browser storage.

### 10.2 Browser and E2E scenarios

1. Define an expected-source entry covering a scanner address, replay scanner
   traffic, and confirm confidence and notification behaviour change while
   evidence remains, producing `AC-CF-003` and `AC-CF-004` evidence.
2. Confirm the suppressed activity is still retrievable as evidence.
3. Merge two incidents produced by one actor and confirm evidence identifiers
   are preserved and old identifiers still resolve, producing `AC-INC-004`
   evidence.
4. Split an incident containing two distinct source sequences and confirm the
   reconciliation and the resulting histories.
5. Attempt a split whose preview does not reconcile and confirm it is blocked.
6. Confirm every correction appears in the audit surfaces.
7. Narrow-viewport rendering at 375 and 320 pixels.
8. Axe scan of every new screen and dialog.

## 11. Acceptance criteria and Definition of Done

1. Expected-source entries can be managed and satisfy `AC-CF-003` and
   `AC-CF-004`.
2. Suppression always states retention and downgrade, always carries a scope,
   and never deletes.
3. An expected source is never presented as trusted or safe.
4. Merge and split work with previews, evidence preservation, and audit
   records, satisfying `AC-INC-004`.
5. Split never orphans evidence.
6. Neither correction is optimistic and failures state that nothing changed.
7. `task web:check` and `task web:e2e` pass within budget.
8. The security review is recorded.

## 12. Evidence required

- `AC-CF-003`, `AC-CF-004`, and `AC-INC-004` browser evidence in all three
  engines.
- Evidence that suppressed activity remains retrievable.
- Split reconciliation evidence, including the blocked non-reconciling case.
- Audit records for every correction with actor, time, object, and
  before-and-after references.
- Negative-assertion output proving no deletion wording exists.
- Axe reports and viewport screenshots.
- `security/wcx-17-suppression-review.md`.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- a suppression rule's scope cannot be stated exactly from the contract;
- the contract permits an unscoped or global suppression;
- a split preview cannot reconcile every evidence record;
- merged incident identifiers do not remain resolvable, which would break
  existing links and notification references;
- suppression alters severity or confidence rather than notification
  eligibility, which would contradict `CS-06`;
- an operator workflow appears to require evidence deletion.

## 14. Deliverables

Expected-source entry management, scoped suppression policy management with
retention-first wording and an effect surface, incident merge and split with
reconciling previews and evidence preservation, correction history in the
audit surfaces, the browser scenarios producing `AC-CF-003`, `AC-CF-004`, and
`AC-INC-004`, the security review, and the runbook section.
