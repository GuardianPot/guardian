# Device enrollment development runbook

This runbook operates only the P1-W4 development boundary. P1-W2 owns the real
browser-session authorizer, P1-W3 owns environment semantics, P1-W5 owns the
live management channel, and P1-W9 owns health projection. Until P1-W2 is
integrated, operator HTTP endpoints deliberately return `401`; tests supply an
explicit bounded authorizer fixture.

## Initialize the Control Plane

Apply migrations first. Create the master key outside the checkout on the
Control Plane host and restrict both its directory and file:

```sh
install -d -m 0700 /var/lib/guardian/secrets
umask 077
openssl rand 32 > /var/lib/guardian/secrets/device-ca-master.key
test "$(wc -c < /var/lib/guardian/secrets/device-ca-master.key)" -eq 32
```

Provision a separate server TLS certificate/key whose certificate chains to
the Edge operating-system trust store. Then initialize the product device CA
exactly once:

```sh
export GUARDIAN_DATABASE_URL='postgres://guardian:<password>@127.0.0.1:5432/guardian'
export GUARDIAN_MASTER_KEY_FILE='/var/lib/guardian/secrets/device-ca-master.key'
go -C apps/control-plane run ./cmd/control-plane migrate
go -C apps/control-plane run ./cmd/control-plane init-device-ca
```

A second `init-device-ca` invocation is rejected. Losing the mounted master key
makes the encrypted CA unavailable; the recovery action is an owner-reviewed
development reset/reseed, not silent CA replacement.

Start the enrollment-enabled server:

```sh
export GUARDIAN_TLS_CERT_FILE='/etc/guardian/tls/control-plane.crt'
export GUARDIAN_TLS_KEY_FILE='/etc/guardian/tls/control-plane.key'
export GUARDIAN_HTTP_ADDRESS='127.0.0.1:8443'
go -C apps/control-plane run ./cmd/control-plane serve
```

## Create and consume a one-time token

After P1-W2 supplies its authorizer, an authorized operator creates a token
with the `__Host-guardian_session` cookie plus same-origin CSRF and Origin
headers:

```sh
curl --fail-with-body --request POST \
  --cookie '__Host-guardian_session=<opaque-session>' \
  --header 'X-CSRF-Token: <csrf-token>' \
  --header 'Origin: https://console.guardian.test' \
  --header 'Content-Type: application/json' \
  --data '{"device_name":"edge-one"}' \
  'https://control-plane.guardian.test/v1/environments/<environment-uuid>/enrollment-tokens'
```

Copy the returned 43-character token directly into the Edge command through
standard input. Do not place it in an argument, URL, environment variable,
configuration file, shell history, or log:

```sh
read -r -s enrollment_token
printf '%s\n' "$enrollment_token" | guardian-edge enroll --config /etc/guardian/edge.json
unset enrollment_token
```

Edge generates an ECDSA P-256 key locally, submits only a bounded CSR over TLS,
validates the returned CA chain, UUID URI SAN, serial, validity, and public-key
match, then installs the certificate/key pair recoverably. The identity
directory must be owner-only; the private-key file is mode `0600` and must not
be a symlink. Starting `guardian-edge serve` validates the identity and begins
automatic, deterministic-jitter rotation during the final ten days.

## Disable, revoke, and re-enroll

The operator mutation endpoints use the same session, CSRF, Origin, and
environment-scope boundary:

- `POST /v1/environments/{environmentId}/devices/{deviceId}/disable` blocks new
  authenticated sessions without reactivating anything later by itself.
- `POST /v1/environments/{environmentId}/devices/{deviceId}/revoke` permanently
  revokes every active serial.
- `POST /v1/environments/{environmentId}/devices/{deviceId}/re-enrollment-token`
  is the explicit re-enable/re-enrollment action. It retains the stable device
  ID, permanently revokes prior active certificates and unused tokens, moves
  the record to `pending`, and returns one new 15-minute token.

Run the Edge `enroll` command with the new token. No previous certificate,
private key, token, or serial is reactivated.

## Failure and recovery

- Token failures return a generic denial. Five failures within five minutes
  trigger persistent source/token throttling with bounded exponential delay.
- A consumed, expired, revoked, malformed, or replayed token cannot be retried;
  request a new explicit re-enrollment token.
- Rotation writes a durable install journal and staged files. On restart, Edge
  selects a matching old, new, or final pair and removes abandoned stages.
- If initial enrollment was consumed but identity installation did not produce
  a recoverable pair, use explicit re-enrollment; never copy a private key from
  the Control Plane because it never possesses one.
- Disabled/revoked/stale serial checks combine X.509 chain verification with
  current PostgreSQL eligibility. P1-W5 must call this verifier before every
  new management session.

## Evidence

Run:

```text
task enrollment:integration
task go:check
task contracts
task policy
```

The integration fixture creates disposable PostgreSQL 18 state and disposable
CA/Edge keys, proves one winner under concurrent token replay, exercises stable
re-enrollment, rotation, disable/revoke, durable audit, TLS exchange,
filesystem permissions, and scans committed paths for secret-shaped material.
It removes the test database volume at exit.
