# P1-W4 device enrollment security review

- Review date: 2026-08-24
- Work package: P1-W4
- Decisions: CM-03, CM-04, CM-05, SA-02, SA-03, SA-05, SA-06, AUTH-06
- Owner contract: W4-C1-A through W4-C7-A
- Scope: durable enrollment, product device CA, Edge key custody, rotation,
  disable/revoke/re-enrollment, and the mTLS eligibility verifier

## Trust-boundary conclusion

The Control Plane creates a UUIDv7 stable device ID and binds a 32-random-byte
Base64URL token to exactly one environment and device. PostgreSQL receives only
the SHA-256 token hash, 15-minute expiry, consume/revoke timestamps, and
non-secret metadata. Token selection and `consumed_at` transition share row
locks and one transaction; concurrent contenders produce exactly one issued
certificate and a bounded replay denial.

The bootstrap endpoint is TLS-only and accepts the token only as one bounded
Bearer header. Body size and shape are bounded, unknown/trailing JSON is
rejected, errors are generic, and responses are `no-store`. Persistent hashes
of source and token scopes drive a five-minute failure window and bounded
one-to-fifteen-minute backoff. Denials are audited without the presented token
or raw source value.

Operator endpoints accept only the approved browser-session seam plus CSRF,
Origin, and environment scope. The default authorizer denies before request
body processing or service access. P1-W2 must provide the real implementation;
P1-W4 does not create a parallel session or role model.

## Key and certificate conclusion

Edge generates its ECDSA P-256 private key locally and sends only a signed CSR.
The Control Plane rejects malformed CSRs, unsupported key/signature algorithms,
extensions, and failed CSR signatures. Issued client-auth certificates have a
random 128-bit serial, one `urn:guardian:device:<uuid>` URI SAN, a 30-day
validity window, and no DNS/IP/email SAN or server-auth use.

The local SecretStore loads a required regular, non-symlink, owner-only,
exactly 32-byte file and uses AES-256-GCM with a random nonce and purpose-bound
associated data. PostgreSQL stores the public CA certificate and authenticated
private-key envelope, never a plaintext CA private key. `serve` does not create
or replace a missing CA. No configured enrollment bundle leaves the device API
unavailable; partial, missing, invalid, tampered, or wrong-key bundles fail
closed at startup.

Edge verifies the returned chain, client-auth usage, stable URI SAN, serial,
validity, and public-key match before installation. Certificate and private key
share an owner-only non-symlink directory; the key is owner-only. A durable
journal, staged files, old-pair fallback, file synchronization, and startup
recovery prevent a rotation crash from selecting a mismatched pair. Unjournaled
stages are never trusted.

## Rotation and revocation conclusion

Automatic rotation is scheduled deterministically from the non-secret
certificate fingerprint within the final ten days, leaving at least one hour
before expiry. Failure retry grows from one to fifteen minutes and logs no
credential material. The database locks the active serial and atomically
inserts the replacement, marks the prior serial revoked, and appends the
rotation audit event.

The product verifier first validates the X.509 chain and stable URI identity,
then checks the current device and serial state in PostgreSQL. Disabled devices,
revoked devices, revoked serials, stale serials, and out-of-window certificates
fail closed. Explicit re-enrollment keeps the stable device record but
permanently retires every prior active certificate and unused token before
issuing a new one-time token. No old serial or token transition returns to an
active state.

P1-W5 must invoke this verifier before each new management session. P1-W4 does
not claim live channel or connected/healthy evidence; those remain P1-W5,
P1-W9, and P1-W11 gates under W4-C1-A.

## Required evidence

The final candidate must pass:

- unit tests for SecretStore wrong-key/tamper behavior, CSR policy, token
  handling, default-deny APIs, TLS-only enrollment, jitter, retry, and Edge
  install recovery/permissions;
- PostgreSQL 18 integration for one-winner concurrent replay, hashed token at
  rest, encrypted CA material, durable audit, disable, stable re-enrollment,
  rotation-window enforcement, and permanent serial revocation;
- a real TLS Edge enrollment exchange proving the private key is absent from
  the request and the returned identity is validated before installation;
- Go race/vet/tests, migration/generated checks, Protobuf/OpenAPI/schema checks,
  repository secret scan, dependency verification, and the complete validation
  workflow.

## Residual risks and later gates

- The local SecretStore is development-only. Hosted KMS/HSM selection, key
  rotation, backup policy, and disaster recovery require a separate
  owner-reviewed package.
- Server TLS certificate issuance and operating-system trust distribution are
  deployment responsibilities; the product device CA is not reused as a
  general server CA.
- PostgreSQL remains the online revocation authority. Availability, backup,
  privileged administrator control, and multi-region consistency inherit the
  database deployment.
- Source-address throttling is defense in depth and may aggregate clients behind
  NAT. P1-W5 can add channel-specific controls without weakening token scope.
- Initial enrollment interrupted after server-side consumption but before any
  recoverable Edge pair requires explicit re-enrollment. Private-key transfer
  or token reuse is intentionally unavailable.
- P1-W2 authorization, P1-W3 environment ownership, P1-W5 live channel,
  P1-W9 health projection, and P1-W11 browser evidence remain outside P1-W4.

No P1-W4 result may be represented as production KMS readiness, live device
connectivity, or browser acceptance evidence.
