# Adım Bir Araştırma Raporu: Honeypot, Cyber Deception ve AI-Native Ürün Pazarı

## Araştırma çerçevesi ve süreç standardı

Bu rapor, üzerinde anlaştığımız ürün hipotezlerini **değiştirmeden** hazırlanmıştır. Şu aşamada hedefimiz ürünümüz için mimari veya teknoloji seçmek değil; önce pazarın, rakiplerin ve açık kaynak ekosisteminin gerçekte nasıl çalıştığını anlamaktır.

Araştırmanın varsayım tabanı şu şekildedir:

| Konu | Mevcut durum |
|---|---|
| Hedef müşteri hipotezi | **APPROVED** — küçük/orta ölçekli işletmeler öncelikli araştırma ekseni |
| Kullanıcı personası | **APPROVED** — ayrı SOC ekibi bulunmayan veya güvenliğin IT/teknik sorumlunun ek görevi olduğu organizasyonlar ana varsayım |
| Temel ürün ilkeleri | **APPROVED** |
| Deployment felsefesi | **RESEARCHED / OPEN** — seçenekleri karşılaştıracağız, henüz seçim yapmayacağız |
| Aktif deception sınırı | **APPROVED** — kontrollü deception; saldırgana karşı saldırı veya kontrol dışı offensive davranış yok |
| Windows desteği | **APPROVED REQUIREMENT** — fakat uygulanma ve lisanslama modeli henüz açık |
| Gerçek zafiyetli sistemler | **RESEARCHED / OPEN** — emülasyon, container, gerçek VM ve vulnerable VM ayrı sınıflar olarak ele alınacak |
| AI-native kapsamı | **APPROVED** — hem ürünün kendisi hem geliştirme/operasyon süreci |
| Karar yönetimi | **APPROVED PROCESS STANDARD** |

Bundan sonra tüm önemli kararları şu durum makinesiyle takip edeceğiz:

**OPEN → RESEARCHED → RECOMMENDED → OWNER DECISION → APPROVED**

Önemli bir ayrım: Bu raporda benim vardığım sonuçlar **RECOMMENDED** seviyesini geçmez. Bunlardan hiçbiri sizin açık kararınız olmadan **APPROVED** sayılmayacaktır.

Araştırmada Guardpot'un mevcut ürün sitesi ve Mayıs 2026'da güncellenmiş dokümantasyonu, ticari deception üreticilerinin güncel ürün sayfaları ve lisans dokümanları, ayrıca açık kaynak projelerin doğrudan repository'leri incelendi. Guardpot kendisini bugün doğrudan “AI-powered honeypot and cyber deception platform for enterprise cybersecurity” şeklinde konumlandırıyor; yani bizim onayladığımız SMB hipotezi ile kesişmekle birlikte mevcut mesajının merkezi **enterprise** tarafında. citeturn21search0turn27search1

Bu ilk bulgu önemlidir: **Guardpot yerli ve fonksiyonel açıdan en yakın karşılaştırmalardan biri olsa da hedef müşteri ve ürün sadeliği açısından bire bir aynı ürün hipotezine sahip olmak zorunda değiliz.**

## Guardpot'un derinlemesine analizi

### Ürün artık yalnızca bir honeypot yöneticisi değil

Guardpot'un güncel dokümantasyonunun menü yapısı bile ürün stratejisi hakkında oldukça fazla şey söylüyor. Platformda Honeypot Management yanında Attack Surface Management, Secure Link, G-Token, MailPot, Virtual Guarded Network, Load Tester, Intelligence Area, Alarms, Monitoring, Export ve Reports gibi bağımsız modüller bulunuyor. Ayrıca sistem yönetimi, kullanıcı/yetki yönetimi, uyarı/bildirim ve Guardpot Agent Management ayrı yönetim alanları olarak tanımlanmış. citeturn28view0

Dolayısıyla Guardpot'un evrimi kabaca:

**honeypot → deception platform → threat intelligence platform → daha geniş proactive-security/security-operations platform**

şeklinde okunabilir. Bu benim ürün dokümantasyonundan yaptığım çıkarımdır; şirketin resmi bir ürün yol haritası beyanı değildir. citeturn28view0turn28view4

Ana honeypot yönetimi merkezi bir fleet-management yapısına sahip. Guardpot instance'ları durum, sürüm, lokasyon ve IP gibi bilgilerle yönetiliyor; sistemde machine pool ve önceden yapılandırılmış makineler üzerinden deployment kavramı bulunuyor. citeturn29view0turn29view1

Bu, ilk fikrinizde tarif ettiğiniz:

> “Bir panelden tak-çalıştır şekilde farklı sahte sistemler ayağa kaldırma”

yaklaşımının piyasada doğrulanmış bir ürün modeli olduğunu gösteriyor.

### Agent ve VM yaklaşımı

Guardpot'un deployment tarafında iki model bulunuyor. “Remote Install” yönteminde yönetici platform hedef sisteme SSH ile bağlanıyor, root/sudo yetkisi kullanarak honeypot agent'ını indirip kuruyor ve yönetim paneline kaydediyor. İkinci modelde agent hedef makineye manuel kuruluyor ve bir serial key ile merkezi sisteme bağlanıyor. Windows Server/Desktop, BSD ve önemli Linux dağıtımları için kurulum rehberleri sunuluyor. citeturn29view0

Bu son derece önemli bir mimari ipucu:

**Guardpot yalnızca merkezi bir VM içinde tüm sahte servisleri çalıştırmıyor; merkezi control-plane + dağıtık agent/host modelini de kullanıyor.**

Ayrıca “G-Host” adı verilen katmanla QEMU/KVM virtualization host'ları yönetiliyor. Yönetici bir KVM/QEMU sunucusunu tanımlayabiliyor, maksimum VM sayısını belirleyebiliyor ve saldırgan oturumundan sonra VM disk snapshot'ı otomatik alınabiliyor. Snapshot forensic inceleme için saklanıp indirilebiliyor. citeturn29view1

Bu da sizin düşündüğünüz “gerçek VM içinde gerçek veya gerçekçi sistem” yaklaşımının piyasada teknik karşılığı olduğunu gösteriyor.

Daha sonra ayrıca incelememiz gereken önemli mimari spektrum şimdiden ortaya çıkıyor:

**emulated service → emulated operating environment → containerised application → dedicated VM → real vulnerable VM**

Bunlar aynı şey değil ve security/isolation/resource-cost düzeyleri dramatik biçimde değişiyor.

### Deception token'ları

Guardpot'un G-Token sistemi yalnızca network honeypot fikrinden daha geniş bir deception katmanı oluşturuyor. Sistem Web Bug, QR, Microsoft Word/Excel dosyaları, custom executable/binary, CSS, JavaScript, Kubernetes profile, OpenVPN/WireGuard config, Active Directory user/group, RDP profile, SSH profile, FTP credential ve browser cookie gibi çeşitli deception artefact'ları oluşturabiliyor. Ayrıca bunların Guardpot'lar ve servislerle ilişkileri graph üzerinde gösterilebiliyor ve Active Directory üzerinden toplu dağıtım yapılabiliyor. citeturn29view3

