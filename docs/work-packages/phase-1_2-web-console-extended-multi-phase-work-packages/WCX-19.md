---
id: WCX-19
phase: 5
wave: capability
title: Update and rollback control, operational health, diagnostics bundle
status: approved-for-implementation
risk: high
components:
  - web-console
decision_refs:
  - UP-01
  - UP-02
  - UP-03
  - UP-04
  - UP-05
  - UP-06
  - UX-07
  - OPS-01
  - OPS-03
  - OPS-04
  - AIM-08
  - SEC-02
  - AUTH-06
  - WC-D15
  - WC-D16
  - WC-D24
  - WC-D28
acceptance_refs:
  - AC-UP-001
  - AC-UP-002
  - AC-UP-003
  - AC-UP-004
  - OPS-04 diagnostics bundle
  - UX-07 environment and Edge health minimum
depends_on:
  - WCX-16
  - WCX-18
  - P5-W3
  - P5-W4
  - P5-W8
  - P5-W11
allowed_paths:
  - "apps/web-console/src/**"
  - "tests/e2e/web-console/**"
  - "security/wcx-19-update-diagnostics-review.md"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-19.md"
forbidden_paths:
  - "apps/control-plane/**"
  - "apps/edge-agent/**"
  - "openapi/**"
  - "proto/**"
  - "0-planning-documents/**"
requires_owner_review: true
requires_security_review: true
---

# WCX-19 — Update and rollback control, operational health, diagnostics bundle

## 1. Purpose

Deliver the remaining approved operational surfaces: the explicit update
trigger and rollback control for the Edge and decoy packs, the full `UX-07`
environment and Edge health minimum including queue and loss state, and the
console entry point for the redacted diagnostics bundle.

## 2. Why now

`UP-02` approves a controlled updater with an **explicit trigger**, which by
definition requires an operator control. `UP-03` makes rollback core and
`UP-04` requires update visibility; `P5-W4` names visibility as a deliverable.
`UX-07` approves an environment and Edge health minimum that includes local
queue state, decoy health counts, network coverage checks, AI provider status,
and update status — the console today shows only the eight `P1-W9` conditions.
`P5-W8` defines an explicit evidence-loss state that an operator must be able
to see, and `OPS-04` requires a redacted diagnostics bundle.

## 3. Inputs and decisions

- `UP-01` — unsigned or untrusted decoy packs cannot activate; a release
  blocker.
- `UP-02` — controlled signed updater with an explicit trigger. Automatic
  scheduling and channel UX are post-MVP.
- `UP-03` — rollback to a previous known-good binary and configuration after a
  failed post-update health check.
- `UP-04` — decoy pack update by signed digest-pinned pull with verification,
  rollout, and health result.
- `UP-05` — offline update bundles are out. `UP-06` — staged fleet rollout is
  out because the MVP has one active Edge per environment.
- `UX-07` — the approved environment and Edge health minimum.
- `OPS-03` — staleness. `OPS-04` — redacted diagnostics bundle.
- `AIM-08` — defined behaviour when AI is unavailable, surfaced as provider
  status.
- `WC-D28` — the console's own frontend diagnostic report is separate and is
  delivered by `WCX-15`.

## 4. Dependencies

`WCX-16` for coverage state and `WCX-18` for channel health, both of which
feed the `UX-07` surface. **`P5-W3`, `P5-W4`, `P5-W8`, and `P5-W11` must be
accepted first.**

## 5. Scope

1. Edge update: available version, explicit trigger, staged progress,
   post-update health result, and rollback.
2. Decoy pack update: digest-pinned target, verification result, rollout
   progress, health result, and rollback to the previous digest.
3. The full `UX-07` environment and Edge health surface.
4. Queue and spool pressure state including the explicit loss state.
5. AI provider status.
6. The diagnostics bundle request, progress, and retrieval entry point.

## 6. Non-goals

