# Branch, Pull Request, Rulesets & CODEOWNERS Policy
## Step 7 Final Approved Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Implementation authority:** Henüz verilmedi. Bu dosyalar onaylanmadan GitHub repository/settings/permissions oluşturulmaz veya değiştirilmez.  
> **Development-time estimate:** Kullanılmaz.  
> **Bağlayıcı girdiler:** Step 2–6 APPROVED/FINAL çıktıları.  
> **Temel kural:** AI agent kod yazabilir ve değişiklik önerebilir; product, architecture, security-boundary ve release-authority kararlarını onaylayamaz.


# 1. Branch model

## Karar BR-01 — Branching model

### A — GitFlow
`main/develop/release/hotfix`.
### B — Trunk-based
`main` + short-lived branches.
### C — Direct main.

**APPROVED DECISION — B.**

Agent-parallel development'ta long-lived integration branch drift yaratır.

## Karar BR-02 — Long-lived branches
**APPROVED DECISION:** only `main`. Release maintenance branch yalnız gerçek supported release need doğarsa.

## Karar BR-03 — Branch naming

**APPROVED patterns:**
- `agent/P0-W5-routed-networking`
- `human/P0-W3-adr-system`
- `fix/<issue>-slug`
- `dep/<package>-<version>`
- `proposal/<decision-id>-slug`

Branch name work-package/issue traceability taşımalı.

## Karar BR-04 — Direct push to main
**APPROVED DECISION — BLOCKED.**

## Karar BR-05 — Force push/delete main
**APPROVED DECISION — BLOCKED.**

# 2. Protection mechanism

## Karar BR-06 — Classic branch protection vs Rulesets

### A — Classic branch protection
Mature/simple.
### B — GitHub Rulesets
Layerable, auditable, multiple rules can apply, status controls.
### C — Both permanently
Potential confusion due most restrictive aggregation.

**APPROVED DECISION — B as primary.**

Temporary classic protection only bootstrap fallback if account plan/settings block rulesets.

## Karar BR-07 — PR required
**APPROVED DECISION — YES.**

## Karar BR-08 — Required status checks
**APPROVED DECISION — YES.**
Checks defined in CI policy.

## Karar BR-09 — Require branch up-to-date before merge
### A — Strict
More CI, lower stale-base risk.
### B — Loose
Fewer CI runs.

**APPROVED DECISION — Strict for security/contract-heavy monorepo initially.**

Review if agent concurrency causes excessive rebuild.

## Karar BR-10 — Conversation resolution
**APPROVED DECISION — REQUIRED.**

## Karar BR-11 — Linear history
**APPROVED DECISION — YES**, via squash merge only.

## Karar BR-12 — Merge queue
**APPROVED DECISION — NO initially.**
Trigger: sustained parallel-agent PR contention.

## Karar BR-13 — Signed commits required on main
**APPROVED DECISION — NO initially.**

Reason:
- squash merge + GitHub/UI/agent author semantics can complicate signature enforcement,
- supply-chain trust is artifact/release signing + PR/CI.
Signed commits remain future hardening option.

## Karar BR-14 — Required approving review for every PR
### A — 1 human approval every PR
Maximum control, high solo friction.
### B — No global approval; owner-only merge process + sensitive CODEOWNERS reviews
### C — AI approval satisfies gate

**APPROVED DECISION — B.**

AI approval never substitutes owner review on sensitive paths.

## Karar BR-15 — Sensitive path approval
**APPROVED DECISION — REQUIRED CODEOWNER review.**

## Karar BR-16 — Approval of most recent push
For sensitive CODEOWNER paths:

**APPROVED DECISION — require owner approval after most recent agent push** where GitHub plan/ruleset supports.

## Karar BR-17 — Stale reviews
**APPROVED DECISION:** sensitive path review dismissed/re-required after relevant code-modifying push.

## Karar BR-18 — Agent merge permission
**APPROVED DECISION — NONE.**
Agent can create/update PR branch; no merge/bypass permission.

## Karar BR-19 — Admin bypass
**APPROVED DECISION:** owner retains emergency bypass but routine use forbidden; bypass reason documented in issue/PR/audit.

# 3. Merge policy

## Karar BR-20 — Merge method
**APPROVED DECISION — Squash merge.**