Burada doğrudan karşılaştırabileceğimiz Thinkst Canary de benzer stratejiyi kullanıyor. Canarytokens ücretsiz ve limitsiz dijital tripwire'lar olarak sunuluyor; DNS, document token, API key, VPN profile, Windows folder, cloud credentials, QR code ve çeşitli mail/web artefact'ları destekleniyor. citeturn23search1

Bu ürün kategorisinde önemli bir pazar doğrulaması var:

**Deception artık sadece “ağda sahte bir SSH sunucusu koymak” anlamına gelmiyor. Sahte credential, document, config, API key ve identity artefact'ları ürünün doğal bir parçasına dönüşmüş durumda.**

Bu noktanın bizim ürünümüz için kapsam kararı henüz **OPEN** kalmalıdır.

### Threat intelligence ve forensic yaklaşım

Guardpot event modelinde en azından connection, interaction ve credential olayları ayrılıyor. Olay üzerinden saldırgan IP'sine gidildiğinde geolocation, first/last seen, provider, security/reliability scores, saldırı geçmişi, protocol activity, MITRE ATT&CK eşlemesi ve forensic session bilgileri gösteriliyor. citeturn28view4

Bu sizin ilk tarifinizdeki:

> “Alarm üretmek nihai amaç değil; saldırgan hakkında toplayabildiğimiz kadar bilgi toplamak”

fikrinin doğrudan ticari ürün karşılığını oluşturuyor.

Guardpot ayrıca QEMU/KVM VM'lerinde saldırgan oturumundan sonra disk snapshot'ı alarak yalnızca telemetry değil, stateful forensic artefact bırakabiliyor. citeturn29view1

Burada ileride önemli bir ürün kararı vereceğiz:

**Biz bir “intrusion tripwire” mı olacağız, yoksa “attacker observation/forensics platform”una doğru mu ilerleyeceğiz?**

İkincisi, birincinin doğal devamı olabilir ancak operasyonel ve teknik maliyeti çok daha yüksek.

### Attack Surface, MailPot ve VPN genişlemesi

Guardpot ayrıca internet-facing altyapı, service/port ve olası zafiyetleri tarayan bir Attack Surface Management katmanı sunuyor. citeturn21search3

MailPot ise bağlı mailbox'ları IMAP/POP3 üzerinden takip ediyor ve e-postaları Normal, Suspicious, Phishing ve Spam sınıflarına ayırıyor; e-posta detayında AI-generated threat score gösteriyor ve policy tabanlı bildirimler üretilebiliyor. citeturn28view3

Daha da ilginci, VGN modülü WireGuard tabanlı encrypted virtual network oluşturuyor. Site-to-site/location bağlantıları, NAT/routing, split/all-tunnel, CIDR, DNS, DHCP, LDAP/local group authorization ve MFA gibi özelliklere sahip; Windows, macOS, Linux, iOS ve Android client'ları tanımlanıyor. citeturn29view2

Bu bana göre Guardpot araştırmasından çıkan en önemli ürün-stratejisi derslerinden biridir:

> **Rakibin yaptığı her şeyi yapmak zorunda değiliz.**

Guardpot deception çekirdeğinin yanına ASM, email security, VPN/access, load testing ve diğer modülleri ekleyerek geniş bir security platformuna doğru ilerliyor. Bu genişlik Guardpot için ticari açıdan doğru olabilir. Fakat bizim onayladığımız “hafif, SOC'siz küçük işletme” hipotezinde aynı yaklaşım ürün odağını kaybettirebilir.

Bu sonuç şu anda **RECOMMENDED**, henüz karar değildir.

### Guardpot'un AI yaklaşımı

Guardpot artık sadece pazarlama metninde “AI-powered” demiyor. Attacker Flow Mapping ürün materyalinde saldırı akışlarının **on-prem fine-tuned LLM** ile özetlendiği ve verinin dışarı çıkarılmadığı belirtiliyor. citeturn21search1

MailPot'ta da AI classification ve AI-generated threat score ürün akışının doğrudan parçası. citeturn28view3

Dolayısıyla Guardpot'un AI kullanımını en az iki kategoriye ayırabiliriz:

**analyst augmentation:** saldırıların anlaşılması/özetlenmesi,

**classification:** e-posta veya event gibi verilerin risk açısından değerlendirilmesi.

Bu, bizim “AI-native sadece development sürecinde değil ürünün içinde de olmalı” kararımız açısından önemli bir benchmark.

### Gelir, fiyatlandırma ve satış modeli

Guardpot doğrudan B2B olarak tanımlanıyor. Fonangels yatırım dokümanı ürünün büyüme stratejisinde farklı ülkelerde distribütör saha satışlarını ve hardware cihaz serisini açıkça sayıyor. citeturn27search0

Guardpot'un şirket profili fiziksel ve sanal deployment, SaaS, IaaS ve on-prem seçeneklerinden; düşük kaynak tüketimli edge cihazlardan ve binlerce honeypot'un merkezi yönetiminden söz ediyor. citeturn27search3

İncelediğim kaynaklarda standart bir “$X/user/month” self-service fiyat listesi yerine teklif ve kanal bazlı satış yapısı ön plana çıkıyor. Guardpot Support Panel'de quotation, opportunity, order, distributor/partner/customer rol ayrımları, ödeme vadeleri ve indirim oranları bulunuyor; Türkiye'deki ürün listelemelerinde de “Teklif Al” modeli kullanılıyor. Bu nedenle mevcut ticari modelin **kurumsal teklif + partner/distribütör kanalına dayalı B2B satış** ağırlıklı olduğu çıkarımını yapmak makul. citeturn10search0turn10search9

Fonangels kampanya açıklamasına göre şirket cirosu 2024'te 400 bin TL, 2025'te 6,7 milyon TL olarak beyan edilmiş ve şirket ilk yıldan beri kârlı olduğunu ifade etmiş. Bunlar yatırım kampanyasındaki şirket beyanlarıdır; bağımsız denetlenmiş sonuçlar olarak yorumlanmamalıdır. citeturn27search0

22 Aralık 2025–19 Şubat 2026 arasındaki paya dayalı kitle fonlama kampanyasında %9 pay karşılığında 10,8 milyon TL hedef belirlenmiş; kampanya %120 seviyesine ulaşarak 12,96 milyon TL toplam yatırım ve 442 yatırımcıyla kapanmış. citeturn27search2

Bu bilgiler bize rakibin sadece teknik ürünü hakkında değil, go-to-market yaklaşımı hakkında da sinyal veriyor: **enterprise/B2B, partner/distributor, on-prem/hardware opsiyonu ve doğrudan teklif satışı.**

## Ticari rakipler ve pazarın nasıl bölündüğü

Pazar araştırmasında tek tip “honeypot ürünü” olmadığını görüyoruz. Çok farklı ürün felsefeleri aynı kategori altında bulunuyor.

