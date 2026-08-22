# Step 2 — Product Definition & Strategy
## APPROVED Decision Record

**Belge türü:** Owner-approved product decision record  
**Durum:** APPROVED — Step 2 karar seti kapalı  
**Kaynak çalışma:** *Product Definition & Strategy — Draft Study*  
**Son güncelleme:** 2026-08-22

---

## 0. Belgenin amacı

Bu belge, Step 2 kapsamında tartışılan **Product Definition, Threat Model ve Product Strategy** kararlarının Product Owner tarafından onaylanmış halini kaydeder.

Karar yönetimi standardı:

`OPEN → RESEARCHED → RECOMMENDED → OWNER DECISION → APPROVED`

Bu turda Product Owner:

- 2.1–2.6 arasındaki tüm karar maddelerinde önerilen seçimleri açıkça kabul etti.
- 2.7–2.12 arasındaki **tüm** karar maddeleri ve önerileri eksiksiz kabul etti.
- Draft Study içerisindeki tüm Step 2 önerilerini onayladı.
- Step 2 çerçevesini onayladı.
- Özellikle istenmedikçe çalışma/artifact formatı olarak **Markdown** kullanılmasını istedi.

### Önemli ayrım

**Step 2 kararlarının kapanmış olması, bütün ürün hipotezlerinin kanıtlandığı anlamına gelmez.**

- `APPROVED`: ürün yönü/ilke/karar olarak karara bağlanmıştır.
- `VALIDATION NEEDED`: kullanıcı/market/MVP doğrulamasıyla test edilmesi gereken varsayımdır.
- `DEFERRED`: bilinçli olarak Step 3, Step 4 veya MVP/roadmap aşamasına bırakılmış karardır.

Validation-needed varsayımlar ve riskler açık owner-decision değildir; sonraki çalışmaların doğrulama girdileridir.

## 0.1 Step 2 dışında bırakılan ve hâlâ karara bağlanmamış konular

- Programlama dili / runtime
- Backend ve frontend framework'leri
- Database / storage teknolojileri
- Message broker / event bus
- Container / VM / hypervisor seçimleri
- Kubernetes veya alternatif orchestration kararı
- Client/controller/sensor implementasyon mimarisi
- Agent olup olmayacağı ve agent teknolojisi
- Network overlay / VPN teknolojisi
- Cloud sağlayıcı
- LLM sağlayıcısı ve model seçimi
- Observability teknoloji stack'i
- OSS component seçimi / build-vs-integrate
- Repo yapısı
- MVP feature selection
- MVP acceptance metric'lerinin sayısal hedefleri
- Roadmap / release breakdown

Bu konular sonraki adımlarda aynı owner-decision süreciyle ele alınacaktır.

## 0.2 APPROVED North Star

> **Ağının içine biri girdiğinde veya içeride keşif/lateral movement yapmaya başladığında bunu mümkün olduğunca erken, yüksek güvenle fark et; saldırganın ne yaptığını anlaşılır biçimde göster ve güvenlik uzmanı olmadan atılması gereken sonraki adımı açıkla.**

---

# 1. APPROVED Product Definition & Strategy Decisions

# 2.1 Problem Definition

## P-01 — Ürünün çözmeye çalıştığı birincil problem nedir?

**Durum:** APPROVED

**RECOMMENDED — D seçeneği.**

Önerilen problem statement:

> **Dedicated SOC/security uzmanlığı olmayan küçük ve orta ölçekli organizasyonlar, bir saldırgan perimeter'i geçtikten veya içeride yetkisiz keşif/credential abuse/lateral movement başlattıktan sonra bunu erken ve yüksek güvenle tespit etmekte; parçalı sinyalleri tek bir saldırı hikâyesine dönüştürmekte ve hangi aksiyonun güvenli ve öncelikli olduğunu belirlemekte zorlanır.**

Bu ifade “honeypot” kelimesini bilinçli olarak problem statement'tan çıkarır. Honeypot/deception çözüm mekanizmasıdır.

---

## P-02 — Ürünün odaklandığı güvenlik anı nerede başlar?

**Durum:** APPROVED

**RECOMMENDED — B.**

Ürün “breach prevention replacement” değildir. **Assume breach** mantığıyla, perimeter aşıldığında veya içeriden uygunsuz hareket başladığında erken uyarı katmanı olur. Pazarlama dili “saldırganı içeri alıyoruz” gibi savunmacı değil; “diğer kontroller aşıldığında ikinci bir görünürlük hattı sağlıyoruz” şeklinde olmalıdır.

---

## P-03 — Deception ürünün kendisi mi, yoksa ana mekanizması mı?

**Durum:** APPROVED

**RECOMMENDED — B, fakat kategori açıklamasında deception açıkça görünür.**

Ürün outcome-first anlatılmalıdır. “Honeypot yönetim platformu” olması yeterli differentiation sağlamaz. Buna rağmen deception, teknik kimliğin ve kategori konumlandırmasının temelidir. Önerilen zihinsel model:

**Product promise = internal breach visibility**  
**Primary mechanism = deception**  
**User outcome = high-confidence incident + attacker journey + guidance**

---

## P-04 — Ürün mevcut security stack'in yerine mi geçer, tamamlayıcı mı olur?

**Durum:** APPROVED

**RECOMMENDED — B.**

Ürünün değeri, mevcut ürünlerin yapamadığı her şeyi yapmak değil; yüksek güvenli deception sinyali ile özellikle internal discovery ve lateral movement boşluğunu doldurmaktır. EDR varsa onunla çelişmemeli; EDR yoksa da değer üretebilmelidir. Bu ilke ileride integrations için yön verir, ama Adım 2'de entegrasyon teknolojisi seçilmez.

---

## P-05 — Ürün “detect” ile mi biter, “respond” alanına da girer mi?

**Durum:** APPROVED

**RECOMMENDED — C.**

North Star zaten “sonraki adımı açıkla” der. Ancak ürün ilk aşamada kullanıcı adına otomatik containment kararı almamalıdır. Guidance üç katmanlı düşünülmelidir:

1. **Observe / verify:** olayı doğrula, evidence göster.
2. **Containment recommendation:** hangi cihaz/kullanıcı/servis için hangi kontrol düşünülebilir.
3. **Recovery / follow-up:** credential reset, host inspection, log review, scope expansion gibi öneriler.

İleride integrations üzerinden tek tık containment düşünülebilir; fakat **human approval** product principle olarak korunmalıdır.

---

## P-06 — Ürünün birincil başarı sonucu nedir?

**Durum:** APPROVED

**RECOMMENDED — başarı “volume” değil “decision quality” üzerinden tanımlansın.**

Adım 2 seviyesinde ürün başarı metrik ailesi:

- **Detection quality:** gerçek ilgilenilmesi gereken aktivitelerin yüksek güvenle yüzeye çıkması.
- **Noise control:** kullanıcının gereksiz alarm bombardımanına uğramaması.
- **Time-to-understand:** incident ekranından ne olduğunun hızla anlaşılması.
- **Source clarity:** mümkün olduğunda hangi cihaz/kaynak/kimlikten geldiğinin gösterilmesi.
- **Action clarity:** kullanıcının “şimdi ne yapacağım?” sorusuna cevap alması.
- **Operational burden:** sürekli bakım gerektirmemesi.
- **Trust:** AI çıktılarının evidence'dan ayrılmaması.

Bu aşamada sayısal hedefler belirlenmemelidir; hedefler ürün doğrulama/MVP acceptance aşamasında ölçülebilir hale getirilecektir.

---

## P-07 — Hangi problem varsayımları henüz doğrulanmamış kabul edilmelidir?

**Durum:** APPROVED

Aşağıdakiler **ASSUMPTION — VALIDATION NEEDED** olarak tutulmalıdır:

- SOC'u olmayan security-lean organizasyonların bu problem için bütçe ayıracağı.
- “High-confidence alert + clear guidance” değerinin decoy çeşitliliğinden daha önemli olduğu.
- Generalist IT kullanıcısının deception deployment'ını kabul edeceği.
- İç network'te yeterli sayıda anlamlı deception touchpoint oluşturulabileceği.
- Kullanıcının AI-generated explanation'a güveneceği; evidence görünürlüğünün bu güveni artıracağı.
- Kurulum/operasyon basitliğinin satın alma ve kullanım üzerinde belirleyici olduğu.
- Müşterinin EDR/firewall olmasına rağmen ek internal tripwire katmanına ihtiyaç duyacağı.

**Owner decision:** Bu varsayımların Adım 2'de “doğru” kabul edilmesi değil, “ürün hipotezinin test edilmesi gereken parçaları” olarak kabul edilmesi önerilir.

---

## 2.1 Approved Section Output

**RECOMMENDED Problem Statement**

> **SOC veya dedicated security ekibi olmayan, internal network'ü bulunan küçük/orta ölçekli organizasyonlar; bir saldırgan veya kompromize cihaz iç ağda discovery, credential abuse veya lateral movement davranışına başladığında bunu erken ve güvenilir biçimde fark etmekte, parçalı sinyalleri tek bir saldırı hikâyesine dönüştürmekte ve güvenli bir sonraki aksiyonu belirlemekte zorlanır. Ürün, deception yoluyla yüksek-sinyalli evidence üretir; evidence'ı incident ve attacker journey bağlamında sunar; teknik olmayan abartıdan kaçınarak ne olduğunu, ne kadar ciddi olduğunu ve ne yapılması gerektiğini açıklar.**

**Durum: RECOMMENDED — OWNER DECISION REQUIRED**

---


# 2.2 Customer, Buyer & User Definition

## C-01 — İlk ICP çalışan sayısına mı, security maturity'ye mi göre tanımlanmalı?

**Durum:** APPROVED

**RECOMMENDED — C primary, headcount secondary heuristic.**

Ürünü “15 kişilik şirket” gibi dar ve yanıltıcı bir sayı ile tanımlamamak gerekir. 20 kişilik bir fintech security açısından olgun olabilir; 300 kişilik bölgesel işletmenin dedicated SOC'u olmayabilir. İlk ICP şu şekilde tanımlansın:

> **Internal network/hybrid infrastructure işleten, dedicated SOC'u bulunmayan ve güvenlik operasyonunu generalist IT/infra/DevOps/CTO rolünün yürüttüğü security-lean organization.**

SMB, pazar iletişiminde pratik etiket olarak kullanılabilir; ürün tasarımında asıl persona security operating model'dır.

---

## C-02 — İlk müşteri ortamı hangi altyapı profilini taşımalı?

**Durum:** APPROVED

**RECOMMENDED — B, ancak ilk değer ispatı internal routable network olan ortamlarda.**

Cloud-first/remote-only organizasyonlar deception için ileride identity, endpoint, cloud credential ve SaaS deception gerektirir. Bunlar vizyonda yer almalıdır fakat ilk ürün problemi internal network olduğu için “her ortam eşit” denmemelidir.

**Product implication:** Customer profile'da en az bir anlamlı internal trust zone, subnet/VLAN veya VPN üzerinden erişilen private network olması tercih edilir.

---

## C-03 — Primary operator kimdir?

**Durum:** APPROVED

**RECOMMENDED — Primary operator: generalist technical owner (IT manager/sysadmin/DevOps/CTO); dedicated security analyst secondary persona.**

Ürün UX'i şu kullanıcıyı varsaymalıdır:

- Network ve sunucu kavramlarını bilir.
- IP/subnet/VLAN gibi temel kavramlardan tamamen yabancı değildir.
- SIEM query dili, malware reverse engineering veya DFIR uzmanlığı yoktur.
- Güvenlik ürünü için sınırlı operasyon zamanı vardır.
- Alert geldiğinde “hangi cihaz, ne oldu, ne yapayım?” sorusuna cevap ister.

---

## C-04 — Hiç IT personeli olmayan şirket primary persona olabilir mi?

**Durum:** APPROVED

**RECOMMENDED — C.**

Deception ürününün network placement, incident containment ve cihaz identification gibi konuları tamamen teknik bilgiden bağımsız hale getirmek gerçekçi değildir. UI sade olmalı, ancak “teknik sorumlusu olmayan kullanıcı” için güvenli response guidance vermek ayrıca hizmet/partner katmanı gerektirir. Bu nedenle doğrudan ilk persona olmamalıdır.

---

## C-05 — Buyer kimdir?

**Durum:** APPROVED

**RECOMMENDED — Economic buyer ve operator küçük organizasyonda çoğu zaman çakışır; primary buyer teknik bütçe sahibi CTO/IT manager/owner-operator olmalıdır.**

Enterprise procurement/CISO workflow'una göre ürün tasarlamak ilk hipotezle çelişir. Ancak karar verici ile günlük operator ayrıştığında, ürünün “neden gerekli” raporlaması ve security posture görünürlüğü buyer için yeterli olmalıdır.

---

## C-06 — Incident responder kim kabul edilmeli?

**Durum:** APPROVED

**RECOMMENDED — Primary responder aynı generalist technical operator; secondary escalation path external IT/security provider.**

Ürün yalnızca “SOC analyst'e telemetry gönderir” yaklaşımına dayanamaz. Guidance doğrudan first responder'a hitap etmelidir. Bununla birlikte bazı aksiyonlar “uzmana escalate et” şeklinde sonuçlanabilir.

---

## C-07 — İlk ürün tek bir sektöre mi odaklanmalı?

**Durum:** APPROVED

**RECOMMENDED — A başlangıç product definition; verticalization gelecekte GTM doğrulamasına göre.**

