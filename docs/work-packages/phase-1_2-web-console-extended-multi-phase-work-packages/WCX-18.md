---
id: WCX-18
phase: 4
wave: capability
title: Notification channel configuration and escalation contacts
status: approved-for-implementation
risk: high
components:
  - web-console
decision_refs:
  - NT-02
  - NT-03
  - NT-04
  - NT-05
  - NT-06
  - NT-07
  - SEC-02
  - AUTH-06
  - WC-D16
  - WC-D24
acceptance_refs:
  - AC-NT-002
  - AC-NT-003
  - Phase 4 exit gate "in-product + email + webhook"
depends_on:
  - WCX-14
  - P4-W11
  - P4-W12
allowed_paths:
  - "apps/web-console/src/**"
  - "tests/e2e/web-console/**"
  - "security/wcx-18-notification-configuration-review.md"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-18.md"
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

# WCX-18 — Notification channel configuration and escalation contacts

## 1. Purpose

Give the operator the console surfaces required to configure and operate the
email and webhook notification channels: escalation contacts, webhook endpoint
and secret rotation, delivery status, and per-channel trigger settings.

## 2. Why now

`NT-02` makes email core and `NT-03` makes a generic webhook a supported
channel; `P4-W11` and `P4-W12` build both, and the Phase 4 exit gate requires
in-product, email, and webhook together. `NT-07` makes escalation contacts an
approved, operator-configurable concept. `WCX-14` deliberately scoped itself
to in-product notification and named channel configuration as out of scope,
which left `AC-NT-002` and `AC-NT-003` without a console home: a channel
nobody can configure or verify is not a delivered capability.

## 3. Inputs and decisions

- `NT-02` — email is core; the adapter may be deployment-specific.
- `NT-03` — a generic webhook is supported, with signed or authenticated
  delivery, retry and backoff, delivery audit, and secret rotation per
  `P4-W12`.
- `NT-04` — native Slack and Teams integrations are out.
- `NT-05` — triggers are new incidents and material severity or confidence
  escalation only; raw events never notify.
- `NT-06` — email and webhook delivery state is separate from operator
  acknowledgement, which `WCX-14` owns.
- `NT-07` — one primary and one optional secondary contact. Complex schedules
  and on-call rotations are out.
- `SEC-02` — no production secrets in the repository or in artefacts.
- `AUTH-06` — configuration changes are audited.

## 4. Dependencies

`WCX-14` for the notification model and the in-product centre. **`P4-W11` and
`P4-W12` must be accepted first.**

## 5. Scope

1. Escalation contact management: primary and optional secondary.
2. Email channel configuration and its verification flow.
3. Webhook endpoint configuration, signing secret handling, and rotation.
4. Per-channel trigger settings within the `NT-05` bounds.
5. Delivery status and delivery audit presentation.
6. Channel health and degraded-delivery reporting.

## 6. Non-goals

- No Slack, Teams, PagerDuty, or any named third-party integration. `NT-04`
  excludes them.
- No on-call schedule, rotation, escalation chain, or time-based routing.
  `NT-07` limits this to primary plus optional secondary.
- No per-user notification preferences. `AUTH-01` defines a single
  organization with a local account.
- No raw-event notification option. `NT-05` forbids it and the console must
  not offer a setting that would enable it.
- No SMTP server credential entry unless the approved contract exposes it as a
  configurable field; deployment-specific transport configuration stays with
  deployment, per `NT-02`.
- No in-product notification centre work. `WCX-14` owns it.
- No notification payload template editor.

## 7. Allowed paths

Only the paths in frontmatter.

## 8. Security constraints

1. **A webhook signing secret is one-time material.** It is displayed exactly
   once at creation or rotation, under the same rules the enrollment secret
   already follows: it bypasses the query cache, lives only in route-local
   state, leaves the DOM on dismissal, route exit, or unload, and never enters
   browser storage, a URL, a log, a trace, a video, or a screenshot.
2. After creation the console shows only non-secret metadata: an identifier,
   a creation time, a rotation time, and a fingerprint if the contract
   supplies one. It must never re-display a stored secret, and must not offer
   a control that would reveal one.
3. Rotation is a level-2 confirmation stating that the previous secret stops
   being valid according to the backend's overlap policy, and stating what an
   operator must update at the receiving end before or after rotation.
4. The webhook endpoint URL is operator-supplied and is a server-side request
   target. The console validates that it is an absolute HTTPS URL and rejects
   anything else, but the console is **not** the authority: server-side
   request forgery protection is a Control Plane responsibility, and the
   console must not imply that its validation makes an endpoint safe.