| Ürün | Temel konumlandırma | Ürün felsefesi | Deployment / ticari sinyal |
|---|---|---|---|
| **Thinkst Canary** | High-signal breach detection | Çok basit decoy + token | Fiziksel/virtual Canary; açık fiyatlandırma |
| **Guardpot** | Honeypot + deception + TI + geniş security platformu | Merkezi yönetim, agent, VM, token | SaaS/IaaS/on-prem/hardware, teklif/kanal |
| **FortiDeceptor** | Enterprise network deception | Çok sayıda decoy, segment/VLAN coverage | Appliance, VM, DaaS, subscription |
| **Acalvio ShadowPlex** | Enterprise-wide adaptive deception | AI-driven, agentless, identity/cloud/OT | Enterprise deployment |
| **Zscaler Deception** | Zero Trust ekosistemine gömülü deception | Network + identity + cloud + GenAI deception | Zscaler platform entegrasyonu |
| **SentinelOne Hologram** | EDR/XDR/Identity çevresinde deception | Decoy + identity defence | SentinelOne security platform ekosistemi |
| **CounterCraft** | Deception-powered threat intelligence | Attacker engagement ve adversary intelligence | Enterprise/government/MSSP |

### Thinkst Canary: sadeliğin benchmark'ı

Thinkst bizim için özellikle önemli, çünkü ürün felsefesi “çok özellik” değil **çok net sinyal** üretmek üzerine kurulmuş.

Canary'nin güncel resmi fiyatlandırmasında beş Thinkst Canary için yıllık 7.500 USD başlangıç noktası gösteriliyor. Aynı şirketin Canarytokens ürünü ise ücretsiz ve limitsiz. citeturn23search0turn23search1

Thinkst'in değer önerisinin özü şudur:

Bir üretim sistemi olmaması gereken şeye birisi dokunuyorsa, bunun meşru bir nedeni çok azdır.

Bu yaklaşım özellikle bizim personası açısından kritik: ayrı bir SOC'u olmayan kullanıcıya yüz bin network event'i değil, **“buna bakmalısın” denilecek birkaç yüksek güvenli sinyal** vermek.

Ayrıca ilginç bir operasyonel ayrıntı var. Thinkst dokümantasyonu Microsoft Defender'ın discovery scan'lerinin Canary'leri tetikleyebileceğini ve bunun için Canary IP'lerinin Defender keşif mekanizmalarından hariç tutulmasını öneriyor. citeturn23search4

Bu küçük detay ileride bizim için büyük bir ürün problemi olacaktır:

> **Meşru network scanners ile gerçek recon nasıl ayrılacak?**

Örneğin vulnerability scanner, EDR, asset discovery, IT monitoring ve kullanıcı tarafından yapılan Nmap testleri false positive kaynağı olabilir.

Bu konu şimdiden **OPEN technical requirement** olarak kaydedilmelidir.

### FortiDeceptor: enterprise deception'ın ölçeği

Fortinet'in Şubat 2026 tarihli güncel ordering guide'ı FortiDeceptor'un appliance veya VM subscription modeliyle çalıştığını gösteriyor. Tek appliance modeline bağlı olarak 20 deception VM'e ve toplam 480 decoy IP'ye kadar çıkabiliyor; Linux, SCADA, IoT, custom ve cloud decoy'ları bulunuyor. citeturn24view0turn25view0

Fortinet burada çok önemli bir mimari tekniği kullanıyor: tek tek 480 tam VM çalıştırmak yerine bir deception VM'in 24 IP/decoy sunabilmesiyle sanallaştırma maliyeti azaltılıyor. citeturn24view0turn25view0

Bu, ileride “her fake IP = bir VM” varsayımına kesinlikle kapılmamamız gerektiğini gösteriyor.

FortiDeceptor'un ticari lisansı network segment/VLAN coverage üzerinden ölçekleniyor. `/24` network tek VLAN lisansı olarak değerlendiriliyor; ürünler 2–128 segment aralığında ölçeklenebiliyor ve çoklu appliance yönetimi ayrıca central-management lisansına tabi olabiliyor. citeturn24view0turn25view2

Windows konusu ise daha da önemli. Built-in Windows decoy'ları ek Windows lisansı gerektiriyor; custom decoy özelliğinde müşteri kendi kurumsal Windows lisansını kullanabiliyor. DaaS modelinde Windows lisansı subscription'ın içine dahil edilmiş. citeturn25view0turn25view2

Bu araştırma, daha önce onayladığınız “Windows mutlaka olmalı” gereksiniminin doğru olduğunu fakat **Windows desteğinin yalnızca teknik bir geliştirme problemi olmadığını** doğruluyor. Licensing ve image-distribution stratejisi başlı başına product architecture konusu.

Bunu Adım Dört'e kadar çözmeden bırakabiliriz; ancak artık durum:

**Windows capability: APPROVED**

**Windows delivery/licensing model: OPEN**

### Acalvio: AI ile adaptive deception

Acalvio ShadowPlex kendisini agentless, enterprise ölçekli bir deception katmanı olarak konumlandırıyor ve IT, cloud, OT ve identity alanlarını kapsıyor. Şirket, AI kullanarak deception coverage'ını saldırı sırasında dinamik şekilde değiştirebildiğini ve reconnaissance, credential harvesting ve lateral movement gibi davranışları ortaya çıkardığını ifade ediyor. citeturn26search0turn26search8

Buradaki AI modeli Guardpot'un “incident summary” kullanımından farklı:

**AI sadece sonuçları açıklamıyor; deception sisteminin nasıl konumlandırılacağını ve adapte olacağını da yönetiyor.**

Bu bizim AI-native araştırmamız açısından çok daha ilginç bir referans.

### Zscaler: deception artık LLM'leri de kandırıyor

Zscaler ürününde deception doğrudan Zero Trust ve enterprise platformuna entegre edilmiş durumda. Daha önemlisi, 2026 itibarıyla ürün **GenAI deception** sunuyor.

Zscaler interactive decoy'ları gerçek chatbot, LLM server, open-source AI application ve API altyapısını taklit edebiliyor. Bu decoy'lar prompt injection, data poisoning, jailbreaking ve training-data extraction gibi AI sistemlerine yönelik saldırıları tespit etmek üzere kullanılıyor. citeturn26search5turn26search13

Daha da ileri giderek MCP Server decoy desteği bulunuyor; sahte MCP server'ın LLM/chatbot ile sahte application/tool'ları ilişkilendirebildiği dokümante edilmiş. citeturn26search21

Bu bulgu özellikle önemli:

**AI-native deception yalnızca saldırganla konuşmak için LLM kullanmak değildir. AI altyapısının kendisini bir deception surface haline getirmek de yeni bir ürün kategorisi olmaya başlamış durumda.**

Bu fikir, bizim başlangıç MVP'miz için muhtemelen fazla kapsamlıdır; fakat ürün vizyonu açısından **OPEN opportunity** olarak tutulmalıdır.

### SentinelOne: deception + identity

SentinelOne Singularity Identity tarafında deception; stolen credentials, reconnaissance, identity misuse, conditional access ve MFA gibi identity defence yetenekleriyle birlikte konumlandırılıyor. citeturn26search10

Hologram network-based decoy'ları saldırgan ve insider'ların sahte sistemlerle etkileşime girmesini sağlıyor; bu telemetry daha sonra investigation ve adversary intelligence için kullanılabiliyor. citeturn26search6

Bu da pazarın önemli yönlerinden birini doğruluyor:

**Credential deception ve identity deception, network honeypot'lardan ayrı bir niş değil; modern deception platformlarının doğal genişleme alanı.**

### CounterCraft: alarmdan ziyade saldırgan istihbaratı

