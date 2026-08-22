# Technical Feasibility & Requirements
## Step 3 — Final Approved Output

**Belge durumu:** APPROVED / FINAL  
**Amaç:** Adım 2'de onaylanan Product Definition & Strategy kararlarını teknik gereksinimlere, güvenlik sınırlarına, feasibility sonuçlarına, validation ihtiyaçlarına ve Adım 4 mimari girdilerine dönüştürmek.  
**Karar otoritesi:** Product Owner  
**Bu belgede yer alan 113 Step 3 kararının tamamı Product Owner tarafından APPROVED edilmiştir.**  
**Bu belge:** Step 3'ün tamamlanmış final çıktısıdır.


### Step 3 sonucu

- **113 / 113 karar:** APPROVED
- **Açık owner decision:** 0
- **Technology stack seçimi:** Yapılmadı
- **Architecture implementation seçimi:** Yapılmadı
- **MVP scope seçimi:** Yapılmadı
- **Validation backlog:** Açık ve bilinçli
- **Bir sonraki aşama:** Step 4 — System Architecture & Technology Decisions

---

# 0. Çalışma ilkeleri

Bu belge **architecture/technology selection belgesi değildir**. Burada cevaplamaya çalıştığımız soru:

> Onaylanan ürün davranışlarını güvenilir, güvenli ve operasyonel olarak sürdürülebilir biçimde gerçekleştirmek için sistem **ne yapmak zorundadır**, hangi teknik gerçeklerle karşılaşır ve hangi ürün/teknik sınırların OWNER DECISION ile belirlenmesi gerekir?

Bu nedenle aşağıdakiler bu aşamada **seçilmeyecektir**:

- Programlama dili
- Backend/frontend framework
- Database ürünü
- Queue/message broker ürünü
- Container runtime
- Hypervisor ürünü
- Kubernetes veya başka orchestration ürünü
- Cloud sağlayıcı
- VPN/overlay ürünü
- LLM sağlayıcısı/modeli
- Observability ürünü
- SIEM/EDR/NDR vendor entegrasyonu
- Belirli OSS bileşeninin build-vs-integrate kararı
- Repository yapısı
- MVP feature listesi
- Roadmap/release planı

Bunlar daha sonraki **System Architecture & Technology Decisions** aşamasının konusudur.

## 0.1 Karar süreci

Her madde aynı statü akışını kullanır:

`OPEN → RESEARCHED → RECOMMENDED → OWNER DECISION → APPROVED`

Bu dokümandaki 113 karar noktası Product Owner tarafından incelenmiş ve önerilen yönleriyle **APPROVED** edilmiştir.

## 0.2 Değerlendirme lensleri

Her karar şu beş açıdan değerlendirilir:

1. **Product value:** Primary persona için ne değişir?
2. **Security value:** Gerçek saldırı davranışını görme/güvenle açıklama yeteneğine etkisi nedir?
3. **System implication:** Daha sonra architecture için hangi requirement oluşur?
4. **Risk / failure mode:** Yanlış karar hangi tür failure üretir?
5. **Operational burden:** Security-lean müşteri için bakım/konfigürasyon yükü nedir?

## 0.3 Adım 2'den gelen bağlayıcı ilkeler

Aşağıdakiler tekrar tartışılmayan APPROVED girdilerdir:

- Product promise: **internal breach visibility**
- Primary mechanism: **deception**
- Outcome: **high-confidence incident + attacker journey + guidance**
- Assume-breach / post-compromise odak
- Primary threats: recon/discovery, credential abuse, lateral movement
- Product EDR/NDR/SIEM/SOAR/firewall replacement değildir
- Primary persona: dedicated SOC'u olmayan generalist technical owner
- AI evidence üretmez; evidence'ı yorumlar
- High signal over high volume
- Human owns security decisions
- AI unavailable olduğunda core detection truth çalışmalıdır
- Public/global honeynet ilk ürün kimliği değildir
- Full offline/on-prem gelecekte mümkündür; ilk sürüm için internet bağımlılığı kategorik olarak yasak değildir
- Network + endpoint + identity deception uzun vadeli vizyondadır
- High-interaction/vulnerable systems core detection'ın zorunlu ilk katmanı değildir
- Level C hedefi: “şuradan geldi, şunları gezdi, şunları yaptı”

---

# 0.4 Araştırma dayanakları

Aşağıdaki kaynaklar seçeneklerin ve risklerin teknik gerçekliğini çapraz kontrol etmek için kullanılmıştır. Bunlar ürün kararının yerine geçmez.

- **[R1] NIST SP 800-61 Rev. 3 (2025)** — Incident response'u risk yönetimine entegre eder; detect/respond/recover akışında context ve organization-specific judgment gerekliliğini vurgular.
- **[R2] NIST CSF 2.0 Small Business guidance** — Küçük/orta işletmeler için Govern/Identify/Protect/Detect/Respond/Recover perspektifi.
- **[R3] CISA SMB logging guidance** — User activity, admin actions, network traffic, logins ve system events gibi telemetry'nin merkezi ve korunmuş biçimde tutulmasının incident investigation değerini vurgular.
- **[R4] MITRE ATT&CK Enterprise** — Discovery ve Lateral Movement kapsamında SMB, RDP, SSH, WinRM ve remote services davranışlarını standardize eder.
- **[R5] OpenCanary documentation** — Düşük etkileşimli service deception, source ignore list ve event correlation için pratik referans.
- **[R6] Thinkst Canary documentation** — Known scanner ignore list'leri ve network/identity/document/cloud token gibi çok çeşitli deception artefact modelleri.
- **[R7] Cowrie documentation** — Medium/high-interaction SSH/Telnet, fake filesystem, command/session logging, file transfer capture, proxy-to-real-system ve deneysel LLM interaction yaklaşımı.
- **[R8] NIST SP 800-125A/B ve SP 800-190** — VM/hypervisor/container isolation, segmentation ve virtual network security gereksinimleri.
- **[R9] Microsoft Windows Security Auditing documentation** — Authentication event'lerinde source IP/workstation/account gibi alanların protokole ve authentication context'ine göre değişebildiğini gösterir.
- **[R10] Microsoft Windows Server virtualization licensing guidance** — Real Windows Server VM kullanımının licensing/entitlement gereksinimleri doğurabileceğini doğrular.
- **[R11] OWASP GenAI Security — Prompt Injection 2025** — External/attacker-controlled content'in indirect prompt injection kaynağı olduğunu; least privilege ve human approval gerekliliğini vurgular.
- **[R12] NIST AI RMF Generative AI Profile** — GenAI trustworthiness, evaluation, human oversight ve risk management için referans.
- **[R13] Honeynet Project high-interaction/hybrid honeynet çalışmaları** — High interaction'ın daha zengin telemetry sunduğunu fakat outbound control/containment zorunluluğunu gösteren tarihsel/pratik referans.
- **[R14] CISA joint LOTL guidance** — Segmentation ve stratejik sensor placement'ın lateral movement riskini sınırlama ve visibility sağlama değerini vurgular.

---

# 1. Internal Network Visibility Feasibility

## Amaç

Ürün internal/private network içindeki deception assets'e gelen interaction'ları görebilmeli ve mümkün olduğunda source context üretebilmelidir. Ancak “ağın içinde olmak” tek bir teknik durum değildir: routed VLAN'lar, flat LAN, site-to-site VPN, remote-access VPN, NAT, cloud VPC/VNet, branch network ve segmented environments farklı visibility sınırları yaratır.

### Karar NV-01 — Ürünün network visibility modeli tek bir observation point varsayabilir mi?

**Durum:** APPROVED

**Karar sonucu:** APPROVED

**Seçenek A — Tek merkezi observation/appliance**
- Operasyon basittir.
- Küçük/flat networklerde yeterli olabilir.
- Ancak L3 segmentation, ACL ve routing nedeniyle her subnet'te gerçekçi decoy presence sağlayamayabilir.
- Bir merkezi cihaz “bütün internal hareketi görür” iddiası teknik olarak doğru olmayabilir.

**Seçenek B — Her segmentte zorunlu sensor/decoy node**
- Visibility ve local realism artar.
- Deployment/upgrade/health operasyonu büyür.
- Primary persona için “hafif ürün” ilkesini zorlayabilir.

**Seçenek C — Hybrid: merkezi control plane + ihtiyaç olduğunda distributed presence**
- Basit networkte tek node ile başlayabilir.
- Segmentation varsa ek presence point'leri eklenebilir.
- Architecture daha sonra distributed bileşenleri desteklemek zorunda kalır.

**Karşıt görüş:** İlk ürün yalnız küçük flat LAN hedeflerse dağıtık model erken over-engineering olabilir.

**APPROVED REQUIREMENT:** **C.** Product requirement “tek appliance her networkü kapsar” olmamalı. Basit deployment tek node ile mümkün olabilir; fakat ürün modeli **multi-segment presence** kabiliyetini daha sonraki architecture'a açık requirement olarak bırakmalıdır.

**System implication:** Control/management ile deception execution/presence kavramları ayrılabilir olmalıdır; teknoloji seçimi daha sonra.

**Owner decision:** APPROVED — önerilen yön kabul edildi.

---

### Karar NV-02 — Visibility “network traffic'i izlemek” mi, “decoy'a gelen interaction'ı görmek” mi olmalı?

**Durum:** APPROVED

**Seçenek A — Full passive network visibility**
- NDR benzeri geniş telemetry sağlar.
- Source/path context kuvvetlenebilir.
- Packet capture/mirroring/TAP/SPAN ihtiyacı ve noise artar.
- Step 2 non-goal olan general NDR'a kayma riski vardır.

**Seçenek B — Yalnız decoy endpoint/service interaction**
- Product boundary nettir.
- High-signal yaklaşım korunur.
- Attacker'ın decoy dışındaki yolculuğuna dair kör noktalar kalır.

**Seçenek C — Deception-first; gerektiğinde sınırlı contextual network metadata**
- Core truth decoy interaction olarak kalır.
- Flow/path/DNS/ARP benzeri bağlam daha sonra yardımcı evidence olabilir.
- NDR'a dönüşmeden correlation kalitesi artırılabilir.

**APPROVED REQUIREMENT:** **C.** Deception interaction **primary evidence**; passive/contextual network data **supporting evidence** olmalı.

**Karşıt görüş:** “Level C attacker journey” yalnız decoy interaction ile eksik kalabilir. Bu doğrudur; ancak çözüm otomatik olarak full NDR inşa etmek değildir.

---

### Karar NV-03 — Routed/segmented networklerde “aynı VLAN'da görünme” bir requirement mı?

**Durum:** APPROVED

**Seçenek A — Evet, her decoy hedef segmentte L2 presence göstermeli**
- Realism güçlüdür.
- Placement operasyonu ve network privileges artar.