Adım 2'de ürünün çekirdeği sektör bağımsız internal attacker behavior'dır. OT/ICS gibi alanlar farklı risk ve protokol seti yarattığından aynı seviyede ele alınmamalıdır. Software companies ileride AI/MCP/cloud deception için cazip bir vertical olabilir; ancak şimdiden ürün kimliğine kilitlenmemelidir.

---

## C-08 — Windows-heavy ortamlara yaklaşım

**Durum:** APPROVED

**RECOMMENDED — Customer model “Linux-only” varsaymamalı; Windows workstation/server, SMB/RDP/identity davranışları uzun vadeli threat/deception modelinin doğal parçası sayılmalıdır.**

Bu, sonraki mimaride licensing, image management, domain realism ve protocol support gereksinimleri doğurur.

---

## C-09 — MSP/MSSP persona'sı ne durumda tutulmalı?

**Durum:** APPROVED

**RECOMMENDED — “Future strategic persona” olarak kayıtlı kalsın, ancak mevcut product decisions tek-customer/single-environment mental model ile verilsin.**

Böylece bugünkü sadelik korunur; gelecekte central multi-customer operasyonu için ürün konsepti yeniden tasarlanabilir.

---

## C-10 — Explicit Non-Personas kimlerdir?

**Durum:** APPROVED

Aşağıdaki kullanıcı grupları ilk product definition'ın **NON-PERSONA** listesine alınsın:

- Dedicated SOC ve olgun SIEM/SOAR operasyonu olan büyük enterprise'ın Tier-1 analyst'i (ürünü kullanabilir ama tasarım merkezi değildir).
- Global threat-intelligence araştırmacısı için public honeynet yöneten ekip.
- OT/ICS specialist organization.
- Ev/homelab hobici kullanıcı.
- Hiç teknik operasyon kapasitesi olmayan direct end customer.
- MSSP'nin yüzlerce tenant'ını yöneten SOC operatorü.
- Offensive security/red-team engagement platformu arayan ekip.

Bu dışlama “asla desteklenmeyecek” anlamına gelmez; ilk ürün davranışlarının bu persona'lara optimize edilmemesi anlamına gelir.

---

## 2.2 Approved Section Output

**RECOMMENDED Primary Customer**

> **Internal/private network veya hybrid infrastructure işleten; dedicated SOC'u bulunmayan; güvenlik operasyonunun IT manager, sysadmin, DevOps, CTO veya benzeri generalist teknik role ait olduğu security-lean küçük/orta ölçekli organizasyon.**

**RECOMMENDED Primary Persona**

> **Network ve sistem operasyonunu bilen fakat full-time security analyst olmayan teknik sorumlu.**

**RECOMMENDED Buyer Model**

> **Teknik bütçe sahibi ile operator çoğu zaman aynı veya yakın roldedir; enterprise procurement odaklı ürün deneyimi ilk hedef değildir.**

**Durum: RECOMMENDED — OWNER DECISION REQUIRED**

---


# 2.3 Jobs-to-be-Done (JTBD)

## J-01 — Primary functional job nedir?

**Durum:** APPROVED

**RECOMMENDED:**

> **“Ağımın içinde yetkisiz veya saldırgan davranış başladığında bunu erken ve güvenilir şekilde fark etmek; kaynağı ve hareketleri anlamak; uygun sonraki aksiyonu belirlemek.”**

---

## J-02 — Detection JTBD ne kadar geniş olmalı?

**Durum:** APPROVED

**RECOMMENDED:** Kullanıcı “her saldırıyı tespit et” beklentisine sokulmamalıdır. Detection job şu şekilde tanımlansın:

> **Normal kullanıcıların dokunmaması gereken deception assets/artefacts ile etkileşimi ve bu interaction'ların gösterdiği discovery, credential abuse ve lateral movement davranışlarını yüksek-sinyalle ortaya çıkarmak.**

Bu ifade deception'ın gücünü korur ve “EDR replacement” beklentisi yaratmaz.

---

## J-03 — Investigation JTBD ürünün çekirdeğinde mi?

**Durum:** APPROVED

**RECOMMENDED — Evet, çekirdekte.**

Daha önce approved Level C hedefi nedeniyle ürün yalnızca “decoy hit” gösteremez. Kullanıcının işi:

> **“Bu tekil event ne demek?” değil, “Bu olayda ne oldu?”**

olmalıdır.

Bu nedenle incident correlation, timeline ve attacker journey capability'leri feature değil, primary JTBD'nin parçasıdır.

---

## J-04 — Attribution/source-identification JTBD ne olmalı?

**Durum:** APPROVED

**RECOMMENDED — “Source identification” kullanılsın; “real-world attacker attribution” ürün sözü olmasın.**

Ürün mümkün olduğunda IP'nin ötesinde hostname/device/user/VLAN/VPN identity gibi bağlam göstermeyi hedeflemeli; ancak confidence belirsizse bunu açıkça belirtmelidir.

---

## J-05 — Severity assessment JTBD

**Durum:** APPROVED

**RECOMMENDED:** Kullanıcı tek başına raw MITRE technique veya port numarasından severity çıkarmak zorunda kalmamalıdır. Ürün:

- ne görüldüğünü,
- neden şüpheli olduğunu,
- evidence'ın ne kadar güçlü olduğunu,
- potansiyel etkiyi,
- hangi belirsizliklerin bulunduğunu

ayırarak sunmalıdır.

Burada **confidence** ile **severity** ayrı kavramlardır. Bu ayrım Bölüm 2.9'da formalize edilir.

---

## J-06 — Response JTBD'nin kapsamı

**Durum:** APPROVED

**RECOMMENDED — Context-aware recommendation, human approval.**

Örnek job:

> “Bu olay gerçek bir saldırı ihtimali taşıyorsa önce hangi cihazı kontrol etmeliyim, hangi hesabı riskli saymalıyım ve hangi doğrulama/containment adımlarını hangi sırayla yapmalıyım?”

---

## J-07 — Operational JTBD

**Durum:** APPROVED

**RECOMMENDED:**

> **“Ürünü sürekli bakımını yaptığım ayrı bir security projesine dönüştürmeden çalışır halde tutmak.”**

Bu JTBD, product simplicity kararının operasyonel karşılığıdır. Sonraki mimaride update, health, failure visibility ve lifecycle requirements doğurur.

---

## J-08 — Confidence/noise JTBD

**Durum:** APPROVED

**RECOMMENDED:**

> **“Günde yüzlerce belirsiz güvenlik uyarısı arasında hangisinin önemli olduğunu seçmek zorunda kalmamak.”**

Bu JTBD ürünün core quality bar'ı olarak kabul edilmelidir.

---

## J-09 — Product feedback JTBD

**Durum:** APPROVED

**RECOMMENDED:** Kullanıcı false positive/benign activity gördüğünde ürünün davranışını güvenli biçimde düzeltmeli ve benzer durumların tekrarını azaltabilmelidir. “Acknowledge”, “benign”, “known scanner”, “expected activity” gibi feedback kavramları product loop'a dahil edilmelidir.

---

## 2.3 Approved Section Output

| JTBD | Önerilen ifade | Öncelik |
|---|---|---|
| Detection | İç network'te yetkisiz discovery/credential/lateral behavior başladığında yüksek güvenle fark et | Core |
| Investigation | Parçalı event'leri tek incident ve attacker journey olarak anla | Core |
| Source identification | Olayın hangi cihaz/kullanıcı/segmentten geldiğini mümkün olduğunca belirle | Core |
| Assessment | Confidence, severity ve impact'i anlaşılır ayır | Core |
| Response | Güvenli ve öncelikli sonraki adımı belirle | Core |
| Notification | Gerçekten önemli olduğunda doğru kişiye ulaş | Core |
| Operations | Sürekli babysitting olmadan deception coverage'ını çalışır tut | Core |
| Feedback | Benign davranışı işaretle, noise'u azalt | Supporting/Core-quality |
| Reporting | Olayı sonradan kanıt ve zaman çizelgesiyle incele | Supporting |

**Durum: RECOMMENDED — OWNER DECISION REQUIRED**

---


# 2.4 Threat Model

## T-01 — Primary threat starting condition nedir?

**Durum:** APPROVED

**RECOMMENDED — B primary; C observable secondary; A current out-of-scope.**

Ürün tehdit modelinin ana başlangıç varsayımı:

> **Attacker has a foothold, stolen credential, compromised endpoint, VPN access veya başka biçimde internal/private network'e erişebilen bir execution point.**

Bu threat model “initial access nasıl oldu?” sorusunu çözmez; initial access sonrası davranışı gözlemlemeye odaklanır.

---

## T-02 — Malicious insider ürün kapsamına alınmalı mı?

**Durum:** APPROVED

**RECOMMENDED — C.**

Detection mekanizması actor motive'ını bilmez. Malicious insider, careless admin veya kompromize cihaz aynı deception asset'e dokunabilir. Ürün evidence'ı göstermeli, “bu kesin malicious insider” diye iddia etmemelidir. Insider detection ayrı kategori olarak pazarlanırsa HR/legal/UEBA beklentileri doğar ve ürün kapsamı genişler.

---

## T-03 — Primary threat behaviors hangileridir?

**Durum:** APPROVED

**RECOMMENDED primary behavior set:**

1. **Recon / network probing** — subnets, hosts, ports, services hakkında bilgi toplama.
2. **Discovery / enumeration** — share, service, database, remote management, identity veya asset keşfi.
3. **Credential abuse** — honey credential veya decoy credential kullanımı; weak/default credential denemeleri; credential reuse işaretleri.
4. **Lateral movement attempts** — SSH, SMB, RDP, WinRM, database, remote administration vb. üzerinden diğer sistemlere erişim/movement.
5. **Decoy interaction depth** — login, command, file access, query, token use gibi daha güçlü intent göstergeleri.

Bu davranışlar ürün promise'iyle en güçlü kesişimdir.

---

## T-04 — Initial Access detection ürünün hedefi mi?

**Durum:** APPROVED

**RECOMMENDED — Hayır, direct primary coverage değil.**

Phishing, drive-by compromise, public vulnerability exploitation, malicious attachment veya supply-chain initial access başka ürün ailelerinin güçlü alanlarıdır. Deception artefacts gelecekte bazı initial-access benzeri sinyaller üretebilir; ancak ürün “initial access detection platform” olarak konumlandırılmamalıdır.

---

## T-05 — Credential Access ile Credential Abuse arasındaki sınır

**Durum:** APPROVED

**RECOMMENDED — Ürün sözünde “credential theft detection” yerine “deception credential access/use and credential abuse signals” denmeli.**

Bu, gerçek coverage ile iddia arasındaki farkı korur.

---

## T-06 — Malware/C2/exfiltration/impact coverage nasıl tanımlanmalı?

**Durum:** APPROVED

**RECOMMENDED — B.**

High-interaction decoy içinde malware download, shell commands veya exfiltration attempt gibi davranışlar ileride görülebilir. Bunlar önemli evidence'dır fakat “malware sandbox”, “C2 detection” veya “DLP” ürün kimliği oluşturmaz.

---

## T-07 — Ransomware ürün iddiası

**Durum:** APPROVED

**RECOMMENDED — “Ransomware prevention” iddiası yok; “ransomware ve diğer intrusions öncesindeki internal discovery/lateral movement davranışlarını erken görünür kılabilir” şeklinde kontrollü ilişki.**

Ürün outcome'u detection/guidance'dır; guaranteed prevention claim yapılmamalıdır.

---

## T-08 — Public internet attackers threat modelde mi?

**Durum:** APPROVED

**RECOMMENDED — Threat modelin primary source'u private/internal addressable environment.** Public exposure ancak müşteri kendi internal decoy'unu yanlışlıkla veya bilinçli olarak expose ederse güvenlik politikası konusu olarak ele alınmalıdır; ürün global internet telemetry network'üne dönüşmemelidir.

---

## T-09 — Legitimate scanner/admin activity tehdit midir?

**Durum:** APPROVED

**RECOMMENDED — Hayır; fakat threat modelin ana ambiguity source'udur.**

Legitimate vulnerability scanners, EDR discovery, monitoring systems, inventory tools, backup systems veya admin scripts decoy'a dokunabilir. Thinkst'in ignore/annotation mekanizmaları bunun pratik bir ürün problemi olduğunu gösterir. [S6][S7]

Bu yüzden threat model yalnız “attacker behavior” değil, **benign lookalikes** listesini de içermelidir.

---

## T-10 — MITRE ATT&CK ürün mantığının merkezi mi olmalı?

**Durum:** APPROVED

**RECOMMENDED — B.**

MITRE mapping security professional için değerli bağlam sağlar ve terminolojiyi standardize eder. Ancak primary persona'nın “T1021.004” görmesi tek başına değer değildir. ATT&CK label secondary detail olmalı; ana açıklama insan dilinde olmalıdır.

---

## T-11 — Identity/cloud threats threat modelde hangi statüde?

**Durum:** APPROVED

**RECOMMENDED — Current threat model iki katmanlı olsun:**

- **Primary coverage:** internal network discovery, credential abuse, lateral movement.
- **Future expansion:** endpoint breadcrumbs, identity deception, cloud credentials, SaaS/AI infrastructure deception.

Böylece vizyon kaybolmaz; current product promise bulanıklaşmaz.

---

## T-12 — Threat coverage güven seviyeleri nasıl ifade edilmeli?

**Durum:** APPROVED

**RECOMMENDED:** Threat modelde her davranış için üç unsur tutulmalıdır:

- **Coverage intent:** Primary / Secondary / Future / Out-of-scope
- **Expected signal confidence:** Low / Medium / High potential
- **Evidence type:** direct interaction / correlated behavior / contextual enrichment

