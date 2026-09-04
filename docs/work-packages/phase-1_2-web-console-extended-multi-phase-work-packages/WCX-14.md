---
id: WCX-14
phase: 4
wave: capability
title: AI explanation, notification centre, disposition
status: approved-for-implementation
risk: high
components:
  - web-console
decision_refs:
  - WC-D17
  - WC-D24
  - WC-D15
  - WC-D05
  - AIM-01
  - AIM-08
  - AIM-09
  - AIM-13
  - RG-01
  - RG-02
  - RG-03
  - RG-05
  - NT-01
  - NT-05
  - NT-06
  - CS-06
  - CS-07
  - CS-08
  - EV-04
  - SEC-09
  - PERF-06
acceptance_refs:
  - AC-NT-001
  - AC-NT-004
  - AC-RG-001
  - AC-RG-002
  - AC-RG-003
  - Phase 4 exit gate items for explanation, guidance, and in-product notification
depends_on:
  - WCX-13
  - P4-W8
  - P4-W9
  - P4-W10
allowed_paths:
  - "apps/web-console/src/**"
  - "tests/e2e/web-console/**"
  - "security/wcx-14-ai-surface-review.md"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-14.md"
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

# WCX-14 — AI explanation, notification centre, disposition

## 1. Purpose

Add the operator workflow layer: AI explanation and curated guidance presented
without ever outranking evidence, an in-product notification centre, and the
disposition and suppression approval flows.

## 2. Why now

Phase 4 delivers the AI job pipeline, the guidance composer, and the
notification engine. Their operator surfaces are the last thing standing
between the Phase 3 incident record and a usable workflow for a generalist
operator without a SOC.

## 3. Inputs and decisions

- `AIM-01` — incident evidence summary and plain-language explanation are the
  MVP AI core. `AIM-08` — defined behaviour when AI is unavailable.
  `AIM-09` — evidence-grounding bar. `AIM-13` — AI latency never blocks an
  incident.
- `RG-01` — three guidance layers. `RG-02` and `RG-03` — no automatic
  containment and no one-click containment control. `RG-05` — unsafe or
  generic advice guardrail.
- `NT-01`, `NT-05`, `NT-06` — in-product notification, trigger set, and
  acknowledgement.
- `CS-06`, `CS-07`, `CS-08` — suppression retains evidence, benign disposition
  exists, and no autonomous learning; suppression suggestions require explicit
  operator approval.
- `EV-04` — evidence precedes inference. `SEC-09` — prompt injection is a
  release blocker.
- `WC-D17` — long-running work is shown on the object. `WC-D24` — AI output is
  untrusted content.

## 4. Dependencies

`WCX-13` for incident detail. **`P4-W8`, `P4-W9`, and `P4-W10` must be
accepted first.**

## 5. Scope

1. Replace the `WCX-13` AI placeholder with the explanation surface.
2. Replace the guidance placeholder with the three curated layers.
3. Build the notification centre with acknowledgement.
4. Build disposition, including benign closure.
5. Build the suppression-suggestion approval flow.
6. Add the AI-surface security review.

## 6. Non-goals

- No containment execution, no one-click containment, no action that reaches
  an Edge or a network. `RG-02` and `RG-03` forbid it and there is no
  execution endpoint.
- No free-form chat, prompt entry, or natural-language query surface. The
  console sends no operator text to a model.
- No client-side model call, no provider key, no direct provider request. All
  AI traffic is Control Plane to provider.
- No email or webhook channel configuration, escalation contacts, secret
  rotation, or delivery-audit surface. `WCX-18` delivers those against
  `P4-W11` and `P4-W12`; only in-product notification is in scope here.
- No autonomous suppression, no automatic learning, no rule that applies
  without explicit operator approval.
- No incident merge or split beyond what the backend exposes.

## 7. Allowed paths

Only the paths in frontmatter. `ai/**` is forbidden; this package renders AI
output and never defines prompts, models, or grounding.

## 8. Security constraints

1. **AI output is untrusted content.** It is derived from attacker-influenced
   evidence and must render through the `WCX-06` untrusted components. It may
   never produce a link, an image, an executable action, a form, or any
   URL-bearing attribute. A model that emits Markdown or HTML has that content
   escaped, not interpreted.