**Seçenek B — Hayır, routed reachability yeterlidir**
- Deployment kolaylaşır.
- Attacker enumeration sırasında decoy'un “başka segmentte” görünmesi gerçekçilik kaybı yaratabilir.

**Seçenek C — Capability bazlı**
- Bazı deception servisleri routed erişimle yeterli.
- L2/hostname/discovery realism isteyen persona'lar için local presence gerekir.

**APPROVED REQUIREMENT:** **C.** Product requirement deployment modeline göre “routed decoy” ve “local-segment decoy” ayrımını desteklemelidir. Hepsine tek kural uygulanmamalıdır.

---

### Karar NV-04 — NAT/VPN/gateway arkasında source identity nasıl ele alınmalı?

**Durum:** APPROVED

**Seçenek A — Source IP'yi doğrudan actor identity kabul et**
- Basittir.
- VPN concentrator, NAT, proxy ve shared jump host durumlarında yanlış attribution üretir.

**Seçenek B — Network identity yalnız “observed source” olarak tutulur**
- Daha dürüsttür.
- Kullanıcıya daha az kesin cevap verebilir.

**Seçenek C — Observed source + enrichment graph**
- Source IP, hostname, VPN identity, authenticated username, DHCP/asset context gibi sinyaller ayrı provenance ile ilişkilendirilir.
- “Identity” tek bir alan değil confidence'lı bağlam olur.

**APPROVED REQUIREMENT:** **C.** Level C hedefinin güvenilir olması için ürün source identity'yi **composite and evidence-backed** modellemelidir.

---

### Karar NV-05 — Ürün network placement feasibility'sini kurulum öncesinde doğrulamalı mı?

**Durum:** APPROVED

**Seçenek A — Kullanıcının doğru network tasarımını yaptığı varsayılır**
- Basit.
- Sessiz coverage failure riski yüksektir.

**Seçenek B — Deployment validation zorunlu capability**
- Routing/reachability, port conflict, decoy visibility ve management connectivity gibi durumları doğrular.
- Ürünün “çalışıyor görünüyor ama aslında görünmüyor” riskini azaltır.

**APPROVED REQUIREMENT:** **B.** Security product'ta **coverage verification** health modelinin parçası olmalıdır.

---

# 2. Deception Deployment Feasibility

## Amaç

Adım 2 deception taxonomy'sini teknik gerçekliğe çevirmek: fake host/service/application/database/Windows endpoint/credential/identity/high-interaction sistemlerin her biri farklı privilege, realism, telemetry ve risk profiline sahiptir.

### Karar DD-01 — Deception capability tek tip “decoy instance” olarak mı modellenmeli?

**Durum:** APPROVED

**A — Tek abstraction**
- UI basit.
- Ancak token, fake credential, emulated SSH ve real VM'nin lifecycle'ları kökten farklıdır.

**B — Ayrı ürün aileleri**
- Teknik gerçeklik daha doğru.
- UI/mental model parçalanabilir.

**C — Ortak “Deception Object” üst modeli + subtype'lar**
- Ortak metadata: owner, placement, persona, health, risk, lifecycle.
- Subtype-specific requirements korunur.

**APPROVED REQUIREMENT:** **C.**

**System implication:** Architecture ileride polymorphic capability model gerektirebilir; henüz schema/DB seçilmez.

---

### Karar DD-02 — Realism ürün requirement'ı nasıl ifade edilmeli?

**Durum:** APPROVED

**A — Maximum realism**
- Fingerprinting'e direnç artar.
- Cost/risk/maintenance hızla büyür.

**B — Minimum sufficient realism**
- Her threat behavior için gerekli kadar gerçekçilik.
- Product simplicity'ye uygundur.

**C — Kullanıcı seçsin**
- Esnektir.
- Security-lean persona için yanlış configuration riski doğurur.

**APPROVED REQUIREMENT:** **B**, advanced kullanıcı için kontrollü override gelecekte değerlendirilebilir.

**Karşıt görüş:** Sophisticated attacker düşük realism'i hızlıca fark eder. Doğru; bu nedenle realism “önemsiz” değildir. Öneri “düşük realism” değil, **signal objective için yeterli realism** yaklaşımıdır.

---

### Karar DD-03 — Decoy deployment yüksek host privilege isteyebilir mi?

**Durum:** APPROVED

**A — Root/admin privilege normal kabul edilir**
- Protocol binding/network manipulation kolaylaşır.
- Compromise blast radius büyür.

**B — Least privilege kesin requirement**
- Bazı capability'ler zorlaşır.
- Security product için daha güvenli baseline.

**C — Privilege tiering**
- Default least privilege.
- Belirli capability için ihtiyaç duyulan ek privilege açıkça gerekçelendirilir ve izole edilir.

**APPROVED REQUIREMENT:** **C.** “Security productだから root” varsayımı yasaklanmalı; privilege capability-specific olmalıdır.

---

### Karar DD-04 — Production asset ile decoy aynı host üzerinde çalışabilir mi?

**Durum:** APPROVED

**A — Evet**
- Resource efficiency.
- Compromise/isolation/fingerprinting ve operational coupling riski.

**B — Hayır, deception execution production workload'dan ayrılmalı**
- Daha güvenli.
- Deployment footprint artabilir.

**APPROVED REQUIREMENT:** **B** default principle. Endpoint breadcrumb gibi host-resident artefact'lar ayrı kategoridir; interactive decoy service ile karıştırılmamalıdır.

---

### Karar DD-05 — Her decoy için explicit safety profile tutulmalı mı?

**Durum:** APPROVED

**A — Gereksiz; decoy decoy'dur**
- Basit.
- Medium/high interaction riskini görünmez yapar.

**B — Evet**
Örnek alanlar:
- Interaction level
- Can execute attacker input?
- Can accept file upload?
- Can initiate outbound?
- Has real OS?
- Has credentials/secrets?
- Reset strategy
- Isolation requirement
- Data sensitivity

**APPROVED REQUIREMENT:** **B.** Safety profile daha sonraki placement policy ve architecture için zorunlu girdidir.

---

# 3. Low / Medium / High Interaction Model

## Amaç

“Interaction level” pazarlama etiketi değil, **attacker'a ne kadar gerçek computation/state sunduğumuz** ve buna karşılık ne kadar telemetry/risk aldığımızın modeli olmalıdır. OpenCanary düşük etkileşimli service simulation'a; Cowrie medium/high interaction ve proxy modeline pratik örnekler sunar. [R5][R7]

### Karar IL-01 — Default interaction seviyesi ne olmalı?

**Durum:** APPROVED

**A — Low interaction default**
- Güvenli ve hafif.
- Command/file/process gibi zengin behavior kaçabilir.

**B — Medium interaction default**
- Daha zengin evidence.
- Emulation fidelity ve bakım yükü artar.

**C — High interaction default**
- Maximum behavior.
- Escape, outbound abuse, malware, licensing ve reset burden çok daha yüksektir.

**D — Layered default**
- Discovery/initial touch için low.
- Intent yükseldikçe medium/high capability seçilebilir.

**APPROVED REQUIREMENT:** **D.** Step 2'de APPROVED layered modelin teknik karşılığı budur.

---

### Karar IL-02 — Interaction escalation otomatik mi olmalı?

**Durum:** APPROVED

**A — Her session dinamik olarak high interaction'a taşınır**
- Maximum intel.
- Session migration teknik olarak zor; yeni attack surface.

**B — Önceden seçilmiş decoy level sabit**
- Predictable.
- Adaptive deception vizyonunu sınırlar.

**C — Current stage'de preconfigured levels; future controlled escalation capability**
- Güvenlik ve complexity yönetilebilir.
- Redirection feasibility ayrıca incelenir.

**APPROVED REQUIREMENT:** **C.**

---

### Karar IL-03 — Emulated shell/application hangi noktada “yetersiz” sayılmalı?

**Durum:** APPROVED

**A — Attacker fingerprint edebiliyorsa yetersiz**
- Çok yüksek bar; her deception eventually fingerprint edilebilir.

**B — Intended evidence objective'i karşılamıyorsa yetersiz**
Örneğin amaç credential use yakalamaksa gerçek kernel gerekmeyebilir; amaç post-login commands/malware behavior ise gerekir.

**APPROVED REQUIREMENT:** **B.** Capability acceptance “gerçek makineye ne kadar benziyor?” kadar “hangi threat hypothesis'i güvenilir test ediyor?” ile ölçülmeli.

---

### Karar IL-04 — High interaction ne zaman zorunlu kabul edilmeli?

**Durum:** APPROVED

**APPROVED kriter seti:**
High interaction ancak aşağıdakilerden biri gerekiyorsa justified:
- Arbitrary attacker command execution'ını gerçek OS semantiğiyle gözlemlemek
- Unknown tool/binary behavior görmek
- Persistence/file/process changes yakalamak
- Real application exploit sonrası davranış gözlemlemek
- Attacker'ı daha uzun süre gerçekçi environment'ta tutmak

Sadece port scan/login attempt yakalamak için high interaction **gereksiz risk** sayılmalı.

---

# 4. Environment Understanding / Discovery

## Amaç

Ürünün “doğru yere doğru deception” önerebilmesi için environment context gerekir. Fakat full asset discovery/vulnerability scanner olmak Step 2 non-goal'dır.

### Karar EU-01 — Environment context nasıl edinilmeli?

**Durum:** APPROVED

**A — Kullanıcı manual girer**
- Güvenli ve deterministic.
- Setup burden yüksek, stale data riski.

**B — Active discovery**
- Hızlı context.
- Scanner noise, permissions, müşteri policy etkisi.

**C — Passive observation**
- Düşük intrusion.
- Yeterli trafik görmeyebilir; sensor placement ister.

**D — Existing sources/integrations**
- Rich context.
- Vendor bağımlılığı ve integration scope.

**E — Hybrid minimal-context**
- Manual seed + safe discovery + optional integrations.

**APPROVED REQUIREMENT:** **E.** “Full CMDB” değil, deception placement için **minimum sufficient context**.

---

### Karar EU-02 — Active scanning default açık mı olmalı?

**Durum:** APPROVED

**A — Evet**
- Onboarding kolay.
- Müşterinin scanner/IDS/operations süreçleriyle çakışabilir.

**B — Hayır, explicit opt-in**
- Güvenli.
- Product automation azalır.

**C — Safe bounded discovery + explicit consent**
- Yalnız local subnet/basic protocol metadata gibi düşük-impact alanlarla sınırlanabilir.

**APPROVED REQUIREMENT:** **C**, fakat exact discovery methods Adım 4'te seçilir.

---

### Karar EU-03 — Passive discovery core requirement mı?

**Durum:** APPROVED

**A — Core**
- Continuous context sağlar.
- SPAN/TAP/privilege ve NDR'a kayma riski.