Bu sınıflandırma teknik implementation seçmeden capability önceliklendirmesine yardımcı olur.

---

## 2.4 Approved Section Output

> **Primary threat model: saldırganın veya kompromize bir internal device/account'un private network erişimi elde ettiği; ardından recon/discovery, credential abuse ve lateral movement davranışları sergilediği post-compromise senaryolar. Ürün bu davranışların deception ile kesiştiği noktalarda yüksek-sinyalli evidence üretmeyi hedefler. Initial access, malware prevention, public internet threat intelligence ve full kill-chain coverage primary product promise değildir.**

**Durum: RECOMMENDED — OWNER DECISION REQUIRED**


# 2.5 Explicit Non-Goals

## NG-01 — Antivirus / EDR olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — B.**

Endpoint bileşeni ileride deception breadcrumb/token veya telemetry amacıyla gerekebilir. Ancak process monitoring, malware prevention, exploit blocking, behavioral EDR engine gibi kapsamlar **NON-GOAL** olmalıdır. Ürün EDR ile entegre olabilir; EDR olmaya çalışmamalıdır.

---

## NG-02 — IDS/IPS/NDR olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — Hayır, full passive network monitoring veya inline blocking ürünü olmayacağız.**

Network telemetry yardımcı context olarak kullanılabilir, ancak ürünün çekirdek sinyali deception interaction'dır. Bu sınır üç nedenle önemlidir:

1. Full NDR, packet inspection ve anomaly detection ayrı bir ürün kategorisi yaratır.
2. Noise ve tuning yükünü ciddi artırır.
3. “High signal” ürün prensibini zayıflatabilir.

**Non-goal:** Her network packet'ını anlamaya çalışan genel NDR.

---

## NG-03 — Firewall / IPS / NAC olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — Hayır.**

Ürün ileride firewall/NAC/EDR sistemine response action önerebilir veya entegrasyonla approved containment tetikleyebilir. Ancak policy enforcement cihazı olmak ürünün çekirdeği değildir.

---

## NG-04 — SIEM olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — Hayır. “Deception incident system” ile “general SIEM” ayrılmalı.**

Ürün kendi evidence'ını ve gerekli contextual telemetry'yi tutabilir; fakat müşterinin tüm log kaynaklarını ingest edip genel-purpose query/search/compliance platformu olmayı hedeflememelidir.

---

## NG-05 — SOAR / fully automated response platformu olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — Hayır.**

Guided response ve gelecekte kontrollü integrations olabilir. Ancak arbitrary playbook engine, complex orchestration designer veya human approval olmadan geniş çaplı remediation **NON-GOAL** olmalıdır.

NIST'in incident-response yaklaşımı da containment gibi aksiyonlarda organization-specific judgment ve süreç gerektirir; ürün guidance sunabilir fakat kullanıcı bağlamını yok sayan otomasyon güvenli değildir. [S1]

---

## NG-06 — Vulnerability scanner / Attack Surface Management olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — Full scanner NON-GOAL; deception placement için gerekli kadar environment understanding SUPPORTING capability.**

Ürün “CVE bulma ve patch management” satın alınan bir ASM/vulnerability product olmamalıdır. Fakat gerçekçi decoy önermek için network profile ve asset context öğrenmek product loop'un parçası olabilir.

---

## NG-07 — Email security / phishing gateway olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — Hayır.**

Fake document/token gelecekte deception artefact olabilir; bu, mailbox classification veya anti-phishing gateway ürünü olmamız anlamına gelmez.

---

## NG-08 — VPN / ZTNA / secure access platformu olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — Hayır.**

Network connectivity bazı deployment senaryolarında gerekli olabilir; ancak kullanıcıya VPN/ZTNA ürünü satmak product scope değildir.

---

## NG-09 — Public honeynet / global threat-intelligence network olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — Explicit non-goal olarak yazılsın.**

İleride anonymized shared intelligence veya community signals ayrıca değerlendirilebilir; fakat ürünün ilk kimliği global attacker IP collection değildir.

---

## NG-10 — Malware sandbox olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — Hayır.**

High-interaction decoy'da attacker'ın indirdiği dosya veya komut evidence olabilir. Ancak arbitrary malware execution analysis, detonation scoring ve reverse engineering ürün alanı değildir.

---

## NG-11 — Offensive security / hack-back yapacak mıyız?

**Durum:** APPROVED

Explicit product principle:

> **Saldırganı yanıltmak, fake environment'a çekmek ve kontrollü sistem içinde gözlemlemek kabul edilebilir; gerçek attacker infrastructure'ına zarar vermek, yetkisiz erişim sağlamak veya müşteri ağı dışına saldırı başlatmak kabul edilemez.**

---

## NG-12 — Autonomous AI security operator olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — Hayır.**

AI:

- evidence yorumlayabilir,
- incident açıklayabilir,
- seçenek üretebilir,
- recommendation verebilir,
- gelecekte controlled deception önerileri hazırlayabilir.

Ancak AI'ın tek başına “bu cihaz saldırgan, kapatıyorum” gibi irreversibly operational kararlar vermesi product principle'a aykırıdır.

---

## NG-13 — Compliance/reporting ürünü olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — Compliance platform NON-GOAL.**

Audit-friendly evidence, incident export veya rapor bulunabilir; fakat ISO/NIST/SOC2 compliance management ürünü olmak ayrı bir problem alanıdır.

---

## NG-14 — Tam SOC replacement olacak mıyız?

**Durum:** APPROVED

**RECOMMENDED — “SOC replacement” iddiası yapılmamalı.**

Ürün bir güvenlik uzmanının yaptığı her işi yapmaz. Daha doğru ifade:

> **Dedicated SOC'u olmayan organizasyona, deception kaynaklı high-confidence incidents'i anlamak ve ilk doğru adımı atmak için security expertise leverage sağlar.**

---

## NG-15 — MSSP multi-tenant platformu current product definition'a dahil mi?

**Durum:** APPROVED

Current non-goal listesine açıkça eklenmeli: tenant hierarchy, delegated administration, MSP billing, reseller operations ve multi-customer SOC console bu aşamada yoktur.

---

## 2.5 Approved Section Output

**RECOMMENDED:**

- Antivirus / full EDR
- General IDS/IPS/NDR
- Firewall/NAC/ZTNA/VPN product
- General-purpose SIEM
- General-purpose SOAR
- Vulnerability scanner / ASM platform
- Email security gateway
- Malware sandbox
- Public/global honeynet as primary product
- Full threat-intelligence vendor
- Compliance management suite
- Autonomous destructive response
- Hack-back/offensive retaliation
- Full SOC replacement claim
- Current multi-tenant MSSP platform

**Durum: RECOMMENDED — OWNER DECISION REQUIRED**

---


# 2.6 Core Product Loop

## L-01 — Product loop'un ilk adımı “deploy decoy” mu, “understand environment” mı?

**Durum:** APPROVED

**RECOMMENDED — B, minimal/contextual understanding.**

Product promise “deploy deception, not complexity” ise kullanıcıdan onlarca network detail istemek doğru değildir. Buna karşılık tamamen context-free decoy, gerçekçilik ve placement kalitesini azaltır.

İlke:

> **Ürün full CMDB kurmaya çalışmadan, deception placement için yeterli network/environment context edinmelidir.**

---

## L-02 — Deception placement manual mı, assisted mı, autonomous mu?

**Durum:** APPROVED

**RECOMMENDED — Assisted placement + explicit owner approval.**

Product flow:

1. Environment hakkında sınırlı context edin.
2. “Bu subnet için Windows file server + PostgreSQL decoy anlamlı olabilir” gibi öneri sun.
3. Kullanıcı placement/persona'yı onaylasın veya değiştirsin.
4. Sistem deployment'ı uygulasın.

Gelecekte adaptive/autonomous deception ayrı capability olabilir; current principle human control'dür.

---

## L-03 — Deception default olarak sparse mı, dense mı olmalı?

**Durum:** APPROVED

**RECOMMENDED — Başlangıç ürün felsefesi “purposeful sparse coverage”; future adaptive density.**

Security-lean müşteri için decoy sayısı başarı metriği olmamalıdır. Az ama doğru yerde ve gerçekçi decoy, operational burden'ı azaltır. İleride risk-based/adaptive expansion değerlendirilebilir.

---

## L-04 — Event geldiğinde sistem doğrudan alert üretmeli mi?

**Durum:** APPROVED

**RECOMMENDED — Üçüncü model.**

Her network connection aynı ürün anlamını taşımamalıdır. Product loop:

**Raw interaction/event → signal/evidence → correlation → incident candidate → confidence/severity → notification**

Detaylı kavramlar Bölüm 2.9'da tanımlanır.

---

## L-05 — Correlation'ın product-level görevi nedir?

**Durum:** APPROVED

**RECOMMENDED:** Aynı source/actor/context'e ilişkin parçalı interaction'ları “bir attack story” haline getirmek.

Correlation sonucu mutlaka deterministic olmak zorunda değildir; ancak hangi evidence'ın neden birlikte gösterildiği kullanıcıya izlenebilir olmalıdır. AI soft inference yapabilir fakat raw evidence kaybolmamalıdır.

---

## L-06 — AI ürün loop'unun hangi noktasında devreye girer?

**Durum:** APPROVED

**RECOMMENDED — Evidence sonrası reasoning; selected assisted use earlier.**

Minimum ürün davranışı AI yokken de:

- interaction'ı kaydetmeli,
- evidence oluşturmalı,
- deterministic alerting çalıştırabilmeli,
- incident history'yi koruyabilmelidir.

AI mevcutsa:

- incident narrative,
- correlation hypothesis,
- severity/context explanation,
- recommended next actions

gibi değer ekler.

---

## L-07 — Response “recommendation”dan sonra ürün ne yapar?

**Durum:** APPROVED

**RECOMMENDED — İkinci model.**

Product loop'un kapanması için incident lifecycle gereklidir:

- Acknowledge
- Investigating
- Benign/expected
- Confirmed suspicious/incident
- Resolved
- Follow-up recommended

Bu durumlar sadece UI status değildir; noise learning ve trust için gereklidir.

---

## L-08 — Benign activity feedback ürün davranışını değiştirmeli mi?

**Durum:** APPROVED

**RECOMMENDED — Evet, ancak kontrollü.**

Kullanıcının “bu scanner beklenen” demesi gelecekte suppression/annotation oluşturabilir. Fakat tek bir yanlış feedback tüm network behavior'ını sessizleştirmemelidir. Product requirement: **feedback reversible ve visible olmalıdır.**

---

## L-09 — AI unavailable olduğunda product degraded mode ne olmalı?

**Durum:** APPROVED

**RECOMMENDED product principle:**

> **AI outage, quota, provider error veya offline condition detection truth'u durdurmamalıdır.**

AI unavailable ise incident daha az açıklamalı olabilir ama core evidence/alert devam etmelidir. Bu ilke daha sonra architecture'da provider abstraction, queueing/fallback gibi gereksinimler doğurabilir; çözüm bu aşamada seçilmez.

---

## L-10 — Product loop “learn” adımında ne öğrenir?

**Durum:** APPROVED

**RECOMMENDED — Hepsi product-level opportunity, fakat autonomous self-modification yok.**

Learn adımı:

- false-positive pattern'larını azaltma,
- decoy placement önerilerini iyileştirme,
- incident explanation quality'yi artırma,
- stale/unhealthy deception'ı tespit etme,
- future adaptive deception için data üretme

amaçlarına hizmet etmelidir.

---

## 2.6 Approved Section Output

1. **Deploy:** Kullanıcı ürünü private environment'a ekler.
2. **Understand:** Sistem deception için gerekli minimum environment context'i edinir.
3. **Recommend:** Uygun decoy/persona/placement önerir.
4. **Approve & Place:** Kullanıcı onaylar; deception assets etkinleşir.
5. **Observe:** Interaction telemetry sürekli toplanır; health izlenir.
6. **Detect:** Meaningful interaction deterministic evidence'a dönüşür.
7. **Correlate:** Related evidence tek incident/attacker journey altında ilişkilendirilir.
8. **Explain:** Kullanıcıya ne olduğu ve neden önemli olduğu gösterilir.
9. **Recommend:** Güvenli, bağlama uygun sonraki aksiyon sunulur.
10. **Resolve:** Kullanıcı acknowledge/investigate/resolve/benign olarak sınıflandırır.
11. **Learn:** Feedback ve environment değişimleri coverage/noise kalitesini iyileştirir.

**Durum: RECOMMENDED — OWNER DECISION REQUIRED**

---


# 2.7 Detection → Investigation → Response User Journey & Incident Model

## I-01 — Event, Signal, Alert ve Incident aynı şey mi?

**Durum:** APPROVED

**RECOMMENDED — Hayır; ayrı kavramlar.**

- **Event:** Sensör/deception asset tarafından gözlenen ham veya normalize edilmiş tek olay.
- **Signal:** Security anlamı taşıyan yorumlanmış event/pattern.
- **Evidence:** Kullanıcıya gösterilebilen, kaynağı/provenance'ı bilinen destekleyici veri.
- **Finding:** Bir veya daha fazla evidence'dan çıkarılan güvenlik bulgusu.
- **Incident:** Kullanıcının değerlendirmesi/aksiyonu gereken, ilişkili findings/evidence bütünü.
- **Notification:** Incident veya önemli state için kullanıcıya iletilen mesaj.

Bu ayrım Bölüm 2.9 ile birlikte ürünün information model temelidir.

---

