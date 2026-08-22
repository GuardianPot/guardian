# ADR & Architecture Change Control
## Step 7 Final Approved Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Implementation authority:** Henüz verilmedi. Bu dosyalar onaylanmadan GitHub repository/settings/permissions oluşturulmaz veya değiştirilmez.  
> **Development-time estimate:** Kullanılmaz.  
> **Bağlayıcı girdiler:** Step 2–6 APPROVED/FINAL çıktıları.  
> **Temel kural:** AI agent kod yazabilir ve değişiklik önerebilir; product, architecture, security-boundary ve release-authority kararlarını onaylayamaz.


# 1. Goal

Step 4'te 202 architecture/technology decision APPROVED. Bunları 202 ayrı ADR'ye kopyalamak yerine **major architectural rationale + future changes** için sürdürülebilir ADR sistemi gerekir.

# 2. ADR strategy

## Karar ADR-01 — Every Step 4 decision = ADR?
**APPROVED DECISION — NO.**

Master architecture document authoritative decision register olarak kalır.

## Karar ADR-02 — Foundational ADR set
**APPROVED DECISION:** 15–25 high-impact architectural ADR hazırlanır/import edilir.

Candidate ADRs:
1. modular monolith Control Plane
2. Control/Edge decomposition
3. Go backend/edge
4. React SPA
5. native Edge + privilege helper
6. OCI decoy/containerd
7. QEMU/KVM high interaction
8. gRPC/Protobuf/mTLS device plane
9. REST/OpenAPI UI/integration
10. PostgreSQL central
11. SQLite Edge spool/state
12. no broker initially
13. deterministic correlation + AI reasoning
14. provider-neutral AI gateway
15. monorepo
16. signed update/Cosign+TUF
17. OpenTelemetry
18. default-deny deception egress
19. product-specific device PKI
20. evidence/provenance model.

## Karar ADR-03 — ADR location
`docs/adr/ADR-0001-*.md`.

## Karar ADR-04 — ADR status
- Proposed
- Accepted
- Rejected
- Superseded
- Deprecated.

Only Product Owner can change Proposed→Accepted for material architecture decisions.

## Karar ADR-05 — ADR immutability
Accepted ADR historical rationale is not rewritten to pretend history changed. New ADR supersedes old.

# 3. Change proposal triggers

## Karar ADR-06 — New deployable component
Requires ADR/change proposal.

## Karar ADR-07 — New database/broker/cache/search engine
Requires ADR.

## Karar ADR-08 — Language/framework/runtime change
Requires ADR.

## Karar ADR-09 — Wire protocol/schema compatibility policy change
Requires ADR.

## Karar ADR-10 — Trust boundary/privilege change
Requires ADR + security review.

## Karar ADR-11 — Decoy egress policy weakening
Requires owner approval + threat model update.

## Karar ADR-12 — AI tool/action authority
Requires product + architecture + security decision; normal feature PR cannot do it.

## Karar ADR-13 — New production external service/vendor dependency
Requires ADR if it changes availability/privacy/security/operations.

## Karar ADR-14 — New GitHub/agent permission
Requires engineering governance change approval.

## Karar ADR-15 — Breaking public/device API
Requires ADR/compatibility plan.

# 4. Change proposal issue

## Karar ADR-16 — GitHub Issue type
`type:change-proposal`.

Fields:
- current decision/ADR
- discovered problem
- evidence
- options
- approved option
- security/product/ops impact
- migration
- rollback
- affected work packages.

Agent may create this issue but cannot mark accepted.

# 5. Technical discovery

## Karar ADR-17 — Spike can invalidate architecture
**APPROVED DECISION:** spike conclusion = evidence, not automatic architecture change.

Workflow:
```text
Spike fails assumption
→ change proposal
→ research/options
→ owner decision
→ ADR
→ architecture doc update
→ work package re-plan
```

# 6. Decision traceability

## Karar ADR-18
Maintain `docs/architecture/decision-index.md` mapping:
- Step 2/3/4 decision IDs
- ADRs
- work packages
- code modules/contracts.

## Karar ADR-19 — AI agent instruction
Agent must cite relevant ADR/decision in non-trivial PR.

# 7. Emergency security exception

## Karar ADR-20
A security patch may temporarily disable/contain capability without full architecture review when needed to prevent harm.

But:
- no silent new architecture,
- emergency change logged,
- owner informed,
- follow-up ADR/change review mandatory.

# 8. Acceptance

A PR introducing NATS, Redis, new root privilege, new AI tool or new externally reachable management port without approved ADR/change proposal must fail governance review.

---

## Final Document Status

- **Decision status:** APPROVED / FINAL
- **Authority:** Product Owner
- **Autonomous agent authority to change this policy:** NONE
- **Change control:** Any material modification requires an explicit governance/change proposal and Product Owner approval.