**B — Optional enrichment**
- Ürün deception-first kalır.

**APPROVED REQUIREMENT:** **B.**

---

### Karar EU-04 — Kullanıcıya network modelini düzenleme imkânı verilmeli mi?

**Durum:** APPROVED

**A — Hayır; sistem bilir**
- Sade.
- Yanlış inference'ın silent failure riski.

**B — Evet; system suggestion + owner correction**
- Human control ilkesine uyumlu.

**APPROVED REQUIREMENT:** **B.** Environment model “AI truth” olmamalı.

---

### Karar EU-05 — Environment drift nasıl ele alınmalı?

**Durum:** APPROVED

**Seçenekler:**
- One-time onboarding snapshot
- Periodic revalidation
- Continuous change detection

**APPROVED REQUIREMENT:** Product requirement **staleness awareness** olmalı: sistem context'in ne zaman son doğrulandığını bilmeli ve coverage'ın stale olabileceğini gösterebilmelidir. Exact frequency daha sonra belirlenir.

---

# 5. Attacker Source Identification Feasibility

## Amaç

Level C hedefi “şuradan geldi” der; fakat network security'de source IP her zaman cihaz/user identity değildir. Microsoft Windows auditing dokümanları bile source IP/workstation alanlarının protocol/authentication context'e göre eksik olabileceğini belirtir. [R9]

### Karar SI-01 — Ürün attribution konusunda hangi söz seviyesini vermeli?

**Durum:** APPROVED

**A — “Attacker identity”**
- Güçlü marketing.
- Teknik olarak aşırı iddialı.

**B — “Source identification”**
- Observed source + evidence context.
- Gerçek-world person attribution iddiası yok.

**APPROVED REQUIREMENT:** **B**, Step 2 kararıyla uyumlu.

---

### Karar SI-02 — Identifier hierarchy nasıl düşünülmeli?

**Durum:** APPROVED

**APPROVED evidence hierarchy:**
1. Observed network source (IP/port/interface/segment)
2. Session/protocol identity
3. Authentication identity (username/account)
4. Device clues (hostname/workstation/MAC/DHCP/asset record)
5. VPN/access identity
6. Endpoint/identity platform enrichment
7. Human-confirmed mapping

Hiçbiri otomatik olarak “gerçek saldırgan kişi” değildir.

---

### Karar SI-03 — Endpoint/identity enrichment current core requirement mı?

**Durum:** APPROVED

**A — Evet**
- Attribution güçlü.
- Agent/integration footprint büyür.

**B — Hayır**
- Simplicity.
- Source clarity sınırlı olabilir.

**C — Product modelde desteklenebilir enrichment, current deployment için zorunlu değil**
- Long-term network+endpoint+identity vision'a uyumlu.

**APPROVED REQUIREMENT:** **C.**

---

### Karar SI-04 — Belirsiz source'lar nasıl gösterilmeli?

**Durum:** APPROVED

**A — Tek en olası source seç**
- UX basit.
- False certainty.

**B — Confidence + evidence**
Örnek:
- Observed source IP: high confidence
- Hostname mapping: medium
- User mapping: low/medium
- VPN identity: high if authoritative source exists

**APPROVED REQUIREMENT:** **B.** Honest uncertainty core requirement.

---

# 6. Telemetry & Evidence Model Feasibility

## Amaç

Step 2'de `Event → Signal → Evidence → Finding → Incident` ayrımı onaylandı. Teknik feasibility için raw telemetry'nin güvenilir provenance, time, source ve integrity özellikleri gerekir. CISA/NIST logging guidance investigation için yeterli detail ve korunmuş logların önemini vurgular. [R1][R3]

### Karar TE-01 — Canonical evidence modeli zorunlu mu?

**Durum:** APPROVED

**A — Her decoy kendi log formatını kullanır**
- Entegrasyon kolay başlar.
- Correlation/AI/UI karmaşıklaşır.

**B — Normalize edilmiş canonical event/evidence contract**
- Upstream decoy çeşitliliğini absorbe eder.
- Schema governance gerekir.

**APPROVED REQUIREMENT:** **B.** Technology-neutral canonical contract Step 3 requirement'ıdır.

Minimum semantic fields:
- event/evidence ID
- timestamp + clock quality
- deception object
- source/destination context
- protocol/session
- action/interaction
- authentication context
- raw-source reference
- parser/source version
- integrity/provenance
- confidence dimensions
- sensitivity classification

---

### Karar TE-02 — Ne kadar telemetry capture edilmeli?

**Durum:** APPROVED

**A — Minimum metadata**
- Privacy/storage avantajı.
- Investigation zayıf.

**B — Everything/full packet/full session**
- Rich forensics.
- Privacy/storage/secrets/malware risk.

**C — Capability-specific evidence depth**
- Port touch için metadata
- Auth için username/result
- Shell için commands/session
- High interaction için file/process/network changes vb.

**APPROVED REQUIREMENT:** **C.**

---

### Karar TE-03 — Raw attacker payload saklanmalı mı?

**Durum:** APPROVED

**A — Daima**
- Re-analysis mümkün.
- Sensitive/malicious content riski.

**B — Asla**
- Güvenli.
- Forensics ve parser improvement sınırlanır.

**C — Selective/raw evidence retention policy**
- High-value interaction için tutulabilir.
- Sensitivity + retention + access controls gerekir.

**APPROVED REQUIREMENT:** **C.**

---

### Karar TE-04 — Time synchronization product requirement mı?

**Durum:** APPROVED

**A — “Best effort” yeter**
- Kolay.
- Attacker journey sıralaması ve correlation bozulabilir.

**B — Time quality first-class**
- Timestamp source/offset/skew awareness gerekir.

**APPROVED REQUIREMENT:** **B.** Distributed presence varsa özellikle kritik.

---

### Karar TE-05 — Evidence immutability/integrity ne seviyede olmalı?

**Durum:** APPROVED

**A — Normal application log**
- Basit.
- Compromised decoy veya admin manipulation ayırt edilemeyebilir.

**B — Tam forensic/WORM zorunlu**
- Güçlü.
- Operasyon ve cost artar.

**C — Tamper-evident provenance baseline; stronger retention optional**
- Evidence chain'de modification detection ve source version tutulur.

**APPROVED REQUIREMENT:** **C.** “Court-admissible forensics” iddiası yok; ancak evidence'ın sessizce değişmemesi requirement.

---

# 7. Correlation & Attacker Journey Feasibility

## Amaç

Tekil decoy hit'lerini “aynı saldırı hikâyesi” altında toplamak Level C için çekirdektir. Ancak shared NAT, jump hosts, DHCP churn ve reused credentials nedeniyle “aynı IP = aynı actor” güvenilir değildir.

### Karar CR-01 — Incident correlation'ın primary key'i ne olmalı?

**Durum:** APPROVED

**A — Source IP**
- Basit; yanlış merge/split riski.

**B — User/account**
- Credential abuse'da yararlı; stolen/shared account.

**C — Session**
- Kesin ama journey çok parçalanır.

**D — Multi-evidence correlation graph**
- Time, source, account, segment, protocol, token, device context gibi dimensions.

**APPROVED REQUIREMENT:** **D.**

---

### Karar CR-02 — Correlation deterministic mi probabilistic mi?

**Durum:** APPROVED

**A — Yalnız deterministic**
- Explainable.
- Cross-session journey kaçabilir.

**B — Yalnız ML/AI**
- Flexible.
- Trust ve reproducibility zayıf.

**C — Deterministic anchors + probabilistic hypotheses**
- Strong links deterministic
- Soft links confidence ile

**APPROVED REQUIREMENT:** **C.**

---

### Karar CR-03 — LLM correlation'ın “truth engine”i olabilir mi?

**Durum:** APPROVED

**A — Evet**
- Semantic flexibility.
- Hallucination, prompt injection, nondeterminism.

**B — Hayır; LLM yalnız narrative**
- Güvenli.
- Bazı semantic relationship fırsatları kaçabilir.

**C — LLM hypothesis producer; deterministic/evidence graph truth source**
- AI “bu event'ler ilişkili olabilir” diyebilir.
- UI bunu inference olarak etiketler.

**APPROVED REQUIREMENT:** **C.**

---

### Karar CR-04 — Attacker journey yalnız decoy event'lerinden mi oluşmalı?

**Durum:** APPROVED

**A — Evet**
- Evidence purity.
- Gerçek hareketin araları görünmez.

**B — Decoy evidence + contextual evidence + explicit inference**
- Daha anlaşılır.
- Katmanların UI'da ayrılması gerekir.

**APPROVED REQUIREMENT:** **B.**
Journey node types:
- Observed
- Correlated
- Inferred
- User-confirmed

---

### Karar CR-05 — Yanlış merge/split düzeltilebilir olmalı mı?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** **Evet.**
Owner/operator:
- incident merge
- incident split
- evidence detach/attach
- source mapping correction
yapabilmeli veya en azından system hypothesis'ini override edebilmelidir.

Bu feedback ileride correlation quality improvement için kullanılabilir; autonomous retraining kararı değildir.

---

# 8. Confidence, Severity & False Positive Feasibility

## Amaç

Deception high-signal olabilir ama “decoy'a dokunan herkes attacker” mutlak kural değildir. Known vulnerability scanners ve management automation legitimate interaction üretebilir; OpenCanary/Thinkst ürünlerinde ignore-list mekanizmalarının bulunması bunun pratik kanıtıdır. [R5][R6]

### Karar CF-01 — Confidence neyi ölçer?

**Durum:** APPROVED

**A — Olayın zarar büyüklüğünü**
- Severity ile karışır.

**B — Evidence'ın “beklenmeyen/suspicious behavior” hipotezini destekleme gücünü**
- Daha doğru.

**APPROVED REQUIREMENT:** **B.**

---

### Karar CF-02 — Confidence modeli tek numeric score mu olmalı?

**Durum:** APPROVED

**A — 0–100**
- UX basit.
- False precision riski.

**B — Low/Medium/High**
- Sade.
- Nedenini açıklamak gerekir.

**C — Level + factor breakdown**
Örn:
- Direct decoy interaction: strong
- Known scanner match: lowers confidence
- Valid honey credential use: very strong
- Multi-decoy movement: strong
- Source identity uncertainty: separate dimension

**APPROVED REQUIREMENT:** **C.** Numeric internal hesap olabilir; user-facing model explainable olmalı.

---

### Karar CF-03 — Known scanner/admin allowlist nasıl davranmalı?

**Durum:** APPROVED

**A — Event'i drop et**
- Noise azalır.
- Forensic blind spot.

**B — Capture but suppress notification**
- Evidence/history korunur.
- Noise kontrol edilir.

**C — Contextual policy**
- Known scanner + expected target/port/time için suppress
- Deviations surface

