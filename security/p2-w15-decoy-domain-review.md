# P2-W15 decoy domain security review

- Review date: 2026-09-08
- Work package: P2-W15
- Decisions: UX-06, UX-08, DC-01, DC-11, DC-12, INT-01, ON-05, OPS-02, SEC-06,
  SA-11, CP-06
- Scope: the decoy record and its storage, the owner REST surface,
  desired-state transport, observed-state reporting and storage, the decoy
  health condition set, the audit vocabulary, and the Edge decoy component with
  its null runtime

## Trust-boundary conclusion

The decoy is now a real object, and it is deliberately a *thin* one. It carries
no key, certificate, credential, image reference, command, mount, capability,
or port — at any layer. The write contract has no field for one, the desired
object on the wire has no field for one, and the database has no column for
one. Runtime detail is resolved server-side from `decoys/index.json`, which an
operator cannot write to, so the only path by which a container reference could
enter desired state is a committed change to that index.

Observed state is written by exactly one caller: the authenticated device
channel, through `deception.ChannelHandler`. The `DecoyService` interface the
HTTP server holds has no observed-state method, so no operator request — valid,
malformed, or hostile — can assert that a decoy is healthy. The reverse also
holds: the channel's decoy seam is separate from its reconciliation seam, so an
installed reconciler cannot discard or overwrite observed decoy truth, and the
decoy handler cannot publish desired state.

## The honesty controls

These are the ones this package exists for, and they are stated as controls
because misreporting coverage is the security failure of a deception product.

- **Absence is `unknown`.** A decoy with no observed row reads `unknown` with
  all six conditions `Unknown`. The read path materializes that rather than
  defaulting a column, and there is no code path that turns absence into
  `deployed`, `healthy`, or `absent`.
- **Desired and observed never merge.** Two tables, two response objects, two
  Go types. No combined field exists at any level of the JSON, and a test
  asserts the response has no `state` key.
- **A partial report stays partial.** A dimension the Edge did not report
  remains `Unknown`; it is never completed with a favourable value. Both the
  Edge component and the Control Plane read path enforce this independently.
- **No digest is invented.** `pack_digest` is `null` until `P2-W4` supplies a
  manifest to hash. Recording a placeholder would assert an artifact identity
  nothing has verified.
- **The null runtime admits what it is.** It reports `unknown` for every decoy
  and registers component health `degraded` / `no-decoy-runtime`, so an Edge
  running without `P2-W3` is visibly not confirming anything.

## AC-SEC-002, in the direction this package can prove

The Edge decoy package imports neither `internal/identity`,
`internal/devicepki`, `internal/privileged`, nor `internal/privclient`, and
imports no containerd package, `os/exec`, `netlink`, or `crypto/tls`. A
source-level test in `apps/edge-agent/internal/decoy/boundary_test.go` asserts
this at build time, and the integration runner repeats it as a grep an auditor
can run by hand. The package's only outward capabilities are "ask the Runtime
to converge" and "publish an observation".

The package is also forbidden from editing `internal/devicepki` and
`internal/privileged` at all, so it cannot connect what it cannot import.

The runtime direction — that a running decoy container cannot reach the
container runtime socket — is `P2-W3`'s to prove, because `P2-W3` owns the
container lifecycle. Nothing here creates a container.

## SEC-06: a device that lost management

A decoy whose reporting device is `disabled` or `revoked` is projected as
`unmanaged` at read time and stays in the list, with its last-known conditions
intact. It is not healthy, not removed, and not silently missing, so an
operator can see both that the decoy exists and that Guardian can no longer
manage it.

`unmanaged` has no value in the wire enum. An Edge cannot report it about
itself: the device that has lost management is precisely the one whose
self-report about that cannot be trusted, so the Control Plane derives it from
device state it already owns.

## Untrusted content

Exactly one field in the decoy contract is operator-supplied free text:
`display_name`. It is NFC-normalized, trimmed, control-character-rejected, and
bounded to 128 code points and 512 bytes, and the OpenAPI description and the
API source both mark it untrusted to any renderer. Every other string in the
desired object is a closed token — family, persona, interaction level, desired
state — so a console can treat those as categories without rendering an
unbounded backend string as one.

