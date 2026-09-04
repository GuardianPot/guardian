# Agent context map

This map keeps agent context bounded. Read the root `AGENTS.md`, the current
work package, and only the required sources listed here. Approved planning
documents remain authoritative if a summary conflicts with them.

## Global context

| Concern | Required source |
|---|---|
| Product scope and acceptance | `0-planning-documents/step-5-mvp-scope-and-acceptance-criteria.md` |
| Architecture and technology | `0-planning-documents/step-4-system-architecture-and-technology-decisions.md` |
| Phase sequencing | `0-planning-documents/step-6-roadmap/00_Step_6_Roadmap_Master.md` |
| Agent work-package governance | `0-planning-documents/step-7-repository-and-ai-agent-workflow/04_Agent_Context_and_Work_Package_Protocol.md` |
| GitHub execution model | `0-planning-documents/step-7-repository-and-ai-agent-workflow/08_GitHub_Issues_Projects_and_Execution_Model.md` |
| Architecture changes | `docs/adr/README.md` and `docs/change-proposals/README.md` |

## Phase 1 packages

| Package | Required context | Primary accepted ADRs |
|---|---|---|
| P1-G0 | Step 7 sections CTX/WP/PM; Phase 1 roadmap | ADR 0001 |
| P1-G1 | P1-G1; CP-02; WP-09/10 | ADR 0002, 0007 |
| P1-W1 | Phase 1 P1-W1; CP-01..07; DA-01/02/07 | ADR 0002, 0003, 0007, 0014 |
| P1-W2 | Phase 1 P1-W2; IA-01..06; SA-11/13/14; AUTH-01..06 | ADR 0003, 0006, 0007 |
| P1-W3 | Phase 1 P1-W3; IA-01; CP-02/04; AC-ON-003 | ADR 0002, 0006, 0007 |
| P1-W4 | Phase 1 P1-W4; CM-03..05; SA-02..06; AC-ON-001/002 and AC-SEC-005/006 | ADR 0005, 0013, 0017 |
| P1-W5 | Phase 1 P1-W5; CM-01..10; SA-03/04/14 | ADR 0005, 0017 |
| P1-W6 | Phase 1 P1-W6; EN-05/07/08; CM-07/08 | ADR 0005, 0008, 0017 |
| P1-W7 | Phase 1 P1-W7; EN-01..12 | ADR 0004, 0008, 0014 |
| P1-W8 | Phase 1 P1-W8; EN-02..04; SA-10; SEC-04; change proposal 0002 | ADR 0004, 0009, 0010 |
| P1-W9 | Phase 1 P1-W9; OB-01..06; failure-mode decisions | ADR 0014 |
| P1-W10 | Phase 1 P1-W10; SA-13; AUTH-06 | ADR 0002, 0007 |
| P1-W11 | Phase 1 P1-W11; DT-07; CP-04/08; TS-02..05; SA-11 | ADR 0002, 0006 |

## Web Console extended packages

`WCX-nn` packages live in
`docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/`.
Their governing decisions are closed in `00-master-decision-record.md`, which
every `WCX` package requires as context in addition to the rows below. All of
them carry `status: draft` until the Product Owner promotes them.

| Package | Phase | Required context | Primary ADRs |
|---|---|---|---|
| WCX-01 | 2 | WCX-01; WCX-000; CP-02; IA-06; TS-02/04 | ADR 0002, 0018 |
| WCX-02 | 2 | WCX-02; WCX-000; RE-10; CP-04; TS-06 | ADR 0006, 0018 |
| WCX-03 | 2 | WCX-03; WCX-000; CS-01/02; SRC-07 | ADR 0018 |
| WCX-04 | 2 | WCX-04; WCX-000; OPS-03; SEC-08 | ADR 0018 |
| WCX-05 | 2 | WCX-05; WCX-000; WCAG 2.2 AA target | ADR 0018 |
| WCX-06 | 2 | WCX-06; WCX-000; SEC-07/08; SA-11; RE-11/12; DATA-02..05 | ADR 0018 |
| WCX-07 | 2 | WCX-07; WCX-000; PERF-05/07/08; TS-05 | ADR 0018 |
| WCX-08 | 2 | WCX-08; WCX-000; SRC-07; EV-06; TP-03 | ADR 0018 |
| WCX-09 | 2 | WCX-09; WCX-000; change proposal 0003; IA-04/05/06; AUTH-01/02/06; SEC-06 | ADR 0006 |
| WCX-10 | 2 | WCX-10; WCX-000; UX-01/07; ENV-01/02 | ADR 0018 |
| WCX-11 | 2 | WCX-11; WCX-000; change proposal 0004; UX-06; ON-05; DC-11/12; OPS-02 | ADR 0006, 0011 |
| WCX-12 | 3 | WCX-12; WCX-000; UX-01/02/07; CS-01/02/09; SRC-01/07; PERF-05/07 | ADR 0018 |
| WCX-13 | 3 | WCX-13; WCX-000; UX-03/04/05; COR-04/05; EV-01/02/04; DATA-02..05; AUTH-06 | ADR 0018 |
| WCX-14 | 4 | WCX-14; WCX-000; AIM-01/08/09/13; RG-01/02/03/05; NT-01/05/06; CS-06/07/08; SEC-09 | ADR 0012 |
| WCX-15 | 5 | WCX-15; WCX-000; SA-11; SEC-08/09; PERF-07/08 | ADR 0018 |
| WCX-16 | 2 | WCX-16; WCX-000; ON-01..06; OPS-02; UX-07; AC-ON-005/006 | ADR 0010, 0011 |
| WCX-20 | 2 | WCX-20; WCX-000; DC-09/10; CS-03; DATA-02; SEC-02; W11-C3-A | ADR 0011 |
| WCX-21 | 2 | WCX-21; WCX-000; DATA-01/03..06; CS-06; EV-05; AUTH-06 | ADR 0007 |
| WCX-17 | 3 | WCX-17; WCX-000; CS-05..09; COR-06/07; AC-CF-003/004; AC-INC-004 | ADR 0007 |
| WCX-18 | 4 | WCX-18; WCX-000; NT-02..07; SEC-02; AUTH-06; AC-NT-002/003 | ADR 0006 |
| WCX-19 | 5 | WCX-19; WCX-000; UP-01..04; UX-07; OPS-01/03/04; AIM-08; AC-UP-001..004 | ADR 0015 |

## Phase gates

- Phase 0: `docs/phase-gates/phase-0.md`
- Phase 1: `docs/phase-gates/phase-1.md`

An agent must stop if a package requires context outside this map that changes
product behavior, architecture, security boundaries, privileges, or an
acceptance criterion. Add context through owner-reviewed package revision or a
change proposal; do not silently widen scope.