**APPROVED REQUIREMENT:** **C**, minimum olarak **B** seviyesinde evidence retention.

---

### Karar CF-04 — User feedback otomatik confidence modelini değiştirebilir mi?

**Durum:** APPROVED

**A — Evet, self-learning**
- Convenient.
- Poisoning/mistake riski.

**B — Hayır**
- Stable.
- Repeated false positives.

**C — Reversible rule proposals**
- User “benign scanner” der
- System scoped suppression önerir
- Owner scope'u görür/onaylar

**APPROVED REQUIREMENT:** **C.**

---

### Karar CF-05 — Severity confidence'dan bağımsız mı?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** **Evet.**
Örnek:
- Tek port scan decoy hit: confidence high, severity low/medium
- Honey privileged credential kullanımı: confidence high, severity high
- Inferred suspicious sequence: confidence medium, potential severity high

UI bu iki ekseni birleştirip tek “risk” kelimesine indirmemeli.

---

# 9. Incident Information Model

## Amaç

Henüz database schema seçmeden, incident'ı doğru temsil etmek için gereken domain objects'i tanımlamak.

### Karar IM-01 — Minimum first-class domain objects

**Durum:** APPROVED

**APPROVED REQUIREMENT:**
- Environment
- Network zone/segment
- Deception object
- Persona
- Source entity
- Identity/account entity
- Session
- Event
- Evidence
- Finding
- Incident
- Journey node/edge
- Recommendation
- Notification
- User feedback/disposition
- Health/coverage state

Bu liste final schema değildir; conceptual requirement'tır.

---

### Karar IM-02 — Evidence incident'tan bağımsız tutulmalı mı?

**Durum:** APPROVED

**A — Incident içine gömülü**
- Basit.
- Re-correlation/split zor.

**B — Evidence immutable-ish first-class object, incident references it**
- Merge/split/history mümkün.

**APPROVED REQUIREMENT:** **B.**

---

### Karar IM-03 — Incident lifecycle sabit state machine mi?

**Durum:** APPROVED

**A — Serbest labels**
- Esnek.
- Automation/reporting zor.

**B — Core lifecycle + optional labels**
Örnek:
New → Acknowledged → Investigating → Resolved
Disposition: Benign / Expected / Suspicious / Confirmed Incident / Unknown

**APPROVED REQUIREMENT:** **B.**

---

### Karar IM-04 — MITRE ATT&CK mapping first-class mı?

**Durum:** APPROVED

**A — Ana UI taxonomisi**
- Security pros sever.
- Persona için jargon.

**B — Secondary enrichment**
- Human-readable explanation primary.

**APPROVED REQUIREMENT:** **B**, Step 2 ile uyumlu.

---

### Karar IM-05 — Recommendation kendi provenance'ına sahip olmalı mı?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** **Evet.**
Her recommendation için:
- rule/knowledge/AI source
- supporting evidence
- assumptions
- confidence/limitations
- required approval
- reversibility/risk
tutulmalıdır.

---

# 10. Windows Feasibility

## Amaç

Windows-heavy customer environment Step 2'de doğal scope kabul edildi. Windows tarafında SMB, RDP, WinRM, NTLM/Kerberos, domain realism ve licensing Linux service emulation'dan farklı gereksinimler doğurur. MITRE ATT&CK bunları lateral movement için merkezi remote-service teknikleri olarak sınıflandırır. [R4]

### Karar WIN-01 — Windows deception first-class requirement mı?

**Durum:** APPROVED

**A — Sonraya bırak**
- MVP basitleşir.
- ICP'nin önemli bir bölümünde realism/coverage zayıflayabilir.

**B — Product architecture requirement olarak first-class; MVP scope ayrı karar**
- Step 3 requirement netleşir, MVP seçimi daha sonra.

**APPROVED REQUIREMENT:** **B.**

---

### Karar WIN-02 — Windows realism için emulation mı real Windows mı?

**Durum:** APPROVED

**A — Protocol/service emulation**
- Licensing/isolation kolay.
- Deep interaction sınırlı.

**B — Real Windows VM**
- RDP/SMB/OS behavior gerçekçi.
- Licensing, patch/image lifecycle, escape/egress risk.

**C — Layered**
- Low/medium protocol emulation default
- High-interaction için licensed real Windows capability

**APPROVED REQUIREMENT:** **C.**

---

### Karar WIN-03 — Decoy gerçek Active Directory domain'ine join edilmeli mi?

**Durum:** APPROVED

**A — Evet**
- Maximum realism/identity semantics.
- Production directory'ye riskli write/trust ilişkisi, compromise blast radius.

**B — Asla**
- Güvenli.
- Domain-aware behavior sınırlı.

**C — Default no production-domain join; separate controlled domain-deception strategies future**
- Decoy identity/addressing/artefacts ile realism oluşturulabilir.
- Production directory write ancak ayrıca risk-reviewed capability olabilir.

**APPROVED REQUIREMENT:** **C.**

---

### Karar WIN-04 — Fake AD users/computers production directory'de oluşturulabilir mi?

**Durum:** APPROVED

**A — Evet, deception için**
- Discovery realism güçlü.
- Credential/account lifecycle ve accidental use risk.

**B — Hayır**
- Güvenli ama identity deception zayıf.

**C — Future identity-deception capability; current requirement olarak explicit isolation/least privilege/rollback şartı**
- Önce threat/operational feasibility doğrulanır.

**APPROVED REQUIREMENT:** **C.**

---

### Karar WIN-05 — Windows licensing ürün requirement'ına yazılmalı mı?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** **Evet.**
Real Windows Server/Windows guest kullanımı “teknik olarak çalışıyor” ile bitmez. Entitlement, virtualization rights, customer-owned license vs bundled image ve redistribution hakları architecture/business model girdisidir. [R10]

**Karşıt görüş:** “Sonra hukuk bakar.” Yanlış; licensing capability feasibility'yi doğrudan etkiler.

---

### Karar WIN-06 — Windows telemetry neye dayanmalı?

**Durum:** APPROVED

**Seçenekler:**
- Emulated service telemetry
- Native Windows audit events
- Host instrumentation
- Network/context enrichment
- Kombinasyon

**APPROVED REQUIREMENT:** **Capability-specific combination.**
Önemli constraint: Windows event fields authentication protocolüne göre eksik olabilir; source attribution tek event field'a bağlanmamalıdır. [R9]

---

# 11. Credential & Identity Deception Feasibility

## Amaç

Honey credentials/tokens yüksek-sinyalli olabilir çünkü normal kullanıcıların kullanmaması beklenen artefact'lardır. Thinkst Canarytokens örnekleri AD login, AWS key, Azure identity, document, SQL, Slack API key, Windows folder gibi geniş deception yüzeyinin mümkün olduğunu gösterir. [R6]

### Karar CI-01 — Credential deception türleri tek seviyede mi?

**Durum:** APPROVED

**APPROVED taxonomy:**
1. **Non-authenticating token** — yalnız trigger üretir
2. **Synthetic credential** — gerçek production privilege yok
3. **Decoy-service credential** — yalnız decoy'a login olur
4. **Directory-visible fake identity** — discovery value
5. **Cloud/SaaS honey token** — provider-side trigger
6. **Real-but-contained credential** — en yüksek risk, özel capability

Risk level aynı değildir; UI bunu ayırmalıdır.

---

### Karar CI-02 — Honey credential gerçek production erişimi vermeli mi?

**Durum:** APPROVED

**A — Evet**
- Daha gerçekçi.
- Credential leak gerçek breach'e dönüşebilir.

**B — Hayır, zero production authority**
- Güvenli baseline.

**C — Exceptional contained credential**
- Yalnız özel high-interaction scenario.

**APPROVED REQUIREMENT:** **B default; C ancak future explicitly isolated capability.**

---

### Karar CI-03 — Credential/token placement nasıl yapılmalı?

**Durum:** APPROVED

**A — Random/dense**
- Touch probability artar.
- Operational confusion, accidental trigger.

**B — Contextual purposeful placement**
Örn fake config, admin note, connection profile, share, source-code-like artefact — yalnız gerçek environment bağlamında mantıklıysa.

**APPROVED REQUIREMENT:** **B.**

---

### Karar CI-04 — Production identity directory'sine write yapmak core requirement mı?

**Durum:** APPROVED

**A — Evet**
- Identity deception güçlü.
- High privilege/integrity risk.

**B — Hayır, optional advanced capability**
- Network-first ürün boundary korunur.

**APPROVED REQUIREMENT:** **B.**

---

### Karar CI-05 — Token lifecycle first-class mı?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** **Evet.**
Her token/credential için:
- created
- placed
- owner/location
- expected legitimate touch
- expiry/rotation
- revocation
- triggered
- stale/orphaned
durumları izlenmelidir.

---

# 12. High-Interaction / Vulnerable Systems Feasibility

## Amaç

High interaction daha zengin command/file/process/malware evidence'ı üretir; Cowrie proxy/QEMU ve tarihsel Honeynet çalışmalarında bunun pratik örnekleri vardır. Ancak gerçek vulnerable host compromise edildiğinde decoy artık saldırgan-controlled compute haline gelebilir. [R7][R13]

### Karar HI-01 — High interaction ürünün mandatory core capability'si mi?

**Durum:** APPROVED

**A — Evet**
- Differentiation güçlü.
- Risk ve operasyon ağır.

**B — Hayır, advanced capability**
- Core detection low/medium interaction ile başlar.
- Level C journey için her zaman zorunlu değildir.

**APPROVED REQUIREMENT:** **B**, Step 2 kararıyla uyumlu.

---

### Karar HI-02 — High-interaction isolation security boundary nasıl olmalı?

**Durum:** APPROVED

**A — Normal application sandbox yeter**
- Basit.
- Attacker kernel exploit/escape durumunda risk.

**B — Strong workload isolation + network isolation**
- VM/hypervisor veya eşdeğer güçlü boundary türleri architecture aşamasında değerlendirilir.
- NIST virtualization guidance isolation ve virtual network controls'ü temel güvenlik sorumluluğu sayar. [R8]

**APPROVED REQUIREMENT:** **B requirement**, fakat teknoloji seçilmez.

---

### Karar HI-03 — Outbound connectivity policy

**Durum:** APPROVED

**A — Serbest internet/internal egress**
- Attacker gerçek tool/C2 kullanabilir, intel artar.
- Customer/third-party harm riski kabul edilemez.

**B — Tam deny**
- Güvenli.
- Realism ve malware/tool observation sınırlı.

**C — Default deny + controlled/simulated/allowlisted egress**
- DNS/HTTP emulation, sinkhole, rate-limited destinations gibi yaklaşımlar daha sonra değerlendirilebilir.

