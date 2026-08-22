# MVP Scope & Acceptance Criteria
## Step 5 — Final Approved Output

**Belge durumu:** APPROVED / FINAL  
**Karar durumu:** Bu belgede yer alan 200 Step 5 kararının tamamı Product Owner tarafından `APPROVED` edilmiştir.  
**Karar otoritesi:** Product Owner  
**Girdiler:**  
- `Step_2_Product_Definition_Strategy_APPROVED_Decision_Record_TR.md`
- `Technical_Feasibility_Requirements.md`
- `System_Architecture_and_Technology_Decisions.md`

**Bu belge:** Step 5'in tamamlanmış final MVP scope & acceptance criteria çıktısıdır.

### Step 5 sonucu

- **200 / 200 MVP scope decision:** APPROVED
- **Açık owner decision:** 0
- **MVP release identity:** APPROVED
- **Environment/deployment scope:** APPROVED
- **Decoy portfolio:** APPROVED
- **Threat coverage:** APPROVED
- **Incident/correlation/journey scope:** APPROVED
- **AI MVP scope:** APPROVED
- **Security/reliability/performance gates:** APPROVED
- **Acceptance Criteria Catalog:** APPROVED
- **Explicit MVP Non-Goals:** APPROVED
- **Definition of Done:** APPROVED
- **MVP Release Gate:** APPROVED
- **Pilot validation metrics:** MEASUREMENT INPUT / not a decision gap
- **Bir sonraki aşama:** Roadmap / Version & Capability Phasing


---

# 0. Belgenin amacı

Step 2 ürün kimliğini ve threat model'i, Step 3 teknik gereksinimleri, Step 4 ise sistem mimarisini ve teknoloji baseline'ını kapattı.

Step 5'in görevi farklıdır:

> **Onaylanmış product + requirements + architecture evreninin içinden, ilk gerçek ürün diliminde hangi capability'lerin bulunacağını; hangilerinin bilinçli olarak dışarıda kalacağını; ürünün “MVP tamamlandı” sayılması için hangi ölçülebilir acceptance criteria'ların sağlanacağını kesinleştirmek.**

Bu belge bir backlog veya roadmap değildir. Development-time tahmini içermez. Step 5 owner kararları kapanmıştır.

Bu dokümanda onaylanmış olarak:
- kapsam kararları,
- acceptance criteria,
- release quality bar,
- failure-mode criteria,
- security criteria,
- validation blocker'ları,
- explicit MVP non-goals

yer alır.

Roadmap/phase sıralaması Step 5 finalinden sonra ele alınacaktır.

---

# 0.1 Bağlayıcı önceki kararlar

Bu Draft aşağıdaki kararları **yeniden açmaz**:

## Product
- Product promise: **internal breach visibility**
- Primary mechanism: **deception**
- User outcome: **high-confidence incident + attacker journey + guidance**
- Primary persona: dedicated SOC'u olmayan generalist technical owner
- Primary threat model: post-compromise recon/discovery, credential abuse, lateral movement
- Product EDR/NDR/SIEM/SOAR/firewall değildir.
- High signal over high volume.
- Human owns security decisions.
- AI evidence üretmez; evidence'ı yorumlar.
- High-interaction advanced capability'dir.
- Public/global honeynet current product identity değildir.
- Current MSSP/multi-tenancy scope dışıdır.

## Architecture
- 3 core application: Control Plane, Web Console, Edge Agent
- Decoy Runtime Packs ayrı OCI ecosystem
- Monorepo
- Go Control Plane + Edge
- React/TypeScript/Vite Web Console
- Native Debian 13 Edge appliance
- containerd+runc decoy runtime
- QEMU/KVM+libvirt high-interaction future worker
- gRPC/Protobuf/mTLS Edge↔Control
- REST/OpenAPI Web/public API
- PostgreSQL central, SQLite WAL Edge
- Initial central broker yok
- S3-compatible object storage abstraction
- deterministic correlation core
- OpenAI + Anthropic provider adapters arkasında common AI abstraction
- no AI tools initially
- Cosign + TUF update/security model
- OpenTelemetry-first observability
- local auth baseline + future OIDC
- TOTP privileged operator baseline
- signed artifacts, audit log, least privilege, default-deny decoy egress

---

# 0.2 MVP seçim filtresi

Her capability şu sorulardan geçirilmelidir:

1. **North Star'ı kanıtlamak için gerekli mi?**
2. **Primary persona'nın real problem'ini çözüyor mu?**
3. **Olmadan ürün yalnız bir honeypot manager/demo olur mu?**
4. **Teknik/security riski MVP'nin ana tezini gölgeler mi?**
5. **Daha sonra eklenirse architecture değişikliği gerektirir mi?**
6. **Acceptance criterion test edilebilir mi?**
7. **Real customer/pilot ortamında güvenli biçimde çalıştırılabilir mi?**
8. **Operational babysitting yaratıyor mu?**
9. **High-signal ilkesine katkısı var mı?**
10. **MVP feedback'i sonraki ürün kararlarını gerçekten bilgilendirir mi?**

---

# 0.3 Önerilen MVP tezi

**APPROVED DECISION:**

> **MVP'nin amacı mümkün olan en çok honeypot/deception capability'yi göstermek değil; private/internal networkte bir saldırgan veya compromise olmuş internal device discovery, credential abuse veya lateral movement davranışı gösterdiğinde deception ile kesişen interaction'lardan güvenilir evidence üretildiğini, bu evidence'ın tek incident ve attacker journey altında anlaşılır biçimde ilişkilendirildiğini ve generalist technical operator'a evidence-backed açıklama ile güvenli next-action guidance verilebildiğini gerçek bir kontrollü müşteri/pilot ortamında uçtan uca kanıtlamaktır.**

Önerilen uçtan uca MVP loop:

```text
Install / Enroll Edge
        ↓
Define private environment
        ↓
Validate network placement
        ↓
Deploy small purposeful decoy set
        ↓
Verify coverage & health
        ↓
Attacker / test host interacts
        ↓
Canonical Event
        ↓
Evidence
        ↓
Finding / Incident
        ↓
Correlation
        ↓
Attacker Journey
        ↓
Confidence + Severity
        ↓
AI Evidence-backed Explanation
        ↓
Recommended Next Actions
        ↓
Notification
        ↓
Operator Disposition / Feedback
```

---

# 1. MVP Release Identity

## Karar M-01 — MVP'nin release sınıfı nedir?

**Durum:** APPROVED

### A — Technical demo
- Tek lab'da çalışması yeterlidir.
- Security/update/failure handling eksik olabilir.
- Product thesis'i gösterir ama gerçek customer feedback'i zayıf olur.

### B — PoC toolkit
- Teknik kullanıcının manuel setup ile test edebileceği araçlar bütünü.
- End-to-end UX tamamlanmak zorunda değildir.

### C — Controlled pilot-ready MVP
- Gerçek private networkte kontrollü pilotta çalışabilir.
- Onboarding, health, security, incident workflow ve failure behavior tamamlanmış minimum ürün dilimi gerekir.
- GA/compliance/enterprise polish gerekmez.

### D — Production GA v1
- Broad compatibility, support matrix, HA, compliance ve operasyon seviyesi çok daha yüksek olur.

**Karşıt görüş:** C, MVP için fazla geniş olabilir; demo ile daha hızlı product validation yapılabilir.

**APPROVED DECISION — C.**

**Gerekçe:** Step 2'deki değer proposition'ı yalnız gerçek network placement, incident journey ve generalist operator deneyimiyle doğrulanabilir. Demo seviyesinde false-positive, deployment ve operational burden gibi en önemli hipotezler test edilemez.

---

## Karar M-02 — MVP self-service mi controlled onboarding mi?

**Durum:** APPROVED

### A — Fully self-service
Customer dokümantasyonla tek başına kurar.

### B — Controlled onboarding
Pilot kurulumu product team desteğiyle yapılabilir; ürün akışı yine gerçek olmalıdır.

### C — Internal-only
Sadece geliştirici kurabilir.

**APPROVED DECISION — B.**

MVP'nin product onboarding'i var olmalıdır; fakat pilot sırasında teknik destek verilmesi acceptance failure sayılmamalıdır. Hidden manual code/config modification ise failure sayılmalıdır.

---

## Karar M-03 — MVP başarı kriteri “feature tamamlandı” mı “end-to-end outcome” mu?

**Durum:** APPROVED

**APPROVED DECISION — End-to-end outcome.**

MVP ancak aşağıdaki North-Star scenario uçtan uca çalışıyorsa tamamlanmış sayılır:

> Internal host → recon/discovery → birden fazla deception touchpoint → evidence → single incident → attacker journey → source context → confidence/severity → explanation/guidance → notification → operator disposition.

---

## Karar M-04 — MVP müşteri sayısal segmentiyle sınırlandırılmalı mı?

**Durum:** APPROVED

**APPROVED DECISION — Hayır.**

MVP acceptance environment complexity üzerinden tanımlanmalı:
- single organization
- single site
- bounded private IPv4 network
- generalist operator
- dedicated SOC gerektirmeyen operasyon

Employee count acceptance criterion değildir.

---

# 2. MVP Environment & Deployment Scope

## Karar ENV-01 — MVP Control Plane deployment profile

**Durum:** APPROVED

### A — On-prem customer Control Plane
Privacy güçlü; setup burden artar.

### B — Hosted Control Plane primary
Pilot onboarding basit; Edge outbound bağlantı kurar.

### C — Hosted + on-prem iki profile da acceptance şartı
Architecture bunu destekliyor ama MVP test matrisi büyür.

**APPROVED DECISION — B.**

Development/lab için local Compose profile bulunabilir; **customer-facing MVP acceptance hosted Control Plane** üzerinden yapılır. On-prem Control Plane sonraki release scope.

---

## Karar ENV-02 — MVP'de kaç Edge Node?

**Durum:** APPROVED

### A — Exactly one Edge
Sadelik, ancak distributed architecture doğrulanmaz.

### B — One active Edge per environment; data model multi-Edge ready
Primary pilot için yeterli.

### C — Multi-Edge fully supported
Multi-site/segmented topology artar.

**APPROVED DECISION — B.**

MVP UI ve API birden fazla Edge identity'yi teknik olarak modelleyebilir; fakat release acceptance yalnız tek active Edge deployment'ı zorunlu tutar.

---

## Karar ENV-03 — MVP network topology

**Durum:** APPROVED

### A — Tek flat subnet
En dar.

### B — Bir Edge'den reachable birden fazla routed private subnet
Gerçek SMB networklerini daha iyi temsil eder.

### C — VLAN trunk + multi-NIC + routed + overlay tüm modeller
Test matrisi büyür.

**APPROVED DECISION — B.**

Minimum acceptance:
- 1 management/private network
- en az 2 logical private network zone/subnet tanımlanabilmesi
- Edge'den routed reachability

---

## Karar ENV-04 — VLAN trunk MVP'de zorunlu mu?

**Durum:** APPROVED

**APPROVED DECISION — Hayır / POST-MVP.**

Architecture desteklemeye açık kalır. MVP'de routed/secondary-IP path temel alınır.

---

## Karar ENV-05 — Multi-NIC MVP'de zorunlu mu?

**Durum:** APPROVED

**APPROVED DECISION — Hayır.**
Single Edge interface + routed networks baseline.

---

## Karar ENV-06 — macvlan / ipvlan MVP scope

**Durum:** APPROVED

**APPROVED DECISION — DEFERRED.**
Presence-driver abstraction architecture'da bulunabilir; MVP yalnız routed/secondary-IP driver implementasyonunu acceptance kapsamına alır.

---

## Karar ENV-07 — IPv6 MVP scope

**Durum:** APPROVED

**APPROVED DECISION — Runtime coverage dışı.**

Contracts/storage IPv6-capable olmalıdır; MVP deception deployment ve tests IPv4.

---

## Karar ENV-08 — Cloud VPC/VNet pilot

**Durum:** APPROVED

