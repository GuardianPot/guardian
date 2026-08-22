# Phase 4 — AI, Guidance, Notifications & Operator Workflow
## Final Approved Implementation Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Faz amacı

Phase 3'te oluşan güvenilir incident/evidence yapısını, security specialist olmayan operator için:
- anlaşılır,
- önceliklendirilebilir,
- aksiyon alınabilir

hale getirmek.

AI burada **truth source değil enrichment/reasoning layer** olarak eklenir.

# 2. Dependencies

- deterministic incident model stable
- evidence IDs/provenance stable
- confidence/severity stable
- incident UI stable
- curated action taxonomy
- AI provider abstraction from Step 4

# 3. Workstreams

## P4-W1 — Incident Context Package

Deterministic builder produces:
- incident snapshot/version
- evidence summaries
- evidence IDs
- source context
- timeline
- confidence factors
- severity
- uncertainty
- environment context
- untrusted attacker fields separated

Acceptance:
- same incident snapshot creates stable hash,
- attacker command never becomes system instruction field.

## P4-W2 — AI provider gateway

Implement:
- common Go interface
- OpenAI adapter
- Anthropic adapter
- capability profile config
- timeout/cancel
- token/cost metadata
- structured output validation

No provider SDK calls outside adapter package.

Trace: AIM-07, AC-AI-001/002.

## P4-W3 — Structured incident analysis schema

Schema:
- concise summary
- factual statements with evidence IDs
- hypotheses
- uncertainty
- severity rationale
- prioritized actions
- limitations

Validation:
- unknown evidence ID invalid
- factual claim without evidence ref rejected/reclassified

Acceptance: AC-AI-003.

## P4-W4 — Prompt injection boundary

Implement:
- explicit untrusted data sections
- size limits
- escaping/serialization
- prompt templates versioned
- hostile corpus
- no tool calling

Acceptance:
- AC-AI-005
- AIM-10 release blocker tests.

## P4-W5 — AI asynchronous jobs

Incident creation does not wait.

Flow:
incident snapshot → enqueue durable AI job → call provider → validate → persist → UI update.

Failure:
- timeout
- rate limit
- malformed structured output
- provider outage

all produce explicit status.

Acceptance: AC-AI-004.

## P4-W6 — AI caching/dedup/budget

Key:
incident snapshot hash + prompt/schema version + model profile.

Controls:
- per incident call limit
- retry cap
- environment budget
- event flood never directly maps to LLM calls

Acceptance: AC-AI-007.

## P4-W7 — Curated response action catalog

Create versioned action objects:
- category
- title
- rationale
- prerequisites
- impact/risk
- reversible?
- suggested evidence triggers
- escalation note

MVP categories:
1. Verify/investigate
2. Containment consideration
3. Recovery/follow-up

No execution endpoint.

## P4-W8 — Guidance composer

AI may:
- select/rank appropriate curated actions
- contextualize explanation

Deterministic fallback:
- rule/incident type maps to generic curated actions.

Acceptance:
- AC-RG-001..003.

## P4-W9 — Incident explanation UI

Show:
- AI status
- evidence-backed summary
- citations to evidence
- hypotheses separately
- uncertainty
- generated-at/model metadata if operator detail opens

Observed evidence remains visually primary.

## P4-W10 — In-product notifications

Notification center:
- new incident
- material severity/confidence escalation
- delivery/read status

No raw event notifications.

Acceptance: AC-NT-001/004.

## P4-W11 — Email notification

Payload:
- incident title
- source
- severity/confidence
- key summary
- console link
- no sensitive raw credential

Acceptance: AC-NT-002.

## P4-W12 — Generic webhook

Implement:
- structured incident payload
- signed/authenticated delivery
- retry/backoff
- delivery audit
- secret rotation

Acceptance: AC-NT-003.

## P4-W13 — Operator acknowledgement/disposition flow

From incident:
- acknowledge
- investigating
- benign/expected
- confirmed
- resolved

Feedback can propose suppression, not silently self-learn.

## P4-W14 — AI eval suite

Golden fixtures:
- recon
- credential abuse
- multi-decoy journey
- benign scanner
- ambiguous source
- prompt injection
- provider output malformed

Metrics:
- schema compliance
- evidence citation coverage
- unsupported claim rate
- unsafe action rate
- injection resistance

Owner-approved thresholds finalized before Phase 5 release gate.

# 4. Non-goals

- AI tool execution
- automatic containment
- attacker-facing LLM
- adaptive deception
- live web search
- vector DB/RAG
- autonomous correlation truth

# 5. Exit gate

- [ ] OpenAI adapter contract pass
- [ ] Anthropic adapter contract pass
- [ ] evidence-cited structured output
- [ ] provider outage non-blocking
- [ ] prompt injection corpus pass
- [ ] no tools/automatic actions
- [ ] curated guidance three layers
- [ ] deterministic fallback guidance
- [ ] in-product + email + webhook
- [ ] operator disposition flow
- [ ] AI budget/dedup
- [ ] eval suite baseline established

# 6. Product state after phase

The product now expresses the full user outcome: **detect → understand → decide what to do next**. It is not yet pilot-ready until Phase 5 hardening/release gates pass.

---

## Final Phase Status

- **Phase:** Phase 4 — AI, Guidance, Notifications & Operator Workflow
- **Status:** APPROVED / FINAL
- **Owner decision:** Tüm bu faz kapsamı, sequencing, dependencies, acceptance criteria ve exit gate'leri onaylandı.
- **Change policy:** Bu fazın kapsamı implementation sırasında sessizce genişletilemez/daraltılamaz; değişiklik Product Owner change-control sürecine tabidir.
