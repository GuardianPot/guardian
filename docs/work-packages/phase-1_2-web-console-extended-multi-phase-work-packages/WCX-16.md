---
id: WCX-16
phase: 2
wave: capability
title: Guided onboarding, placement validation, and coverage verification
status: draft
risk: high
components:
  - web-console
decision_refs:
  - ON-01
  - ON-02
  - ON-03
  - ON-04
  - ON-05
  - ON-06
  - OPS-02
  - UX-07
  - WC-D15
  - WC-D16
  - WC-D17
  - WC-D24
acceptance_refs:
  - AC-ON-005
  - AC-ON-006
  - ON-01 approved twelve-step onboarding flow
  - Phase 2 exit gate "coverage functional health works"
depends_on:
  - WCX-11
  - P2-W14
integration_dependencies:
  - P2-W1
  - P2-W2
allowed_paths:
  - "apps/web-console/src/**"
  - "tests/e2e/web-console/**"
  - "security/wcx-16-placement-validation-review.md"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-16.md"
forbidden_paths:
  - "apps/control-plane/**"
  - "apps/edge-agent/**"
  - "openapi/**"
  - "proto/**"
  - "decoys/**"
  - "0-planning-documents/**"
requires_owner_review: true
requires_security_review: true
---

# WCX-16 — Guided onboarding, placement validation, and coverage verification

## 1. Purpose

Deliver the console surfaces for the approved twelve-step onboarding flow that
no other package covers: consent-gated placement and reachability validation,
deterministic decoy placement proposal with operator override, owner approval
of deployment, coverage verification, and the environment readiness state that
concludes onboarding.

## 2. Why now

`ON-01` approves a twelve-step onboarding flow. `WCX-09`, `WCX-10`, and
`WCX-11` together cover steps 1 through 6 and step 10. Steps 7, 8, 9, 11, and
12 have no console home in the planning set, and `ON-06` makes coverage
verification a core requirement rather than an enhancement: a running decoy
process is explicitly not sufficient evidence that deception coverage exists.
`P2-W14` produces the functional coverage backend and `AC-ON-005` and
`AC-ON-006` are Phase 2 exit criteria.

## 3. Inputs and decisions

- `ON-01` — the approved twelve-step onboarding flow.
- `ON-02` — manual CIDR and zone definition plus **minimal** bounded checking,
  performed only with explicit consent and only for deception placement. No
  full asset inventory.
- `ON-03` — limited, explicitly triggered safe probes only: IP conflict, basic
  reachability, selected address availability, optional minimal environment
  hints. No broad vulnerability scan and no service fingerprinting.
- `ON-04` — deterministic rule-based placement recommendation. AI-generated
  placement is explicitly outside the MVP.
- `ON-05` — the owner may override proposed address, hostname or persona, and
  decoy selection; unsafe or conflicting configuration fails validation.
- `ON-06` — coverage verification is core; a functional probe over the Edge
  local network path must confirm the configured address and port respond.
- `OPS-02` — functional decoy health.
- `UX-07` — network coverage checks belong to the environment health surface.

## 4. Dependencies

`WCX-11` for the decoy feature, forms, and lifecycle patterns. **`P2-W14` must
be accepted first**, because coverage state is backend truth this package
renders. `P2-W1` and `P2-W2` are integration dependencies for real placement
and egress behaviour during browser evidence.

## 5. Scope

1. The onboarding progress surface covering all twelve `ON-01` steps.
2. Consent-gated placement and reachability validation with results.
3. Deterministic placement proposal with per-item operator override.
4. Explicit owner approval of a proposed deployment set.
5. Coverage verification trigger and results.
6. Environment readiness state and its contribution to the health surface.

## 6. Non-goals

- No network discovery, asset inventory, port sweep, service fingerprinting,
  or vulnerability scan. `ON-02` and `ON-03` bound this package to safe,
  explicitly triggered probes for deception placement only.
- No AI-assisted placement or persona generation. `ON-04` places it outside
  the MVP and `AIM-06` excludes persona generation.