CounterCraft kendisini “deception-powered threat intelligence” olarak konumlandırıyor. Kontrollü deception environment'larının gerçek organizasyon ortamını taklit ederek saldırgan TTP ve davranışlarını toplamasına odaklanıyor. citeturn26search3turn26search15

Hedef müşteri olarak büyük işletmeler, kamu ve ulusal güvenlik organizasyonlarını açık şekilde vurguluyor. Güncel platform mesajlarında MSSP ve multi-environment deployment da yer alıyor. citeturn26search19turn26search27

CounterCraft bu nedenle sizin başlangıç fikrinizdeki:

> “Saldırganı mümkün olduğunca uzun süre fake ortamda tutup hakkında bilgi toplamak”

vizyonunun ticari uç noktasına yakın bir örnektir.

Fakat aynı zamanda ürün kapsamının nasıl kontrolden çıkabileceğinin de göstergesidir. “Someone is scanning my 15-person office network” problemi ile “nation-state adversary intelligence” problemi teknik olarak akraba olsa da tamamen farklı ürünlerdir.

## Açık kaynak honeypot ekosistemi

Burada benim için en önemli sonuç şu oldu:

**Bizim bütün honeypot protokollerini sıfırdan yazmamız teknik olarak gerekli görünmüyor.**

Olgun açık kaynak projeler hem teknik referans hem de potansiyel component olarak değerlendirilebilir. Ancak “kullanabiliriz” ile “ürüne embed etmeliyiz” aynı karar değildir. Lisans, bakım, security boundary ve lifecycle konuları daha sonra ayrıca değerlendirilmelidir.

### OpenCanary

OpenCanary araştırmadaki en önemli projelerden biri.

Thinkst tarafından geliştirilen açık kaynak OpenCanary, özellikle **non-public network'e girilmiş olduğunu tespit etmek** amacıyla tasarlanmış multi-protocol network honeypot. Python ile yazılmış; Raspberry Pi veya düşük kaynaklı VM'lerde çalışabilecek kadar az resource istediği açıkça belirtiliyor. Linux en geniş feature set'ini sağlıyor. citeturn31view2

Bu tanım neredeyse bizim ilk problem statement'ımızın açık kaynak karşılığı:

> “Ofis network'ünün içinde biri tarama yapıyor mu, dolaşıyor mu?”

OpenCanary'nin BSD-3-Clause lisanslı olması da teknik değerlendirmede not edilmesi gereken bir unsur. citeturn31view2

Projenin Temmuz 2026'da v0.9.9 sürümünü yayımlamış olması da aktif bakım açısından olumlu bir sinyal. citeturn19search1

Benim mevcut değerlendirmem:

**OpenCanary = bizim ürün hipotezimizin lightweight/low-interaction katmanı için en önemli referans projelerden biri.**

Bu henüz “dependency olarak kullanacağız” kararı değildir.

### Cowrie

Cowrie SSH/Telnet tarafında çok daha ileri interaction sağlar.

Default medium-interaction modunda Python içerisinde fake UNIX filesystem ve shell oluşturuyor; saldırgan komut çalıştırabiliyor, dosyaları okuyabiliyor ve sisteme dosya upload/download edebiliyor. High-interaction modunda ise SSH/Telnet proxy olarak gerçek backend'e bağlanabiliyor veya Cowrie kendi QEMU server pool'unu yönetebiliyor. JSON log ve session replay desteği de bulunuyor. citeturn31view3

Bu, bizim düşündüğümüz sistem açısından kritik bir kanıttır:

**“Basit SSH servisi” ile “saldırganın gerçekten içinde dolaşabildiği VM” arasında tek ürün içinde kademeli interaction seviyeleri teknik olarak mümkündür.**

Cowrie'nin daha da ilginç özelliği experimental LLM mode. OpenAI GPT gibi modeller kullanarak predefined command-response listelerine bağlı kalmaksızın saldırgan komutlarına dinamik cevap üretebiliyor ve session context'i koruyabiliyor. citeturn31view3

Dolayısıyla:

```text
Attacker
   │
   ▼
SSH
   │
   ├── Static fake shell
   │
   ├── LLM-generated shell
   │
   └── Proxy
         │
         ▼
      QEMU VM
```

gibi bir interaction escalation modeli yalnızca teorik değil, halihazırda çalışan açık kaynak yazılımlarda uygulanmış durumda. citeturn31view3

Cowrie'nin Ağustos 2026'da v3.0.12 sürümünün yayımlanmış olması da projenin aktif biçimde geliştirildiğini gösteriyor. citeturn19search2

### T-Pot

T-Pot farklı bir kategori.

T-Pot tek honeypot değil; **all-in-one multi-honeypot platform**. 20'den fazla honeypot'u ve Elastic Stack tabanlı visualization/analytics bileşenlerini bir araya getiriyor; standalone “Hive” veya Hive + distributed sensors topolojisiyle çalışabiliyor. citeturn31view0

Tek sistem içinde honeypot catalogue, Docker-based orchestration, log collection, Elastic analytics, attack maps, Suricata ve çok sayıda security tool'u birleştirmesi nedeniyle bizim için çok değerli bir **reference architecture**. citeturn31view0

Ancak T-Pot'un lightweight olmadığı açık. Güncel dokümanda Hive için 16 GB RAM/256 GB SSD, sensor için 8 GB RAM/128 GB SSD öneriliyor; installation/operation için outbound internet bağlantısı da bekleniyor. citeturn31view0

Dolayısıyla ilk onayladığımız SMB/lightweight hedef açısından:

**T-Pot'un tamamını ürünümüz olarak paketlemek şu anda güçlü bir aday görünmüyor.**

Ama şu konularda çok önemli bir araştırma kaynağı:

- honeypot plugin catalogue,
- container orchestration,
- common logging,
- distributed sensor architecture,
- visualization,
- lifecycle/update yönetimi.

T-Pot ayrıca LLM-based honeypot'ları deployment seçenekleri arasında açıkça gösteriyor. citeturn31view0

Ve çok önemli bir güvenlik uyarısı yapıyor: honeypot compromise ihtimalinin hiçbir zaman tamamen ortadan kaldırılamayacağını ve honeypot üzerinde sensitive data bulundurulmaması gerektiğini özellikle belirtiyor. citeturn31view0

Bu, bizim daha önce belirlediğimiz active-deception/isolation sınırımızı teknik olarak destekleyen önemli bir gerçek.

### Galah

Galah, doğrudan AI-native honeypot açısından araştırmadaki en ilginç açık kaynak projelerden biri.

Go ile yazılmış bir LLM-powered web honeypot. Sabit WordPress/Joomla/router response template'leri oluşturmak yerine gelen arbitrary HTTP request'e göre LLM ile gerçekçi HTTP header ve body oluşturuyor. OpenAI, Google AI, Vertex AI, Anthropic, Cohere ve local Ollama backend'lerini destekliyor. citeturn30view4

Aynı request için sürekli model çağırmamak amacıyla response cache kullanıyor ve opsiyonel olarak HTTP request'leri Suricata rules ile değerlendirebiliyor. citeturn30view4

Projenin geliştiricisi aynı zamanda iki çok gerçek ürün problemine dikkat çekiyor:

**fingerprinting:** LLM response latency veya tutarsızlıklar honeypot olduğunu belli edebilir,