- No automatic update, update scheduling, or release-channel selection.
  `UP-02` places scheduling and channel UX post-MVP.
- No offline update bundle upload. `UP-05` excludes it.
- No staged or fleet rollout control. `UP-06` excludes it.
- No signature, digest, or metadata verification in the console. Verification
  is the updater's responsibility; the console renders its result.
- No log viewer. The diagnostics bundle carries logs; the console does not
  render them.
- No Edge shell, remote command, or configuration file editor.
- No frontend error telemetry. `WCX-15` owns the console's own diagnostic
  report under `WC-D28`.

## 7. Allowed paths

Only the paths in frontmatter.

## 8. Security constraints

1. Triggering an update or a rollback changes production software on a device
   that holds a private key and faces an attacker. Both are level-3
   irreversible-security actions under `WCX-04`: typed confirmation naming the
   device, plus step-up reauthentication. If change proposal `0003` is not
   approved, these controls are not shipped and the package delivers only the
   read-only surfaces.
2. The console must never present an unverified artefact as installable.
   `UP-01` is a release blocker; the surface shows the backend's verification
   result and offers no override, no force, and no skip-verification control.
3. A failed post-update health check must be visible as a failure even when
   rollback succeeded. Reporting only the recovered state would hide that an
   update was bad.
4. Version strings, digests, release notes, and updater messages originate
   outside the console and render through the `WCX-06` untrusted components. A
   digest is displayed in full or not at all; a truncated digest must not be
   presented as an identity the operator can rely on.
5. **The explicit evidence-loss state from `P5-W8` must never be
   silenced or aggregated away.** If the backend reports that events were
   dropped, the console states it, with the count and time range the backend
   supplies, on the environment health surface and not only in a detail view.
6. The diagnostics bundle is redacted server-side. The console never assembles
   it, never inspects its contents, and never renders them. It requests
   generation, shows progress, and hands the operator the backend-provided
   retrieval path.
7. Requesting a diagnostics bundle is a level-2 confirmation stating what
   classes of information the bundle contains and that it must be handled as
   sensitive support material.
8. No update state, queue state, or diagnostics content reaches browser
   storage or the `WCX-15` diagnostic report.

## 9. Implementation requirements

### 9.1 Edge update and rollback

Per device:

1. Current version, available version if any, and the backend's verification
   result for the available artefact.
2. An explicit update control, enabled only when the backend reports a
   verified available artefact and the device is in a state that permits it.
   The reason is shown when it is disabled.
3. Update is long-running and follows `WC-D17`: staged progress on the device
   object showing the stage the updater reports — staged, verified, activated,
   restarted, health-checked — with elapsed time. No blocking modal.
4. The post-update health result is shown explicitly as passed or failed.
5. Rollback is offered when the backend reports a known-good previous version.
   It is a separate level-3 action and states which version it restores.
6. A rollback that follows a failed health check shows both facts: the update
   failed, and the previous version was restored.
7. Update history per device: version, time, outcome, and who triggered it,
   sourced from the audit trail per `AUTH-06`.

### 9.2 Decoy pack update

1. Per decoy or pack: current digest, target digest, and the verification
   result.
2. Digests are displayed in full, with a copy control, never truncated as an
   identity.
3. Update is triggered explicitly, is long-running, and reports pull, verify,
   controlled stop and start, and health outcome as the backend reports them.
4. Rollback to the previous digest is offered where the backend supports it,
   as a level-2 action because a decoy is not a device holding a private key.
5. An unverified pack renders as not installable with the backend reason, and
   no control offers to proceed.

### 9.3 Full `UX-07` health surface

The environment health surface renders the complete approved minimum, each as
a distinct signal that is never merged into a single verdict:

| Element | Source |
|---|---|
| Edge connected or offline, last contact | `P1-W9` conditions |
| Control-plane connection | `P1-W9` conditions |
| Local queue and spool state | `P5-W8` |
| Decoy health counts | `P2-W14`, rendered by `WCX-11` |
| Network coverage checks | `WCX-16` coverage state |
| AI provider status | `P4-W2` provider state |
| Update status | 9.1 and 9.2 |

