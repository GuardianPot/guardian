# CI/CD, Testing & Quality Gates
## Step 7 Final Approved Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Implementation authority:** Henüz verilmedi. Bu dosyalar onaylanmadan GitHub repository/settings/permissions oluşturulmaz veya değiştirilmez.  
> **Development-time estimate:** Kullanılmaz.  
> **Bağlayıcı girdiler:** Step 2–6 APPROVED/FINAL çıktıları.  
> **Temel kural:** AI agent kod yazabilir ve değişiklik önerebilir; product, architecture, security-boundary ve release-authority kararlarını onaylayamaz.


# 1. CI principles

- PR is the integration boundary.
- Fast deterministic checks first.
- Privileged/high-cost tests isolated.
- No untrusted PR code receives privileged secrets.
- CI failure cannot be overridden by an agent.
- Required checks evolve by phase but removal requires governance review.

# 2. GitHub Actions security

## Karar CI-01 — CI platform
**APPROVED DECISION — GitHub Actions**, Step 4 baseline.

## Karar CI-02 — Default `GITHUB_TOKEN`
**APPROVED DECISION:** repository workflow default `permissions: contents: read`.

Per job explicit additional permissions only when required.

## Karar CI-03 — Third-party actions
**APPROVED DECISION:** allow only GitHub-owned + explicitly reviewed actions; pin every action to full-length commit SHA.

GitHub states full SHA pinning is the immutable-use method for Actions.

## Karar CI-04 — Reusable workflows
**APPROVED DECISION:** local repo reusable workflows for common build/test policy; external reusable workflow requires review/pinning strategy.

## Karar CI-05 — `pull_request_target`
**APPROVED DECISION — DO NOT use to execute/check out untrusted PR code with secrets.**

If metadata-only use ever required, strict no-head-code execution.

## Karar CI-06 — Workflow modifications
`.github/workflows/**` CODEOWNER protected and policy-checked.

## Karar CI-07 — Caches
No secret-bearing cache. Keys include dependency lock hash/toolchain version. Untrusted PR cache poisoning risk considered.

# 3. Required PR fast checks

## Karar CI-08 — Repository policy check
Required from first real PR:
- work-package ID valid
- allowed paths
- protected decision files
- PR template fields
- generated contract drift.

## Karar CI-09 — Go checks
Initial:
- gofmt
- go vet
- go test
- govulncheck.

Race tests may run slower/nightly and become required for concurrency-sensitive packages.

## Karar CI-10 — Frontend checks
- lint
- TypeScript typecheck
- unit/component tests
- production build.

## Karar CI-11 — Contract checks
- Buf lint
- Buf breaking
- Protobuf generation freshness
- OpenAPI validation
- generated client freshness.

## Karar CI-12 — Secret scan
**APPROVED DECISION:** Gitleaks-class portable CI scanner + GitHub secret scanning where available.

## Karar CI-13 — Dependency/license check
Generate inventory/license policy; forbidden/unknown license requires review.

## Karar CI-14 — Vulnerability scans
- govulncheck for Go
- dependency alert tooling
- Trivy-class filesystem/container scan for images/packages
- JS ecosystem audit/OSV signal.

No one scanner is sole security truth.

# 4. Integration checks

## Karar CI-15 — PostgreSQL integration
Run container/service DB integration tests in normal GitHub-hosted CI.

## Karar CI-16 — SQLite crash/replay
Deterministic local tests + failure injection.

## Karar CI-17 — containerd/network lab
Requires privileged disposable runner.

## Karar CI-18 — Browser E2E
Playwright-class E2E becomes required as user flows appear.

Specific UI test framework can be selected during Phase 1 implementation if not already architecture-fixed; changes do not alter product behavior.

## Karar CI-19 — Attack scenario tests
Run safe local virtual lab; never target public/third-party systems.

## Karar CI-20 — AI evals
Phase 4 onwards:
- schema
- evidence citation
- prompt injection
- unsupported claim
- guidance safety.

# 5. Runner model

## Karar CI-21 — GitHub-hosted runners
**APPROVED DECISION:** default for unprivileged build/test.

## Karar CI-22 — Self-hosted runner persistence
### A — Long-lived mutable runner
Fast, high contamination risk.
### B — Ephemeral disposable VM runner
Stronger isolation.

**APPROVED DECISION — B** for privileged network/containerd/KVM tests.

## Karar CI-23 — Privileged runner credentials
No production secrets/signing keys. Minimum GitHub job token.

## Karar CI-24 — Runner network
No customer/internal production networks. Dedicated isolated lab networks.

## Karar CI-25 — Cleanup
Runner/workspace destroyed after privileged job; no cross-PR state.

# 6. Required check layers

## Layer A — every PR
- policy
- format/lint/type
- unit
- contracts
- secrets
- basic dependency vuln
- build.

## Layer B — affected component
- DB integration
- container build
- UI E2E
- decoy fixture.

## Layer C — privileged/security
- network lab
- containerd
- egress
- device PKI
- attack scenarios.

## Layer D — release
- full security regression
- AI eval
- performance
- SBOM
- provenance
- signing/update verification.

# 7. Status check decisions

## Karar CI-26 — Required checks source
Where GitHub allows, required status checks should be tied to expected GitHub App/source rather than any actor.

## Karar CI-27 — Flaky test
Flaky required test is treated as defect. “rerun until green” is not acceptance evidence.

## Karar CI-28 — Test skipping
Agent cannot disable/skip required checks to merge.

## Karar CI-29 — Coverage percentage
**APPROVED DECISION:** no global vanity coverage threshold initially. Critical deterministic/security logic gets targeted coverage requirements.

## Karar CI-30 — Performance
Reference benchmarks become Phase 5 release gate; not every PR.

# 8. Artifact provenance

## Karar CI-31 — Build provenance
**APPROVED DECISION:** generate SBOM + provenance/attestation metadata for release candidates; GitHub artifact attestations can supplement Cosign/TUF if available.

## Karar CI-32 — PR artifacts
Test binaries/images are non-production and cannot be promoted directly without protected release workflow.

# 9. Acceptance

A malicious PR modifying a workflow to request `contents: write` or secrets must:
- trigger CODEOWNER,
- fail/require policy approval,
- never automatically obtain production credentials.

---

## Final Document Status

- **Decision status:** APPROVED / FINAL
- **Authority:** Product Owner
- **Autonomous agent authority to change this policy:** NONE
- **Change control:** Any material modification requires an explicit governance/change proposal and Product Owner approval.
