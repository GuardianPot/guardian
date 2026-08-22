# GitHub Repository Topology & Settings
## Step 7 Final Approved Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Implementation authority:** Henüz verilmedi. Bu dosyalar onaylanmadan GitHub repository/settings/permissions oluşturulmaz veya değiştirilmez.  
> **Development-time estimate:** Kullanılmaz.  
> **Bağlayıcı girdiler:** Step 2–6 APPROVED/FINAL çıktıları.  
> **Temel kural:** AI agent kod yazabilir ve değişiklik önerebilir; product, architecture, security-boundary ve release-authority kararlarını onaylayamaz.


# 1. Hedef

GitHub'ın yalnız Git hosting değil, repository policy enforcement boundary olarak kullanılması.

# 2. Repository ownership/topology decisions

## Karar GHR-01 — Personal account mı dedicated GitHub Organization mı?

### A — Personal repository
- en az setup,
- küçük side-project için yeterli,
- team/role/policy büyümesi daha zayıf.

### B — Dedicated GitHub Organization
- role/policy/repository settings ayrımı,
- future collaborators/MSP/company transition daha temiz,
- Projects/organization policies için doğal boundary.

**APPROVED DECISION — B.**

## Karar GHR-02 — Repository visibility

### A — Public
OSS/community avantajı; security product source/attack surface'i açığa çıkarır.

### B — Private
Ürün stratejisi, decoy logic, security tests ve agent governance kontrollü kalır.

**APPROVED DECISION — B initially.**

Open-source kararı daha sonra belirli packs/libraries için ayrı verilebilir.

## Karar GHR-03 — Repository sayısı

**APPROVED DECISION:** One product monorepo. Step 4 RE-01'i uygular.

Third-party upstream Cowrie/OpenCanary kendi repository'lerinde kalır; bizim wrappers/adapters monorepo içindedir.

## Karar GHR-04 — GitHub plan dependency

**Seçenekler:** Free/Pro/Team/Enterprise.

**APPROVED DECISION:** Dedicated Organization için **GitHub Team veya capability-equivalent** baseline; ancak release design GitHub Enterprise'a özgü environment required-reviewer özelliğine zorunlu bağımlı yapılmaz.

Not: GitHub docs'a göre private/internal repo environment secrets Team/Pro'da kullanılabilirken, required reviewer gibi bazı deployment protection rules Free/Pro/Team'de public repo ile sınırlıdır. Bu nedenle security authority GitHub plan feature'ına tek noktadan bağlanmamalıdır.

## Karar GHR-05 — Actual organization name

**OWNER INPUT REQUIRED.**

Recommendation criteria:
- product/company name ile uyumlu,
- generic personal username'a bağlı olmayan,
- future legal entity migration'a uygun.

## Karar GHR-06 — Actual repository name

**OWNER INPUT REQUIRED.**

Recommendation:
- lowercase kebab-case,
- product identity,
- internal codename değilse uzun ömürlü,
- `-backend`, `-server` gibi component-specific suffix yok çünkü monorepo.

## Karar GHR-07 — Default branch

**APPROVED DECISION — `main`.**

## Karar GHR-08 — Issues

**APPROVED DECISION — ENABLED.**

Work package/status/bug/change proposal execution object.

## Karar GHR-09 — GitHub Projects

**APPROVED DECISION — ENABLED / organization-level project.**

No date/effort estimation fields.

## Karar GHR-10 — Discussions

**APPROVED DECISION — DISABLED initially.**

Team/community forum ihtiyacı yok.

## Karar GHR-11 — Wiki

**APPROVED DECISION — DISABLED.**

Documentation repo içinde version-controlled.

## Karar GHR-12 — GitHub Releases

**APPROVED DECISION — ENABLED.**

Phase 5 sonrası signed release artifacts/release notes için.

## Karar GHR-13 — Packages/Container Registry

### A — GitHub Container Registry (GHCR)
### B — External registry
### C — Both.

**APPROVED DECISION — GHCR initial internal registry**, OCI abstraction korunur. Product architecture registry vendor'a bağlanmaz.

## Karar GHR-14 — Forking

**APPROVED DECISION — DISABLED/limited for private repository unless collaboration model requires.**

Agent tasks branches/worktrees kullanır; forks gerekmez.

## Karar GHR-15 — Auto-delete head branches

**APPROVED DECISION — ENABLED after merge.**

## Karar GHR-16 — Merge methods

**APPROVED DECISION:** Squash merge enabled; merge-commit/rebase merge disabled initially.

## Karar GHR-17 — Repository archive/delete authority

**APPROVED DECISION:** Organization owner only; agents/integration apps no admin permission.

## Karar GHR-18 — GitHub Apps authorization scope

**APPROVED DECISION:** each coding/integration app only this repository, not “all current and future repositories”, unless the provider technically requires broader scope and owner explicitly accepts.

## Karar GHR-19 — Personal Access Tokens for agents

**APPROVED DECISION — DO NOT USE as baseline.**

Prefer provider GitHub Apps/OAuth installations. If a CLI automation absolutely needs a token:
- fine-grained,
- single repo,
- minimum permission,
- expiration,
- no admin/secrets/environment permissions.

## Karar GHR-20 — Repository security features

**APPROVED baseline:**
- dependency graph: on
- Dependabot alerts: on
- Dependabot security updates: on
- secret scanning: on if plan supports; otherwise CI scanner mandatory
- push protection: on if plan supports
- code scanning: plan/tool-dependent, supplemental to portable CI checks.

# 3. Proposed repository layout

```text
/
├── AGENTS.md
├── CLAUDE.md
├── GEMINI.md
├── README.md
├── SECURITY.md
├── CONTRIBUTING.md
│
├── .github/
│   ├── CODEOWNERS
│   ├── copilot-instructions.md
│   ├── ISSUE_TEMPLATE/
│   ├── PULL_REQUEST_TEMPLATE.md
│   ├── workflows/
│   └── instructions/
│
├── apps/
│   ├── control-plane/
│   ├── web-console/
│   ├── edge-agent/
│   └── hi-worker/
│
├── decoys/
├── proto/
├── openapi/
├── schemas/
├── pkg/
├── rules/
├── ai/
├── security/
├── deploy/
├── tests/
├── tools/
└── docs/
    ├── product/
    ├── architecture/
    ├── roadmap/
    ├── adr/
    ├── work-packages/
    ├── phase-gates/
    ├── engineering/
    └── runbooks/
```

# 4. Repository metadata

## Karar GHR-21 — README kapsamı
**APPROVED DECISION:** concise internal project orientation; architecture detail docs'a linklenir.

## Karar GHR-22 — SECURITY.md
**APPROVED DECISION — Day 1.**
Security reporting/process and “do not run against unauthorized networks” boundary.

## Karar GHR-23 — CONTRIBUTING.md
**APPROVED DECISION — Day 1.**
AI-agent ve human contribution aynı PR/CI/change-control kurallarını referans eder.

## Karar GHR-24 — License
Repository private olduğu için public OSS license koymak zorunlu değildir.

**APPROVED DECISION:** proprietary/internal notice initially; third-party license inventory ayrı tutulur. Open-source kararı owner tarafından ayrıca verilir.

# 5. Acceptance

Step 7 bootstrap'ta:
- repo private,
- only approved features enabled,
- no unrestricted app installation,
- no unprotected `main`,
- no real secrets in initial commit.

---

## Final Document Status

- **Decision status:** APPROVED / FINAL
- **Authority:** Product Owner
- **Autonomous agent authority to change this policy:** NONE
- **Change control:** Any material modification requires an explicit governance/change proposal and Product Owner approval.
