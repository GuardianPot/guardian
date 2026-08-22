# ADR 0015: Reproducible artifacts and provenance

- Status: Accepted
- Decision refs: RE-07, SEC-01, SEC-02
- Source: Step 4 system architecture and technology decisions

## Decision

Build reproducible-ish Go binaries, digest-addressed OCI images, Debian
appliance artifacts, SBOM/provenance, and signed release checksums. Base images
and external Actions are pinned where practical.

## Consequences

Artifact identity and supply-chain evidence remain reviewable. Release signing
authority stays outside agent environments.
