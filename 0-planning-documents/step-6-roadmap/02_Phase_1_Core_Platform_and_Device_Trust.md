# Phase 1 — Core Platform & Device Trust
## Final Approved Implementation Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Faz amacı

Control Plane, Web Console ve Edge Agent arasında **güvenli, durable, observable yönetim döngüsünü** kurmak.

Phase 1 sonunda:
- operator login olabilir,
- environment oluşturabilir,
- Edge enroll olabilir,
- mTLS channel kurulur,
- desired-state gönderilebilir,
- Edge observed-state döndürür,
- local persistence/offline state vardır,
- health UI temel platform durumunu gösterir.

Henüz attacker-facing decoy portfolio tamamlanmış değildir.

# 2. Upstream dependencies

Required:
- Phase 0 PKI spike
- Phase 0 SQLite spike
- Phase 0 contract skeleton
- monorepo CI
- reproducible lab

# 3. Workstreams

## P1-W1 — Control Plane service shell

Implement Go modular monolith skeleton modules:
- auth
- environment
- devices
- deception config placeholder
- health
- audit
- jobs/outbox
- API

Database:
- PostgreSQL migrations
- explicit SQL access
- transaction boundaries
- audit append records

Acceptance:
- fresh DB migration works,
- downgrade/rollback strategy documented,
- service restart preserves state.

## P1-W2 — Local user authentication

Implement:
- one-time admin bootstrap
- Argon2id password storage
- server-side sessions
- secure cookie
- TOTP enrollment/verification
- login throttling
- auth audit

Acceptance:
- no default password,
- session secret not stored client-side,
- TOTP required for privileged owner/admin flow,
- failed login audit entries.

Step 5 trace: AUTH-01..03, AC-UX prerequisites.

## P1-W3 — Environment domain

Implement:
- organization singleton
- environment create/read/update
- zone/CIDR model
- owner-visible status
- environment audit history

Acceptance:
- at least 2 private IPv4 zones definable,
- invalid/overlapping dangerous config validated according policy,
- no arbitrary active scan triggered merely by save.

Traceability: AC-ON-003.

## P1-W4 — Edge enrollment

Production implementation of Phase 0 PKI:
- enrollment token API
- Edge key generation
- CSR
- certificate issuance
- device record
- connection status
- token revoke/use-once
- device revoke

Acceptance: AC-ON-001, AC-ON-002, AC-SEC-005, AC-SEC-006.

## P1-W5 — gRPC device channel

Implement:
- Edge outbound connection over 443-compatible deployment profile
- mTLS auth
- reconnect/backoff
- heartbeat
- protocol version negotiation
- desired-state stream
- observed-state/status stream
- bounded message sizes

Acceptance:
- Control restart reconnects,
- invalid cert rejected,
- Edge protocol incompatibility creates explicit health condition.

## P1-W6 — Desired-state reconciler skeleton

Control Plane stores revisioned desired state.
Edge stores:
- desired revision
- observed revision
- conditions
- reconciliation attempts

Initially supported objects:
- Edge configuration
- network zone metadata
- placeholder decoy desired object

Acceptance:
- replaying same desired revision is idempotent,
- failed reconcile reports reason rather than pretending healthy,
- last-known-good persists across restart.

## P1-W7 — Edge daemon production skeleton

Components:
- main unprivileged agent
- local SQLite store
- telemetry spool interfaces
- config reconciler
- health reporter
- privileged-helper client
- local diagnostics CLI

Acceptance:
- Edge restart recovers local state,
- DB corruption test has explicit failure state/recovery path,
- no silent fallback to insecure enrollment.

## P1-W8 — Privileged helper

Implement typed UDS RPC for only approved privileged operations:
- address manipulation
- nftables policy application
- container lifecycle hook (initial)
- network namespace operations

Security:
- peer credential validation
- no shell string API
- allowlisted paths/interfaces
- audit

Acceptance: approved decision SEC-04 and Phase 1 privileged-helper security
tests. AC-SEC-004 remains in Phase 2 primary validation and Phase 5 final
regression for unsigned/tampered OCI artifact rejection (change proposal 0002).

## P1-W9 — Platform health model

Conditions:
- Edge connected
- cert valid
- config converged
- local DB healthy
- spool healthy
- clock quality
- container runtime reachable
- privileged helper reachable

Web Console:
- environment health page
- Edge status
- actionable degraded reason

Acceptance:
- intentionally stop helper/runtime and verify precise degraded condition.

## P1-W10 — Audit model

Audit:
- auth
- enrollment/revocation
- environment update
- desired-state changes
- security settings

Acceptance:
- actor/time/object/before-after references,
- audit records cannot be edited through product API.

## P1-W11 — Initial Web Console shell

Implement:
- login/MFA
- environment screen
- Edge enrollment wizard
- device health screen
- zone definition
- no fake/mock “green” health disconnected from backend truth

Browser E2E:
login → create env → token → simulated/real Edge enroll → healthy.

# 4. Non-goals

- real decoy deployment completeness
- incident correlation
- AI
- email/webhook
- update/rollback production flow
- self-service no-support guarantee
- multi-edge acceptance

# 5. Exit gate

- [ ] authentication + TOTP
- [ ] environment/zone CRUD
- [ ] Edge enrollment and revocation
- [ ] production mTLS channel
- [ ] desired/observed state reconciliation
- [ ] SQLite durable local state
- [ ] privileged helper security tests
- [ ] platform health UI
- [ ] audit baseline
- [ ] browser onboarding skeleton E2E
- [ ] AC-ON-001/002/003 and AC-SEC-005/006 pass
- [ ] restart/reconnect smoke passes

# 6. Product state after phase

System is a **secure connected edge-control platform**. It is not yet a useful deception product; Phase 2 adds attacker-facing capability and evidence.

---

## Final Phase Status

- **Phase:** Phase 1 — Core Platform & Device Trust
- **Status:** APPROVED / FINAL
- **Owner decision:** Tüm bu faz kapsamı, sequencing, dependencies, acceptance criteria ve exit gate'leri onaylandı.
- **Change policy:** Bu fazın kapsamı implementation sırasında sessizce genişletilemez/daraltılamaz; değişiklik Product Owner change-control sürecine tabidir.
