# Step 6 — Roadmap / Version & Capability Phasing
## Master Roadmap — FINAL

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Amaç

Bu belge Step 6 içindeki bütün fazların master index'i ve sequencing gerekçesidir.

Step 6'nın temel sorusu:

> **Onaylanmış MVP ve uzun vadeli capability evrenini hangi bağımlılık sırasıyla implement edeceğiz; her faz hangi somut ürün çıktısını üretmeli ve hangi acceptance gate'i geçmeden sonraki faza geçmemeliyiz?**

Bu roadmap:
- development-time tahmini vermez,
- MVP scope'u yeniden seçmez,
- architecture'ı yeniden tasarlamaz,
- capability'leri bağımlılık ve risk sırasına koyar,
- AI coding agent'larına doğrudan verilebilecek work-package sınırları üretir.

# 2. Temel roadmap ilkeleri

1. **Vertical outcome before breadth.** Çok sayıda decoy yerine ilk önce end-to-end detection→incident→journey→guidance zinciri.
2. **Security boundary before attacker-facing capability.** Edge privilege, PKI, network placement, egress ve signed artifacts attacker-facing runtime'dan önce veya onunla birlikte doğrulanır.
3. **Deterministic truth before AI.** Evidence/correlation modeli AI integration'dan önce stabil olmalıdır.
4. **Operational health before pilot.** “Çalışıyor görünüyor ama coverage yok” kabul edilmez.
5. **Failure behavior is implementation, not polish.** Offline replay, restart, backpressure ve rollback MVP fazlarının parçasıdır.
6. **Post-MVP scope yalnız Step 5 non-goal listesinden kontrollü biçimde açılır.**
7. **Architecture spike sonucu APPROVED architecture ile çelişirse implementation zorlanmaz; architecture change-control açılır.**
8. **Her faz bir deployable/observable/testable ürün durumu bırakır.**

# 3. Faz haritası

| Faz | Ürün durumu | Ana amaç | MVP durumu |
|---|---|---|---|
| Phase 0 | Engineering/architecture substrate | Blocker spike'ları, repo, contracts, lab ve güvenlik temelini doğrulamak | MVP prerequisite |
| Phase 1 | Connected platform skeleton | Control Plane + Edge trust, enrollment, desired state, persistence, auth/health | MVP |
| Phase 2 | Working deception sensor | Networking, container runtime, 4 decoy family, canonical telemetry/evidence | MVP |
| Phase 3 | Security product core | Detection, correlation, incident, attacker journey, confidence/severity, incident-first UX | MVP |
| Phase 4 | Actionable operator experience | AI explanation, curated guidance, notifications, feedback/disposition | MVP |
| Phase 5 | Controlled pilot-ready release | Security hardening, updates, resilience, performance, release gates | **MVP COMPLETE** |
| Phase 6 | Deployment & operability expansion | Self-service, multi-Edge/network modes, on-prem/offline and broader connectivity | Post-MVP |
| Phase 7 | Deception breadth & Windows expansion | RDP/extra services/personas/tokens and richer Windows network deception | Post-MVP |
| Phase 8 | Endpoint & identity deception | Endpoint agent, breadcrumbs, directory/identity/cloud token deception | Post-MVP strategic |
| Phase 9 | High-interaction contained engagement | QEMU/KVM worker, real OS/app interaction, malware/file capture | Advanced |
| Phase 10 | Adaptive deception & fake network | Controlled redirection, coherent fake topology, adaptive/attacker-facing AI | Advanced |
| Phase 11 | Platform scale / MSSP / enterprise integration | Multi-tenancy, RBAC/SSO, integrations, scale-triggered data plane evolution | Growth/Platform |

# 4. Dependency DAG

```text
Phase 0
  ↓
Phase 1
  ↓
Phase 2
  ↓
Phase 3
  ↓
Phase 4
  ↓
Phase 5 ────────────────┐
  │                     │
  ├──→ Phase 6 ──┐      │
  │              ├──→ Phase 8
  ├──→ Phase 7 ──┘      │
  │                     │
  └────────→ Phase 9 ───┤
                         ↓
                     Phase 10

Phase 6 + validated product demand
               ↓
            Phase 11
```

Phase 6 ve Phase 7 bazı workstream'lerde paralel ilerleyebilir; ancak Phase 8 identity/endpoint capability'leri deployment/operations maturity'den yararlanır. Phase 10, Phase 9 containment modelini varsayar.

# 5. MVP completion definition

MVP yalnız Phase 5 tamamlandığında oluşur.

