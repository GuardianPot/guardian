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

P1-W5 adds a dedicated TLS 1.3 gRPC listener. The application profile listens
inside the nonroot container on `8443`; development may publish that port on
loopback, while production ingress maps the dedicated device hostname to TCP
443. Configure `GUARDIAN_DEVICE_CHANNEL_ADDRESS` separately from the Web/API
listener. Both listeners use the configured server certificate/key, while
device client trust comes from the P1-W4 product CA stored by the Control Plane.

The optional Compose `application` profile demonstrates the port boundary. It
requires mounted development master-key and TLS files and an already migrated
database; it never creates or commits those credentials.

The Control Plane runtime image is a digest-pinned Distroless Debian 13 static
nonroot image. It contains no shell or package manager. Production diagnostics
must use structured logs, health endpoints, and approved external or ephemeral
debug tooling; a debug image must never replace the reviewed runtime artifact.