5. Contact email addresses are personal data. They are rendered as entered,
   through the `WCX-06` untrusted components, are never placed in a URL, and
   are excluded from the `WCX-15` diagnostic report.
6. Delivery audit entries may contain response codes and error text from an
   operator-configured third-party endpoint. That content is untrusted and
   renders through the `WCX-06` components; a failing webhook receiver must
   not be able to inject content into the console.
7. No notification setting may enable raw-event notification or widen the
   trigger set beyond `NT-05`. A control that could is a defect.
8. A verification or test send must not be usable as an arbitrary message
   sender. Its payload is fixed by the backend; the console supplies no
   operator-authored content.

## 9. Implementation requirements

### 9.1 Escalation contacts

1. A primary contact is required for the email channel to be enabled; a
   secondary is optional, per `NT-07`.
2. Each contact shows its address, its verification state, and when it was
   last verified.
3. Adding or changing a contact requires verification before the channel
   treats it as deliverable. An unverified contact renders as unverified and
   never as active.
4. Removing the primary contact while email is enabled is a level-2
   confirmation stating that email notification will stop.

### 9.2 Email channel

1. Enable and disable, with the enabled state reflecting backend truth rather
   than the operator's last click.
2. Trigger settings limited to the `NT-05` set: new incident, and material
   severity or confidence escalation. Severity thresholds are offered only if
   the contract defines them.
3. A verification send with a fixed backend payload, whose result is shown
   with its outcome and time.
4. Channel state renders through the `WCX-04` matrix: `degraded` when the
   backend reports delivery failures, with the backend reason.

### 9.3 Webhook channel

1. Endpoint URL, enable and disable, and trigger settings as in 9.2.
2. Secret creation and rotation per 8.1 to 8.3.
3. A verification send with a fixed payload, showing the receiver's response
   status and the delivery outcome.
4. Retry and backoff state is displayed as the backend reports it: attempts
   made, next attempt time, and whether delivery was abandoned. An abandoned
   delivery must be visible, because a silently dropped notification is
   indistinguishable from no incident.

### 9.4 Delivery status and audit

1. A delivery list per channel showing incident reference, attempt count,
   last attempt time, outcome, and receiver response code where applicable.
2. Delivery state is explicitly separate from operator acknowledgement per
   `NT-06`; the two are never merged into one indicator.
3. Failed and abandoned deliveries are prominent, not buried behind a filter
   default.
4. Configuration changes and rotations appear in the organization audit
   viewer from `WCX-13`.
5. `WCX-12`'s incident-list notification-state column consumes this delivery
   state.

### 9.5 UI/UX requirements

1. The settings surface states plainly which channels are active and which
   incidents would reach the operator, so a misconfiguration is visible rather
   than silent.
2. A channel enabled without a verified contact or a reachable endpoint
   renders as not deliverable, with the reason.
3. The one-time secret dialog reuses the enrollment-secret pattern from
   `P1-W11` so operators meet one consistent handling model.
4. At 320 pixels the settings and delivery lists stack without dropping the
   outcome or attempt count.

### 9.6 Accessibility requirements

1. Every setting control has a persistent label and a described current state.
2. The one-time secret dialog meets the `WCX-04` dialog contract, and its
   announcement contains no secret material.
3. Delivery lists are semantic tables with captions and row-scoped action
   names.
4. Verification results announce once on completion.
5. Axe reports no serious or critical issue at both viewports.

### 9.7 API and data contracts

Consumed from `P4-W11` and `P4-W12`. This package defines no contract. If the
contract does not expose delivery outcome separately from acknowledgement, or
does not expose abandoned deliveries, stop and escalate, because `NT-06` and
8.7 cannot otherwise be satisfied.

### 9.8 Error and failure behaviour

1. A failed configuration save leaves the previous configuration displayed and
   states that nothing changed.
2. A failed verification marks the contact or endpoint unverified and shows
   the backend reason; it never leaves a previously verified state implied.
3. A failed rotation leaves the previous secret valid and states so
   explicitly, because an operator who believes rotation succeeded will update
   the receiver and break delivery.
4. A channel whose delivery is degraded renders `degraded` and never a healthy
   enabled state.
5. An unavailable delivery list renders `unknown`, never an empty list, since
   an empty delivery list would imply no notifications were attempted.

### 9.9 Internationalisation and theme

All settings, verification, rotation, and delivery wording enters the `WCX-08`
catalogue. Rotation and abandoned-delivery wording is security-critical and is
reviewed as such. Channel health uses the health palette, never a severity
token; a failing channel is an operational condition, not an incident
severity.

