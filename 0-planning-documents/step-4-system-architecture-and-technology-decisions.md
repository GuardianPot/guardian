# System Architecture & Technology Decisions
## Step 4 — Final Approved Output

**Belge durumu:** APPROVED / FINAL  
**Karar durumu:** Bu belgede yer alan 202 Adım 4 kararının tamamı Product Owner tarafından `APPROVED` edilmiştir.  
**Karar otoritesi:** Product Owner  
**Girdi:** `Technical_Feasibility_Requirements.md` — Step 3 APPROVED / FINAL  
**Bu belge:** Step 4'ün tamamlanmış final architecture & technology decision çıktısıdır.

### Step 4 sonucu

- **202 / 202 architecture & technology decision:** APPROVED
- **Açık owner decision:** 0
- **Software/component boundaries:** APPROVED
- **Deployment/network architecture:** APPROVED
- **Control Plane / Edge / Decoy / High-Interaction architecture:** APPROVED
- **Data / telemetry / correlation architecture:** APPROVED
- **AI architecture and provider strategy:** APPROVED
- **Security / identity / update / observability architecture:** APPROVED
- **Programming languages and technology baseline:** APPROVED
- **Repository / CI/CD / agentic engineering governance:** APPROVED
- **Validation / architecture spike backlog:** OPEN BY DESIGN
- **Bir sonraki ürün aşaması:** MVP Scope & Acceptance Criteria


---

# 0. Belgenin amacı ve sınırı

Adım 2'de **ne inşa ettiğimizi ve neden** tanımladık.  
Adım 3'te bu ürünün **hangi teknik gereksinimleri, güvenlik sınırları ve feasibility gerçekleri** taşıdığını kapattık.

Adım 4'ün sorusu artık şudur:

> **Onaylanmış ürün ve teknik gereksinimleri hangi sistem mimarisi, component sınırları, protokoller, runtime'lar, veri teknolojileri, güvenlik mekanizmaları ve engineering standartlarıyla gerçekleştireceğiz?**

Bu aşamada artık teknoloji seçmek serbesttir. Ancak teknoloji seçimi “popüler olduğu için” değil, Adım 2 ve Adım 3 kararlarını karşılamak için yapılacaktır.

Her karar yine şu governance akışındadır:

`OPEN → RESEARCHED → RECOMMENDED → OWNER DECISION → APPROVED`

Bu dosyadaki tüm Adım 4 kararları Product Owner tarafından önerilen yönleriyle `APPROVED` edilmiştir.

---

# 0.1 Step 3'ten gelen bağlayıcı architecture constraints

Aşağıdaki ilkeler tekrar tartışılmayan APPROVED girdilerdir:

- Deception-first visibility; full NDR değil.
- Basit networklerde tek presence point, gerektiğinde distributed/multi-segment presence.
- Routed ve local-L2 deception modelleri desteklenebilmeli.
- Attacker-facing deception plane ile management/control plane ayrılmalı.
- Evidence plane attacker-controlled workload'dan korunmalı.
- High-interaction zone güçlü workload + network isolation taşımalı.
- Canonical normalized evidence contract zorunlu.
- Event / evidence / finding / incident / inference ayrımı korunmalı.
- Evidence provenance ve tamper-evident semantics bulunmalı.
- Attacker journey multi-evidence correlation üzerine kurulmalı.
- AI deterministic truth engine değildir.
- AI unavailable olsa dahi core detection/evidence/health devam etmeli.
- High-interaction default capability değildir.
- Production secrets deception plane'e taşınmamalı.
- Internet outage sırasında local capture/buffering devam etmeli.
- Resource abuse ve AI denial-of-wallet sınırlandırılmalı.
- Windows first-class architecture requirement'tır.
- Production AD join/write default değildir.
- Current multi-tenant MSSP platform scope dışıdır.
- Full RBAC, Kubernetes, microservices, SIEM/NDR/SOAR gibi genişleme hedefleri ürünün current requirement'ı değildir.

---

# 0.2 Değerlendirme lensleri

Her karar şu açılardan değerlendirilecektir:

1. **Product fit** — Security-lean SMB persona için complexity ve değer.
2. **Security** — Attack surface, privilege, containment ve evidence integrity.
3. **Operational simplicity** — Deployment, updates, debugging ve failure recovery.
4. **Extensibility** — Endpoint/identity/high-interaction/future MSSP yolunu engelleyip engellemediği.
5. **Agentic development suitability** — AI coding agent'larının açık contract'larla güvenli ve kontrollü çalışabilmesi.
6. **Portability** — Cloud/on-prem/offline future seçenekleri.
7. **Cost/scale** — Gereksiz distributed infrastructure yaratmamak.
8. **Observability/testability** — Sistem davranışının deterministik biçimde test ve debug edilebilir olması.

---

# 0.3 Araştırma dayanakları

Teknoloji ve architecture adayları güncel resmi dokümantasyonlarla kontrol edilmiştir.

- **[S01] Go Release History / Specification** — Go 1.27.0 güncel stable hattı; compiled, statically typed ve concurrency odaklı model.
- **[S02] PostgreSQL 18 Documentation / Versioning Policy** — PostgreSQL 18 current supported; JSONB, declarative partitioning ve relational model.
- **[S03] SQLite WAL Documentation** — single-host local durable queue/state için WAL concurrency ve crash recovery özellikleri.
- **[S04] gRPC Official Documentation** — HTTP/2 tabanlı bidirectional streaming ve TLS/mTLS authentication.
- **[S05] Buf Documentation** — Protobuf linting ve breaking-change detection.
- **[S06] containerd Documentation** — OCI runtime integration ve namespaces; containerd namespaces'in security boundary olmadığı uyarısı.
- **[S07] QEMU / libvirt Documentation** — KVM acceleration, snapshots, network modes ve Windows/Linux full-system virtualization.
- **[S08] Firecracker Documentation** — KVM microVM isolation ve düşük overhead; Linux/OSv guest odağı.
- **[S09] Linux Kernel networking / IPVLAN + Netfilter nftables** — network namespace/presence ve policy primitives.
- **[S10] OpenTelemetry** — vendor-neutral traces/metrics/logs instrumentation ve Collector.
- **[S11] Prometheus** — autonomous time-series monitoring.
- **[S12] Grafana Loki** — metadata-indexed log aggregation, object-storage-oriented model.
- **[S13] Sigstore Cosign** — artifact/container signature verification.
- **[S14] The Update Framework (TUF)** — rollback/freeze/key compromise gibi update attacks'e karşı signed metadata model.
- **[S15] SPIFFE/SPIRE** — workload identity ve short-lived X.509 SVID modeli; değerlendirilmiş ancak başlangıç için ağır olabilir.
- **[S16] React + TypeScript + Vite official documentation** — typed SPA/tooling.
- **[S17] OpenAI Responses API** — JSON Schema structured outputs ve function/tool calling.
- **[S18] Anthropic Claude API** — structured outputs ve strict tool schemas.
- **[S19] Google Gemini API** — structured outputs ve function calling.
- **[S20] OpenCanary** — BSD-3-Clause, Python, low-interaction multi-protocol honeypot.
- **[S21] Cowrie** — SSH/Telnet medium/high-interaction capabilities, container support ve current development.
- **[S22] T-Pot** — multi-honeypot containerized appliance architecture için referans.
- **[S23] NATS JetStream** — persistence, consumers, acknowledgements ve edge/leaf topology seçenekleri.
- **[S24] Apache Kafka** — durable event log ve high-scale stream-processing semantics.
- **[S25] Debian 13 / Ubuntu 26.04 LTS** — current supported Linux appliance candidates.

---

# 0.4 Approved Architecture Baseline

Aşağıdaki baseline bu dokümanda ayrıntılı olarak tartışılacaktır:

```text
                        ┌───────────────────────────────┐
                        │        CONTROL PLANE          │
                        │  Go modular monolith          │
                        │                               │
                        │ Environment / Config          │
                        │ Evidence / Incidents          │
                        │ Correlation / Journey         │
                        │ AI orchestration              │
                        │ Notifications                 │
                        │ Updates / Device Mgmt         │
                        └───────────────┬───────────────┘
                                        │
                              HTTPS / gRPC + mTLS
                                        │
                        ┌───────────────▼───────────────┐
                        │          EDGE NODE             │
                        │   native Go daemon on Linux    │
                        │                                │
                        │ desired-state reconciler       │
                        │ local detection                │
                        │ telemetry normalization        │
                        │ SQLite buffer                  │
                        │ containerd runtime control     │
                        │ Linux network primitives       │
                        └───────┬─────────┬──────────────┘
                                │         │
                   ┌────────────▼─┐    ┌──▼──────────────┐
                   │ Decoy Packs  │    │ Context/Health  │
                   │ OCI runtimes │    │ Discovery       │
                   └──────────────┘    └─────────────────┘

Future:
   Endpoint Deception Agent
   High-Interaction Worker → QEMU/KVM/libvirt
```

**Approved software baseline:**
- 3 core applications: Control Plane, Web Console, Edge Node
- 1 extensible Decoy Runtime ecosystem
- Future deployables: Endpoint Agent + High-Interaction Worker

---

# 1. 4.1 — System Decomposition & Software Boundaries

## Karar SD-01 — Başlangıçta microservices mı modular monolith mi?

**Durum:** APPROVED

**Seçenek A — Microservices**
- Incident, telemetry, correlation, AI, notifications, device management ayrı servisler.
- Bağımsız scaling/deployment sağlar.
- Network calls, distributed transactions, schema/versioning, observability ve CI complexity büyür.

**Seçenek B — Tek monolith**
- Basit.
- Domain boundary'leri eriyebilir; ileride ayrıştırma zorlaşabilir.

**Seçenek C — Modular monolith Control Plane + bağımsız edge/deception sınırları**
- Control Plane içinde compile-time/domain modülleri.
- Gerçek deployment/security sınırı gereken Edge ve high-interaction ayrı process/deployable.
- Internal modüller açık interface/event contract kullanır.

**Karşıt görüş:** “Cybersecurity ürünü ölçeklenecek, baştan microservice doğru olur.”  
Bu yaklaşım mevcut customer scale ve ekip modelinden önce operational complexity satın alır. Dağıtık sistem sınırları yalnız gerçek deployment/security/scale gereksinimi varsa kurulmalıdır.

**APPROVED DECISION — C.**

**Gerekçe:** Step 3'teki security plane ayrımlarını korur ama gereksiz distributed infrastructure yaratmaz.

---

## Karar SD-02 — Kaç ana deployable software component kabul edilmeli?

**Durum:** APPROVED

**A — Tek appliance**
- Web, control, edge, decoy her şey tek sistem.
- Basit demo; trust boundary ve cloud/on-prem ayrımı kötü.

**B — 3 core app + decoy ecosystem**
1. Control Plane
2. Web Console
3. Edge Node
4. Decoy Packs bir plugin/runtime ailesi

**C — Baştan 6–10 servis**
- Endpoint, AI, correlation, notification, high interaction vb. ayrı.

**APPROVED DECISION — B.**

**Not:** Web Console ayrı codebase/application'dır ancak production'da static artifact olarak Control Plane tarafından serve edilebilir; deployable sayısı ile codebase sınırı aynı olmak zorunda değildir.

---

## Karar SD-03 — Future Endpoint Agent şimdi ayrı deployable olarak tasarlanmalı mı?

**Durum:** APPROVED

**A — Şimdi implement edilir**
- Future vision hızlı açılır.
- Current network-first scope büyür.

**B — Contract/interface reservation yapılır, implementation deferred**
- Product model endpoint capability'yi engellemez.
- Current architecture complexity artmaz.

**APPROVED DECISION — B.**

---

## Karar SD-04 — High-Interaction Worker Edge Node ile aynı process mi?

**Durum:** APPROVED

**A — Aynı daemon**
- Tek install.
- KVM/libvirt privileges ve attacker-controlled VM lifecycle Edge TCB'yi büyütür.

**B — Ayrı deployable worker**
- Stronger isolation ve optional installation.
- High interaction kullanmayan customer ek dependency taşımaz.

**APPROVED DECISION — B.**

---

## Karar SD-05 — AI ayrı microservice mi?

**Durum:** APPROVED

**A — Ayrı AI service**
- Provider/scale isolation.
- Başlangıçta unnecessary RPC/deployment boundary.

**B — Control Plane içinde `ai` domain module**
- Structured provider interface.
- Daha sonra process boundary'ye çıkarılabilir.

**APPROVED DECISION — B.**

---

## Karar SD-06 — Correlation ayrı service mi?

**Durum:** APPROVED

**A — Ayrı stream-processing service**
- Independent scale.
- Event broker/state store zorunluluğu yaratır.

**B — Control Plane domain module + background workers**
- Transaction/evidence context'e yakın.
- İlk scale için yeterli.

**APPROVED DECISION — B.**

---

## Karar SD-07 — Notification engine ayrı service mi?

**Durum:** APPROVED