**denial-of-wallet:** saldırgan kasıtlı olarak çok sayıda istek üreterek LLM API maliyeti yaratabilir. citeturn30view4

Bu iki konu bizim AI-native deception tasarımımızda şimdiden **OPEN risk** olarak kaydedilmelidir.

Galah Apache-2.0 lisansıyla yayımlanıyor. citeturn30view4

### Dionaea

Dionaea daha çok exploit/malware yakalama perspektifinde değerlidir. IPv6/TLS desteği ve shellcode tespiti yanında FTP, HTTP, MQTT, MSSQL, MySQL, SMB, SIP, TFTP ve UPnP gibi çok sayıda protokolü emulate edebiliyor; JSON, SQLite ve hpfeeds logging sunuyor. citeturn31view4

Dionaea GPLv2+ lisanslı. citeturn31view4

Bu bizim için özellikle “her protocol handler'ını sıfırdan yazmalı mıyız?” sorusuna iyi bir örnek oluşturuyor.

### Heralding

Heralding çok daha basit ve ilginç bir yaklaşım kullanıyor: amacı doğrudan credential yakalamak. FTP, Telnet, SSH, HTTP(S), POP3(S), IMAP(S), SMTP, VNC, PostgreSQL ve SOCKS5 gibi protocol'lerde login attempt'lerini kaydediyor. citeturn20search1

Bu özellikle sizin ilk mesajınızdaki:

> “Bir VM içinde PostgreSQL çalışıyor gibi görünmesi ve weak/no-password login denenmesi”

senaryosu için ilginç bir referans.

Ancak repository'nin formal release yayımlamamış olması ve geçmişte Python compatibility sorunları yaşamış olması nedeniyle dependency adayı olarak OpenCanary/Cowrie kadar güçlü görünmüyor. Bu, repository maintenance sinyallerinden çıkardığım mevcut değerlendirmedir. citeturn20search7turn20search11

### Conpot

Conpot ICS/SCADA sistemlerini emulate ederek industrial control systems hedefleyen saldırganlar hakkında intelligence toplamaya yönelik açık kaynak honeypot. GPL-2.0 lisanslı ve template bazlı çalışma yaklaşımı kullanıyor. citeturn20search0

Bizim ilk hedef persona için ICS şu anda gereksiz scope olabilir. Buna rağmen mimari açıdan önemli olan fikir:

**“Honeypot” yerine “persona/template” tanımlamak.**

Yani kullanıcı teknik servisleri tek tek kurmak yerine ileride:

> “Small Office NAS”

> “Windows File Server”

> “Old Dev Server”

> “Internal PostgreSQL”

> “WordPress Intranet”

> “Legacy Windows Machine”

gibi bir **deception persona catalogue** seçebilir.

Bu fikir Conpot, T-Pot ve ticari ürünlerin template yaklaşımından yaptığım bir ürün çıkarımıdır. Henüz karar değildir. citeturn20search0turn29view0turn31view0

### Açık kaynak ekosisteminin genel resmi

Araştırma sonunda açık kaynak tarafını şöyle sınıflandırabiliriz:

| Proje | Interaction / amaç | Kaynak ihtiyacı | Bizim açımızdan anlamı |
|---|---|---:|---|
| **OpenCanary** | Low-interaction/internal tripwire | Çok düşük | **Çok yüksek önem** |
| **Cowrie** | Medium/high SSH-Telnet | Düşük → VM'e göre yükselir | **Çok yüksek önem** |
| **T-Pot** | Multi-honeypot platform | Yüksek | **Reference architecture** |
| **Galah** | LLM web honeypot | Model kullanımına bağlı | **AI-native R&D referansı** |
| **Dionaea** | Exploit/malware/protocol traps | Orta | Protocol component referansı |
| **Heralding** | Credential capture | Düşük | Basit service persona referansı |
| **Conpot** | ICS/SCADA | Değişken | Template/persona yaklaşımı |

Buradan şu sonucu çıkarıyorum:

> **Bizim fikrimizin asıl teknolojik değeri “bir SSH honeypot yazmak” olmayacak.**

Açık kaynakta bunu zaten yapan çok fazla proje var.

Potansiyel ürün değeri daha çok şu katmanda oluşacak:

```text
                         PRODUCT LAYER
 ┌────────────────────────────────────────────────────────┐
 │ Deployment UX                                          │
 │ Persona / template catalogue                           │
 │ Network integration                                    │
 │ Isolation & lifecycle                                  │
 │ Event normalization & correlation                      │
 │ High-confidence alerting                               │
 │ Forensics                                              │
 │ AI investigation / explanation                         │
 │ Adaptive deception                                     │
 │ Multi-decoy orchestration                              │
 └────────────────────────────────────────────────────────┘
            │               │               │
            ▼               ▼               ▼
       OpenCanary        Cowrie          Our runtime
            │               │               │
            ├───────────────┼───────────────┤
            ▼               ▼               ▼
       Containers       Emulators          VMs
```

Bu şema bir **RECOMMENDED research direction**'dır; mimari karar değildir.

Ayrıca open-source lisansları ileride ayrı workstream olarak incelenmelidir. Örneğin OpenCanary BSD-3-Clause iken T-Pot GPL-3.0, Dionaea GPLv2+, Conpot GPL-2.0, Heralding GPL-3.0 ve Galah Apache-2.0'dır. citeturn31view2turn31view0turn31view4turn20search0turn20search1turn30view4

Bunlardan hangilerinin commercial distribution içinde nasıl kullanılabileceğini henüz karara bağlamıyorum; o konu ürünleştirme aşamasında lisans bazında ayrı değerlendirilmelidir.

## AI-native deception açısından pazarın geldiği nokta

Bu araştırmanın benim için en şaşırtıcı sonucu, AI-native deception'ın artık teorik bir fikir olmaması.

Piyasada en az dört farklı AI kullanım modeli oluşmuş durumda.

### Saldırganla dinamik etkileşim

Cowrie LLM mode, predefined shell response yerine context tutan dinamik shell cevapları üretebiliyor. citeturn31view3

Galah arbitrary HTTP request'lere LLM ile dinamik fake web-system cevapları oluşturabiliyor. citeturn30view4

Buradaki hedef:

> saldırgana daha uzun süre “gerçek bir sistemdeyim” hissi vermek.

Bu tam olarak sizin “oyalamak ve hakkında bilgi toplamaya devam etmek” vizyonuyla örtüşüyor.

Ancak LLM'yi doğrudan attacker-facing loop içine koyduğumuz anda yeni problemler doğuyor:

- response latency,
- hallucination nedeniyle persona tutarsızlığı,
- model fingerprinting,
- prompt injection,
- runaway interaction,
- model/API maliyeti,
- denial-of-wallet,
- data residency,
- saldırgan tarafından modelin istenmeyen amaçlarla kullanılması.

Galah projesinin kendisi response latency/fingerprinting ve denial-of-wallet risklerine özellikle dikkat çekiyor. citeturn30view4

Dolayısıyla “LLM koyarız, honeypot daha akıllı olur” yaklaşımı yeterli değil.

### Güvenlik analistini AI ile güçlendirme

Guardpot attacker flow için on-prem LLM summary sunuyor; MailPot da AI-assisted classification ve threat scoring yapıyor. citeturn21search1turn28view3

