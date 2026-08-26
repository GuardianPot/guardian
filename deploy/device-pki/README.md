# Guardian device PKI

P1-W4 replaces the Phase 0 in-memory proof with the product-owned durable
enrollment boundary. The Control Plane stores public CA/certificate metadata
and an AES-256-GCM envelope for the CA private key in PostgreSQL. The 32-byte
envelope master key is a mounted owner-only file outside the repository and
database. Edge private keys are generated and retained on the Edge host.

The local SecretStore is a development backend, not a production KMS. A
production deployment must replace that backend through the same interface
after an owner-reviewed package; it must not copy the development master-key
file into an image, repository, backup bundle, or database.

Device enrollment is enabled only when all three runtime paths are configured:

- `GUARDIAN_MASTER_KEY_FILE`: regular, non-symlink, exactly 32 bytes, owner-only;
- `GUARDIAN_TLS_CERT_FILE`: Control Plane server certificate;
- `GUARDIAN_TLS_KEY_FILE`: matching Control Plane server private key.

No values means the device service remains unavailable. A partial bundle,
invalid file, missing encrypted CA, or authentication failure stops startup.
The `serve` command never creates or replaces a CA. Run the explicit,
single-use `init-device-ca` command after migrations and before the first
enrollment-enabled start.

The HTTPS listener requires TLS 1.3. Enrollment uses a one-time bearer token;
rotation uses a currently active product mTLS certificate. The server
certificate must chain to trust installed in the Edge operating-system trust
store. Product device CA material is separate from server TLS identity.

See [the development enrollment runbook](../../docs/runbooks/enrollment/development.md)
for exact initialization, enrollment, disable, re-enrollment, and recovery
steps. Run the disposable evidence fixture from the repository root with:

```text
task enrollment:integration
```

The retained `task device:pki` command is the accepted P0-W9 in-memory fixture;
it is not evidence of the P1-W4 durable product path.