**APPROVED DECISION — Hayır.**
Control Plane içerisinde provider-agnostic notification module. Outbox üzerinden async delivery. Daha sonra ayrı service ancak gerçek scale/availability ihtiyacı doğarsa.

---

## Karar SD-08 — Decoy'lar product process'iyle aynı binary mi?

**Durum:** APPROVED

**A — Edge binary içine compile edilir**
- Tek artifact.
- Her decoy Edge TCB'ye girer; independent update zor.

**B — Ayrı OCI runtime/packs**
- Failure/isolation/versioning daha iyi.
- container runtime gerekir.

**C — External processes, mixed**
- Esnek ama lifecycle standardization zor.

**APPROVED DECISION — B primary, C adapter path.**

---

# 2. 4.2 — Deployment Topology

## Karar DT-01 — Edge deployment şekli

**Durum:** APPROVED

**A — Customer'ın mevcut sunucusuna normal application install**
- Düşük footprint.
- Production workload ile trust/port/network conflict.

**B — Dedicated Linux VM/appliance**
- Isolation, predictable networking, lifecycle.
- Ek VM resource'u.

**C — Customer Kubernetes cluster'ında pod**
- Cloud-native.
- Primary SMB persona ve L2 deception için gereksiz dependency.

**APPROVED DECISION — B.**

**Secondary:** Lab/dev için package/native install desteklenebilir.

---

## Karar DT-02 — Edge host işletim sistemi

**Durum:** APPROVED

**A — Ubuntu 26.04 LTS**
- Yaygın enterprise/server ecosystem, uzun destek.

**B — Debian 13 stable**
- Minimal, conservative, predictable appliance base; 2030'a uzanan lifecycle.

**C — Distro-agnostic arbitrary Linux**
- Esnek.
- Network/runtime matrix test yükü büyür.

**APPROVED DECISION — Debian 13 stable appliance image; supported-host policy daha sonra Ubuntu LTS'yi ikinci hedef olarak ekleyebilir.**

**Karşıt görüş:** Ubuntu daha yaygın.  
**Cevap:** Appliance'ın kendi OS'ini taşıdığı modelde minimal predictable base daha değerlidir; customer host compatibility'si hypervisor seviyesine taşınır.

---

## Karar DT-03 — Edge CPU architecture

**Durum:** APPROVED

**A — amd64 only**
- Windows/KVM/enterprise hypervisor uyumu en geniş.
- ARM edge cihazlarını dışlar.

**B — amd64 + arm64 from day one**
- Portability.
- VM images/decoy tests iki kat matris.

**APPROVED DECISION — amd64 primary; Go/OCI contracts arm64-compatible tutulur, arm64 validation deferred.**

---

## Karar DT-04 — Control Plane deployment modeli

**Durum:** APPROVED

**A — SaaS-only**
- Operasyon merkezi.
- Future offline/on-prem zor.

**B — On-prem-only**
- Security/privacy.
- Initial AI/cloud operasyonu ve distribution daha zor.

**C — Aynı Control Plane artifact'i cloud-hosted veya customer-hosted çalışabilecek şekilde**
- Single codebase.
- Deployment profile ayrımı.

**APPROVED DECISION — C.**

---

## Karar DT-05 — Initial Control Plane runtime

**Durum:** APPROVED

**A — Kubernetes**
- HA/scale.
- Üründen önce platform işletme ihtiyacı.

**B — OCI containers + Compose/single-host deployment**
- Basit, portable.
- Extreme scale değil.

**C — Native systemd services**
- Az dependency.
- On-prem packaging daha zahmetli.

**APPROVED DECISION — B. Kubernetes current requirement değildir.**

---

## Karar DT-06 — Hosted Control Plane için cloud vendor lock-in

**Durum:** APPROVED

**A — AWS-native**
**B — Azure-native**
**C — GCP-native**
**D — Portable OCI + PostgreSQL + S3-compatible abstractions; vendor daha sonra deployment kararı**

**APPROVED DECISION — D.**

Cloud-specific managed services kullanılabilir ancak application contract vendor API'sine doğrudan bağlanmamalıdır.

---

## Karar DT-07 — Web Console ayrı server mı?

**Durum:** APPROVED

**A — Ayrı Node.js web server**
- SSR mümkün.
- Authenticated security console için gereksiz runtime.

**B — Static SPA build; Control Plane veya reverse proxy serve eder**
- Basit distribution.
- SSR/SEO gerekmiyor.

**APPROVED DECISION — B.**

---

## Karar DT-08 — Customer başına Control Plane instance modeli

**Durum:** APPROVED

**A — Shared multi-tenant SaaS**
- Efficient.
- Tenant isolation/RBAC Step 2 scope dışı.

**B — Single-organization logical instance**
- Product model basit.
- Hosted ortamda gerekirse customer isolation deployment-level yapılabilir.

**APPROVED DECISION — B.**

Data modelde `organization/environment ownership` semantic'i korunur; current tenant-management feature yazılmaz.

---

# 3. 4.3 — Control Plane Architecture

## Karar CP-01 — Control Plane primary language

**Durum:** APPROVED

**A — Go**
- Network/concurrency, static deployment, strong typing, low runtime footprint.
- UI/data science ecosystem Python/TS kadar zengin değil.

**B — Python**
- AI/security ecosystem güçlü.
- Long-running concurrent device-control service için runtime/packaging discipline daha fazla gerekir.

**C — Java/Kotlin**
- Mature enterprise.
- Appliance/side-project simplicity ve binary footprint daha ağır.

**D — Rust**
- Memory safety/performance.
- Development complexity ve library ergonomics daha yüksek.

**APPROVED DECISION — Go 1.27.0.**

Go'nun güncel stable hattı 1.27.0'dır [S01]. Primary control ve edge dilinin aynı olması shared protocol/security utilities için avantajdır.

---

## Karar CP-02 — Control Plane internal architecture style

**Durum:** APPROVED

**A — Layered CRUD application**
**B — Domain-oriented modular monolith**
**C — Generic plugin microkernel**

**APPROVED DECISION — B.**

Önerilen modules:
- `environment`
- `devices`
- `deception`
- `telemetry`
- `evidence`
- `incidents`
- `correlation`
- `journey`
- `ai`
- `notifications`
- `updates`
- `auth`
- `audit`

Cross-module direct table access yasaklanmalı; public module API veya domain event kullanılmalı.

---

## Karar CP-03 — HTTP server/framework

**Durum:** APPROVED

**A — Büyük web framework**
**B — Go standard `net/http` + minimal router/middleware**
**C — GraphQL framework**

**APPROVED DECISION — B.**

Go stdlib güçlüdür; routing/middleware için minimal library kullanılabilir. Framework product architecture'ı belirlememelidir.

---

## Karar CP-04 — Web/public API style

**Durum:** APPROVED

**A — GraphQL**
- Flexible UI.
- Authorization/caching/schema complexity.

**B — REST/JSON + OpenAPI**
- Ecosystem ve integrations için açık.

**C — Protobuf/Connect-only**
- Strong contract.
- External integration ergonomics daha teknik.

**APPROVED DECISION — REST/JSON + OpenAPI for Web/public API; Protobuf/gRPC device plane için.**

---

## Karar CP-05 — Background jobs

**Durum:** APPROVED

**A — Ayrı worker service**
**B — Control Plane process içinde durable job runner**
**C — Cloud queue/serverless**

**APPROVED DECISION — B initially.**

Jobs DB-backed durable state taşımalı; process crash sonrası resumable olmalı.

---

## Karar CP-06 — Control Plane statefulness

**Durum:** APPROVED

**APPROVED DECISION:** Application processes **stateless-ish** tutulmalı; durable state PostgreSQL/object storage'da. In-memory cache correctness source'u olmamalı.

---

## Karar CP-07 — Horizontal scaling current requirement mı?

**Durum:** APPROVED

**APPROVED DECISION:** Architecture horizontal scale'ı **engellememeli**, ancak initial deployment tek Control Plane instance ile çalışabilmeli. Background jobs için distributed locking/lease semantics DB üzerinden tasarlanmalı.

---

## Karar CP-08 — Server-side rendered UI gerekli mi?

**Durum:** APPROVED

**APPROVED DECISION — Hayır.**
Authenticated operational console; SEO ve public content ihtiyacı yok. SPA daha basit.

---

# 4. 4.4 — Edge Node Architecture

## Karar EN-01 — Edge Node language

**Durum:** APPROVED

**APPROVED DECISION — Go 1.27.0.**

Gerekçe:
- Tek static binary distribution.
- Strong concurrency/networking.
- Linux system APIs için yeterli ecosystem.
- Control Plane ile shared Protobuf/security packages.
- AI agents için nispeten küçük/sade language surface.

**Karşıt görüş — Rust:** Memory safety daha güçlüdür.  
**Tavır:** Rust future privilege-separated low-level helper için değerlendirilebilir; bütün Edge'i Rust'a taşımak current complexity için gerekçeli değildir.

---

## Karar EN-02 — Edge container içinde mi native host daemon mı?

**Durum:** APPROVED

**A — Container**
- Package kolay.
- Host network namespaces/container runtime'ı yönetmek için privileged Docker-socket benzeri erişim gerekir.

**B — Native root-owned system service**
- Network/runtime primitives üzerinde explicit control.
- Packaging/update sorumluluğu.

**APPROVED DECISION — B.**

Edge `systemd` service olarak çalışır; privileged operations internal privilege-separated helpers ile sınırlandırılır.

---

## Karar EN-03 — Edge tek process mi supervisor + helpers mı?

**Durum:** APPROVED

**A — Monolithic root daemon**
- Basit.
- TCB büyük.

**B — Unprivileged main daemon + narrow privileged helper**
- Network/container operations least privilege.
- IPC gerekir.

**APPROVED DECISION — B.**

Main agent mümkün olduğunca non-root; `edge-privd` benzeri küçük helper yalnız netlink/nftables/container lifecycle gibi yetkili işlemleri expose eder.

---

## Karar EN-04 — Privileged helper API

**Durum:** APPROVED

**A — Shell command execution**
- Kolay ama injection/audit riski.

**B — Typed local RPC over Unix domain socket**
- Allowlisted operations, typed arguments.

**APPROVED DECISION — B.**

---

## Karar EN-05 — Edge local persistent state

**Durum:** APPROVED

**A — Flat files**
**B — PostgreSQL**
**C — SQLite WAL**
**D — Embedded KV DB**

**APPROVED DECISION — SQLite WAL.**

SQLite single-host model ve WAL concurrency/crash recovery [S03] edge için uygundur.

Edge DB:
- desired/observed config revisions
- local event queue metadata
- decoy lifecycle
- health
- enrollment/cert metadata
- retry state

Large binary payloads SQLite içine gömülmemeli.

---

## Karar EN-06 — Large local spool

**Durum:** APPROVED

**APPROVED DECISION:** Filesystem content-addressed spool + SQLite metadata/index.

Uploaded malware/raw transcripts ayrı files; hash ile adreslenir.

---

## Karar EN-07 — Edge desired-state modeli

**Durum:** APPROVED

**A — imperative commands (“start X”)**
**B — declarative desired state + reconciliation loop**

**APPROVED DECISION — B.**

Control Plane revisioned desired state yollar; Edge actual state'i converge eder ve status/condition döner.

---

## Karar EN-08 — Edge offline behavior

**Durum:** APPROVED

**APPROVED DECISION:**
- last-known-good config çalışmaya devam eder.
- local detection devam eder.
- telemetry local queue'da birikir.
- destructive config change yapılmaz.
- capacity threshold health warning üretir.
- reconnect sonrası idempotent replay.

---

## Karar EN-09 — Edge local detection scope

**Durum:** APPROVED

**A — Raw forwarding only**
**B — Basic deterministic rules + rate/dedup + evidence formation**
**C — Full correlation/AI**

**APPROVED DECISION — B.**

Edge outage sırasında core security signal kaybolmamalı; global journey correlation Control Plane'de.

---

## Karar EN-10 — Edge plugin model

**Durum:** APPROVED

**A — In-process Go plugins**
- Version/ABI/security risk.

**B — Process/OCI boundary + versioned contract**
- Language-independent.

**APPROVED DECISION — B.**

Go runtime plugin mechanism product extension boundary olarak kullanılmamalı.

---

## Karar EN-11 — Discovery engine Edge içinde mi?

**Durum:** APPROVED

**APPROVED DECISION — Evet, bounded discovery executor Edge module'ü.**
Control Plane policy/consent verir; actual network probes local Edge'den çıkar.

---

## Karar EN-12 — Edge management UI local mı?

**Durum:** APPROVED

**A — Full local UI**
- Offline useful.
- Duplicated auth/UI.

**B — Minimal diagnostics CLI/local status endpoint**
- Main UX Control Plane.

**APPROVED DECISION — B.**

---

# 5. 4.5 — Decoy Runtime Architecture

## Karar DR-01 — Low/medium interaction runtime technology

**Durum:** APPROVED

**A — Native host processes**
**B — OCI containers**
**C — VM per decoy**