Rules:

1. Each element carries its own state from the `WCX-04` matrix and its own
   observation age. An unavailable element renders `unknown`, never healthy
   and never omitted.
2. No aggregate roll-up may render as a single healthy verdict. A summary may
   state how many elements are healthy, unknown, and failing, but never
   collapse them into one positive indicator.
3. Queue state shows depth or bytes against the configured threshold, the
   warning state, and, when the backend reports it, the **explicit loss
   state** with count and time range. The loss state is visually prominent and
   is stated in the summary, not only in a detail panel.
4. AI provider status reflects `AIM-08`: unavailable is an operational
   condition that does not degrade incidents, and the surface says so.

### 9.4 Diagnostics bundle

1. A request control in the account or environment operations area.
2. A level-2 confirmation naming the information classes the bundle contains
   and stating that it is sensitive support material.
3. Generation is long-running with progress on the request object.
4. On completion the console presents the backend-provided retrieval path and
   the bundle's identifier, size, and generation time. The console does not
   read, parse, or render bundle contents.
5. Request history with actor and time, sourced from the audit trail.
6. A failed generation renders `degraded` with the backend reason and offers
   retry.

### 9.5 UI/UX requirements

1. An operator must be able to answer "is anything degraded, and has anything
   been lost" from one screen.
2. Every long-running operation names what it is waiting on.
3. Update controls are grouped with the device they affect, never in a global
   action bar where the target could be mistaken.
4. At 320 pixels every health element and its state remain visible; nothing is
   dropped or hidden behind a summary.

### 9.6 Accessibility requirements

1. Health elements form a semantic list or table with each state in its
   accessible name, never conveyed by colour alone.
2. The loss state is announced once when it first appears.
3. Update progress announces on stage change, not on each poll.
4. Confirmation and step-up dialogs meet the `WCX-04` contract.
5. Digest copy controls have accessible names identifying what is copied.
6. Axe reports no serious or critical issue at both viewports.

### 9.7 API and data contracts

Consumed from `P5-W3`, `P5-W4`, `P5-W8`, `P5-W11`, `P4-W2`, and the existing
health projection. This package defines no contract. If the backend does not
expose an explicit loss state, an update verification result, or a post-update
health outcome separable from the current state, stop and escalate.

### 9.8 Error and failure behaviour

1. A failed update leaves the reported version unchanged and states that
   nothing was installed.
2. A failed rollback states explicitly that the device remains on the failed
   version, because an operator who believes rollback succeeded will stop
   investigating.
3. An unavailable health element renders `unknown`, never healthy and never
   omitted from the surface.
4. A stale health element shows its age under `OPS-03`.
5. A failed diagnostics generation renders `degraded`; no partial bundle is
   offered.
6. No update, rollback, or generation action is optimistic.

### 9.9 Internationalisation and theme

All update, rollback, queue, loss, and diagnostics wording enters the `WCX-08`
catalogue. Loss-state and rollback-failure wording is security-critical and is
reviewed as such. The loss state uses the health palette's failing token, not
a severity token, because it is an operational condition rather than an
incident severity.

### 9.10 Performance

Update and generation progress polls under the `operational` class and stops
when hidden. The health surface must remain within the `WCX-12` interaction
budget with every element present. The operations feature is its own chunk.

### 9.11 Observability

None in the console beyond rendering backend state. Diagnostics content is
never read by the console.

### 9.12 Documentation

Add an `Updates, operational health, and diagnostics` section to
`docs/runbooks/web-console/development.md` covering the explicit-trigger
model, the rollback reporting rule, the seven `UX-07` elements, the loss-state
rule, and the diagnostics request flow. Record
`security/wcx-19-update-diagnostics-review.md`.

## 10. Required tests

### 10.1 Unit and component

