# Secrets, Signing & Release Security
## Step 7 Final Approved Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Implementation authority:** Henüz verilmedi. Bu dosyalar onaylanmadan GitHub repository/settings/permissions oluşturulmaz veya değiştirilmez.  
> **Development-time estimate:** Kullanılmaz.  
> **Bağlayıcı girdiler:** Step 2–6 APPROVED/FINAL çıktıları.  
> **Temel kural:** AI agent kod yazabilir ve değişiklik önerebilir; product, architecture, security-boundary ve release-authority kararlarını onaylayamaz.


# 1. Secret domains

Separate:
1. local developer secrets,
2. CI test secrets,
3. AI provider development credentials,
4. artifact registry credentials,
5. product production secrets,
6. signing/TUF keys.

Agents must not collapse these domains.

# 2. Local development

## Karar SEC-01 — `.env`
**APPROVED DECISION:** `.env.example` committed; `.env.local`/real `.env` gitignored.

## Karar SEC-02 — Local secret storage
Prefer OS keychain/password manager. `.env.local` acceptable for low-risk dev key only when file permissions and gitignore are enforced.

## Karar SEC-03 — Real customer/production secrets in local dev
**DENY.**

# 3. GitHub secrets

## Karar SEC-04 — Repository secrets
Only CI-scoped low-risk credentials that cannot use OIDC/short-lived auth.

## Karar SEC-05 — Environment secrets
Use `ci-ai`, `release` style environments where useful. Do not assume private-repo required-reviewer capability exists on every GitHub plan.

## Karar SEC-06 — OIDC
**APPROVED DECISION:** prefer GitHub Actions OIDC/short-lived federation for cloud/registry/provider auth where supported, rather than long-lived keys.

## Karar SEC-07 — AI provider CI keys
Only Phase 4 AI eval jobs; never available to arbitrary PRs/forks. Budget-limited account/project.

# 4. Release authority

## Karar SEC-08 — Agent can trigger production release
**DENY.**

## Karar SEC-09 — Release trigger
**APPROVED DECISION:** owner-authorized `workflow_dispatch` or protected release tag after Phase Gate. No automatic “merge main = production release”.

## Karar SEC-10 — GitHub Environment required reviewer
**APPROVED DECISION:** enable as defense-in-depth if plan supports private-repo feature; do not make it sole control.

Fallback:
- release workflow permission restricted,
- production signing key not in general GitHub repo secret,
- owner-controlled external/keyless signing identity.

# 5. Cosign/TUF key model

## Karar SEC-11 — Test signing keys
Disposable/test-only keys in CI acceptable.

## Karar SEC-12 — Production artifact signing
**APPROVED DECISION:** keyless/OIDC Sigstore where artifact/distribution context supports it, or KMS/HSM-backed key. Not an agent-readable static key.

## Karar SEC-13 — TUF root
**APPROVED DECISION:** root trust key offline/outside agent/normal CI. Exact threshold/key-custody design finalized before Phase 5 production signing.

## Karar SEC-14 — Online TUF roles
Targets/snapshot/timestamp can use automated protected signing identity according to final Phase 5 threat model.

## Karar SEC-15 — Signing separation
Build job cannot arbitrarily sign unreviewed artifact as production. Signing consumes artifact digest from protected release workflow.

# 6. Actions and supply chain

## Karar SEC-16 — Actions pinning
Full commit SHA mandatory for third-party and GitHub actions if repository policy supports enforcement.

## Karar SEC-17 — Dependency bots
Bot PRs have no auto-merge initially.

## Karar SEC-18 — Registry images
Base images and decoy dependencies digest-pinned where practical; update bot opens reviewable PR.

## Karar SEC-19 — SBOM
Release candidate must include SBOM.

## Karar SEC-20 — Provenance
Release records:
- source commit
- build workflow
- artifact digest
- SBOM
- signature/attestation
- component version.

# 7. Dependency automation

## Karar SEC-21 — Renovate vs Dependabot for version management

### A — Dependabot only
GitHub-native, supports several ecosystems/groups.
### B — Renovate only
Flexible monorepo, Docker/GitHub Actions digest pinning.
### C — Renovate version updates + Dependabot alerts/security signals.

**APPROVED DECISION — C.**

Renovate has strong multi-manager/digest-pinning control; Dependabot remains useful native vulnerability alert/security-update signal.

## Karar SEC-22 — Renovate hosting
Prefer GitHub App/Mend-hosted integration if acceptable; self-host only if privacy/control need justifies operations.

## Karar SEC-23 — Update grouping
Group low-risk patch/dev updates by ecosystem; major/runtime/security-sensitive updates separate.

## Karar SEC-24 — Automatic dependency merge
**NO initially.**
Later only patch-level low-risk packages with complete CI and no runtime/security authority change.

# 8. Secret incidents

## Karar SEC-25
If a secret is committed:
- revoke/rotate first,
- remove from current code,
- assess history exposure,
- do not assume history rewrite alone solves compromise,
- open security incident/audit entry.

# 9. Acceptance

No coding agent environment contains:
- production signing root,
- GitHub org admin token,
- production customer secret,
- production deployment credential.

A normal PR cannot access release secrets.

---

## Final Document Status

- **Decision status:** APPROVED / FINAL
- **Authority:** Product Owner
- **Autonomous agent authority to change this policy:** NONE
- **Change control:** Any material modification requires an explicit governance/change proposal and Product Owner approval.
