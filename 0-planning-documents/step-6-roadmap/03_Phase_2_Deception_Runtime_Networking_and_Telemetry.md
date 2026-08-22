# Phase 2 — Deception Runtime, Networking & Telemetry
## Final Approved Implementation Specification

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Faz amacı

Edge'i gerçek bir deception sensor haline getirmek.

Phase 2 sonunda:
- routed/secondary IP decoy placement çalışır,
- containerd lifecycle productionized,
- SSH/HTTP/PostgreSQL/SMB decoy families deploy edilir,
- synthetic credential marker desteklenir,
- canonical events/evidence Edge'de normalize edilir,
- local spool ve central ingestion çalışır,
- coverage/decoy health gerçek network path ile doğrulanır.

# 2. Upstream dependencies

- Phase 1 desired state
- privileged helper
- device channel
- health model
- Phase 0 networking/containerd/Cowrie spikes

# 3. Workstreams

## P2-W1 — Routed presence driver

Implement:
- address reservation
- conflict pre-check
- add/remove/reconcile secondary IP
- route/interface binding
- cleanup
- desired vs observed status

Acceptance:
- AC-ON-004
- reboot/reconcile restores intended addresses,
- failed deploy does not leave orphan IP.

## P2-W2 — nftables egress policy

Default:
- decoy→production arbitrary egress deny
- decoy→internet arbitrary egress deny
- required local/collector paths explicit

Acceptance:
- AC-SEC-003
- policy persists/reconciles,
- decoy cannot bypass by changing container process.

## P2-W3 — containerd production runtime manager

Implement:
- signed/digest-capable image references (signature enforcement final Phase 5)
- create/start/stop/remove
- cgroup limits
- health probes
- namespace cleanup
- restart policy
- runtime state reconciliation

Acceptance:
- crash/restart tests,
- AC-SEC-002,
- no runtime socket mount.

## P2-W4 — Decoy manifest schema

Fields:
- type/version
- interaction level
- ports
- network mode requirements
- privilege/capability requirements
- telemetry adapter
- health probe
- resource limits
- egress requirements
- content/persona metadata

Acceptance:
- unsupported privilege request rejected,
- manifest version mismatch explicit.

## P2-W5 — SSH/Cowrie pack

Implement production wrapper:
- medium interaction
- login capture
- username/auth result
- command/session capture
- synthetic credential recognition
- bounded transcripts
- health
- canonical telemetry

Acceptance:
- AC-SSH-001..005
- hostile content stays data
- decoy has no production secret.

## P2-W6 — HTTP/Admin pack

Implement curated fake admin persona:
- request capture
- selected headers
- bounded body
- fake login form
- no arbitrary server-side code execution
- XSS-safe evidence rendering contract

Acceptance: AC-HTTP-001..004.

## P2-W7 — PostgreSQL pack

Implement:
- handshake
- database/user attempt
- auth result
- synthetic credential marker
- bounded query/session detail if emulated

Acceptance: AC-PG-001..003.

## P2-W8 — SMB low-interaction pack

Implement:
- Windows-flavored persona
- SMB probe/enumeration telemetry
- clear UI/metadata that it is emulated, not real Windows host

Acceptance: AC-SMB-001/002.

## P2-W9 — Synthetic credential domain

Implement:
- generate/define decoy-only credential
- associate with decoy
- secret not granting production access
- marker/hash/reference handling
- manual pilot placement workflow
- trigger lifecycle

Acceptance:
- SSH/PG valid decoy credential creates strong evidence,
- normal UI does not reveal reusable secret after intended workflow where possible.

## P2-W10 — Canonical event/evidence envelope

Implement full Step 5 fields:
- event ID/time
- observed/ingested time
- env/edge/decoy
- runtime version
- source/destination
- protocol/action
- session/auth
- raw ref
- schema
- sensitivity
- provenance

Acceptance: AC-EV-001..003.

## P2-W11 — Edge normalization adapters

Architecture:
- native typed UDS event path
- third-party log adapter path
- parser limits
- malformed event quarantine/drop reason
- rate/burst dedup pre-processing

Acceptance:
- AC-SEC-008,
- oversized malformed input cannot crash Edge.

## P2-W12 — Local spool + central ingest

Implement:
- append before send
- ack state
- retry/backoff
- at-least-once
- central unique constraint/idempotency
- queue metrics

Acceptance:
- AC-HL-001/002,
- AC-EV-001,
- Control disconnect and reconnect replay.

## P2-W13 — Blob/quarantine plumbing

Implement object abstraction for:
- long transcripts
- optional uploaded files
- raw bounded payloads

Security:
- no inline execution/render
- content type untrusted
- hash/size metadata

## P2-W14 — Functional decoy health & coverage

Healthy requires:
- runtime healthy
- IP applied
- port responds
- telemetry heartbeat/probe path
- policy applied
- expected digest/version

Acceptance:
- AC-ON-005/006,
- kill process / remove IP / break telemetry each yields distinct degraded state.

## P2-W15 — Decoy management UI

Web:
- list
- type/persona
- IP/zone
- health
- version
- last interaction
- deploy/remove
- enable/disable
- current desired/observed status

No incident intelligence yet.

# 4. Security gates

Before Phase 2 exit:
- decoys cannot reach device key
- decoys cannot reach runtime socket
- no production secrets
- egress default deny
- XSS fixture safe
- parser limits
- resource quotas

# 5. Exit gate

- [ ] routed placement passes
- [ ] SSH/HTTP/Postgres/SMB deployed through same lifecycle
- [ ] synthetic credential works
- [ ] canonical event/evidence pipeline durable
- [ ] offline replay works
- [ ] coverage functional health works
- [ ] all Phase 2 Step5 ACs pass
- [ ] 4 decoy families shown in Web Console
- [ ] security isolation tests pass

# 6. Product state after phase

We now have a **working high-signal deception sensor**, but still mostly event/evidence oriented. Phase 3 turns evidence into the actual product: incidents and attacker journey.

---

## Final Phase Status

- **Phase:** Phase 2 — Deception Runtime, Networking & Telemetry
- **Status:** APPROVED / FINAL
- **Owner decision:** Tüm bu faz kapsamı, sequencing, dependencies, acceptance criteria ve exit gate'leri onaylandı.
- **Change policy:** Bu fazın kapsamı implementation sırasında sessizce genişletilemez/daraltılamaz; değişiklik Product Owner change-control sürecine tabidir.
