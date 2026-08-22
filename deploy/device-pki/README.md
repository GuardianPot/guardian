# Device PKI fixture

P0-W9 validates enrollment, replay resistance, mTLS rotation, and revocation
with an in-memory P-256 test CA. No production CA key, certificate, or trust
material is stored in the repository.

Run the disposable fixture from the repository root:

```powershell
task device:pki
```

The fixture covers nonce-bound CSR proof, one-time challenge consumption,
atomic certificate rotation, revocation, and a TLS 1.3 handshake that requires
an active device certificate. Test state is held in memory and Go test cleanup
owns the temporary test lifecycle.
