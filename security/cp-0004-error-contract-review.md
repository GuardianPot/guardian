# CP-0004 error-code vocabulary and disclosure review

- Review date: 2026-09-08
- Change proposal: `0004-web-console-error-contract.md` (APPROVED, Option B,
  2026-09-04)
- Decisions: `CP-04`, `TS-06`, `RE-10`, `WC-D03`
- Scope: the four optional fields added to the Guardian error body, the closed
  error-code, field-path, and field-reason vocabularies, the correlation
  identifier, and the environment, zone, and decoy handlers migrated onto them

This review is the evidence the proposal names first: *a review of the proposed
code vocabulary for information disclosure*. The other four evidence items are
tests, listed at the end.

## What was added

`StatusResponse` gains four optional properties. `status` is untouched — same
values, same meanings, same HTTP statuses — so the change is additive and a
client reading only `status` cannot observe it.

| Field | Shape | Emitted by |
|---|---|---|
| `code` | closed enum, 24 entries | every migrated response |
| `field_errors[]` | `{field, code}` from two closed enums | responses attributable to a request-body field |
| `retry_after` | integer seconds, 1..3600 | the two `503` service-unavailable responses only |
| `request_id` | 32 lowercase hex characters | every migrated response |

All four are absent rather than null when unset, so the console's
absent-by-default handling covers them with no null case.

## The disclosure question

The proposal is explicit that Option B's cost is a new information-disclosure
surface, and that a carelessly chosen `code` is the way it would be paid. Four
properties were required of every vocabulary entry, and each was checked by
reading all 24 codes, 8 field paths, and 6 reasons.

**1. No entry names an implementation detail.** No code contains a table name,
column, index, constraint, driver, host, port, file path, query, or dependency.
Every entry is built from words already public in `openapi/guardian.yaml`:
resource names (`environment`, `zone`, `decoy`), request-body field names
(`display_name`, `cidr`, `address`, `pack`, `pack_version`, `zone_id`), and
outcome words (`invalid`, `not_found`, `conflicting`, `stale`, `required`,
`unknown`, `overlapping`, `exhausted`, `outside_zone`, `unavailable`). A caller
learns nothing from a code that the published contract did not already tell
them.

`TestErrorVocabulariesAreClosedOpaqueAndMatchTheContract` enforces this
mechanically against a forbidden-substring list, so a future `zone.cidr.pgx_23505`
fails the lane rather than shipping.

**2. No entry distinguishes states a caller may not distinguish.** This is the
one that needed judgement rather than a regex, because a code is finer-grained
than a status by design and finer-grained authorization oracles are a real
failure mode.

- Authentication and authorization are **not migrated**. A `401` still returns
  `{"status": "unauthorized"}` with no code, no field errors, and no
  correlation identifier. An unauthenticated or unauthorized caller therefore
  gains nothing at all from this change, which is the property that matters:
  every code below is only reachable by an owner session already authorized for
  the environment.
- `decoy.zone.not_found` is distinguishable from `decoy.not_found`. Both are
  `404 not_found`. The distinction is between a `zone_id` the caller just typed
  into the body and a decoy path segment, and an authorized owner may already
  enumerate both zones and decoys through `GET`, so this reveals nothing they
  could not read directly. It is the distinction `WCX-11` needs to mark the
  right form field.
- `decoy.budget.exhausted` reveals that the environment holds 64 decoys. The
  bound is published in `openapi/guardian.yaml` and the count is readable from
  the decoy list, so this is not new information.
- `zone.cidr.overlapping` reveals that *some* zone overlaps the submitted
  prefix. It does not say which, and the owner can list zones anyway.
- `internal.unexpected` is the only code permitted on a `500`, and it is
  deliberately contentless. There is no `internal.database`, no
  `internal.timeout`, and no per-subsystem variant, because a code that
  partitioned server failures would let a caller probe which subsystem they had
  reached.

**3. No free text anywhere.** There is no `message`, `detail`, `title`,
`description`, `reason`, or `hint` field at any level of the body, and
`TestMigratedErrorsCarryTheClosedVocabulary` walks every decoded response to
assert none appears. `TestNoHandlerEmitsOperatorFacingProse` parses the API
package's own source and asserts that every `status`, every code suffix, and
every denial slug at every call site is a lowercase machine-token string
literal, so prose cannot enter through a computed or formatted argument either.

`field_errors[].message_key` is declared in the contract, because constraint 3
defines what it must be when present, but no handler sets one. The console has
no per-code catalogue entry to key against — `WCX-08` owns wording — so
supplying a key would assert something that does not exist. A test asserts it
stays empty.

**4. No submitted value is echoed.** This is the leakage path the proposal
names directly, and it is closed structurally rather than by discipline.

