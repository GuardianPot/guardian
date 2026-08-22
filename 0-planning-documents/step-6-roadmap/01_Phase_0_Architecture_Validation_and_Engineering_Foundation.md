# Phase 0 — Architecture Validation & Engineering Foundation
## Final Approved Implementation Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Faz amacı

Kod tabanını feature geliştirmeye açmadan önce **MVP'yi teknik olarak taşıyacak en riskli substrate'ları kanıtlamak**.

Bu faz product feature üretme fazı değildir. Ama sonraki bütün fazların güvenli biçimde çalışabilmesi için gerekli:
- repository governance,
- contracts,
- reproducible lab,
- device trust,
- networking spike,
- container lifecycle,
- local durable buffering,
- core decoy adapter feasibility

kanıtlanır.

# 2. Neden ilk faz?

Step 5 blocker'ları:
- SP-01 routed secondary-IP networking
- SP-02 containerd lifecycle
- SP-03 Cowrie adapter
- SP-06 SQLite replay/crash
- SP-07 device PKI

MVP implementation'ın merkezindedir. Bunlardan biri başarısızsa feature geliştirmek teknik borç değil, yanlış substrate üzerinde ilerlemek olur.

# 3. Entry criteria

- Step 2–5 final decision docs mevcut.
- Monorepo strategy APPROVED.
- Architecture baseline APPROVED.
- MVP decoy portfolio APPROVED.
- No codebase assumptions that override ADR process.

# 4. Workstreams

## P0-W1 — Monorepo bootstrap

Implement:
- `/apps/control-plane`
- `/apps/web-console`
- `/apps/edge-agent`
- `/decoys`
- `/proto`
- `/openapi`
- `/schemas`
- `/ai`
- `/rules`
- `/security`
- `/tests`
- `/docs/adr`
- `/deploy`
- `/tools`

Required:
- Go workspace/module policy
- frontend workspace
- common Make/Task targets
- formatting/lint commands
- local developer bootstrap
- CODEOWNERS/critical path review policy
- PR template with decision/requirement IDs

Acceptance:
- clean checkout reproducibly builds empty/skeleton apps,
- one command validates lint/test/contracts,
- agent cannot introduce undocumented top-level app structure without review.

## P0-W2 — CI foundation

Implement GitHub Actions gates:
- Go format/vet/static analysis/test
- TypeScript lint/typecheck/test
- Protobuf lint
- Buf breaking check
- OpenAPI validation
- dependency/license inventory
- container build smoke
- security secret scanning
- generated artifact freshness checks

Acceptance:
- intentionally breaking Protobuf change fails CI,
- failing Go/TS test blocks merge,
- generated contracts cannot drift silently.

## P0-W3 — Architecture Decision Record system

Create:
- ADR template
- architecture change proposal template
- status semantics: Proposed / Accepted / Superseded
- owner approval field
- decision IDs back-reference

Acceptance:
- any new external datastore/message broker/runtime requires ADR,
- AI-generated ADR cannot self-mark Accepted.

## P0-W4 — Reproducible virtual network lab

Lab must model:
- Control Plane placeholder
- Debian 13 Edge VM
- attacker/test host
- at least 2 logical private routed zones
- addresses reserved for decoys
- deterministic reset

Approved implementation may use local virtualization/containers according to Step 4, but lab must not alter production architecture.

Acceptance:
- fresh lab rebuild repeatable,
- attacker host can route to planned decoy addresses,
- management path and deception path distinguishable.

Traceability: TEST-01.

## P0-W5 — Routed secondary-IP networking spike

Questions to prove:
- secondary address add/remove idempotent?
- reboot/reconcile behavior?
- IP conflict detection before bind?
- multiple decoy IPs?
- route/source selection?
- nftables default-deny egress?
- cleanup after failure?

Deliver:
- executable spike
- packet captures/test logs
- ADR confirmation or change proposal
- automated lab tests

Acceptance:
- repeated add/remove leaves no stale address/rule,
- occupied IP is not taken over,
- decoy source/destination behavior is deterministic,
- forbidden egress blocked.

