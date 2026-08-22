# ADR 0008: SQLite WAL edge durability

- Status: Accepted
- Decision refs: EN-05, DA-09, SP-06
- Source: Step 4 system architecture and technology decisions

## Decision

Use SQLite with WAL mode for local Edge durable state and replay. The approved
development driver is `modernc.org/sqlite`; crash and replay behavior are
validated in P0-W8.

## Consequences

Edge operation can continue through control-plane connectivity loss while
preserving stable event IDs and replay evidence.
