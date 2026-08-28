# Environment-domain development runbook

P1-W3 provides one immutable local organization, multiple environments, and
canonical private IPv4 zones. An environment status is configuration
completeness only: `needs_zones` means no zones are configured and
`zones_defined` means at least one exists. Neither value claims that devices,
health, protection, or deception are active.

## Preconditions

- Apply all embedded migrations through version `00005` with the explicit
  `control-plane migrate` command.
- Serve the API over HTTPS and configure `GUARDIAN_PUBLIC_ORIGIN` to the exact
  browser origin.
- Bootstrap and authenticate the sole local owner through the P1-W2 flow.
- Keep the session cookie in the browser cookie jar and the synchronizer CSRF
  value in memory. Do not place either value in a URL, shell history, log, or
  committed example.

Development databases created before P1-W3 can contain device rows whose
environment does not exist. Migration `00005` intentionally fails rather than
preserving those orphans. The approved development recovery is an explicit
database reset followed by migration and authorized reseeding; there is no
compatibility or synthetic-environment layer.

## API workflow

All responses use `Cache-Control: no-store`. Reads require the secure owner
session cookie. Mutations additionally require the exact `Origin` and
`X-CSRF-Token` headers.

Read the implicit organization and list environments:

```console
curl --fail --silent --show-error \
  --cookie "$GUARDIAN_COOKIE_FILE" \
  'https://control-plane.guardian.test/v1/organization'

curl --fail --silent --show-error \
  --cookie "$GUARDIAN_COOKIE_FILE" \
  'https://control-plane.guardian.test/v1/environments?limit=50'
```

Create an environment, supplying the in-memory CSRF value without printing it:

```console
curl --fail --silent --show-error \
  --cookie "$GUARDIAN_COOKIE_FILE" \
  --header 'Content-Type: application/json' \
  --header "Origin: $GUARDIAN_PUBLIC_ORIGIN" \
  --header "X-CSRF-Token: $GUARDIAN_CSRF_TOKEN" \
  --data '{"display_name":"Production"}' \
  'https://control-plane.guardian.test/v1/environments'
```

The create/read response includes a strong decimal `ETag`, for example a
revision represented as a quoted integer. Preserve that exact header for a
subsequent rename:

```console
curl --fail --silent --show-error \
  --cookie "$GUARDIAN_COOKIE_FILE" \
  --header 'Content-Type: application/json' \
  --header "Origin: $GUARDIAN_PUBLIC_ORIGIN" \
  --header "X-CSRF-Token: $GUARDIAN_CSRF_TOKEN" \
  --header "If-Match: $GUARDIAN_ENVIRONMENT_ETAG" \
  --request PATCH \
  --data '{"display_name":"Production primary"}' \
  "https://control-plane.guardian.test/v1/environments/$GUARDIAN_ENVIRONMENT_ID"
```

Create two non-overlapping zones:

```console
curl --fail --silent --show-error \
  --cookie "$GUARDIAN_COOKIE_FILE" \
  --header 'Content-Type: application/json' \
  --header "Origin: $GUARDIAN_PUBLIC_ORIGIN" \
  --header "X-CSRF-Token: $GUARDIAN_CSRF_TOKEN" \
  --data '{"display_name":"Application","cidr":"10.20.0.0/24"}' \
  "https://control-plane.guardian.test/v1/environments/$GUARDIAN_ENVIRONMENT_ID/zones"

curl --fail --silent --show-error \
  --cookie "$GUARDIAN_COOKIE_FILE" \
  --header 'Content-Type: application/json' \
  --header "Origin: $GUARDIAN_PUBLIC_ORIGIN" \
  --header "X-CSRF-Token: $GUARDIAN_CSRF_TOKEN" \
  --data '{"display_name":"Database","cidr":"192.168.20.0/24"}' \
  "https://control-plane.guardian.test/v1/environments/$GUARDIAN_ENVIRONMENT_ID/zones"
```

Zone update and removal use the zone resource's own current `ETag`. Missing
`If-Match` returns `428`; a stale value returns `412`. Environment deletion is
not a Phase 1 operation and returns `405`.

## Validation rules

Display names are trimmed, NFC-normalized, limited to 128 Unicode code points
and 512 UTF-8 bytes, and unique by Unicode case-fold within the parent. Inputs
with unknown JSON fields, trailing JSON, a body over 16 KiB, or an unsupported
query parameter are rejected.

A zone CIDR must already be a canonical, host-bit-free IPv4 prefix wholly
inside exactly one of `10.0.0.0/8`, `172.16.0.0/12`, or `192.168.0.0/16`.
Public, loopback, link-local, multicast, unspecified, IPv6, zero-prefix,
malformed, duplicate, nested, or overlapping input is rejected. Overlap is
scoped to one environment, so separate environments may intentionally reuse a
private address plan.

Concurrent zone mutations are serialized by a transaction-scoped advisory
lock keyed by environment. The overlap check, data mutation, environment
revision advance, and P1-W10 audit append commit together. A failed audit
append rolls back the domain mutation.

## Evidence and diagnosis

Run the focused PostgreSQL 18 and HTTPS fixture:

```console
task environment:integration
```

The fixture creates an isolated PostgreSQL project and proves singleton
immutability, UUIDv7 identities, name/CIDR boundaries, two valid zones,
same-environment overlap rejection, cross-environment reuse, deterministic
concurrent conflict, rollback on audit failure, device referential integrity,
restart persistence, real P1-W2 session/CSRF/Origin authorization, ETags,
request bounds, forbidden environment deletion, redacted snapshots, and the
absence of scanning/routing/firewall primitives from the save path.

For a conflict, re-read the resource and compare its new `ETag` before deciding
whether to retry. Do not bypass preconditions, edit revision columns, disable
the audit guard, delete audit history, or manually weaken the device foreign
key. A database error is deliberately returned to the client only as a generic
status; inspect sanitized server logs and database health under an authorized
operator procedure.

Saving an environment or zone performs PostgreSQL work only. It never probes a
CIDR, discovers hosts, allocates addresses, changes routes, or changes firewall
state. P1-W9 health, device activity, desired state, and deception placement
are separate workflows.
