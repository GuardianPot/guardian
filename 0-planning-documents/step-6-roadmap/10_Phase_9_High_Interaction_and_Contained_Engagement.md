# Phase 9 — High-Interaction & Contained Engagement
## Final Approved Implementation Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Faz amacı

Attacker'a real OS/application semantics sağlayarak:
- arbitrary command behavior,
- uploaded tools/files,
- persistence attempts,
- richer post-compromise TTP evidence

gözlemlemek; bunu production network veya üçüncü taraflar için saldırı platformu yaratmadan yapmak.

# 2. New deployable

**High-Interaction Worker**
- dedicated Linux node approved
- Go control daemon
- libvirt
- QEMU/KVM
- qcow2 overlays
- isolated virtual networks

# 3. Entry criteria

- Phase 5 product stable
- containment threat model approved
- high-interaction customer/security value validated
- dedicated worker operational model accepted
- Windows licensing model reviewed

# 4. Workstreams

## P9-W1 — Worker enrollment/health

Reuse device PKI but separate capability class.

## P9-W2 — libvirt/QEMU lifecycle

- golden image
- COW overlay
- boot
- health
- quarantine
- destroy/reset
- resource quotas.

## P9-W3 — Isolated virtual network

Default:
- no route to production
- no arbitrary internet
- separate management/control channel
- nftables enforcement outside guest.

## P9-W4 — Controlled egress simulation

Possible:
- DNS sink/simulation
- HTTP proxy/sink
- allowlisted research endpoints if owner explicitly approves

Never unrestricted by default.

## P9-W5 — Linux high-interaction template

Real OS semantics with no production secrets.

## P9-W6 — Windows high-interaction template

Customer-provided licensed media/image workflow.
No bundled redistribution unless later licensing decision.

## P9-W7 — Engagement telemetry

Capture:
- auth/session
- commands
- process/file change where feasible
- network attempts
- uploaded file hashes/blobs
- timeline.

Must preserve evidence provenance independently of compromised guest.

## P9-W8 — Malware/file quarantine

- content-addressed storage
- no inline execution
- restricted download
- hash/metadata
- optional later analysis integration.

## P9-W9 — Reset/quarantine lifecycle

States:
Clean → Engaged → Assumed Compromised → Quarantined → Reset.

## P9-W10 — Incident integration

High-interaction evidence is richer subtype, not separate incident product.

# 5. Required red-team tests

- guest root/admin compromise
- guest attempts production route
- guest attempts management interface
- resource exhaustion
- reboot/persistence
- malformed telemetry
- snapshot reset
- worker restart
- hostile file capture.

# 6. Exit gate

- [ ] dedicated worker security boundary validated
- [ ] production pivot denied
- [ ] arbitrary internet abuse denied
- [ ] disposable reset proven
- [ ] Linux template
- [ ] Windows workflow if included
- [ ] high-interaction evidence integrated
- [ ] malware quarantine safe
- [ ] operator health/quarantine UX

---

# 7. Detailed Phase 9 Acceptance Criteria

## P9-AC-001 — Guest compromise does not equal host compromise
**Given** attacker obtains root/Administrator inside the guest  
**When** they attempt host/management access  
**Then** the worker, libvirt control path, device credentials and production network remain inaccessible under the tested threat model.

## P9-AC-002 — Production pivot denied
**Given** a fully compromised guest  
**When** it attempts connections to protected customer production ranges  
**Then** the external worker/network policy denies them and records the attempts.

## P9-AC-003 — Third-party abuse denied
Arbitrary internet scanning/attacks from the guest are denied by default; any controlled egress simulator/allowlist is explicit and audited.

## P9-AC-004 — Known-good reset
**Given** an engaged/compromised guest  
**When** the engagement is closed  
**Then** the next clean instance is built from verified golden state rather than trusting in-guest cleanup.

## P9-AC-005 — Worker restart recovery
A worker restart cannot accidentally reconnect a quarantined compromised guest to production or mark an unknown VM as clean.

## P9-AC-006 — Evidence survives guest tampering
High-value session/network/file evidence required by the product is collected or committed outside the attacker-controlled guest trust domain.

## P9-AC-007 — Hostile file quarantine
Captured malware/tool files are hash-addressed, access-controlled and never inline-executed/rendered by the Web Console.

## P9-AC-008 — Windows licensing provenance
Any Windows high-interaction instance has recorded image/licensing provenance consistent with the approved customer-provided media/image model.

## P9-AC-009 — Resource exhaustion
Guest CPU/memory/disk/process/network abuse cannot starve the worker control plane beyond approved quotas without producing critical health state.

## P9-AC-010 — Incident continuity
Escalation from low/medium deception to high-interaction evidence updates the same evidence/incident/journey model; a separate incompatible investigation silo is not created.

# 8. Promotion Gate

High interaction remains optional/advanced until independent security testing validates isolation and egress behavior.

---

## Final Phase Status

- **Phase:** Phase 9 — High-Interaction & Contained Engagement
- **Status:** APPROVED / FINAL
- **Owner decision:** Tüm bu faz kapsamı, sequencing, dependencies, acceptance criteria ve exit gate'leri onaylandı.
- **Change policy:** Bu fazın kapsamı implementation sırasında sessizce genişletilemez/daraltılamaz; değişiklik Product Owner change-control sürecine tabidir.
