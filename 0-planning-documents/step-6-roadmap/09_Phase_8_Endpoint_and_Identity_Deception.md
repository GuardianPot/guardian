# Phase 8 — Endpoint & Identity Deception
## Final Approved Implementation Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Faz amacı

Product vision'daki network-only boundary'yi genişleterek:
- endpoint breadcrumbs,
- synthetic credentials,
- fake config/references,
- identity deception

ile attacker'ın host/credential discovery davranışını daha erken ve daha zengin biçimde yakalamak.

# 2. New deployable

Introduce **Endpoint Deception Agent** as architecture-approved future component.

Approved properties:
- small native service
- Windows first, Linux later/parallel based on customer mix
- outbound mTLS device identity
- narrow permissions
- no EDR/process-monitoring scope
- no arbitrary remediation engine

# 3. Workstreams

## P8-W1 — Endpoint agent enrollment/lifecycle

Reuse device PKI concepts but separate device type/capability.

Acceptance:
- install/uninstall
- cert rotation/revocation
- health
- signed update
- local audit.

## P8-W2 — Breadcrumb placement framework

Typed placement objects:
- fake config file
- connection profile
- admin note
- share reference
- synthetic credential reference
- database connection hint

Placement must be:
- reversible
- owner-approved
- inventory tracked
- stale/orphan detectable.

## P8-W3 — Honey credential endpoint placement

Credentials:
- zero production authority
- decoy-only or trigger-only
- scoped placement
- revocation

Trigger joins existing incident model.

## P8-W4 — Windows credential/artefact safety

Avoid:
- LSASS manipulation
- production credential injection
- techniques that create actual privilege
unless separately approved.

## P8-W5 — Directory-visible fake identities

Future-controlled capability:
- fake users/computers/service accounts
- default no privileged production access
- explicit directory-write policy
- rollback
- collision/name lifecycle
- audit.

Production AD write requires dedicated security review.

## P8-W6 — Identity evidence model

Add:
- identity entity
- placement ID
- token/credential ID
- endpoint source context
- directory source
- confidence provenance.

## P8-W7 — Endpoint/Network journey correlation

Example:
endpoint breadcrumb accessed → honey credential used → SMB decoy → SSH decoy.

Acceptance:
- same synthetic identity creates strong correlation edge,
- observed vs inferred still separated.

## P8-W8 — Privacy/data controls

Endpoint artefacts can involve employee/device context.
Required:
- minimization
- placement visibility
- retention
- no collection beyond deception job.

# 4. Explicit non-goals

Still not:
- antivirus
- process-wide EDR
- keylogging
- arbitrary file monitoring
- autonomous isolation
- production credential theft detection engine
- UEBA.

# 5. Dependencies

- mature device lifecycle from Phase 6
- signed updater
- incident/entity graph
- synthetic credential lifecycle
- pilot validation of customer appetite for endpoint footprint

# 6. Exit gate

- [ ] Endpoint Agent deployable/healthy/revocable
- [ ] at least 3 safe breadcrumb types
- [ ] endpoint synthetic credential workflow
- [ ] reversible cleanup
- [ ] network+endpoint incident correlation
- [ ] no EDR scope creep
- [ ] privacy/security review complete

---

# 7. Detailed Phase 8 Acceptance Criteria

## P8-AC-001 — Endpoint agent lifecycle
**Given** a supported endpoint  
**When** the agent is installed/enrolled/updated/uninstalled  
**Then** device identity, health, signed update and cleanup are complete without leaving orphan deception artifacts.

## P8-AC-002 — Least privilege
The endpoint agent runs with the minimum privilege required by the approved breadcrumb types; a new privileged capability requires separate security review.

## P8-AC-003 — Reversible breadcrumb
**Given** an approved breadcrumb placement  
**When** the operator retires it  
**Then** the exact product-created artifact can be removed without deleting legitimate neighboring customer content.

## P8-AC-004 — Zero-authority credential
A synthetic credential placed on an endpoint must not authenticate to production resources; its accepted authority is limited to approved decoy/trigger semantics.

## P8-AC-005 — Placement inventory
The product can answer for every active breadcrumb: what it is, which endpoint/path/context owns it, when it was placed, current health/staleness and how to revoke it.

## P8-AC-006 — Endpoint-to-network journey
**Given** an endpoint breadcrumb/credential trigger followed by network decoy interaction  
**When** strong shared identifiers exist  
**Then** the incident can correlate both evidence families without rewriting either observed source.

## P8-AC-007 — Directory write safety
If production directory-visible deception is enabled, every object creation/update/delete is explicit, audited, reversible and constrained to approved object classes/containers; no privileged production access is granted by default.

## P8-AC-008 — No EDR creep
The endpoint agent does not continuously collect arbitrary process/file/user activity unrelated to approved deception jobs.

## P8-AC-009 — Privacy minimization
Endpoint telemetry contains only fields required for deception placement/trigger/health/correlation and follows configurable retention.

# 8. Security Review Gate

Before endpoint-wide rollout:
- Windows/Linux privilege review,
- installer/update review,
- artifact cleanup test,
- directory-deception threat model if used,
- endpoint tamper behavior,
- privacy data-flow review.

---

## Final Phase Status

- **Phase:** Phase 8 — Endpoint & Identity Deception
- **Status:** APPROVED / FINAL
- **Owner decision:** Tüm bu faz kapsamı, sequencing, dependencies, acceptance criteria ve exit gate'leri onaylandı.
- **Change policy:** Bu fazın kapsamı implementation sırasında sessizce genişletilemez/daraltılamaz; değişiklik Product Owner change-control sürecine tabidir.