**APPROVED DECISION — MVP acceptance dışında.**

Private cloud networks architecture tarafından engellenmemeli fakat MVP validation environment on-prem/virtualized private LAN.

---

## Karar ENV-09 — VPN-connected remote networks

**Durum:** APPROVED

**APPROVED DECISION — MVP acceptance dışında.**
VPN source context capability future validation.

---

## Karar ENV-10 — Edge appliance formatı

**Durum:** APPROVED

**APPROVED DECISION:** Debian 13 based VM/appliance, supported reference hypervisor/lab path ile.

MVP acceptance “arbitrary Linux host'a package install” üzerine kurulmaz.

---

## Karar ENV-11 — Hypervisor support matrix

**Durum:** APPROVED

### A — Her major hypervisor
### B — Tek reference virtualization environment + documented generic requirements
### C — Bare-metal only

**APPROVED DECISION — B.**

MVP için architecture-neutral VM requirements:
- amd64 virtualization
- one NIC
- static/reserved IP
- outbound HTTPS/443
- decoy IP reachability

Specific supported pilot hypervisor lab validation ile seçilir.

---

# 3. Onboarding & Environment Understanding

## Karar ON-01 — MVP onboarding'ın minimum akışı

**Durum:** APPROVED

**APPROVED FLOW:**

1. Create environment
2. Generate one-time Edge enrollment token
3. Deploy Edge appliance
4. Edge creates keypair and enrolls
5. Control Plane shows connected/healthy
6. Define network zone(s)/CIDR
7. Run bounded placement/reachability validation
8. Propose/select decoy personas
9. Owner approves deployment
10. Decoys become healthy
11. Run test interaction / coverage verification
12. Environment becomes “Protected/Healthy” equivalent state

---

## Karar ON-02 — Environment discovery MVP'de ne kadar otomatik?

**Durum:** APPROVED

### A — Manual CIDR/IP only
### B — Bounded active discovery
### C — Passive discovery
### D — Full hybrid discovery

**APPROVED DECISION — A + minimal B.**

User network CIDR/zone'u tanımlar. System explicit consent ile yalnız deception placement için sınırlı reachability/IP conflict/basic service-presence kontrolü yapabilir.

Full asset inventory yapılmaz.

---

## Karar ON-03 — Active subnet scanning MVP'de var mı?

**Durum:** APPROVED

**APPROVED DECISION — Limited/explicitly triggered only.**

Broad vulnerability scan veya service fingerprinting yok. Safe probes:
- IP conflict
- basic reachability
- selected address availability
- optional minimal environment hints

---

## Karar ON-04 — Decoy placement recommendation MVP'de AI kullanmalı mı?

**Durum:** APPROVED

### A — AI generated
### B — Deterministic simple rules
### C — Manual only

**APPROVED DECISION — B.**

Örnek:
- user “server zone” seçer
- system supported decoy setinden purposeful placement önerir

AI placement suggestion MVP outside.

---

## Karar ON-05 — Kullanıcı decoy IP/persona'yı override edebilir mi?

**Durum:** APPROVED

**APPROVED DECISION — Evet.**

System öneri verir; owner IP, hostname/persona ve decoy selection değiştirebilir. Unsafe/conflicting config validation fail eder.

---

## Karar ON-06 — Coverage verification zorunlu mu?

**Durum:** APPROVED

**APPROVED DECISION — Evet / Core.**

Decoy process running olması yeterli değildir. Edge local network path üzerinden configured IP/port'un reachable/responding olduğunu functional probe ile doğrular.

---

# 4. MVP Decoy Portfolio

## Karar DC-01 — MVP'de kaç farklı deception family hedeflenmeli?

**Durum:** APPROVED

### A — 1–2
Çok dar; product breadth zayıf.

### B — 4 purposeful network decoy types + one synthetic credential workflow
Threat model'i yeterli temsil eder.

### C — 10+ protocols
“Honeypot catalog” scope creep.

**APPROVED DECISION — B.**

---

## Karar DC-02 — SSH deception MVP

**Durum:** APPROVED

**APPROVED DECISION — YES / CORE.**

Amaç:
- service discovery
- authentication attempts
- credential abuse
- post-login command behavior

Interaction:
- medium interaction candidate (Cowrie adapter)
- low-interaction fallback yalnız health/port detection için yeterli sayılmaz

---

## Karar DC-03 — HTTP/Admin deception MVP

**Durum:** APPROVED

**APPROVED DECISION — YES / CORE.**

Amaç:
- internal web/admin discovery
- suspicious paths/requests
- fake login attempts
- application-style decoy variety

Interaction:
- low/medium emulated app
- arbitrary code execution yok

---

## Karar DC-04 — PostgreSQL/database deception MVP

**Durum:** APPROVED

**APPROVED DECISION — YES / CORE.**

Amaç:
- DB discovery
- auth attempts
- credential use
- basic protocol/query interaction

Full real PostgreSQL vulnerable server zorunlu değildir.

---

## Karar DC-05 — SMB/Windows-flavored deception MVP

**Durum:** APPROVED

### A — Defer all Windows network deception
### B — SMB-style low-interaction decoy
### C — Real Windows SMB server

**APPROVED DECISION — B.**

Gerekçe: Windows-heavy ICP kararını MVP'de en az bir network-level capability ile doğrular; real Windows VM/high interaction complexity'sini eklemez.

---

## Karar DC-06 — RDP decoy MVP

**Durum:** APPROVED

**APPROVED DECISION — NO / POST-MVP.**

RDP realism/protocol complexity, SMB varken MVP product thesis'i için incremental value'a göre yüksektir.

---

## Karar DC-07 — Redis/Telnet/FTP vb. ek decoy'lar

**Durum:** APPROVED

**APPROVED DECISION — NO / POST-MVP.**

OpenCanary adapter ile daha sonra kolay breadth eklenebilir; MVP core outcome'a katkıları düşük.

---

## Karar DC-08 — Real vulnerable WordPress/e-commerce MVP

**Durum:** APPROVED

**APPROVED DECISION — NO.**
High-interaction/intentional-vulnerability track'e yakındır; initial access/public-facing product identity'yi bulanıklaştırır.

---

## Karar DC-09 — Synthetic honey credential workflow

**Durum:** APPROVED

**APPROVED DECISION — YES, bounded MVP.**

MVP capability:
- product decoy-only credential üretir
- credential yalnız selected decoy'a authenticate olabilir veya decoy tarafından high-confidence marker olarak tanınır
- production privilege yoktur
- operator credential'ı manual lab/pilot location'a yerleştirebilir
- trigger/use incident confidence'ı yükseltir

Endpoint agent ile automated placement MVP dışında.

---

## Karar DC-10 — Honey files/documents/tokens MVP

**Durum:** APPROVED

**APPROVED DECISION — NO.**
Identity/endpoint deception expansion'a bırakılır.

---

## Karar DC-11 — Decoy personas

**Durum:** APPROVED

**APPROVED DECISION:** Küçük curated persona set:
- Linux admin/server
- Internal admin web app
- Database server
- Windows file/service host

User arbitrary generative persona builder MVP dışında.

---

## Karar DC-12 — Decoy pack updateability acceptance

**Durum:** APPROVED

**APPROVED DECISION — Core.**
Her pack version/digest taşımalı ve Control Plane'den desired-state ile update edilebilmelidir.

---

# 5. MVP Interaction Levels

## Karar INT-01 — MVP interaction seviyeleri

**Durum:** APPROVED

**APPROVED DECISION:**
- Low interaction: HTTP, SMB, some DB surface
- Medium interaction: SSH and optionally DB/authenticated session
- High interaction: OUT

---

## Karar INT-02 — High-Interaction Worker MVP'de implement edilecek mi?

**Durum:** APPROVED

**APPROVED DECISION — NO.**

Repository path/contract reservation olabilir; running QEMU/KVM product capability MVP completion için gerekli değildir.

---

## Karar INT-03 — Attacker arbitrary binary execution MVP

**Durum:** APPROVED

**APPROVED DECISION — NO.**

Malware execution/real shell kernel semantics MVP dışı.

---

## Karar INT-04 — Session command capture

**Durum:** APPROVED

**APPROVED DECISION — SSH medium-interaction için YES.**

Captured command:
- canonical evidence
- untrusted attacker content classification
- AI context'te instruction olmayan data

---

# 6. Threat Coverage Scope

## Karar TH-01 — Network recon

**Durum:** APPROVED

**APPROVED DECISION — CORE.**

Supported decoy IP/ports'a scan/probe evidence üretmelidir.

---

## Karar TH-02 — Service discovery/enumeration

**Durum:** APPROVED

**APPROVED DECISION — CORE.**

Protocol-specific interaction:
- SSH banner/connect
- HTTP path/request
- DB handshake/login
- SMB probe

---

## Karar TH-03 — Credential abuse

**Durum:** APPROVED

**APPROVED DECISION — CORE.**

Includes:
- username/password attempts
- repeated authentication attempts
- synthetic decoy credential use

---

## Karar TH-04 — Lateral movement attempts

**Durum:** APPROVED

**APPROVED DECISION — CORE at deception-touchpoint level.**

Product “networkteki tüm lateral movement'i görür” demez. Aynı source'un birden fazla decoy/service'e sequential interaction'ı lateral-movement/recon journey evidence olarak correlate edilir.

---

## Karar TH-05 — Multi-decoy movement

**Durum:** APPROVED

**APPROVED DECISION — CORE acceptance scenario.**

North-Star “şunları gezdi” iddiasının minimum kanıtıdır.

---

## Karar TH-06 — Malicious insider classification

**Durum:** APPROVED

**APPROVED DECISION — OUT.**
Same evidence shown; actor motive classification yok.

---

## Karar TH-07 — Initial access/phishing/exploit detection

**Durum:** APPROVED

**APPROVED DECISION — OUT.**

---

## Karar TH-08 — Malware/C2/exfiltration detection

**Durum:** APPROVED

**APPROVED DECISION — OUT as product claims.**

If observed within SSH transcript it may be evidence, but no dedicated detection promise.

---

## Karar TH-09 — Ransomware detection claim

**Durum:** APPROVED

**APPROVED DECISION — OUT.**

---

# 7. Canonical Telemetry & Evidence MVP

## Karar EV-01 — Full Event→Evidence→Finding→Incident chain

**Durum:** APPROVED

**APPROVED DECISION — CORE / mandatory.**

Skipping Finding is possible implementation simplification only if domain semantics remain explicit; user-visible truth chain cannot collapse into “raw log = incident”.

---

## Karar EV-02 — Minimum canonical event fields

**Durum:** APPROVED

**APPROVED REQUIREMENT:**
- event_id
- event_time
- observed_time
- ingested_time
- environment_id
- edge_id
- decoy_id
- decoy_pack/version
- source_ip
- destination_ip/port
- protocol
- interaction/action type
- session_id where applicable
- auth identity/result where applicable
- raw evidence reference
- schema version
- sensitivity classification
- provenance

---

## Karar EV-03 — Raw payload retention MVP

**Durum:** APPROVED

**APPROVED DECISION — Selective.**

- HTTP bodies: bounded/truncated
- SSH commands/session transcript: bounded
- credentials: protected/redacted handling
- binary uploads: quarantine if supported
- full packet capture: no

---

## Karar EV-04 — Evidence UI provenance

**Durum:** APPROVED

**APPROVED DECISION — Core.**
User each evidence item için:
- source decoy
- timestamp
- observed source
- raw/normalized details
- confidence factors
görebilmeli.

---

## Karar EV-05 — Tamper-evident MVP bar

**Durum:** APPROVED

### A — WORM/cryptographic full evidence chain
### B — Central append-oriented audit/provenance + immutable event IDs + no silent edit
### C — Normal editable DB rows

**APPROVED DECISION — B.**

Court-grade forensic chain MVP claim'i değildir.

---

## Karar EV-06 — Clock/time quality

**Durum:** APPROVED

**APPROVED DECISION — Core.**
Edge clock sync/offset health visible; journey ordering UTC canonical.