**APPROVED REQUIREMENT:** **C.** High interaction'ın **müşteri ağına veya üçüncü taraflara saldırı platformuna dönüşmemesi** non-negotiable requirement.

---

### Karar HI-04 — Reset/rebuild nasıl ele alınmalı?

**Durum:** APPROVED

**A — Manual reset**
- Basit.
- Persistence contamination.

**B — Disposable/known-good reset capability**
- Compromise sonrası clean baseline'a dönmek gerekir.

**APPROVED REQUIREMENT:** **B.** Exact snapshot/image mechanism Adım 4 konusu.

---

### Karar HI-05 — Attacker-uploaded malware/file saklanabilir mi?

**Durum:** APPROVED

**A — Evet, normal file storage**
- Unsafe handling.

**B — Hiç saklama**
- Forensic value kaybı.

**C — Quarantined evidence object**
- Non-executable handling
- Restricted access
- hash/metadata
- optional controlled analysis later

**APPROVED REQUIREMENT:** **C.**

---

### Karar HI-06 — High-interaction decoy production network'e hangi yönde erişebilir?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** **Inbound lure/reachability ile outbound/pivot permission ayrılmalıdır.**
Attacker decoy'a ulaşabiliyor diye decoy'un production'a arbitrary connection yapabilmesi gerekmemelidir.

---

# 13. Fake-Network Redirection Feasibility

## Amaç

Ultimate vision'daki “attacker'ı ayrı fake network'e çekme” fikrinin teknik anlamını ayırmak. Tarihsel hybrid honeypot çalışmaları low→high interaction redirection'ın session state replay/sequence translation gibi ciddi complexity ürettiğini gösterir. [R13]

### Karar FR-01 — Fake-network redirection current core requirement mı?

**Durum:** APPROVED

**A — Evet**
- Vizyon doğrudan ürün olur.
- Step 3/4 complexity'yi dramatik artırır.

**B — Future advanced capability; architecture'da engellenmemesi gereken yön**
- Current product promise korunur.

**APPROVED REQUIREMENT:** **B.**

---

### Karar FR-02 — “Redirection” ne anlama gelmeli?

**Durum:** APPROVED

**A — Existing live session'ı transparently başka backend'e migrate etmek**
- En etkileyici.
- TCP/application state, crypto/session semantics zor.

**B — Attacker'a next-hop lure sunmak**
- Fake credentials/links/hosts ile attacker kendisi ilerler.
- Çok daha doğal ve güvenli.

**C — Network-level policy redirect yeni connections**
- Belirli destinations deception zone'a route edilir.

**APPROVED REQUIREMENT:** **B + C önce**, A future research item.

---

### Karar FR-03 — Session migration product requirement mı?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** **Hayır, current requirement değil.**
Bunu zorunlu tutmak architecture'ı çok erken daraltır. “Adaptive escalation” session-preserving olmak zorunda değildir.

---

### Karar FR-04 — Fake topology ne kadar tutarlı olmalı?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** Future capability için topology consistency first-class requirement:
- hostname/domain naming
- IP/subnet relationships
- credential relationships
- service dependencies
- file/content cross-references
- temporal state

LLM bu content'i üretebilir fakat consistency deterministic constraints ile validate edilmelidir.

---

### Karar FR-05 — Redirection safety constraint

**Durum:** APPROVED

**APPROVED REQUIREMENT:** Fake network **production trust boundary'den ayrılmalı**, attacker-controlled environment'tan real assets'e route/pivot default-deny olmalıdır. [R8][R14]

---

# 14. AI-Native Capability Feasibility

## Amaç

AI “üründe olsun” diye değil, belirli işleri daha iyi yapıyorsa kullanılmalıdır.

### Karar AF-01 — Incident explanation için AI gerekli mi?

**Durum:** APPROVED

**A — Rule/template yeter**
- Deterministic.
- Complex multi-evidence narrative zayıf.

**B — AI primary**
- Natural explanation güçlü.
- Outage/hallucination.

**C — Deterministic facts + AI narrative**
- Facts/evidence kod tarafından sağlanır.
- AI bunları anlaşılır özetler.

**APPROVED REQUIREMENT:** **C.** AI için en güçlü erken use case.

---

### Karar AF-02 — Incident correlation için AI gerekli mi?

**Durum:** APPROVED

**A — Hayır**
- Deterministic graph/rules yeterli olabilir.

**B — Evet**
- Soft semantic relationships.

**C — Deterministic baseline + AI hypothesis enrichment**
- Core correlation AI'sız çalışır.
- AI ambiguous cases için suggestion üretir.

**APPROVED REQUIREMENT:** **C.**

---

### Karar AF-03 — Remediation guidance için AI rolü

**Durum:** APPROVED

**A — Static playbooks**
- Güvenli ama generic.

**B — Free-form AI advice**
- Contextual ama dangerous/hallucinatory.

**C — Curated action library + AI contextualization**
- AI approved action primitives arasından açıklama/sıralama önerir.
- High-risk action human approval.

**APPROVED REQUIREMENT:** **C.**

---

### Karar AF-04 — Environment understanding için AI

**Durum:** APPROVED

**A — AI network modelini otomatik çıkarır**
- Flexible.
- False inference.

**B — Deterministic discovery + AI labeling/suggestion**
- AI “bu host muhtemelen DB” diyebilir; evidence separate.

**APPROVED REQUIREMENT:** **B.**

---

### Karar AF-05 — Deception persona/content generation için AI

**Durum:** APPROVED

**A — Core first use**
- Dynamic realism.
- Prompt/security consistency risk.

**B — Future assisted capability**
- Owner preview/approval
- deterministic validators

**APPROVED REQUIREMENT:** **B.**

---

### Karar AF-06 — Attacker-facing LLM

**Durum:** APPROVED

**A — İlk sürümde**
- Differentiation.
- Untrusted conversation, cost, jailbreak, realism, data leakage.

**B — Future high-interaction capability**
- Ayrı safety boundary ve budget control.

**APPROVED REQUIREMENT:** **B.**

---

# 15. AI Trust, Hallucination & Prompt-Injection Boundaries

## Amaç

Attacker commands, filenames, HTTP bodies, SQL strings ve uploaded documents **untrusted adversarial content**'tir. Bunları LLM context'ine koymak indirect prompt injection riskidir. OWASP bunu açık bir GenAI vulnerability sınıfı olarak tanımlar. [R11]

### Karar AT-01 — Attacker telemetry LLM'e “instruction” olarak gidebilir mi?

**Durum:** APPROVED

**A — Raw prompt içine doğrudan**
- Basit.
- Prompt injection.

**B — Untrusted data channel olarak structured/escaped**
- System policy attacker content'ten ayrılır.

**APPROVED REQUIREMENT:** **B non-negotiable.**

---

### Karar AT-02 — AI explanation evidence citation zorunlu mu?

**Durum:** APPROVED

**A — Hayır**
- UX sade.
- Trust düşük.

**B — Evet**
- Her önemli factual claim supporting evidence'a bağlanır.

**APPROVED REQUIREMENT:** **B.**

---

### Karar AT-03 — AI action authority seviyesi

**Durum:** APPROVED

**Tier 0:** Explain only  
**Tier 1:** Recommend  
**Tier 2:** Prepare action requiring approval  
**Tier 3:** Execute reversible low-risk action  
**Tier 4:** Execute containment/destructive action autonomously

**APPROVED REQUIREMENT:** Current product **Tier 1**, future kontrollü **Tier 2**. Tier 3/4 ayrıca owner decision gerektirir. Step 2 human-approval principle korunur. [R11][R12]

---

### Karar AT-04 — AI output free text mi structured contract mı?

**Durum:** APPROVED

**A — Free text**
- Flexible.
- Automation/validation zor.

**B — Structured output + user-facing narrative**
- Evidence IDs, recommendation IDs, uncertainty gibi fields validate edilebilir.

**APPROVED REQUIREMENT:** **B.**

---

### Karar AT-05 — Sensitive telemetry external AI provider'a gönderilebilir mi?

**Durum:** APPROVED

**A — Daima**
- Kolay.
- Privacy/customer trust risk.

**B — Asla**
- Offline-friendly.
- İlk ürün feasibility zorlaşabilir.

**C — Data classification + configurable policy**
- Redaction/minimization
- provider policy
- future local model option

**APPROVED REQUIREMENT:** **C.**

---

### Karar AT-06 — AI unavailable olduğunda product behavior

**Durum:** APPROVED

**APPROVED REQUIREMENT:** Detection/evidence/incident persistence/notification **çalışmaya devam eder**. AI explanation ve guidance degraded/queued olabilir. AI availability hiçbir zaman raw detection truth dependency'si olmamalıdır.

---

# 16. Notification Feasibility

## Amaç

High signal ürün promise'ini notification semantics bozabilir. Her raw event ayrı push/email olursa ürün SIEM noise problemine geri döner.

### Karar NO-01 — Notification unit

**Durum:** APPROVED

**A — Event**
- Immediate.
- Noise.

**B — Finding**
- Daha anlamlı.

**C — Incident/significant incident update**
- Correlated user outcome.

**APPROVED REQUIREMENT:** **C primary**, critical direct evidence için exceptional immediate signal değerlendirilebilir.

---

### Karar NO-02 — Notification channels product modelde provider-specific mi?

**Durum:** APPROVED

**A — Email/Slack/Teams vb. ayrı logic**
- Erken coupling.

**B — Abstract notification destination/capability**
- Provider adapters daha sonra seçilir.

**APPROVED REQUIREMENT:** **B.**

---

### Karar NO-03 — Escalation semantics gerekli mi?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** **Evet**, fakat full SOAR değildir.
Concepts:
- primary contact
- backup/escalation contact
- severity/confidence threshold
- acknowledgement timeout semantics
- maintenance/snooze

Exact channels/cadence MVP aşamasında seçilebilir.

---

### Karar NO-04 — Dedup/rate control zorunlu mu?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** **Evet.**
Brute-force gibi çok event tek incident update olmalıdır; OpenCanary correlator bu pattern'in pratik örneğidir. [R5]

---

### Karar NO-05 — Notification acknowledgement incident state'iyle bağlı mı?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** **Evet.** “Mesaj gönderildi” ile “insan gördü” farklı state'lerdir.

---

# 17. On-Prem / Internet Dependency Model

## Amaç

İlk ürün internet kullanabilir; ancak security product'ın detection capability'sinin external connectivity failure ile tamamen kaybolması kabul edilebilir değildir.

### Karar OP-01 — Hangi fonksiyonlar local survivability requirement taşımalı?

**Durum:** APPROVED

**APPROVED local-core:**
- deception interaction
- raw event capture
- evidence creation
- basic deterministic detection
- health visibility
- local buffering/persistence

Cloud-assisted olabilir:
- AI narrative/guidance
- optional enrichment
- remote management/analytics
- update distribution