Phase 0–4 sonunda demo veya internally usable slices oluşabilir; bunlar **MVP complete** sayılmaz.

MVP release authority:
- Step 5 `Approved MVP Release Gate`
- Step 5 acceptance criteria catalog
- Phase 5 exit gate
- no unresolved blocker spike
- no unapproved scope/architecture deviation

# 6. Roadmap dosyaları

| Dosya | İçerik |
|---|---|
| `01_Phase_0_Architecture_Validation_and_Engineering_Foundation.md` | Blocker spike'lar, monorepo/CI/lab/contracts |
| `02_Phase_1_Core_Platform_and_Device_Trust.md` | CP/Edge enrollment, auth, persistence, desired state |
| `03_Phase_2_Deception_Runtime_Networking_and_Telemetry.md` | containerd, routed IP, decoy packs, canonical evidence |
| `04_Phase_3_Detection_Incident_and_Attacker_Journey.md` | detection/correlation/incidents/journey/noise/UX |
| `05_Phase_4_AI_Guidance_Notifications_and_Operator_Workflow.md` | AI, guidance, notifications, dispositions |
| `06_Phase_5_Pilot_Hardening_and_MVP_Release.md` | security/reliability/performance/update/release gates |
| `07_Phase_6_Post_MVP_Deployment_and_Operability_Expansion.md` | multi-edge, network modes, on-prem/offline |
| `08_Phase_7_Post_MVP_Deception_Breadth_and_Windows_Expansion.md` | RDP, more decoys, richer Windows deception |
| `09_Phase_8_Endpoint_and_Identity_Deception.md` | endpoint agent + credentials/identity |
| `10_Phase_9_High_Interaction_and_Contained_Engagement.md` | VM-grade high interaction |
| `11_Phase_10_Adaptive_Deception_and_Fake_Network.md` | adaptive/fake-network/redirection |
| `12_Phase_11_Platform_Scale_MSSP_and_Enterprise_Integrations.md` | MSSP, tenant/RBAC, integrations, scale |
| `13_Cross_Phase_Dependency_Traceability_and_Release_Gates.md` | dependency/AC/decision traceability |
| `14_AI_Agent_Execution_and_Work_Package_Governance.md` | agent execution model |

# 7. Version semantics recommendation

Faz adı product semantic version ile eş anlamlı değildir.

**APPROVED DECISION:**
- Phase 0–4: pre-MVP internal builds.
- Phase 5 exit: first controlled pilot MVP release line.
- Phase 6–7: post-MVP product capability releases.
- Phase 8–10: strategic capability families; gerekirse feature flag / limited preview.
- Phase 11: platform/business-model expansion.

Exact `v0.x/v1.x` numbering release hazırlığında belirlenebilir; roadmap dependency'si değildir.

# 8. Owner decisions in Step 6

Bu roadmap setindeki önemli kararlar:
- phase boundaries,
- capability sequencing,
- post-MVP ordering,
- blocker/parallelization policy,
- phase exit gates.

Bunların tamamı Product Owner tarafından **APPROVED** edilmiştir. Step 6 final dosya seti bu belgelerden oluşur.

---

# 9. Step 6 Closure

Step 6, Product Owner onayıyla **CLOSED / APPROVED** durumundadır.

- Faz sınırları: APPROVED
- MVP implementation sırası (Phase 0–5): APPROVED
- MVP completion noktası (Phase 5 exit): APPROVED
- Post-MVP sequencing (Phase 6–11): APPROVED
- Cross-phase dependency model: APPROVED
- Acceptance / release-gate modeli: APPROVED
- AI agent work-package governance: APPROVED
- Development-time estimate kullanılmaması: APPROVED
- Açık roadmap owner decision: 0

Bu roadmap bundan sonra implementation execution için bağlayıcı baseline'dır. Faz veya capability sırası değiştirilecekse ilgili change-control süreci işletilmelidir.

---

# 10. Next Step

Step 6'nın kapanmasıyla bir sonraki mantıksal aşama:

> **Repository Setup & AI-Agent Development Workflow → ardından Phase 0 implementation execution**

olur.

Bu aşamada:
- GitHub monorepo oluşturulur,
- repository skeleton ve governance kuralları uygulanır,
- CI/ADR/CODEOWNERS/PR policy hazırlanır,
- Phase 0 work-package'ları issue/task seviyesine ayrılır,
- AI coding agent'larının izinleri ve stop/escalate kuralları teknik olarak enforce edilir,
- ardından Phase 0 blocker spike'ları ve engineering foundation implementasyonuna başlanır.