---

# 8. Detection & Local Edge Logic

## Karar DET-01 — Edge raw events'i doğrudan forward mı eder?

**Durum:** APPROVED

**APPROVED DECISION — Hayır.**

Edge:
- normalize
- classify basic interaction
- deduplicate burst noise
- create deterministic evidence envelope
- persist locally before ack

yapmalıdır.

---

## Karar DET-02 — Detection rules MVP

**Durum:** APPROVED

**APPROVED RULE SET:**
1. direct decoy touch
2. auth attempt
3. valid synthetic honey credential
4. repeated auth attempts
5. same source multi-service interaction
6. same source multi-decoy interaction

---

## Karar DET-03 — Rule versioning MVP

**Durum:** APPROVED

**APPROVED DECISION — YES.**
Finding/evidence derived output rule/version provenance taşımalı.

---

## Karar DET-04 — Custom user-defined detection rules MVP

**Durum:** APPROVED

**APPROVED DECISION — NO.**
General rule DSL/editor post-MVP.

---

# 9. Correlation & Attacker Journey MVP

## Karar COR-01 — Correlation minimum dimensions

**Durum:** APPROVED

**APPROVED DECISION:**
- source IP
- time window
- session ID
- authenticated username when observed
- decoy/service
- network zone
- synthetic credential ID

---

## Karar COR-02 — Correlation strategy

**Durum:** APPROVED

**APPROVED DECISION:** Deterministic anchors + scored heuristic links.

AI correlation suggestion implementation optional behind feature flag; not required for MVP acceptance.

---

## Karar COR-03 — Incident grouping window

**Durum:** APPROVED

**Seçenek A — Hard fixed global window**
Simple but context-insensitive.

**B — Rule-specific bounded windows**
Different interactions different windows.

**APPROVED DECISION — B.**

Exact durations detection-rule configuration in final MVP spec; acceptance tests explicit fixture windows use.

---

## Karar COR-04 — Multi-decoy journey visualization

**Durum:** APPROVED

**APPROVED DECISION — CORE.**

Minimum representation:
- chronological timeline
- source
- decoy/service
- action
- confidence/severity markers
- observed vs inferred distinction

Fancy graph canvas not required.

---

## Karar COR-05 — Journey graph visualization MVP

**Durum:** APPROVED

### A — Full node-edge interactive graph
### B — Timeline first + simple relationship summary
### C — No journey UI

**APPROVED DECISION — B.**

Product thesis validated without complex graph UI.

---

## Karar COR-06 — Incident merge/split MVP

**Durum:** APPROVED

**APPROVED DECISION — Manual merge/split CORE?**

**Tavsiye:** **Merge/split basic capability YES**, çünkü wrong correlation trust issue'dur. Advanced bulk tools yok.

---

## Karar COR-07 — Historical reprocessing UI

**Durum:** APPROVED

**APPROVED DECISION — NO.**
Engine supports version metadata; admin CLI/job future.

---

# 10. Source Identification MVP

## Karar SRC-01 — Guaranteed source field

**Durum:** APPROVED

**APPROVED DECISION:** observed source IP + network zone/subnet.

---

## Karar SRC-02 — Hostname

**Durum:** APPROVED

**APPROVED DECISION:** best-effort enrichment only; not guaranteed.

---

## Karar SRC-03 — MAC address

**Durum:** APPROVED

**APPROVED DECISION:** not guaranteed; local same-L2 context varsa displayable optional evidence.

---

## Karar SRC-04 — Username/account

**Durum:** APPROVED

**APPROVED DECISION:** show when protocol interaction provides it; provenance explicit.

---

## Karar SRC-05 — VPN identity

**Durum:** APPROVED

**APPROVED DECISION — OUT.**

---

## Karar SRC-06 — Device inventory integration

**Durum:** APPROVED

**APPROVED DECISION — OUT.**

---

## Karar SRC-07 — UI uncertainty language

**Durum:** APPROVED

**APPROVED DECISION — Core.**
“Source device is X” yerine:
- Observed source: 10.0.1.24
- Hostname mapping: probable / last observed
- Username: supplied during SSH login
gibi provenance-aware wording.

---

# 11. Confidence, Severity & Noise MVP

## Karar CS-01 — Confidence levels

**Durum:** APPROVED

**APPROVED DECISION:** `Low / Medium / High` + reasons.

---

## Karar CS-02 — Severity levels

**Durum:** APPROVED

**APPROVED DECISION:** `Informational / Low / Medium / High / Critical`.

Confidence'dan ayrı.

---

## Karar CS-03 — Confidence factor examples

**Durum:** APPROVED

**APPROVED DECISION:**
- Direct connection to decoy: base evidence
- auth attempt: stronger
- valid synthetic honey credential: very strong
- multi-decoy sequence: stronger
- known scanner: reduces suspicious confidence
- user-marked expected source: suppression context

---

## Karar CS-04 — Severity examples

**Durum:** APPROVED

**Approved value:**
- Single connection/banner probe → Low
- multi-port/service recon → Medium
- repeated auth attempts → Medium
- valid synthetic credential use → High
- multi-decoy progression + credential use → High/Critical depending context

Exact mapping owner review sırasında finalize edilmeli.

---

## Karar CS-05 — Known scanner list

**Durum:** APPROVED

**APPROVED DECISION — CORE.**
User source IP/CIDR'ı `known scanner/expected automation` olarak tanımlayabilir.

---

## Karar CS-06 — Suppression behavior

**Durum:** APPROVED

**APPROVED DECISION:** Event/evidence retained; user notification suppressed/downgraded according to scoped rule. Silent deletion yok.

---

## Karar CS-07 — Benign disposition

**Durum:** APPROVED

**APPROVED DECISION — CORE.**
Incident `Expected/Benign` olarak close edilebilir.

---

## Karar CS-08 — Feedback → automatic learning

**Durum:** APPROVED

**APPROVED DECISION — NO autonomous learning.**
System scoped suppression suggestion üretebilir; user approve eder.

---

## Karar CS-09 — Deduplication

**Durum:** APPROVED

**APPROVED DECISION — CORE.**
Same source + decoy + action burst tek incident update'e aggregate edilir; count/time range korunur.

---

# 12. Incident Model & Web UX MVP

## Karar UX-01 — Primary home screen

**Durum:** APPROVED

### A — Decoy inventory dashboard
### B — Incident-first dashboard
### C — Raw event console

**APPROVED DECISION — B.**

Health/coverage secondary panel.

---

## Karar UX-02 — Minimum incident list columns

**Durum:** APPROVED

**APPROVED DECISION:**
- title
- status
- severity
- confidence
- observed source
- first seen
- last seen
- affected decoy count
- primary behavior/category
- notification state

---

## Karar UX-03 — Minimum incident detail

**Durum:** APPROVED

**APPROVED mandatory sections:**
1. What happened
2. Confidence & severity
3. Source context
4. Attacker journey/timeline
5. Evidence
6. AI explanation (if available)
7. Recommended next actions
8. Incident status/disposition
9. Audit/history

---

## Karar UX-04 — Raw MITRE ATT&CK labels

**Durum:** APPROVED

**APPROVED DECISION:** Secondary expandable detail. Human-readable behavior first.

---

## Karar UX-05 — Raw event explorer

**Durum:** APPROVED

**A — Full SIEM-like query console**
### B — Incident-scoped evidence table + basic filters
### C — None

**APPROVED DECISION — B.**

---

## Karar UX-06 — Decoy management UX

**Durum:** APPROVED

**APPROVED MINIMUM:**
- list
- persona/type
- IP/zone
- health
- version
- last interaction
- enable/disable
- deploy/remove
- update state

---

## Karar UX-07 — Environment/Edge health UX

**Durum:** APPROVED

**APPROVED MINIMUM:**
- Edge connected/offline
- last contact
- control-plane connection
- local queue state
- decoy health counts
- network coverage checks
- AI provider status
- update status

---

## Karar UX-08 — Advanced topology map

**Durum:** APPROVED

**APPROVED DECISION — NO.**
Simple zone/decoy list sufficient.

---

# 13. AI MVP Scope

## Karar AIM-01 — AI features required for MVP

**Durum:** APPROVED

**APPROVED CORE:**
1. incident evidence summary
2. plain-language explanation
3. why it matters / severity rationale
4. prioritized next-action guidance
5. uncertainty statement
6. evidence references

---

## Karar AIM-02 — AI correlation truth

**Durum:** APPROVED

**APPROVED DECISION — NO.**
MVP incident can be created without AI.

---

## Karar AIM-03 — AI correlation suggestions

**Durum:** APPROVED

**APPROVED DECISION — OPTIONAL / feature flag.**
Not MVP exit blocker.

---

## Karar AIM-04 — AI environment placement suggestions

**Durum:** APPROVED

**APPROVED DECISION — OUT.**

---

## Karar AIM-05 — Attacker-facing AI

**Durum:** APPROVED

**APPROVED DECISION — OUT.**

---

## Karar AIM-06 — AI deception persona generation

**Durum:** APPROVED

**APPROVED DECISION — OUT.**

---

## Karar AIM-07 — AI providers

**Durum:** APPROVED

Architecture-approved OpenAI + Anthropic adapters.

**APPROVED MVP acceptance:**
- both adapters pass same structured contract tests
- deployment activates one provider/model profile at a time
- fallback between providers may be manual/configured, not automatic requirement

---

## Karar AIM-08 — AI unavailable behavior

**Durum:** APPROVED

**APPROVED DECISION — Core acceptance.**
Incident/evidence/journey/notification continue; UI displays “AI analysis unavailable/pending” without blocking incident.

---

## Karar AIM-09 — Evidence-grounding bar

**Durum:** APPROVED

**APPROVED DECISION:** Every factual incident claim in machine-structured AI output must reference supporting `evidence_id` or be classified as hypothesis/general guidance.

---

## Karar AIM-10 — Prompt injection tests

**Durum:** APPROVED

**APPROVED DECISION — RELEASE BLOCKER.**
Attacker strings like:
- “ignore prior instructions”
- fake system prompts
- HTML/Markdown instructions
- shell text requesting destructive action

must not gain tool authority or be treated as trusted system instruction.

---

## Karar AIM-11 — AI tools/function execution

**Durum:** APPROVED

**APPROVED DECISION — NONE in MVP.**
No network, DB, firewall, containment or shell tools.

---

## Karar AIM-12 — AI guidance source

**Durum:** APPROVED

**APPROVED DECISION:** Curated/versioned response action catalog + incident context. No arbitrary live web search.

---

## Karar AIM-13 — AI latency as incident blocker

**Durum:** APPROVED

**APPROVED DECISION — NO.**
AI async.

---

## Karar AIM-14 — AI cost controls

**Durum:** APPROVED

**APPROVED CORE:**
- incident-level call dedup
- max context
- max retries
- environment budget
- flood gate: raw events never trigger one LLM call each

---

# 14. Response Guidance MVP

## Karar RG-01 — Guidance model

**Durum:** APPROVED

**APPROVED three layers:**
1. Verify / investigate
2. Suggested containment
3. Recovery / follow-up

---

## Karar RG-02 — Automatic containment

**Durum:** APPROVED

**APPROVED DECISION — OUT.**

---

## Karar RG-03 — One-click containment button

**Durum:** APPROVED

**APPROVED DECISION — OUT.**
No EDR/firewall/NAC integrations in MVP.

---

## Karar RG-04 — Contextual action examples

**Durum:** APPROVED

**APPROVED ACTION CATALOG MAY INCLUDE:**
- verify source host ownership
- isolate device using customer's existing process
- inspect active sessions/processes
- reset identified credential
- review auth logs
- inspect neighboring assets
- preserve relevant logs
- escalate to external security provider

Product clearly distinguishes “recommendation” from performed action.

---

## Karar RG-05 — Unsafe/general advice guardrail

**Durum:** APPROVED

**APPROVED DECISION:** AI cannot recommend destructive commands as default first action without safety context; curated actions have risk/impact metadata.