---

### Karar OP-02 — Cloud control plane zorunlu olabilir mi?

**Durum:** APPROVED

**A — Evet**
- SaaS simplicity.
- Offline/future on-prem path zorlaşır.

**B — Hayır**
- Full on-prem complexity.

**C — Product model control/data functions'i ayrıştırmalı; initial deployment seçimi sonra**
- Offline future vision açık kalır.

**APPROVED REQUIREMENT:** **C.**

---

### Karar OP-03 — Internet outage behavior

**Durum:** APPROVED

**APPROVED REQUIREMENT:** Store-and-forward/degraded-mode semantics requirement:
- detection continues
- telemetry lost olmamalı (capacity limits visible)
- AI/cloud enrichment gecikebilir
- operator health warning almalı

Exact buffering technology later.

---

### Karar OP-04 — Remote management plane erişimi inbound mu outbound-initiated mı?

**Durum:** APPROVED

Bu architecture kararıdır; Step 3 requirement:
- customer firewall exposure minimum
- authentication strong
- management plane attacker-facing decoy network'ten ayrılmış
- offline/local admin path mümkün

**APPROVED REQUIREMENT:** Inbound public management exposure **zorunlu requirement yapılmamalı**; safer connection models Adım 4'te karşılaştırılmalı.

---

### Karar OP-05 — Updates internet dependency'si

**Durum:** APPROVED

**APPROVED REQUIREMENT:** Online automatic update + offline/manual package path gelecekte desteklenebilir product requirement olarak korunmalı. Update authenticity security requirement'tır.

---

# 18. Security of the Security Product

## Amaç

Ürün bilerek attacker interaction kabul eder. Bu nedenle kendi threat model'i sıradan SaaS uygulamasından daha sert olmalıdır.

### Karar PS-01 — Trust zones tanımlanmalı mı?

**Durum:** APPROVED

**APPROVED minimum conceptual zones:**
1. Attacker-facing deception plane
2. Management/control plane
3. Telemetry/evidence plane
4. AI/external integration boundary
5. Customer production network
6. High-interaction containment zone

Bunların aynı host/process/networkte olup olmayacağı architecture kararıdır; fakat **trust separation requirement** Step 3'te kabul edilmelidir.

---

### Karar PS-02 — Management plane decoy interface'lerinden erişilebilir mi?

**Durum:** APPROVED

**A — Aynı interface/port family**
- Kolay.
- Discovery/attack surface.

**B — Logically/cryptographically/access-control olarak ayrılmış management path**
- Güvenli.

**APPROVED REQUIREMENT:** **B.**

---

### Karar PS-03 — Decoy compromise threat modelde “expected” mi?

**Durum:** APPROVED

**A — Exceptional bug**
- Yanlış varsayım.

**B — Medium/high interaction için expected possibility**
- Security architecture containment'a dayanır.

**APPROVED REQUIREMENT:** **B.** “Decoy asla compromise olmaz” güvenlik varsayımı yasaktır.

---

### Karar PS-04 — Telemetry tampering

**Durum:** APPROVED

**APPROVED REQUIREMENT:** Compromised decoy'un local log'u tek truth source olmamalı. Evidence collection mümkün olduğunca decoy attacker control plane'inden ayrıştırılmalı veya tamper evidence sağlamalı.

---

### Karar PS-05 — Secrets management

**Durum:** APPROVED

**APPROVED principles:**
- Production secret decoy'a kopyalanmaz
- Honey secret clearly classified
- Management credentials attacker-facing filesystem/content'te bulunmaz
- Rotation/revocation
- Least privilege
- AI prompt/context içinde secret tutulmaz [R11]

---

### Karar PS-06 — Update/supply-chain security requirement

**Durum:** APPROVED

**APPROVED REQUIREMENT:**
- authenticated/signed update provenance
- rollback/recovery
- component inventory/provenance
- update failure visibility
- compromised update channel threat model

Exact signing/packaging technology later.

---

### Karar PS-07 — DoS / resource abuse

**Durum:** APPROVED

Attacker:
- connection flood
- disk fill via uploads
- log explosion
- high-interaction CPU abuse
- LLM denial-of-wallet
üretebilir.

**APPROVED REQUIREMENT:** Quotas/backpressure/rate/budget boundaries product security requirement.

---

### Karar PS-08 — AI prompt injection product threat modelinde first-class mı?

**Durum:** APPROVED

**APPROVED REQUIREMENT:** **Evet.**
Attacker telemetry adversarial prompt olarak ele alınmalıdır; model tools/privileges least privilege olmalı ve high-risk action human approval gerektirmelidir. [R11]

---

# 19. Privacy & Data Handling

## Amaç

Honeypot telemetry'si saldırgan verisi gibi görünse de müşteri employee usernames, internal hostnames, credentials, documents ve accidental legitimate interactions içerebilir.

### Karar PR-01 — Data minimization principle

**Durum:** APPROVED

**A — Her şeyi topla**
- Forensics maximum.
- Privacy/risk/storage maximum.

**B — Sadece alert fields**
- Güvenli.
- Investigation zayıf.

**C — Purpose-limited capability-specific collection**
- Her field “hangi product/security job için gerekli?” sorusunu geçmeli.

**APPROVED REQUIREMENT:** **C.**

---

### Karar PR-02 — Captured credentials nasıl tutulmalı?

**Durum:** APPROVED

**A — Plaintext**
- Investigation kolay.
- High sensitivity.

**B — Asla saklama**
- Credential abuse evidence kaybı.

**C — Type-specific handling**
- username gerekebilir
- password secret/redacted/hash/protected representation
- honey credential known-reference olarak eşleştirilebilir
- real customer credential exposure minimize edilir

**APPROVED REQUIREMENT:** **C.** Exact cryptographic handling architecture/security design konusu.

---

### Karar PR-03 — Uploaded files/content

**Durum:** APPROVED

**APPROVED REQUIREMENT:** Default UI'da doğrudan render/execute edilmemeli. Malware/HTML/document content hostile kabul edilmeli; metadata/hash ve quarantined access modeli gereklidir.

---

### Karar PR-04 — Retention

**Durum:** APPROVED

**A — Forever**
- Intel.
- Cost/privacy.

**B — Çok kısa**
- Investigation kaybı.

**C — Data-class-specific configurable retention**
- Raw payload daha kısa
- normalized evidence/incident daha uzun olabilir

**APPROVED REQUIREMENT:** **C.** Exact süreler validation/compliance/customer needs aşamasında.

---

### Karar PR-05 — Cloud/AI transfer policy

**Durum:** APPROVED

**APPROVED REQUIREMENT:** Data classification + redaction + customer-configurable external-processing policy. “AI için tüm raw session cloud'a gider” default assumption olmamalıdır.

---

### Karar PR-06 — Future multi-tenancy isolation

**Durum:** APPROVED

Current scope single-customer olsa da evidence model customer/environment ownership bilgisi taşımalıdır; fakat full tenant authorization architecture şimdi seçilmez.

**APPROVED REQUIREMENT:** Future isolation'ı imkânsızlaştırmayacak ownership semantics; current MSSP complexity eklenmez.

---

# 20. Operational Feasibility

## Amaç

“Babysitting gerektirmeyen security product” yalnız UI sadeliği değildir. Deception'ın gerçekten reachable, healthy, non-stale ve safe olduğunu sistem kendi başına gösterebilmelidir.

### Karar OF-01 — Health yalnız process-up/down mı?

**Durum:** APPROVED

**A — Service running**
- Yetersiz.

**B — Functional health**
- process
- network reachability
- protocol response
- telemetry pipeline
- storage capacity
- management connectivity
- time sync
- decoy safety controls

**APPROVED REQUIREMENT:** **B.**

---

### Karar OF-02 — Update philosophy

**Durum:** APPROVED

**A — Manual only**
- Control.
- Security patch burden.

**B — Forced automatic**
- Secure freshness.
- Customer outage/change risk.

**C — Safe managed updates + policy/rollback visibility**
- Default automation
- maintenance controls
- signed provenance
- health check/rollback

**APPROVED REQUIREMENT:** **C.**

---

### Karar OF-03 — Stale deception nasıl tanınır?

**Durum:** APPROVED

Examples:
- Environment'da artık olmayan OS/persona
- IP conflict
- DNS/hostname inconsistency
- expired token
- moved subnet
- unreachable service
- obsolete fake data

**APPROVED REQUIREMENT:** “Staleness” first-class health dimension olmalı; system refresh/replacement önerir, autonomous production-impacting change yapmaz.

---

### Karar OF-04 — Configuration drift

**Durum:** APPROVED

**A — User config current truth**
- External network changes kaçabilir.

**B — Desired state vs observed state**
- Coverage difference görünür.

**APPROVED REQUIREMENT:** **B conceptual requirement.**

---

### Karar OF-05 — Backup/recovery neyi kapsamalı?

**Durum:** APPROVED

**APPROVED separation:**
- Product configuration
- Environment model
- Deception definitions
- Incident/evidence history
- Secrets
- High-interaction disposable state

Bunların hepsi aynı recovery policy'ye sahip olmamalıdır.

---

### Karar OF-06 — Decoy lifecycle

**Durum:** APPROVED

**APPROVED lifecycle:**
Proposed → Approved → Deploying → Healthy → Degraded → Stale → Disabled → Retired

High-interaction için ayrıca:
Clean → Engaged → Compromised/Assumed Compromised → Quarantined → Reset

---

### Karar OF-07 — Operator'a hangi operational surface gösterilmeli?

**Durum:** APPROVED

**A — Çok teknik metrics dashboard**
- SOC/DevOps burden.

**B — Sadece green/red**
- Root cause yetersiz.

**C — Outcome-oriented health**
Örn:
- “3 subnet'ten 2'sinde coverage healthy”
- “Windows decoy unreachable”
- “Telemetry buffered; cloud AI unavailable”
- “Token stale”
- “High-interaction environment quarantined”

**APPROVED REQUIREMENT:** **C.**

---

# 21. Cross-Cutting Feasibility Decisions

Bu bölüm yukarıdaki alanlar arasında tekrarlanan ve architecture'a doğrudan requirement olacak kararları tek yerde toplar.

## Karar X-01 — Deterministic core boundary

**Durum:** APPROVED

**APPROVED REQUIREMENT:**
Aşağıdakiler AI olmadan çalışabilmelidir:
- telemetry ingestion
- evidence provenance
- basic detection
- health
- incident persistence
- alert/notification baseline
- owner configuration

AI optional reasoning/enrichment layer'dır.

---

## Karar X-02 — Evidence vs inference separation

**Durum:** APPROVED

**APPROVED REQUIREMENT:**
UI/data model:
- Observed fact
- Deterministically derived fact
- Correlated hypothesis
- AI inference
- User-confirmed fact

