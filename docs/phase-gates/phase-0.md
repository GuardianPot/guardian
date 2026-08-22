# Phase 0 gate

Phase 0 is not complete when a spike works once. It is complete only when
every approved Phase 0 work package has acceptance evidence, unresolved
blockers are either resolved or explicitly escalated, and the Product Owner
approves the gate.

## Required evidence

- P0-W1 through P0-W10 issue and PR links
- reproducible commands and clean-checkout results
- failure-injection or restart evidence where applicable
- security and license notes for external components
- ADR conclusions for architecture-affecting spikes
- no production secrets, signing keys, or unauthorized target data

## Gate authority

CI can report technical conditions. It cannot mark Phase 0 `APPROVED` or
`CLOSED`; that decision belongs to the Product Owner.

## Current status

`IN REVIEW — NOT APPROVED`

## Evidence audit — 2026-08-22

The authorized execution batch has completed and the technical evidence is
recorded for the following packages:

| Work package | Issue | PR | Evidence state |
|---|---:|---:|---|
| P0-W1 | [#1](https://github.com/GuardianPot/guardian/issues/1) | [#11](https://github.com/GuardianPot/guardian/pull/11) | Accepted |
| P0-W2 | [#2](https://github.com/GuardianPot/guardian/issues/2) | [#12](https://github.com/GuardianPot/guardian/pull/12) | Accepted |
| P0-W3 | [#3](https://github.com/GuardianPot/guardian/issues/3) | [#13](https://github.com/GuardianPot/guardian/pull/13) | Accepted |
| P0-W4 | [#4](https://github.com/GuardianPot/guardian/issues/4) | [#14](https://github.com/GuardianPot/guardian/pull/14) | Accepted |
| P0-W5 | [#5](https://github.com/GuardianPot/guardian/issues/5) | [#15](https://github.com/GuardianPot/guardian/pull/15) | Accepted |
| P0-W6 | [#6](https://github.com/GuardianPot/guardian/issues/6) | [#16](https://github.com/GuardianPot/guardian/pull/16) | Accepted |
| P0-W7 | [#7](https://github.com/GuardianPot/guardian/issues/7) | [#21](https://github.com/GuardianPot/guardian/pull/21) | Accepted |
| P0-W8 | [#8](https://github.com/GuardianPot/guardian/issues/8) | [#17](https://github.com/GuardianPot/guardian/pull/17) | Accepted |
| P0-W9 | [#9](https://github.com/GuardianPot/guardian/issues/9) | [#18](https://github.com/GuardianPot/guardian/pull/18) | Accepted |
| P0-W10 | [#10](https://github.com/GuardianPot/guardian/issues/10) | [#19](https://github.com/GuardianPot/guardian/pull/19) | Accepted |

Reproducible evidence includes `task validate`, the isolated lab and routed
secondary-IP fixtures, `task decoy:lifecycle`, `task edge:wal`, `task
device:pki`, `task cowrie:adapter`, `bash tools/cowrie-fixture.sh`, and `task
contracts`. Security reviews, test-only CA boundaries, runtime socket
isolation, egress denial, provenance/license records, and no-secret checks are
committed under `security/` and the relevant `deploy/` directories.

All P0-W1 through P0-W10 evidence is now recorded. This audit deliberately
does not mark the Phase 0 gate `APPROVED` or `CLOSED`; that final decision
remains with the Product Owner.