Bizim personası açısından bunun değeri enterprise SOC'tan bile daha büyük olabilir.

Çünkü hedef kullanıcı muhtemelen:

> “T1059.004 Unix Shell ve T1021.004 SSH activity gözlendi”

mesajıyla ne yapacağını bilmeyebilir.

Ama:

> “Bu cihaz ofis ağınızda önce 18 IP'yi taradı, ardından fake PostgreSQL sunucusuna `postgres/admin123` ile giriş yapmayı denedi ve sonrasında SSH üzerinden root erişimi aradı. Bu normal kullanıcı davranışına benzemiyor. Öncelikle 192.168.1.37 cihazını network'ten izole edin.”

şeklindeki açıklama doğrudan aksiyon üretir.

Burada AI'nin görevi **detection üretmek zorunda değildir**.

Deterministic telemetry gerçeğin kaynağı olabilir:

```text
Raw telemetry
      │
      ▼
Deterministic detection
      │
      ▼
Event correlation
      │
      ▼
Evidence package
      │
      ▼
LLM reasoning / explanation
      │
      ▼
Human-readable incident
      │
      ▼
Recommended response
```

Bana göre bizim personası için bu, attacker-facing generative honeypot'tan dahi daha yüksek ilk ürün değeri taşıyabilir.

Bu bir **RECOMMENDED** görüş, karar değil.

### Adaptive deception orchestration

Acalvio AI'ı yalnızca olay açıklamak için değil, deception coverage'ını saldırıya göre adapte etmek için kullanıyor. citeturn26search0turn26search16

Uzun vadede bunun bizim ürün karşılığı şöyle olabilir:

```text
Normal state

Workstation ───── PostgreSQL decoy
     │
     └─────────── SSH decoy


Recon detected from Workstation X

                     ┌─ Fake Git server
                     ├─ Fake Jenkins
Workstation X ───────┼─ Fake PostgreSQL
                     ├─ Fake SSH host
                     └─ Fake SMB share
```

Burada AI “saldırgana saldırmıyor”; kontrollü deception yüzeyini olayın bağlamına göre genişletiyor.

Bu, aktif deception sınırımızla prensipte uyumlu olabilir ancak gerçek network routing ve isolation mekanizmaları Adım Üç'te kanıtlanmadan uygulanabilir varsayılmayacaktır.

### AI sistemlerinin kendisini decoy yapmak

Zscaler'ın GenAI decoy ve MCP server deception ürünleri yeni bir yön gösteriyor: fake LLM server, fake API, fake chatbot, fake vector/RAG artefact veya MCP tool saldırgan için bir lure olabiliyor. citeturn26search1turn26search21

Bu, bizim başlangıç hedefimizden ileri bir use-case fakat özellikle yazılım şirketleri hedeflenirse gelecekte çok anlamlı hale gelebilir.

Örneğin bir yazılım şirketindeki fake:

```text
internal-ai.company.local
llm-gateway.company.local
mcp-jira.internal
mcp-github.internal
vector-db.internal
```

servisleri gerçek çalışanlar tarafından hiç kullanılmayıp sadece deception amacı taşıyabilir.

Bir saldırgan bunları enumerate ettiğinde yüksek kaliteli sinyal elde edilir.

Bu **OPEN future product opportunity** olarak kaydedilmelidir.

## Kendi ürün hipotezimiz açısından çıkarımlar

Araştırma başlangıç fikrinizi zayıflatmadı; tersine temel teknik ve ticari varsayımların büyük bölümünü doğruladı. Fakat “biz de Guardpot yapalım” yaklaşımından uzak durmamız gerektiğini düşünüyorum.

### En önemli doğrulama: problem gerçek ve ürünleşmiş

OpenCanary doğrudan private network breach detection amacıyla tasarlanmış. Thinkst Canary aynı fikri ticari ve son derece basit bir ürün haline getirmiş. Guardpot, Fortinet, Acalvio, SentinelOne ve Zscaler ise bunun daha geniş enterprise deception karşılıklarını oluşturuyor. citeturn31view2turn23search0turn21search0turn23search5turn26search0turn26search10turn26search13

Dolayısıyla şu problem statement geçerli:

> “Normalde hiç kimsenin dokunmaması gereken sahte sistemlere biri dokunuyorsa, bu çok yüksek değerli bir security signal olabilir.”

### En önemli uyarı: “lightweight honeypot” tek başına farklılaştırıcı değil

OpenCanary zaten lightweight.

Thinkst Canary zaten kolay.

Guardpot edge/minimal-resource deployment iddiasında.

T-Pot zaten çok sayıda açık kaynak honeypot'u tek platformda topluyor. citeturn31view2turn23search0turn27search3turn31view0

Dolayısıyla ürünümüzün vaadi sadece:

> “Kolay kurulan honeypot.”

olursa güçlü bir differentiation elde etmeyebiliriz.

Benim araştırma sonucundaki **RECOMMENDED positioning hypothesis** şu:

> **SOC'u olmayan küçük/orta ölçekli şirketler için, kurulduktan sonra güvenlik uzmanlığı gerektirmeden iç ağdaki recon, credential abuse ve lateral movement'i yüksek güvenle tespit eden; ne olduğunu ve ne yapılması gerektiğini AI ile anlaşılır biçimde açıklayan deception platformu.**

Burada anahtar kelime “honeypot” değil.

Anahtar değer:

**“Birisi ağında olmaması gereken bir şey yapıyor; bunu erken fark et ve ne yapacağını bil.”**

### SMB için ürünün düşmanı saldırgan kadar karmaşıklık

Enterprise ürünlerde şu yeteneklerin hemen hepsini gördük:

- yüzlerce decoy,
- OT/SCADA,
- identity deception,
- cloud,
- SaaS,
- central manager,
- SOC/SIEM/SOAR,
- attack surface management,
- threat intelligence,
- MFA/VPN,
- forensic analysis,
- adaptive deception,
- endpoint integration.

Fakat 15 kişilik yazılım şirketindeki teknik sorumlu muhtemelen bunların çoğunu istemiyor.

Onun muhtemel job-to-be-done'u:

> “Bunu bir sunucuya kurayım. Ağımı tanısın. Bana birkaç mantıklı fake asset önersin. Birisi onlara dokunursa bana gerçekten önemli bir şey olduğunda haber versin. Sonra bana ne olduğunu ve ne yapmam gerektiğini söylesin.”

Bu persona yaklaşımı zaten sizin tarafınızdan **APPROVED** olduğundan, bundan sonraki ürün tartışmalarında bu kullanıcıyı enterprise SOC analyst'e dönüştürmemeye dikkat etmemizi öneriyorum.

### “Gerçek zafiyetli VM” ürünün merkezi olmak zorunda değil

Araştırma önemli bir spektrum gösteriyor.

OpenCanary gibi sistemler gerçek vulnerability gerektirmeden detection yapabiliyor. citeturn31view2

Cowrie emulated shell'den gerçek QEMU backend'e kadar interaction seviyesini artırabiliyor. citeturn31view3

Guardpot QEMU/KVM host ve forensic snapshot yönetiyor. citeturn29view1

FortiDeceptor gerçekçi Linux/Windows/SCADA/IoT/custom decoy'ları virtualized biçimde sunuyor. citeturn25view0

