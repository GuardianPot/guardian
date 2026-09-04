# Phase 1 gate

Phase 1 establishes a secure, durable, and observable management loop between
the Control Plane, Web Console, and Edge Agent. It is not complete until every
approved Phase 1 package has acceptance evidence and the Product Owner closes
this gate.

## Entry dependencies

- Phase 0 technical evidence is complete.
- Phase 0 gate is Product Owner approved and closed (2026-08-23).
- P1-W1 through P1-W11 specifications are Product Owner approved (recorded
  2026-08-22).
- Required development PostgreSQL and isolated Edge test environments exist.

## Entry decision — 2026-08-23

The Product Owner authorized Phase 1 product implementation. Wave 1 begins with
P1-W1 and P1-W7; all other packages remain dependency-blocked until their
approved predecessors close.

## Required exit evidence

- authentication and TOTP;
- environment and zone CRUD;
- Edge enrollment, rotation, and revocation;
- production-shaped mTLS device channel;
- desired/observed-state reconciliation;
- SQLite durable local state and explicit corruption behavior;
- privileged-helper security tests;
- platform health UI with truthful degraded reasons;
- append-oriented audit baseline;
- browser onboarding E2E;
- AC-ON-001/002/003 and AC-SEC-005/006;
- restart, reconnect, and idempotency smoke evidence.

## Web Console scope clarification — 2026-09-03

The Product Owner resolved how the `Edge enrollment, rotation, and revocation`
exit evidence applies to the Web Console. Revocation is satisfied at
capability level by the `P1-W4` API, the device-channel refusal behaviour, and
their acceptance evidence. A console interface for device disable and revoke,
enrollment-token list and revoke, zone edit and delete, session list and
revoke, and password change is **Phase 2 work**, delivered by `WCX-09` in
`docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/`.

This gate therefore does not wait on `WCX-09`. The clarification records a
scope boundary only; it changes no exit-evidence item and no gate status.

Two implementation defects in the delivered `P1-W11` console are recorded in
`WCX-000` section 4 as `GAP-1`, the operator block including sign-out being
hidden below 900 pixels, and `GAP-2`, the absence of any error boundary. They
are scheduled in `WCX-10` and `WCX-04`. Whether either must close before this
gate is closed remains a Product Owner decision; nothing in the planning set
asserts that it must.

## Work-package evidence

| Package | Issue | PR | Acceptance state |
|---|---|---|---|
| P1-W1 | [#26](https://github.com/GuardianPot/guardian/issues/26) | Pending | Ready to implement |
| P1-W2 | [#30](https://github.com/GuardianPot/guardian/issues/30) | Pending | Blocked by dependencies |
| P1-W3 | [#27](https://github.com/GuardianPot/guardian/issues/27) | Pending | Blocked by dependencies |
| P1-W4 | [#32](https://github.com/GuardianPot/guardian/issues/32) | Pending | Blocked by dependencies |
| P1-W5 | [#35](https://github.com/GuardianPot/guardian/issues/35) | Pending | Blocked by dependencies |
| P1-W6 | [#34](https://github.com/GuardianPot/guardian/issues/34) | Pending | Blocked by dependencies |
| P1-W7 | [#31](https://github.com/GuardianPot/guardian/issues/31) | Pending | Ready to implement |
| P1-W8 | [#29](https://github.com/GuardianPot/guardian/issues/29) | Pending | Blocked by dependencies |
| P1-W9 | [#28](https://github.com/GuardianPot/guardian/issues/28) | Pending | Blocked by dependencies |
| P1-W10 | [#36](https://github.com/GuardianPot/guardian/issues/36) | Pending | Blocked by dependencies |
| P1-W11 | [#33](https://github.com/GuardianPot/guardian/issues/33) | Pending | Blocked by dependencies |

## Gate authority

CI and agents may record evidence but cannot mark this gate `APPROVED` or
`CLOSED`. Only the Product Owner can do so.

## Current status

`IN PROGRESS — WAVE 1 READY`