2. **AI output is never authority.** No AI value may set or change severity,
   confidence, status, disposition, or any backend field. The console renders
   AI output beside the record and never merges it into the record's fields.
   A test asserts that no AI-derived value reaches a mutation payload.
3. **Evidence stays primary.** Per `EV-04`, the evidence section retains
   visual precedence over the AI section on every viewport. A test asserts
   document order and that the AI section is not rendered above evidence.
4. Hypotheses are rendered in a separate, explicitly labelled region from the
   evidence-backed summary. A hypothesis must never be styled or worded as an
   observation.
5. Every AI claim shows its citations to specific evidence records per
   `AIM-09`. A claim without a citation is rendered as an uncited statement
   and visually demoted; the console does not hide it, because hiding would
   misrepresent what the model produced, and does not promote it either.
6. Prompt-injection resistance is a rendering concern here: attacker text that
   instructs the model, and any model output that repeats an instruction, must
   render as inert quoted content. The `WCX-06` hostile corpus is extended
   with prompt-injection fixtures and exercised against the AI surface, per
   `SEC-09` and `RE-12`.
7. Guidance is curated backend content. The console renders it and never
   composes, reorders by its own logic, or adds an action.
8. Suppression approval must state precisely what will be suppressed, that
   evidence is retained per `CS-06`, and that notification is downgraded
   rather than deleted. Silent deletion wording is forbidden.
9. Disposition changes are level-1 confirmations that are reversible where the
   backend permits, and are audited server-side. Closing an incident as benign
   must state that evidence and history remain.
10. Notification content must never include a credential, a synthetic
    credential value, or raw evidence payload.
11. No AI output, notification content, or disposition state may be written to
    browser storage.

## 9. Implementation requirements

### 9.1 AI explanation surface

States, drawn from the `WCX-04` matrix:

| Backend condition | State | Rendering |
|---|---|---|
| No AI job yet | `unknown` | states that no explanation has been requested or produced |
| Job pending | pending on the object | shows elapsed time; never blocks the incident |
| Job succeeded | content | summary, citations, hypotheses, metadata |
| Job failed | `degraded` | states that explanation is unavailable and that the incident is unaffected, per `AIM-08` |
| Provider unavailable | `degraded` | same, naming the dependency |

Content layout, in order: evidence-backed summary with per-claim citations,
then a separately labelled hypotheses region, then an uncertainty statement.
Generation metadata — model identifier, generated-at timestamp using the
`WCX-08` `Timestamp`, and job identifier — is available behind an explicit
disclosure, not shown by default.

`AIM-13` and `PERF-06` mean the surface is asynchronous: the incident is fully
usable while the job is pending, and the thirty-second target is a UX
expectation, never a blocking wait. Pending state uses the `operational`
freshness class from `WCX-07`, not a tighter one.

### 9.2 Guidance surface

Three layers per `RG-01`, each labelled: verify and investigate, containment
consideration, and recovery and follow-up. Rendering rules:

1. Each action is curated backend text rendered as inert content.
2. Containment consideration is explicitly advisory. It must state that the
   product performs no containment, per `RG-02`.
3. No control anywhere in this section performs an action. `RG-03` forbids a
   one-click containment button.
4. When AI ranking is unavailable, the deterministic fallback guidance from
   `P4-W8` renders with no visible degradation of usefulness, and the surface
   states that the ordering is the default set.
5. `RG-05` guardrail: guidance that the backend marks as generic is labelled
   as general advice rather than incident-specific.

### 9.3 Notification centre

Per `NT-01`, `NT-05`, and `NT-06`:

1. A shell-level control shows an unacknowledged count and opens the centre.
2. Entries cover new incidents and material severity or confidence
   escalations only. Raw event notifications are forbidden.
3. Each entry shows the incident title, severity, confidence, observed source,
   time, delivery status, and read or acknowledged status.
4. Acknowledgement is an explicit operator action per `NT-06`, sent to the
   backend, and never inferred from merely opening the centre.
5. The unacknowledged count uses the `critical` freshness class. When the
   count read fails, the control renders `unknown` rather than zero, because
   rendering zero would assert that nothing needs attention.
6. Opening an incident from a notification does not acknowledge it.
7. `WCX-12`'s notification-state column stops rendering `unknown` and begins
   rendering real delivery state.