One field is Edge-supplied free text: a condition `message`, bounded to 512
bytes, UTF-8 validated, and control-character-rejected on ingest. Reason codes
are `^[a-z][a-z0-9_]{0,63}$`. `WCX-11` owns hostile-content rendering
(`SEC-08`); this package's obligation is to bound what reaches it and to say
which fields are untrusted, and it does both.

## Placement

An address must be a canonical RFC1918 IPv4 host address inside the referenced
zone's CIDR, and may be neither the zone's network nor its broadcast address.
The check runs in the same transaction as the write, against the zone prefix
read from the database, and the Edge repeats it on receipt rather than trusting
the Control Plane. A composite foreign key prevents a decoy's zone from
belonging to a different environment than the decoy does.

This is a configuration decision, not a network action: the save path contains
no dial, listen, exec, route, firewall, or netlink primitive, and the
integration runner greps for each.

## Bounds and abuse resistance

| Surface | Bound |
|---|---|
| Decoys per environment | 64, refused at create rather than truncated in the snapshot |
| Decoy list page | 200, following the environment and zone convention |
| Request body | 16 KiB, unknown properties rejected |
| Edge decoy report | 16 KiB encoded, 64 observations, 6 conditions each |
| Decoy report rate | Token bucket per device, matching the health report limiter |
| Display name | 128 code points / 512 bytes |
| Condition message | 512 bytes |

The 64-decoy bound is the value `P1-W6` reserved, so the device-channel
message-size guarantee is unchanged; a unit test encodes a full 64-decoy
snapshot and asserts it stays inside the budget.

## Threat review

| Threat | Control and evidence |
|---|---|
| Console claims coverage that does not exist | Observed defaults to `unknown` with six `Unknown` conditions; no read path presents an unreported decoy as deployed; unit, API, and integration tests each assert it. |
| Operator asserts health through the API | `DecoyService` has no observed-state method; observed rows are written only by the device-channel handler. |
| Container escape via an API field | No image, command, argument, mount, capability, or port field exists in the request, the wire object, or the schema; the pack index is the only source of runtime detail, and the API test rejects each such field by name. |
| Decoy reaches device identity or privilege | Import boundary test plus forbidden-path constraint; the Edge decoy package imports no identity, PKI, privileged, or runtime package. |
| One Edge reports on another's decoys | Reporting device comes from the mTLS identity, never a frame field; ingest verifies each decoy belongs to the reporting device's environment and rejects the whole report otherwise. |
| Revoked Edge appears healthy | SEC-06 projection overrides the stored state with `unmanaged` at read time from device state. |
| Decoy silently dropped from desired state | Create refuses the 65th decoy; the snapshot projection fails loudly rather than truncating. |
| Decoy on a production address | Zone containment enforced in the write transaction, re-validated on the wire and again on the Edge; network and broadcast addresses excluded. |
| Unrecognised vocabulary applied as a default | Every enum mapping falls through to `UNSPECIFIED`, and every validator rejects `UNSPECIFIED` as well as unknown values, on both sides of the channel. |
| Removal erases evidence | Removal is a lifecycle transition, not a row delete; audit rows are append-only and the decoy identity stays resolvable. |
| Attacker-supplied text rendered as a category | Only `display_name` and condition `message` are free text, both bounded and marked untrusted; families and personas are closed tokens. |

## Deliberately not covered

- Signature or digest **verification**. `AC-SEC-004` is a Phase 5 gate; this
  package records a digest field and verifies nothing.
- Functional health probing (`P2-W14`) and real container lifecycle
  (`P2-W3`). This package defines the conditions they populate and the runtime
  seam they fill.
- Attacker-facing behaviour of any kind. No pack, adapter, or image is
  implemented here.
- Console rendering (`WCX-11`).

## Conclusion

No new privilege, credential path, or network authority is introduced. The
decoy is a configuration record with a closed vocabulary and a separate,
device-only observed projection. The package's residual risk is concentrated in
what it does *not* claim: with the null runtime in place every decoy reads
`unknown`, which is an accurate report of a system that is not yet deploying
anything.