Bunun sonucunda şu model teknik açıdan daha mantıklı bir araştırma çerçevesi:

```text
Interaction Level

LOW
│
│  Port / service simulation
│  Fake credentials
│  Tokens
│
├── Protocol emulation
│
├── Interactive fake environment
│
├── Containerised fake application
│
├── VM-backed environment
│
└── Intentionally vulnerable full system
HIGH
```

Risk ve kaynak tüketimi aşağıdan yukarıya doğru artacaktır.

Dolayısıyla **“her decoy ayrı vulnerable VM olsun” varsayımı şu aşamada tavsiye edilmiyor.**

Gerçek vulnerable VM'ler ileri seviye/high-interaction template sınıfı olabilir.

Durum: **RECOMMENDED — OWNER DECISION bekliyor.**

### Windows bir feature değil, product/platform problemi

Fortinet örneği özellikle bunu kanıtlıyor. Windows decoy'ların teknik desteği yanında lisans modeli doğrudan SKU ve deployment modelini etkiliyor. citeturn25view2

Dolayısıyla ileride ayrı ayrı cevaplayacağımız sorular şunlar olacak:

Windows'u gerçekten çalıştıracak mıyız?

Windows'u emulate mı edeceğiz?

Fake SMB/RDP/WinRM servisleri yeterli olacak mı?

Customer-supplied Windows image destekleyecek miyiz?

Customer-owned licence mı kullanılacak?

Golden image nasıl patchlenecek?

Snapshot/reset nasıl yapılacak?

Malware execution ihtimalinde outbound traffic nasıl sınırlanacak?

Bunları bugün çözmeyeceğiz.

### “Attacker IP intelligence” iç ağ için farklı düşünülmeli

Public honeypot sistemlerinde source IP için ASN, ülke, city, reputation ve global history oldukça yararlı. Guardpot bunu yoğun biçimde kullanıyor. citeturn28view4

Ama bizim ana kullanımımız internal network olduğunda saldırganın IP'si çoğu zaman:

`192.168.x.x`

`10.x.x.x`

`172.16.x.x`

olacaktır.

Bu durumda asıl değer:

```text
IP
 ↓
MAC
 ↓
DHCP identity
 ↓
Hostname
 ↓
Device fingerprint
 ↓
Switch/AP/VLAN
 ↓
Known employee/device
 ↓
Observed previous behaviour
```

zincirinde olabilir.

Yani bizim saldırgan intelligence yaklaşımımız Guardpot'un internet threat-intelligence yaklaşımından farklılaşabilir.

Bu nokta Adım Üç'te network teknik araştırmasının en önemli başlıklarından biri olmalı.

### Asıl “moat” honeypot engine olmayabilir

Araştırma sonucunda benim en güçlü çıkarımım budur.

Open source zaten:

SSH sağlar. citeturn31view3

PostgreSQL credential trap sağlar. citeturn20search1

SMB/MySQL/MSSQL vb. sağlar. citeturn31view4

ICS sağlar. citeturn20search0

LLM-generated HTTP sağlar. citeturn30view4

20+ honeypot'u container olarak paketleyen platform sağlar. citeturn31view0

Dolayısıyla bizim teknik/ürün değerimiz muhtemelen şurada oluşacaktır:

**orchestration + effortless deployment + network placement + signal quality + correlation + isolation + attacker journey + AI explanation + automated guidance.**

Bunu çok önemli bir **RECOMMENDED product thesis** olarak kaydediyorum.

## Araştırmadan çıkan ürün alanı

Bütün rakipleri bir eksene yerleştirdiğimizde şu tablo oluşuyor:

```text
                    HIGH OPERATIONAL COMPLEXITY
                              ▲
                              │
          CounterCraft       │        Acalvio
                              │
                    FortiDeceptor
                              │
                     Guardpot │
                              │
──────────────────────────────┼──────────────────────────────►
Simple Detection              │                  Full Deception /
                              │                  Threat Intelligence
                              │
       Thinkst Canary         │
                              │
       OpenCanary             │
                              │
                    ? OUR PRODUCT ?
                              │
                    LOW OPERATIONAL COMPLEXITY
```

Buradaki `? OUR PRODUCT ?` henüz positioning kararı değildir.

Fakat araştırma sonucunda ortaya çıkan olası boşluk şu:

> **Thinkst/OpenCanary düzeyinde deployment ve operasyon sadeliği + Guardpot/Cowrie düzeyinde giderek derinleşebilen deception + SOC'u olmayan kullanıcıya AI-native investigation.**

Bunu şu şekilde düşünmek mümkün:

```text
                  Detection confidence
                         HIGH
                          ▲
                          │
                 ┌────────┴────────┐
                 │                 │
              Decoys          Deception
              Tokens           Personas
                 │                 │
                 └────────┬────────┘
                          │
                  Event Correlation
                          │
                          ▼
                  Incident Evidence
                          │
                ┌─────────┴─────────┐
                │                   │
             Human UI              AI
                │              Investigation
                │                   │
                └─────────┬─────────┘
                          │
                          ▼
                  "What happened?"
                  "How serious?"
                  "Which device?"
                  "What should I do?"
```

Bu noktada AI'nın ürünün üzerine yapıştırılmış bir chatbot olmaması gerekiyor.

Gerçek anlamda AI-native ürün için modelin en azından aşağıdaki yerlerden bazılarında asli fonksiyon taşıması gerekir:

**investigation:** yüzlerce event'i tek attacker journey'ye dönüştürmek,

**explanation:** teknik telemetry'yi personasının anlayacağı dile çevirmek,

**response guidance:** mevcut kanıta göre kontrollü remediation önerileri oluşturmak,

**deception generation:** network'e uygun persona/template üretmek,

**adaptive deception:** gözlenen saldırıya göre kontrollü fake surface'i değiştirmek,

**interactive deception:** attacker-facing shell/web/application davranışını daha gerçekçi yapmak,

**AI infrastructure deception:** fake LLM/MCP/API gibi yeni nesil decoy'lar oluşturmak.

Bunların hiçbirini henüz MVP kapsamına almıyorum.

Fakat AI-native için ürün vizyonumuz artık “dashboard'da bir chat kutusu olsun” seviyesinin çok ötesine taşınabilir.

## Karar kaydı ve bir sonraki kapı

Bu raporla birlikte karar modelimiz resmi olarak işletilmeye başlanmıştır.

### RESEARCHED

Aşağıdaki konular yeterli ilk pazar araştırması seviyesine ulaştı:

| Araştırma konusu | Durum |
|---|---|
| Guardpot'un güncel ürün kapsamı | **RESEARCHED** |
| Guardpot deployment/agent/QEMU yaklaşımı | **RESEARCHED** |
| Guardpot AI-native özellikleri | **RESEARCHED** |
| Guardpot satış ve gelir modeli sinyalleri | **RESEARCHED** |
| Thinkst Canary ürün yaklaşımı | **RESEARCHED** |
| FortiDeceptor enterprise architecture/licensing yaklaşımı | **RESEARCHED** |
| Acalvio adaptive deception yaklaşımı | **RESEARCHED** |
| Zscaler GenAI deception yaklaşımı | **RESEARCHED** |
| SentinelOne identity deception yaklaşımı | **RESEARCHED** |
| CounterCraft attacker-intelligence yaklaşımı | **RESEARCHED** |
| OpenCanary | **RESEARCHED** |
| Cowrie | **RESEARCHED** |
| T-Pot | **RESEARCHED** |
| Galah | **RESEARCHED** |
| Dionaea / Heralding / Conpot | **RESEARCHED — secondary candidates** |
| AI-native deception pattern'ları | **RESEARCHED** |

