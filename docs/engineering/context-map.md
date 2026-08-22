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

## Phase gates

- Phase 0: `docs/phase-gates/phase-0.md`
- Phase 1: `docs/phase-gates/phase-1.md`

An agent must stop if a package requires context outside this map that changes
product behavior, architecture, security boundaries, privileges, or an
acceptance criterion. Add context through owner-reviewed package revision or a
change proposal; do not silently widen scope.
