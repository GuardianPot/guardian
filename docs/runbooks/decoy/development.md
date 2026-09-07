# Decoy-domain development runbook

P2-W15 makes a decoy a real object: recorded, addressed, versioned, reconciled
to an Edge, and reported back. The one thing to understand before reading
anything else is that a decoy has **two** states and they are never merged.

- `decoy.desired_state` is what an operator asked for: `deployed`, `disabled`,
  or `removed`.
- `observed.observed_state` is what a device reported: `unknown`, `deployed`,
  `degraded`, `absent`, or `unmanaged`.

A decoy no Edge has reported on is `unknown`. It is never `deployed` because
someone asked for it, and never `absent` because nothing has been heard. Until
`P2-W3` supplies a container runtime, **every decoy in a development
environment reads `unknown`, and that is the correct output, not a bug.**

## Preconditions

- Apply all embedded migrations through version `00009` with the explicit
  `control-plane migrate` command.
- Serve the API over HTTPS with `GUARDIAN_PUBLIC_ORIGIN` set to the exact
  browser origin, and authenticate the sole local owner through the P1-W2 flow.
- Create an environment and at least one zone (P1-W3). A decoy is placed into a
  zone, and its address must lie inside that zone's CIDR.

## Placing a decoy

A create request names a pack by `(pack, pack_version)`:

```http
POST /v1/environments/{environmentId}/decoys
Content-Type: application/json
Origin: https://guardian.example.test
X-CSRF-Token: <43-character synchronizer token>

{
  "zone_id": "<zone UUIDv7>",
  "display_name": "Finance file server",
  "family": "smb",
  "persona": "windows_file_service_host",
  "address": "10.20.0.40",
  "pack": "smb-fileshare",
  "pack_version": "0.1.0"
}
```

There is no image, command, mount, capability, port, digest, or
interaction-level field, and an unknown property is rejected rather than
ignored. Everything a runtime needs is resolved server-side from
`decoys/index.json`, so the API has nowhere for a container reference to enter.
`interaction_level` comes from the pack, because INT-01 makes it part of a
decoy's identity rather than an operator preference: SSH is medium, HTTP and
SMB are low, and a database surface may be either.

`pack_digest` is `null`. `P2-W4` defines what a manifest contains and
`P2-W5`–`P2-W8` build the packs, so there is nothing yet to hash. A placeholder
digest would assert an artifact identity nothing has verified, which is the
same failure as reporting an unconfirmed decoy as healthy.

## The rest of the surface

| Operation | Endpoint |
|---|---|
| List | `GET /v1/environments/{environmentId}/decoys` |
| Read | `GET /v1/environments/{environmentId}/decoys/{decoyId}` |
| Update configuration | `PATCH /v1/environments/{environmentId}/decoys/{decoyId}` |
| Remove | `DELETE /v1/environments/{environmentId}/decoys/{decoyId}` |
| Enable | `POST /v1/environments/{environmentId}/decoys/{decoyId}/enable` |
| Disable | `POST /v1/environments/{environmentId}/decoys/{decoyId}/disable` |

Every mutation on an existing decoy requires the strong `If-Match` revision
ETag, exactly as the zone domain does; a missing one is `428` and a stale one
is `412`. Enable and disable are separate operations rather than a `PATCH`
field because `WC-D16` assigns confirmation levels to operations.

No REST path writes observed state. Observed truth arrives only over the
authenticated device channel, so no operator request can assert that a decoy is
healthy.

## Reading the response

```json
{
  "decoy":    { "desired_state": "deployed", "...": "..." },
  "observed": { "observed_state": "unknown", "conditions": [ "..." ] }
}
```

Two sibling objects, and no merged field at any level. The six conditions —
`runtime_healthy`, `address_applied`, `port_responding`,
`telemetry_reporting`, `policy_applied`, `version_matches_desired` — are always
the complete ordered set. A dimension nothing reported is `Unknown`, never
absent and never favourable. They are separate from the start because
`P2-W14`'s acceptance is that killing the process, removing the address, and
breaking telemetry each yield a *distinct* degraded state.

## What each error means

| Status | Body `status` | Cause |
|---|---|---|
| `400` | `address_outside_zone` | The address is not a host address inside the zone's CIDR |
| `400` | `unknown_pack` | `(pack, pack_version)` is not in the server-side index |
| `400` | `invalid_request` | Bounds, vocabulary, ETag shape, or an unknown property |
| `404` | `not_found` | The environment, zone, or decoy does not exist |
| `409` | `name_conflict` / `address_conflict` | Another active decoy holds it |
| `409` | `decoy_budget_exhausted` | The environment already holds the 64 decoys the device channel can carry |
| `412` | `precondition_failed` | The strong revision ETag is stale |
| `428` | `precondition_required` | The strong revision ETag is missing |

## Removal is not deletion of history

`DELETE` retires the decoy: it leaves the active list and releases its name and
address for reuse, but its row and identity survive, and its audit trail —
`decoy.created`, `decoy.updated`, `decoy.enabled`, `decoy.disabled`,
`decoy.removed` — is untouched. Interactions already attributed to that decoy
identity remain attributable.

## SEC-06: a revoked Edge

When a decoy's reporting device is disabled or revoked, the decoy reads
`unmanaged` and **stays in the list**. Not healthy, not removed, not silently
missing. The last-known conditions are preserved, because the decoy may still
be running; what Guardian has lost is the ability to manage it. An Edge can
never report `unmanaged` about itself — the wire enum has no such value, since
the device that lost management is precisely the one whose claim about that
cannot be trusted.

## Desired state reaching the Edge

The reconciler publishes decoys in the existing desired-state snapshot, bounded
at 64 per device, which is the value `P1-W6` reserved for placeholder objects.
A removed decoy is absent from the snapshot rather than present as a tombstone.
The Edge validates placement again on receipt rather than trusting it.

On the Edge, the decoy manager converges through a `Runtime` interface and this
package ships the null one: it accepts desired state, converges nothing, and
reports `unknown` with every dimension `Unknown`. Its component health is
`degraded` with reason `no-decoy-runtime`, which is the honest signal that
nothing it says about a decoy is a confirmation. `P2-W3` replaces it.

## Verifying

```bash
task decoy:integration
```

That brings up a disposable PostgreSQL 18, exercises placement rejection,
lifecycle transitions and their audit rows, desired/observed separation, the
SEC-06 unmanaged projection, the 64-decoy channel bound, and the source-level
check that the decoy packages reach neither device identity nor a privileged
operation.