- No decoy CRUD, configuration form, or lifecycle control. `WCX-11` owns
  those; this package composes them into the guided flow.
- No topology map or graphical placement editor. `UX-08` forbids it.
- No probe implementation. The console triggers backend probes and renders
  their results; it never performs a network operation itself.
- No incident, evidence, AI, or notification capability.

## 7. Allowed paths

Only the paths in frontmatter.

## 8. Security constraints

1. **Explicit consent is a hard gate.** No probe, reachability check, or
   address availability test may be initiated without an affirmative operator
   action for that specific run against that specific scope. Consent is never
   implied by navigating to a screen, never remembered across runs, and never
   defaulted to enabled.
2. The consent surface must state, before the action, exactly what will be
   probed, that probing touches the operator's real network, and that the
   scope is limited to deception placement. Vague wording here is a defect,
   because an operator authorising a network action must know what they are
   authorising.
3. The console must never present a capability the approved decisions exclude.
   No control may offer a broad scan, a port sweep, service fingerprinting, or
   asset discovery, even if a backend endpoint would accept it.
4. Probe results describe the operator's own network and are sensitive. They
   are never written to browser storage, never placed in a URL, and never
   included in the `WCX-15` diagnostic report.
5. Any value the probe reports back that originates outside the product —
   observed banners, responding hostnames, conflicting host identifiers — is
   untrusted and renders through the `WCX-06` components.
6. A failed, partial, or skipped coverage verification must never render as
   verified. `ON-06` exists precisely because process liveness is not
   coverage; the console must not reintroduce that inference.
7. Deployment approval is an operator decision that places attacker-facing
   assets on a real network. It is a level-2 confirmation naming the zone and
   every address that will be occupied.
8. An environment must never display a protected or ready state derived from
   configuration alone. Readiness requires backend-confirmed coverage.

## 9. Implementation requirements

### 9.1 Onboarding progress surface

A persistent surface listing all twelve `ON-01` steps with, for each: its
state drawn from the `WCX-04` matrix, what completed it, and the action that
advances it when the operator holds the capability.

Rules:

1. Step state is derived from backend truth only. A step is complete when the
   backend reports the underlying condition, never when the operator visited a
   screen.
2. Steps that are not yet reachable are shown with their blocking dependency
   named, not hidden.
3. The surface is informational and non-modal. An operator may work out of
   order where the backend permits it; the flow guides, it does not trap.
4. It renders `unknown` for any step whose state cannot be determined, never a
   default of incomplete or complete.

### 9.2 Consent-gated placement and reachability validation

Covering `ON-01` step 7 within the `ON-02` and `ON-03` bounds.

1. The trigger is an explicit control, disabled until a zone exists.
2. Activating it opens a consent dialog stating the target zone and CIDR, the
   exact check set — IP conflict, basic reachability, selected address
   availability, and optional minimal environment hints — and that no
   vulnerability scan or fingerprinting occurs.
3. Consent is per-run. Re-running requires re-consenting.
4. The run is long-running and follows `WC-D17`: progress lives on the zone
   and the onboarding surface, with elapsed time. No blocking modal.
5. Results are shown per checked address with an explicit outcome: available,
   conflicting, unreachable, or not determined. `not determined` is a distinct
   outcome and never collapses into available.
6. A conflict result names the conflicting observation and blocks proposing
   that address for placement.
7. Results carry the observation time using the `WCX-08` `Timestamp` and go
   stale under the `WCX-07` `configuration` class, because a network can
   change after a probe.

### 9.3 Placement proposal and override

Covering `ON-01` steps 8 and 9.

1. The proposal is produced by the backend from deterministic rules per
   `ON-04`. The console renders it and never generates, ranks, or filters it.
2. Each proposed item shows the decoy type and persona, the target zone and
   address, and the rule basis the backend supplies for the proposal.
3. Per `ON-05` the operator may override address, hostname or persona, and
   decoy selection on any item, and may remove an item from the set.
4. Overridden values are validated by the backend before approval. An unsafe
   or conflicting override fails with the backend reason attached to the item;
   the console never accepts an override the backend rejected.