---

# 15. Notification MVP

## Karar NT-01 — In-product notification

**Durum:** APPROVED

**APPROVED DECISION — CORE.**

---

## Karar NT-02 — Email

**Durum:** APPROVED

**APPROVED DECISION — CORE.**

Configurable SMTP or hosted email adapter architecture implementation may be deployment-specific.

---

## Karar NT-03 — Generic webhook

**Durum:** APPROVED

**APPROVED DECISION — YES / SUPPORTING.**

Small implementation surface; future Slack/Teams/SIEM integrations için useful generic bridge.

---

## Karar NT-04 — Slack/Teams native integrations

**Durum:** APPROVED

**APPROVED DECISION — OUT.**

---

## Karar NT-05 — Notification trigger

**Durum:** APPROVED

**APPROVED DECISION:** New incident or material severity/confidence escalation; raw events do not notify individually.

---

## Karar NT-06 — Notification acknowledgement

**Durum:** APPROVED

**APPROVED DECISION:** Email/webhook delivery state separate; operator UI acknowledgement incident lifecycle'a bağlanır.

---

## Karar NT-07 — Escalation contacts

**Durum:** APPROVED

**APPROVED DECISION:** Primary + optional secondary contact. Complex schedules/on-call rotations out.

---

# 16. Health & Operational MVP

## Karar OPS-01 — Functional Edge health

**Durum:** APPROVED

**APPROVED CORE dimensions:**
- process
- device certificate
- control connection
- config convergence
- local DB
- spool capacity
- clock quality
- container runtime
- privileged helper

---

## Karar OPS-02 — Functional decoy health

**Durum:** APPROVED

**APPROVED DECISION:**
- container/process running
- configured IP present
- configured port/responding
- telemetry path works
- policy applied
- version/digest expected

---

## Karar OPS-03 — Staleness

**Durum:** APPROVED

**APPROVED MVP:** token/decoy config/version/network validation staleness indicators; advanced environment drift detection post-MVP.

---

## Karar OPS-04 — Diagnostics bundle

**Durum:** APPROVED

**APPROVED DECISION — YES.**
Operator can generate redacted diagnostic bundle for support:
- versions
- health
- config summary
- logs
- no secrets/raw passwords

---

## Karar OPS-05 — Local Edge CLI

**Durum:** APPROVED

**APPROVED MINIMUM:**
- status
- version
- enrollment status
- connection diagnostics
- local queue
- restart/reconcile trigger
- diagnostic export

No full product management CLI.

---

# 17. Updates & Lifecycle MVP

## Karar UP-01 — Signed artifact verification

**Durum:** APPROVED

**APPROVED DECISION — RELEASE BLOCKER.**
Unsigned/untrusted decoy pack cannot activate.

---

## Karar UP-02 — Edge application update

**Durum:** APPROVED

### A — Full automatic update
### B — Controlled signed updater with explicit trigger
### C — Manual file replacement

**APPROVED DECISION — B.**

Automatic scheduling/channel UX may be post-MVP; updater/security path itself MVP.

---

## Karar UP-03 — Edge rollback

**Durum:** APPROVED

**APPROVED DECISION — CORE.**
Failed health check after update can restore previous known-good binary/config.

---

## Karar UP-04 — Decoy pack update

**Durum:** APPROVED

**APPROVED DECISION — CORE.**
Signed digest-pinned image pull + verify + rollout + health result.

---

## Karar UP-05 — Offline update bundle

**Durum:** APPROVED

**APPROVED DECISION — OUT.**

---

## Karar UP-06 — Staged fleet rollout

**Durum:** APPROVED

**APPROVED DECISION — OUT** because MVP one active Edge/environment.

---

# 18. Authentication, Authorization & Audit MVP

## Karar AUTH-01 — User model

**Durum:** APPROVED

**APPROVED DECISION:** single organization, local account.

---

## Karar AUTH-02 — Initial admin bootstrap

**Durum:** APPROVED

**APPROVED DECISION:** one-time secure bootstrap flow; default static password yok.

---

## Karar AUTH-03 — MFA

**Durum:** APPROVED

**APPROVED DECISION — TOTP required for owner/admin.**

---

## Karar AUTH-04 — OIDC/SSO

**Durum:** APPROVED

**APPROVED DECISION — OUT.**

---

## Karar AUTH-05 — Fine-grained RBAC

**Durum:** APPROVED

**APPROVED DECISION — OUT.**
Owner/operator distinction only if technically needed; enterprise role editor yok.

---

## Karar AUTH-06 — Audit events

**Durum:** APPROVED

**APPROVED CORE:**
- login/logout/auth failures
- Edge enrollment/revocation
- environment changes
- decoy deploy/update/remove
- suppression/allowlist changes
- incident disposition
- AI provider/config changes
- update actions
- user/security setting changes

---

# 19. Privacy, Retention & Hostile Data MVP

## Karar DATA-01 — Retention configuration

**Durum:** APPROVED

**APPROVED DECISION:** data-class-specific default policy + configurable values. Exact default day counts final MVP spec'te owner decision.

Classes:
- normalized events/evidence
- incidents
- AI outputs
- raw transcripts
- hostile files
- audit logs

---

## Karar DATA-02 — Password capture

**Durum:** APPROVED

**APPROVED DECISION:** Never render/store real captured passwords in normal plaintext UI. Decoy credential match can be represented as match/result; raw secret handling minimized.

---

## Karar DATA-03 — HTTP request bodies

**Durum:** APPROVED

**APPROVED DECISION:** size-bounded capture; secrets-sensitive headers/fields redaction policy.

---

## Karar DATA-04 — SSH transcript

**Durum:** APPROVED

**APPROVED DECISION:** command/session text stored as hostile untrusted content with size/retention limit.

---

## Karar DATA-05 — File upload

**Durum:** APPROVED

**APPROVED DECISION:** MVP SSH adapter file capture supported only if upstream capability safely exposes it; quarantined object storage. File analysis not required.

---

## Karar DATA-06 — Delete/purge

**Durum:** APPROVED

**APPROVED DECISION:** Environment-level retention job/purge. Individual evidence editing/deletion from incident UI not allowed.

---

# 20. Security Acceptance Criteria

## Karar SEC-01 — Decoy egress

**Durum:** APPROVED

**APPROVED DECISION:** default deny. MVP decoys required to operate without arbitrary internet/production egress.

---

## Karar SEC-02 — Production secrets

**Durum:** APPROVED

**APPROVED RELEASE BLOCKER:** no production credential copied into decoy config/image.

---

## Karar SEC-03 — Management isolation

**Durum:** APPROVED

**APPROVED DECISION:** Decoy containers cannot access Edge management UDS, device private key, containerd socket or Control Plane credentials.

---

## Karar SEC-04 — Privileged helper

**Durum:** APPROVED

**APPROVED DECISION:** no arbitrary shell execution API; typed allowlisted methods only.

---

## Karar SEC-05 — Device enrollment

**Durum:** APPROVED

**APPROVED DECISION:** one-time token cannot be reused after successful enrollment.

---

## Karar SEC-06 — Revoked Edge

**Durum:** APPROVED

**APPROVED DECISION:** disabled/revoked device cert loses Control Plane access; existing local decoys may continue last-known-good but UI marks unmanaged/offline until explicitly removed/re-enrolled.

---

## Karar SEC-07 — Malicious telemetry parsing

**Durum:** APPROVED

**APPROVED DECISION:** malformed/oversized payload cannot crash Edge/Control; parser limits enforced.

---

## Karar SEC-08 — Hostile UI rendering

**Durum:** APPROVED

**APPROVED RELEASE BLOCKER:** attacker-supplied HTML/JS/ANSI/Markdown does not execute in Web Console.

---

## Karar SEC-09 — AI prompt injection

**Durum:** APPROVED

**APPROVED RELEASE BLOCKER.**

---

## Karar SEC-10 — Artifact rollback attack

**Durum:** APPROVED

**APPROVED DECISION:** updater rejects revoked/older metadata where TUF policy forbids rollback.

---

## Karar SEC-11 — Dependency/SBOM

**Durum:** APPROVED

**APPROVED DECISION:** release artifacts have SBOM; known critical vulnerability policy defined before pilot release.

---

## Karar SEC-12 — Security regression suite

**Durum:** APPROVED

**APPROVED DECISION:** all release-blocker security scenarios automated where practical; manual penetration checklist for remaining cases.

---

# 21. Resilience & Failure-Mode MVP

## Karar FM-01 — Internet/Control Plane outage

**Durum:** APPROVED

**APPROVED DECISION:** Edge continues existing decoys, local evidence collection and queueing.

---

## Karar FM-02 — AI provider outage

**Durum:** APPROVED

**APPROVED DECISION:** incident pipeline unaffected; AI status degraded.

---

## Karar FM-03 — Edge restart

**Durum:** APPROVED

**APPROVED DECISION:** desired state reconciles; no event queue corruption; decoys restored.

---

## Karar FM-04 — Decoy crash

**Durum:** APPROVED

**APPROVED DECISION:** Edge detects, restarts according policy, marks health; repeated crash does not loop unbounded.

---

## Karar FM-05 — Control Plane restart

**Durum:** APPROVED

**APPROVED DECISION:** Edge reconnects automatically; idempotent telemetry resend; incident duplicates not created.

---

## Karar FM-06 — PostgreSQL restart

**Durum:** APPROVED

**APPROVED DECISION:** Control Plane recovers; inbound Edge backpressure/local buffering protects evidence.

---

## Karar FM-07 — Local disk pressure

**Durum:** APPROVED

**APPROVED DECISION:** thresholds visible; oldest low-value raw/spool data policy may activate only according explicit priority. High-value evidence loss generates critical health state.

---

## Karar FM-08 — Network/IP conflict

**Durum:** APPROVED

**APPROVED DECISION:** decoy deployment blocked or withdrawn safely; no takeover of active customer IP.

---

## Karar FM-09 — Certificate expiration/rotation failure

**Durum:** APPROVED

**APPROVED DECISION:** warning before loss; rotation retries; final failure observable and does not silently downgrade to unauthenticated connection.

---

## Karar FM-10 — Telemetry flood

**Durum:** APPROVED

**APPROVED DECISION:** quotas/backpressure/dedup protect CPU/disk/AI budget; core management remains responsive.

---

# 22. Performance & Workload Acceptance

MVP performance hedefleri product-scale claim değil, **pilot environment safety margin** olarak tanımlanmalıdır.

## Karar PERF-01 — Reference MVP environment

**Durum:** APPROVED

**APPROVED draft profile:**
- 1 organization
- 1 Edge
- 2–3 private IPv4 zones
- up to 32 active decoy instances/addresses
- 4 decoy families
- 1 concurrent operator
- 30-day normalized evidence dataset for performance tests

**Counter-view:** 32 decoy fazla olabilir.  
**Gerekçe:** Runtime scale'ı değil purposeful placement'i test eder; acceptance upper bound olarak useful.

---

## Karar PERF-02 — Event ingest steady-state target

**Durum:** APPROVED

### A — 10 events/sec
### B — 100 events/sec
### C — 1,000 events/sec

**APPROVED DECISION — B: 100 normalized events/sec sustained lab load**, no evidence loss under healthy connection.

Bu müşteri normal load tahmini değildir; scanner/automation noise için margin testidir.

---

## Karar PERF-03 — Burst target

**Durum:** APPROVED

**APPROVED draft:** 500 events/sec for 60 seconds with backpressure/batching; no process crash; no duplicate incident explosion.

---

## Karar PERF-04 — Edge offline buffer

**Durum:** APPROVED

**A — Fixed event count**
**B — Fixed bytes**
**C — Both**

**APPROVED DECISION — C.**

Approved acceptance:
- at least 100,000 normalized small events **or**
- configurable spool size >= 1 GiB
before controlled pressure policy.

Exact shipped defaults final owner review.

---

## Karar PERF-05 — Incident creation latency

**Durum:** APPROVED