**APPROVED DECISION — OCI containers.**

Isolation/lifecycle/image versioning için iyi denge. Ancak container **high-interaction security boundary değildir** [S06].

---

## Karar DR-02 — Container runtime

**Durum:** APPROVED

**A — Docker Engine**
- Kullanıcı dostu.
- Product embedding için daha büyük daemon/API surface.

**B — containerd + runc**
- Lower-level embeddable OCI runtime.
- Direct lifecycle control.

**C — Podman**
- Daemonless/rootless seçenekler.
- Product orchestration API modeli farklı.

**APPROVED DECISION — containerd 2.x + OCI runtime (`runc`) baseline.**

Kubernetes gerektirmez. containerd namespace'leri isolation security feature sayılmamalıdır [S06].

---

## Karar DR-03 — Decoy pack packaging

**Durum:** APPROVED

**APPROVED DECISION — Signed OCI images + manifest metadata.**

Manifest:
- decoy type/version
- interaction level
- exposed ports/protocols
- privilege/capability needs
- network modes
- telemetry contract version
- health probe
- resource limits
- egress policy
- supported architectures
- upstream license/provenance

---

## Karar DR-04 — Decoy runtime contract

**Durum:** APPROVED

**A — Parse stdout logs**
**B — Standard local telemetry RPC**
**C — Both**

**APPROVED DECISION — C.**

Native/custom decoys typed UDS/Protobuf event contract kullanır. Third-party OSS adapters structured/file log'u normalize edebilir.

---

## Karar DR-05 — Decoy config format

**Durum:** APPROVED

**APPROVED DECISION:** Versioned declarative schema, human-readable representation YAML/JSON olabilir; internal canonical representation Protobuf/domain struct.

Config arbitrary shell/env injection'a izin vermemeli.

---

## Karar DR-06 — OSS honeypot strategy

**Durum:** APPROVED

**A — Her şeyi sıfırdan yaz**
- Control.
- Yavaş capability breadth.

**B — Tüm ürünü OpenCanary/T-Pot üzerine kur**
- Hızlı.
- Product architecture upstream limits'e bağlanır.

**C — Adapter-based selective integration**
- OpenCanary/Cowrie gibi mature tools capability pack olarak.
- Product truth/evidence contract bize ait.

**APPROVED DECISION — C.**

OpenCanary current BSD-3-Clause ve low-interaction multi-protocol adaydır [S20]. Cowrie SSH/Telnet medium/high interaction için güçlü adaydır [S21].

---

## Karar DR-07 — OpenCanary'ın rolü

**Durum:** APPROVED

**APPROVED DECISION:** Universal core engine değil; **candidate low-interaction pack/reference implementation**.

Sebep:
- Python/Twisted upstream architecture product Edge lifecycle'ımızı belirlememeli.
- Bazı network features iptables gibi assumptions taşıyabilir.
- Telemetry own canonical schema'ya normalize edilmelidir.

---

## Karar DR-08 — Cowrie'nin rolü

**Durum:** APPROVED

**APPROVED DECISION:** SSH/Telnet medium-interaction için **strong integration candidate**.

Wrapper:
- isolated OCI container
- outbound policy
- canonical session/command/file events
- upstream version pinning
- security patch monitoring

---

## Karar DR-09 — Resource controls

**Durum:** APPROVED

**APPROVED DECISION:** Her decoy için cgroup CPU/memory/PID/file/disk quotas ve connection/session limits.

Attacker resource abuse ürünün host'u tüketmemeli.

---

## Karar DR-10 — Container hardening baseline

**Durum:** APPROVED

**APPROVED DECISION:**
- non-root where protocol permits
- read-only rootfs where possible
- dropped capabilities
- no-new-privileges
- seccomp
- AppArmor/SELinux profile
- dedicated network namespace
- no host filesystem mounts except explicit read-only artefacts
- no container runtime socket
- bounded tmpfs
- default-deny egress

---

# 6. 4.6 — High-Interaction Architecture

## Karar HI-ARCH-01 — High-interaction virtualization technology

**Durum:** APPROVED

**A — Containers**
- Insufficient boundary for intentionally compromised OS.

**B — Firecracker**
- Strong/light microVM; Linux/OSv focus [S08].
- Windows requirement'i karşılamaz.

**C — QEMU/KVM**
- Linux + Windows, broad device/network/snapshot support [S07].

**D — VMware/Hyper-V APIs**
- Customer dependency.

**APPROVED DECISION — QEMU/KVM baseline.**

---

## Karar HI-ARCH-02 — VM management layer

**Durum:** APPROVED

**A — Raw QEMU CLI/API**
- Maximum control.
- Lifecycle/network/snapshot code yükü.

**B — libvirt**
- Mature domain/network/storage/snapshot abstraction [S07].

**APPROVED DECISION — libvirt + QEMU/KVM.**

---

## Karar HI-ARCH-03 — High-interaction worker placement

**Durum:** APPROVED

**A — Normal Edge host**
- Simpler.
- Nested virtualization/customer hypervisor restrictions ve risk.

**B — Optional dedicated HI worker/node**
- Strong boundary.
- Only customers using feature install.

**APPROVED DECISION — B.**

Dev/lab aynı hostta çalışabilir; production recommendation ayrı node.

---

## Karar HI-ARCH-04 — Guest disk model

**Durum:** APPROVED

**APPROVED DECISION:** Read-only/golden base image + disposable copy-on-write overlay (qcow2/external snapshot semantics). Engagement sonrası overlay quarantine/destroy.

---

## Karar HI-ARCH-05 — Windows images

**Durum:** APPROVED

**A — Product redistributes prebuilt Windows images**
- UX iyi.
- Licensing/redistribution complexity.

**B — Customer supplies licensed media/base image; product automation prepares template**
- Legal boundary daha güvenli.

**APPROVED DECISION — B initial.**

---

## Karar HI-ARCH-06 — Linux high-interaction images

**Durum:** APPROVED

**APPROVED DECISION:** Product-owned hardened/golden templates + intentionally vulnerable application layers separately versioned.

---

## Karar HI-ARCH-07 — VM egress

**Durum:** APPROVED

**APPROVED DECISION:** Separate isolated virtual network + nftables default-deny egress. DNS/HTTP controlled simulation/proxy future capability; arbitrary production/internal routing yasak.

---

## Karar HI-ARCH-08 — Malware capture

**Durum:** APPROVED

**APPROVED DECISION:** Guest→collector out-of-band channel veya host-side disk/network capture; operator'a file doğrudan executable olarak expose edilmez.

---

## Karar HI-ARCH-09 — Firecracker future role

**Durum:** APPROVED

**APPROVED DECISION:** Linux-only ephemeral interaction farms için future optimization candidate; Windows-first high-interaction baseline olamaz.

---

# 7. 4.7 — Networking Architecture

## Karar NW-01 — Edge network control primitives

**Durum:** APPROVED

**APPROVED DECISION:** Linux network namespaces + iproute2/netlink + nftables.

Shelling out mümkün olduğunca azaltılır; Go netlink libraries/typed helper kullanılır. nftables policy source of truth generated declarative ruleset olur.

---

## Karar NW-02 — Decoy presence driver abstraction

**Durum:** APPROVED

**APPROVED DECISION:** En az üç logical driver:
1. `routed/alias`
2. `macvlan`
3. `ipvlan`

Capability/customer network'e göre seçilir.

---

## Karar NW-03 — Default presence mode

**Durum:** APPROVED

**A — macvlan unique L2 identity**
- Realistic MAC.
- Switch port-security/multi-MAC problems.

**B — ipvlan**
- Parent MAC sharing, port-security friendly.
- L2 fingerprint realism lower.

**C — routed/secondary-IP**
- Setup kolay.
- Shared host MAC/routing configuration.

**APPROVED DECISION:** **Routed/secondary-IP default**, `macvlan` advanced realism, `ipvlan` compatibility fallback.

Product onboarding reachability test hangi mode'un çalıştığını doğrulamalı.

---

## Karar NW-04 — VLAN support

**Durum:** APPROVED

**APPROVED DECISION:** Edge NIC üzerinde optional 802.1Q subinterface/trunk support architecture capability'si. UI kullanıcıdan VLAN ID ve interface mapping alabilir; auto-trunk config varsayılmaz.

---

## Karar NW-05 — Multi-NIC

**Durum:** APPROVED

**APPROVED DECISION:** Edge logical presence profiles belirli physical/virtual interface'e bind edilebilmeli. Management interface deception interface'inden ayrılabilmeli.

---

## Karar NW-06 — DHCP ile decoy IP alma

**Durum:** APPROVED

**A — DHCP**
- Natural.
- Lease control/audit ve predictable address zor.

**B — Static/reserved IP**
- Predictable.

**C — Both**
- Future realism.

**APPROVED DECISION — static/reserved IP baseline; DHCP decoy persona future.**

---

## Karar NW-07 — IP conflict detection

**Durum:** APPROVED

**APPROVED DECISION:** Placement öncesi ARP/NDP/probe + passive conflict checks; conflict halinde deployment fail-safe durur.

---

## Karar NW-08 — IPv6

**Durum:** APPROVED

**A — IPv4 only**
**B — Dual stack from day one**

**APPROVED DECISION:** Data/contracts IPv6-capable from day one; initial decoy packs IPv4-first olabilir. Source/IP types hiçbir yerde IPv4-specific string assumption taşımamalı.

---

## Karar NW-09 — Decoy outbound network

**Durum:** APPROVED

**APPROVED DECISION:** Policy engine default-deny; allow rules capability manifest tarafından istenir ve owner-approved policy ile sınırlandırılır.

---

## Karar NW-10 — Edge management connection

**Durum:** APPROVED

**APPROVED DECISION:** Edge→Control outbound initiated; inbound internet-exposed management port gerektirmez.

---

## Karar NW-11 — VPN/overlay technology

**Durum:** APPROVED

**A — WireGuard overlay zorunlu**
- Stable private routing.
- Ürün VPN'e yaklaşır.

**B — Application-level mTLS connection yeter**
- Basit.

**C — Overlay optional for complex sites**

**APPROVED DECISION — B baseline, C future.**  
Control/telemetry için application mTLS yeterlidir; deception traffic overlay'den geçmemelidir.

---

## Karar NW-12 — Passive packet capture

**Durum:** APPROVED

**APPROVED DECISION:** Core runtime requirement değil. Optional context collector interface geleceğe açık tutulur; raw pcap default capture edilmez.

---

## Karar NW-13 — DNS behavior

**Durum:** APPROVED

**APPROVED DECISION:** Decoy hostname/DNS integration explicit capability:
- local static mapping
- customer DNS manual record
- future managed DNS integration

Product production DNS'ine default write yapmaz.

---

## Karar NW-14 — Network policy auditability

**Durum:** APPROVED

**APPROVED DECISION:** Edge uyguladığı nftables/network namespace state'ini desired-state revision'a bağlar; operator “neden bu route/rule var?” görebilir. Generated rules doğrudan user-edit edilen opaque state olmamalıdır.

---

# 8. 4.8 — Control Plane ↔ Edge Communication

## Karar CM-01 — Primary transport

**Durum:** APPROVED

**A — REST polling**
- Firewall-friendly.
- Real-time/reconnect/bidirectional config daha kaba.

**B — WebSocket**
- Bidirectional.
- Typed schema/tooling gRPC kadar güçlü değil.

**C — gRPC bidirectional streaming over HTTP/2**
- Typed Protobuf, flow control, streaming, TLS/mTLS [S04].

**APPROVED DECISION — C.**

---

## Karar CM-02 — Connection initiation

**Durum:** APPROVED

**APPROVED DECISION:** Edge always dials outbound Control Plane endpoint, default TCP/443. Control Plane Edge'e doğrudan inbound connect etmez.

---

## Karar CM-03 — Authentication

**Durum:** APPROVED

**APPROVED DECISION:** Mutual TLS with per-device X.509 certificate.

User/API auth ile device identity ayrı security domain.

---

## Karar CM-04 — Enrollment

**Durum:** APPROVED

**APPROVED DECISION:** Short-lived one-time enrollment token → device keypair generated on Edge → CSR/attestation metadata → Control Plane issues device certificate → token invalidated.

Private key Edge'den çıkmaz.

---

## Karar CM-05 — Certificate lifecycle

**Durum:** APPROVED

**APPROVED DECISION:** Short/medium-lived certs + automatic rotation + revocation/device disable semantics. Exact lifetime final security tuning'de belirlenir.

---

## Karar CM-06 — Wire schema

**Durum:** APPROVED

**APPROVED DECISION:** Protobuf v3 + Buf lint/breaking checks [S05].

Schema packages explicit version:
`product.edge.v1`, `product.telemetry.v1` gibi.

---

## Karar CM-07 — Delivery semantics

**Durum:** APPROVED

**A — Exactly-once**
- Distributed “exactly once” illusion/complexity.

**B — At-most-once**
- Evidence loss.

**C — At-least-once + idempotent event IDs**
- Retry safe.