5. Address and persona fields carry the attacker-visibility warning
   established in `WCX-11`, associated with the field.
6. Approval is a single level-2 confirmation over the whole set, naming the
   zone and enumerating every address that will be occupied.
7. Nothing is deployed before approval. The console must not deploy an item
   as a side effect of editing it.

### 9.4 Coverage verification

Covering `ON-01` step 11 and `ON-06`.

1. An explicit trigger runs the backend functional probe over the Edge local
   network path.
2. Results are per decoy and per configured address and port, with outcomes:
   responding, not responding, or not determined.
3. A decoy whose runtime is healthy but whose coverage probe does not respond
   renders as **not covered**, and the surface states plainly that a running
   process is not coverage.
4. Coverage state is separate from decoy runtime health and from the eight
   Edge conditions. Three distinct signals, never merged into one indicator.
5. Coverage results go stale under the `operational` class and show their
   observation age.
6. `AC-ON-005` and `AC-ON-006` evidence is produced from this surface.

### 9.5 Environment readiness

Covering `ON-01` step 12.

1. A readiness state on the environment, computed by the backend, never by
   the console.
2. It is rendered only when backend-confirmed coverage exists. Configuration
   completeness alone never produces it.
3. The wording must not overclaim. Per the product's truth semantics it states
   what is verified — that configured decoys are responding on their
   configured addresses as of an observation time — and never asserts that the
   environment is secure or protected from attack.
4. When any contributing signal is unknown, stale, or degraded, readiness
   renders `unknown` or `degraded` rather than a positive state.
5. Readiness contributes to the `UX-07` environment health surface as the
   network coverage checks element.

### 9.6 UI/UX requirements

1. A generalist operator must be able to complete onboarding from this surface
   without reading documentation.
2. Every long-running step shows elapsed time and what it is waiting on.
3. A blocked step names the blocker and links to where it is resolved.
4. At 320 pixels the step list, probe results, and proposal set stack without
   dropping any field.

### 9.7 Accessibility requirements

1. The step list is a semantic list with each step's state in its accessible
   name, not conveyed by colour or icon alone.
2. The consent dialog meets the `WCX-04` dialog contract, and its scope
   statement is part of the accessible description.
3. Probe, proposal, and coverage results are semantic tables with captions and
   row-scoped action names.
4. Long-running completion announces once, politely.
5. Override fields have persistent labels and associated warnings and errors.
6. Axe reports no serious or critical issue at both viewports.

### 9.8 API and data contracts

Consumed from `P2-W14` and the Phase 2 placement and decoy contracts. This
package defines no contract. If probe outcomes cannot distinguish
`not determined` from a negative result, or if coverage state is not separable
from runtime health, stop and escalate, because collapsing those distinctions
would violate `ON-06`.

### 9.9 Error and failure behaviour

1. A failed probe run renders `degraded` with the backend reason and leaves
   previous results visible with their age; it never clears them silently.
2. A partially completed run renders `partial` and names which addresses were
   not checked.
3. A rejected override leaves the item unchanged with the backend reason
   attached.
4. A failed approval deploys nothing and states that nothing changed.
5. A failed coverage verification never downgrades to a previously verified
   state; the previous result is shown with its age and the failure is stated.

### 9.10 Internationalisation and theme

All consent wording, probe outcome labels, coverage wording, and readiness
wording enter the `WCX-08` catalogue and are reviewed as security-critical
language. Coverage uses the health palette; readiness must not reuse the
brand accent, per the `WCX-03` disjointness rule.

### 9.11 Performance

Probe and coverage runs are long-running and polled under the `operational`
class. The onboarding surface must not poll while hidden. The feature shares
the decoy chunk from `WCX-11`.

### 9.12 Observability

None in the console. Probe and coverage results are excluded from the
`WCX-15` diagnostic report.

### 9.13 Documentation

Add a `Guided onboarding and coverage` section to
`docs/runbooks/web-console/development.md` covering the twelve steps, the
consent contract, the probe bounds, the three distinct health signals, and the
readiness wording rule. Record
`security/wcx-16-placement-validation-review.md`.