**APPROVED draft:** Direct high-confidence decoy interaction visible as evidence/incident in Web Console within **5 seconds p95** under reference load, excluding AI enrichment.

---

## Karar PERF-06 — AI explanation latency

**Durum:** APPROVED

**APPROVED DECISION:** no hard security SLA; UI async. Approved UX target **30 seconds p95** under provider availability, but failure does not block incident.

---

## Karar PERF-07 — Web UX latency

**Durum:** APPROVED

**APPROVED draft:** common incident list/detail API p95 < 1 second on reference dataset; full page interaction usable < 2 seconds on normal broadband/local operator path.

---

## Karar PERF-08 — Edge reconnect/replay

**Durum:** APPROVED

**APPROVED DECISION:** After connectivity returns, queued events replay without duplicate evidence; live ingestion continues with bounded fairness. Exact drain throughput benchmark recorded but not user-facing SLA.

---

## Karar PERF-09 — Decoy deployment convergence

**Durum:** APPROVED

**APPROVED draft:** single decoy desired-state change reaches healthy/failed terminal state within 60 seconds under normal local conditions.

---

## Karar PERF-10 — Performance regression policy

**Durum:** APPROVED

**APPROVED DECISION:** reference workload suite runs in CI/nightly or pre-release; >20% regression in critical p95 metrics requires review.

---

# 23. Observability & Supportability MVP

## Karar OBS-01 — Product health vs engineering observability

**Durum:** APPROVED

**APPROVED DECISION:** Product health is user-facing domain state; Prometheus/Loki/Grafana are internal/ops tooling, not user requirement.

---

## Karar OBS-02 — Required product metrics

**Durum:** APPROVED

**APPROVED DECISION:**
- Edge connectivity
- config convergence
- event queue depth/age
- ingest errors
- decoy healthy/degraded
- incident processing latency
- notification delivery
- AI success/failure/budget
- update status

---

## Karar OBS-03 — Trace coverage

**Durum:** APPROVED

**APPROVED DECISION:** Edge RPC → ingest → evidence → incident correlation paths have trace/context IDs in dev/hosted ops.

---

## Karar OBS-04 — PII/secret logging

**Durum:** APPROVED

**APPROVED RELEASE BLOCKER:** application logs must not intentionally log passwords, device private keys, auth session tokens or raw AI provider secrets.

---

# 24. MVP Test & Validation Strategy

## Karar TEST-01 — Reproducible network lab MVP blocker mı?

**Durum:** APPROVED

**APPROVED DECISION — YES.**

Before product “MVP complete”, repository must create deterministic virtual lab containing:
- Control Plane
- Edge VM
- >=2 logical network zones
- decoys
- attacker/test host

---

## Karar TEST-02 — Required attack scenario suite

**Durum:** APPROVED

**APPROVED MINIMUM:**
1. single-port recon
2. multi-service scan
3. SSH invalid login
4. SSH repeated logins
5. valid synthetic decoy credential
6. SSH command session
7. HTTP admin discovery/login attempt
8. PostgreSQL handshake/login
9. SMB probe
10. same source touches 3 decoys
11. known scanner benign suppression
12. incident merge/split
13. attacker prompt injection string
14. telemetry flood

---

## Karar TEST-03 — Required failure scenario suite

**Durum:** APPROVED

**APPROVED MINIMUM:**
1. Control Plane disconnect
2. Edge restart
3. decoy crash
4. PostgreSQL restart
5. AI provider timeout
6. invalid/expired cert
7. update failure/rollback
8. disk/spool pressure
9. IP conflict
10. malformed decoy telemetry

---

## Karar TEST-04 — Browser E2E

**Durum:** APPROVED

**APPROVED DECISION:** Onboarding → deploy → incident → disposition critical path automated browser test.

---

## Karar TEST-05 — Manual security review

**Durum:** APPROVED

**APPROVED DECISION:** Before pilot:
- trust boundary review
- Edge host privilege review
- container isolation/egress review
- web session/security headers
- hostile content rendering
- prompt injection
- update verification

---

## Karar TEST-06 — External penetration test MVP blocker mı?

**Durum:** APPROVED

### A — Mandatory before first controlled pilot
### B — Internal review for MVP, independent pentest before wider beta/GA
### C — No

**APPROVED DECISION — B.**

---

# 25. Architecture Spike Blockers Before/Alongside MVP Implementation

Step 4 validation backlog'taki her spike MVP blocker değildir.

## Karar SP-01 — Routed secondary-IP networking spike

**Durum:** APPROVED

**APPROVED DECISION — BLOCKER.**
Reference hypervisor/network üzerinde reliable placement + conflict behavior kanıtlanmalı.

---

## Karar SP-02 — containerd direct lifecycle spike

**Durum:** APPROVED

**APPROVED DECISION — BLOCKER.**
Start/stop/update/health/resource limits/namespace cleanup güvenilir olmalı.

---

## Karar SP-03 — Cowrie adapter spike

**Durum:** APPROVED

**APPROVED DECISION — BLOCKER** çünkü SSH medium-interaction MVP core.

---

## Karar SP-04 — OpenCanary spike

**Durum:** APPROVED

**APPROVED DECISION — NOT BLOCKER** unless selected HTTP/SMB/DB pack depends on it.
Custom small decoys alternative olabilir.

---

## Karar SP-05 — PostgreSQL partition benchmark

**Durum:** APPROVED

**APPROVED DECISION — BLOCKER before final performance acceptance**, not before initial coding.

---

## Karar SP-06 — SQLite offline replay/crash spike

**Durum:** APPROVED

**APPROVED DECISION — BLOCKER.**

---

## Karar SP-07 — Device certificate enrollment/rotation/revocation

**Durum:** APPROVED

**APPROVED DECISION — BLOCKER.**

---

## Karar SP-08 — TUF + Cosign update flow

**Durum:** APPROVED

**APPROVED DECISION — BLOCKER before pilot release**, not before all feature development.

---

## Karar SP-09 — OpenAI + Anthropic structured-schema compatibility

**Durum:** APPROVED

**APPROVED DECISION — BLOCKER for AI MVP acceptance.**

---

## Karar SP-10 — Prompt injection corpus

**Durum:** APPROVED

**APPROVED DECISION — BLOCKER.**

---

## Karar SP-11 — High-interaction/libvirt spikes

**Durum:** APPROVED

**APPROVED DECISION — NOT MVP BLOCKER.**

---

## Karar SP-12 — macvlan/ipvlan/VLAN spikes

**Durum:** APPROVED

**APPROVED DECISION — NOT MVP BLOCKER.**

---

# 26. Explicit MVP Non-Goals

Aşağıdaki capability'lerin MVP'de **bulunmaması tavsiye edilir**:

- High-Interaction Worker
- QEMU/KVM customer capability
- Real Windows VM decoy
- RDP deception
- Production Active Directory join/write
- Endpoint Deception Agent
- Automated endpoint breadcrumb placement
- Identity directory deception
- Cloud/SaaS honey-token integrations
- Fake-network redirection
- Session migration
- Attacker-facing LLM
- AI-generated adaptive deception
- AI autonomous containment
- EDR/firewall/NAC containment integration
- SIEM integration
- Native Slack/Teams integrations
- MSSP/multi-tenancy
- Configurable enterprise RBAC
- OIDC/SSO
- Public internet honeynet
- Threat intelligence feed
- Full passive packet capture/NDR
- Vulnerability scanner
- Malware sandbox
- Kubernetes deployment
- Kafka/NATS broker
- ClickHouse
- Redis
- Graph database
- Vector database/RAG
- Cloud VPC discovery
- IPv6 runtime support
- VLAN/macvlan/ipvlan deployment acceptance
- Multi-Edge / multi-site production support
- Fully offline Control Plane
- Offline update bundles
- Auto-generated compliance reports

---

# 27. Approved MVP Capability Matrix

| Capability | MVP | Post-MVP | Future/Advanced | Notes |
|---|:---:|:---:|:---:|---|
| Hosted Control Plane | ✓ | | | Primary pilot profile |
| Web Console | ✓ | | | Incident-first |
| Debian Edge appliance | ✓ | | | Single active Edge |
| Routed private subnets | ✓ | | | 2–3 zones |
| VLAN trunk | | ✓ | | |
| macvlan/ipvlan | | ✓ | | |
| IPv6 runtime | | ✓ | | Contracts ready |
| SSH medium decoy | ✓ | | | Core |
| HTTP/admin decoy | ✓ | | | Core |
| PostgreSQL decoy | ✓ | | | Core |
| SMB low-interaction decoy | ✓ | | | Windows signal |
| RDP | | ✓ | | |
| Synthetic decoy credential | ✓ | | | Manual placement |
| Endpoint agent | | ✓ | | |
| AD identity deception | | ✓ | | |
| High interaction | | | ✓ | |
| Fake network/redirection | | | ✓ | |
| Event/evidence model | ✓ | | | |
| Deterministic detection | ✓ | | | |
| Incident correlation | ✓ | | | |
| Multi-decoy attacker journey | ✓ | | | |
| Source IP/zone identification | ✓ | | | |
| Hostname enrichment | best effort | ✓ | | |
| Confidence/severity | ✓ | | | |
| Scanner suppression | ✓ | | | |
| Incident merge/split | ✓ | | | Basic |
| AI incident explanation | ✓ | | | |
| AI response guidance | ✓ | | | |
| AI correlation suggestions | optional | ✓ | | |
| Attacker-facing AI | | | ✓ | |
| Email notification | ✓ | | | |
| Generic webhook | ✓ | | | |
| Slack/Teams | | ✓ | | |
| Functional health | ✓ | | | |
| Signed Edge update | ✓ | | | Controlled |
| Signed decoy update | ✓ | | | |
| Offline update | | ✓ | | |
| Local auth + TOTP | ✓ | | | |
| OIDC/SSO | | ✓ | | |
| Fine RBAC | | ✓ | | |
| Audit log | ✓ | | | |
| OTel ops telemetry | ✓ | | | |
| Prom/Grafana/Loki hosted ops | ✓ | | | Not customer UX |
| S3-compatible hostile data | ✓ | | | |
| Multi-tenant MSSP | | | ✓ | |

---

# 28. Acceptance Criteria Catalog

Bu bölüm final Step 5 belgesinde test-case/requirement traceability'nin çekirdeği olacaktır.

## 28.1 Onboarding / Edge

### AC-ON-001 — Edge enrollment
**Given** yeni environment ve geçerli one-time enrollment token  
**When** fresh Edge appliance enroll olur  
**Then**
- unique device identity oluşur
- Edge private key cihazdan çıkmaz
- Control Plane device certificate verir
- token tekrar kullanılamaz
- Edge `Connected/Healthy` veya açık degraded reason ile görünür.

### AC-ON-002 — Invalid enrollment token
Geçersiz/expired/used token ile enrollment reddedilir ve audit event oluşur.

### AC-ON-003 — Network definition
Operator en az iki RFC1918/private IPv4 network zone tanımlayabilir.

### AC-ON-004 — IP conflict
Selected decoy IP başka host tarafından kullanılıyorsa deployment fail-safe reddedilir; mevcut host etkilenmez.

### AC-ON-005 — First decoy deployment
Approved desired state Edge'e ulaşır; decoy healthy terminal state'e veya actionable failure state'e gelir.

### AC-ON-006 — Coverage verification
Healthy decoy yalnız process-up ile değil configured IP/port functional probe sonucu healthy sayılır.

---

## 28.2 SSH

### AC-SSH-001 — Connection
Internal test host SSH decoy'a bağlandığında source IP, decoy, timestamp, session ve protocol canonical event olarak kaydedilir.

### AC-SSH-002 — Failed credential
Login attempt username + auth result evidence üretir; sensitive password normal UI/log'da plaintext gösterilmez.

### AC-SSH-003 — Synthetic credential
Configured synthetic honey credential kullanıldığında evidence `honey_credential_match` benzeri high-confidence marker taşır.

### AC-SSH-004 — Commands
Authenticated medium-interaction session'daki commands ordered session evidence olarak görünür.