ayrımını kaybetmemelidir.

---

## Karar X-03 — Safe-by-default placement

**Durum:** APPROVED

**APPROVED REQUIREMENT:** Yeni deception capability default olarak:
- minimum privilege
- no production secrets
- restricted egress
- explicit management separation
- health verification
- reversible lifecycle
ilkelerini sağlamalıdır.

---

## Karar X-04 — Coverage over decoy count

**Durum:** APPROVED

**APPROVED REQUIREMENT:** Teknik quality metric “kaç honeypot var?” değil:
- hangi trust zones covered
- hangi threat behaviors have touchpoints
- decoys reachable/healthy mi
- evidence quality yeterli mi

olmalıdır.

---

## Karar X-05 — Architecture extensibility requirement sınırı

**Durum:** APPROVED

**Seçenek A — Future bütün vision'u şimdiden generic platform olarak tasarla**
- Over-engineering.

**Seçenek B — Yalnız current capability**
- Future endpoint/identity/high-interaction path kapanabilir.

**APPROVED REQUIREMENT:** **Bounded extensibility:** current network deception use case optimize edilir, ancak conceptual interfaces subtype/distributed presence/evidence provenance gibi bilinen future requirements'ı bloke etmemelidir.

---

# 22. Consolidated APPROVED Decision Register

Aşağıdaki 113 kararın tamamı Product Owner tarafından önerilen yönleriyle APPROVED edilmiştir.

| ID | Karar konusu | APPROVED yön | Statü |
|---|---|---|---|
| NV-01 | Observation topology | Hybrid central + distributed presence when needed | APPROVED |
| NV-02 | Visibility boundary | Deception-first + contextual network metadata | APPROVED |
| NV-03 | Segment presence | Capability-specific routed/local presence | APPROVED |
| NV-04 | NAT/VPN source identity | Composite evidence-backed source context | APPROVED |
| NV-05 | Placement validation | Functional coverage verification | APPROVED |
| DD-01 | Deception object model | Common abstraction + subtypes | APPROVED |
| DD-02 | Realism | Minimum sufficient realism | APPROVED |
| DD-03 | Privilege | Capability-specific least privilege tiers | APPROVED |
| DD-04 | Production co-hosting | Interactive deception separated from production workloads | APPROVED |
| DD-05 | Safety profile | First-class per deception object | APPROVED |
| IL-01 | Default interaction | Layered low→medium→high | APPROVED |
| IL-02 | Escalation | Preconfigured now; controlled adaptive future | APPROVED |
| IL-03 | Emulation sufficiency | Threat/evidence objective based | APPROVED |
| IL-04 | High-interaction criterion | Only when real execution/state is required | APPROVED |
| EU-01 | Environment understanding | Hybrid minimum sufficient context | APPROVED |
| EU-02 | Active discovery | Bounded + explicit consent | APPROVED |
| EU-03 | Passive discovery | Optional enrichment | APPROVED |
| EU-04 | Owner correction | System suggestion + owner override | APPROVED |
| EU-05 | Drift | Staleness awareness/revalidation | APPROVED |
| SI-01 | Attribution promise | Source identification, not person attribution | APPROVED |
| SI-02 | Source model | Evidence hierarchy/composite identity | APPROVED |
| SI-03 | Endpoint enrichment | Supported future enrichment, not mandatory current | APPROVED |
| SI-04 | Uncertainty | Explicit confidence/provenance | APPROVED |
| TE-01 | Evidence contract | Canonical normalized evidence model | APPROVED |
| TE-02 | Capture depth | Capability-specific | APPROVED |
| TE-03 | Raw payload | Selective policy | APPROVED |
| TE-04 | Time | Time quality first-class | APPROVED |
| TE-05 | Integrity | Tamper-evident provenance baseline | APPROVED |
| CR-01 | Correlation key | Multi-evidence graph | APPROVED |
| CR-02 | Correlation logic | Deterministic anchors + probabilistic hypotheses | APPROVED |
| CR-03 | LLM correlation | Hypothesis layer, not truth engine | APPROVED |
| CR-04 | Journey model | Observed + context + explicit inference | APPROVED |
| CR-05 | Merge/split | Reversible owner correction | APPROVED |
| CF-01 | Confidence meaning | Evidence support strength | APPROVED |
| CF-02 | Confidence UI | Level + factor breakdown | APPROVED |
| CF-03 | Allowlist | Capture + contextual suppression | APPROVED |
| CF-04 | Feedback learning | Reversible scoped rule proposals | APPROVED |
| CF-05 | Severity | Separate from confidence | APPROVED |
| IM-01 | Domain objects | Explicit conceptual model | APPROVED |
| IM-02 | Evidence ownership | First-class independent evidence | APPROVED |
| IM-03 | Lifecycle | Core state machine + disposition | APPROVED |
| IM-04 | ATT&CK | Secondary enrichment | APPROVED |
| IM-05 | Guidance provenance | Evidence/assumption/authority metadata | APPROVED |
| WIN-01 | Windows scope | First-class requirement, MVP later | APPROVED |
| WIN-02 | Windows realism | Layered emulation + real Windows high interaction | APPROVED |
| WIN-03 | Domain join | No production-domain join by default | APPROVED |
| WIN-04 | Directory deception | Future controlled advanced capability | APPROVED |
| WIN-05 | Licensing | First-class feasibility constraint | APPROVED |
| WIN-06 | Windows telemetry | Capability-specific combination | APPROVED |
| CI-01 | Credential types | Risk-tiered taxonomy | APPROVED |
| CI-02 | Real authority | No production authority by default | APPROVED |
| CI-03 | Placement | Contextual purposeful | APPROVED |
| CI-04 | Directory writes | Optional advanced, not core | APPROVED |
| CI-05 | Lifecycle | First-class token/credential lifecycle | APPROVED |
| HI-01 | High interaction role | Advanced capability | APPROVED |
| HI-02 | Isolation | Strong workload + network isolation requirement | APPROVED |
| HI-03 | Egress | Default deny + controlled/simulated exceptions | APPROVED |
| HI-04 | Reset | Disposable/known-good recovery | APPROVED |
| HI-05 | Malware storage | Quarantined evidence | APPROVED |
| HI-06 | Production access | Reachability ≠ pivot permission | APPROVED |
| FR-01 | Fake-network redirection | Future advanced capability | APPROVED |
| FR-02 | Redirection model | Lure/new-flow redirect before live session migration | APPROVED |
| FR-03 | Session migration | Not current requirement | APPROVED |
| FR-04 | Fake topology | Consistency constraints | APPROVED |
| FR-05 | Redirection safety | Production trust separation | APPROVED |
| AF-01 | AI explanation | Deterministic facts + AI narrative | APPROVED |
| AF-02 | AI correlation | Deterministic baseline + AI hypotheses | APPROVED |
| AF-03 | AI remediation | Curated actions + contextual AI | APPROVED |
| AF-04 | AI environment model | Deterministic context + AI labeling | APPROVED |
| AF-05 | AI persona generation | Future assisted capability | APPROVED |
| AF-06 | Attacker-facing LLM | Future high-interaction capability | APPROVED |
| AT-01 | Untrusted telemetry | Structured untrusted-data boundary | APPROVED |
| AT-02 | AI citations | Evidence-linked claims | APPROVED |
| AT-03 | AI authority | Recommend now; prepare-with-approval future | APPROVED |
| AT-04 | AI output | Structured contract + narrative | APPROVED |
| AT-05 | External AI data | Classification/redaction/configurable policy | APPROVED |
| AT-06 | AI outage | Core detection continues | APPROVED |
| NO-01 | Notification unit | Incident/significant update | APPROVED |
| NO-02 | Channel model | Provider-agnostic destination abstraction | APPROVED |
| NO-03 | Escalation | Lightweight incident escalation semantics | APPROVED |
| NO-04 | Dedup | Mandatory | APPROVED |
| NO-05 | Acknowledgement | Linked to incident lifecycle | APPROVED |
| OP-01 | Local survivability | Detection/evidence/health local-core | APPROVED |
| OP-02 | Cloud dependency | Control/data functions separable | APPROVED |
| OP-03 | Internet outage | Store-and-forward/degraded mode | APPROVED |
| OP-04 | Remote management | No mandatory inbound public exposure | APPROVED |
| OP-05 | Updates | Online + future offline path | APPROVED |
| PS-01 | Trust zones | Explicit conceptual separation | APPROVED |
| PS-02 | Management plane | Separate protected path | APPROVED |
| PS-03 | Decoy compromise | Expected possibility | APPROVED |
| PS-04 | Telemetry tamper | Independent/tamper-evident collection | APPROVED |
| PS-05 | Secrets | No production secrets in deception plane | APPROVED |
| PS-06 | Supply chain | Authenticated update provenance + recovery | APPROVED |
| PS-07 | Resource abuse | Quota/backpressure/budget boundaries | APPROVED |
| PS-08 | Prompt injection | First-class product threat | APPROVED |
| PR-01 | Data minimization | Purpose-limited collection | APPROVED |
| PR-02 | Credential data | Type-specific protected handling | APPROVED |
| PR-03 | Uploaded files | Hostile/quarantined handling | APPROVED |
| PR-04 | Retention | Data-class-specific configurable | APPROVED |
| PR-05 | Cloud transfer | Classification + redaction + policy | APPROVED |
| PR-06 | Future tenancy | Ownership semantics without current MSSP complexity | APPROVED |
| OF-01 | Health | Functional coverage health | APPROVED |
| OF-02 | Updates | Managed safe updates + rollback visibility | APPROVED |
| OF-03 | Staleness | First-class health dimension | APPROVED |
| OF-04 | Drift | Desired vs observed state | APPROVED |
| OF-05 | Recovery | State-class-specific backup/recovery | APPROVED |
| OF-06 | Lifecycle | Explicit deception/high-interaction lifecycle | APPROVED |
| OF-07 | Ops UX | Outcome-oriented health | APPROVED |
| X-01 | Deterministic core | AI-independent core | APPROVED |
| X-02 | Evidence/inference | Explicitly separated | APPROVED |
| X-03 | Safe default | Least privilege/no prod secret/restricted egress | APPROVED |
| X-04 | Coverage metric | Coverage/evidence quality over decoy count | APPROVED |
| X-05 | Extensibility | Bounded extensibility | APPROVED |

---

# 23. Validation & Research Backlog — OPEN BY DESIGN

Bu maddeler Step 3 owner decision'larından bağımsızdır. Product Owner yönü onaylamış olsa da bu hipotezler prototip, lab, customer discovery, security testing veya focused research ile doğrulanmalıdır. Bu bölümün açık olması Step 3'te açık karar kaldığı anlamına gelmez.