## I-02 — Primary UI object Alert mı Incident mı?

**Durum:** APPROVED

**RECOMMENDED — Incident-centric, raw evidence accessible.**

User landing experience “12 alerts” değil “1 incident — source X, 5 related actions” olmalıdır. Ancak her incident altında raw event/evidence incelenebilmelidir.

---

## I-03 — Attacker Journey ürünün merkezinde mi, opsiyonel visualization mı?

**Durum:** APPROVED

**RECOMMENDED — First-class product object.**

Attacker journey en az şu soruları cevaplamalıdır:

- İlk görülen meaningful activity neydi?
- Hangi source'tan geldi?
- Hangi deception assets'e dokundu?
- Hangi credential/service denemelerini yaptı?
- Interaction sırası neydi?
- Hangi behavior daha yüksek confidence yarattı?
- En son ne yaptı?
- Olay hâlâ aktif mi?

---

## I-04 — Incident'in source identity modeli ne olmalı?

**Durum:** APPROVED

**RECOMMENDED — Multi-evidence source identity.**

Internal network'te IP değişebilir, NAT/VPN olabilir veya shared devices bulunabilir. Product-level concept:

**Source Entity** = observed IP + optional hostname + MAC/device fingerprint + user/account + network segment/VPN/session context + confidence.

Burada nasıl elde edileceği mimari karardır ve sonraya bırakılır.

---

## I-05 — Confidence ve Severity ayrı mı olmalı?

**Durum:** APPROVED

**RECOMMENDED — Kesinlikle ayrı.**

Örnek:

- Honey credential kullanımı: **confidence very high**, severity medium olabilir.
- Şüpheli widespread scan: **severity potentially high**, confidence medium olabilir.
- Confirmed decoy admin login + commands: hem confidence hem severity yüksek olabilir.

Tek bir “risk score” bu ayrımı gizleyebilir.

---

## I-06 — Incident explanation hangi yapıda olmalı?

**Durum:** APPROVED

**RECOMMENDED standart açıklama şablonu:**

1. **What happened?** Kısa insan dili özeti.
2. **Why this matters?** Neden normal davranıştan farklı.
3. **Who/where did it come from?** Source entity + confidence.
4. **What did it touch?** Decoys/tokens/services.
5. **Timeline / journey.** Kronolojik evidence.
6. **Confidence.** Ne kadar eminiz ve neden.
7. **Potential impact.** Gerçek production impact değilse açıkça ayır.
8. **Recommended next actions.** Öncelikli adımlar.
9. **What we do not know.** Belirsizlikler.
10. **Evidence details.** Technical operator için drill-down.

Bu yapı AI explanation'ın da sözleşmesi olmalıdır.

---

## I-07 — Evidence görünürlüğü ne kadar olmalı?

**Durum:** APPROVED

**RECOMMENDED — Üçüncü model.**

AI'ın “saldırgan lateral movement yaptı” demesi yeterli değildir. Kullanıcı hangi interaction'ın bunu desteklediğini görebilmelidir. Product trust için **claim → evidence linkage** gerekir.

---

## I-08 — MITRE ATT&CK UI'da nasıl görünmeli?

**Durum:** APPROVED

**RECOMMENDED — Secondary enrichment.**

Örnek:

> “Source 10.0.10.42 önce file share'leri enumerate etti, sonra decoy SMB share'e erişti.”

Alt detay:

> ATT&CK mapping: Discovery / Network Share Discovery; Lateral Movement related context.

Primary persona'yı technique ID'leriyle boğmamak gerekir.

---

## I-09 — Notification hangi objeden üretilmeli?

**Durum:** APPROVED

**RECOMMENDED — Notification policy incident/significant finding üzerinden çalışmalı.**

Her raw event için SMS/email gönderilmemelidir. Notification şu bilgileri minimum taşımalıdır:

- Source
- What happened
- Confidence/severity
- Affected decoy/persona
- “Open incident” action

Daha detaylı channel/integration seçimi sonraki aşamadadır.

---

## I-10 — Incident lifecycle hangi statülere sahip olmalı?

**Durum:** APPROVED

**RECOMMENDED product semantics:**

- New
- Acknowledged
- Investigating
- Benign / Expected
- Confirmed Suspicious / Confirmed Incident
- Resolved
- Reopened

“Closed” tek başına yetersizdir; neden kapandığı feedback ve reporting için önemlidir.

---

## I-11 — Recommended action ne kadar prescriptive olmalı?

**Durum:** APPROVED

**RECOMMENDED — Context-aware ordered checklist.**

Örnek action types:

- Source device'ı owner envanterinde doğrula.
- O cihazın network erişimini geçici olarak sınırlandırmayı değerlendir.
- İlgili kullanıcı credential'larını reset etmeyi değerlendir.
- EDR/AV varsa full scan ve process review başlat.
- VPN/session loglarını kontrol et.
- Aynı source'un başka internal targets ile etkileşimini araştır.

AI, olmayan ürün entegrasyonunu varmış gibi söylememeli ve “kesin kapat” yerine risk/operational impact'i ayırmalıdır.

---

## I-12 — Incident kapandıktan sonra ürün ne göstermeli?

**Durum:** APPROVED

**RECOMMENDED — Post-incident record:**

- final classification,
- timeline,
- source resolution,
- actions taken,
- unresolved questions,
- recommended follow-up,
- related incidents,
- feedback/suppression changes.

Bu, ürünün basit bir ephemeral alert system yerine güvenilir incident history oluşturmasını sağlar.

---


# 2.8 Deception Capability Model

## D-01 — Deception domains hangi sınıflarda tanımlanmalı?

**Durum:** APPROVED

**RECOMMENDED taxonomy:**

1. **Network Deception** — fake hosts, ports, services, protocols.
2. **Application Deception** — fake web apps, databases, admin panels, internal tools.
3. **Endpoint Deception** — fake files, configs, connection profiles, bookmarks, artefacts.
4. **Credential/Secret Deception** — fake passwords, keys, API tokens, configs.
5. **Identity Deception** — fake users, service accounts, AD/cloud identities.
6. **Data Deception** — fake documents/shares/records that trigger interaction.
7. **AI Infrastructure Deception** — fake LLM/API/MCP/vector/RAG assets.
8. **High-Interaction Environment** — VM-backed or otherwise stateful interactive systems.

Bu taxonomy “MVP listesi” değildir; product capability universe'ın kavramsal haritasıdır.

---

## D-02 — Interaction levels nasıl tanımlanmalı?

**Durum:** APPROVED

**APPROVED direction'ın formal önerisi:**

- **Low interaction:** connection/login/protocol seviyesinde sınırlı simulation; düşük risk ve operasyon yükü.
- **Medium interaction:** attacker'a meaningful session/command/application behavior sunan emulated environment.
- **High interaction:** gerçek veya yüksek gerçekçilikte OS/app runtime; saldırganın daha fazla davranış sergilemesine izin verir; isolation/risk yükü yüksektir.

Ayrıca “interaction level” ile “deception surface” ayrı eksen olmalıdır. Bir credential token low-interaction olabilir ama signal çok yüksek olabilir.

---

## D-03 — Default product strategy hangi interaction seviyesinde başlamalı?

**Durum:** APPROVED

**RECOMMENDED — Low/medium default; high interaction controlled advanced capability.**

Gerekçe:

- High interaction daha gerçekçi telemetry sağlar.
- Fakat compromise/pivot, resource, lifecycle ve safety riskini büyütür.
- Core product promise çoğu zaman high interaction gerektirmeden decoy interaction ile karşılanabilir.

---

## D-04 — Kullanıcı “service” mi, “persona” mı deploy etmeli?

**Durum:** APPROVED

**RECOMMENDED — İki seviye.**

Primary UX persona/template driven olmalı. Advanced kullanıcı gerektiğinde servisleri özelleştirebilir. Bu product choice ileride template schema ve capability composition gereksinimi yaratır; implementation seçilmez.

---

## D-05 — Persona realism neye göre belirlenmeli?

**Durum:** APPROVED

**RECOMMENDED:** Persona, yalnız banner/name değişikliği değil, environment ile tutarlı bir kimlik taşımalıdır:

- plausible hostname,
- plausible services,
- plausible OS/application profile,
- plausible credentials/artefacts,
- network segment role ile uyum,
- stale/contradictory metadata'dan kaçınma.

Realism amacı saldırganı sonsuza kadar kandırmak değil; detection eşiğine kadar inandırıcı ve güvenli bir interaction sağlamaktır.

---

## D-06 — Endpoint breadcrumbs ürün vizyonunda hangi statüde?

**Durum:** APPROVED

**RECOMMENDED — Extended Deception / future capability.**

Örnekler:

- fake RDP/SSH profile,
- fake config,
- fake document,
- fake network share shortcut,
- fake credential/token,
- browser artefact.

Primary network product definition bunlara bağımlı olmamalıdır; fakat capability universe'ta tutulmalıdır.

---

## D-07 — Identity deception hangi statüde?

**Durum:** APPROVED

**RECOMMENDED — Strategic future capability.**

Identity deception lateral movement/credential abuse detection için yüksek sinyal potansiyeline sahiptir. Ancak AD/cloud identity integration ve lifecycle gereksinimleri daha derin teknik/security çalışma ister. Current product promise'in zorunlu ilk mechanism'i yapılmamalıdır.

---

## D-08 — Windows deception'ın ürün rolü nedir?

**Durum:** APPROVED

**RECOMMENDED — Windows tek bir runtime requirement olarak değil, deception persona ailesi olarak düşünülmeli:**

- Windows workstation-like persona
- Windows file server/SMB persona
- RDP/remote admin persona
- Domain/identity-adjacent deception
- Windows service/application persona

Gerçek Windows OS çalıştırma gerekip gerekmediği Step 3/4 teknik kararıdır.

---

## D-09 — Vulnerable VM'in rolü

**Durum:** APPROVED

Formal ürün kuralı:

> **Intentionally vulnerable full systems, daha fazla attacker behavior toplamak için seçilebilir advanced deception environments'tır; ürünün standart detection coverage'ı bunlara bağlı değildir.**

---

## D-10 — AI infrastructure deception

**Durum:** APPROVED

**RECOMMENDED — Future opportunity; current core değil.**

Özellikle software/AI-heavy müşteriler için ileride güçlü differentiation olabilir.

---

## D-11 — Adaptive deception ürün vizyonunda mı?

**Durum:** APPROVED

**RECOMMENDED — Long-term vision: human-governed adaptive deception.**

Örnek:

- Source X recon davranışı gösterir.
- Sistem “bu segmentte ek Git/DB/SMB decoy yüzeyi oluşturmayı” önerir.
- Policy veya insan onayıyla deception genişler.

AI'nın attacker behavior'a karşı kendi kendine network topology değiştirmesi current product principle değildir.

---

## D-12 — Deception'ın güvenlik sınırı

**Durum:** APPROVED

**RECOMMENDED product safety principle:**

- Decoy production secret içermez.
- Compromise varsayımıyla tasarlanır.
- Attacker'ın decoy üzerinden production'a pivot etmesi engellenmelidir.
- Outbound abuse riskleri control requirement olarak ele alınmalıdır.
- High-interaction capability daha yüksek isolation class'ına sahip olmalıdır.
- Deception telemetry untrusted input kabul edilmelidir.

Nasıl izole edileceği Step 3/4 konusudur.

---

## D-13 — Coverage “çok decoy” ile mi, “doğru touchpoint” ile mi ölçülmeli?

**Durum:** APPROVED

**RECOMMENDED — Meaningful deception coverage.**

Coverage kavramı decoy count değil:

- kritik network zones,
- plausible attacker paths,
- credential discovery opportunities,
- remote administration paths,
- internal services

gibi saldırganın keşif/movement sırasında karşılaşabileceği touchpoint'lerin temsil edilmesi olarak düşünülmelidir.

---

## 2.8 Approved Section Output

| Domain | Interaction | Product role | Current status |
|---|---|---|---|
| Fake network host/service | Low | Core mechanism | Vision Core |
| Service login/credential trap | Low/Medium | Core high-signal detection | Vision Core |
| Application persona (DB/web/admin) | Low/Medium | Core/Extended | Vision Core |
| Interactive shell/session | Medium | Deeper attacker observation | Extended |
| Endpoint breadcrumbs | Low | Discovery/credential lure | Extended/Future |
| Honey credentials/secrets | Low | Very high-signal use detection | Extended/Future |
| Identity deception | Low/Medium | Credential/lateral detection | Future Strategic |
| High-interaction VM | High | Behavior/forensics | Advanced |
| Vulnerable VM | High | Attacker engagement | Advanced |
| AI-generated interactive persona | Medium/High | Realism/engagement | Future R&D |
| Adaptive deception | Variable | Dynamic coverage | Future Strategic |
| AI/LLM/MCP decoys | Variable | Emerging attack surface | Future Opportunity |

**Durum: RECOMMENDED — OWNER DECISION REQUIRED**


# 2.9 Signal, Confidence & False Positive Model

## S-01 — Event → Signal → Evidence → Finding → Incident zinciri kabul edilmeli mi?

**Durum:** APPROVED

**RECOMMENDED — Evet.**

Ürün information modelinde aşağıdaki ayrım formalize edilmelidir:

**Event**  
Bir sensörün gözlediği atomik olaydır. Örnek: TCP connection, login attempt, file read, command.

**Signal**  
Event'in security anlamı taşıyan yorumudur. Örnek: “normal kullanıcının erişmemesi gereken decoy'a authentication attempt”.