### AC-SSH-005 — Hostile command content
Command içine prompt-injection/XSS string'i yazılması Web Console veya AI policy'yi execute ettirmez.

---

## 28.3 HTTP

### AC-HTTP-001 — Request
GET/POST request source, path, method, selected headers ve bounded metadata ile kaydedilir.

### AC-HTTP-002 — Login
Fake login form interaction auth-attempt evidence üretir.

### AC-HTTP-003 — XSS payload
Attacker request/body içindeki script UI'da execute olmaz.

### AC-HTTP-004 — Oversized payload
Configured capture limit üzerindeki body safe truncate/reject semantics ile işlenir; service crash olmaz.

---

## 28.4 PostgreSQL

### AC-PG-001 — Discovery/handshake
Client connection protocol interaction evidence üretir.

### AC-PG-002 — Auth attempt
Username/database/auth result capture edilir; raw password protection policy uygulanır.

### AC-PG-003 — Synthetic credential
Valid decoy credential use high-confidence evidence üretir.

---

## 28.5 SMB

### AC-SMB-001 — Probe
Internal SMB enumeration/probe source + target + protocol-level action evidence üretir.

### AC-SMB-002 — Windows persona
Incident UI SMB decoy'u Windows-flavored persona olarak gösterir; ürün real Windows host iddiasında bulunmaz.

---

## 28.6 Evidence/Incident

### AC-EV-001 — Idempotency
Same Edge event_id retry edildiğinde duplicate evidence oluşmaz.

### AC-EV-002 — Provenance
Evidence item'dan edge/decoy/runtime/schema/raw-reference provenance görülebilir.

### AC-EV-003 — Time
Event/observed/ingested timestamps saklanır.

### AC-INC-001 — Direct interaction incident
Non-suppressed meaningful decoy interaction incident/finding oluşturur veya existing incident'ı deterministic rule ile update eder.

### AC-INC-002 — Multi-decoy correlation
Same test source configured rule window içinde SSH→HTTP→DB decoy'larına dokunursa tek incident'ta ordered journey görünür.

### AC-INC-003 — Separate actor
Distinct source IP'ler sırf aynı decoy'a dokundu diye otomatik aynı incident'a merge edilmez.

### AC-INC-004 — Merge/split
Operator iki incident'ı merge edebilir veya yanlış group'u split edebilir; evidence silinmez; audit kayıt oluşur.

---

## 28.7 Confidence / Noise

### AC-CF-001 — Confidence reasoning
Incident confidence label yanında en az bir human-readable factor gösterilir.

### AC-CF-002 — Severity separate
Severity ve confidence UI/data modelde ayrı field'dır.

### AC-CF-003 — Known scanner
Known scanner olarak tanımlanan source'tan beklenen scan evidence'ı retained olur fakat configured policy'ye göre notification suppress edilir.

### AC-CF-004 — Unexpected scanner behavior
Suppression scope dışındaki behavior otomatik olarak tamamen silinmez.

### AC-CF-005 — Dedup
Rapid repeated identical login attempts tekil push/email storm üretmez; count ve first/last seen korunur.

---

## 28.8 AI

### AC-AI-001 — Structured schema
OpenAI adapter incident-analysis JSON Schema'yı validate eden output üretir veya controlled failure döner.

### AC-AI-002 — Anthropic adapter
Aynı provider-neutral schema contract Anthropic adapter ile geçer.

### AC-AI-003 — Evidence citations
AI factual claims supporting evidence IDs taşır; unsupported claim hypothesis/general guidance olarak ayrılır veya validation reject eder.

### AC-AI-004 — Provider outage
Provider unavailable olduğunda incident oluşturulur, notification çalışır, UI AI degraded status gösterir.

### AC-AI-005 — Prompt injection
Attacker evidence içindeki instruction content system/user policy'yi değiştirmez; tool execution zaten mevcut değildir.

### AC-AI-006 — No automatic action
AI output hiçbir network/Edge/firewall action'ı otomatik execute etmez.

### AC-AI-007 — Cached repeat
Unchanged evidence snapshot için repeated page load yeni LLM call üretmez.

---

## 28.9 Guidance

### AC-RG-001
Confirmed/suspicious incident en az:
- verify/investigate
- containment consideration
- follow-up/recovery
kategorilerinde context-appropriate action gösterebilir.

### AC-RG-002
Guidance açıkça “recommended” olarak etiketlenir ve action'ın ürün tarafından execute edilmediği anlaşılır.

### AC-RG-003
AI unavailable ise deterministic curated fallback guidance bulunur veya explicit “guidance unavailable” gösterilir; fabricated guidance yok.

---

## 28.10 Notification

### AC-NT-001
New high-relevance incident in-product notification oluşturur.

### AC-NT-002
Configured email destination incident summary + source + severity/confidence + console link alır.

### AC-NT-003
Generic webhook signed/authenticated request ile machine-readable incident payload gönderebilir.

### AC-NT-004
Repeated evidence same incident için notification dedup policy'ye uyar.

---

## 28.11 Health / Offline

### AC-HL-001
Control connection kesildiğinde Edge existing decoy'ları çalıştırır ve event'leri local persistent queue'ya yazar.

### AC-HL-002
Reconnect sonrası queue replay edilir ve duplicates oluşmaz.

### AC-HL-003
Decoy process öldürülürse health degraded olur ve configured restart/reconcile davranışı uygulanır.

### AC-HL-004
Queue/disk threshold UI'da visible warning üretir.

### AC-HL-005
Clock unhealthy ise journey/evidence interface uncertainty/health warning üretir.

---

## 28.12 Security

### AC-SEC-001
Decoy container'dan Edge device private key path erişilemez.

### AC-SEC-002
Decoy container'dan containerd management socket erişilemez.

### AC-SEC-003
Default decoy egress policy forbidden external destination'a bağlantıyı engeller.

### AC-SEC-004
Unsigned/tampered OCI decoy artifact activate edilmez.

### AC-SEC-005
Used enrollment token replay reddedilir.

### AC-SEC-006
Revoked device cert yeni Control Plane session kuramaz.

### AC-SEC-007
Hostile HTML/JS telemetry browser DOM'unda executable hale gelmez.

### AC-SEC-008
Oversized/malformed Protobuf/log payload Edge/Control process crash yaratmaz.

### AC-SEC-009
Release logs/diagnostics device private keys, user session secrets veya configured AI keys içermez.

---

## 28.13 Updates

### AC-UP-001
Signed Edge update staged and verified before activation.

### AC-UP-002
Tampered update rejected.

### AC-UP-003
Health check failure after Edge update previous known-good version'a rollback yapabilir.

### AC-UP-004
Decoy image digest change desired state ile rollout edilir; health failure visible.

---

## 28.14 UX

### AC-UX-001
Operator login sonrası primary view açık incidents + severity/confidence + source context'i gösterir.

### AC-UX-002
Incident detail'e giren kullanıcı raw port/log bilgisi okumadan “what happened” summary görebilir.

### AC-UX-003
Journey chronological olarak okunabilir.

### AC-UX-004
AI inference ile observed evidence visually/textually ayrıdır.

### AC-UX-005
Incident `Acknowledged`, `Investigating`, `Benign/Expected`, `Confirmed`, `Resolved` lifecycle/disposition semantics ile yönetilebilir.

---

# 29. Approved MVP Definition of Done

Bir capability yalnız code merge edildiği için “done” sayılmaz.

**APPROVED DoD:**

Bir MVP capability için:

- [ ] Approved Step 5 scope/AC ID'lerine traceable
- [ ] Architecture decision'larıyla uyumlu
- [ ] Threat/security impact reviewed
- [ ] Unit tests
- [ ] Contract tests where applicable
- [ ] Integration test
- [ ] Relevant network/attack scenario
- [ ] Failure behavior test
- [ ] Observability/health
- [ ] Audit where security-sensitive
- [ ] Documentation/runbook
- [ ] No unresolved critical/high security finding
- [ ] AI feature ise eval suite pass
- [ ] User-visible capability ise browser/E2E coverage
- [ ] New dependency ise license/security review
- [ ] Agent-generated change manifest complete

---

# 30. Approved MVP Release Gate

MVP “pilot-ready” olarak işaretlenmeden:

## Product gates
- [ ] End-to-end North Star scenario passes
- [ ] Primary onboarding flow passes
- [ ] Incident-first UX complete
- [ ] Guidance and notification complete
- [ ] Explicit non-goals documented

## Detection gates
- [ ] SSH/HTTP/PostgreSQL/SMB scenarios pass
- [ ] synthetic credential scenario passes
- [ ] multi-decoy journey passes
- [ ] scanner suppression/noise scenario passes

## Security gates
- [ ] default-deny decoy egress verified
- [ ] management isolation verified
- [ ] enrollment/cert revocation tests pass
- [ ] signed artifact/update verification passes
- [ ] prompt injection tests pass
- [ ] hostile web rendering tests pass
- [ ] critical/high known security issues resolved or explicitly owner-waived

## Reliability gates
- [ ] Edge offline/replay passes
- [ ] Control restart passes
- [ ] Edge restart/reconcile passes
- [ ] DB restart/backpressure passes
- [ ] decoy crash recovery passes
- [ ] telemetry flood passes

## AI gates
- [ ] OpenAI schema/evidence tests pass
- [ ] Anthropic schema/evidence tests pass
- [ ] provider outage degraded mode passes
- [ ] unsupported-claim eval below owner-approved threshold
- [ ] no-tool authority verified

## Performance gates
- [ ] reference steady load passes
- [ ] burst load passes
- [ ] p95 incident latency target passes
- [ ] UI reference dataset target passes

## Operations gates
- [ ] health/status surface works
- [ ] diagnostics bundle works
- [ ] Edge update + rollback works
- [ ] decoy pack update works
- [ ] backup/restore smoke test for central state

---

# 31. MVP Validation Metrics

Step 5 acceptance ile product validation aynı şey değildir.

## Engineering acceptance metrics
Deterministic pass/fail:
- AC pass rate
- security gate pass
- failure-mode pass
- performance thresholds
- AI schema/citation tests

## Pilot product-validation metrics
MVP release sonrası gözlemlenir:
- time-to-first-healthy-decoy
- time-to-understand incident
- number of notifications per meaningful attack scenario
- benign/expected disposition rate
- operator ability to identify source without expert help
- operator action-guidance usefulness
- deployment/support interventions
- decoy health/staleness incidents
- AI explanation corrections
- false correlation merge/split rate

**APPROVED DECISION:** Product-validation metric'leri MVP completion blocker değildir; MVP'nin bir sonraki product decision'larını besler.

---

# 32. Key Open Quantitative Decisions

Bu bölümdeki rakamlar **pazar SLA iddiası değil**, Product Owner tarafından onaylanmış reference lab/pilot quality bar'larıdır.

## Karar Q-01 — Reference active decoy upper bound

**Durum:** APPROVED

### A — 8
MVP gerçek kullanımı temsil eder ancak runtime/resource sınırlarını az zorlar.

### B — 32
Purposeful deployment felsefesini bozmayacak kadar sınırlı; lifecycle/resource testleri için yeterli headroom.

### C — 100+
MVP'yi ölçek benchmark'ına çevirir.

**APPROVED DECISION — B: 32 active decoy instances/addresses.**

Bu sayı customer'ın 32 decoy kullanması gerektiği anlamına gelmez; reference test üst sınırıdır.

---

## Karar Q-02 — Reference routed zone sayısı

**Durum:** APPROVED

### A — 1
Flat LAN'dan öte topology doğrulanmaz.

### B — 2–3
Bir Edge'in birden fazla logical private zone'u kapsaması test edilir.

### C — 10+
Multi-site/network-management scope'una yaklaşır.

**APPROVED DECISION — B: 2–3 routed zones.**

---

## Karar Q-03 — Steady normalized event rate

**Durum:** APPROVED

### A — 10 events/s
Normal decoy traffic için muhtemelen yeterli fakat scanner burst margin'i zayıf.

