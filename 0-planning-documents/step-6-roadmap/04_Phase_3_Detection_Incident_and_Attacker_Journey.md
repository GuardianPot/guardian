# Phase 3 — Detection, Incident & Attacker Journey
## Final Approved Implementation Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Faz amacı

Raw deception capability'yi product value'a çevirmek:

> Event → Evidence → Finding → Incident → Attacker Journey

Phase 3 sonunda kullanıcı raw honeypot log'ları yerine **ne oldu, nereden geldi, neye dokundu, ne kadar güvenilir ve ne kadar ciddi** sorularına cevap almalıdır.

# 2. Dependencies

- Phase 2 canonical evidence
- stable source/provenance fields
- multiple decoy families
- durable central persistence
- health/replay semantics

# 3. Workstreams

## P3-W1 — Evidence central persistence

PostgreSQL:
- normalized common columns
- JSONB protocol-specific attributes
- evidence provenance
- immutable IDs
- raw refs
- indexes
- partition-ready event tables

Acceptance:
- AC-EV-002/003,
- evidence can be fetched without decoy-specific parser.

## P3-W2 — Finding model

Finding responsibilities:
- deterministic rule result
- rule ID/version
- supporting evidence IDs
- confidence factors
- candidate severity
- source/entity references

No AI truth.

## P3-W3 — Detection rule set

Implement Step 5 core:
1. direct decoy touch
2. auth attempt
3. synthetic honey credential
4. repeated auth
5. same source multi-service
6. same source multi-decoy

Rules:
- versioned
- deterministic
- fixture-tested
- declarative typed definitions where applicable

## P3-W4 — Entity/source model

Create first-class:
- observed source IP
- network zone
- session
- supplied username
- synthetic credential ID
- decoy/service

Hostname/MAC optional enrichment.

UI wording must preserve uncertainty.

Trace: SRC-01..07.

## P3-W5 — Incident lifecycle

Implement:
- New
- Acknowledged
- Investigating
- Resolved

Disposition:
- Expected/Benign
- Suspicious
- Confirmed
- Unknown

Audit state transitions.

Acceptance: AC-UX-005.

## P3-W6 — Deterministic correlation engine

Dimensions:
- source IP
- bounded rule-specific time window
- session
- username
- decoy/service
- zone
- synthetic credential

Behavior:
- update existing incident or create new
- avoid merge across distinct source anchors
- preserve confidence factors

Acceptance:
- AC-INC-001
- AC-INC-002
- AC-INC-003

## P3-W7 — Multi-decoy attacker journey

Represent:
- chronological nodes
- source
- target decoy
- action
- observed vs inferred
- confidence/severity
- evidence drill-down

MVP UX is timeline-first, not fancy graph.

Acceptance:
- AC-INC-002
- AC-UX-003/004

## P3-W8 — Manual merge/split

Implement auditable correction:
- merge incidents
- split evidence subset / source sequence
- preserve evidence IDs
- record user action

Acceptance: AC-INC-004.

## P3-W9 — Confidence engine

Labels:
- Low / Medium / High

Factors:
- direct decoy touch
- auth attempt
- valid honey credential
- multi-decoy sequence
- known scanner
- operator expected source context

Acceptance: AC-CF-001.

## P3-W10 — Severity engine

Levels:
- Informational
- Low
- Medium
- High
- Critical

Separate from confidence.

Acceptance: AC-CF-002.

## P3-W11 — Known scanner / suppression

Implement:
- IP/CIDR expected scanner entry
- scoped suppression policy
- evidence retention
- notification eligibility flag
- audit

Acceptance:
- AC-CF-003
- AC-CF-004

## P3-W12 — Dedup/aggregation

For burst login/scan:
- preserve count
- first/last seen
- avoid incident explosion
- no evidence deletion

Acceptance: AC-CF-005.

## P3-W13 — Incident-first dashboard

Home:
- open incidents
- severity
- confidence
- source
- first/last seen
- decoy count
- behavior
- status

Health secondary.

Acceptance: AC-UX-001.

## P3-W14 — Incident detail

Sections:
1. What happened
2. Confidence/severity
3. Source
4. Journey
5. Evidence
6. AI placeholder/pending section
7. Guidance placeholder
8. status/disposition
9. audit

Acceptance: AC-UX-002..005.

## P3-W15 — Incident-scoped evidence explorer

Basic filter/search:
- time
- protocol
- decoy
- action
- source
- evidence ID

No SIEM query language.

# 4. North-Star test scenario

Required deterministic test:

```text
Attacker host
  → SMB probe
  → HTTP admin discovery
  → SSH login attempts
  → valid synthetic credential
  → SSH commands
  → PostgreSQL auth attempt
```

Expected:
- one or correctly linked incident journey according approved windows,
- ordered observed evidence,
- source IP/zone,
- confidence high,
- severity high/critical according mapping,
- no AI required.

# 5. Exit gate

- [ ] six rule classes live
- [ ] incident lifecycle live
- [ ] multi-decoy correlation passes
- [ ] separate actor test passes
- [ ] merge/split passes
- [ ] confidence/severity separate
- [ ] known scanner suppression passes
- [ ] dedup passes
- [ ] incident-first UX passes
- [ ] attacker journey readable
- [ ] observed/inferred distinction exists
- [ ] North-Star deterministic scenario passes without AI

# 6. Product state after phase

At Phase 3 exit, core product promise is visible for the first time: **high-confidence internal breach evidence + attacker journey**. Phase 4 makes it understandable/actionable for the primary persona.

---

## Final Phase Status

- **Phase:** Phase 3 — Detection, Incident & Attacker Journey
- **Status:** APPROVED / FINAL
- **Owner decision:** Tüm bu faz kapsamı, sequencing, dependencies, acceptance criteria ve exit gate'leri onaylandı.
- **Change policy:** Bu fazın kapsamı implementation sırasında sessizce genişletilemez/daraltılamaz; değişiklik Product Owner change-control sürecine tabidir.