**Evidence**  
Bir claim'i destekleyen, provenance'ı ve zamanı bilinen gözlemdir. Raw event'in tamamı veya normalize edilmiş temsil olabilir.

**Finding**  
Bir veya daha fazla evidence'dan çıkarılmış anlamlı güvenlik bulgusudur. Örnek: “credential abuse attempt”.

**Incident**  
Birlikte değerlendirilmesi gereken related findings/evidence bütünüdür.

Bu ayrım AI açıklamasının evidence'dan kopmasını önler ve incident-centric UX için temel oluşturur.

---

## S-02 — Confidence ve Severity iki ayrı eksen mi?

**Durum:** APPROVED

**RECOMMENDED — Evet; tek risk score'a indirgenmemeli.**

**Confidence:** “Bu davranışın beklenmeyen/suspicious olduğundan ne kadar eminiz?”

**Severity:** “Eğer olay gerçek malicious activity ise potansiyel iş/güvenlik etkisi ne kadar büyük?”

Örnekler:

| Örnek | Confidence | Severity |
|---|---|---|
| Known scanner tek port kontrolü | Low/Medium | Low |
| Unknown host geniş subnet scan | Medium | Medium |
| Honey credential kullanımı | Very High | Medium/High |
| Decoy admin login + command execution | Very High | High |
| Benign IT test olduğu doğrulanan SSH login | Evidence strong, malicious confidence low | Low |

Bu iki boyut kullanıcıya ayrı gösterilmelidir.

---

## S-03 — Alerting philosophy precision-first mı recall-first mı?

**Durum:** APPROVED

**RECOMMENDED — B default, C advanced visibility olarak.**

Primary persona SOC analyst değildir. Default notification kanalı precision-first olmalıdır. Bununla birlikte ürün event explorer veya lower-confidence findings görünürlüğünü tamamen yok etmemelidir. Böylece:

- **Notifications:** high-confidence / high-importance.
- **Console findings:** daha geniş context.
- **Raw events:** drill-down/advanced investigation.

---

## S-04 — “Zero false positives” iddiası kullanılmalı mı?

**Durum:** APPROVED

**RECOMMENDED — Hayır.**

Deception yüksek signal-to-noise sağlayabilir; ancak legitimate scanners, monitoring, admin tests veya yanlış placement false-positive-benzeri olaylara yol açabilir. “Near-zero” gibi competitor language mevcut olsa da bizim product principle'ımız doğrulanabilir ve ölçülebilir iddiadır. [S5][S6]

Önerilen dil:

> **“High-confidence, low-noise detection designed around interactions that legitimate users normally should not perform.”**

Sayısal/absolute claim ancak gerçek production data ile doğrulanırsa kullanılır.

---

## S-05 — Known scanner / trusted automation nasıl ele alınmalı?

**Durum:** APPROVED

**RECOMMENDED — Combination, kullanıcı kontrolünde.**

Product semantics:

- Known/trusted source tanımlanabilir.
- Sadece belirli behavior suppression yapılabilir; ör. port scan ignore, credential login yine alarm.
- Ürün suspected known-tool activity'yi annotate edebilir.
- Suppression reversible ve audit-visible olmalıdır.
- AI “bu Defender scan” diyebilir ama tek başına kalıcı suppression yaratmamalıdır.

---

## S-06 — Correlation confidence nasıl ele alınmalı?

**Durum:** APPROVED

**RECOMMENDED:** Incident içinde iki farklı güven kavramı gerekebilir:

1. **Finding confidence:** tek bir davranışın malicious/suspicious olma güveni.
2. **Linkage confidence:** iki event/finding'in aynı source/actor/session'a ait olduğuna dair güven.

Bu distinction attacker journey'nin sahte kesinlik üretmesini engeller.

---

## S-07 — Source identification confidence gösterilmeli mi?

**Durum:** APPROVED

**RECOMMENDED — Evet.**

Örnek:

- Source IP: observed directly — high confidence.
- Hostname: DHCP/lookup correlation — medium/high.
- User: inferred from VPN/session mapping — medium.
- “Employee X”: yalnız asset inventory mapping varsa.

Ürün “10.0.0.4 = Ahmet'in laptopu” sonucunu kanıt yoksa kesinleştirmemelidir.

---

## S-08 — Deduplication product requirement mı?

**Durum:** APPROVED

**RECOMMENDED — Evet.**

Aynı source'un 500 tekrar login denemesi 500 notification olmamalıdır. Product semantics:

- repeated related events incident altında aggregate edilir,
- meaningful state change notification üretebilir,
- raw count/evidence korunur.

Time-window algoritması sonraki teknik tasarım konusudur.

---

## S-09 — Risk/severity scoring AI tarafından mı yapılmalı?

**Durum:** APPROVED

**RECOMMENDED — Üçüncü model.**

Severity için deterministic inputs (interaction depth, credential use, command execution, repeat behavior vb.) temel oluşturabilir. AI business/context implication ekleyebilir. Son kullanıcıya “AI says 93/100” gibi opaque skor yerine nedenler gösterilmelidir.

---

## S-10 — Unknown/uncertain state ürün modelinde olmalı mı?

**Durum:** APPROVED

**RECOMMENDED — Evet.**

Security ürünleri çoğu zaman forced binary “malicious/benign” sınıflandırmaya düşer. Bunun yerine:

- Known benign
- Likely benign
- Unknown / needs review
- Suspicious
- High-confidence malicious-like interaction

şeklinde kontrollü uncertainty dili kullanılabilir. Exact labels final UX aşamasında refine edilir.

---

## S-11 — Evidence retention product requirement mı?

**Durum:** APPROVED

**RECOMMENDED — Evet, ama retention süresi/teknolojisi değil.**

Bir incident sonradan incelendiğinde explanation'ın dayandığı evidence kaybolmamalıdır. Product requirement:

- incident'in kararını destekleyecek minimum evidence,
- timeline,
- provenance,
- relevant payload/command metadata

yeterli süre boyunca tutulabilmelidir.

Retention duration, storage tier ve privacy implementation daha sonra belirlenir.

---

## S-12 — Telemetry privacy product principle mı?

**Durum:** APPROVED

**RECOMMENDED — Evet.**

Attacker interaction telemetry şu içerikleri taşıyabilir:

- gerçek username/password denemeleri,
- uploaded files,
- commands,
- IP/device identifiers,
- possibly personal data,
- secrets accidentally used against decoy.

Ürün “deception data = harmless data” varsaymamalıdır. Data minimization, access control, retention ve AI provider'a gönderilecek içerik ayrı product/privacy policy gerektirir.

---

## S-13 — Sensor/decoy health security signal olarak ayrı mı tutulmalı?

**Durum:** APPROVED

**RECOMMENDED — Evet.**

“Hiç alert yok” iki farklı anlama gelebilir:

- saldırı/interaction yok,
- sensör çalışmıyor.

Bu nedenle product loop'ta coverage/health state görünürlüğü core operational requirement olmalıdır. Health alarmı security incident ile karıştırılmamalıdır.

---

## 2.9 Approved Section Output

1. **High signal over high volume.**
2. **Connection tek başına her zaman alert değildir.**
3. **Interaction depth confidence'ı artırabilir.**
4. **Confidence ≠ Severity.**
5. **Correlation evidence'ı gizlemez.**
6. **Known benign activity suppress/annotate edilebilir.**
7. **Suppression reversible ve audit-visible'dır.**
8. **AI classification evidence'ın yerine geçmez.**
9. **Unknown kabul edilebilir bir sonuçtur.**
10. **Health failure “no incidents” olarak yorumlanmaz.**
11. **Absolute zero-false-positive claim yapılmaz.**
12. **Evidence provenance kullanıcı tarafından görülebilir.**

**Durum: RECOMMENDED — OWNER DECISION REQUIRED**

---


# 2.10 AI-Native Product Model

## AI-01 — AI ürünün hangi probleminde “necessary differentiator” olabilir?

**Durum:** APPROVED

**RECOMMENDED priority hierarchy:**

**Tier 1 — Core AI value**
1. Incident narrative/explanation
2. Investigation assistance
3. Context-aware remediation guidance
4. Soft correlation hypotheses / attacker journey enrichment

**Tier 2 — Supporting AI value**
5. Environment/deception placement suggestions
6. Incident summarization across time
7. Natural-language exploration of evidence

**Tier 3 — Advanced/Future**
8. Adaptive deception recommendations
9. Persona/config generation
10. Attacker-facing dynamic interaction
11. AI infrastructure decoys

Bu sınıflandırma MVP seçimi değildir; product-value order'dır.

---

## AI-02 — “AI evidence üretmez” ilkesi nasıl formalize edilmeli?

**Durum:** APPROVED

**RECOMMENDED product rule:**

> **Security truth = observed telemetry + deterministic/system-derived evidence. AI bu evidence'ı organize edebilir, ilişkilendirebilir, yorumlayabilir ve hypothesis üretebilir; gözlenmemiş olayı gerçekmiş gibi incident evidence'a dönüştüremez.**

AI'nın çıkardığı sonuçlar tür olarak işaretlenmelidir:

- Observed fact
- Deterministic inference
- AI-assisted hypothesis
- Recommendation

Bu provenance UI'da görünür olmalıdır.

---

## AI-03 — AI incident correlation yapabilir mi?

**Durum:** APPROVED

**RECOMMENDED — Üçüncü model.**

Hard correlation örnekleri deterministic olabilir: aynı source/session veya direct token relationship. AI daha yumuşak ilişki önerebilir: benzer sequence, intent veya temporal context. Ancak “aynı attacker” sonucu confidence ile ve gerekçeyle sunulmalıdır.

---

## AI-04 — AI remediation guidance nasıl güvenli tutulmalı?

**Durum:** APPROVED

**RECOMMENDED — Recommendation must be evidence-grounded, ordered, reversible-aware and uncertainty-aware.**

AI şu kurallara tabi olmalıdır:

- Olay evidence'ına referans verir.
- Customer environment hakkında bilmediği şeyi uydurmaz.
- Destructive/availability-impacting action'ı otomatik uygulamaz.
- “Şunu kesin yap” yerine risk/impact belirtir.
- Gerekirse “security professional'a escalate et” seçeneği sunar.
- Production command/script üretse bile kullanıcı approval olmadan execute etmez.

---

## AI-05 — AI provider agnostic product principle olmalı mı?

**Durum:** APPROVED

**RECOMMENDED — Evet, product principle seviyesinde.**

Ama “her model aynı kalitede çalışacak” şeklinde teknik garanti değildir. Product requirements:

- core product semantics model vendor'a bağlanmamalı,
- model capability/failure değişimine dayanıklı olmalı,
- data policy provider-specific olabilir ama açık olmalı,
- future local/on-prem model path mümkün olmalı.

Spesifik provider/model seçimi Adım 4'e bırakılır.

---

## AI-06 — AI detection için hard dependency olabilir mi?

**Durum:** APPROVED

**RECOMMENDED — Hayır.**

AI unavailable olduğunda:

- decoy interaction kaydı,
- evidence capture,
- deterministic detection,
- incident persistence,
- basic alerting

çalışmaya devam etmelidir.

AI capability degraded olabilir; security visibility tamamen kaybolmamalıdır.

---

## AI-07 — Attacker-facing LLM interaction'ın statüsü

**Durum:** APPROVED

**RECOMMENDED — Advanced R&D capability.**

Potansiyel fayda:

- daha uzun attacker engagement,
- statik fake shell fingerprinting'ini azaltma,
- daha gerçekçi application behavior,
- yeni TTP evidence.

Riskler:

- latency fingerprinting,
- persona inconsistency/hallucination,
- prompt injection,
- denial-of-wallet,
- misuse as proxy/tool,
- outbound actions,
- sensitive data leakage,
- unpredictable content.

Bu riskler çözülmeden core detection'a bağlanmamalıdır.

---

## AI-08 — Attacker telemetry LLM'e güvenilir input mu?

**Durum:** APPROVED

**RECOMMENDED — Kesinlikle hayır.**

Attacker-controlled commands, HTTP payloads, filenames, documents ve prompt-like strings **untrusted adversarial input** kabul edilmelidir. AI pipeline product requirement'ları:

- instruction/data separation,
- prompt injection resistance,
- tool access restrictions,
- output validation,
- sensitive context minimization,
- audit trail.

Implementation daha sonra seçilir.

---

## AI-09 — AI'ın environment understanding rolü

**Durum:** APPROVED

**RECOMMENDED — İkinci model.**

Örnek:

> “Bu subnet'te gerçek Windows file servers ve PostgreSQL hizmetleri var; attacker için plausible fakat production'dan ayrıştırılabilir iki deception persona öneriyorum.”

Bu öneri kullanıcı onayına tabidir.

---

## AI-10 — AI persona/deception generation

**Durum:** APPROVED

**RECOMMENDED — Future supporting capability; generated artefact policy/template validation'dan geçmeli.**

AI, fake hostname, service mix, fake files veya application content önerebilir. Ancak generated deception:

- gerçek customer secret kullanmamalı,
- accidental production collision yaratmamalı,
- safe-by-default constraints'i aşmamalıdır.

---

## AI-11 — Adaptive deception AI tarafından otomatik yürütülebilir mi?

**Durum:** APPROVED

**RECOMMENDED — Current principle: AI proposes, policy/human approves.**

Gelecekte bounded autonomy değerlendirilebilir; örneğin önceden onaylanmış safe template seti içinde otomatik decoy activation. Fakat ürün sahibinin “AI agent product/architecture/security decision almaz” felsefesiyle uyumlu biçimde autonomy sınırları explicit olmalıdır.

---

## AI-12 — Cloud AI telemetry privacy modeli