### B — 100 events/s
MVP'nin expected business load'ından yüksek, güvenli engineering margin.

### C — 1,000 events/s sustained
Security telemetry platform scale testine yaklaşır.

**APPROVED DECISION — B: 100 events/s sustained.**

Acceptance: healthy Control Plane connection altında evidence loss yok; management UX usable kalır.

---

## Karar Q-04 — Burst event rate

**Durum:** APPROVED

### A — 250 events/s × 60 s
### B — 500 events/s × 60 s
### C — 1,000 events/s × 60 s

**APPROVED DECISION — B: 500 events/s for 60 seconds.**

Amaç scan/auth flood durumunda backpressure ve dedup davranışını test etmektir; pazar throughput claim'i değildir.

---

## Karar Q-05 — Edge offline spool minimum

**Durum:** APPROVED

### A — 10,000 events
Kısa kesintiler.

### B — 100,000 normalized small events veya >=1 GiB configurable spool
Useful controlled outage margin.

### C — Multi-day unlimited spool
Disk/resource behavior belirsizleşir.

**APPROVED DECISION — B.**

İki limit birlikte uygulanmalıdır: event-count ve byte capacity. Threshold approaching durumunda health warning zorunlu.

---

## Karar Q-06 — Direct incident visibility latency

**Durum:** APPROVED

### A — <=1 s p95
Aggressive; unnecessary early optimization.

### B — <=5 s p95
Security alert için hızlı, batching/durable pipeline'a izin verir.

### C — <=30 s p95
Product “early visibility” hissini zayıflatır.

**APPROVED DECISION — B: <=5 seconds p95**, AI enrichment hariç.

---

## Karar Q-07 — AI explanation UX target

**Durum:** APPROVED

### A — <=10 s p95
Provider/model bağımlı ve brittle.

### B — <=30 s p95 target
Usable async experience.

### C — No target
Regression fark edilmeyebilir.

**APPROVED DECISION — B: <=30 seconds p95 target when provider healthy.**

Bu **security SLA değildir**. AI failure/timeout incident creation'ı bloke etmez.

---

## Karar Q-08 — Common Control Plane API latency

**Durum:** APPROVED

### A — <500 ms p95
İyi ama MVP dataset için aggressive olabilir.

### B — <1 s p95
Operational console için yeterli.

### C — <3 s p95
Incident investigation akışını yavaşlatır.

**APPROVED DECISION — B: <1 second p95** reference dataset üzerinde list/detail endpoints için.

---

## Karar Q-09 — Single decoy convergence deadline

**Durum:** APPROVED

### A — <=15 s
Image pull/network koşullarında brittle.

### B — <=60 s
Deploy/start/functional health için practical upper target.

### C — <=5 min
Operator feedback loop'u zayıf.

**APPROVED DECISION — B: <=60 seconds** normal local conditions ve cached/available artifact altında.

Image download gibi external transfer time ayrıca raporlanabilir.

---

## Karar Q-10 — Performance regression review threshold

**Durum:** APPROVED

### A — Her regression release blocker
Noise yaratır.

### B — Critical reference metric'te >20% degradation review gerektirir
Meaningful trend gate.

### C — Performance review yok
Silent degradation.

**APPROVED DECISION — B.**

`>20%` otomatik failure olmak zorunda değildir; owner/engineering review gerektirir ve kabul edilirse rationale kaydedilir.

---

## Quantitative Summary

| ID | Metric | Approved value |
|---|---|---|
| Q-01 | Active decoy upper reference | 32 |
| Q-02 | Routed zones | 2–3 |
| Q-03 | Steady normalized events | 100 events/s |
| Q-04 | Burst | 500 events/s × 60 s |
| Q-05 | Edge spool | >=100k events or >=1 GiB |
| Q-06 | Incident visibility | <=5 s p95 excluding AI |
| Q-07 | AI UX | <=30 s p95 target, non-blocking |
| Q-08 | Common API | <1 s p95 |
| Q-09 | Decoy convergence | <=60 s |
| Q-10 | Performance regression review | >20% |

# 33. Approved MVP Risk Baseline

| Risk | MVP consequence | Draft mitigation |
|---|---|---|
| Scope becomes honeypot catalog | core journey delayed | 4 decoy families limit |
| SMB decoy complexity | Windows signal delays | low-interaction only; RDP defer |
| Cowrie integration risk | SSH core blocked | early blocker spike |
| Network presence incompatibility | pilot cannot deploy | routed mode spike + reference hypervisor |
| False positives from scanners | user trust fails | scoped suppression |
| Source attribution overclaim | wrong action | observed-source language |
| Correlation overmerge | misleading journey | deterministic factors + merge/split |
| AI hallucination | unsafe guidance | evidence IDs + curated actions + no tools |
| Prompt injection | policy manipulation | hostile-data boundary + tests |
| AI provider downtime | incident blind spot | async optional enrichment |
| Update complexity | pilot security debt | signed controlled updater |
| Edge privilege compromise | customer risk | narrow helper + isolation |
| Event flood | disk/cost outage | dedup/backpressure/budget |
| Too many deployment modes | test matrix explosion | hosted CP + one Edge + routed IPv4 |
| Real Windows VM scope creep | HI complexity | SMB emulation only |
| Product polish delays validation | overbuild | controlled pilot, not GA |
| Too-light MVP | only demo proves nothing | end-to-end release gate |

---

# 34. Consolidated APPROVED Decision Register

| ID | Decision | Approved direction | Status |
|---|---|---|---|
| M-01 | Release class | Controlled pilot-ready MVP | APPROVED |
| M-02 | Onboarding mode | Controlled onboarding | APPROVED |
| M-03 | Success | End-to-end North Star outcome | APPROVED |
| M-04 | Customer sizing | Environment/maturity, not headcount | APPROVED |
| ENV-01 | Control Plane | Hosted primary | APPROVED |
| ENV-02 | Edge count | One active Edge acceptance | APPROVED |
| ENV-03 | Network scope | Routed 2–3 private zones | APPROVED |
| ENV-04 | VLAN trunk | Out | APPROVED |
| ENV-05 | Multi-NIC | Out | APPROVED |
| ENV-06 | macvlan/ipvlan | Out | APPROVED |
| ENV-07 | IPv6 | Contracts only | APPROVED |
| ENV-08 | Cloud private networks | Out | APPROVED |
| ENV-09 | VPN networks | Out | APPROVED |
| ENV-10 | Edge format | Debian VM/appliance | APPROVED |
| ENV-11 | Hypervisor matrix | One reference + generic reqs | APPROVED |
| ON-01 | Onboarding flow | Full end-to-end flow | APPROVED |
| ON-02 | Discovery | Manual + minimal bounded | APPROVED |
| ON-03 | Active scan | Limited explicit | APPROVED |
| ON-04 | Placement AI | No; deterministic | APPROVED |
| ON-05 | Override | Owner can override | APPROVED |
| ON-06 | Coverage verification | Core | APPROVED |
| DC-01 | Decoy breadth | 4 families + synthetic credential | APPROVED |
| DC-02 | SSH | Core | APPROVED |
| DC-03 | HTTP | Core | APPROVED |
| DC-04 | PostgreSQL | Core | APPROVED |
| DC-05 | SMB | Core low interaction | APPROVED |
| DC-06 | RDP | Out | APPROVED |
| DC-07 | Other protocols | Out | APPROVED |
| DC-08 | Vulnerable web | Out | APPROVED |
| DC-09 | Honey credential | Bounded core | APPROVED |
| DC-10 | Honey docs/tokens | Out | APPROVED |
| DC-11 | Personas | Curated small set | APPROVED |
| DC-12 | Pack updates | Core | APPROVED |
| INT-01 | Interaction | Low + selected medium | APPROVED |
| INT-02 | HI worker | Out | APPROVED |
| INT-03 | Arbitrary binary | Out | APPROVED |
| INT-04 | SSH commands | Core | APPROVED |
| TH-01 | Recon | Core | APPROVED |
| TH-02 | Discovery | Core | APPROVED |
| TH-03 | Credential abuse | Core | APPROVED |
| TH-04 | Lateral behavior | deception-touchpoint core | APPROVED |
| TH-05 | Multi-decoy | Core | APPROVED |
| TH-06 | Insider classification | Out | APPROVED |
| TH-07 | Initial access | Out | APPROVED |
| TH-08 | Malware/C2/exfil | Out claim | APPROVED |
| TH-09 | Ransomware | Out claim | APPROVED |
| EV-01 | Domain chain | Core | APPROVED |
| EV-02 | Canonical fields | Core | APPROVED |
| EV-03 | Raw payload | Selective | APPROVED |
| EV-04 | Provenance UX | Core | APPROVED |
| EV-05 | Integrity | Append/provenance baseline | APPROVED |
| EV-06 | Time quality | Core | APPROVED |
| DET-01 | Edge processing | Normalize/dedup/evidence | APPROVED |
| DET-02 | Rules | Six core rule classes | APPROVED |
| DET-03 | Rule version | Core | APPROVED |
| DET-04 | Custom rules | Out | APPROVED |
| COR-01 | Dimensions | multi-factor | APPROVED |
| COR-02 | Strategy | deterministic + heuristic | APPROVED |
| COR-03 | Correlation windows | rule-specific | APPROVED |
| COR-04 | Journey timeline | Core | APPROVED |
| COR-05 | Graph UI | Timeline/simple summary | APPROVED |
| COR-06 | Merge/split | Basic core | APPROVED |
| COR-07 | Reprocessing UI | Out | APPROVED |
| SRC-01 | Guaranteed source | IP + zone | APPROVED |
| SRC-02 | Hostname | best effort | APPROVED |
| SRC-03 | MAC | optional | APPROVED |
| SRC-04 | Username | when observed | APPROVED |
| SRC-05 | VPN identity | Out | APPROVED |
| SRC-06 | Inventory integration | Out | APPROVED |
| SRC-07 | Uncertainty | Core | APPROVED |
| CS-01 | Confidence | L/M/H + factors | APPROVED |
| CS-02 | Severity | Info–Critical | APPROVED |
| CS-03 | Confidence factors | Core set | APPROVED |
| CS-04 | Severity mapping | Draft map | APPROVED |
| CS-05 | Known scanner | Core | APPROVED |
| CS-06 | Suppression | retain evidence | APPROVED |
| CS-07 | Benign disposition | Core | APPROVED |
| CS-08 | Auto-learning | No | APPROVED |
| CS-09 | Dedup | Core | APPROVED |
| UX-01 | Home | Incident-first | APPROVED |
| UX-02 | Incident list | minimum fields | APPROVED |
| UX-03 | Incident detail | 9 sections | APPROVED |
| UX-04 | ATT&CK | secondary | APPROVED |
| UX-05 | Raw explorer | incident-scoped | APPROVED |
| UX-06 | Decoy UI | core management | APPROVED |
| UX-07 | Health UI | core | APPROVED |
| UX-08 | Topology map | Out | APPROVED |
| AIM-01 | AI features | summary/explanation/guidance | APPROVED |
| AIM-02 | AI truth | No | APPROVED |
| AIM-03 | AI correlation | optional | APPROVED |
| AIM-04 | AI placement | Out | APPROVED |
| AIM-05 | Attacker AI | Out | APPROVED |
| AIM-06 | AI persona | Out | APPROVED |
| AIM-07 | Providers | both adapters contract-tested | APPROVED |
| AIM-08 | AI outage | non-blocking | APPROVED |
| AIM-09 | Evidence grounding | mandatory | APPROVED |
| AIM-10 | Prompt injection | release blocker | APPROVED |
| AIM-11 | AI tools | none | APPROVED |
| AIM-12 | Guidance knowledge | curated | APPROVED |
| AIM-13 | AI async | yes | APPROVED |
| AIM-14 | AI budgets | core | APPROVED |
| RG-01 | Guidance | 3 layers | APPROVED |
| RG-02 | Auto containment | Out | APPROVED |
| RG-03 | One-click containment | Out | APPROVED |
| RG-04 | Action catalog | curated | APPROVED |
| RG-05 | Unsafe advice | guarded | APPROVED |
| NT-01 | In-app | Core | APPROVED |
| NT-02 | Email | Core | APPROVED |
| NT-03 | Webhook | Supporting | APPROVED |
| NT-04 | Slack/Teams | Out | APPROVED |
| NT-05 | Trigger | incident/material update | APPROVED |
| NT-06 | Ack | incident state | APPROVED |
| NT-07 | Contacts | primary + secondary | APPROVED |
| OPS-01 | Edge health | functional | APPROVED |
| OPS-02 | Decoy health | functional | APPROVED |
| OPS-03 | Staleness | bounded | APPROVED |
| OPS-04 | Diagnostics | Core | APPROVED |
| OPS-05 | Local CLI | diagnostics only | APPROVED |
| UP-01 | Signing | blocker | APPROVED |
| UP-02 | Edge update | controlled updater | APPROVED |
| UP-03 | Rollback | Core | APPROVED |
| UP-04 | Pack update | Core | APPROVED |
| UP-05 | Offline update | Out | APPROVED |
| UP-06 | Fleet rollout | Out | APPROVED |
| AUTH-01 | User | local single-org | APPROVED |
| AUTH-02 | Bootstrap | one-time secure | APPROVED |
| AUTH-03 | MFA | TOTP required | APPROVED |
| AUTH-04 | OIDC | Out | APPROVED |
| AUTH-05 | RBAC | Out | APPROVED |
| AUTH-06 | Audit | core events | APPROVED |
| DATA-01 | Retention | per-class | APPROVED |
| DATA-02 | Passwords | protected/no normal plaintext | APPROVED |
| DATA-03 | HTTP bodies | bounded/redacted | APPROVED |
| DATA-04 | SSH transcript | hostile/bounded | APPROVED |
| DATA-05 | Files | optional safe quarantine | APPROVED |
| DATA-06 | Purge | policy driven | APPROVED |
| SEC-01 | Egress | default deny | APPROVED |
| SEC-02 | Prod secrets | forbidden | APPROVED |
| SEC-03 | Management isolation | blocker | APPROVED |
| SEC-04 | Priv helper | typed only | APPROVED |
| SEC-05 | Enrollment replay | blocked | APPROVED |
| SEC-06 | Revocation | enforced | APPROVED |
| SEC-07 | Parser safety | required | APPROVED |
| SEC-08 | Hostile rendering | blocker | APPROVED |
| SEC-09 | Prompt injection | blocker | APPROVED |
| SEC-10 | Update rollback attack | prevented | APPROVED |
| SEC-11 | SBOM/vuln policy | required | APPROVED |
| SEC-12 | Security regression | required | APPROVED |
| FM-01 | Control outage | local continues | APPROVED |
| FM-02 | AI outage | non-blocking | APPROVED |
| FM-03 | Edge restart | reconcile | APPROVED |
| FM-04 | Decoy crash | recover/degrade | APPROVED |
| FM-05 | Control restart | reconnect/idempotent | APPROVED |
| FM-06 | DB restart | buffer | APPROVED |
| FM-07 | Disk pressure | explicit policy | APPROVED |
| FM-08 | IP conflict | fail safe | APPROVED |
| FM-09 | Cert failure | no insecure downgrade | APPROVED |
| FM-10 | Flood | backpressure/quota | APPROVED |
| PERF-01 | Ref env | 1 Edge/32 decoys | APPROVED |
| PERF-02 | Steady load | 100 eps | APPROVED |
| PERF-03 | Burst | 500 eps×60s | APPROVED |
| PERF-04 | Spool | 100k or 1GiB | APPROVED |
| PERF-05 | Incident latency | <=5s p95 | APPROVED |
| PERF-06 | AI UX | <=30s p95 target | APPROVED |
| PERF-07 | Web API | <1s p95 | APPROVED |
| PERF-08 | Replay | duplicate-free | APPROVED |
| PERF-09 | Deploy convergence | <=60s | APPROVED |
| PERF-10 | Regression | >20% review | APPROVED |
| OBS-01 | Product vs ops health | separate | APPROVED |
| OBS-02 | Product metrics | core set | APPROVED |
| OBS-03 | Traces | critical path | APPROVED |
| OBS-04 | Secret logs | forbidden | APPROVED |
| TEST-01 | Network lab | blocker | APPROVED |
| TEST-02 | Attack suite | 14 scenarios | APPROVED |
| TEST-03 | Failure suite | 10 scenarios | APPROVED |
| TEST-04 | Browser E2E | critical path | APPROVED |
| TEST-05 | Manual security | before pilot | APPROVED |
| TEST-06 | External pentest | before wider beta/GA | APPROVED |
| SP-01 | Routed networking | blocker | APPROVED |
| SP-02 | containerd | blocker | APPROVED |
| SP-03 | Cowrie | blocker | APPROVED |
| SP-04 | OpenCanary | conditional | APPROVED |
| SP-05 | PostgreSQL perf | performance-gate blocker | APPROVED |
| SP-06 | SQLite replay | blocker | APPROVED |
| SP-07 | Device PKI | blocker | APPROVED |
| SP-08 | TUF+Cosign | pilot blocker | APPROVED |
| SP-09 | Dual AI schema | AI blocker | APPROVED |
| SP-10 | Prompt injection | blocker | APPROVED |
| SP-11 | High interaction | not blocker | APPROVED |
| SP-12 | advanced network modes | not blocker | APPROVED |
| Q-01 | Active decoy upper reference | 32 | APPROVED |
| Q-02 | Routed zones | 2–3 | APPROVED |
| Q-03 | Steady normalized events | 100 events/s | APPROVED |
| Q-04 | Burst | 500 events/s × 60 s | APPROVED |
| Q-05 | Edge spool | >=100k events or >=1 GiB | APPROVED |
| Q-06 | Incident visibility | <=5 s p95 excluding AI | APPROVED |
| Q-07 | AI UX | <=30 s p95 target | APPROVED |
| Q-08 | Common API | <1 s p95 | APPROVED |
| Q-09 | Decoy convergence | <=60 s | APPROVED |
| Q-10 | Performance regression review | >20% | APPROVED |

