# ADR 0014: OpenTelemetry instrumentation

- Status: Accepted
- Decision refs: OB-01, OB-02
- Source: Step 4 system architecture and technology decisions

## Decision

Use OpenTelemetry-first instrumentation for traces, metrics, and logs while
keeping observability backend choice separate from product correctness.

## Consequences

Trace/context identity can cross Edge ingestion, evidence, incident, and AI
boundaries without making a specific Grafana or vendor stack a product
dependency.
