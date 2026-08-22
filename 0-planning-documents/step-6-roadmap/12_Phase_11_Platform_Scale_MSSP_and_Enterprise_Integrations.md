# Phase 11 — Platform Scale, MSSP & Enterprise Integrations
## Final Approved Implementation Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Faz amacı

Product design center'i değiştirmeden daha büyük operational models'i desteklemek:
- multiple organizations/customers,
- delegated operation,
- enterprise identity,
- broader security integrations,
- data-plane scaling only where metrics justify.

# 2. Prerequisite principle

Bu fazdaki distributed technologies **feature değil**, measured scale/availability needs'e cevaptır.

Kafka, ClickHouse, Kubernetes vb. yalnız architecture change-control ve benchmark trigger ile değerlendirilir.

# 3. Workstreams

## P11-W1 — Multi-tenancy

Introduce:
- tenant/org hierarchy
- strict data ownership
- authorization boundaries
- tenant-aware object storage
- tenant-aware device identities
- migration from single-org model.

Security isolation tests mandatory.

## P11-W2 — MSSP console

Capabilities:
- customer list
- incident queue across tenants
- delegated access
- per-customer policy
- escalation
- no cross-tenant evidence leakage.

## P11-W3 — Fine-grained RBAC

Roles/capabilities:
- owner
- operator
- responder
- read-only
- delegated MSSP

Permission model deny-by-default.

## P11-W4 — OIDC/SSO enterprise hardening

Providers:
- generic OIDC
- SAML only if demanded, separate decision.

MFA policy integrates external IdP where appropriate.

## P11-W5 — Native notification integrations

- Slack
- Teams
- Pager/on-call systems.

Generic webhook remains foundational.

## P11-W6 — SIEM integration

Export:
- incidents
- evidence summaries
- health/security audit

Avoid turning product into general SIEM ingestion.

## P11-W7 — EDR/NAC/firewall response integrations

Initially:
- read/context integration
- prepare containment action
- explicit human approval
- reversible where possible.

Autonomous response remains separately governed.

## P11-W8 — Data-plane scale triggers

Measure:
- event ingest
- retention/query
- independent consumer fan-out
- correlation throughput.

Potential evolutions:
- NATS JetStream when durable fan-out/replay needed
- ClickHouse when Postgres telemetry analytics proven bottleneck
- Redis only if actual low-latency ephemeral use case appears
- graph DB only if relational graph proven inadequate.

## P11-W9 — Control Plane HA

Add:
- multiple app replicas
- leader/lease for jobs
- database HA deployment profile
- object store HA
- graceful rolling upgrades.

## P11-W10 — Kubernetes deployment profile

Only if:
- hosted scale/ops team benefit is proven,
- not required for customer Edge.

Kubernetes does not become product dependency.

## P11-W11 — Compliance/audit exports

Customer demand may justify:
- audit reports
- retention controls
- evidence exports

Not a generic compliance suite.

# 4. Exit gate

Phase 11 scope itself should be demand-driven. For approved subset:
- [ ] tenant isolation independently tested
- [ ] delegated auth no cross-tenant leakage
- [ ] integrations preserve human approval
- [ ] scale technology introduced only with benchmark ADR
- [ ] single-customer simplicity still available

---

# 5. Detailed Phase 11 Acceptance Criteria

## P11-AC-001 — Tenant isolation
**Given** two tenants/customers  
**When** any API/UI/query/object-storage/device path is exercised  
**Then** one tenant cannot read, mutate, enumerate or infer protected data belonging to the other.

## P11-AC-002 — Delegated MSSP access
Delegated operator permissions are explicitly scoped by customer/role and are fully audited.

## P11-AC-003 — RBAC deny-by-default
A newly introduced privileged action is unavailable to roles until an explicit permission mapping exists.

## P11-AC-004 — Enterprise identity
OIDC/SSO login maps to a local authorization identity without making external IdP claims a substitute for product authorization checks.

## P11-AC-005 — SIEM export boundary
SIEM integration exports approved product evidence/incidents/audit; it does not silently convert the product into a general log-ingestion platform.

## P11-AC-006 — Human-approved containment integration
Any EDR/NAC/firewall containment action requires explicit human approval at the approved authority tier and has an audit record/result.

## P11-AC-007 — Scale technology evidence
NATS/ClickHouse/Kubernetes/graph DB/Redis or other new platform technology cannot enter production baseline without an ADR containing measured bottleneck, alternatives and rollback/migration impact.

## P11-AC-008 — Horizontal Control Plane
If HA is in scope, loss of one application replica does not corrupt jobs/incidents or cause duplicate irreversible side effects.

## P11-AC-009 — Backward-compatible device fleet
Control Plane scale/HA evolution preserves supported Edge protocol compatibility and does not require synchronized whole-fleet upgrade.

## P11-AC-010 — Single-customer simplicity
Enterprise/MSSP features do not force a single-customer deployment to configure tenant hierarchy/complex RBAC unnecessarily.

# 6. Demand Gate

Phase 11 work packages are activated by validated business/operational demand, not by “enterprise architecture completeness”.

---

## Final Phase Status

- **Phase:** Phase 11 — Platform Scale, MSSP & Enterprise Integrations
- **Status:** APPROVED / FINAL
- **Owner decision:** Tüm bu faz kapsamı, sequencing, dependencies, acceptance criteria ve exit gate'leri onaylandı.
- **Change policy:** Bu fazın kapsamı implementation sırasında sessizce genişletilemez/daraltılamaz; değişiklik Product Owner change-control sürecine tabidir.