**Durum:** APPROVED

**RECOMMENDED product policy:**

- AI'a gönderilecek data sınıfları açıkça tanımlanmalı.
- Raw credential/payload gibi sensitive fields mümkün olduğunda minimize/redact edilmeli.
- Kullanıcı hangi data'nın cloud'a gidebileceğini anlayabilmeli.
- “No AI cloud data” future mode'u product architecture requirement olarak korunmalı.
- Provider training/retention claims teknik/legal seçim sırasında ayrıca incelenmeli.

---

## AI-13 — Denial-of-wallet product risk olarak kayıtlı mı?

**Durum:** APPROVED

**RECOMMENDED — Evet, explicit risk.**

Attacker-controlled interaction model çağrılarını tetikliyorsa saldırgan maliyet yaratabilir. Product-level guardrails:

- attacker-facing AI core dependency olmamalı,
- budget/rate policy gerekecek,
- repeated interaction cache/bounded context gibi teknik seçenekler Step 3/4'te değerlendirilecek.

---

## AI-14 — AI explanation “chat” mi, structured incident mı?

**Durum:** APPROVED

**RECOMMENDED — Structured incident first; optional conversational exploration second.**

Chat, product value'nın kendisi değildir. İlk değer:

- “What happened”
- “Why important”
- “Evidence”
- “What next”

alanlarının güvenilir biçimde doldurulmasıdır. Chat daha sonra “bu incident'i bana basitçe açıkla” gibi exploration sağlayabilir.

---

## AI-15 — AI quality nasıl product requirement yapılmalı?

**Durum:** APPROVED

**RECOMMENDED — AI features için eval-driven product governance gerekli.**

Metrik aileleri:

- Evidence fidelity: gözlenmeyen fact uyduruyor mu?
- Action safety: riskli/destructive suggestion veriyor mu?
- Completeness: kritik evidence'ı atlıyor mu?
- Uncertainty calibration: bilmediğini söylüyor mu?
- Persona clarity: generalist IT anlayabiliyor mu?
- Consistency: aynı evidence için benzer sonuç veriyor mu?

Bu testlerin framework/implementation'ı sonraki AI architecture çalışmasının parçasıdır.

---

## AI-16 — AI generated severity/confidence kullanıcı tarafından override edilebilir mi?

**Durum:** APPROVED

**RECOMMENDED — Kullanıcı classification/feedback verebilmeli; system score provenance korunmalı.**

AI veya system judgement ile human disposition birbirine karıştırılmamalıdır. Örneğin system “high confidence suspicious”, user “known pentest” diye resolve edebilir. Bu feedback öğrenme ve audit için ayrı saklanmalıdır.

---

## AI-17 — AI infrastructure deception ürün vizyonunda kalmalı mı?

**Durum:** APPROVED

**RECOMMENDED — Evet, “Future Opportunity” statüsünde.**

Fake LLM endpoint, MCP server, vector DB, internal AI API veya fake RAG data store özellikle software/AI organizations için gelecekte yüksek-value lure olabilir. Current customer/problem definition bunu gerektirmemektedir. [S9]

---

## 2.10 Approved Section Output

1. **Evidence before inference.**
2. **Observed fact ile AI hypothesis ayrılır.**
3. **AI unavailable ise detection devam eder.**
4. **AI recommendation, human decision'ın yerine geçmez.**
5. **Attacker-controlled input adversarial kabul edilir.**
6. **Structured incident value, chatbot novelty'den önce gelir.**
7. **Cloud AI kullanımı transparent ve minimize edilmiş data ile yapılır.**
8. **Provider agnostic product contract korunur.**
9. **Autonomy bounded ve reversible olmalıdır.**
10. **AI quality eval'lerle ölçülür.**

**Durum: RECOMMENDED — OWNER DECISION REQUIRED**

---


# 2.11 Product Capability Universe

## CU-01 — Capability sınıfları

**Durum:** APPROVED

**RECOMMENDED sınıflandırma:**

- **Core:** Product promise'i tanımlayan capability.
- **Supporting:** Core value'nun güvenli/kullanılabilir olmasını sağlayan capability.
- **Advanced:** Daha fazla depth/interaction/intelligence sağlayan capability.
- **Future Strategic:** Uzun vadeli ürün yönü için önemli ama current core promise'e gerekli değil.
- **Explicitly Excluded:** Bilinçli non-goal.

---

## CU-02 — Capability tree'de “Core” ne anlama gelir?

**Durum:** APPROVED

**RECOMMENDED — Core = ürün kimliğinin uzun vadeli minimum sözleşmesi; MVP zorunluluğu değildir.**

Bu distinction özellikle önemlidir. Örneğin source entity resolution ürün kimliği için core olabilir ama ilk MVP'de teknik fizibiliteye göre sınırlı bir versiyonu bulunabilir. MVP kapsamı Adım 3/4 sonrası seçilecektir.

---

## CU-03 — Product capability universe Guardpot/competitor parity hedeflemeli mi?

**Durum:** APPROVED

**RECOMMENDED — Hayır.**

Rakip capability, “pazarda mümkün/istenen olabilir” sinyali sağlar; ürün backlog'unu otomatik oluşturmaz. Her capability şu testlerden geçmelidir:

1. Hangi JTBD'yi çözüyor?
2. North Star'a katkısı nedir?
3. Primary persona gerçekten kullanır mı?
4. Signal quality artırıyor mu, noise mı yaratıyor?
5. Operational burden ne yönde değişiyor?
6. Risk/safety sınırı nedir?
7. Differentiation'a katkısı var mı?

---

## 2.11 Approved Section Output

Yukarıdaki Capability Universe'ın **tam ürün uzayı** olarak kabul edilmesi; fakat **MVP seçiminin yapılmaması** önerilir.

**Durum: RECOMMENDED — OWNER DECISION REQUIRED**

---


# 2.12 Differentiation, Product Principles & Positioning

## DP-01 — Ana differentiation tezi nedir?

**Durum:** APPROVED

**RECOMMENDED — D, fakat messaging tek cümlede outcome'a indirgenmeli.**

Stratejik differentiation bileşenleri:

1. **High-signal deception** — noise değil meaningful touch.
2. **Attacker journey** — tek alarm değil story.
3. **AI-assisted investigation** — telemetry'yi generalist IT için anlaşılır hale getirme.
4. **Guided response** — “ne yapacağım?” sorusuna context-aware cevap.
5. **Operational simplicity** — SOC-tool complexity'sinden kaçınma.
6. **Progressive deception depth** — low → medium → high capability path.

Bu altı unsurun hepsi headline'da olmak zorunda değildir; differentiation thesis'te birlikte bulunur.

---

## DP-02 — Ürün kategori adı ne olmalı?

**Durum:** APPROVED

**RECOMMENDED:**

**Primary category:** **Internal Breach Detection & Deception Platform**  
**Descriptor:** for security-lean / SOC-less organizations.

“Honeypot” mekanizma/SEO/technical explanation olarak kullanılır; category headline'ı yalnız honeypot'a kilitlenmez.

---

## DP-03 — “Honeypot” kelimesi pazarlamada ne kadar önde olmalı?

**Durum:** APPROVED

**RECOMMENDED — Technical explanation'da görünür, value proposition'da secondary.**

Örnek:

> “Deploy realistic decoys and honey assets inside your network to catch suspicious movement early — then see the attack journey and what to do next.”

Burada mekanizma açık ama headline kullanıcı sonucudur.

---

## DP-04 — EDR/NDR olan müşteriye “neden bize ihtiyaç var?” cevabı

**Durum:** APPROVED

**RECOMMENDED differentiation answer:**

- EDR production endpoints üzerinde geniş telemetry toplar; deception ise “legitimate user'ın normalde dokunmaması gereken” varlıklarda intentional tripwire yaratır.
- NDR network behavior'ını genel olarak analiz eder; deception known-fake assets üzerinden yüksek-context interaction signal üretir.
- Ürün bunların yerine geçmek yerine breach detection confidence'ını yükselten complementary layer'dır.
- SOC-less kullanıcı için fark, ayrı tool telemetry'sini yorumlamak yerine incident story ve guidance görmesidir.

Bu claim'ler ürün performansı doğrulanınca daha kesin hale getirilecektir.

---

## DP-05 — OpenCanary'ye karşı differentiation

**Durum:** APPROVED

**RECOMMENDED answer:**

OpenCanary lightweight multi-protocol honeypot olarak güçlü bir reference implementation'dır ve private network breach detection use-case'ini açıkça hedefler. [S11]

Bizim ürün değerimiz engine parity değil:

- guided deployment/placement,
- persona catalogue,
- multi-decoy lifecycle,
- normalized evidence,
- incident correlation,
- attacker journey,
- source identification,
- AI explanation,
- response guidance,
- product health/operations.

Dolayısıyla “OpenCanary'yi yeniden yazmak” differentiation değildir.

---

## DP-06 — Thinkst Canary'ye karşı differentiation

**Durum:** APPROVED

**RECOMMENDED answer:**

Thinkst Canary'nin güçlü benchmark'ı simplicity ve high-signal alarmdır. [S5][S6]

Bizim olası ayrışma tezimiz:

> **Canary-like simplicity/high-signal philosophy + daha derin attacker journey/investigation + progressive deception capabilities + SOC-less operator'a AI-assisted response guidance.**

Bu bir superiority claim değildir; ürün yönü hipotezidir.

---

## DP-07 — Guardpot ve enterprise deception'a karşı differentiation

**Durum:** APPROVED

**RECOMMENDED answer:**

Guardpot ve enterprise deception platformları geniş capability surface sunabilir. [S8][S9][S10]

Bizim farklılık tezimiz:

- daha dar primary problem,
- security-lean operator,
- low operational complexity,
- incident/action clarity,
- feature accumulation yerine purposeful deception.

Rakibin VPN, mail security, ASM veya full TI capability'sini kopyalamak hedef değildir.

---

## DP-08 — Moat nerede oluşabilir?

**Durum:** APPROVED

**RECOMMENDED — Tek moat varsayımı yapılmamalı; compound product moat hedeflenmeli.**

En güçlü aday bileşim:

> **Environment-aware deception orchestration + high-quality normalized evidence + incident correlation/attacker journey + AI investigation/guidance quality + operational simplicity.**

Open-source honeypot engines nedeniyle protocol emulation tek başına sürdürülebilir moat değildir.

---

## DP-09 — Product Principles final seti

**Durum:** APPROVED

**RECOMMENDED — Bu 12 ilke Product Principles v1 olarak owner approval'a sunulsun.**

---

## DP-10 — Positioning statement seçenekleri

**Durum:** APPROVED

**RECOMMENDED — B ana positioning; A category descriptor olarak destek; C kullanılmasın veya secondary messaging olsun.**

---

## DP-11 — Ürün sözü hangi capability seviyesinde olmalı?

**Durum:** APPROVED

**RECOMMENDED formal product strategy:**

> **Core product promise Level C. Level D ve E differentiation expansion path'tir; current product identity'nin kabul kriteri değildir.**

---

## DP-12 — AI branding ürün isminde/positioning'de merkezi olmalı mı?

**Durum:** APPROVED

**RECOMMENDED — AI capability güçlü biçimde görünür olabilir; fakat category/value promise AI'a bağımlı yazılmamalı.**

Neden:

- model/provider hızla commoditize olabilir,
- güvenlik buyer'ı outcome ister,
- AI olmayan degraded mode ürünün core detection'ını sürdürmelidir,
- “AI-powered” tek başına differentiation değildir.

Doğru kullanım: “AI-assisted investigation and response guidance” gibi spesifik outcome.

---

## DP-13 — Product complexity ceiling

**Durum:** APPROVED

**RECOMMENDED product rule:**

Primary operator günlük kullanım için şu işleri yapmak zorunda kalmamalıdır:

- detection rule language yazmak,
- SIEM query yazmak,
- packet analysis yapmak,
- honeypot package dependency yönetmek,
- VM image patch pipeline yönetmek,
- LLM prompt engineering yapmak,
- her scanner için manuel tuning projesi yürütmek.

Advanced mode olabilir; default product deneyimi bunları abstract etmelidir.

---

## DP-14 — Initial commercial motion product definition'ı etkilemeli mi?

**Durum:** APPROVED

**RECOMMENDED — İlk ürün mümkün olduğunca low-friction evaluation/deployment'a uygun tasarlansın; enterprise professional-services dependency primary product requirement olmasın.**

Bu, “self-service satış modeli kesin seçildi” anlamına gelmez. Yalnız product UX'in demo/consultant olmadan anlaşılabilir olmasını hedefler.

---

## 2.12 Approved Section Output

> **Ürün, başka bir honeypot engine veya geniş enterprise security suite olmaya çalışmaz. SOC'u olmayan/security-lean organizasyonlarda internal breach sonrası discovery, credential abuse ve lateral movement'i deception ile yüksek-sinyalle görünür hale getirir; parçalı interaction'ları attacker journey olarak ilişkilendirir; evidence ile AI yorumunu ayırarak ne olduğunu ve hangi aksiyonun izlenmesi gerektiğini açıklar. Bunu enterprise deception platformlarının operasyonel karmaşıklığını mümkün olduğunca taşımadan yapmayı hedefler.**

**Durum: RECOMMENDED — OWNER DECISION REQUIRED**

---

# 2.7 Approved Incident/User Journey Synthesis