### RECOMMENDED — owner decision gerekli

Araştırma sonucunda ben aşağıdaki önerileri **Adım İki'nin başlangıç hipotezi** olarak sunuyorum; bunları otomatik olarak kabul edilmiş saymıyorum.

**Ürün kategorisi önerisi:** Kendimizi ilk aşamada “başka bir enterprise deception platform” olarak değil, **SOC'u olmayan SMB'ler için internal breach detection/deception ürünü** olarak değerlendirelim.

**Değer önerisi önerisi:** Ana değer “çok honeypot çalıştırmak” değil, **yüksek güvenli sinyal + saldırgan journey'si + ne olduğunu/ne yapılacağını açıklama** olsun.

**Interaction modeli önerisi:** Ürünü tek honeypot türüne kilitlemeyelim. Teknik araştırmayı low → medium → high interaction şeklinde katmanlı model üzerinden sürdürelim.

**Open-source yaklaşımı önerisi:** OpenCanary, Cowrie, T-Pot ve Galah'ı “kullanacağımız teknolojiler” olarak değil, önce **reference implementation / candidate building block** olarak tutalım. Build-vs-integrate kararı Adım Dört'te verilsin.

**Gerçek vulnerable VM önerisi:** Ürünün temel detection mekanizması değil, ileri seviye/high-interaction deception sınıfı olarak araştırılsın.

**AI-native öncelik önerisi:** İlk ürün araştırmasında en yüksek önceliği attacker-facing chatbot değil, **incident correlation + explanation + remediation guidance** tarafına verelim. Interactive LLM deception ikinci ayrı araştırma hattı olarak ilerlesin.

**Deterministik güvenlik önerisi:** AI security event'in tek doğruluk kaynağı olmasın. Network/honeypot telemetry ve deterministic evidence esas olsun; AI bunun üzerinde reasoning ve explanation katmanı olarak çalışsın. Daha ileri sürümlerde adaptive deception ayrıca değerlendirilsin.

**Product simplicity önerisi:** Guardpot'un VGN/VPN, mail security, ASM gibi çevre modüllerini şu aşamada kopyalanacak feature set olarak görmeyelim. Core job-to-be-done dışındaki modüller ancak açık bir kullanıcı problemiyle gerekçelendirildiğinde kapsama girsin.

### OPEN — sonraki owner decision noktaları

Adım İki'ye geçerken özellikle şu ürün sorularını sizinle tartışmamız gerektiğini düşünüyorum:

**Network-only deception mı, yoksa deception tokens da ürün vizyonunun parçası mı?**

Bu ciddi bir kapsam ayrımı. Fake SSH/PostgreSQL/Windows sistemleri network deception'dır; fake credentials, Word files, API keys, SSH config, browser cookies ve AD identities ise endpoint/identity deception'a geçiştir. Thinkst ve Guardpot ikincisinin çok değerli olduğunu gösteriyor. citeturn23search1turn29view3

**İç ağ mı birincil, internet-facing honeypot da mı?**

İki kullanım birbirine benzese de ürün problemi farklıdır. Public honeypot ağı global threat intelligence üretirken internal honeypot doğrudan breach/lateral-movement sinyali üretir. Guardpot ikisini geniş ölçekte birleştiriyor; OpenCanary özellikle private network post-breach detection'a odaklanıyor. citeturn28view4turn31view2

**MSP/MSSP gelecekte ayrı persona olacak mı?**

CounterCraft gibi platformlarda multi-client/MSSP merkezi yönetimi stratejik bir kanal haline gelmiş durumda; Guardpot da distributor/partner yapısıyla ölçekleniyor. citeturn26search27turn10search0

Biz SMB'ye doğrudan satmak yerine bir noktada:

> “Bir IT hizmet şirketi 50 müşterisinin küçük ağlarını tek panelden izler”

modeline gidebiliriz.

Bu çok güçlü bir distribution modeli olabilir fakat multi-tenancy, RBAC, tenant isolation, billing ve central management gereksinimlerini ciddi biçimde değiştirir. Bu yüzden şimdi değil ama erken product-positioning aşamasında bilinçli karar verilmesi gerekir.

**Tamamen on-prem/offline çalışma bir ürün ilkesi olacak mı?**

Guardpot on-prem LLM kullanımını veri dışarı çıkmaması avantajıyla anlatıyor. citeturn21search1 Galah ise OpenAI, Anthropic, Google ve local Ollama gibi hem cloud hem local modelleri destekliyor. citeturn30view4

Biz “AI provider agnostic” kalabiliriz; fakat müşterinin security telemetry'sinin cloud model provider'a gidip gitmeyeceği önemli bir product decision olacaktır.

**Ürünün nihai kimliği detection mı, deception mı, yoksa attacker intelligence mı?**

Bunları bir progression olarak görebiliriz:

```text
          Level A
        "Biri burada."
             │
             ▼
          Level B
   "Şunu yapmaya çalışıyor."
             │
             ▼
          Level C
 "Şuradan geldi, şuraları gezdi."
             │
             ▼
          Level D
   "Onu fake ortamda tuttuk."
             │
             ▼
          Level E
 "Davranışından yeni intelligence
          ürettik."
```

OpenCanary ve Thinkst ağırlıklı olarak ilk katmanlarda çok güçlü. citeturn31view2turn23search1

Guardpot orta ve ileri katmanlara yayılıyor. citeturn28view4turn29view1

CounterCraft ise intelligence tarafını ürünün merkezine taşıyor. citeturn26search3turn26search15

Bizim hangi seviyeyi **ürünün sözü**, hangilerini **ilerleyen capability'ler** olarak tanımlayacağımız Adım İki'nin esas ürün kararı olacaktır.

Bu nedenle Adım Bir sonunda benim en önemli araştırma sonucum şudur:

> **Başlangıç fikrimizin teknik karşılıkları var, pazar doğrulanmış durumda ve açık kaynak yapı taşları oldukça zengin. Dolayısıyla esas problem “honeypot yapabilir miyiz?” değil. Esas problem, enterprise deception ürünlerinin karmaşıklığına düşmeden, SOC'u olmayan küçük işletme için hangi problemi olağanüstü derecede iyi çözeceğimizi seçmek.**

Ve bana göre araştırmanın şu an işaret ettiği en güçlü aday problem:

> **“Ağının içine biri girdiğinde veya içeride keşif/lateral movement yapmaya başladığında bunu mümkün olduğunca erken, yüksek güvenle fark et; saldırganın ne yaptığını anlaşılır biçimde göster ve güvenlik uzmanı olmadan atılması gereken sonraki adımı açıkla.”**

Bu ifade şu anda **RECOMMENDED** durumundadır. **OWNER DECISION** olmadan ürün positioning'i olarak **APPROVED** durumuna geçirilmemiştir.