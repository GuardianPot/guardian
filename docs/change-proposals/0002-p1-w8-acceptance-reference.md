# Change proposal 0002: correct P1-W8 acceptance reference

- Status: APPROVED
- Owner decision: `@sinanganiz` approved option A as D7-A on 2026-08-22.
- Affected decision IDs: `SEC-04`
- Affected acceptance criteria: `AC-SEC-004`
- Work package: P1-W8

## Problem and context

The Phase 1 roadmap assigned `AC-SEC-004` to the privileged-helper package.
The approved acceptance catalog defines that criterion as rejection of an
unsigned or tampered OCI decoy artifact. Cross-phase traceability assigns its
primary validation to Phase 2 and full regression to Phase 5. P1-W8 instead
implements the typed privileged-helper boundary approved by `SEC-04`.

## Options

1. Correct P1-W8 to `SEC-04` plus its Phase 1 privileged-helper security tests,
   leaving `AC-SEC-004` in Phase 2 primary validation and Phase 5 regression.
2. Pull OCI artifact verification into P1-W8, expanding Phase 1 into Phase 2
   supply-chain and decoy-runtime scope.

## Recommendation

Option 1. It restores traceability without changing product behavior,
architecture, or the intended phase boundary.

## Impact

- Product scope: no change.
- Architecture: no change.
- Contracts/data: no change.
- Security/trust boundary: preserves the typed privileged-helper boundary and
  keeps artifact verification in its approved lifecycle.
- Operations/release: no change.

## Rollout and failure behavior

Update the Phase 1 roadmap, context map, P1-W8 metadata, and policy checks in
the P1-G0 planning PR. No runtime migration or compatibility path is required.

## Owner decision record

- Decision: OPTION A — APPROVED
- Decided by: `@sinanganiz`
- Date: 2026-08-22
- Rationale: preserve the approved phase boundary and bind P1-W8 to the
  security decision and tests it actually implements.
