# Phase 7 — Post-MVP Deception Breadth & Windows Expansion
## Final Approved Implementation Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Faz amacı

MVP'deki purposeful four-family portfolio'yu, customer discovery sonuçlarına göre **daha geniş deception coverage** ile büyütmek; özellikle Windows-heavy environments için network-level realism'i artırmak.

# 2. Sequencing principle

Yeni decoy yalnız şu koşullardan birini karşılıyorsa eklenmeli:
- önemli threat behavior gap kapatıyor,
- ICP'de sık görülen service'i temsil ediyor,
- correlation/journey için yeni evidence türü sağlıyor,
- customer pilot demand ile doğrulanmış.

“Catalog count” başarı metriği değildir.

# 3. Workstreams

## P7-W1 — RDP deception

Options:
- protocol-level low/medium emulation
- controlled real Windows only later Phase 9

Approved here:
- safe protocol-level RDP discovery/auth interaction where feasible.

Acceptance:
- source/session/auth evidence,
- no real Windows licensing dependency for baseline.

## P7-W2 — Additional Windows services

Candidate based on demand:
- WinRM
- MSSQL
- LDAP-like directory discovery decoy (without prod AD write)
- richer SMB persona

Each requires threat-value decision.

## P7-W3 — Redis / MySQL / FTP / Telnet / remote admin packs

Use:
- custom implementation
- OpenCanary adapters where security/quality sufficient

Each pack must meet common:
- signed OCI
- manifest
- canonical telemetry
- health
- egress
- resource limits
- attack fixtures

## P7-W4 — OpenCanary integration decision realization

If spike validated:
- wrap pinned OpenCanary
- map modules to separate product pack/persona semantics
- normalize logs
- do not expose upstream configuration complexity to operator.

## P7-W5 — Richer personas

Curated industry-neutral internal personas:
- backup server
- file server
- build server
- database cluster member
- admin portal
- legacy service

No arbitrary LLM-generated personas yet.

## P7-W6 — Honey documents / URL tokens

Add non-agent/manual placement:
- document/open token
- URL token
- share lure

Lifecycle:
created → placed → triggered → revoked/stale.

## P7-W7 — Cloud/SaaS honey tokens

Only after privacy/provider threat model:
- cloud access-key style decoy
- webhook-triggered token
- no production authority

May overlap Phase 8 identity scope; owner can move.

## P7-W8 — Notification integration breadth

Candidates:
- Slack
- Teams
- Pager-style webhook providers

Native integration only if generic webhook insufficient for customer UX.

# 4. Acceptance pattern for every new decoy

Required:
- threat hypothesis
- persona
- interaction level
- canonical evidence mapping
- confidence factors
- health
- no production secret
- egress profile
- resource profile
- attacker input safety
- correlation fixtures
- UI rendering
- signed update
- license/provenance review

# 5. Exit gate

Not “all listed protocols implemented”.

Exit when Product Owner-approved Phase 7 portfolio:
- [ ] closes validated coverage gaps,
- [ ] passes common decoy contract,
- [ ] does not materially increase alert noise,
- [ ] Windows network deception materially richer than MVP,
- [ ] incident/journey model requires no architecture rewrite.

---

# 6. Detailed Phase 7 Acceptance Criteria

## P7-AC-001 — Common pack contract
Every newly approved decoy pack must expose a valid manifest, health probe, canonical telemetry adapter, resource policy, egress policy, signed digest and attack fixture before it can be enabled in a release.

## P7-AC-002 — RDP evidence
**Given** an RDP deception capability  
**When** an internal source discovers/connects/authenticates as supported by the implementation  
**Then** protocol-level evidence is normalized with source/session/persona provenance and does not falsely claim a real Windows host.

## P7-AC-003 — Additional protocol isolation
**Given** any new Redis/MySQL/FTP/Telnet/WinRM/MSSQL-style pack  
**When** attacker-controlled input is sent  
**Then** malformed input cannot escape the pack boundary, access Edge management resources or bypass egress/resource policy.

## P7-AC-004 — Noise regression
**Given** the approved benign scanner/admin fixture set  
**When** the new pack is enabled  
**Then** alert/incident volume does not exceed the owner-approved noise threshold without an explicit rule/suppression update.

## P7-AC-005 — Persona consistency
**Given** a curated persona  
**When** hostname, banner, protocol metadata and UI identity are inspected  
**Then** cross-protocol facts do not contradict the declared persona.

## P7-AC-006 — Honey document/token lifecycle
**Given** a placed token/document  
**When** it is triggered, moved, expired or revoked  
**Then** the product can identify its placement/lifecycle state and does not continue treating a retired token as an active trusted lure.

## P7-AC-007 — Cloud/SaaS token authority
Any cloud/SaaS honey token introduced in this phase must have zero production authority by default and have a documented revocation path.

## P7-AC-008 — Upstream OSS upgrade
**Given** a new upstream OpenCanary/Cowrie/other decoy version  
**When** the pack is updated  
**Then** canonical contract tests, security fixtures, license/provenance checks and regression scenarios pass before the new digest is promoted.

# 7. Portfolio Promotion Rule

A new decoy family moves from experiment to supported product capability only when:
- customer/threat-value rationale exists,
- common pack ACs pass,
- noise impact is measured,
- support/runbook exists,
- owner approves promotion.

---

## Final Phase Status

- **Phase:** Phase 7 — Post-MVP Deception Breadth & Windows Expansion
- **Status:** APPROVED / FINAL
- **Owner decision:** Tüm bu faz kapsamı, sequencing, dependencies, acceptance criteria ve exit gate'leri onaylandı.
- **Change policy:** Bu fazın kapsamı implementation sırasında sessizce genişletilemez/daraltılamaz; değişiklik Product Owner change-control sürecine tabidir.
