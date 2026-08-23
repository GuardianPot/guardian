# Control Plane development deployment

This directory owns the PostgreSQL 18 development dependency for P1-W1. The
image is pinned by tag and digest. Compose binds PostgreSQL only to loopback and
requires the caller to supply a development password at runtime; no credential
is committed.

The repeatable integration harness creates a unique Compose project, a random
password, and disposable test databases:

```bash
task control-plane:integration
```

The harness removes only its uniquely named containers, network, and volume.
The persistent manual-development workflow and destructive reset procedure are
documented in `docs/runbooks/control-plane/development.md`.

The Control Plane runtime image is a digest-pinned Distroless Debian 13 static
nonroot image. It contains no shell or package manager. Production diagnostics
must use structured logs, health endpoints, and approved external or ephemeral
debug tooling; a debug image must never replace the reviewed runtime artifact.
