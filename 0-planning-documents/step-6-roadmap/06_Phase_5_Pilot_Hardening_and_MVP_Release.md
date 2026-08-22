# Phase 5 — Pilot Hardening & MVP Release
## Final Approved Implementation Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Faz amacı

Phase 0–4'te çalışan ürün dilimini **controlled pilot-ready MVP** seviyesine yükseltmek.

Bu faz yeni product breadth ekleme fazı değildir. Focus:
- security hardening
- signed updates
- failure recovery
- performance
- diagnostics/supportability
- backup/restore
- release gates
- end-to-end acceptance

Phase 5 exit = **MVP COMPLETE**.

# 2. Dependencies

All Phase 0–4 exit gates.

Step 5 blocker spikes additionally:
- SP-05 PostgreSQL partition benchmark
- SP-08 TUF + Cosign flow
- SP-09 dual-provider schema compatibility (Phase 4)
- SP-10 prompt injection corpus (Phase 4)

# 3. Workstreams

## P5-W1 — Artifact signing

Implement:
- signed Edge release
- signed decoy OCI image
- digest pin
- verification before activation
- test/prod key separation
- audit of verification

Acceptance: AC-SEC-004, AC-UP-001/002.

## P5-W2 — TUF-style update metadata

Implement:
- version metadata
- expiry
- rollback protection
- revoked version handling
- staged download
- verify before switch

Acceptance: AC-SEC-010.

## P5-W3 — Edge updater and rollback

Separate privileged updater:
- stage
- signature/metadata verify
- atomic activation
- restart
- health check
- rollback to known-good

Acceptance: AC-UP-001..003.

## P5-W4 — Decoy pack update

Desired-state digest update:
- pull
- verify
- stop/start controlled
- health
- rollback/previous digest manual or policy path
- visibility

Acceptance: AC-UP-004.

## P5-W5 — Hostile-data hardening

Test/implement:
- HTML/JS/Markdown/ANSI safe rendering
- URL/path escaping
- oversized HTTP
- malformed protocol/log
- object quarantine
- diagnostics redaction

Acceptance: AC-SEC-007..009.

## P5-W6 — Edge/management isolation review

Verify:
- private key inaccessible
- runtime socket inaccessible
- UDS access control
- helper allowlist
- no production secrets
- container capabilities/seccomp/no-new-privileges
- egress deny

Acceptance: AC-SEC-001..003.

## P5-W7 — Resilience suite

Automate:
- CP disconnect
- Edge restart
- decoy crash
- CP restart
- PostgreSQL restart
- AI timeout
- disk pressure
- cert rotation failure
- telemetry flood

Acceptance: FM decisions and AC-HL-001..005.

## P5-W8 — Queue/disk pressure policy

Implement:
- queue byte/event thresholds
- warnings
- priority classes
- explicit loss state
- no silent high-value evidence drop

Reference: >=100k small events or >=1GiB configurable spool.

## P5-W9 — Performance benchmark

Reference environment:
- 1 org
- 1 Edge
- 2–3 zones
- up to 32 decoys
- 4 families

Targets:
- 100 normalized eps sustained
- 500 eps × 60s burst
- incident visibility <=5s p95 excluding AI
- common API <1s p95
- decoy convergence <=60s normal cached artifact
- AI <=30s p95 UX target, non-blocking

Results versioned.

## P5-W10 — PostgreSQL storage benchmark/partition readiness

Prove:
- reference 30-day dataset simulation
- indexes
- time partitions where justified
- incident list/detail latency
- ingest during query load
- retention/purge behavior

If threshold fails, tune within approved Postgres architecture; do not introduce ClickHouse without change-control.

## P5-W11 — Diagnostics bundle

Redacted bundle:
- versions
- config summary
- health
- Edge queue
- logs
- recent errors
- no passwords/private keys/session/AI secrets

Acceptance: OPS-04.

## P5-W12 — Backup/restore smoke

At minimum:
- central PostgreSQL state restore
- object references consistency
- config/evidence/incident preserved
- secret store recovery procedure documented
- disposable decoy state not restored as truth

## P5-W13 — Full browser E2E

Automated:
login/TOTP → env → enroll Edge → define zones → deploy decoys → test attack → incident → journey → AI → notification → disposition.

## P5-W14 — Security review

Manual/internal:
- trust boundaries
- helper
- container isolation
- web session/CSP/CSRF
- hostile rendering
- prompt injection
- update flow
- secret/log review

External pentest approved before wider beta/GA, not first controlled pilot blocker.

## P5-W15 — Pilot runbook

Document:
- prerequisites
- installation
- network requirements
- onboarding
- safe test attack
- known limitations
- rollback
- diagnostic collection
- incident response if product itself appears compromised
- support escalation

# 4. Final MVP release gate

Must pass Step 5:
- Product gates
- Detection gates
- Security gates
- Reliability gates
- AI gates
- Performance gates
- Operations gates

No waived gate is silent. Any owner waiver:
- documented
- scoped
- rationale
- expiry/revisit condition

# 5. Exit criteria

- [ ] all Step 5 69 ACs relevant to MVP pass
- [ ] all MVP release gates pass or explicit owner waiver
- [ ] no unresolved critical/high security issue absent owner waiver
- [ ] all blocker spikes closed
- [ ] end-to-end North Star passes in reference lab
- [ ] controlled pilot installation runbook tested
- [ ] update + rollback tested
- [ ] backup/restore smoke tested
- [ ] diagnostics tested
- [ ] performance targets recorded
- [ ] explicit non-goals present in release notes
- [ ] pilot metrics instrumentation present

# 6. Result

**Phase 5 exit is the first point where the project may be labeled controlled pilot-ready MVP.**

No post-MVP capability is required to claim MVP completion.

---

## Final Phase Status

- **Phase:** Phase 5 — Pilot Hardening & MVP Release
- **Status:** APPROVED / FINAL
- **Owner decision:** Tüm bu faz kapsamı, sequencing, dependencies, acceptance criteria ve exit gate'leri onaylandı.
- **Change policy:** Bu fazın kapsamı implementation sırasında sessizce genişletilemez/daraltılamaz; değişiklik Product Owner change-control sürecine tabidir.