**APPROVED DECISION — C.**

---

## Karar CM-08 — Config synchronization

**Durum:** APPROVED

**APPROVED DECISION:** revisioned desired-state snapshots + incremental commands only as optimization. Edge ACK observed revision and conditions.

---

## Karar CM-09 — Large binary transfer

**Durum:** APPROVED

**APPROVED DECISION:** Control stream'i büyük malware/file payload ile bloklama. Chunked/blob upload API; metadata event önce gelir, binary asynchronous.

---

## Karar CM-10 — Compatibility policy

**Durum:** APPROVED

**APPROVED DECISION:** Control Plane en az bir önceki Edge protocol minor generation ile backward-compatible olmalı; incompatible major upgrade explicit state üretir. Exact support window final release policy'de belirlenir.

---

# 9. 4.9 — Telemetry Pipeline

## Karar TP-01 — Canonical serialization

**Durum:** APPROVED

**A — JSON everywhere**
- Debug kolay.
- Wire/storage inefficiency ve schema drift.

**B — Protobuf internal canonical + JSON projections**
- Typed versioning.

**APPROVED DECISION — B.**

---

## Karar TP-02 — Event IDs

**Durum:** APPROVED

**APPROVED DECISION:** Globally unique, time-sortable IDs (UUIDv7-class semantics). Edge event ID'yi üretir; retry aynı ID'yi korur.

---

## Karar TP-03 — Timestamp model

**Durum:** APPROVED

**APPROVED FIELDS:**
- `event_time`
- `observed_time`
- `ingested_time`
- `source_clock_offset/quality` when known

UTC canonical.

---

## Karar TP-04 — Edge normalization

**Durum:** APPROVED

**APPROVED DECISION:** Third-party/raw decoy logs Edge'de canonical event'e normalize edilir. Control Plane vendor-specific parser jungle olmamalı.

---

## Karar TP-05 — Batching

**Durum:** APPROVED

**APPROVED DECISION:** Small bounded batches + backpressure. Critical direct evidence batch beklemeden flush edilebilir; flood event'leri batch/dedup edilir.

---

## Karar TP-06 — Central message broker

**Durum:** APPROVED

**A — Kafka**
- High scale/replay; operationally heavy [S24].

**B — NATS JetStream**
- Daha hafif durable messaging, edge topology seçenekleri [S23].

**C — Broker yok; gRPC ingest + PostgreSQL durable outbox/jobs**
- Basit.

**APPROVED DECISION — C initial.**

**Future trigger:** Independent consumers/replay fan-out veya ingest throughput PostgreSQL/job modelini zorladığında NATS JetStream ilk aday; Kafka ancak gerçekten high-scale stream platform ihtiyacı oluşursa.

---

## Karar TP-07 — Exactly-once processing

**Durum:** APPROVED

**APPROVED DECISION:** End-to-end exactly-once hedeflenmez. Idempotency keys, unique constraints ve transactional state transitions ile duplicate-safe processing.

---

## Karar TP-08 — Raw event retention

**Durum:** APPROVED

**APPROVED DECISION:** Normalized raw event relational store; large opaque payload object storage. Retention class policy.

---

## Karar TP-09 — Backpressure

**Durum:** APPROVED

**APPROVED FLOW:**
Control slowdown → gRPC flow control → Edge queue → bounded spool thresholds → drop policy only lowest-value duplicate/noise classes; high-value evidence loss explicit critical health event.

---

## Karar TP-10 — Telemetry integrity

**Durum:** APPROVED

**APPROVED DECISION:** Each event carries device identity, decoy ID, runtime version, schema version and ingestion provenance. Optional chain/hash batches future; central ingestion audit immutable-ish.

---

# 10. 4.10 — Data Architecture

## Karar DA-01 — Primary central database

**Durum:** APPROVED

**A — PostgreSQL**
**B — MySQL**
**C — Document DB**
**D — ClickHouse only**

**APPROVED DECISION — PostgreSQL 18.x.**

Relational incidents/config + JSONB flexible evidence + partitioning [S02].

---

## Karar DA-02 — PostgreSQL version policy

**Durum:** APPROVED

**APPROVED DECISION:** Support current PostgreSQL major and one stable previous major; deployment pins tested minor version, security updates regularly. Approved baseline PostgreSQL 18.

---

## Karar DA-03 — Event/evidence partitioning

**Durum:** APPROVED

**APPROVED DECISION:** Time-based declarative partitioning for high-volume event tables. Partitioning should be introduced before scale makes migration painful, but over-partitioning avoided [S02].

---

## Karar DA-04 — JSONB kullanımı

**Durum:** APPROVED

**APPROVED DECISION:** Canonical common fields relational columns; protocol-specific extensible attributes JSONB. “Her şeyi JSON blob” yapılmaz.

---

## Karar DA-05 — Graph database

**Durum:** APPROVED

**A — Neo4j/graph DB**
- Natural journey graph.
- New datastore/ops burden.

**B — PostgreSQL relational node/edge tables**
- Yeterli initial graph traversal/query.

**APPROVED DECISION — B.**

Graph DB ancak proven query/performance need olursa.

---

## Karar DA-06 — ClickHouse

**Durum:** APPROVED

**A — Baştan telemetry store**
- Büyük analytics performansı [ClickHouse candidate].
- İkinci DB complexity.

**B — Deferred scale option**
- PostgreSQL initial.

**APPROVED DECISION — B.**

Trigger: partitioned Postgres event queries/retention cost veya high-volume analytics verified bottleneck haline gelirse.

---

## Karar DA-07 — Redis

**Durum:** APPROVED

**APPROVED DECISION — Current architecture'da yok.**
Cache correctness source'u olmayacak. Rate limit/session/job ihtiyaçları önce in-process/Postgres ile çözülür.

---

## Karar DA-08 — Object storage

**Durum:** APPROVED

**APPROVED DECISION:** S3-compatible object interface.

Stores:
- attacker-uploaded files
- long transcripts
- optional raw payloads
- diagnostic bundles
- update artifacts where appropriate

Hosted backend vendor-specific olabilir; on-prem filesystem/S3-compatible backend.

---

## Karar DA-09 — Edge database

**Durum:** APPROVED

**APPROVED DECISION:** SQLite 3 WAL, network filesystem üzerinde değil [S03].

---

## Karar DA-10 — Vector database / pgvector

**Durum:** APPROVED

**A — Baştan vector/RAG**
- AI buzzword architecture.
- Unproven need.

**B — No vector store initially**
- Curated guidance structured knowledge + direct evidence context.

**APPROVED DECISION — B.**

`pgvector` ancak eval'ler retrieval benefit gösterirse.

---

## Karar DA-11 — Full-text search

**Durum:** APPROVED

**APPROVED DECISION:** Initial incident/evidence search PostgreSQL native indexing/full-text capabilities ile. Elasticsearch/OpenSearch yok.

---

## Karar DA-12 — Data migrations

**Durum:** APPROVED

**APPROVED DECISION:** Version-controlled forward migrations, app start'ta uncontrolled auto-DDL yok. Migration compatibility Edge/Control rolling upgrade policy ile test edilir.

---

## Karar DA-13 — Backup architecture

**Durum:** APPROVED

**APPROVED DECISION:**
- PostgreSQL point-in-time capable backup strategy
- object storage version/lifecycle
- config/secrets separate backup
- disposable decoy runtime state backup edilmez
- incident/evidence history recoverable

Specific backup vendor deployment profile'a bağlıdır.

---

# 11. 4.11 — Correlation & Attacker Journey Architecture

## Karar CE-01 — Correlation engine location

**Durum:** APPROVED

**APPROVED DECISION:** Control Plane modular monolith `correlation` module + durable worker.

---

## Karar CE-02 — Correlation data model

**Durum:** APPROVED

**APPROVED DECISION:** Entity/evidence graph:
- source entity
- account
- session
- decoy
- zone
- evidence
- temporal relationship

Physical storage PostgreSQL node/edge tables.

---

## Karar CE-03 — Rule representation

**Durum:** APPROVED

**A — Hard-coded if statements**
**B — General-purpose arbitrary rule DSL**
**C — Typed rule model + versioned declarative rule definitions**

**APPROVED DECISION — C.**

No embedded arbitrary scripting in security rule engine initially.

---

## Karar CE-04 — Rule evaluation modes

**Durum:** APPROVED

**APPROVED DECISION:**
- per-event deterministic rules
- time-window aggregation
- sequence correlation
- entity-link scoring
- incident update rules

---

## Karar CE-05 — Probabilistic inference

**Durum:** APPROVED

**APPROVED DECISION:** Separate hypothesis layer; confidence factors persisted. Probabilistic inference cannot mutate observed evidence.

---

## Karar CE-06 — AI correlation

**Durum:** APPROVED

**APPROVED DECISION:** AI may propose edge/link/incident merge suggestions; deterministic engine/user can accept. AI output `inferred` flag ile.

---

## Karar CE-07 — Reprocessing

**Durum:** APPROVED

**APPROVED DECISION:** Versioned correlation rules permit historical replay in bounded jobs. Original evidence immutable; derived findings/incidents carry engine/rule version.

---

## Karar CE-08 — Incident merge/split

**Durum:** APPROVED

**APPROVED DECISION:** First-class operations with audit trail. User correction does not delete source evidence.

---

## Karar CE-09 — ATT&CK mapping

**Durum:** APPROVED

**APPROVED DECISION:** Versioned enrichment map stored as derived metadata, not primary correlation key.

---

# 12. 4.12 — AI Architecture

## Karar AI-01 — Provider abstraction

**Durum:** APPROVED

**A — Direct provider SDK calls throughout code**
- Fast; lock-in.

**B — Internal `ModelGateway` abstraction**
- Common request/response schema, provider adapters.

**APPROVED DECISION — B.**

Common subset:
- structured JSON output
- system/developer instruction
- text context
- token/budget metadata
- timeout/cancel
- provider/model ID
- optional tool schemas future

---

## Karar AI-02 — Initial provider strategy

**Durum:** APPROVED

**A — OpenAI only**
**B — Anthropic only**
**C — Gemini only**
**D — At least two adapters to validate abstraction**

**APPROVED DECISION — D: OpenAI + Anthropic initial adapters; Gemini adapter next candidate.**

Gerekçe:
- OpenAI Responses API JSON Schema structured outputs [S17].
- Anthropic structured outputs/strict tool schemas [S18].
- İki gerçek provider abstraction'ın common denominator'unu test eder.
- Product vendor lock-in iddiası yalnız interface yazarak değil, ikinci adapter çalıştırarak doğrulanır.

**Karşıt görüş:** İki provider gereksiz scope.  
**Cevap:** Adapter yüzeyi deliberately küçük tutulur; provider-specific agent/tool ecosystem kullanılmaz.

---

## Karar AI-03 — Default model

**Durum:** APPROVED

**APPROVED DECISION:** Model adı architecture constant olmamalı. Deployment config'de capability profile:
- `reasoning_high`
- `reasoning_fast`
- `offline/local_future`

Provider adapter bu profile'ı concrete model ID'ye map eder.

Final default model seçimi release-time operational decision olarak değişebilir.

---

## Karar AI-04 — Structured outputs

**Durum:** APPROVED

**APPROVED DECISION:** JSON Schema validated output mandatory for machine-consumed:
- incident explanation facts
- hypotheses
- severity rationale
- recommended actions
- citations/evidence IDs
- uncertainty

Free-form text sadece rendered narrative.

---

## Karar AI-05 — AI input builder

**Durum:** APPROVED

**APPROVED DECISION:** Model raw DB/query erişimi almaz. Application deterministic **Incident Context Package** üretir.

Contains:
- summarized evidence
- evidence IDs
- source confidence
- timeline
- known environment context
- allowed guidance knowledge
- explicit untrusted attacker fields

---

## Karar AI-06 — Attacker-controlled content treatment

**Durum:** APPROVED

**APPROVED DECISION:** Commands, filenames, HTTP bodies, uploaded text separate `untrusted_observation` fields; length-limited; no instruction role. Prompt explicitly bu content'in command olmadığını belirtir.

---

## Karar AI-07 — Tool calling

**Durum:** APPROVED

**A — AI'a DB/network/containment tools ver**
- Agentic.
- Security risk.

**B — No tools initially; structured reasoning only**
- Minimum agency.

**C — Read-only safe tools only**

**APPROVED DECISION — B first; C after dedicated threat model.**

Containment tool current architecture'da yok.

---

## Karar AI-08 — RAG/knowledge base

**Durum:** APPROVED

**A — Vector DB + documents**
**B — Versioned curated response knowledge as structured content**
**C — External web/search**

**APPROVED DECISION — B.**
Security guidance source content code/review sürecinden geçer. RAG only if evaluation proves need.

---

## Karar AI-09 — AI asynchronous mı synchronous mı?

**Durum:** APPROVED

**APPROVED DECISION:** Incident creation/notification AI'a bloklanmaz. AI explanation async enhancement; UI deterministic incident'ı hemen gösterir.

