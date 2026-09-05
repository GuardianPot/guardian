# AI Coding-Agent Strategy & Permissions
## Step 7 Final Approved Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Implementation authority:** Henüz verilmedi. Bu dosyalar onaylanmadan GitHub repository/settings/permissions oluşturulmaz veya değiştirilmez.  
> **Development-time estimate:** Kullanılmaz.  
> **Bağlayıcı girdiler:** Step 2–6 APPROVED/FINAL çıktıları.  
> **Temel kural:** AI agent kod yazabilir ve değişiklik önerebilir; product, architecture, security-boundary ve release-authority kararlarını onaylayamaz.


# 1. Problem

Product architecture provider-agnostic AI olsa da **development coding-agent runtime** ayrı bir engineering decision'dır.

Goal:
- parallel autonomous implementation,
- strict repo/security bounds,
- PR-based evidence,
- no product/architecture authority.

# 2. Candidate comparison

| Candidate | Strength | Permission/security model | Fit |
|---|---|---|---|
| Codex cloud/CLI | cloud parallel tasks, GitHub PR workflow, worktrees/environments, coding focus | sandbox/environment controls, repository context, CLI approval modes | Strong primary implementation candidate |
| Claude Code | fine-grained allow/ask/deny, hooks, sandbox, strong terminal workflow | deny-first rules, managed settings/hooks, plan mode | Strong independent review/escalation/local agent |
| GitHub Copilot cloud agent | native GitHub issue/PR integration, custom instructions, firewall | repo/org firewall + GitHub controls | Strong GitHub-native fallback/secondary |
| Gemini CLI | open-source CLI, sandbox, headless mode, trusted folders | Docker/Podman sandbox, tool allow/exclude, approval modes | Good local fallback/research agent |

# 3. Agent portfolio decisions

## Karar AG-01 — Single-vendor vs vendor-neutral workflow

### A — One agent-specific process
Simple, lock-in.
### B — Vendor-neutral work-package/CI/PR protocol; adapters per agent.

**APPROVED DECISION — B.**

The repository, not vendor conversation state, is source of truth.

## Karar AG-02 — Primary implementation agent

**APPROVED DECISION — Codex cloud.**

Gerekçe:
- current product explicitly supports parallel cloud coding workflows and GitHub PR creation,
- long-running/parallel implementation matches project operating model,
- can use repo `AGENTS.md`,
- output remains normal Git diff/PR.

This is a development-tool choice, not product LLM provider choice.

## Karar AG-03 — Primary local/interactive implementation tool

**APPROVED DECISION — Codex CLI**, same task protocol, with approval/sandbox configuration.

## Karar AG-04 — Independent secondary reviewer

### A — Same Codex session/model
Less independent.
### B — Claude Code review agent
Different vendor/permission implementation.
### C — human-only.

**APPROVED DECISION — B as optional but strongly preferred for security/architecture-sensitive PRs.**

It is supporting review, not merge authority.

## Karar AG-05 — GitHub Copilot cloud agent

**APPROVED DECISION — optional fallback / GitHub-native reviewer, not mandatory baseline.**

Reason: substantial overlap with Codex primary; keep work-package protocol compatible.

## Karar AG-06 — Gemini CLI

**APPROVED DECISION — optional local sandboxed fallback/research agent.**

No critical process depends on it.

## Karar AG-07 — Agent bake-off before lock-in

**APPROVED DECISION — YES.**
Use Phase 0 non-destructive tasks to compare:
- instruction adherence,
- test evidence,
- PR quality,
- scope discipline,
- cost/usage,
- security policy friction.

Changing primary agent does not require product architecture ADR; it requires engineering governance change approval.

# 4. Agent roles

## Karar AG-08 — Implementer role
May edit only approved work-package scope.

## Karar AG-09 — Reviewer role
Read/diff/test; may leave review comments/change suggestions; no merge.

## Karar AG-10 — Research/spike role
Can run lab/benchmark within isolated environment and produce evidence; architecture conclusion remains proposal.

## Karar AG-11 — Dependency update role
Can prepare dependency PR with changelog/security/license/test evidence.

## Karar AG-12 — Release role
**APPROVED DECISION — NO autonomous agent release role.**

Agents may prepare release notes/build candidate; protected human workflow publishes/signs.

# 5. GitHub permissions

