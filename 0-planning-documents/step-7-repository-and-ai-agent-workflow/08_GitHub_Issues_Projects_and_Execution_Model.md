# GitHub Issues, Projects & Execution Model
## Step 7 Final Approved Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Implementation authority:** Henüz verilmedi. Bu dosyalar onaylanmadan GitHub repository/settings/permissions oluşturulmaz veya değiştirilmez.  
> **Development-time estimate:** Kullanılmaz.  
> **Bağlayıcı girdiler:** Step 2–6 APPROVED/FINAL çıktıları.  
> **Temel kural:** AI agent kod yazabilir ve değişiklik önerebilir; product, architecture, security-boundary ve release-authority kararlarını onaylayamaz.


# 1. Planning philosophy

Roadmap phase is dependency/maturity sequence, not time estimate.

GitHub must not reintroduce:
- story points,
- hour estimates,
- sprint-date commitments

as hidden planning assumptions.

# 2. GitHub Project

## Karar PM-01 — Project vs Issues only
**APPROVED DECISION — one organization-level GitHub Project.**

Issues remain execution records; Project is portfolio view.

## Karar PM-02 — Project count
**APPROVED DECISION — one product engineering project** through MVP. Split only when operational scale justifies.

## Karar PM-03 — Iteration field
**APPROVED DECISION — NO.**
No sprint/timebox dependency.

## Karar PM-04 — Estimate/Story Points
**APPROVED DECISION — NO.**

## Karar PM-05 — Required Project fields
- Phase
- Work Package ID
- Status
- Component
- Type
- Priority
- Security Impact
- Architecture Impact
- Agent/Executor
- Acceptance Status
- Blocked By.

## Karar PM-06 — Status values
- BLOCKED-BY-DEPENDENCY
- READY
- IN-PROGRESS
- PR-REVIEW
- SECURITY-REVIEW
- AC-VALIDATION
- CHANGE-PROPOSAL-REQUIRED
- DONE.

Matches Step 6.

## Karar PM-07 — Priority
**APPROVED values:**
- Blocker
- High
- Normal
- Deferred.

Not urgency-by-date. Blocker = dependency/security gate.

# 3. Issue types/templates

## Karar PM-08 — Work package issue
Generated/created from approved Markdown spec.

## Karar PM-09 — Bug
Fields:
- observed
- expected
- reproduction
- environment
- security impact
- affected phase/AC.

## Karar PM-10 — Change proposal
ADR trigger fields.

## Karar PM-11 — Security finding
Sensitive issue form/process. Private repo still requires careful content; exploit secrets/real credentials not pasted casually.

## Karar PM-12 — Research/spike
Must produce evidence + recommendation, not code breadth.

## Karar PM-13 — Issue Forms vs Markdown templates
GitHub Issue Forms provide structured required inputs but remain preview-sensitive.

**APPROVED DECISION:** use Issue Forms for work-package/bug/change/security templates, keeping schemas simple and version-controlled; because output is normal Markdown issue body, migration risk is low.

# 4. Labels

## Karar PM-14 — Label explosion
Avoid encoding every Project field as label.

Minimal labels:
- `type:bug`
- `type:spike`
- `type:security`
- `type:change-proposal`
- `type:dependency`
- `agent:blocked`
- `needs-owner-decision`.

Component/Phase/Status live in Project fields.

# 5. Milestones

## Karar PM-15
**APPROVED DECISION — no GitHub Milestones initially.**
Phase field/phase-gate docs already represent dependency progression. Add milestone later only for release grouping, not date estimation.

# 6. Issue lifecycle

## Karar PM-16 — Ready
Only if:
- work package approved,
- dependencies done,
- owner decision refs closed,
- test environment available.

## Karar PM-17 — Agent assignment
Assign agent only READY issue.

## Karar PM-18 — PR linkage
Every implementation PR must link one primary work package issue.

## Karar PM-19 — Auto-close on merge
**APPROVED DECISION — avoid relying solely on `Closes #...`.**
Merge can move issue to AC-VALIDATION; closure occurs after acceptance evidence.

## Karar PM-20 — DONE authority
Project automation may propose; required AC validation must be recorded. Phase gate still owner-approved.

# 7. Views

Approved:
1. **Roadmap by Phase**
2. **Ready Queue**
3. **Agent In Progress**
4. **Security Review**
5. **Change Proposals**
6. **AC Validation**
7. **Phase 0 Board**

# 8. Automation

## Karar PM-21 — New approved work package → Project
Automate where possible.

## Karar PM-22 — PR opened
Move issue `PR-REVIEW`.

## Karar PM-23 — Sensitive path
Mark/route `SECURITY-REVIEW` if relevant.

## Karar PM-24 — PR merged
Move `AC-VALIDATION`, not directly DONE.

## Karar PM-25 — Required tests all green
May add evidence/status but cannot phase-approve.

# 9. Issue content injection risk

Issue text can be read by coding agents.

**APPROVED DECISION:** agent policy treats issue comments as task context but not higher authority than approved work-package and protected instructions. Any comment requesting architecture/security bypass triggers escalation.

# 10. Acceptance

A Project board must answer without reading source code:
- what is READY,
- what is blocked and by what,
- which agent owns current work,
- which PR is under review,
- which AC still fails,
- which phase gate is open.

---

## Final Document Status

- **Decision status:** APPROVED / FINAL
- **Authority:** Product Owner
- **Autonomous agent authority to change this policy:** NONE
- **Change control:** Any material modification requires an explicit governance/change proposal and Product Owner approval.