---

# 35. MVP Scope Change Control

Step 5 owner review tamamlanmıştır. Bu belgeye göre MVP scope veya acceptance kriterlerinde açık `OWNER DECISION` bulunmamaktadır.

Bundan sonra MVP scope, quantitative target veya acceptance criterion değiştirilmek istenirse:

1. İlgili decision ID / AC ID belirlenir.
2. Değişikliğin North Star, security, architecture ve pilot-validasyon etkisi yazılır.
3. Gerekirse focused spike/test yapılır.
4. Yeni yön `RECOMMENDED` olarak sunulur.
5. Product Owner kararı olmadan final MVP baseline değiştirilmez.
6. Onaylanan değişiklik bu master belgeye ve ilgili test/requirement izine işlenir.

AI coding agent'ları capability ekleyemez, acceptance criterion silemez veya MVP non-goal'ünü sessizce scope içine alamaz. Böyle bir ihtiyaç görürse change proposal/draft üretmelidir.

---

# 36. Step 5 Closure / Exit Criteria

Step 5 aşağıdaki koşullarla **CLOSED / APPROVED** kabul edilir:

- [x] MVP release identity
- [x] Target environment/deployment profile
- [x] Edge count/topology
- [x] Onboarding flow
- [x] Discovery boundary
- [x] Decoy portfolio
- [x] Interaction levels
- [x] Threat behavior coverage
- [x] Synthetic credential boundary
- [x] Event/evidence/finding/incident scope
- [x] Detection rules
- [x] Correlation dimensions
- [x] Attacker journey minimum
- [x] Source identification guarantees
- [x] Confidence/severity/noise rules
- [x] Incident UX
- [x] AI capability scope
- [x] AI provider acceptance
- [x] Prompt-injection/evidence grounding gates
- [x] Response guidance
- [x] Notifications
- [x] Operational health
- [x] Update/rollback
- [x] Authentication/MFA/audit
- [x] Data/retention/privacy
- [x] Security release gates
- [x] Failure-mode behavior
- [x] Performance reference workload and thresholds
- [x] Observability
- [x] Attack/failure test suites
- [x] Architecture spike blockers
- [x] Explicit MVP non-goals
- [x] Capability matrix
- [x] Acceptance criteria catalog
- [x] Definition of Done
- [x] MVP Release Gate
- [x] Validation metrics
- [x] No development-time estimate introduced

**Sonuç:** 200 / 200 Step 5 kararı APPROVED. Açık MVP scope/acceptance owner decision bulunmamaktadır.

---

# 37. Final Step 5 Conclusion

Step 5 sonunda onaylanan ana MVP ilkesi:

> **MVP'yi “çok sayıda honeypot çalıştırabilen ilk sürüm” olarak değil, gerçek private networkte küçük ama purposeful deception setiyle detection → evidence → correlation → attacker journey → explanation → guidance → notification → disposition zincirini güvenli biçimde kapatan controlled pilot-ready ürün olarak tanımlamak.**

Onaylanan minimum proof surface:

```text
1 Hosted Control Plane
1 Edge appliance
2–3 routed IPv4 zones
4 decoy families:
  SSH
  HTTP/Admin
  PostgreSQL
  SMB
+ 1 synthetic decoy credential workflow

Core outcomes:
  recon/discovery evidence
  credential abuse evidence
  multi-decoy journey
  high-signal incident
  source IP/zone
  confidence + severity
  AI evidence-backed explanation
  prioritized response guidance
  email/in-app/webhook notification
  operator disposition
  functional health
  offline buffering
  signed updates
  security/failure-mode release gates
```

Buna karşılık high-interaction, real Windows VM, RDP, endpoint/identity agents, fake-network redirection, autonomous AI response, MSSP/multi-tenancy ve broad enterprise integrations bilinçli olarak MVP dışında tutulur.

Bu scope, ürün tezini kanıtlamak için yeterince **complete**, architecture'ın advanced vizyonuna saplanmamak için yeterince **bounded** olacak şekilde Product Owner tarafından onaylanmıştır.


---

# 38. Handoff — Roadmap / Version & Capability Phasing

Step 5'in kapanmasıyla artık MVP'nin **tam olarak ne olduğu** ve “tamamlandı” sayılması için hangi acceptance bar'ını geçmesi gerektiği kesinleşmiştir.

Bir sonraki aşamada:

- MVP içindeki workstream'ler dependency sırasına konulacak,
- architecture spike blocker'ları uygun fazlara yerleştirilecek,
- capability'ler implementation order açısından gruplanacak,
- MVP sonrası `Post-MVP` ve `Future/Advanced` capability'ler versiyon/fazlara ayrılacak,
- her faz için giriş/çıkış kriterleri tanımlanacak,
- development-time tahmini kullanılmayacak,
- repo/agent çalışma modelinin execution planı hazırlanacak.

Roadmap, MVP scope'u yeniden tartışma yeri değildir. Roadmap sırasında bir capability'nin scope değişikliği gerektiği anlaşılırsa önce bu belgenin **MVP Scope Change Control** süreci işletilir.

---

# 39. Document Control

- **Document:** `MVP_Scope_and_Acceptance_Criteria.md`
- **Step:** 5 — MVP Scope & Acceptance Criteria
- **Status:** APPROVED / FINAL
- **Approved decision count:** 200
- **Open owner-decision count:** 0
- **Explicit acceptance criteria catalog:** 69 criteria
- **Quantitative MVP quality-bar decisions:** 10 approved
- **MVP release class:** Controlled pilot-ready MVP
- **Architecture spike backlog:** Classified; blockers explicitly identified
- **MVP non-goals:** APPROVED
- **Definition of Done:** APPROVED
- **MVP Release Gate:** APPROVED
- **Next stage:** Roadmap / Version & Capability Phasing
- **Decision authority:** Product Owner
- **Default artifact format:** Markdown
