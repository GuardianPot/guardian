# Step 7 — Repository Setup & AI-Agent Development Workflow
## Final Approved Engineering Governance Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Implementation authority:** Henüz verilmedi. Bu dosyalar onaylanmadan GitHub repository/settings/permissions oluşturulmaz veya değiştirilmez.  
> **Development-time estimate:** Kullanılmaz.  
> **Bağlayıcı girdiler:** Step 2–6 APPROVED/FINAL çıktıları.  
> **Temel kural:** AI agent kod yazabilir ve değişiklik önerebilir; product, architecture, security-boundary ve release-authority kararlarını onaylayamaz.


# 1. Step 7'nin amacı

Step 2–6 sonunda:
- product definition,
- threat model,
- feasibility requirements,
- architecture/technology baseline,
- MVP scope/acceptance criteria,
- MVP + post-MVP roadmap

kapanmıştır.

Step 7'nin sorusu artık:

> **Bu onaylanmış sistemi GitHub üzerinde hangi engineering-governance modeliyle, hangi repository/security ayarlarıyla ve hangi AI coding-agent execution protokolüyle güvenli biçimde geliştireceğiz?**

Step 7 product scope seçmez ve architecture yeniden tasarlamaz.

# 2. Step 7'nin final çıktıları

Onay sonrası finalde aşağıdaki dosya seti repository'nin engineering source-of-truth'u olacaktır:

1. `Repository_and_Engineering_Governance.md`
2. `GitHub_Repository_Topology_and_Settings.md`
3. `Branch_PR_Review_and_CODEOWNERS_Policy.md`
4. `AI_Coding_Agent_Strategy_and_Permissions.md`
5. `Agent_Context_and_Work_Package_Protocol.md`
6. `ADR_and_Architecture_Change_Control.md`
7. `CI_CD_Testing_and_Quality_Gates.md`
8. `Secrets_Signing_and_Release_Security.md`
9. `GitHub_Issues_Projects_and_Execution_Model.md`
10. `Phase_0_Execution_Plan.md`
11. `Repository_Bootstrap_and_Step_7_Exit_Checklist.md`
12. `External_Research_and_Constraints.md`

# 3. Önerilen execution operating model

```text
APPROVED ROADMAP
      ↓
Approved Work Package Spec
      ↓
GitHub Issue / Project Item
      ↓
AI Agent Task
      ↓
short-lived branch / isolated worktree
      ↓
Pull Request
      ↓
CI + Security + Policy checks
      ↓
AI independent review (supporting)
      ↓
Owner review where required
      ↓
Squash merge by authorized human
      ↓
Acceptance evidence
      ↓
Work Package DONE
      ↓
Phase Gate
      ↓
Product Owner phase approval
```

# 4. Executive recommendation

**APPROVED baseline:**

- Dedicated GitHub Organization
- One private monorepo
- GitHub Team-class organization plan if needed for private-repo protections; workflow must not depend on private-environment required-reviewer features that may require a higher plan
- `main` only long-lived branch
- trunk-based development
- GitHub Rulesets instead of relying only on classic branch protection
- squash merge only
- direct push/force push/delete to `main` blocked
- sensitive paths require CODEOWNER approval
- `AGENTS.md` canonical vendor-neutral agent policy
- thin `CLAUDE.md`, `GEMINI.md`, `.github/copilot-instructions.md` adapters
- Codex cloud as primary implementation agent
- Claude Code as optional but approved independent secondary reviewer/escalation agent
- GitHub Copilot cloud agent as GitHub-native fallback/optional reviewer
- Gemini CLI as local/sandboxed fallback/research tool
- AI agents never receive repository admin, production deployment, production signing or secret-management authority
- work package Markdown = specification source-of-truth
- GitHub Issue = execution/status object
- GitHub Project = portfolio/phase view
- GitHub Actions = CI
- privileged network/KVM tests = ephemeral isolated runner
- Actions pinned to full commit SHA
- default `GITHUB_TOKEN` = read-only; per-job least privilege
- Renovate = dependency-update orchestrator
- Dependabot alerts/security updates = enabled as an additional GitHub-native vulnerability signal
- production signing/root trust never placed in agent environment
- phase gates require human/Product Owner approval

# 5. Step 7 karar kümeleri