## Karar AP-01 — Repository admin
**Agent: DENY.**

## Karar AP-02 — Repository settings/rulesets
**Agent: DENY.**

## Karar AP-03 — Secrets/environments
**Agent: DENY read/write where possible.**
Agent task environment receives only narrow task secrets, never GitHub settings secret-management ability.

## Karar AP-04 — Main push
**DENY by Ruleset.**

## Karar AP-05 — Feature branch write
**ALLOW** on agent-owned task branches.

## Karar AP-06 — PR create/update
**ALLOW.**

## Karar AP-07 — Issue read/comment
**ALLOW.**
Issue edit/close only if task integration needs; closing does not equal acceptance.

## Karar AP-08 — Merge PR
**ALLOW**, bounded (change proposal 0009, 2026-09-05).
The agent may squash-merge a pull request it opened once every required check
has passed and the branch is mergeable without an override.
`--admin`, `--merge`, `--rebase`, merging past a pending or failing check, and
any change to the branch protection rules stay **DENY**.
Merging does not equal acceptance; acceptance evidence is still recorded and
still approved by the owner.

## Karar AP-09 — Release create/publish
**DENY.**

## Karar AP-10 — Production deployment
**DENY.**

## Karar AP-11 — Signing keys
**DENY.**

## Karar AP-12 — Architecture docs
Read allowed. Edit only in `proposal/*` PR; Accepted status owner-only.

# 6. Runtime tool permissions

## Karar AP-13 — Shell commands
Allow common build/test/lint/git-read commands in sandbox.
Destructive host/system commands denied.

## Karar AP-14 — `git push`
Cloud agent integration may push its own branch as required.
Local agents:
- allowed only task branch,
- no force push main,
- preferably wrapper verifies branch.

## Karar AP-15 — `gh` CLI
**APPROVED DECISION:** read PR/issue, create/update task PR, and squash-merge a
task PR within the `AP-08` bound (change proposal 0009). No `gh repo edit`,
secret, environment, ruleset, or release commands, and no `--admin` merge.

## Karar AP-16 — Docker/containerd/KVM privileges
Normal agent tasks no privileged host access.
Privileged Phase 0/5 test runs execute on disposable isolated lab runner.

## Karar AP-17 — Internet access
### A — unrestricted
### B — deny by default/allowlist dependency and docs endpoints
### C — none.

**APPROVED DECISION — B.**

Cloud coding agents should receive only required package/documentation network access. No production/internal customer networks.

## Karar AP-18 — MCP/plugins/extensions
**APPROVED DECISION:** default deny; each external tool/server requires explicit review.

# 7. Claude Code permission recommendation

If used:
- `plan` for review/research,
- `dontAsk`/explicit allowlist for headless CI-like task,
- block `bypassPermissions`,
- project/managed deny rules,
- PreToolUse hooks can hard-block release/admin commands and an `--admin` merge.

Claude docs expose deny→ask→allow semantics and hooks that can deny even when permissive modes are requested.

# 8. Gemini CLI recommendation

If used:
- sandbox required,
- trusted-folder explicit,
- no `yolo` outside disposable container,
- tool allowlist,
- telemetry preference explicitly configured,
- no production credentials.

# 9. Codex recommendation

- connect only selected repo,
- task environment has build deps, not production secrets,
- repo instructions point to approved work package and AGENTS policy,
- internet allowlist/minimum required,
- each task produces diff/tests/PR,
- no direct release/merge.

# 10. Prompt-injection from repository content

## Karar AP-19
Treat:
- source comments,
- issue text,
- dependency docs,
- attacker fixture payloads,
- generated logs

as untrusted context for coding agents.

**APPROVED DECISION:** repo agent policy states that instructions are authoritative only from approved instruction/governance files and current work-package prompt.

## Karar AP-20 — Agent-generated governance modification
Agent must not modify its own controlling policies in same implementation PR unless that policy change is the explicit approved work package.

# 11. Acceptance

A compromised/over-eager agent should still be unable to:
- push main,
- alter rulesets,
- read production signing secrets,
- approve ADR,
- publish release,
- change protected agent instructions without owner review.

---

## Final Document Status

- **Decision status:** APPROVED / FINAL
- **Authority:** Product Owner
- **Autonomous agent authority to change this policy:** NONE
- **Change control:** Any material modification requires an explicit governance/change proposal and Product Owner approval.