Field attribution is produced by the domain validator that made the rejection,
as a `FieldViolation` carrying a closed `Field` and a closed `Reason` and
nothing else. The type has nowhere to put a value, so a validator cannot leak
one by accident. The API layer reads only that attribution and the error
sentinel; it never reads the request body when building an error. Conflict
sentinels (`ErrNameConflict`, `ErrAddressConflict`, `ErrCIDRConflict`,
`ErrAddressOutsideZone`, `ErrUnknownPack`) each name exactly one field by
construction, so those attributions are static.

Three value-carrying error strings were removed from the deception domain in
the same change — `AddressWithinZone` formatted the submitted address and the
zone prefix into its message, and `ResolvePack` formatted the pack and version.
Those strings never reached a response, but they reached logs, and a violation
carrying two closed tokens is strictly better than a formatted string that has
to be trusted not to escape.

`TestNoSubmittedValueReachesAnyErrorResponse` puts a distinctive marker into
each of the seven decoy write fields in turn, plus the zone and environment
write fields, plus an operator-chosen unknown JSON key, plus the
`X-Request-ID` header, drives each through both the create and update paths,
and asserts the marker appears nowhere in the response — not in a value, not in
a key, not in a code.

## The correlation identifier

`request_id` is 16 bytes from `crypto/rand`, hex-encoded. It contains no
timestamp, no counter, no sequence, and no environment, zone, decoy, device,
session, or actor identity.

It is deliberately **not** a ULID. The proposal's illustrative JSON showed
`"01J…"`, but constraint 5 requires that the identifier carry no embedded
meaning, and a ULID embeds its creation time. The constraint is the binding
half, so the example was not followed.

`TestRequestIDCarriesNoDerivableMeaning` covers the three usual leaks: it
asserts 512 identifiers are unique and non-monotonic (no clock, no counter),
that every one of the 32 character positions takes at least 8 distinct values
(no constant or derived segment), and that eight byte-identical requests
produce eight different identifiers (not a function of the request).

If the entropy source fails, `newRequestID` returns empty and the field is
omitted. A missing identifier costs support one correlation; a predictable one
would break the contract.

Each migrated error also writes one bounded log line carrying the identifier,
the status, the code, the HTTP status, the method, and the route pattern. No
operator input reaches that line. `4xx` logs at info and `5xx` at error, which
preserves the previous logging behaviour for server failures.

The caller-supplied `X-Request-ID` mutation header is unrelated and is never
echoed. Connecting the two would put caller-chosen input into a field the
contract promises is opaque and server-generated.

## Trust-boundary conclusion

The trust boundary is unchanged. No new privilege, credential path, network
authority, or datastore access is introduced; nothing here reads or writes
anything it could not already reach, and no successful response shape changed.

The proposal's assessment — neutral to slightly positive when handled with
closed vocabularies, no free text, no echoed values, and an opaque identifier —
holds. The positive half is real: the console stops inferring `validation` from
`400` and `conflict` from `409`, so a backend that changed a status code can no
longer silently change what an operator is told, and `WCX-04`'s requirement
that `denied` be distinguishable from `empty` now rests on a stated contract
rather than a convention.

## Deliberately not covered

- **Authentication and authorization responses.** `401` and `426` stay on the
  Phase 1 body, so an unauthenticated caller gains no code, no field error, and
  no correlation identifier. Migrating them is a separate decision with a
  different threat model.
- **Rate-limited responses.** `429` on the auth surface is unmigrated;
  `retry_after` is currently emitted only by the two `503` service-unavailable
  responses, where it is a hint and not a promise.
- **Console rendering.** `WCX-02` shipped the taxonomy and already treats these
  fields as absent-by-default; `WCX-11` will consume `field_errors`. No console
  change was required or made here beyond regenerating the OpenAPI types.
- **Every other endpoint.** Device, enrollment, audit, telemetry, and health
  endpoints are unmigrated and return `{"status"}` unchanged. That is the
  incremental rollout the proposal specifies, not an oversight.

## Evidence index

| Proposal requirement | Where |
|---|---|
| Code vocabulary reviewed for disclosure | this document; `TestErrorVocabulariesAreClosedOpaqueAndMatchTheContract` |
| No submitted value in any error response | `TestNoSubmittedValueReachesAnyErrorResponse` |
| `request_id` carries no derivable meaning | `TestRequestIDCarriesNoDerivableMeaning` |
| An unmigrated endpoint still behaves correctly | `TestUnmigratedEndpointStillReturnsThePhase1Body` |
| No handler emits operator-facing prose | `TestNoHandlerEmitsOperatorFacingProse` |
| Drifted code fails contract linting | `TestErrorVocabulariesAreClosedOpaqueAndMatchTheContract` compares the Go vocabulary against the `ErrorCode`, `FieldPath`, and `FieldErrorCode` enums in `openapi/guardian.yaml` |

All are in `apps/control-plane/internal/api/errorcontract_test.go`, and
`TestMigratedErrorsCarryTheClosedVocabulary` additionally pins the exact
status, code, and field errors for all 27 migrated failure paths.