| Prefix | Alan |
|---|---|
| GOV | Engineering governance |
| GHR | GitHub repository/topology/settings |
| BR | Branching/rulesets/merge |
| CO | CODEOWNERS/path protection |
| AG | Coding-agent strategy |
| AP | Agent permissions/security |
| CTX | Agent context/instructions |
| WP | Work-package protocol |
| ADR | Architecture decision/change control |
| CI | CI/test/quality gates |
| SEC | Secrets/release/supply-chain |
| PM | Issues/Projects/work management |
| P0 | Phase 0 execution |
| EXIT | Step 7 exit |

# 6. Karar governance

Bütün yeni kararlar:

`OPEN → RESEARCHED → RECOMMENDED → OWNER DECISION → APPROVED`

Step 7 kapsamındaki tüm material kararlar Product Owner tarafından APPROVED edilmiştir.

# 7. Sıralama

## Step 7A — Decision Closure
Bu draft seti owner tarafından incelenir.

## Step 7B — GitHub Bootstrap
Owner onayı sonrası:
- organization/repository oluşturulur,
- settings/rulesets uygulanır,
- docs/policies commit edilir,
- CI skeleton kurulur.

## Step 7C — Phase 0 Activation
- P0 work packages GitHub Issues'a dönüştürülür,
- Project oluşturulur,
- dependency graph/status fields kurulur,
- agent environment bağlanır.

## Step 8 — Implementation Execution
İlk kod/spike execution:
- P0-W1 → P0-W10
- Phase 0 exit gate
- sonra Phase 1.

# 8. Master kararlar

## Karar GOV-01 — Step 7 repository oluşturulmadan önce kapanmalı mı?
**A:** Repo hemen açılır, governance sonra eklenir.  
**B:** Önce governance kararları kapanır, sonra repo bootstrap yapılır.

**APPROVED DECISION — B.**

Gerekçe: branch protection, agent permission ve instruction modelini sonradan eklemek ilk commit/agent davranışını kontrolsüz bırakır.

## Karar GOV-02 — Repository source-of-truth hangi kapsamı içerir?
**A:** Yalnız code.  
**B:** Code + architecture/product/roadmap/engineering governance.

**APPROVED DECISION — B.**

Kararların koddan ayrı yerde kalması AI agent'larda context drift yaratır.

## Karar GOV-03 — AI-generated change için human merge authority zorunlu mu?
**APPROVED DECISION — YES.**

Agent PR oluşturabilir/güncelleyebilir; `main` merge veya release trigger authority taşımaz.

## Karar GOV-04 — Phase gate otomatik kapanabilir mi?
**APPROVED DECISION — NO.**

CI “technical conditions satisfied” gösterebilir. Phase status `APPROVED/CLOSED` yalnız Product Owner kararıdır.

## Karar GOV-05 — Step 7'de production infrastructure deploy edilecek mi?
**APPROVED DECISION — NO.**

Step 7 engineering operating system kurar. Product environment/deploy execution Phase 0+ work package'larıyla yapılır.

# 9. Owner Decision Result

Product Owner bu Step 7 çalışma setindeki tüm önerileri onaylamıştır.

GitHub organization ve repository isimleri **execution-time owner input** olarak henüz verilmemiştir; bu durum governance karar açığı değildir ancak gerçek repository bootstrap'ının başlaması için çözülmelidir.

# 10. Step 7 Decision Closure

Step 7'nin **decision/governance design** kısmı CLOSED / APPROVED durumundadır.

- Repository governance: APPROVED
- GitHub topology/settings policy: APPROVED
- Branch/PR/Rulesets/CODEOWNERS: APPROVED
- Coding-agent strategy: APPROVED
- Agent permissions/security: APPROVED
- Work-package/context protocol: APPROVED
- ADR/change control: APPROVED
- CI/CD/testing gates: APPROVED
- Secrets/signing/release security: APPROVED
- Issues/Projects execution model: APPROVED
- Phase 0 GitHub execution plan: APPROVED
- Explicit decision count: 240
- Open governance owner decisions: 0

## Execution inputs still required

These are not unresolved governance choices; they are concrete bootstrap values/actions:

1. GitHub organization name.
2. GitHub repository name.
3. Confirmation of actual GitHub plan/features available.
4. Confirmation/connection of primary Codex GitHub access.
5. Optional confirmation/connection of Claude Code secondary reviewer access.

## Next action

Once the organization/repository names are supplied, Step 7B begins:

> **Create GitHub organization/repository → apply approved governance → commit Step 2–7 source-of-truth docs → configure Rulesets/CODEOWNERS/CI/Project → create Phase 0 issues → run protection acceptance tests.**

Only after the bootstrap checklist passes does development execution (Phase 0) begin.