1. Update and rollback require typed confirmation and step-up; no request is
   issued when either is cancelled.
2. No control offers force, skip-verification, or override; a source-level
   check asserts no such request is constructible.
3. An unverified artefact renders as not installable with the backend reason.
4. A failed post-update health check is reported as a failure even when
   rollback succeeded; both facts are present.
5. A failed rollback states that the device remains on the failed version.
6. Digests render in full; a truncated digest never appears as an identity.
7. All seven `UX-07` elements render, each with its own state and age, and no
   aggregate renders a single healthy verdict.
8. An unavailable element renders `unknown` and is never omitted.
9. The explicit loss state renders with count and time range and appears in
   the summary, not only in a detail panel.
10. AI provider unavailability renders as operational and states that
    incidents are unaffected.
11. The diagnostics confirmation names the information classes; the console
    never parses or renders bundle content.
12. Hostile version strings, release notes, and updater messages render inert.
13. No update, queue, or diagnostics content reaches browser storage.

### 10.2 Browser and E2E scenarios

1. Trigger an Edge update against a verified artefact and confirm staged
   progress and a passing health result, producing `AC-UP-001` and `AC-UP-002`
   evidence.
2. Force a failing post-update health check and confirm rollback occurs and
   both facts are reported, producing `AC-UP-003` evidence.
3. Update a decoy pack by digest and confirm verification, rollout, and health
   result, producing `AC-UP-004` evidence.
4. Present an unsigned or tampered artefact and confirm it renders as not
   installable with no path to proceed.
5. Drive the spool past its threshold until the backend reports loss and
   confirm the loss state is prominent on the health surface.
6. Take the AI provider offline and confirm provider status degrades while
   incidents remain unaffected.
7. Request a diagnostics bundle and confirm the retrieval path is presented
   without the console reading its contents.
8. Narrow-viewport rendering at 375 and 320 pixels with all seven elements.
9. Axe scan of every new screen and dialog.

## 11. Acceptance criteria and Definition of Done

1. Edge update and rollback are operable under level-3 confirmation and
   satisfy `AC-UP-001` through `AC-UP-003`.
2. Decoy pack update satisfies `AC-UP-004`, and an unverified artefact is
   never installable.
3. A failed update is visible even after a successful rollback; a failed
   rollback is stated explicitly.
4. All seven `UX-07` elements render as distinct signals with no single
   healthy verdict.
5. The explicit evidence-loss state is prominent and never silenced.
6. The diagnostics bundle can be requested and retrieved without the console
   reading its contents.
7. `task web:check` and `task web:e2e` pass within budget.
8. The security review is recorded.

## 12. Evidence required

- `AC-UP-001` through `AC-UP-004` browser evidence in all three engines.
- Failed-health-check and rollback evidence showing both facts reported.
- Unsigned-artefact evidence showing no path to proceed.
- Loss-state evidence with count and time range on the health surface.
- AI provider outage evidence with incidents unaffected.
- Diagnostics request evidence with no content rendering.
- Axe reports and viewport screenshots.
- `security/wcx-19-update-diagnostics-review.md`.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- change proposal `0003` is unapproved, so update and rollback cannot be
  gated by step-up reauthentication;
- the backend does not expose an explicit evidence-loss state, making silent
  loss unavoidable;
- a post-update health outcome cannot be distinguished from the current
  health state after rollback;
- the contract exposes a force or skip-verification path, which must not be
  surfaced regardless;
- rollback semantics do not report whether the device remains on the failed
  version;
- the diagnostics contract requires the console to assemble or redact any part
  of the bundle.

## 14. Deliverables

Edge update and rollback controls with staged progress and truthful failure
reporting, decoy pack update by verified digest, the complete `UX-07` health
surface with seven distinct signals, queue and explicit loss state, AI
provider status, the diagnostics bundle request and retrieval entry point, the
browser scenarios producing `AC-UP-001` through `AC-UP-004`, the security
review, and the runbook section.