---

## Karar AI-10 — Caching/dedup

**Durum:** APPROVED

**APPROVED DECISION:** AI result key = incident evidence snapshot hash + prompt/schema version + model profile. Same context unnecessary re-run edilmez.

---

## Karar AI-11 — Budget control

**Durum:** APPROVED

**APPROVED DECISION:** Per-environment/day ve per-incident hard budgets; max context; retry ceiling; attacker-triggered event floods AI call fan-out yaratamaz.

---

## Karar AI-12 — AI eval architecture

**Durum:** APPROVED

**APPROVED DECISION:** Versioned golden incident fixtures:
- factuality
- evidence citation coverage
- unsupported claim rate
- remediation safety
- prompt injection resistance
- schema compliance
- provider/model regression

Prompt/model change CI'da eval gate'e tabi olabilir.

---

## Karar AI-13 — Local model future

**Durum:** APPROVED

**APPROVED DECISION:** Provider contract local OpenAI-compatible veya native adapter eklenmesine izin verir; current architecture GPU/local inference dependency taşımaz.

---

## Karar AI-14 — AI output persistence

**Durum:** APPROVED

**APPROVED DECISION:** Store:
- provider/model
- prompt template version
- input evidence snapshot references
- structured output
- validation result
- timestamps/cost metadata

Private chain-of-thought saklanmaz/istenmez.

---

# 13. 4.13 — Security Architecture

## Karar SA-01 — Trust zone topology

**Durum:** APPROVED

**APPROVED ZONES:**
1. User/Web
2. Control Plane
3. Device management channel
4. Edge management
5. Decoy runtime
6. Customer production network
7. High-interaction containment
8. External AI/integration
9. Artifact/update supply chain

Data flow diagramları bu zone'lar üzerinde hazırlanmalı.

---

## Karar SA-02 — Device PKI

**Durum:** APPROVED

**A — SPIRE**
- Strong workload identity [S15].
- Current topology için server/agent/attestation complexity.

**B — Product-specific X.509 device CA using standard crypto**
- Minimal.
- CA lifecycle bizim sorumluluğumuz.

**C — External enterprise PKI required**
- Customer burden.

**APPROVED DECISION — B.**
SPIFFE/SPIRE future scale/microservice option.

---

## Karar SA-03 — Custom cryptography

**Durum:** APPROVED

**APPROVED DECISION:** Yasak. Standard TLS/X.509/AEAD primitives ve maintained libraries; home-grown cipher/protocol yok.

---

## Karar SA-04 — Management channel encryption

**Durum:** APPROVED

**APPROVED DECISION:** TLS 1.3 preferred, mTLS device authentication. Supported cipher suites platform standardsına bırakılır; obsolete TLS disabled.

---

## Karar SA-05 — Edge key storage

**Durum:** APPROVED

**A — Plain root-readable file**
**B — OS protected file + permission/hardening**
**C — TPM-backed mandatory**

**APPROVED DECISION — B baseline; TPM optional future hardening.**

TPM mandatory yapmak commodity VM compatibility'yi düşürür.

---

## Karar SA-06 — Central CA/master secret storage

**Durum:** APPROVED

**APPROVED DECISION:** SecretStore abstraction:
- hosted: cloud KMS/HSM-backed encryption candidate
- on-prem: protected local master key + encrypted secret records
- raw private keys logs/DB plain text içinde yok

Cloud KMS vendor Step 4 deployment profile tarafından seçilmeyebilir; interface sabit.

---

## Karar SA-07 — Artifact signing

**Durum:** APPROVED

**APPROVED DECISION:** Cosign/Sigstore-compatible signing for OCI decoy images and release artifacts [S13].

Edge verifies signature/digest before activation.

---

## Karar SA-08 — Update metadata

**Durum:** APPROVED

**A — Signature only**
**B — TUF-style signed metadata/version/expiry/rollback protection**

**APPROVED DECISION — B**, Cosign artifact signature ile birlikte.

TUF rollback/freeze/key compromise scenarios için tasarlanmıştır [S14].

---

## Karar SA-09 — Supply-chain SBOM

**Durum:** APPROVED

**APPROVED DECISION:** Her release için SPDX veya CycloneDX SBOM; container/package dependency scan; provenance artifact.

Specific scanner tool CI bölümünde seçilir.

---

## Karar SA-10 — Edge privileged helper hardening

**Durum:** APPROVED

**APPROVED DECISION:**
- Unix socket local only
- peer credential check
- strict typed methods
- no arbitrary path/command
- allowlisted interface/runtime operations
- audit log
- seccomp/capabilities where practical

---

## Karar SA-11 — Control Plane CSRF/XSS/session security

**Durum:** APPROVED

**APPROVED DECISION:** HttpOnly Secure SameSite cookies, CSRF protection where needed, CSP, no tokens in localStorage, output encoding.

---

## Karar SA-12 — Uploaded hostile content

**Durum:** APPROVED

**APPROVED DECISION:** Object store quarantine bucket/prefix; content-disposition download; no inline HTML/SVG execution; AV scanning optional enrichment but product malware sandbox değildir.

---

## Karar SA-13 — Audit log

**Durum:** APPROVED

**APPROVED DECISION — first-class:** user/device/config/update/incident disposition/AI-action approvals. Append-oriented, actor + before/after references.

---

## Karar SA-14 — Security headers/API rate limits

**Durum:** APPROVED

**APPROVED DECISION:** Server-side request limits, auth brute-force throttling, payload max sizes, per-device ingest quotas.

---

## Karar SA-15 — Security testing

**Durum:** APPROVED

**APPROVED DECISION:** Threat-model-derived integration tests:
- decoy escape
- egress bypass
- privilege helper abuse
- cert replay/revocation
- malformed protobuf
- event flood
- malicious upload
- prompt injection
- update rollback/tamper

---

## Karar SA-16 — Container vs VM trust statement

**Durum:** APPROVED

**APPROVED PRODUCT RULE:** OCI container is operational isolation for low/medium interaction; intentionally attacker-owned arbitrary OS execution requires VM-grade high-interaction boundary.

---

# 14. 4.14 — User Identity, Authentication & Authorization

## Karar IA-01 — Current organization model

**Durum:** APPROVED

**APPROVED DECISION:** Single organization/environment ownership model. Multi-tenant hierarchy yok.

---

## Karar IA-02 — Authentication source

**Durum:** APPROVED

**A — External OIDC mandatory**
- Enterprise integration.
- Small org/offline burden.

**B — Built-in local auth only**
- Self-contained.

**C — Built-in local auth baseline + optional OIDC later**

**APPROVED DECISION — C.**

---

## Karar IA-03 — Password storage

**Durum:** APPROVED

**APPROVED DECISION:** Argon2id-class password hashing with per-user salt and centrally configured work factors. Plain/reversible password storage yok.

---

## Karar IA-04 — MFA

**Durum:** APPROVED

**A — Optional**
**B — TOTP mandatory for privileged operator**
**C — Passkeys only**

**APPROVED DECISION — B baseline; WebAuthn/passkeys future preferred enhancement.**

---

## Karar IA-05 — Browser session

**Durum:** APPROVED

**A — JWT localStorage**
**B — Server-side session + secure HttpOnly cookie**

**APPROVED DECISION — B.**

JWT yalnız machine/API cases için gerekirse.

---

## Karar IA-06 — RBAC

**Durum:** APPROVED

Step 2 current full RBAC'ı scope dışına aldı.

**APPROVED DECISION:** Fine-grained configurable RBAC yazılmaz. Internal authorization model en az:
- authenticated operator
- owner/security-sensitive actions
ayrımını destekleyecek hook taşıyabilir.

---

## Karar IA-07 — API tokens

**Durum:** APPROVED

**APPROVED DECISION:** Future integrations için scoped revocable service tokens; current UI auth ile aynı credential kullanılmaz.

---

## Karar IA-08 — Device identity vs user identity

**Durum:** APPROVED

**APPROVED DECISION:** Tamamen ayrı credential stores/lifecycles. Edge device cert bir human user credential değildir.

---

# 15. 4.15 — Update & Lifecycle Architecture

## Karar UL-01 — Distribution channel

**Durum:** APPROVED

**APPROVED DECISION:**
- Edge binary/package release artifact
- Decoy packs OCI registry
- Control Plane OCI images
- signed update metadata

---

## Karar UL-02 — Edge self-update

**Durum:** APPROVED

**A — Edge daemon kendini overwrite eder**
**B — Separate updater/helper**
**C — OS package manager only**

**APPROVED DECISION — B.**
Updater small privileged component verifies metadata/signature, stages artifact, atomic switch/restart, health-check, rollback.

---

## Karar UL-03 — Decoy update

**Durum:** APPROVED

**APPROVED DECISION:** Pull signed OCI image by immutable digest; desired-state references version/digest; activation after verify.

---

## Karar UL-04 — Automatic update policy

**Durum:** APPROVED

**A — Always force latest**
**B — Manual only**
**C — Managed channels: stable / staged, owner policy + security override semantics**

**APPROVED DECISION — C.**

---

## Karar UL-05 — Rollback

**Durum:** APPROVED

**APPROVED DECISION:** Previous known-good Edge binary/config and decoy digest retained. Health failure can rollback. Security-revoked versions block reinstall.

---

## Karar UL-06 — Offline updates

**Durum:** APPROVED

**APPROVED DECISION:** Signed offline bundle format future-compatible; import aynı verification path'ten geçer.

---

## Karar UL-07 — Schema migration during rolling update

**Durum:** APPROVED

**APPROVED DECISION:** Expand/contract-compatible DB/API migrations; Control Plane must not instantly require only newest Edge protocol.

---

## Karar UL-08 — Update provenance UI

**Durum:** APPROVED

**APPROVED DECISION:** Operator version, channel, digest, signature verification, last update/result görebilmeli.

---

# 16. 4.16 — Observability Architecture

## Karar OB-01 — Instrumentation standard

**Durum:** APPROVED

**APPROVED DECISION:** OpenTelemetry SDK semantic conventions [S10].

---

## Karar OB-02 — Signals

**Durum:** APPROVED

**APPROVED DECISION:**
- structured logs
- metrics
- traces for Control Plane/device RPCs
- audit/security events ayrı domain data

Product evidence logs ile application observability logs karıştırılmaz.

---

## Karar OB-03 — Collector

**Durum:** APPROVED

**APPROVED DECISION:** OTel Collector optional/standard aggregation boundary. Product core correctness Collector availability'ine bağlı olmaz.

---

## Karar OB-04 — Metrics backend

**Durum:** APPROVED

**APPROVED DECISION:** Prometheus [S11] for hosted/dev operational metrics.

Customer-facing health Control Plane domain modelinden gelir; customer Grafana öğrenmek zorunda değildir.

---

## Karar OB-05 — Logs backend

**Durum:** APPROVED

**APPROVED DECISION:** Hosted/dev için Loki candidate [S12]. On-prem minimal profile structured local logs + diagnostic export ile çalışabilmeli.

---

## Karar OB-06 — Dashboarding

**Durum:** APPROVED

**APPROVED DECISION:** Grafana internal engineering/ops dashboards. Product UI içine Grafana embed etmek primary UX değildir.

---

## Karar OB-07 — Trace backend

**Durum:** APPROVED

**APPROVED DECISION:** OTel-export-compatible, backend deployment-specific. Tempo/Jaeger gibi backend seçimi core product dependency değildir.

---

## Karar OB-08 — Edge observability transport

**Durum:** APPROVED

**APPROVED DECISION:** Critical product health Edge control channel üzerinden normalized status olarak gelir. Full debug OTel/log data opt-in diagnostics olabilir.

---

## Karar OB-09 — SLO/alerts

**Durum:** APPROVED

**APPROVED future measurable architecture metrics:**
- edge connected
- queue age/size
- ingest failures
- decoy reachability
- config convergence
- event processing latency
- AI failure/rate/budget
- notification delivery

Exact thresholds MVP acceptance aşamasında.

---

# 17. 4.17 — Concrete Technology Selection

Bu bölüm yukarıdaki architecture kararlarından türeyen technology baseline'ını tek yerde karşılaştırır.

## Karar TS-01 — Primary backend/system language

**Durum:** APPROVED

**Options:** Go / Rust / Python / Java/Kotlin / C#  
**APPROVED DECISION:** **Go 1.27.0** for Control Plane, Edge Agent, HI orchestration control code.

Python yalnız integrated OSS decoy runtime'larında veya narrow tooling'de kabul edilir.

---

## Karar TS-02 — Frontend language/framework

**Durum:** APPROVED

**A — React + TypeScript**
**B — Vue**
**C — Svelte**
**D — server-rendered Go templates**

**APPROVED DECISION:** **React + TypeScript** [S16].

Security console state-heavy UI, timeline/graph/config forms için ecosystem geniştir.

---

## Karar TS-03 — Frontend build tool

**Durum:** APPROVED

**APPROVED DECISION:** **Vite** [S16].