Traceability: SP-01, AC-ON-004, AC-SEC-003.

## P0-W6 — containerd direct lifecycle spike

Prove:
- image pull by digest
- create/start/stop/delete
- namespace cleanup
- cgroup limits
- read-only rootfs/capability profile
- health check
- network namespace attachment
- crash restart semantics
- no runtime socket inside decoy

Acceptance:
- lifecycle is idempotent,
- failed create cleans partial resources,
- runtime restart does not orphan unmanaged container state,
- decoy cannot access containerd socket.

Traceability: SP-02, AC-SEC-002.

## P0-W7 — Cowrie adapter spike

Prove:
- pinned upstream version wrapped in OCI image
- SSH auth attempt normalization
- session/command capture
- canonical event mapping
- hostile input handling
- egress restriction
- health signal
- upstream file capture behavior documented

Acceptance:
- SSH connection/auth/command fixture produces stable canonical event samples,
- prompt-like command stays data,
- adapter crash does not crash Edge proof harness.

Traceability: SP-03, AC-SSH-001..005.

## P0-W8 — SQLite WAL replay/crash spike

Prove:
- append local event
- ack state
- crash before/after send
- restart
- at-least-once replay
- stable event ID
- bounded disk accounting

Acceptance:
- crash injection does not lose acknowledged durability boundary,
- resend produces same event ID,
- replay can be idempotently consumed by test server.

Traceability: SP-06, AC-EV-001, AC-HL-001/002.

## P0-W9 — Device PKI spike

Implement proof:
- one-time token
- Edge-local keypair
- CSR/request
- issued cert
- mTLS connection
- token one-time use
- cert rotation
- revocation

Acceptance:
- used token replay rejected,
- revoked cert cannot reconnect,
- private key never serialized through Control Plane API/logs.

Traceability: SP-07, AC-ON-001/002, AC-SEC-005/006.

## P0-W10 — Canonical contract skeleton

Define first stable contract packages for:
- device identity
- desired/observed state
- health conditions
- event envelope
- evidence provenance
- decoy manifest
- blob reference

Goal is contract skeleton, not final feature breadth.

Acceptance:
- generated Go clients compile,
- version namespace exists,
- breaking change gate works,
- fields support IPv6-safe address representation even though MVP runtime IPv4-first.

# 5. Explicit non-goals

- user-facing product completeness
- final incident UI
- AI
- notifications
- all four production decoy packs
- high interaction
- multi-edge
- self-service onboarding polish

# 6. Parallelization

Can run in parallel after repo bootstrap:
- networking spike
- containerd spike
- SQLite spike
- PKI spike
- Cowrie adapter spike

Contracts need representatives from all spikes; schema finalization follows observed needs.

# 7. Security requirements

- no production secrets in lab fixtures,
- no public internet decoy exposure by default,
- privileged experiments isolated to disposable lab,
- malicious SSH payload fixtures treated hostile,
- update/signing keys not required in this phase; test keys only.

# 8. Exit gate

Phase 0 complete only if:
- [ ] monorepo/CI operational
- [ ] ADR governance operational
- [ ] reproducible lab exists
- [ ] SP-01 pass or architecture change approved
- [ ] SP-02 pass or architecture change approved
- [ ] SP-03 pass or SSH design change approved
- [ ] SP-06 pass
- [ ] SP-07 pass
- [ ] canonical contract skeleton committed
- [ ] critical test fixtures version-controlled
- [ ] no unresolved blocker invalidating Step 4 architecture

# 9. Output state

At exit we have **validated substrate**, not a product demo. Phase 1 is allowed to build real Control Plane/Edge functions on these validated primitives.

---

## Final Phase Status

- **Phase:** Phase 0 — Architecture Validation & Engineering Foundation
- **Status:** APPROVED / FINAL
- **Owner decision:** Tüm bu faz kapsamı, sequencing, dependencies, acceptance criteria ve exit gate'leri onaylandı.
- **Change policy:** Bu fazın kapsamı implementation sırasında sessizce genişletilemez/daraltılamaz; değişiklik Product Owner change-control sürecine tabidir.