2.7 kapsamındaki I-01–I-12 kararlarının tamamı APPROVED’dur. Buna göre ürünün primary investigation object’i **Incident**’tır; event/signal/evidence/finding/incident kavramları ayrıdır; attacker journey first-class product object’tir; confidence ve severity ayrıdır; evidence provenance görünürdür; MITRE ATT&CK secondary enrichment’tır; notifications incident/significant finding seviyesinde üretilir; incident lifecycle ve disposition’lar açıkça tutulur; response guidance context-aware, sıralı ve human-controlled’dür; kapatılan incident kalıcı ve incelenebilir bir history olarak korunur.

---

# 3. Consolidated APPROVED Decision Register

| ID | Karar konusu | Onaylanan yönün kısa özeti | Durum |
|---|---|---|---|
| P-01 | Ürünün çözmeye çalıştığı birincil problem nedir? | **RECOMMENDED — D seçeneği.** | APPROVED |
| P-02 | Ürünün odaklandığı güvenlik anı nerede başlar? | **RECOMMENDED — B.** | APPROVED |
| P-03 | Deception ürünün kendisi mi, yoksa ana mekanizması mı? | **RECOMMENDED — B, fakat kategori açıklamasında deception açıkça görünür.** | APPROVED |
| P-04 | Ürün mevcut security stack'in yerine mi geçer, tamamlayıcı mı olur? | **RECOMMENDED — B.** | APPROVED |
| P-05 | Ürün “detect” ile mi biter, “respond” alanına da girer mi? | **RECOMMENDED — C.** | APPROVED |
| P-06 | Ürünün birincil başarı sonucu nedir? | **RECOMMENDED — başarı “volume” değil “decision quality” üzerinden tanımlansın.** | APPROVED |
| P-07 | Hangi problem varsayımları henüz doğrulanmamış kabul edilmelidir? | Aşağıdakiler **ASSUMPTION — VALIDATION NEEDED** olarak tutulmalıdır: | APPROVED |
| C-01 | İlk ICP çalışan sayısına mı, security maturity'ye mi göre tanımlanmalı? | **RECOMMENDED — C primary, headcount secondary heuristic.** | APPROVED |
| C-02 | İlk müşteri ortamı hangi altyapı profilini taşımalı? | **RECOMMENDED — B, ancak ilk değer ispatı internal routable network olan ortamlarda.** | APPROVED |
| C-03 | Primary operator kimdir? | **RECOMMENDED — Primary operator: generalist technical owner (IT manager/sysadmin/DevOps/CTO); dedicated security analyst secondary persona.** | APPROVED |
| C-04 | Hiç IT personeli olmayan şirket primary persona olabilir mi? | **RECOMMENDED — C.** | APPROVED |
| C-05 | Buyer kimdir? | **RECOMMENDED — Economic buyer ve operator küçük organizasyonda çoğu zaman çakışır; primary buyer teknik bütçe sahibi CTO/IT manager/owner-operator olmalıdır.** | APPROVED |
| C-06 | Incident responder kim kabul edilmeli? | **RECOMMENDED — Primary responder aynı generalist technical operator; secondary escalation path external IT/security provider.** | APPROVED |
| C-07 | İlk ürün tek bir sektöre mi odaklanmalı? | **RECOMMENDED — A başlangıç product definition; verticalization gelecekte GTM doğrulamasına göre.** | APPROVED |
| C-08 | Windows-heavy ortamlara yaklaşım | **RECOMMENDED — Customer model “Linux-only” varsaymamalı; Windows workstation/server, SMB/RDP/identity davranışları uzun vadeli threat/deception modelinin doğal parçası sayılmalıdır.** | APPROVED |
| C-09 | MSP/MSSP persona'sı ne durumda tutulmalı? | **RECOMMENDED — “Future strategic persona” olarak kayıtlı kalsın, ancak mevcut product decisions tek-customer/single-environment mental model ile verilsin.** | APPROVED |
| C-10 | Explicit Non-Personas kimlerdir? | Aşağıdaki kullanıcı grupları ilk product definition'ın **NON-PERSONA** listesine alınsın: | APPROVED |
| J-01 | Primary functional job nedir? | **RECOMMENDED:** | APPROVED |
| J-02 | Detection JTBD ne kadar geniş olmalı? | **RECOMMENDED:** Kullanıcı “her saldırıyı tespit et” beklentisine sokulmamalıdır. Detection job şu şekilde tanımlansın: | APPROVED |
| J-03 | Investigation JTBD ürünün çekirdeğinde mi? | **RECOMMENDED — Evet, çekirdekte.** | APPROVED |
| J-04 | Attribution/source-identification JTBD ne olmalı? | **RECOMMENDED — “Source identification” kullanılsın; “real-world attacker attribution” ürün sözü olmasın.** | APPROVED |
| J-05 | Severity assessment JTBD | **RECOMMENDED:** Kullanıcı tek başına raw MITRE technique veya port numarasından severity çıkarmak zorunda kalmamalıdır. Ürün: | APPROVED |
| J-06 | Response JTBD'nin kapsamı | **RECOMMENDED — Context-aware recommendation, human approval.** | APPROVED |
| J-07 | Operational JTBD | **RECOMMENDED:** | APPROVED |
| J-08 | Confidence/noise JTBD | **RECOMMENDED:** | APPROVED |
| J-09 | Product feedback JTBD | **RECOMMENDED:** Kullanıcı false positive/benign activity gördüğünde ürünün davranışını güvenli biçimde düzeltmeli ve benzer durumların tekrarını azaltabilmelidir. “Acknowledge”, “benign”, “known scanner”, “expected activity” gibi feedback kavramları product loop'a dahil edilmelidir. | APPROVED |
| T-01 | Primary threat starting condition nedir? | **RECOMMENDED — B primary; C observable secondary; A current out-of-scope.** | APPROVED |
| T-02 | Malicious insider ürün kapsamına alınmalı mı? | **RECOMMENDED — C.** | APPROVED |
| T-03 | Primary threat behaviors hangileridir? | **RECOMMENDED primary behavior set:** | APPROVED |
| T-04 | Initial Access detection ürünün hedefi mi? | **RECOMMENDED — Hayır, direct primary coverage değil.** | APPROVED |
| T-05 | Credential Access ile Credential Abuse arasındaki sınır | **RECOMMENDED — Ürün sözünde “credential theft detection” yerine “deception credential access/use and credential abuse signals” denmeli.** | APPROVED |
| T-06 | Malware/C2/exfiltration/impact coverage nasıl tanımlanmalı? | **RECOMMENDED — B.** | APPROVED |
| T-07 | Ransomware ürün iddiası | **RECOMMENDED — “Ransomware prevention” iddiası yok; “ransomware ve diğer intrusions öncesindeki internal discovery/lateral movement davranışlarını erken görünür kılabilir” şeklinde kontrollü ilişki.** | APPROVED |
| T-08 | Public internet attackers threat modelde mi? | **RECOMMENDED — Threat modelin primary source'u private/internal addressable environment.** Public exposure ancak müşteri kendi internal decoy'unu yanlışlıkla veya bilinçli olarak expose ederse güvenlik politikası konusu olarak ele alınmalıdır; ürün global internet telemetry network'üne dönüşmemelidir. | APPROVED |
| T-09 | Legitimate scanner/admin activity tehdit midir? | **RECOMMENDED — Hayır; fakat threat modelin ana ambiguity source'udur.** | APPROVED |
| T-10 | MITRE ATT&CK ürün mantığının merkezi mi olmalı? | **RECOMMENDED — B.** | APPROVED |
| T-11 | Identity/cloud threats threat modelde hangi statüde? | **RECOMMENDED — Current threat model iki katmanlı olsun:** | APPROVED |
| T-12 | Threat coverage güven seviyeleri nasıl ifade edilmeli? | **RECOMMENDED:** Threat modelde her davranış için üç unsur tutulmalıdır: | APPROVED |
| NG-01 | Antivirus / EDR olacak mıyız? | **RECOMMENDED — B.** | APPROVED |
| NG-02 | IDS/IPS/NDR olacak mıyız? | **RECOMMENDED — Hayır, full passive network monitoring veya inline blocking ürünü olmayacağız.** | APPROVED |
| NG-03 | Firewall / IPS / NAC olacak mıyız? | **RECOMMENDED — Hayır.** | APPROVED |
| NG-04 | SIEM olacak mıyız? | **RECOMMENDED — Hayır. “Deception incident system” ile “general SIEM” ayrılmalı.** | APPROVED |
| NG-05 | SOAR / fully automated response platformu olacak mıyız? | **RECOMMENDED — Hayır.** | APPROVED |
| NG-06 | Vulnerability scanner / Attack Surface Management olacak mıyız? | **RECOMMENDED — Full scanner NON-GOAL; deception placement için gerekli kadar environment understanding SUPPORTING capability.** | APPROVED |
| NG-07 | Email security / phishing gateway olacak mıyız? | **RECOMMENDED — Hayır.** | APPROVED |
| NG-08 | VPN / ZTNA / secure access platformu olacak mıyız? | **RECOMMENDED — Hayır.** | APPROVED |
| NG-09 | Public honeynet / global threat-intelligence network olacak mıyız? | **RECOMMENDED — Explicit non-goal olarak yazılsın.** | APPROVED |
| NG-10 | Malware sandbox olacak mıyız? | **RECOMMENDED — Hayır.** | APPROVED |
| NG-11 | Offensive security / hack-back yapacak mıyız? | Explicit product principle: | APPROVED |
| NG-12 | Autonomous AI security operator olacak mıyız? | **RECOMMENDED — Hayır.** | APPROVED |
| NG-13 | Compliance/reporting ürünü olacak mıyız? | **RECOMMENDED — Compliance platform NON-GOAL.** | APPROVED |
| NG-14 | Tam SOC replacement olacak mıyız? | **RECOMMENDED — “SOC replacement” iddiası yapılmamalı.** | APPROVED |
| NG-15 | MSSP multi-tenant platformu current product definition'a dahil mi? | Current non-goal listesine açıkça eklenmeli: tenant hierarchy, delegated administration, MSP billing, reseller operations ve multi-customer SOC console bu aşamada yoktur. | APPROVED |
| L-01 | Product loop'un ilk adımı “deploy decoy” mu, “understand environment” mı? | **RECOMMENDED — B, minimal/contextual understanding.** | APPROVED |
| L-02 | Deception placement manual mı, assisted mı, autonomous mu? | **RECOMMENDED — Assisted placement + explicit owner approval.** | APPROVED |
| L-03 | Deception default olarak sparse mı, dense mı olmalı? | **RECOMMENDED — Başlangıç ürün felsefesi “purposeful sparse coverage”; future adaptive density.** | APPROVED |
| L-04 | Event geldiğinde sistem doğrudan alert üretmeli mi? | **RECOMMENDED — Üçüncü model.** | APPROVED |
| L-05 | Correlation'ın product-level görevi nedir? | **RECOMMENDED:** Aynı source/actor/context'e ilişkin parçalı interaction'ları “bir attack story” haline getirmek. | APPROVED |
| L-06 | AI ürün loop'unun hangi noktasında devreye girer? | **RECOMMENDED — Evidence sonrası reasoning; selected assisted use earlier.** | APPROVED |
| L-07 | Response “recommendation”dan sonra ürün ne yapar? | **RECOMMENDED — İkinci model.** | APPROVED |
| L-08 | Benign activity feedback ürün davranışını değiştirmeli mi? | **RECOMMENDED — Evet, ancak kontrollü.** | APPROVED |
| L-09 | AI unavailable olduğunda product degraded mode ne olmalı? | **RECOMMENDED product principle:** | APPROVED |
| L-10 | Product loop “learn” adımında ne öğrenir? | **RECOMMENDED — Hepsi product-level opportunity, fakat autonomous self-modification yok.** | APPROVED |
| I-01 | Event, Signal, Alert ve Incident aynı şey mi? | **RECOMMENDED — Hayır; ayrı kavramlar.** | APPROVED |
| I-02 | Primary UI object Alert mı Incident mı? | **RECOMMENDED — Incident-centric, raw evidence accessible.** | APPROVED |
| I-03 | Attacker Journey ürünün merkezinde mi, opsiyonel visualization mı? | **RECOMMENDED — First-class product object.** | APPROVED |
| I-04 | Incident'in source identity modeli ne olmalı? | **RECOMMENDED — Multi-evidence source identity.** | APPROVED |
| I-05 | Confidence ve Severity ayrı mı olmalı? | **RECOMMENDED — Kesinlikle ayrı.** | APPROVED |
| I-06 | Incident explanation hangi yapıda olmalı? | **RECOMMENDED standart açıklama şablonu:** | APPROVED |
| I-07 | Evidence görünürlüğü ne kadar olmalı? | **RECOMMENDED — Üçüncü model.** | APPROVED |
| I-08 | MITRE ATT&CK UI'da nasıl görünmeli? | **RECOMMENDED — Secondary enrichment.** | APPROVED |
| I-09 | Notification hangi objeden üretilmeli? | **RECOMMENDED — Notification policy incident/significant finding üzerinden çalışmalı.** | APPROVED |
| I-10 | Incident lifecycle hangi statülere sahip olmalı? | **RECOMMENDED product semantics:** | APPROVED |
| I-11 | Recommended action ne kadar prescriptive olmalı? | **RECOMMENDED — Context-aware ordered checklist.** | APPROVED |
| I-12 | Incident kapandıktan sonra ürün ne göstermeli? | **RECOMMENDED — Post-incident record:** | APPROVED |
| D-01 | Deception domains hangi sınıflarda tanımlanmalı? | **RECOMMENDED taxonomy:** | APPROVED |
| D-02 | Interaction levels nasıl tanımlanmalı? | **APPROVED direction'ın formal önerisi:** | APPROVED |
| D-03 | Default product strategy hangi interaction seviyesinde başlamalı? | **RECOMMENDED — Low/medium default; high interaction controlled advanced capability.** | APPROVED |
| D-04 | Kullanıcı “service” mi, “persona” mı deploy etmeli? | **RECOMMENDED — İki seviye.** | APPROVED |
| D-05 | Persona realism neye göre belirlenmeli? | **RECOMMENDED:** Persona, yalnız banner/name değişikliği değil, environment ile tutarlı bir kimlik taşımalıdır: | APPROVED |
| D-06 | Endpoint breadcrumbs ürün vizyonunda hangi statüde? | **RECOMMENDED — Extended Deception / future capability.** | APPROVED |
| D-07 | Identity deception hangi statüde? | **RECOMMENDED — Strategic future capability.** | APPROVED |
| D-08 | Windows deception'ın ürün rolü nedir? | **RECOMMENDED — Windows tek bir runtime requirement olarak değil, deception persona ailesi olarak düşünülmeli:** | APPROVED |
| D-09 | Vulnerable VM'in rolü | Formal ürün kuralı: | APPROVED |
| D-10 | AI infrastructure deception | **RECOMMENDED — Future opportunity; current core değil.** | APPROVED |
| D-11 | Adaptive deception ürün vizyonunda mı? | **RECOMMENDED — Long-term vision: human-governed adaptive deception.** | APPROVED |
| D-12 | Deception'ın güvenlik sınırı | **RECOMMENDED product safety principle:** | APPROVED |
| D-13 | Coverage “çok decoy” ile mi, “doğru touchpoint” ile mi ölçülmeli? | **RECOMMENDED — Meaningful deception coverage.** | APPROVED |
| S-01 | Event → Signal → Evidence → Finding → Incident zinciri kabul edilmeli mi? | **RECOMMENDED — Evet.** | APPROVED |
| S-02 | Confidence ve Severity iki ayrı eksen mi? | **RECOMMENDED — Evet; tek risk score'a indirgenmemeli.** | APPROVED |
| S-03 | Alerting philosophy precision-first mı recall-first mı? | **RECOMMENDED — B default, C advanced visibility olarak.** | APPROVED |
| S-04 | “Zero false positives” iddiası kullanılmalı mı? | **RECOMMENDED — Hayır.** | APPROVED |
| S-05 | Known scanner / trusted automation nasıl ele alınmalı? | **RECOMMENDED — Combination, kullanıcı kontrolünde.** | APPROVED |
| S-06 | Correlation confidence nasıl ele alınmalı? | **RECOMMENDED:** Incident içinde iki farklı güven kavramı gerekebilir: | APPROVED |
| S-07 | Source identification confidence gösterilmeli mi? | **RECOMMENDED — Evet.** | APPROVED |
| S-08 | Deduplication product requirement mı? | **RECOMMENDED — Evet.** | APPROVED |
| S-09 | Risk/severity scoring AI tarafından mı yapılmalı? | **RECOMMENDED — Üçüncü model.** | APPROVED |
| S-10 | Unknown/uncertain state ürün modelinde olmalı mı? | **RECOMMENDED — Evet.** | APPROVED |
| S-11 | Evidence retention product requirement mı? | **RECOMMENDED — Evet, ama retention süresi/teknolojisi değil.** | APPROVED |
| S-12 | Telemetry privacy product principle mı? | **RECOMMENDED — Evet.** | APPROVED |
| S-13 | Sensor/decoy health security signal olarak ayrı mı tutulmalı? | **RECOMMENDED — Evet.** | APPROVED |
| AI-01 | AI ürünün hangi probleminde “necessary differentiator” olabilir? | **RECOMMENDED priority hierarchy:** | APPROVED |
| AI-02 | “AI evidence üretmez” ilkesi nasıl formalize edilmeli? | **RECOMMENDED product rule:** | APPROVED |
| AI-03 | AI incident correlation yapabilir mi? | **RECOMMENDED — Üçüncü model.** | APPROVED |
| AI-04 | AI remediation guidance nasıl güvenli tutulmalı? | **RECOMMENDED — Recommendation must be evidence-grounded, ordered, reversible-aware and uncertainty-aware.** | APPROVED |
| AI-05 | AI provider agnostic product principle olmalı mı? | **RECOMMENDED — Evet, product principle seviyesinde.** | APPROVED |
| AI-06 | AI detection için hard dependency olabilir mi? | **RECOMMENDED — Hayır.** | APPROVED |
| AI-07 | Attacker-facing LLM interaction'ın statüsü | **RECOMMENDED — Advanced R&D capability.** | APPROVED |
| AI-08 | Attacker telemetry LLM'e güvenilir input mu? | **RECOMMENDED — Kesinlikle hayır.** | APPROVED |
| AI-09 | AI'ın environment understanding rolü | **RECOMMENDED — İkinci model.** | APPROVED |
| AI-10 | AI persona/deception generation | **RECOMMENDED — Future supporting capability; generated artefact policy/template validation'dan geçmeli.** | APPROVED |
| AI-11 | Adaptive deception AI tarafından otomatik yürütülebilir mi? | **RECOMMENDED — Current principle: AI proposes, policy/human approves.** | APPROVED |
| AI-12 | Cloud AI telemetry privacy modeli | **RECOMMENDED product policy:** | APPROVED |
| AI-13 | Denial-of-wallet product risk olarak kayıtlı mı? | **RECOMMENDED — Evet, explicit risk.** | APPROVED |
| AI-14 | AI explanation “chat” mi, structured incident mı? | **RECOMMENDED — Structured incident first; optional conversational exploration second.** | APPROVED |
| AI-15 | AI quality nasıl product requirement yapılmalı? | **RECOMMENDED — AI features için eval-driven product governance gerekli.** | APPROVED |
| AI-16 | AI generated severity/confidence kullanıcı tarafından override edilebilir mi? | **RECOMMENDED — Kullanıcı classification/feedback verebilmeli; system score provenance korunmalı.** | APPROVED |
| AI-17 | AI infrastructure deception ürün vizyonunda kalmalı mı? | **RECOMMENDED — Evet, “Future Opportunity” statüsünde.** | APPROVED |
| CU-01 | Capability sınıfları | **RECOMMENDED sınıflandırma:** | APPROVED |
| CU-02 | Capability tree'de “Core” ne anlama gelir? | **RECOMMENDED — Core = ürün kimliğinin uzun vadeli minimum sözleşmesi; MVP zorunluluğu değildir.** | APPROVED |
| CU-03 | Product capability universe Guardpot/competitor parity hedeflemeli mi? | **RECOMMENDED — Hayır.** | APPROVED |
| DP-01 | Ana differentiation tezi nedir? | **RECOMMENDED — D, fakat messaging tek cümlede outcome'a indirgenmeli.** | APPROVED |
| DP-02 | Ürün kategori adı ne olmalı? | **RECOMMENDED:** | APPROVED |
| DP-03 | “Honeypot” kelimesi pazarlamada ne kadar önde olmalı? | **RECOMMENDED — Technical explanation'da görünür, value proposition'da secondary.** | APPROVED |
| DP-04 | EDR/NDR olan müşteriye “neden bize ihtiyaç var?” cevabı | **RECOMMENDED differentiation answer:** | APPROVED |
| DP-05 | OpenCanary'ye karşı differentiation | **RECOMMENDED answer:** | APPROVED |
| DP-06 | Thinkst Canary'ye karşı differentiation | **RECOMMENDED answer:** | APPROVED |
| DP-07 | Guardpot ve enterprise deception'a karşı differentiation | **RECOMMENDED answer:** | APPROVED |
| DP-08 | Moat nerede oluşabilir? | **RECOMMENDED — Tek moat varsayımı yapılmamalı; compound product moat hedeflenmeli.** | APPROVED |
| DP-09 | Product Principles final seti | **RECOMMENDED — Bu 12 ilke Product Principles v1 olarak owner approval'a sunulsun.** | APPROVED |
| DP-10 | Positioning statement seçenekleri | **RECOMMENDED — B ana positioning; A category descriptor olarak destek; C kullanılmasın veya secondary messaging olsun.** | APPROVED |
| DP-11 | Ürün sözü hangi capability seviyesinde olmalı? | **RECOMMENDED formal product strategy:** | APPROVED |
| DP-12 | AI branding ürün isminde/positioning'de merkezi olmalı mı? | **RECOMMENDED — AI capability güçlü biçimde görünür olabilir; fakat category/value promise AI'a bağımlı yazılmamalı.** | APPROVED |
| DP-13 | Product complexity ceiling | **RECOMMENDED product rule:** | APPROVED |
| DP-14 | Initial commercial motion product definition'ı etkilemeli mi? | **RECOMMENDED — İlk ürün mümkün olduğunca low-friction evaluation/deployment'a uygun tasarlansın; enterprise professional-services dependency primary product requirement olmasın.** | APPROVED |

