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

`NOT STARTED`