### 9.4 Disposition and suppression

1. Status transitions and disposition options come from the backend. The
   console renders the available set and never invents one.
2. Closing as expected or benign per `CS-07` is a level-1 confirmation that
   states evidence and history are retained.
3. A suppression suggestion produced by the system is presented as a proposal
   requiring explicit approval per `CS-08`. The console must never apply one
   automatically and must never present an approved suppression as something
   the system decided.
4. The approval dialog states the exact scope of the suppression, its
   duration or conditions as the backend defines them, that evidence is
   retained, and that notification is downgraded rather than removed.
5. Declining a suggestion is recorded through the backend if it supports it,
   and otherwise simply dismisses without implying a learned outcome, since
   `CS-08` forbids autonomous learning.

### 9.5 UI/UX requirements

1. On every viewport, the reading order on incident detail remains: what
   happened, confidence and severity, source, journey, evidence, then AI, then
   guidance, then status, then audit. AI never precedes evidence.
2. A pending AI job is visible but unobtrusive; it never occupies the position
   evidence would occupy.
3. Guidance reads as advice a generalist operator can act on without a SOC,
   with no jargon that the catalogue has not defined.
4. The notification centre is reachable by keyboard from anywhere in the
   shell.
5. At 320 pixels the AI and guidance sections stack below evidence without
   losing citations.

### 9.6 Accessibility requirements

1. The notification control exposes its unacknowledged count in its accessible
   name and announces a change once, politely, never assertively.
2. The AI section's pending state announces once on completion, not on each
   poll.
3. Hypotheses and summary are distinct regions with headings, so the
   distinction is available non-visually.
4. Citations are links to evidence records within the page, with accessible
   names identifying the cited record.
5. The suppression approval dialog meets the `WCX-04` dialog contract, and its
   scope statement is part of the accessible description.
6. Axe reports no serious or critical issue on every new surface at both
   viewports.

### 9.7 API and data contracts

Consumed from `P4-W8`, `P4-W9`, and `P4-W10`. No contract is defined here. If
the AI contract does not expose citations, hypotheses separately from summary,
job status, or generation metadata, stop and escalate, because `AIM-09` and
the separation requirement cannot be satisfied by client-side parsing of a
single text field.

### 9.8 Error and failure behaviour

1. AI failure never degrades the incident. The incident, journey, evidence,
   and disposition remain fully functional; only the AI section shows
   `degraded`.
2. A guidance failure falls back to the deterministic set and says so.
3. A notification count failure renders `unknown`, never zero.
4. A failed acknowledgement leaves the entry unacknowledged and states that
   nothing changed.
5. A failed disposition change leaves the status unchanged and states so; no
   optimistic update is applied.
6. A failed suppression approval leaves no partial rule; the console reports
   that nothing was applied.

### 9.9 Internationalisation and theme

Section headings, state copy, uncertainty phrasing, guidance layer names, and
the suppression scope statement enter the `WCX-08` catalogue and are reviewed
as security-critical wording. AI output, guidance text, and notification
content originating from the backend are never translated. The AI section uses
a neutral surface treatment and no severity token, so a model's output cannot
borrow the visual authority of a severity signal.

### 9.10 Performance

1. `PERF-06` and `Q-07` set a thirty-second ninety-fifth-percentile UX target
   for AI explanation when the provider is healthy. This is a target, not a
   blocker; the console must remain fully interactive throughout.
2. The notification count poll must not exceed the `critical` class rate and
   must respect the visibility rules from `WCX-07`.
3. The AI and notification features stay within the `WCX-12` interaction
   budget and the `WCX-07` load budgets.

### 9.11 Observability

None in the console. AI job identifiers are displayed for operator reference
but are not transmitted anywhere by the console and are excluded from the
`WCX-15` diagnostic report along with all evidence-derived content.

### 9.12 Documentation

Add an `AI, guidance, notifications, and disposition` section to
`docs/runbooks/web-console/development.md` covering the AI state table, the
evidence-primacy rule, the no-containment rule, the notification trigger set,
and the suppression approval wording. Record
`security/wcx-14-ai-surface-review.md`.

## 10. Required tests

### 10.1 Unit and component

1. Every AI state in 9.1 renders correctly, and a failed or unavailable job
   never degrades the incident.
