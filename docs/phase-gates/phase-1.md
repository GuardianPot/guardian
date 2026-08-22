# Phase 1 gate

Phase 1 establishes a secure, durable, and observable management loop between
the Control Plane, Web Console, and Edge Agent. It is not complete until every
approved Phase 1 package has acceptance evidence and the Product Owner closes
this gate.

## Entry dependencies

- Phase 0 technical evidence is complete.
- Phase 0 gate is Product Owner approved.
- P1-W1 through P1-W11 specifications are Product Owner approved.
- Required development PostgreSQL and isolated Edge test environments exist.

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

## Work-package evidence

| Package | Issue | PR | Acceptance state |
|---|---|---|---|
| P1-W1 | Pending | Pending | Not started |
| P1-W2 | Pending | Pending | Not started |
| P1-W3 | Pending | Pending | Not started |
| P1-W4 | Pending | Pending | Not started |
| P1-W5 | Pending | Pending | Not started |
| P1-W6 | Pending | Pending | Not started |
| P1-W7 | Pending | Pending | Not started |
| P1-W8 | Pending | Pending | Not started |
| P1-W9 | Pending | Pending | Not started |
| P1-W10 | Pending | Pending | Not started |
| P1-W11 | Pending | Pending | Not started |

## Gate authority

CI and agents may record evidence but cannot mark this gate `APPROVED` or
`CLOSED`. Only the Product Owner can do so.

## Current status

`NOT STARTED — BLOCKED BY PHASE 0 GATE AND PACKAGE CONTENT APPROVAL`
