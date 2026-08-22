# External Research & Current Platform Constraints
## Step 7 Final Research Notes — 22 August 2026

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Implementation authority:** Henüz verilmedi. Bu dosyalar onaylanmadan GitHub repository/settings/permissions oluşturulmaz veya değiştirilmez.  
> **Development-time estimate:** Kullanılmaz.  
> **Bağlayıcı girdiler:** Step 2–6 APPROVED/FINAL çıktıları.  
> **Temel kural:** AI agent kod yazabilir ve değişiklik önerebilir; product, architecture, security-boundary ve release-authority kararlarını onaylayamaz.


This file records external platform facts used to form Step 7 recommendations. It is not a replacement for the providers' current documentation; re-check before applying sensitive settings because SaaS features/plans can change.

# 1. GitHub Rulesets

Official GitHub documentation states Rulesets can:
- require PR before merge,
- require status checks,
- block force pushes,
- require code owner reviews and other PR controls,
- layer multiple rulesets; most restrictive applicable settings are enforced.

Sources:
- https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets
- https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets

**Design implication:** Rulesets are preferred primary protection abstraction.

# 2. CODEOWNERS

GitHub automatically requests Code Owner reviews and can require Code Owner approval when protection rules enable it. GitHub explicitly recommends protecting the CODEOWNERS file itself.

Source:
- https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners

**Implication:** `.github/CODEOWNERS` and `.github/**` should be owner-protected.

# 3. GitHub Actions security

GitHub secure-use guidance:
- third-party Actions can gain powerful workflow access,
- pinning to full-length commit SHA is the immutable action reference approach,
- workflow `GITHUB_TOKEN` should use least privilege.

Sources:
- https://docs.github.com/en/actions/reference/security/secure-use
- https://docs.github.com/en/code-security/tutorials/secure-your-organization/protect-against-threats

**Implication:** explicit `permissions`, action allowlist and SHA pinning.

# 4. Deployment environments / plan constraints

GitHub environments can gate deployments and secrets. Current docs note plan-dependent availability; for private/internal repositories some advanced protection rules such as required reviewers are not generally available on Free/Pro/Team in the same way as public repos.

Sources:
- https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments
- https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/review-deployments

**Implication:** production authority must not rely solely on an environment reviewer feature that may require a different plan.

# 5. Issue Forms / Projects

GitHub Issue templates/forms standardize issue input. Projects can track structured metadata via fields. Issue Forms remain subject to GitHub feature evolution.

Sources:
- https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/about-issue-and-pull-request-templates
- https://docs.github.com/en/issues/planning-and-tracking-with-projects/learning-about-projects/quickstart-for-projects

**Implication:** use Project fields for Phase/Status/etc.; avoid duplicating all metadata as labels.

# 6. Codex

Current OpenAI material describes Codex as a coding agent available in cloud/terminal/editor workflows, with GitHub integration and parallel agent workflows. OpenAI documentation also uses `AGENTS.md` as repository instructions/context for testing behavior.

Sources:
- https://openai.com/codex/
- https://openai.com/index/codex-now-generally-available/
- https://help.openai.com/en/articles/11096431
- https://openai.com/index/introducing-codex/

**Implication:** strong primary cloud implementation-agent candidate, while repo governance remains vendor-neutral.

# 7. Claude Code

Current Claude Code docs provide:
- fine-grained allow/ask/deny permissions,
- plan/default/dontAsk and other modes,
- managed policy options,
- PreToolUse hooks that can deny actions,
- sandbox/security guidance.

Sources:
- https://code.claude.com/docs/en/permissions
- https://code.claude.com/docs/en/security
- https://code.claude.com/docs/en/hooks

**Implication:** strong secondary independent reviewer and secure local/headless agent candidate.

# 8. GitHub Copilot cloud agent

GitHub supports repository-wide, path-specific and agent instruction files (`AGENTS.md`, `CLAUDE.md`, `GEMINI.md`) for Copilot features. Copilot cloud agent/code review internet access can be restricted by a firewall. GitHub notes custom instruction behavior can be influenced by instruction files available in the working branch.

Sources:
- https://docs.github.com/en/copilot/reference/custom-instructions-support
- https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/customize-the-firewall
- https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions

**Implication:** keep instruction files CODEOWNER-protected and make the workflow compatible with Copilot without requiring it.

# 9. Gemini CLI

Gemini CLI docs provide:
- container/OS sandboxing,
- tool allow/exclude settings,
- approval modes,
- trusted-folder restricted mode,
- headless execution.

Sources:
- https://google-gemini.github.io/gemini-cli/docs/cli/sandbox.html
- https://google-gemini.github.io/gemini-cli/docs/get-started/configuration.html
- https://google-gemini.github.io/gemini-cli/docs/cli/trusted-folders.html

**Implication:** useful local/sandboxed fallback; no critical pipeline dependency.

# 10. Dependency automation

Renovate supports monorepos, Docker and GitHub Actions dependencies including digest pinning/updating. Dependabot provides GitHub-native alerts/security/version updates.

Sources:
- https://docs.renovatebot.com/
- https://docs.renovatebot.com/modules/manager/github-actions/
- https://docs.renovatebot.com/docker/
- https://docs.github.com/en/code-security/tutorials/secure-your-dependencies/dependabot-quickstart

**Implication:** Renovate for controlled cross-ecosystem version updates + Dependabot vulnerability signals.

# 11. Re-validation triggers

Before applying final repository settings, re-check current docs if:
- GitHub plan changes,
- agent provider permission model changes,
- Rulesets features change,
- private repository environment reviewer availability changes,
- coding agent gains direct merge/deploy features,
- Actions provenance/signing capability changes.

---

## Final Document Status

- **Decision status:** APPROVED / FINAL
- **Authority:** Product Owner
- **Autonomous agent authority to change this policy:** NONE
- **Change control:** Any material modification requires an explicit governance/change proposal and Product Owner approval.