2. Document order places evidence before AI on every viewport; asserted
   structurally, not visually.
3. Summary and hypotheses render in separate labelled regions; a hypothesis
   never carries observation wording.
4. Every cited claim links to its evidence record; an uncited claim is
   visually and programmatically demoted and is not hidden.
5. No AI-derived value appears in any mutation payload; asserted by
   intercepting every request during an AI-populated session.
6. AI output containing Markdown, HTML, ANSI, a URL, or an injected
   instruction renders inert; the extended prompt-injection corpus is
   exercised.
7. No guidance control performs an action; a source-level check asserts no
   mutation is wired into the guidance section.
8. Containment guidance states that the product performs no containment.
9. Deterministic fallback guidance renders when ranking is unavailable and
   says the ordering is the default set.
10. The notification centre lists only incident-level triggers; a raw event
    fixture produces no entry.
11. Acknowledgement is explicit; opening the centre or the incident does not
    acknowledge.
12. A failed count read renders `unknown`, never zero.
13. Benign closure states that evidence and history are retained.
14. A suppression suggestion cannot apply without explicit approval; the
    approval dialog states scope, evidence retention, and notification
    downgrade.
15. Every failure path leaves state unchanged and says so; no optimistic
    update exists.
16. No AI, notification, or disposition content reaches browser storage.

### 10.2 Browser and E2E scenarios

1. Full Phase 4 path: run the North-Star scenario, wait for the AI
   explanation, confirm citations resolve to evidence records, acknowledge the
   notification, and close the incident with a disposition.
2. Provider outage: force AI failure and confirm the incident remains fully
   usable with only the AI section degraded, per `AIM-08`.
3. Prompt injection: seed attacker content designed to instruct the model,
   confirm the rendered output is inert and that no navigation, request, or
   console error results, in all three browsers.
4. Suppression: receive a suggestion, decline it, receive it again, approve
   it, and confirm evidence remains and notification is downgraded rather than
   deleted.
5. Notification acknowledgement across two browser contexts.
6. Narrow-viewport rendering at 375 and 320 pixels with citations intact.
7. Axe scan of every new surface at both viewports.

## 11. Acceptance criteria and Definition of Done

1. AI explanation renders with citations, separated hypotheses, uncertainty,
   and disclosed metadata, and never outranks evidence.
2. AI unavailability never blocks or degrades the incident, satisfying
   `AIM-08` and `AIM-13`.
3. No AI value influences any record field or mutation payload.
4. Guidance renders three curated layers with no executable action, satisfying
   `AC-RG-001` through `AC-RG-003`.
5. The notification centre satisfies `AC-NT-001` and `AC-NT-004` with explicit
   acknowledgement.
6. Disposition and suppression require explicit operator approval and state
   evidence retention.
7. Prompt-injection fixtures render inert in all three browsers, satisfying
   `SEC-09` at the rendering boundary.
8. `task web:check` and `task web:e2e` pass within budget.
9. The AI-surface security review is recorded.

## 12. Evidence required

- Full Phase 4 browser path in all three engines.
- Provider-outage evidence showing an unaffected incident.
- Prompt-injection corpus rendering report.
- Request interception log proving no AI-derived value reaches a mutation.
- Document-order assertion output for evidence primacy.
- Suppression approval dialog screenshots with the scope statement.
- AI latency measurement against the `PERF-06` target.
- Axe reports and viewport screenshots.
- `security/wcx-14-ai-surface-review.md`.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- the AI contract does not separate summary from hypotheses or does not expose
  citations, making `AIM-09` unsatisfiable in the UI;
- guidance content arrives with an embedded action or link;
- a notification trigger outside the approved set appears in the feed;
- suppression semantics would require the console to apply a rule without
  approval;
- prompt-injection content cannot be rendered inert within the `WCX-06`
  contract;
- an operator workflow appears to require a containment control, which `RG-02`
  and `RG-03` forbid.

## 14. Deliverables

The AI explanation surface with citations, separated hypotheses, uncertainty,
and disclosed metadata, the three curated guidance layers with no executable
action, the notification centre with explicit acknowledgement, disposition and
benign closure, the suppression approval flow, the extended prompt-injection
fixture corpus, the browser scenarios, the security review, and the runbook
section.