SSR framework (Next.js vb.) default değil; product console static SPA.

---

## Karar TS-04 — Frontend data fetching/state

**Durum:** APPROVED

**APPROVED DECISION:** Server-state için TanStack Query sınıfı query/cache library; local UI state React primitives. Global Redux-like store yalnız kanıtlanmış cross-cutting need varsa.

---

## Karar TS-05 — Frontend routing

**Durum:** APPROVED

**APPROVED DECISION:** React Router-class client routing; route tree explicit. Specific major version lock release setup'ta.

---

## Karar TS-06 — API IDL

**Durum:** APPROVED

**APPROVED DECISION:** Protobuf + Buf for device/internal typed contracts [S05]; OpenAPI 3.x for REST/public web API.

---

## Karar TS-07 — Device RPC

**Durum:** APPROVED

**APPROVED DECISION:** gRPC over HTTP/2 with mTLS [S04].

---

## Karar TS-08 — Central database

**Durum:** APPROVED

**APPROVED DECISION:** PostgreSQL 18.x [S02].

---

## Karar TS-09 — Edge database

**Durum:** APPROVED

**APPROVED DECISION:** SQLite 3 WAL [S03].

---

## Karar TS-10 — Message broker

**Durum:** APPROVED

**APPROVED DECISION:** **None initially.**
PostgreSQL transactional outbox/jobs + direct gRPC ingestion.
First scale candidate NATS JetStream [S23].
Kafka not baseline [S24].

---

## Karar TS-11 — Container runtime

**Durum:** APPROVED

**APPROVED DECISION:** containerd 2.x + OCI/runc [S06].

---

## Karar TS-12 — High-interaction virtualization

**Durum:** APPROVED

**APPROVED DECISION:** QEMU/KVM + libvirt + qcow2 [S07].

---

## Karar TS-13 — Edge OS baseline

**Durum:** APPROVED

**APPROVED DECISION:** Debian 13 stable appliance [S25].

---

## Karar TS-14 — Network policy

**Durum:** APPROVED

**APPROVED DECISION:** Linux netns/netlink/iproute2 + nftables [S09].

---

## Karar TS-15 — Artifact signing/update security

**Durum:** APPROVED

**APPROVED DECISION:** Cosign/Sigstore signatures + TUF-style update metadata [S13][S14].

---

## Karar TS-16 — Observability stack

**Durum:** APPROVED

**APPROVED BASELINE:**
- OpenTelemetry instrumentation/Collector
- Prometheus metrics
- Grafana dashboards
- Loki logs in hosted/dev ops profile

No product correctness dependency on these backends.

---

## Karar TS-17 — AI provider layer

**Durum:** APPROVED

**APPROVED DECISION:** Provider-neutral Go interface; initial adapters OpenAI Responses + Anthropic Messages; JSON Schema outputs. Gemini adapter candidate [S17][S18][S19].

---

## Karar TS-18 — Object storage

**Durum:** APPROVED

**APPROVED DECISION:** S3-compatible API boundary; hosted vendor/on-prem implementation deployment-specific.

---

# 18. 4.18 — Repository, CI/CD & Agentic Engineering Architecture

## Karar RE-01 — Monorepo vs multirepo

**Durum:** APPROVED

**A — Multirepo per component**
- Independent permissions/releases.
- Contract changes coordination zor.

**B — Monorepo**
- Atomic contract/schema changes.
- AI agents için tek source of truth.

**APPROVED DECISION — B.**

---

## Karar RE-02 — Proposed repository layout

**Durum:** APPROVED

**APPROVED BASELINE:**

```text
/
├── apps/
│   ├── control-plane/
│   ├── web-console/
│   ├── edge-agent/
│   └── hi-worker/              # future/optional
├── decoys/
│   ├── opencanary-pack/
│   ├── cowrie-pack/
│   └── custom/
├── proto/
├── openapi/
├── schemas/
├── pkg/                        # narrowly shared Go packages
├── ai/
│   ├── prompts/
│   ├── schemas/
│   └── evals/
├── rules/
│   ├── detection/
│   └── correlation/
├── deploy/
│   ├── control-plane/
│   ├── edge-appliance/
│   └── lab/
├── security/
│   ├── threat-models/
│   ├── policies/
│   └── test-fixtures/
├── tests/
│   ├── integration/
│   ├── network-lab/
│   └── attack-scenarios/
├── docs/
│   ├── adr/
│   ├── product/
│   └── runbooks/
└── tools/
```

---

## Karar RE-03 — Shared library policy

**Durum:** APPROVED

**APPROVED DECISION:** `pkg/` dumping ground olmamalı. Shared package yalnız iki+ component'te gerçek stable semantic varsa. Domain logic component sınırını delmemeli.

---

## Karar RE-04 — Architecture Decision Records

**Durum:** APPROVED

**APPROVED DECISION:** Her önemli architecture/technology kararı `docs/adr/ADR-xxxx-*.md`.

AI coding agent bir ADR'yi kendi başına APPROVED yapamaz.

---

## Karar RE-05 — Contract-first workflow

**Durum:** APPROVED

**APPROVED DECISION:** Protobuf/OpenAPI/schema değişiklikleri implementation'dan önce review edilir. Buf breaking CI gate [S05].

---

## Karar RE-06 — CI platform

**Durum:** APPROVED

**APPROVED DECISION:** GitHub Actions başlangıç CI.

Pipelines:
- Go lint/test
- frontend lint/typecheck/test
- proto lint/breaking
- integration tests
- container build
- SBOM
- vulnerability scan
- signatures on release
- AI evals
- network/security lab tests scheduled/privileged runner

---

## Karar RE-07 — Build artifacts

**Durum:** APPROVED

**APPROVED DECISION:**
- reproducible-ish Go binaries
- OCI images by digest
- edge Debian package/appliance artifact
- SBOM/provenance
- signed checksums

---

## Karar RE-08 — Go dependency/database style

**Durum:** APPROVED

**A — Heavy ORM**
**B — Explicit SQL + type-safe generated access**
**C — raw database/sql everywhere**

**APPROVED DECISION — B direction:** `pgx` + `sqlc`-class generated queries.  
Gerekçe: schema/query visibility, predictable performance, AI agents için explicit SQL review.

Specific libraries final owner approval bu document üzerinden verilebilir.

---

## Karar RE-09 — Migrations tool

**Durum:** APPROVED

**APPROVED DECISION:** Simple SQL migration tool (Goose/Atlas-class). Migration files human-reviewable SQL olmalı; ORM auto-migration yok.

---

## Karar RE-10 — Frontend API code generation

**Durum:** APPROVED

**APPROVED DECISION:** OpenAPI-generated typed client veya thin handwritten typed wrapper; API DTO'larını elle duplicate etmemek.

---

## Karar RE-11 — Testing pyramid

**Durum:** APPROVED

**APPROVED DECISION:**
1. unit tests — deterministic domain rules
2. contract tests — Protobuf/OpenAPI
3. integration — PostgreSQL/SQLite/containerd
4. network namespace lab
5. decoy protocol tests
6. attack scenario replay
7. high-interaction isolation tests
8. end-to-end browser tests
9. AI evals

---

## Karar RE-12 — Security regression fixtures

**Durum:** APPROVED

**APPROVED DECISION:** Her security bug/escape/prompt injection bir permanent fixture/test üretmeli.

---

## Karar RE-13 — AI coding agent permissions

**Durum:** APPROVED

**APPROVED ARCHITECTURE GOVERNANCE:**
AI agent:
- code yazabilir
- test yazabilir
- docs/draft ADR hazırlayabilir
- dependency update PR hazırlayabilir
- benchmark çalıştırabilir

AI agent **yapamaz**:
- product decision APPROVE
- architecture decision APPROVE
- security boundary değiştirme
- new external dependency'i silent introduce etme
- schema breaking change'i owner approval olmadan merge etme
- production secret/access kullanma
- release signing key erişimi
- high-risk deployment action

---

## Karar RE-14 — Agent change manifest

**Durum:** APPROVED

**APPROVED DECISION:** Her non-trivial AI-generated PR şu metadata'yı taşımalı:
- requirement/decision IDs
- modified components
- new dependencies
- security impact
- migrations/contracts
- tests executed
- unresolved assumptions

Bu automation PR template/check olarak enforce edilebilir.

---

## Karar RE-15 — Dependency policy

**Durum:** APPROVED

**APPROVED DECISION:**
- dependency allowlist/review
- licenses inventory
- pinned lockfiles/checksums
- minimal transitive deps
- automated vulnerability update PRs
- new runtime dependency requires explicit review

---

## Karar RE-16 — Release branches/versioning

**Durum:** APPROVED

**APPROVED DECISION:** Trunk/main development + tagged semantic product releases. Edge/Control protocol compatibility ayrı version fields ile; monorepo olduğu için bütün component'ların versiyonu mutlaka aynı olmak zorunda değil.

---

## Karar RE-17 — Feature flags

**Durum:** APPROVED

**APPROVED DECISION:** Risky/experimental capabilities (high interaction, AI provider, active discovery) explicit server-side/environment feature flags. Flag security authorization substitute değildir.

---

## Karar RE-18 — Dev/lab environment

**Durum:** APPROVED

**APPROVED DECISION:** Reproducible local lab:
- Control Plane containers
- disposable Edge VM
- virtual networks
- decoy packs
- attack simulation hosts

Real corporate/customer network test için zorunlu olmamalı.

---

# 19. Approved Software Inventory

Step 4 sonucunda onaylanan yazılım sınırları:

| # | Software / Artifact | Current/Future | Language/runtime recommendation | Deployment |
|---|---|---|---|---|
| 1 | Control Plane | Current | Go | OCI container(s) |
| 2 | Web Console | Current | TypeScript + React + Vite | Static artifact, Control Plane/reverse proxy serves |
| 3 | Edge Agent | Current | Go | Native Debian appliance service |
| 4 | Decoy Runtime Packs | Current ecosystem | Mixed; OCI contract | containerd/OCI on Edge |
| 5 | High-Interaction Worker | Future/Advanced | Go orchestrator + libvirt/QEMU/KVM | Dedicated Linux node |
| 6 | Endpoint Deception Agent | Future | Likely Go; Windows/Linux specific helpers possible | Endpoint native service |

**AI Reasoning, Correlation ve Notifications başlangıçta ayrı deployable değildir; Control Plane modülleridir.**

Dolayısıyla:

> **Approved current architecture: 3 ana application + decoy runtime ecosystem.**  
> **Approved long-term architecture direction: High-Interaction Worker ve Endpoint Agent eklendikçe yaklaşık 5–6 ana deployable application + decoy/plugin ecosystem.**

Bu sayı microservice sayısı değildir; Step 4 owner decision ile onaylanmış component sınırını ifade eder.

---

# 20. Approved Baseline Technology Matrix

| Area | Approved baseline | Not selected initially | Gerekçe |
|---|---|---|---|
| Control Plane | Go 1.27.0 | Python monolith, Java microservices | Simple compiled network service |
| Edge | Go 1.27.0 native daemon | Privileged Docker container | Host/network/runtime control |
| Web | React + TypeScript + Vite | SSR framework | Authenticated SPA |
| Device protocol | Protobuf + gRPC + mTLS | JSON polling only | Typed bidirectional device channel |
| Public/Web API | REST/JSON + OpenAPI | GraphQL-first | Integration simplicity |
| Central DB | PostgreSQL 18 | MongoDB, ClickHouse-only | Relational + JSONB + partitioning |
| Edge DB | SQLite WAL | Embedded Postgres | Local durable state |
| Broker | None initially | Kafka | Avoid premature distributed infra |
| Future broker | NATS JetStream | Kafka unless proven scale | Lower operational footprint |
| Raw blobs | S3-compatible | DB bytea for all files | Quarantine/retention separation |
| Decoys | OCI packs | In-process plugins | Isolation/versioning |
| OCI runtime | containerd + runc | Kubernetes requirement | Embedded appliance fit |
| HI virtualization | QEMU/KVM + libvirt | Container; Firecracker-only | Windows + snapshots + broad VM |
| Edge OS | Debian 13 | Arbitrary distro matrix | Predictable appliance |
| Network | netns + nftables | Custom kernel module | Standard Linux primitives |
| AI | Provider abstraction; OpenAI + Anthropic adapters | Provider SDK scattered | Portability/testability |
| AI state | Structured JSON schemas | Free-form automation | Validate machine output |
| Observability | OTel + Prometheus/Grafana/Loki profile | Vendor-specific instrumentation | Portable diagnostics |
| Signing | Cosign + TUF metadata | Unsigned auto-update | Supply-chain resilience |
| Repo | Monorepo | Early multirepo | Atomic contract changes |
| CI | GitHub Actions | Bespoke CI | Simple and sufficient |

---

# 21. Major Counter-Proposals

Adım 4 review sırasında özellikle aşağıdaki karşıt architecture tezleri owner tarafından istenirse seçilebilir.

## Counter-Proposal A — Rust-heavy architecture

Control Plane ve Edge Rust olur.

