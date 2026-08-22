# ADR 0012: Provider-abstracted AI boundary

- Status: Accepted
- Decision refs: AI-01, AI-02, AI-07, AI-09
- Source: Step 4 system architecture and technology decisions

## Decision

Keep OpenAI and Anthropic behind a common structured provider interface. AI is
an asynchronous explanation/guidance enhancement; it has no automatic network,
containment, or production action tools in the initial product.

## Consequences

Provider changes remain testable and incidents do not depend on AI availability
for deterministic creation.