### 9.10 Performance

Delivery lists use the `WCX-12` table primitives with cursor pagination and
the `operational` freshness class. The settings feature is its own chunk
loaded only from the account area.

### 9.11 Observability

None in the console. Contact addresses, endpoint URLs, and secrets are
excluded from the `WCX-15` diagnostic report.

### 9.12 Documentation

Add a `Notification channels` section to
`docs/runbooks/web-console/development.md` covering the one-time secret model,
the rotation procedure and its receiver-side implications, the trigger bounds,
and the delivery-versus-acknowledgement distinction. Record
`security/wcx-18-notification-configuration-review.md`.

## 10. Required tests

### 10.1 Unit and component

1. A webhook secret is shown once, leaves the DOM on dismissal, route exit,
   and unload, and never reaches the query cache or browser storage.
2. No control re-displays a stored secret; a source-level check asserts no
   read path exists.
3. Rotation is a level-2 confirmation stating the previous secret's fate; a
   failed rotation states that the previous secret remains valid.
4. A non-HTTPS or relative endpoint URL is rejected, and the surface does not
   claim that validation makes the endpoint safe.
5. No control can widen triggers beyond the `NT-05` set; a fixture proves no
   raw-event option is constructible.
6. An unverified contact never renders as active or deliverable.
7. Delivery state and acknowledgement render as separate indicators.
8. An abandoned delivery is visible without changing any filter.
9. An unavailable delivery list renders `unknown`, never empty.
10. A degraded channel never renders as healthy and enabled.
11. Hostile receiver response text and hostile contact display values render
    inert.
12. A verification send carries no operator-authored content.
13. No contact address, endpoint, or secret reaches browser storage.

### 10.2 Browser and E2E scenarios

1. Configure a primary contact, verify it, enable email, trigger an incident,
   and confirm delivery, producing `AC-NT-002` evidence.
2. Configure a webhook against a disposable receiver, create the secret,
   verify the signature at the receiver, trigger an incident, and confirm
   signed delivery, producing `AC-NT-003` evidence.
3. Rotate the webhook secret, confirm the receiver rejects the old signature
   and accepts the new one, and confirm the console never re-displays either.
4. Point the webhook at a failing receiver and confirm retry, backoff, and
   abandoned state are visible.
5. Return a hostile response body from the receiver and confirm it renders
   inert.
6. Remove the primary contact and confirm email is reported as not
   deliverable.
7. Narrow-viewport rendering at 375 and 320 pixels.
8. Axe scan of every new screen and dialog.

Secret-bearing steps keep traces, video, and screenshots disabled. Screenshots
are taken only after explicit secret dismissal.

## 11. Acceptance criteria and Definition of Done

1. Escalation contacts, email, and webhook are configurable and operable from
   the console, satisfying `AC-NT-002` and `AC-NT-003`.
2. Webhook secrets follow the one-time model with no re-display path.
3. Rotation states its effect and reports failure truthfully.
4. Triggers cannot be widened beyond `NT-05`.
5. Delivery state is separate from acknowledgement, and abandoned deliveries
   are visible.
6. Hostile receiver content is inert.
7. `task web:check` and `task web:e2e` pass within budget.
8. The security review is recorded.

## 12. Evidence required

- `AC-NT-002` and `AC-NT-003` browser evidence in all three engines.
- Signature verification evidence at the disposable receiver, before and after
  rotation.
- Retry, backoff, and abandoned-delivery evidence.
- Hostile receiver response rendering report.
- Storage and cache assertions for the one-time secret.
- Audit records for configuration changes and rotations.
- Axe reports and viewport screenshots.
- `security/wcx-18-notification-configuration-review.md`.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- the contract does not separate delivery outcome from acknowledgement;
- abandoned deliveries are not exposed, making silent notification loss
  invisible;
- the webhook secret can be read back from the contract after creation;
- rotation semantics do not define the previous secret's validity window, so
  the console cannot state the receiver-side implication truthfully;
- the contract permits a trigger configuration outside `NT-05`;
- SMTP credentials would have to be entered in the console, which `NT-02`
  leaves as deployment-specific and `SEC-02` constrains.

## 14. Deliverables

Escalation contact management with verification, email channel configuration
and verification, webhook endpoint configuration with one-time secret creation
and rotation, `NT-05`-bounded trigger settings, delivery status and audit
presentation with visible abandoned deliveries, the browser scenarios
producing `AC-NT-002` and `AC-NT-003`, the security review, and the runbook
section.