**Lehte:**
- Memory safety
- tight resource control
- security perception

**Aleyhte:**
- async/ecosystem complexity
- protocol/UI/business logic development friction
- mixed OSS honeypot integrations yine Python/container boundary ister
- AI agents compile errors ile yönlendirilebilir ama lifetime/async complexity review burden yaratabilir

**APPROVED BASELINE:** Go.

---

## Counter-Proposal B — Kubernetes-first

Bütün control ve decoy workloads K8s üzerinde.

**Lehte:** orchestration/scale/health ecosystem.

**Aleyhte:**
- Primary customer Kubernetes kullanmayabilir
- L2 deception/host networking karmaşıklaşır
- Edge appliance'ın kendi K8s cluster'ını işletmek ürün complexity'sini dramatik artırır

**APPROVED BASELINE:** Kubernetes yok; OCI/containerd directly.

---

## Counter-Proposal C — Kafka + ClickHouse from day one

**Lehte:** cybersecurity telemetry scale için güçlü.

**Aleyhte:** current expected event volume doğrulanmadan iki distributed data infrastructure operational burden.

**APPROVED BASELINE:** PostgreSQL + edge buffer; scale trigger sonrası NATS/ClickHouse.

---

## Counter-Proposal D — Cloud-native SaaS-only

**Lehte:** fastest operations, AI integration.

**Aleyhte:** future offline/on-prem ve sensitive telemetry policy zorlaşır.

**APPROVED BASELINE:** portable deployable Control Plane.

---

## Counter-Proposal E — T-Pot/OpenCanary fork as product core

**Lehte:** Çok hızlı honeypot breadth.

**Aleyhte:** product architecture, evidence model, upgrade lifecycle ve UX upstream assumptions'a bağlanır.

**APPROVED BASELINE:** selective adapter integration.

---

## Counter-Proposal F — Full high-interaction differentiation first

**Lehte:** attacker engagement/intel differentiation.

**Aleyhte:** containment, VM lifecycle, Windows licensing ve malware risk product core'u domine eder.

**APPROVED BASELINE:** advanced separate worker.

---

# 22. Intentionally Deferred Release / Configuration Decisions

Aşağıdaki ayrıntılar Step 4 architecture kararlarının parçası olarak açık bırakılmamıştır; bunlar bilinçli olarak release/configuration/operational tuning seviyesine bırakılmıştır ve Step 4'te açık owner decision sayılmaz:

- Concrete LLM model names
- Cloud vendor account/region
- Exact PostgreSQL instance size
- Exact retention days
- Exact CPU/memory defaults
- Exact TLS certificate lifetime
- Exact AI token limits
- Exact alert timeout
- Exact vulnerability scanner vendor in CI
- Exact UI component library/design system
- Concrete high-interaction Windows SKU/license agreement

Ancak bunların **decision owner**, constraint ve seçilme zamanı final belgede açık olmalıdır.

---

# 23. Validation / Architecture Spike Backlog — OPEN BY DESIGN

Bazı APPROVED architecture kararları implementation öncesi focused spike/test ile doğrulanmalıdır. Bu backlog'un açık olması architecture decision açığı anlamına gelmez.

## Network spikes
- Routed secondary-IP mode across VMware/Hyper-V/Proxmox/KVM
- macvlan switch port-security compatibility
- ipvlan realism/ARP behavior
- VLAN trunk placement
- same-host management ↔ decoy namespace connectivity
- nftables egress bypass tests

## Runtime spikes
- containerd direct Go API lifecycle
- rootless vs privileged runtime feasibility
- third-party decoy logs → canonical UDS/protobuf adapter
- Cowrie isolation/egress/file capture
- OpenCanary module/port behavior

## High-interaction spikes
- libvirt external snapshot/reset speed and failure handling
- nested virtualization constraints
- Windows guest telemetry collection without production domain join
- malware upload quarantine channel
- no-route-to-production enforcement

## Data spikes
- PostgreSQL partitioned event ingestion benchmark
- JSONB vs normalized protocol attribute queries
- correlation graph queries on relational tables
- edge SQLite crash/replay/buffer pressure
- S3/local object backend abstraction

## AI spikes
- identical schema across OpenAI/Anthropic
- evidence citation fidelity
- prompt injection test corpus
- context minimization/redaction
- async AI failure/degraded UX
- provider fallback behavior

## Update/security spikes
- TUF metadata flow + Cosign OCI verification
- atomic Edge updater/rollback
- offline bundle verification
- device certificate enrollment/rotation/revocation
- privileged helper IPC abuse tests

---

# 24. Approved Architecture Risk Baseline

| Risk | Architecture source | Potential effect | Recommended direction |
|---|---|---|---|
| Control Plane becomes hidden microservice monolith | poor module boundaries | maintenance | domain modules + ADRs |
| Edge root daemon compromise | broad privilege | customer network compromise | privileged helper separation |
| Container escape | attacker-facing runtime | host compromise | low/medium only; hardening |
| High-interaction escape/pivot | real compromised VM | critical | dedicated worker + KVM isolation + egress deny |
| L2 deployment incompatibility | switch/VLAN rules | coverage failure | presence drivers + validation |
| Telemetry loss | outage/backpressure | missed incidents | SQLite spool + at-least-once |
| Duplicate evidence | retries | noisy incidents | stable IDs/idempotency |
| PostgreSQL event growth | scan flood | query/storage pressure | partitions + retention + scale trigger |
| Broker overengineering | Kafka/NATS too early | ops burden | no broker baseline |
| AI vendor lock | provider-specific APIs | portability loss | common structured adapter |
| AI prompt injection | attacker telemetry | false guidance/action | no tools + untrusted data boundary |
| AI cost attack | attacker flood | denial-of-wallet | async gating/budgets/cache |
| Update compromise | supply chain | fleet compromise | Cosign + TUF |
| Windows licensing | VM distribution | commercial/legal block | customer-provided image initially |
| OSS upstream vulnerability | decoy dependency | attacker escape | pin/sign/patch monitor |
| Monorepo agent blast radius | AI changes many components | unsafe PR | ownership/checks/contract gates |
| Schema breaking change | Edge/Control versions | fleet outage | Buf breaking + compatibility |
| Observability becomes product dependency | Grafana stack outage | core outage | OTel optional backend |
| On-prem divergence | separate code paths | maintenance | same artifact, deployment profiles |

---

# 25. Consolidated APPROVED Decision Register

Aşağıdaki tablo Step 4 kapsamındaki bütün architecture ve technology kararlarını tek yerde gösterir.

