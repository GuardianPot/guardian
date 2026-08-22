# ADR 0007: PostgreSQL central persistence

- Status: Accepted
- Decision refs: DA-01, DA-02, RE-08
- Source: Step 4 system architecture and technology decisions

## Decision

Use PostgreSQL as the central relational store with explicit SQL and generated,
type-safe access in the `pgx`/`sqlc` class. Use human-reviewable SQL migrations;
do not use ORM auto-migration.

## Consequences

Evidence, incidents, audit state, and product records use a durable relational
model with visible query and migration behavior.