---

# 4. APPROVED Product Principles

1. **High signal over high volume**
2. **Explain before overwhelm**
3. **Evidence before AI inference**
4. **Safe by default**
5. **Progressive sophistication**
6. **Human owns security decisions**
7. **Deception is a means, not the customer's job**
8. **Operational simplicity is a feature**
9. **Honest uncertainty**
10. **Complement, don't pretend to replace everything**
11. **Attacker-facing data is hostile**
12. **Every capability must earn its place**

Ek bağlayıcı ilkeler:

- AI, evidence’ın yerine geçmez.
- AI unavailable olduğunda core detection truth çalışmaya devam etmelidir.
- Response guidance insan onayı olmadan irreversible containment’a dönüşmemelidir.
- Ürün EDR/NDR/SIEM/SOAR/firewall gibi kategorilerin tamamını yeniden inşa etmeye çalışmamalıdır.
- Deception environment attacker pivot/outbound harm açısından safe-by-default olmalıdır.
- Product complexity, primary persona’yı SOC uzmanına dönüştürmemelidir.
- Coverage başarısı decoy sayısıyla değil anlamlı deception touchpoint’leriyle değerlendirilmelidir.

# 5. APPROVED Product Identity

**Product promise:** Internal breach visibility  
**Primary mechanism:** Deception  
**User outcome:** High-confidence incident + attacker journey + guidance  
**Category direction:** Internal Breach Detection & Deception Platform

Outcome-first positioning esastır. “Honeypot” müşterinin satın aldığı sonuç değil, kullanılan temel güvenlik mekanizmasıdır.

# 6. APPROVED Primary Customer & Persona

**Primary customer:** Internal/private network veya hybrid infrastructure işleten; dedicated SOC’u bulunmayan; güvenlik operasyonunun IT manager, sysadmin, DevOps, CTO veya benzeri generalist teknik role ait olduğu security-lean küçük/orta ölçekli organizasyon.

**Primary persona:** Network ve sistem operasyonunu bilen fakat full-time security analyst olmayan teknik sorumlu.

**Buyer model:** Teknik bütçe sahibi ile operator çoğu zaman aynı veya yakın roldedir; enterprise procurement odaklı ürün deneyimi ilk hedef değildir.

**Primary responder:** Generalist technical operator; gerektiğinde external IT/security provider’a escalation.

# 7. APPROVED Primary Threat Model

Ana başlangıç koşulu: attacker veya compromise olmuş internal device/account, private/internal network’e erişebilen bir execution point elde etmiştir.

Primary davranışlar:

- Recon / network probing
- Discovery / enumeration
- Credential abuse
- Lateral movement attempts
- Deception interaction depth

Ürün **initial access platformu**, **ransomware prevention guarantee**, **malware sandbox**, **public honeynet** veya **full kill-chain coverage** ürünü olarak konumlandırılmaz.

# 8. APPROVED Core Product Loop

1. **Deploy**
2. **Understand**
3. **Recommend**
4. **Approve & Place**
5. **Observe**
6. **Detect**
7. **Correlate**
8. **Explain**
9. **Recommend**
10. **Resolve**
11. **Learn**

Autonomous self-modification ve human approval olmadan destructive response bu loop’un parçası değildir.

# 9. Explicit Non-Goals — APPROVED

- Antivirus / full EDR
- General IDS/IPS/NDR
- Firewall / NAC / ZTNA / VPN product
- General-purpose SIEM
- General-purpose SOAR
- Vulnerability scanner / ASM platform
- Email security gateway
- Malware sandbox
- Public/global honeynet as primary product
- Full threat-intelligence vendor
- Compliance management suite
- Autonomous destructive response
- Hack-back / offensive retaliation
- Full SOC replacement claim
- Current multi-tenant MSSP platform

# 10. Decision Closure Status

**Step 2 owner decisions: CLOSED / APPROVED**

Bu belgeye göre Step 2 içerisindeki karar noktalarında bekleyen `OPEN` veya `RECOMMENDED` owner-decision bulunmamaktadır.

Aşağıdakiler kapanmış karar olarak yorumlanmamalıdır:

- doğrulanması gereken market/persona/adoption varsayımları,
- product/security riskleri,
- teknik feasibility soruları,
- architecture/technology seçimleri,
- MVP kapsamı ve acceptance thresholds.

Bunlar sonraki adımlarda ayrı araştırma ve owner-decision kayıtlarıyla ele alınacaktır.

# 11. Document Control

- **Belge:** Step 2 — Product Definition & Strategy — APPROVED Decision Record
- **Format:** Markdown
- **Status:** APPROVED
- **Decision authority:** Product Owner
- **Technology stack decisions included:** No
- **Architecture implementation decisions included:** No
- **MVP selection included:** No
- **Roadmap included:** No