| ID | Decision | Approved direction | Status |
|---|---|---|---|
| SD-01 | Service decomposition | Modular monolith + real distributed boundaries | APPROVED |
| SD-02 | Software count | 3 core apps + decoy ecosystem | APPROVED |
| SD-03 | Endpoint agent | Interface reserved, implementation deferred | APPROVED |
| SD-04 | HI worker | Separate deployable | APPROVED |
| SD-05 | AI service | Control Plane module | APPROVED |
| SD-06 | Correlation service | Control Plane module | APPROVED |
| SD-07 | Notification service | Control Plane module | APPROVED |
| SD-08 | Decoy execution | OCI/runtime boundary | APPROVED |
| DT-01 | Edge form | Dedicated VM/appliance | APPROVED |
| DT-02 | Edge OS | Debian 13 stable | APPROVED |
| DT-03 | CPU arch | amd64 primary | APPROVED |
| DT-04 | Control deployment | Cloud/on-prem same artifact | APPROVED |
| DT-05 | Control runtime | OCI/Compose, no Kubernetes | APPROVED |
| DT-06 | Cloud lock-in | Portable abstractions | APPROVED |
| DT-07 | Web deployment | Static SPA | APPROVED |
| DT-08 | Tenant model | Single-organization instance | APPROVED |
| CP-01 | Control language | Go 1.27.0 | APPROVED |
| CP-02 | Internal architecture | Domain modular monolith | APPROVED |
| CP-03 | HTTP framework | stdlib/minimal | APPROVED |
| CP-04 | Web/public API | REST/OpenAPI | APPROVED |
| CP-05 | Jobs | in-process durable workers | APPROVED |
| CP-06 | App state | DB/object-store durable | APPROVED |
| CP-07 | Scale | single-node capable, scale-safe | APPROVED |
| CP-08 | SSR | no | APPROVED |
| EN-01 | Edge language | Go 1.27.0 | APPROVED |
| EN-02 | Edge process | native systemd service | APPROVED |
| EN-03 | Privilege | main + narrow privileged helper | APPROVED |
| EN-04 | Helper API | typed UDS RPC | APPROVED |
| EN-05 | Local state | SQLite WAL | APPROVED |
| EN-06 | Blob spool | filesystem CAS + SQLite metadata | APPROVED |
| EN-07 | Config | desired-state reconciler | APPROVED |
| EN-08 | Offline | last-good + spool | APPROVED |
| EN-09 | Local detection | deterministic basic layer | APPROVED |
| EN-10 | Plugins | process/OCI contracts | APPROVED |
| EN-11 | Discovery | Edge executor | APPROVED |
| EN-12 | Local UI | diagnostics only | APPROVED |
| DR-01 | Decoy runtime | OCI containers | APPROVED |
| DR-02 | Container engine | containerd+runc | APPROVED |
| DR-03 | Pack format | signed OCI + manifest | APPROVED |
| DR-04 | Telemetry adapter | typed UDS + log adapter | APPROVED |
| DR-05 | Config | versioned declarative | APPROVED |
| DR-06 | OSS | selective adapter integration | APPROVED |
| DR-07 | OpenCanary | optional low-interaction pack | APPROVED |
| DR-08 | Cowrie | SSH/Telnet integration candidate | APPROVED |
| DR-09 | Resource limits | cgroups/quotas | APPROVED |
| DR-10 | Hardening | least privilege profiles | APPROVED |
| HI-ARCH-01 | VMM | QEMU/KVM | APPROVED |
| HI-ARCH-02 | VM manager | libvirt | APPROVED |
| HI-ARCH-03 | Placement | dedicated optional worker | APPROVED |
| HI-ARCH-04 | Disks | golden + qcow2 overlay | APPROVED |
| HI-ARCH-05 | Windows images | customer licensed source | APPROVED |
| HI-ARCH-06 | Linux images | product templates | APPROVED |
| HI-ARCH-07 | Egress | isolated default-deny | APPROVED |
| HI-ARCH-08 | Malware capture | out-of-band quarantine | APPROVED |
| HI-ARCH-09 | Firecracker | future Linux optimization | APPROVED |
| NW-01 | Network primitives | netns/netlink+nftables | APPROVED |
| NW-02 | Presence drivers | routed/macvlan/ipvlan | APPROVED |
| NW-03 | Default presence | routed default | APPROVED |
| NW-04 | VLAN | supported | APPROVED |
| NW-05 | Multi-NIC | supported | APPROVED |
| NW-06 | IP allocation | static/reserved baseline | APPROVED |
| NW-07 | Conflict detection | mandatory | APPROVED |
| NW-08 | IPv6 | contracts dual-stack ready | APPROVED |
| NW-09 | Egress | default deny | APPROVED |
| NW-10 | Management | outbound initiated | APPROVED |
| NW-11 | Overlay VPN | not mandatory | APPROVED |
| NW-12 | Packet capture | optional enrichment | APPROVED |
| NW-13 | DNS writes | no default prod write | APPROVED |
| NW-14 | Policy audit | desired-state traceable | APPROVED |
| CM-01 | Edge protocol | gRPC bidirectional | APPROVED |
| CM-02 | Direction | Edge outbound | APPROVED |
| CM-03 | Device auth | mTLS | APPROVED |
| CM-04 | Enrollment | one-time token + CSR | APPROVED |
| CM-05 | Certs | rotation/revocation | APPROVED |
| CM-06 | Schema | Protobuf+Buf | APPROVED |
| CM-07 | Delivery | at-least-once + idempotency | APPROVED |
| CM-08 | Config sync | revisioned desired-state | APPROVED |
| CM-09 | Blobs | separate chunked upload | APPROVED |
| CM-10 | Compatibility | rolling/backward compatible | APPROVED |
| TP-01 | Serialization | Protobuf canonical | APPROVED |
| TP-02 | IDs | time-sortable global IDs | APPROVED |
| TP-03 | Time | event/observed/ingested | APPROVED |
| TP-04 | Normalize | Edge | APPROVED |
| TP-05 | Batch | bounded/backpressure | APPROVED |
| TP-06 | Broker | none initially | APPROVED |
| TP-07 | Semantics | idempotent at-least-once | APPROVED |
| TP-08 | Retention | DB + object split | APPROVED |
| TP-09 | Backpressure | Edge spool | APPROVED |
| TP-10 | Provenance | device/runtime/schema metadata | APPROVED |
| DA-01 | Central DB | PostgreSQL 18 | APPROVED |
| DA-02 | DB policy | current+previous supported | APPROVED |
| DA-03 | Partitions | time partition raw events | APPROVED |
| DA-04 | JSONB | extensible attributes only | APPROVED |
| DA-05 | Graph DB | no; relational graph | APPROVED |
| DA-06 | ClickHouse | deferred scale option | APPROVED |
| DA-07 | Redis | no initial | APPROVED |
| DA-08 | Object | S3-compatible | APPROVED |
| DA-09 | Edge DB | SQLite | APPROVED |
| DA-10 | Vector DB | no initial | APPROVED |
| DA-11 | Search | PostgreSQL native | APPROVED |
| DA-12 | Migrations | explicit versioned SQL | APPROVED |
| DA-13 | Backup | state-class specific | APPROVED |
| CE-01 | Correlation | Control module | APPROVED |
| CE-02 | Model | entity/evidence graph | APPROVED |
| CE-03 | Rules | typed declarative | APPROVED |
| CE-04 | Modes | event/window/sequence/entity | APPROVED |
| CE-05 | Probabilistic | hypothesis layer | APPROVED |
| CE-06 | AI | suggestion only | APPROVED |
| CE-07 | Replay | versioned reprocessing | APPROVED |
| CE-08 | Merge/split | audited first-class | APPROVED |
| CE-09 | ATT&CK | secondary enrichment | APPROVED |
| AI-01 | Gateway | internal abstraction | APPROVED |
| AI-02 | Providers | OpenAI+Anthropic first | APPROVED |
| AI-03 | Model IDs | capability profile mapping | APPROVED |
| AI-04 | Output | JSON Schema | APPROVED |
| AI-05 | Context | deterministic package | APPROVED |
| AI-06 | Untrusted data | explicit isolation | APPROVED |
| AI-07 | Tools | none initially | APPROVED |
| AI-08 | Knowledge | curated structured | APPROVED |
| AI-09 | Execution | async enhancement | APPROVED |
| AI-10 | Cache | context/version hash | APPROVED |
| AI-11 | Budget | hard limits | APPROVED |
| AI-12 | Evals | golden regression suite | APPROVED |
| AI-13 | Local model | adapter-ready | APPROVED |
| AI-14 | Persistence | model/prompt/evidence provenance | APPROVED |
| SA-01 | Trust zones | explicit | APPROVED |
| SA-02 | Device PKI | product X.509 CA | APPROVED |
| SA-03 | Crypto | standard only | APPROVED |
| SA-04 | Transport | TLS/mTLS | APPROVED |
| SA-05 | Edge keys | protected file baseline | APPROVED |
| SA-06 | Secrets | SecretStore abstraction | APPROVED |
| SA-07 | Signing | Cosign | APPROVED |
| SA-08 | Updates | TUF metadata | APPROVED |
| SA-09 | SBOM | mandatory release artifact | APPROVED |
| SA-10 | priv-helper | strict local API | APPROVED |
| SA-11 | browser security | secure session/CSP/CSRF | APPROVED |
| SA-12 | hostile files | quarantine/no inline exec | APPROVED |
| SA-13 | audit | first-class | APPROVED |
| SA-14 | rate limits | mandatory | APPROVED |
| SA-15 | security tests | threat-model suite | APPROVED |
| SA-16 | container trust | no high-interaction arbitrary OS | APPROVED |
| IA-01 | Org model | single org | APPROVED |
| IA-02 | Auth | local + future OIDC | APPROVED |
| IA-03 | Password | Argon2id | APPROVED |
| IA-04 | MFA | TOTP admin baseline | APPROVED |
| IA-05 | Session | server-side cookie | APPROVED |
| IA-06 | RBAC | no full configurable RBAC | APPROVED |
| IA-07 | API tokens | scoped future | APPROVED |
| IA-08 | Device/user identity | separate | APPROVED |
| UL-01 | Distribution | package/OCI/signed metadata | APPROVED |
| UL-02 | Edge updater | separate helper | APPROVED |
| UL-03 | Decoy update | digest-pinned OCI | APPROVED |
| UL-04 | Policy | staged channels | APPROVED |
| UL-05 | Rollback | known-good | APPROVED |
| UL-06 | Offline | signed bundle future | APPROVED |
| UL-07 | Schema rollout | expand/contract | APPROVED |
| UL-08 | UI provenance | visible | APPROVED |
| OB-01 | Instrumentation | OpenTelemetry | APPROVED |
| OB-02 | Signals | logs/metrics/traces | APPROVED |
| OB-03 | Collector | OTel Collector | APPROVED |
| OB-04 | Metrics | Prometheus | APPROVED |
| OB-05 | Logs | Loki hosted/dev | APPROVED |
| OB-06 | Dashboard | Grafana ops only | APPROVED |
| OB-07 | Traces | backend-agnostic OTLP | APPROVED |
| OB-08 | Edge health | product channel | APPROVED |
| OB-09 | SLO metrics | defined, thresholds later | APPROVED |
| TS-01 | Primary language | Go | APPROVED |
| TS-02 | Frontend | React+TS | APPROVED |
| TS-03 | Build | Vite | APPROVED |
| TS-04 | Server state | TanStack Query class | APPROVED |
| TS-05 | Router | React Router class | APPROVED |
| TS-06 | IDL | Protobuf+OpenAPI | APPROVED |
| TS-07 | RPC | gRPC | APPROVED |
| TS-08 | DB | PostgreSQL | APPROVED |
| TS-09 | Edge DB | SQLite | APPROVED |
| TS-10 | Broker | none | APPROVED |
| TS-11 | Container | containerd+runc | APPROVED |
| TS-12 | VM | QEMU/KVM+libvirt | APPROVED |
| TS-13 | OS | Debian 13 | APPROVED |
| TS-14 | Network | netns+nftables | APPROVED |
| TS-15 | Signing | Cosign+TUF | APPROVED |
| TS-16 | Observability | OTel/Prom/Grafana/Loki | APPROVED |
| TS-17 | AI | provider-neutral, 2 adapters | APPROVED |
| TS-18 | Objects | S3-compatible | APPROVED |
| RE-01 | Repos | monorepo | APPROVED |
| RE-02 | Layout | apps/decoys/contracts/etc. | APPROVED |
| RE-03 | Shared code | strict policy | APPROVED |
| RE-04 | ADRs | mandatory | APPROVED |
| RE-05 | Contracts | contract-first gates | APPROVED |
| RE-06 | CI | GitHub Actions | APPROVED |
| RE-07 | Artifacts | signed reproducible outputs | APPROVED |
| RE-08 | DB access | pgx + sqlc direction | APPROVED |
| RE-09 | Migrations | explicit SQL tool | APPROVED |
| RE-10 | Web client | generated typed API | APPROVED |
| RE-11 | Testing | multilayer lab/tests | APPROVED |
| RE-12 | Security fixes | permanent regression fixture | APPROVED |
| RE-13 | AI agent perms | coding yes; decisions no | APPROVED |
| RE-14 | Agent manifest | required on significant PR | APPROVED |
| RE-15 | Dependencies | reviewed/pinned/licensed | APPROVED |
| RE-16 | Versioning | trunk + tagged releases | APPROVED |
| RE-17 | Feature flags | risky capabilities gated | APPROVED |
| RE-18 | Lab | reproducible virtual lab | APPROVED |

---

# 26. Architecture Decision Change Control

Step 4 owner review tamamlanmıştır. Bu belgeye göre architecture/technology kararlarında açık `OWNER DECISION` bulunmamaktadır.

Bundan sonra bu kararlardan biri değiştirilmek istenirse değişiklik sessiz implementation detayı olarak yapılmamalıdır. Süreç:

1. İlgili decision ID ve etkilenen ADR belirlenir.
2. Değişiklik gerekçesi yazılır.
3. Product, security, operations ve compatibility etkileri analiz edilir.
4. Gerekirse focused spike/benchmark/security test yapılır.
5. Yeni yön `APPROVED` olarak sunulur.
6. Product Owner kararı olmadan architecture baseline değiştirilmez.
7. Onaylanan değişiklik ADR ve bu master architecture belgesine işlenir.

AI coding agent'ları bu süreci atlayamaz. Bir agent yeni dependency, trust boundary, wire protocol, persistence technology veya deployment model değişikliği gerektiğini düşünürse implementation yapmak yerine decision/ADR draft'ı üretmelidir.

---

# 27. Step 4 Closure / Exit Criteria

Adım 4 aşağıdaki koşullarla **CLOSED / APPROVED** kabul edilir:

- [x] Software/component sınırları
- [x] Core deployable sayısı
- [x] Control Plane architecture
- [x] Edge architecture ve privilege split
- [x] Edge appliance OS/deployment
- [x] Decoy pack/runtime contract
- [x] Container runtime
- [x] High-interaction VMM/orchestrator
- [x] Windows image/licensing strategy
- [x] Network presence modes
- [x] VLAN/multi-NIC/IP strategy
- [x] Control↔Edge protocol ve device PKI
- [x] Telemetry delivery/idempotency
- [x] Central/edge databases
- [x] Object storage
- [x] Broker/no-broker kararı
- [x] Correlation engine architecture
- [x] AI provider abstraction ve initial adapters
- [x] AI safety/tool authority
- [x] Authentication/session/MFA baseline
- [x] Secret management
- [x] Artifact signing/update security
- [x] Observability stack
- [x] Programming languages
- [x] Frontend framework/build stack
- [x] API/schema technologies
- [x] Repository strategy
- [x] CI/CD
- [x] Testing/security lab strategy
- [x] AI coding-agent governance
- [x] Architecture validation backlog ayrı tutuldu

**Sonuç:** 202 / 202 karar APPROVED. Adım 4'te açık architecture/technology owner decision bulunmamaktadır.

---

# 28. Final Architecture Conclusion

Step 4 sonunda onaylanan ana architecture ilkesi şudur:

> **Ürünün architecture'ı cloud-native microservice platform olarak değil, güvenlik nedeniyle gerçekten dağıtılması gereken noktaları dağıtan; geri kalanını güçlü modüler sınırlarla basit tutan bir sistem olmalıdır.**

Onaylanan temel teknik karakter:

- **Go-centric control/edge**
- **React/TypeScript SPA**
- **Modular monolith Control Plane**
- **Native Linux Edge appliance**
- **OCI decoy packs via containerd**
- **QEMU/KVM/libvirt high-interaction worker**
- **gRPC/Protobuf/mTLS device plane**
- **REST/OpenAPI user/integration API**
- **PostgreSQL central + SQLite edge**
- **No Kafka/Kubernetes/Redis/ClickHouse initially**
- **S3-compatible hostile/blob storage**
- **Deterministic correlation core + AI reasoning layer**
- **OpenAI/Anthropic provider adapters behind common structured interface**
- **Cosign + TUF supply-chain/update model**
- **OpenTelemetry-first observability**
- **Monorepo + contract-first CI + explicit ADR/owner governance**

Bu architecture setinin amacı “en modern stack”i seçmek değil; Step 2 ve Step 3'te onaylanan **high signal, safe-by-default, human-controlled, operationally simple, AI-assisted but evidence-first** ürün ilkelerini architecture seviyesinde korumaktır.


---

# 29. Handoff — MVP Scope & Acceptance Criteria

Step 4'ün kapanmasıyla artık ürünün teknik possibility space'i ve implementation baseline'ı yeterince tanımlıdır. Bir sonraki aşamada architecture yeniden seçilmeyecek; MVP kapsamı bu architecture içinde **hangi capability'lerin ilk ürün dilimine gireceği** üzerinden belirlenecektir.

MVP aşamasında karara bağlanması gereken ana konular:

- MVP'de hangi threat behaviors gerçekten desteklenecek?
- Hangi decoy pack'leri ilk release'e girecek?
- Hangi network topology'leri acceptance kapsamına alınacak?
- Windows capability'nin MVP seviyesi ne olacak?
- High-interaction MVP dışında mı kalacak?
- Environment discovery'nin hangi seviyesi zorunlu olacak?
- Attacker journey için minimum correlation capability nedir?
- AI explanation/guidance MVP'de hangi kalite bar'ını geçmeli?
- Notification channels'ın minimum seti nedir?
- Health/update/onboarding için minimum complete product experience nedir?
- Hangi validation spike'ları MVP implementation öncesi blocker'dır?
- Her capability için measurable acceptance criteria nedir?

MVP seçimi architecture'ı keyfi biçimde değiştirmemelidir. Bir MVP gereksinimi onaylı architecture ile çelişirse önce bu belgede architecture change-control süreci işletilir.

---

# 30. Document Control

- **Document:** `System_Architecture_and_Technology_Decisions.md`
- **Step:** 4 — System Architecture & Technology Decisions
- **Status:** APPROVED / FINAL
- **Approved decision count:** 202
- **Open owner-decision count:** 0
- **Validation / architecture spike backlog:** OPEN BY DESIGN
- **Technology baseline:** APPROVED
- **Repository / CI / agentic engineering governance:** APPROVED
- **Next stage:** MVP Scope & Acceptance Criteria
- **Decision authority:** Product Owner
- **Default artifact format:** Markdown