## Network / placement validation
- Flat LAN, routed VLAN, site-to-site VPN, remote-access VPN ve cloud private network senaryolarında reachability
- Multi-segment presence ihtiyacının gerçek customer topolojilerinde sıklığı
- Decoy fingerprinting'in farklı interaction seviyelerinde pratik etkisi
- Placement önerisinin minimum context ile kalitesi

## Source identification validation
- IP → device mapping reliability
- DHCP churn/NAT/VPN/jump-host ambiguity
- Windows auth protocolüne göre available source fields
- Endpoint integration olmadan “source clarity”nin kullanıcı için yeterliliği

## Detection/confidence validation
- Common SMB vulnerability scanners/monitoring/inventory tools'un false-positive davranışı
- Honey credential interaction'ın accidental legitimate trigger oranı
- Multi-decoy journey correlation precision/recall
- Confidence explanation'ın primary persona tarafından anlaşılması

## High-interaction validation
- Strong containment overhead
- Egress simulation ile attacker realism dengesi
- Windows high-interaction licensing/operations
- Compromise reset/rebuild güvenilirliği
- Uploaded malware handling safety

## AI validation
- AI narrative factuality
- Evidence-citation adherence
- Remediation recommendation safety
- Indirect prompt injection resistance
- Raw attacker content redaction strategy
- Provider outage/degraded-mode UX
- Cost/denial-of-wallet behavior
- Attacker-facing LLM realism vs fingerprinting

## Operations validation
- “Babysitting yok” beklentisinin ölçülebilir karşılığı
- Update failure/rollback scenarios
- Network drift/stale decoy detection
- Health UX'nin generalist operator için yeterliliği

---

# 24. Approved Risk Baseline

| Risk | Neden önemli | Etki | Draft mitigation direction |
|---|---|---|---|
| False confidence | Decoy hit yanlış actor'a bağlanabilir | Yanlış containment | Evidence provenance + uncertainty |
| False positives | Scanner/admin automation | Alert fatigue | Contextual suppression |
| Coverage illusion | Decoy unreachable/stale | Silent detection gap | Functional health validation |
| Fingerprinting | Attacker decoy'u tanır | Evasion | Minimum sufficient realism + diversity |
| Decoy escape | High interaction compromise | Customer breach | Strong isolation |
| Outbound abuse | Compromised decoy saldırı yapar | Third-party/customer harm | Default-deny egress |
| Management compromise | Security tool attacker'a kapı olur | Critical | Plane separation + least privilege |
| Telemetry tampering | Attacker evidence siler/değiştirir | Investigation corruption | Independent/tamper-evident collection |
| Credential leakage | Honey/real secret karışır | Real access | Zero production authority default |
| Windows licensing | Real VM redistribution/use constraints | Product feasibility | Early licensing review |
| Data privacy | Commands/files/users sensitive olabilir | Trust/legal | Purpose-limited collection |
| Malware handling | Uploaded file operator'a zarar verir | Endpoint compromise | Quarantine/non-executable access |
| AI hallucination | Yanlış explanation/action | Wrong decision | Evidence grounding + structured outputs |
| Prompt injection | Attacker telemetry AI'ı manipüle eder | Integrity/agency breach | Untrusted-data boundary |
| Denial-of-wallet | Attacker LLM kullanımını şişirir | Cost/availability | Budgets/quotas |
| Cloud dependency | Internet/provider outage | Detection degradation | AI-independent local core |
| Complexity creep | NDR/SIEM/SOAR'a dönüşme | Product failure | Non-goals + bounded extensibility |
| Over-automation | Yanlış containment | Business outage | Human approval |
| Environment drift | Persona/placement stale | Low realism/coverage | Revalidation |
| Multi-segment complexity | Sensor sprawl | Operational burden | Progressive deployment |

---

# 25. Step 3 Closure / Exit Criteria

Step 3 aşağıdaki koşullarla **CLOSED / APPROVED** kabul edilir:

- [x] 113 owner-decision maddesinin tamamı APPROVED
- [x] Network visibility/placement requirements kapalı
- [x] Deception deployment/interaction requirements kapalı
- [x] Environment understanding boundary kapalı
- [x] Source identification promise ve evidence hierarchy kapalı
- [x] Telemetry/evidence canonical requirements kapalı
- [x] Correlation/journey model kapalı
- [x] Confidence/severity/false-positive model kapalı
- [x] Incident information model kapalı
- [x] Windows capability boundaries kapalı
- [x] Credential/identity deception safety boundaries kapalı
- [x] High-interaction containment requirements kapalı
- [x] Fake-network redirection current/future boundary kapalı
- [x] AI use-case boundaries kapalı
- [x] AI trust/prompt-injection/human authority boundaries kapalı
- [x] Notification semantics kapalı
- [x] On-prem/internet survivability requirements kapalı
- [x] Product security trust boundaries kapalı
- [x] Privacy/data-handling principles kapalı
- [x] Operational health/lifecycle requirements kapalı
- [x] Validation backlog owner decisions'dan ayrı tutuldu
- [x] Architecture/technology seçimleri bilinçli olarak yapılmadı

**Sonuç:** Step 3'te açık `OWNER DECISION` bulunmamaktadır. Validation ve research backlog maddeleri sonraki teknik doğrulama faaliyetlerinin girdisidir; karar açığı değildir.

---

# 26. Step 4 Handoff — Architecture & Technology Decision Inputs

Step 3 kararları teknoloji seçmeden aşağıdaki bağlayıcı mimari girdileri üretmiştir.

## 26.1 Network / deployment

- Basit networklerde tek presence point ile başlayabilme
- İhtiyaç halinde multi-segment/distributed presence destekleyebilme
- Deception-first visibility; contextual telemetry'nin supporting role olması
- Routed ve local-segment deception modellerini capability bazında destekleyebilme
- Deployment/coverage doğrulamasının functional health parçası olması

## 26.2 Control, deception ve trust boundaries

- Attacker-facing deception plane ile management/control plane ayrımı
- Telemetry/evidence plane'in attacker-controlled workload'dan korunması
- High-interaction containment zone'un production trust boundary'den ayrılması
- Management exposure'ın minimum ve güvenli olması
- Production secrets'in deception plane'e taşınmaması

## 26.3 Evidence & incident architecture

- Canonical normalized evidence contract
- Raw event, evidence, finding, incident ve inference ayrımı
- Evidence provenance ve tamper-evident semantics
- Evidence'ın incident'tan bağımsız first-class varlık olması
- Multi-evidence correlation graph
- Journey üzerinde observed / correlated / inferred / user-confirmed ayrımı
- Time quality ve distributed clock awareness

## 26.4 Deception execution

- Common deception abstraction + capability subtype'ları
- Capability-specific privilege ve safety profile
- Layered low/medium/high interaction
- High interaction için strong workload/network isolation
- Default-deny egress ve controlled exceptions
- Disposable / known-good reset
- Hostile file/malware quarantine

## 26.5 Windows / identity

- Windows deception first-class architecture requirement
- Emulation + real Windows high-interaction için katmanlı yaklaşım
- Production AD join/write default olmaması
- Licensing/entitlement'ın architecture/business constraint olması
- Credential/token lifecycle ve zero-production-authority default'u

## 26.6 AI architecture

- Deterministic core AI'dan bağımsız çalışır
- AI truth engine değil reasoning/hypothesis/narrative katmanıdır
- Evidence-linked AI claims
- Structured AI output contract
- Attacker telemetry untrusted/adversarial input kabul edilir
- External AI processing data classification/redaction policy'sine bağlıdır
- Current AI authority: recommend; future prepare-with-approval

## 26.7 Resilience / operations

- Internet/AI outage sırasında local detection/evidence/health devam eder
- Store-and-forward/degraded mode
- Functional health, staleness ve desired-vs-observed-state
- Managed signed update provenance ve recovery/rollback requirement
- Resource quotas/backpressure ve denial-of-wallet korumaları

## 26.8 Bilinçli olarak Adım 4'e bırakılan seçimler

Aşağıdakiler **OPEN architecture/technology decisions** olarak Adım 4'e aktarılır:

- Control plane / data plane fiziksel ve logical topology
- Agent/sensor/controller process modeli
- Container vs VM vs başka isolation implementation
- Hypervisor/runtime seçimi
- Programming languages ve frameworks
- Data store ve event transport
- Canonical schema'nın fiziksel representation'ı
- Correlation execution architecture
- Local vs cloud control-plane deployment biçimi
- Network connectivity/overlay yaklaşımı
- Windows image/runtime yaklaşımı
- Secret-management implementation
- Update/signing implementation
- Observability stack
- AI provider/model abstraction ve provider seçimi
- Notification provider implementations
- OSS build-vs-integrate kararları

Bu seçimlerin hiçbirinin cevabı Step 3 tarafından önceden belirlenmiş sayılmaz; fakat yukarıdaki approved requirements ile uyumlu olmak zorundadır.

---

# 27. Final Step 3 Conclusion

Bu çalışmanın temel teknik tezi şudur:

> Ürünün sürdürülebilir differentiation'ı “çok sayıda honeypot çalıştırmak” değil; güvenli ve amaçlı deception placement, güvenilir evidence provenance, explainable correlation, source uncertainty'nin dürüst yönetimi, attacker journey ve primary persona'ya uygun response guidance kombinasyonudur.

En kritik feasibility gerilimleri:

1. **Simplicity ↔ distributed network reality**
2. **Realism ↔ containment risk**
3. **Rich telemetry ↔ privacy/operational cost**
4. **Attacker journey ↔ attribution uncertainty**
5. **AI value ↔ hallucination/prompt injection**
6. **High interaction ↔ safe egress**
7. **Windows realism ↔ licensing/operations**
8. **Automation ↔ human control**
9. **Cloud assistance ↔ local survivability**
10. **Future extensibility ↔ present complexity**

Bu gerilimlerin hiçbirinin cevabı teknoloji seçimi değildir. Önce Product Owner seviyesinde **requirement ve risk appetite** kararı verilmelidir. Bu gerilimler için Product Owner seviyesindeki requirement ve risk-appetite kararları artık kapanmıştır. Bir sonraki aşama, bu onaylı gereksinimleri karşılayacak sistem mimarisi ve teknoloji seçeneklerini araştırmak, karşılaştırmak ve ayrı ayrı owner decision süreciyle karara bağlamaktır.


---

# 28. Document Control

- **Document:** `Technical_Feasibility_Requirements.md`
- **Step:** 3 — Technical Feasibility & Requirements
- **Status:** APPROVED / FINAL
- **Approved decision count:** 113
- **Open owner-decision count:** 0
- **Validation/research backlog:** OPEN BY DESIGN
- **Architecture/technology decisions:** DEFERRED TO STEP 4
- **MVP/roadmap decisions:** NOT PART OF STEP 3
- **Decision authority:** Product Owner
- **Default artifact format:** Markdown
