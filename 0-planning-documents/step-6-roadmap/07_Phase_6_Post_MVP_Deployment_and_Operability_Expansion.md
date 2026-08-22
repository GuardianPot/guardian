# Phase 6 — Post-MVP Deployment & Operability Expansion
## Final Approved Implementation Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Faz amacı

Pilot MVP'nin deployment assumptions'ını genişletmek ve ürünün daha fazla gerçek müşteri topolojisinde **self-service ve resilient** çalışmasını sağlamak.

Bu faz deception breadth'ten önce/yanında deployment friction'ını azaltır.

# 2. Why this phase follows MVP

Pilot feedback'in en kritik alanlarından biri:
- network compatibility,
- onboarding burden,
- multiple segments/sites,
- customer-hosted requirements,
- internet dependency

olacaktır.

Bu genişleme yapılmadan endpoint/identity gibi daha fazla footprint eklemek operasyon yükünü büyütebilir.

# 3. Capability groups

## P6-W1 — Self-service onboarding hardening

From controlled onboarding → self-service:
- prerequisite checker
- downloadable appliance
- guided network validation
- failure remediation suggestions
- test interaction wizard
- support-free happy path

Acceptance:
- fresh qualified operator documentation dışında code/config assistance olmadan enroll/deploy/test yapabilir.

## P6-W2 — Multi-Edge

Implement:
- multiple Edge devices per environment
- per-zone ownership/placement
- conflict prevention
- independent health
- cross-edge incident correlation
- upgrade compatibility

Acceptance:
- same source/evidence across two Edge devices can correlate without device identity ambiguity,
- one Edge offline does not mark entire environment blind without coverage explanation.

## P6-W3 — VLAN trunk support

Implement:
- 802.1Q subinterfaces
- owner-specified VLAN ID/interface
- validation
- cleanup/audit

No automatic switch configuration.

## P6-W4 — macvlan presence driver

Goal:
- unique L2 identity when network permits.

Acceptance:
- switch compatibility matrix documented,
- port-security failure yields safe degraded state.

## P6-W5 — ipvlan presence driver

Goal:
- networks where multiple MACs problematic.

Acceptance:
- behavior/fingerprint limitations documented,
- driver selectable per placement.

## P6-W6 — Multi-NIC support

Management and deception interfaces can be separated.

Acceptance:
- no accidental management listener exposure on deception-only interface unless explicitly configured.

## P6-W7 — IPv6 runtime

Implement:
- IPv6 source/destination
- address conflict/NDP
- decoy listeners
- evidence/UI
- correlation

Dual-stack tests required.

## P6-W8 — Cloud private network profile

Validate/support:
- common VPC/VNet routed private subnets
- cloud VM Edge
- security group/firewall prerequisites
- no requirement for public decoy exposure

Specific cloud-native discovery remains optional.

## P6-W9 — VPN-connected network context

Support routed private reachability through customer VPN.
Optional enrichment adapter for VPN identity can be separate later.

## P6-W10 — Customer-hosted Control Plane

Same artifact deployment:
- on-prem Compose profile
- local Postgres/object backend
- local secrets
- external AI configurable

Acceptance:
- no SaaS-only hidden dependency for deterministic core.

## P6-W11 — Fully disconnected/degraded profile foundations

Goal:
- local deterministic operation without internet
- AI disabled/local-future
- manual signed update path prepared

Full offline product may require additional packaging.

## P6-W12 — Signed offline update bundle

Implement TUF/Cosign verified import:
- Edge
- decoy packs
- Control Plane artifacts as applicable

## P6-W13 — OIDC optional authentication

This may be pulled later into Phase 11 if customer demand is weak.

Approved placement: optional post-MVP auth integration after local auth stable.

# 4. Dependencies

- Phase 5 stable updater
- health/coverage model
- desired-state reconciliation
- multi-device protocol already modeled
- product validation showing deployment is primary expansion vector

# 5. Non-goals

- endpoint agent
- high interaction
- MSSP tenant hierarchy
- autonomous containment
- full NDR

# 6. Exit gate

- [ ] multi-edge
- [ ] at least routed + one L2 presence driver production validated
- [ ] VLAN/multi-NIC validated
- [ ] IPv6 support passes
- [ ] on-prem Control Plane profile
- [ ] offline update path
- [ ] self-service onboarding acceptance
- [ ] coverage semantics remain explicit across multiple Edge nodes

---

# 7. Detailed Phase 6 Acceptance Criteria

## P6-AC-001 — Self-service qualified install
**Given** supported customer prerequisites and published documentation  
**When** a qualified generalist operator performs installation without source-code/config-file intervention  
**Then** Control Plane setup, Edge enrollment, zone definition, decoy deployment and test interaction complete through product-supported flows.

## P6-AC-002 — Second Edge enrollment
**Given** an environment with one healthy Edge  
**When** a second Edge enrolls  
**Then** both have unique identity, independent health, desired-state ownership and no certificate/config collision.

## P6-AC-003 — Cross-Edge journey
**Given** two Edge nodes observe evidence attributable to the same strong source/entity anchors  
**When** correlation rules link the sequence  
**Then** the incident journey can span both Edges while provenance preserves which Edge observed each evidence item.

## P6-AC-004 — Partial Edge outage
**Given** two Edge nodes  
**When** one becomes offline  
**Then** the environment does not report blanket healthy coverage; affected zones are marked degraded while the healthy Edge continues operating.

## P6-AC-005 — VLAN placement
**Given** an explicitly configured VLAN trunk and permitted VLAN ID  
**When** a decoy is deployed to that VLAN  
**Then** placement is limited to the selected VLAN and removal cleans the subinterface/address/rules without affecting other VLANs.

## P6-AC-006 — L2 driver incompatibility
**Given** a switch/network that rejects the chosen macvlan/ipvlan behavior  
**When** validation fails  
**Then** deployment stops safely and surfaces an actionable compatibility reason instead of silently falling back.

## P6-AC-007 — Multi-NIC management separation
**Given** dedicated management and deception interfaces  
**When** Edge is configured  
**Then** management listeners/credentials are not exposed through the deception-only interface unless explicitly approved.

## P6-AC-008 — IPv6 evidence
**Given** IPv6-capable decoy placement  
**When** an IPv6 source interacts  
**Then** source/destination addresses, correlation and UI preserve the full IPv6 identity without truncation or IPv4 assumptions.

## P6-AC-009 — Customer-hosted deterministic core
**Given** customer-hosted Control Plane without SaaS connectivity  
**When** Edge sends evidence  
**Then** enrollment/management, deterministic detection, incident creation, journey and local health continue; external AI may be explicitly degraded.

## P6-AC-010 — Offline update trust
**Given** a signed offline update bundle  
**When** it is imported  
**Then** the same signature/metadata/rollback checks used by online update paths are enforced; a tampered or revoked bundle is rejected.

# 8. Phase 6 Validation Artifacts

Required evidence:
- self-service installation recording/checklist,
- multi-Edge network-lab fixture,
- VLAN/macvlan/ipvlan compatibility matrix,
- IPv6 attack fixture,
- on-prem dependency inventory proving no hidden SaaS correctness dependency,
- offline update tamper fixtures.

---

## Final Phase Status

- **Phase:** Phase 6 — Post-MVP Deployment & Operability Expansion
- **Status:** APPROVED / FINAL
- **Owner decision:** Tüm bu faz kapsamı, sequencing, dependencies, acceptance criteria ve exit gate'leri onaylandı.
- **Change policy:** Bu fazın kapsamı implementation sırasında sessizce genişletilemez/daraltılamaz; değişiklik Product Owner change-control sürecine tabidir.
