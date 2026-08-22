# Phase 0 GitHub Execution Plan
## Step 7 Final Approved Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Implementation authority:** Henüz verilmedi. Bu dosyalar onaylanmadan GitHub repository/settings/permissions oluşturulmaz veya değiştirilmez.  
> **Development-time estimate:** Kullanılmaz.  
> **Bağlayıcı girdiler:** Step 2–6 APPROVED/FINAL çıktıları.  
> **Temel kural:** AI agent kod yazabilir ve değişiklik önerebilir; product, architecture, security-boundary ve release-authority kararlarını onaylayamaz.


# 1. Goal

Step 6 Phase 0 workstreams become concrete GitHub execution objects after Step 7 approval.

No Phase 0 coding starts from this draft.

# 2. Dependency order

```text
P0-W1 Monorepo Bootstrap
  ├─→ P0-W2 CI Foundation
  ├─→ P0-W3 ADR System
  └─→ P0-W4 Reproducible Lab
            ├─→ P0-W5 Routed Networking Spike
            ├─→ P0-W6 containerd Spike
            │      └─→ P0-W7 Cowrie Adapter Spike
            ├─→ P0-W8 SQLite Replay Spike
            └─→ P0-W9 Device PKI Spike

P0-W10 Canonical Contract Skeleton
  starts after W1,
  iterates alongside W5–W9,
  finalizes after spike evidence.
```

W5/W6/W8/W9 can execute in parallel after lab prerequisites.

# 3. Work package issue catalog

## Karar P0-01 — P0-W1 executor
**APPROVED DECISION:** primary coding agent + owner review because repository foundation controls future agents.

### Issue
`[P0-W1] Bootstrap monorepo structure and development commands`

Scope:
- approved directories
- toolchain skeleton
- no product feature implementation
- initial AGENTS/engineering docs references.

Required evidence:
- clean checkout build
- commands documented
- no secret.

Human gate: required.

---

## Karar P0-02 — P0-W2 executor
**APPROVED DECISION:** agent implementation, owner review on `.github/workflows/**`.

Issue:
`[P0-W2] Establish baseline CI and repository policy checks`

Scope:
- Go/TS skeleton checks
- proto/openapi
- secret/dependency checks
- SHA pin policy
- minimal token permissions.

Human gate: mandatory due workflow authority.

---

## Karar P0-03 — P0-W3 executor
Issue:
`[P0-W3] Establish ADR and architecture change-control system`

Agent may draft templates/index, owner must approve statuses/rules.

---

## Karar P0-04 — P0-W4 executor
Issue:
`[P0-W4] Build reproducible isolated network lab`

Scope:
- Edge VM
- attacker host
- two routed zones
- deterministic reset.

Privileged disposable runner/local lab required.

---

## Karar P0-05 — P0-W5 executor
Issue:
`[P0-W5] Validate routed secondary-IP deception placement`

Risk: High / network/security.

Allowed areas:
- Edge network spike package
- lab
- tests
- ADR proposal.

Stop if architecture cannot meet IP conflict/cleanup/egress requirements.

Owner reviews spike conclusion before architecture declared validated.

---

## Karar P0-06 — P0-W6 executor
Issue:
`[P0-W6] Validate containerd decoy lifecycle and isolation`

Risk: High.

Must prove:
- pull by digest
- lifecycle idempotency
- resource limits
- no socket inside decoy
- cleanup.

---

## Karar P0-07 — P0-W7 executor
Issue:
`[P0-W7] Validate Cowrie SSH medium-interaction adapter`

Dependency: W6 runtime harness substantially working.

Must produce:
- pinned upstream provenance
- login/session/command canonical fixture
- hostile input
- egress test
- license/security notes.

---

## Karar P0-08 — P0-W8 executor
Issue:
`[P0-W8] Validate SQLite WAL durable replay and crash behavior`

Can run mostly unprivileged in parallel.

Must prove stable event IDs and retry semantics.

---

## Karar P0-09 — P0-W9 executor
Issue:
`[P0-W9] Validate device enrollment, mTLS rotation and revocation`

Security-sensitive CODEOWNER review mandatory.

No real production CA keys.

---

## Karar P0-10 — P0-W10 executor
Issue:
`[P0-W10] Establish versioned canonical device and telemetry contract skeleton`

Owner review due `/proto/**`.

Agent must not over-model Phase 1–5 features before spike evidence.

# 4. Agent allocation strategy

## Karar P0-11 — Parallelism
**APPROVED DECISION:**
After W1/W4 prerequisites, run max independent agents on W5/W6/W8/W9, but one package per isolated worktree/environment.

No two implementation agents edit shared canonical contract simultaneously without coordination.

## Karar P0-12 — Contract owner
P0-W10 serves as integration contract owner; other agents propose field needs rather than independently breaking schema.

## Karar P0-13 — Independent review
For W5/W6/W9:
- primary agent produces spike,
- secondary agent reviews threat/failure assumptions,
- owner decides pass/change proposal.

## Karar P0-14 — Spike success semantics
“Code works once” is insufficient.

Must include:
- reproducible steps,
- automated fixture where possible,
- failure injection,
- limitations,
- architecture conclusion.

# 5. Proposed Phase 0 Project items

| ID | Risk | Dependency | Primary component | Sensitive review |
|---|---|---|---|---|
| P0-W1 | Medium | none | repo | yes |
| P0-W2 | High | W1 | CI | yes |
| P0-W3 | Medium | W1 | docs/governance | owner |
| P0-W4 | High | W1 | lab | security |
| P0-W5 | High | W4 | edge/network | security |
| P0-W6 | High | W4 | edge/runtime | security |
| P0-W7 | High | W6 | decoy/SSH | security |
| P0-W8 | Medium | W1/W4 | edge/storage | normal+AC |
| P0-W9 | Critical | W4 | PKI/device | owner/security |
| P0-W10 | High | W1 + spike input | contracts | owner |

# 6. Phase 0 gate

No agent can mark Phase 0 complete.

Gate document:
`docs/phase-gates/phase-0.md`

Evidence links:
- Issues
- PRs
- CI runs
- lab reports
- ADRs/change proposals.

Product Owner changes gate status to APPROVED/CLOSED after all Step 6 Phase 0 exit criteria satisfied.

---

## Final Document Status

- **Decision status:** APPROVED / FINAL
- **Authority:** Product Owner
- **Autonomous agent authority to change this policy:** NONE
- **Change control:** Any material modification requires an explicit governance/change proposal and Product Owner approval.
