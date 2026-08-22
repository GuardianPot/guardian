# Step 6 — Cross-Phase Dependency, Traceability & Release Gates
## FINAL

> **Statü:** APPROVED / FINAL  
> **Karar otoritesi:** Product Owner  
> **Development-time estimate:** Kullanılmaz. Fazlar bağımlılık ve capability maturity sırasıdır; takvim değildir.  
> **Bağlayıcı girdiler:** Step 2 Product Definition, Step 3 Technical Feasibility, Step 4 System Architecture & Technology Decisions, Step 5 MVP Scope & Acceptance Criteria.  
> **Governance:** Bu roadmap hiçbir önceki APPROVED kararı sessizce değiştiremez. Çelişki halinde ilgili change-control süreci açılır.


# 1. Amaç

Fazların “sıralı yapılacaklar listesi”ne dönüşmesini önlemek. Bu belge:
- hard dependencies,
- parallelizable work,
- Step 5 acceptance criteria ownership,
- blocker spikes,
- release gates

için source of truth onaylanmış baseline'dır.

# 2. Hard dependency matrix

| Capability | Depends on | First phase |
|---|---|---|
| Edge enrollment | PKI contracts/lab | P1 |
| Desired-state decoy deployment | device channel + reconciler | P2 |
| Decoy canonical evidence | runtime + contracts | P2 |
| Incident correlation | canonical evidence | P3 |
| Attacker journey | incidents + correlation | P3 |
| AI explanation | stable evidence IDs/incidents | P4 |
| Guidance | incident context + curated actions | P4 |
| Notifications | incident semantics | P4 |
| Signed update | stable artifacts/runtime lifecycle | P5 |
| Pilot release | P0–P5 gates | P5 |
| Multi-edge | stable device/coverage model | P6 |
| Endpoint agent | device lifecycle + identity model | P8 |
| High interaction | containment worker | P9 |
| Fake network | high interaction + coherent graph | P10 |
| MSSP | stable single-org product + demand | P11 |

# 3. MVP Step 5 AC ownership

## Phase 1 primary ACs
- AC-ON-001
- AC-ON-002
- AC-ON-003
- AC-SEC-005
- AC-SEC-006

## Phase 2 primary ACs
- AC-ON-004
- AC-ON-005
- AC-ON-006
- AC-SSH-001..005
- AC-HTTP-001..004
- AC-PG-001..003
- AC-SMB-001..002
- AC-EV-001..003
- AC-HL-001/002
- AC-SEC-001..004
- AC-SEC-007/008

## Phase 3 primary ACs
- AC-INC-001..004
- AC-CF-001..005
- AC-UX-001..005

## Phase 4 primary ACs
- AC-AI-001..007
- AC-RG-001..003
- AC-NT-001..004

## Phase 5 owns final validation
- AC-HL-001..005 full failure matrix
- AC-SEC-001..009 full regression
- AC-UP-001..004
- all end-to-end composition
- quantitative gates.

# 4. Architecture spike placement

| Spike | Phase | Blocking meaning |
|---|---|---|
| SP-01 routed networking | P0 | no Phase 2 placement implementation without proof |
| SP-02 containerd | P0 | no decoy runtime baseline without proof |
| SP-03 Cowrie | P0 | SSH design must be resolved before P2 |
| SP-04 OpenCanary | P7/optional P2 | only blocks selected pack depending on it |
| SP-05 Postgres benchmark | P5 | blocks final performance gate |
| SP-06 SQLite replay | P0 | blocks durable Edge pipeline |
| SP-07 device PKI | P0 | blocks production enrollment |
| SP-08 TUF/Cosign | P5 | blocks pilot release |
| SP-09 dual AI schema | P4 | blocks AI acceptance |
| SP-10 prompt injection | P4/P5 | blocks release |
| SP-11 high interaction | P9 | not MVP blocker |
| SP-12 advanced network modes | P6 | not MVP blocker |

# 5. Parallelizable work

Within P1:
- frontend auth shell can proceed alongside device channel after API contracts.
- health domain alongside enrollment.

Within P2:
- HTTP/PG/SMB packs parallel after manifest/telemetry contract.
- SSH integration parallel but Cowrie spike prerequisite.
- UI can consume mocked contract fixtures but merge requires real E2E.

Within P3:
- confidence/severity and incident UI parallel after finding/incident contracts.
- merge/split after evidence ownership stable.

Within P4:
- AI gateway/evals parallel with notifications.
- guidance catalog can begin before AI; AI composes only after catalog stable.

Within P5:
- performance, updates, security hardening, diagnostics can run parallel but final gate composes all.

# 6. Anti-dependency rules

Do not:
- start AI narrative before evidence IDs stable,
- build graph database because “journey is a graph,”
- add message broker before direct pipeline measured inadequate,
- build multi-edge before single-edge coverage semantics stable,
- add endpoint agent before signed update/device lifecycle mature,
- add high interaction on same Edge as shortcut,
- implement fake redirection before containment proven.

# 7. Phase change rules

A capability can move earlier/later only if:
1. reason documented,
2. upstream dependencies still satisfied,
3. Step 5 MVP scope unchanged or change-control approved,
4. security impact reviewed,
5. Product Owner approves phase change.

# 8. Release labels

- `internal`: before P5
- `pilot`: P5 exit
- `post-mvp`: P6/P7
- `limited-preview`: P8/P9/P10 candidates
- `platform`: P11 capabilities

These are approved roadmap labels, not dates/version numbers.

---

# 9. Post-MVP Phase Acceptance Namespaces

| Phase | Acceptance namespace | Count |
|---|---|---:|
| Phase 6 | `P6-AC-*` | 10 |
| Phase 7 | `P7-AC-*` | 8 |
| Phase 8 | `P8-AC-*` | 9 |
| Phase 9 | `P9-AC-*` | 10 |
| Phase 10 | `P10-AC-*` | 10 |
| Phase 11 | `P11-AC-*` | 10 |

These criteria are APPROVED as part of Step 6. They do not alter Step 5's approved MVP acceptance catalog.
