# Local authentication development runbook

This runbook operates the P1-W2 single-organization local-owner boundary. It
does not provide OIDC, email reset, multiple roles, production KMS, or browser
journey evidence. P1-W11 owns the real Web Console/localStorage/browser gate.

## Initialize secure runtime material

Apply migrations and create one 32-byte owner-only master key outside the
checkout. The same SecretStore boundary protects the TOTP seed and the P1-W4
development device CA; do not copy this key into PostgreSQL, an image, a log,
or the repository.

```sh
install -d -m 0700 /var/lib/guardian/secrets
umask 077
openssl rand 32 > /var/lib/guardian/secrets/control-plane-master.key
test "$(wc -c < /var/lib/guardian/secrets/control-plane-master.key)" -eq 32

export GUARDIAN_DATABASE_URL='postgres://guardian:<password>@127.0.0.1:5432/guardian'
export GUARDIAN_MASTER_KEY_FILE='/var/lib/guardian/secrets/control-plane-master.key'
export GUARDIAN_PUBLIC_ORIGIN='https://control-plane.guardian.test'
go -C apps/control-plane run ./cmd/control-plane migrate
```

`serve` fails closed unless the master-key path is absolute, clean, regular,
non-symlink, owner-only, and exactly 32 bytes, and unless the public origin is
an exact HTTPS origin without a path, query, fragment, or trailing slash.

The current direct TLS listener is shared with P1-W4 device enrollment. For a
direct-TLS development server, initialize the development device CA once and
configure a separate server certificate/key:

```sh
go -C apps/control-plane run ./cmd/control-plane init-device-ca
export GUARDIAN_TLS_CERT_FILE='/etc/guardian/tls/control-plane.crt'
export GUARDIAN_TLS_KEY_FILE='/etc/guardian/tls/control-plane.key'
export GUARDIAN_HTTP_ADDRESS='127.0.0.1:8443'
go -C apps/control-plane run ./cmd/control-plane serve
```

Authentication endpoints reject non-TLS requests. A reverse-proxy trust
contract is not part of P1-W2, so forwarded headers do not substitute for the
request's authenticated TLS state.

## Bootstrap the sole owner

Create a token only when no local user exists:

```sh
go -C apps/control-plane run ./cmd/control-plane create-bootstrap-token
```

The command prints one 43-character token and its expiry once. The database
stores only its SHA-256 hash. A new command invalidates an older unconsumed
token; every token expires after 15 minutes. Do not capture the output in shell
history, CI logs, chat, tickets, or a file.

Read the token without echo and exchange it over TLS. The literal secret does
not appear in the command history:

```sh
read -r -s bootstrap_token
curl --fail-with-body --request POST \
  --header "Authorization: Bearer ${bootstrap_token}" \
  --header 'Content-Type: application/json' \
  --data '{"username":"owner","password":"<new-password>"}' \
  'https://control-plane.guardian.test/v1/auth/bootstrap'
unset bootstrap_token
```

The successful response contains an `otpauth://` provisioning URI and exactly
ten recovery codes once. Enroll the URI in a TOTP authenticator and move the
recovery codes directly into an approved offline password-manager/recovery
store. Do not screenshot, print, log, or persist the response in application
storage. Bootstrap creates a `pending_mfa` owner and no session. The first
valid password-plus-TOTP login activates it.

If the exchange is interrupted after the owner is created but before MFA and
recovery material is secured, there is no bypass or recovery-code activation.
The approved development recovery is reset/reseed of the development database.

## Login and browser-session contract

`POST /v1/auth/login` accepts `username`, `password`, and exactly one of a
six-digit `totp_code` or a 22-character `recovery_code`. A successful response
sets only:

```text
__Host-guardian_session=<opaque>; Path=/; Secure; HttpOnly; SameSite=Strict
```

There is no `Domain` attribute. The response is `Cache-Control: no-store` and
returns an independent 43-character synchronizer `csrf_token`; the Web Console
must retain that token only in memory. It must never place the session value or
CSRF value in localStorage, sessionStorage, a URL, telemetry, or a log.

Every mutation requires the session cookie, `X-CSRF-Token`, and an `Origin`
header exactly equal to `GUARDIAN_PUBLIC_ORIGIN`. Reads require the session
cookie. Server-side expiry is 15 minutes idle and eight hours absolute.

Useful endpoints are:

- `GET /v1/auth/session` — current session;
- `GET /v1/auth/sessions` — at most 200 newest-first sessions;
- `DELETE /v1/auth/sessions/{sessionId}` — revoke one owned session;
- `POST /v1/auth/logout` — revoke current session and clear its cookie;
- `POST /v1/auth/password` — verify the current password, replace it, revoke
  every existing session, and issue one rotated replacement session.

A recovery-code login atomically consumes one code, revokes every existing
session, and issues a replacement. Codes never reactivate `pending_mfa` and
cannot be reused. There is no email reset or operator override in Phase 1.

## Password, MFA, throttling, and recovery

New passwords are NFC-normalized, 12–128 Unicode code points, at most 1024
UTF-8 bytes, and checked against the local common/context blocklist without a
composition rule. Argon2id records embed parameters and meet the approved floor
of 64 MiB, three passes, four lanes, a 16-byte salt, and a 32-byte tag.

TOTP uses a per-owner encrypted 256-bit seed, HMAC-SHA-256, six digits, 30-second
steps, a plus/minus one-step drift window, and a persisted strictly increasing
counter. Correct system time remains an operator responsibility.

Five failures within five minutes create durable account and source throttles.
Delay grows exponentially and caps at 15 minutes. Restarting the Control Plane
does not clear the block. Responses remain generic; inspect redacted audit
actions and service health, not submitted credentials, when diagnosing a
denial. Do not delete throttle, session, recovery, or audit rows manually.

Loss or corruption of the SecretStore key makes the TOTP seed unrecoverable.
Loss of all recovery codes plus the authenticator has no bypass. The Phase 1
development recovery is an owner-reviewed reset/reseed; production backup,
key rotation, and disaster recovery require a later approved package.

## Evidence

Run from the repository root:

```text
task auth:integration
task go:check
task contracts
task policy
task validate
```

The auth fixture uses disposable PostgreSQL 18 and a real TLS HTTP server. It
proves expired/replayed/concurrent bootstrap behavior, TOTP enrollment and
replay, one-use recovery, session rotation/revocation/expiry, exact cookie and
origin/CSRF behavior, restart-persistent throttling, atomic audit actions, and
secret redaction. P1-W11 still must supply actual browser-screen,
localStorage, and full-journey evidence.
