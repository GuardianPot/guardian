# P1-W2 local authentication security review

- Review date: 2026-08-26
- Work package: P1-W2
- Decisions: IA-01 through IA-06, SA-11, SA-13, SA-14, AUTH-01,
  AUTH-02, AUTH-03, AUTH-06
- Owner contract: W2-C1-A through W2-C7-A
- Scope: sole-owner bootstrap, password/TOTP/recovery authentication,
  server-side sessions, CSRF/origin, throttling, and authentication audit

## Trust-boundary conclusion

There is no default password or implicit owner. A CLI creates 32 random bytes,
prints the Base64URL token once, and persists only SHA-256 with a 15-minute
expiry. Creating a new token revokes an older live token. The exchange locks
the matching row, checks that no user exists, consumes the token, creates the
sole `pending_mfa` owner, and appends audit in one transaction. Concurrent
exchanges have exactly one winner; replay, expiry, revocation, and an existing
owner fail closed. Bootstrap returns the TOTP URI and ten recovery codes once,
creates no session, and requires the first password-plus-TOTP login to activate
the owner.

Password records use pinned `golang.org/x/crypto/argon2` Argon2id with strict
PHC parsing and a floor of 64 MiB, three passes, four lanes, 16 random salt
bytes, and a 32-byte tag. New passwords are NFC-normalized, bounded by Unicode
code points and bytes, checked against local common/context values, and have no
composition rule. Unknown-account authentication uses the same Argon2id dummy
path. Unknown account names share one durable account scope, preventing an
attacker from growing one throttle row per arbitrary username.

The TOTP seed is 256 random bits sealed by the approved AES-256-GCM
SecretStore with a user-bound context. PostgreSQL stores only the authenticated
envelope. Verification is HMAC-SHA-256, six digits, 30-second steps, plus/minus
one step, with the accepted counter advanced under a user-row lock. Concurrent
or later replay cannot pass. Recovery codes are ten independent 128-bit
values; only SHA-256 hashes persist, atomic consumption is single-use, and a
recovery login revokes existing sessions before creating its replacement.

## Session and web-boundary conclusion

Every login and security-sensitive password/recovery transition generates a
new UUIDv7 session identity plus independent 32-byte random session and CSRF
secrets. PostgreSQL stores only SHA-256 hashes. The browser receives the
session only as `__Host-guardian_session` with Secure, HttpOnly,
SameSite=Strict, Path `/`, and no Domain. It receives the synchronizer CSRF
token only in a no-store JSON response; P1-W11 owns proof that the real Web
Console retains it in memory and never uses browser storage.

Reads validate the opaque cookie against a locked server-side record and
enforce active user, revocation, 15-minute idle, and eight-hour absolute
expiry. Mutations additionally require an exact HTTPS configured Origin and a
constant-time CSRF-hash match. Logout and targeted revocation are durable.
Password change validates the current password, revokes every session, and
issues one replacement in the same transaction. Recovery has the same
all-session revocation property.

Auth endpoints require a real TLS request, reject unknown/trailing or oversized
JSON, return generic no-store denials, and install restrictive CSP,
Referrer-Policy, and content-type headers. Startup requires an absolute clean
owner-only SecretStore key file and exact HTTPS public origin. Forwarded
headers are not trusted as a TLS or source-identity substitute.

## Abuse, audit, and storage conclusion

Account and source hashes have a durable five-failure/five-minute window with
exponential delay capped at 15 minutes. Unknown names share a bounded account
scope; source scopes remain independent. A successful login clears its account
counter but does not erase source abuse history. PostgreSQL restart does not
clear a block.

Bootstrap, login success/failure, first MFA activation, recovery use, logout,
password change, and session revocation append the closed P1-W10 actions inside
the same transaction as their mutation where atomicity is required. Audit
snapshots contain only role/status/username and expiry metadata. Integration
scans audit snapshots and database secret shapes for submitted passwords,
bootstrap/session/CSRF/recovery values, and the provisioning secret.

Schema constraints enforce one owner, UUIDv7 identities, strict states,
fixed-size hashes, bounded encrypted TOTP material, legal revocation reasons,
and timestamp ordering. Development migrations are forward-only; recovery may
reset and reseed development data, never silently weaken or recreate auth
material.

## Required evidence

The final candidate must pass:

- Argon2id floor/malformed-record/runtime, password normalization/blocklist,
  TOTP drift/replay, opaque-secret, and SecretStore-binding unit tests;
- PostgreSQL 18 expiry and one-winner bootstrap, first-MFA activation,
  recovery single-use, session rotation/revocation/idle/absolute expiry,
  restart-persistent throttling, atomic audit, and redaction tests;
- real TLS HTTP evidence for the exact session cookie, no-store/security
  headers, CSRF, exact Origin, generic denial, and cookie clearing;
- CLI bootstrap output bounds, generated SQL parity, Go vet/tests, OpenAPI,
  dependency/license/security policy, secret scan, and the complete Linux
  validation workflow.

## Residual risks and later gates

- TOTP is phishable and depends on usable clock synchronization. Passkeys,
  WebAuthn, enterprise identity, and step-up policies require later approved
  packages; Phase 1 does not claim phishing-resistant authentication.
- The local SecretStore is development-only. Hosted KMS/HSM, master-key
  rotation, backup rollback protection, production disaster recovery, and
  database-administrator controls remain later owner-reviewed work.
- Source identity is the direct peer address. A trusted reverse-proxy contract
  is absent; forwarded headers are deliberately ignored. NAT may aggregate
  users, and distributed sources remain a general rate-limit limitation.
- The direct TLS listener currently shares the P1-W4 device-CA configuration.
  Server-certificate issuance and operating-system trust remain deployment
  responsibilities; the product device CA is not a general server CA.
- Authentication failure audit volume and throttle retention need production
  capacity/retention policy. Phase 1 provides no audit deletion workflow.
- A database backup restored to an older point can also restore older session,
  recovery, and TOTP-counter state. Production rollback protection is outside
  this development package.
- P1-W11 must prove actual Web Console cookie behavior, memory-only CSRF,
  absence of localStorage/sessionStorage credentials, accessible failure UX,
  and the complete bootstrap/login/recovery journey. P1-W2 does not claim that
  evidence through an HTTP test double.

No P1-W2 result may be represented as OIDC/SSO, phishing-resistant MFA,
production KMS/disaster-recovery readiness, multi-owner RBAC, or browser
acceptance evidence.