Squash commit title:
`[P0-W5] implement routed networking spike (#123)`

## Karar BR-21 — Commit style
**APPROVED DECISION:** Conventional Commit-inspired for local branch commits, but PR title/work-package traceability is stronger requirement.

## Karar BR-22 — Auto-merge
**APPROVED DECISION — DISABLED for agent-authored implementation PRs.**

Dependency PR auto-merge can later be selectively enabled for patch-only low-risk dependencies after proven CI.

## Karar BR-23 — Draft PR use
**APPROVED DECISION:** agents open draft PR early for long work; `Ready for review` only after mandatory checks/tests/report complete.

# 4. CODEOWNERS

## Karar CO-01 — CODEOWNERS location
**APPROVED DECISION — `.github/CODEOWNERS`.**

GitHub recommends protecting CODEOWNERS itself; `.github` ownership supports this.

## Karar CO-02 — CODEOWNERS owner
Actual owner/team identifier requires GitHub org input.

**APPROVED DECISION:** dedicated `@org/owners` or `@org/security-owners` team; if solo, owner's GitHub handle.

## Karar CO-03 — Protect `.github/**`
**APPROVED DECISION — YES.**

Reason: workflows, issue forms and agent instructions can change execution/security.

## Karar CO-04 — Protect architecture/product docs
Paths:
- `/docs/product/**`
- `/docs/architecture/**`
- `/docs/roadmap/**`
- `/docs/adr/**`
- `/docs/phase-gates/**`

**APPROVED DECISION — YES.**

## Karar CO-05 — Protect agent instruction files
- `/AGENTS.md`
- `/CLAUDE.md`
- `/GEMINI.md`
- `/.github/copilot-instructions.md`
- `/.github/instructions/**`
- `/.claude/**` if committed
- `/.gemini/**` if committed

**APPROVED DECISION — YES.**

Critical reason: coding/review agents may load instructions from the PR head branch; an unreviewed instruction change can alter reviewer/agent behavior.

## Karar CO-06 — Protect security-sensitive code
- `/security/**`
- Edge privileged helper
- PKI/auth
- updater/signing
- network-policy code
- secrets abstractions
- AI prompts/schemas
- migrations affecting evidence/audit.

**APPROVED DECISION — YES.**

## Karar CO-07 — Protect contracts
- `/proto/**`
- `/openapi/**`
- `/schemas/**`

**APPROVED DECISION — YES.**

## Karar CO-08 — Protect dependency/runtime manifests
- `go.mod/go.sum`
- JS package/lock files
- Docker/Containerfiles
- `.github/workflows`
- decoy manifests requesting privilege.

**APPROVED DECISION:** review required when PR changes dependency/runtime authority; exact patterns refined at bootstrap.

# 5. Policy checker

## Karar CO-09 — Allowed-path enforcement
CODEOWNERS does not enforce work-package `allowed_paths`.

**APPROVED DECISION:** CI `work-package-policy` check:
- reads PR work-package ID,
- loads `docs/work-packages/<phase>/<id>.md` metadata,
- compares changed paths against allowed/forbidden globs,
- flags unexpected changes.

Unexpected path ≠ automatic unsafe code. It means work package must be amended/approved or PR split.

## Karar CO-10 — Protect work-package spec
Agent can update implementation notes, but cannot self-change `status`, `scope`, `allowed_paths`, `acceptance_refs`.

**APPROVED DECISION:** those sections owner/codeowner protected.

# 6. PR template

Required fields:
- Work package / Issue
- Decision refs
- Acceptance refs
- Summary
- Files/components
- New dependencies
- Contract/migration changes
- Security impact
- Tests/evidence
- Screenshots where UI
- Known limitations
- Change proposal required? yes/no

# 7. Acceptance

A test PR changing:
- workflow,
- AGENTS.md,
- proto contract,
- security policy

must request/require CODEOWNER review.

A normal isolated implementation PR must still pass status checks but need not have artificial “second human developer” approval.

---

## Final Document Status

- **Decision status:** APPROVED / FINAL
- **Authority:** Product Owner
- **Autonomous agent authority to change this policy:** NONE
- **Change control:** Any material modification requires an explicit governance/change proposal and Product Owner approval.
