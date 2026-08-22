# Repository Bootstrap & Step 7 Exit Checklist
## FINAL

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Implementation authority:** Henüz verilmedi. Bu dosyalar onaylanmadan GitHub repository/settings/permissions oluşturulmaz veya değiştirilmez.  
> **Development-time estimate:** Kullanılmaz.  
> **Bağlayıcı girdiler:** Step 2–6 APPROVED/FINAL çıktıları.  
> **Temel kural:** AI agent kod yazabilir ve değişiklik önerebilir; product, architecture, security-boundary ve release-authority kararlarını onaylayamaz.


# 1. Owner inputs required before bootstrap

## Karar EXIT-01 — GitHub organization name
**OWNER INPUT REQUIRED.**

## Karar EXIT-02 — Repository name
**OWNER INPUT REQUIRED.**

## Karar EXIT-03 — GitHub plan
Owner should confirm available plan/features.

**APPROVED DECISION:** org/private-repo plan supporting required Rulesets/protections; workflow does not require Enterprise-only deployment reviewer features.

## Karar EXIT-04 — Primary coding-agent account/access
**APPROVED DECISION:** Codex access with selected-repo GitHub integration.

## Karar EXIT-05 — Secondary reviewer access
**APPROVED DECISION:** Claude Code available for sensitive PR independent review; optional if owner declines extra provider/cost.

# 2. Bootstrap order

After Step 7 final approval:

1. Create/confirm GitHub Organization.
2. Create private empty repository.
3. Set default `main`.
4. Initial owner-only bootstrap commit with governance skeleton.
5. Configure repository features.
6. Configure GitHub Actions policy.
7. Add Rulesets protecting `main`.
8. Add CODEOWNERS.
9. Add PR/Issue templates.
10. Add agent instruction files.
11. Add final Step 2–7 docs.
12. Add Project fields/views.
13. Install selected coding-agent GitHub Apps limited to repo.
14. Install Renovate / enable Dependabot alerts.
15. Configure baseline CI.
16. Create Phase 0 work-package specs/issues.
17. Validate protections with intentionally failing test PR.
18. Activate first READY package.

# 3. Protection acceptance tests

## Karar EXIT-06 — Direct main push test
Must fail for non-bypass actor.

## Karar EXIT-07 — Missing CI test
PR cannot merge.

## Karar EXIT-08 — Workflow change
Requires CODEOWNER.

## Karar EXIT-09 — AGENTS.md change
Requires CODEOWNER.

## Karar EXIT-10 — Proto breaking change
Buf/policy check fails unless approved compatibility path.

## Karar EXIT-11 — Work-package path violation
Policy check fails.

## Karar EXIT-12 — Agent merge attempt
Fails / agent lacks capability.

## Karar EXIT-13 — Agent secret/admin access
Unavailable.

## Karar EXIT-14 — Release workflow from normal PR
Cannot obtain production release/signing authority.

# 4. Agent environment acceptance

- selected repository only,
- no production network,
- no production secrets,
- branch/worktree isolation,
- build/test commands work,
- instruction files loaded,
- task can create draft PR,
- task cannot alter repo settings.

# 5. Project/issue acceptance

- Phase/Status/Component/Risk fields exist,
- P0-W1..W10 represented,
- dependency blockers visible,
- READY query works,
- PR merge moves package to AC validation rather than auto-DONE.

# 6. Documentation acceptance

Final repo includes:
- Step 2 product decision record
- Step 3 feasibility
- Step 4 architecture
- Step 5 MVP scope
- Step 6 roadmap
- Step 7 engineering governance
- ADR index
- agent policy
- security policy
- contribution guide.

# 7. Step 7 exit criteria

Step 7 can close only when:

- [ ] all Step 7 decisions APPROVED/DEFERRED
- [ ] owner inputs GHR org/repo names resolved
- [ ] private monorepo exists
- [ ] `main` Ruleset active
- [ ] CODEOWNERS active
- [ ] merge method/policy active
- [ ] Actions least-privilege policy active
- [ ] agent instruction policy committed
- [ ] coding agent repo access least privilege
- [ ] CI skeleton green
- [ ] issue/project execution model live
- [ ] ADR/change-control live
- [ ] secret/release security policy live
- [ ] Phase 0 specs/issues created
- [ ] protection negative tests pass
- [ ] no production signing/admin secret exposed to agents
- [ ] Phase 0 READY queue established.

# 8. Next step after exit

**Step 8 — Development Execution / Phase 0**

First implementation work begins only then.

---

# Decision Status vs Bootstrap Status

**Governance decisions:** APPROVED / FINAL.

**Repository bootstrap execution:** NOT YET EXECUTED.

This checklist therefore remains an operational execution checklist. Its unchecked items do **not** mean Step 7 governance decisions are open; they mean the approved GitHub configuration has not yet been physically applied and validated.

Actual organization/repository names must be supplied by the Product Owner before bootstrap.


---

## Final Document Status

- **Decision status:** APPROVED / FINAL
- **Authority:** Product Owner
- **Autonomous agent authority to change this policy:** NONE
- **Change control:** Any material modification requires an explicit governance/change proposal and Product Owner approval.
