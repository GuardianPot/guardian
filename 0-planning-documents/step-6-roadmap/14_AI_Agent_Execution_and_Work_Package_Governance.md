# Step 6 — AI Agent Execution & Work-Package Governance
## FINAL

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Amaç

Roadmap'ı AI coding agent'larının 24/7 çalışabileceği, fakat product/architecture authority kazanmayacağı executable work modeline çevirmek.

# 2. Fundamental rule

> **Agent implements approved work; agent does not approve scope, product behavior, architecture, security boundary or technology changes.**

# 3. Work package format

Her agent task dosyası/issue aşağıdaki alanları içermeli:

```yaml
work_package_id: P2-W5
phase: 2
title: SSH Cowrie Pack
status: approved-for-implementation
decision_refs:
  - Step4: DR-08
  - Step5: DC-02
acceptance_refs:
  - AC-SSH-001
  - AC-SSH-002
dependencies:
  - P0-W7
  - P2-W4
allowed_paths:
  - decoys/cowrie-pack/**
  - proto/telemetry/**
forbidden_changes:
  - architecture
  - new datastore
  - product scope
security_notes:
  - attacker input is hostile
required_tests:
  - ...
```

# 4. Agent permissions

Agent may:
- implement approved code
- write tests
- refactor within approved module boundary
- fix bugs
- improve docs
- prepare benchmark
- prepare ADR/change proposal
- update dependency when policy permits and review requested

Agent may not autonomously:
- add product capability
- change phase scope
- change programming language/framework
- add broker/database/runtime
- weaken TLS/auth
- grant decoy egress
- introduce production secret
- alter retention/security policy
- remove acceptance criterion
- alter confidence/severity semantics
- make AI tool-capable
- merge architectural ADR as Accepted
- sign production release.

# 5. Stop-and-escalate conditions

Agent stops implementation and opens proposal when:
- existing decision impossible to implement safely,
- dependency incompatibility discovered,
- upstream OSS behavior contradicts assumption,
- performance gate requires architecture change,
- security boundary cannot be enforced,
- schema breaking change unavoidable,
- new privilege required,
- new external service/dependency materially changes operations.

# 6. Branch/PR discipline

Every non-trivial PR:
- one coherent work package or tightly coupled set
- decision IDs
- AC IDs
- dependency changes
- migrations
- protocol changes
- security impact
- tests run
- screenshots for UI
- benchmark result if performance-sensitive
- unresolved assumptions.

# 7. Contract-change policy

For Protobuf/OpenAPI:
1. proposal
2. contract diff
3. breaking CI check
4. Edge/Control compatibility analysis
5. generated clients
6. tests
7. owner approval if semantic product behavior changes.

# 8. Test hierarchy expected from agents

1. unit
2. contract
3. integration
4. network lab
5. attack scenario
6. browser E2E
7. failure injection
8. security regression
9. AI eval if applicable.

Agent cannot declare “done” with only unit tests when work package requires higher layer.

# 9. Phase execution board semantics

Approved states:
- `BLOCKED-BY-DEPENDENCY`
- `READY`
- `IN-PROGRESS`
- `PR-REVIEW`
- `SECURITY-REVIEW`
- `AC-VALIDATION`
- `DONE`
- `CHANGE-PROPOSAL-REQUIRED`

No “DONE” without referenced acceptance evidence.

# 10. Phase gate automation

CI may compute:
- AC test status
- blocker work packages
- security scan
- performance regression
- AI eval status.

But **phase gate approval is human/Product Owner authority**.

# 11. Documentation generated alongside code

Required artifacts:
- ADRs
- runbooks
- protocol docs
- threat model deltas
- test fixture descriptions
- support diagnostics
- release notes
- known limitations.

# 12. Dependency update agent

Can monitor/update:
- Go modules
- npm packages
- decoy upstreams
- base OS/image
- container runtime dependencies.

Must include:
- changelog summary
- CVE relevance
- license delta
- compatibility tests
- rollback path.

# 13. Security-sensitive ownership

Paths approved mandatory review:
- `security/**`
- `proto/**`
- `apps/edge-agent/**` privileged helper
- updater/signing
- auth/PKI
- AI prompts/schemas
- network policy
- decoy manifests requiring privilege
- migrations affecting evidence/audit.

# 14. Agent success metric

Not “lines of code” or “number of PRs”.

Preferred:
- accepted work packages
- AC pass evidence
- no architecture drift
- low rework from hidden assumptions
- security regression coverage
- reproducible build/test.

# 15. Exit criteria for agentic execution setup

Before large parallel coding:
- [ ] phase work packages exist
- [ ] dependency graph represented
- [ ] PR template
- [ ] CODEOWNERS/review policy
- [ ] ADR workflow
- [ ] CI gates
- [ ] lab reproducible
- [ ] agent secret isolation
- [ ] production signing keys inaccessible
- [ ] change proposal path documented.