## 10. Required tests

### 10.1 Unit and component

1. No probe is initiated without an explicit per-run consent action; a test
   asserts no request is issued when consent is cancelled.
2. Consent is not remembered; a second run requires a second consent.
3. The consent dialog states scope, network impact, and the exact check set.
4. No control offers a scan, sweep, fingerprint, or discovery operation; a
   source-level check asserts no such request is constructible.
5. `not determined` never renders as available or as responding.
6. A healthy runtime with a failing coverage probe renders **not covered**,
   with an explicit statement that a running process is not coverage.
7. Runtime health, coverage state, and the eight Edge conditions render as
   three distinct signals.
8. Readiness never renders from configuration alone; a fixture with complete
   configuration and absent coverage renders `unknown`.
9. Readiness wording asserts verification, not protection; asserted against
   the catalogue entry.
10. A rejected override leaves the item unchanged with the backend reason.
11. Approval is a single level-2 confirmation enumerating every address.
12. Editing a proposal item deploys nothing.
13. Hostile probe-reported values render inert.
14. No probe, coverage, or proposal content reaches browser storage.
15. Step state derives from backend truth; visiting a screen completes
    nothing.

### 10.2 Browser and E2E scenarios

1. Complete the full `ON-01` twelve-step flow end to end against the real
   backend, producing `AC-ON-005` and `AC-ON-006` evidence.
2. Trigger validation against a zone containing a deliberate IP conflict and
   confirm the conflicting address is blocked from placement.
3. Override a proposed address and persona, confirm backend validation, and
   deploy the approved set.
4. Kill a decoy process and confirm coverage flips to not covered while
   runtime health reports its own distinct state.
5. Remove a decoy address at the network layer and confirm coverage flips
   without runtime health claiming failure.
6. Cancel consent and confirm no probe request was issued.
7. Narrow-viewport rendering at 375 and 320 pixels.
8. Axe scan of every new screen and dialog.

## 11. Acceptance criteria and Definition of Done

1. All twelve `ON-01` steps have a console surface and derive state from
   backend truth.
2. No probe runs without explicit per-run consent, and no excluded scanning
   capability is offered anywhere.
3. Placement proposals are backend-produced, overridable per `ON-05`, and
   backend-validated.
4. Deployment requires explicit owner approval enumerating every address.
5. Coverage verification satisfies `ON-06`, `AC-ON-005`, and `AC-ON-006`, and
   a running process never renders as coverage.
6. Readiness requires confirmed coverage and never overclaims protection.
7. `task web:check` and `task web:e2e` pass within budget.
8. The security review is recorded.

## 12. Evidence required

- Full twelve-step onboarding browser evidence in all three engines.
- `AC-ON-005` and `AC-ON-006` evidence from the coverage surface.
- IP-conflict scenario evidence showing the address blocked.
- Kill-process and remove-address scenarios showing coverage and runtime
  health diverging.
- Consent-cancelled evidence showing no request issued.
- Source-level proof that no scanning capability is constructible.
- Axe reports and viewport screenshots.
- `security/wcx-16-placement-validation-review.md`.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- probe outcomes cannot distinguish `not determined` from a negative result;
- coverage state is not separable from decoy runtime health in the contract;
- the backend exposes a discovery or scanning capability broader than `ON-02`
  and `ON-03` permit, which must not be surfaced regardless;
- placement proposals cannot be rendered with a rule basis, making the
  deterministic origin unverifiable to the operator;
- readiness cannot be computed by the backend and would have to be derived in
  the console;
- an `ON-01` step has no backend condition to derive its state from.

## 14. Deliverables

The twelve-step onboarding surface, consent-gated placement and reachability
validation with per-address outcomes, the deterministic placement proposal
with operator override and backend validation, owner deployment approval,
coverage verification satisfying `ON-06`, the environment readiness state,
the browser scenarios including `AC-ON-005` and `AC-ON-006` evidence, the
security review, and the runbook section.
