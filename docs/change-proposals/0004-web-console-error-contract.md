# Change proposal 0004: structured Web/Public API error contract

- Status: APPROVED
- Owner decision: `@sinanganiz` approved on 2026-09-04.
- Affected decision IDs: `CP-04`, `TS-06`, `RE-10`, `WC-D03`
- Affected acceptance criteria: `PERF-07` common API behaviour on the reference
  dataset; `WCX-04` data-state matrix requirement that `denied` is
  distinguishable from `empty`; `WCX-11` field-level validation display
- Work package: `WCX-11`, with `WCX-02` delivering the frontend half
  independently

## Problem and context

Every Control Plane error response is a single field: `{"status": "<slug>"}`,
written by `writeStatus` in `apps/control-plane/internal/api/server.go` and
described in `openapi/guardian.yaml` as `StatusResponse`.

That shape is sufficient for a five-screen console with one- and two-field
forms. It stops being sufficient at three points that Phase 2 onwards reaches:

1. **Field-level validation is impossible.** A rejected decoy configuration
   with eight interdependent fields returns one slug. The console cannot mark
   the offending field, so `WCX-11` must fall back to a form-level message
   that tells the operator only that something in the form is wrong. For a
   generalist operator without a SOC, that is a poor failure mode on the
   screen with the most fields in the product.
2. **Machine-readable distinctions are carried by convention.** The frontend
   taxonomy in `WCX-02` infers `conflict` from HTTP 409 and 412, `validation`
   from 400 and 422, and so on. That inference is correct today, but the
   mapping lives in the console rather than in the contract, so a backend that
   starts returning 400 for a revision conflict would silently change what the
   operator is told. `WCX-04` requires `denied` to be provably distinguishable
   from `empty`, and today that proof rests on a status-code convention rather
   than on a stated contract.
3. **There is no correlation identifier.** When a pilot operator reports a
   failure, nothing links what they saw to a Control Plane log line. `WCX-15`
   builds an allowlisted diagnostic report and reserves a field for a request
   identifier precisely because none exists yet.

`WC-D03` was accepted on 2026-09-03 as a two-part answer: Option A, a frontend
taxonomy over the current body, delivered in `WCX-02` with no backend change;
and Option B, this proposal. `WCX-02` therefore proceeds regardless of the
outcome here.

## Options

1. **Option A — frontend taxonomy only; contract unchanged.** `WCX-02`
   delivers `toConsoleError()` and the console maps status codes and slugs to
   a closed set of error kinds.

   Advantages: no contract change, no backend work, no new leakage surface; it
   is already accepted and scheduled; it removes the current situation where
   each route invents its own message.

   Disadvantages: field-level validation stays impossible, so `WCX-11` ships a
   materially worse form experience on the product's most complex form; the
   status-code convention remains an undocumented coupling between backend and
   console; no correlation identifier exists for pilot support.

2. **Option B — extend the error body.** Keep `status` and add optional
   `code`, `field_errors`, `retry_after`, and `request_id`. Response shape:

   ```json
   {
     "status": "invalid_argument",
     "code": "zone.cidr.overlapping",
     "field_errors": [
       { "field": "cidr", "code": "overlapping", "message_key": "zone.cidr.overlapping" }
     ],
     "retry_after": null,
     "request_id": "01J…"
   }
   ```

   Advantages: field-level errors become possible; the validation, conflict,
   rate-limit, and degraded distinctions move from convention into the
   contract; a correlation identifier enables real pilot support; the
   `WCX-02` taxonomy gains fidelity without being rewritten, because it was
   designed to consume these fields when present.

   Disadvantages: it touches every handler, the OpenAPI document, and the
   handler test suite. More importantly, it creates a new information-leakage
   surface: a `code` or a `message_key` chosen carelessly can disclose
   internal structure, and a free-text `message` field would be worse. It also
   risks encouraging backends to return operator-facing prose, which would
   bypass the `WCX-08` text catalogue and the wording review that catalogue
   exists to enable.

3. **Option C — adopt RFC 9457 problem details.** Replace the body with
   `application/problem+json`.

   Advantages: a well-known standard shape with existing tooling.

   Disadvantages: it replaces rather than extends the current body, so every
   existing response and test changes at once for no additional capability
   beyond Option B; `type` as a dereferenceable URI implies published
   documentation the project does not have; the standard's `detail` field
   invites exactly the free-text prose that Option B deliberately excludes.

## Recommendation

**Option B**, with these constraints as part of the recommendation:

1. `status` keeps its current values and meaning. Existing clients and tests
   that read only `status` continue to work, so the change is additive rather
   than a replacement.
2. **No free-text operator-facing message is added.** `code` and
   `field_errors[].code` are stable machine identifiers from a closed,
   reviewed vocabulary, mirroring the existing audit vocabulary discipline.
   The console maps them to catalogue keys, so all operator-facing wording
   stays in `WCX-08` and remains reviewable in one place.
3. `message_key` is optional and, when present, is a catalogue key, never a
   sentence. A backend that supplies a key the console does not recognise
   falls back to the generic entry for its `status`; an unrecognised key is
   never rendered verbatim.
4. `field_errors[].field` names a request-body field path. It must never echo
   a submitted value, because echoing a rejected password, token, or persona
   string back into an error response would create a leakage path.
5. `request_id` is a random, opaque identifier with no embedded meaning. It is
   safe to display and safe to place in the `WCX-15` diagnostic report, and it
   is the only field from this contract permitted in that report.
6. The error-code vocabulary is reviewed as a contract surface, exactly as the
   audit vocabulary is, and is enumerated in `openapi/guardian.yaml` so a
   drifted code fails contract linting.
7. Endpoints are migrated incrementally. Every field is optional, so an
   unmigrated endpoint keeps returning `{"status"}` and the console's
   `WCX-02` taxonomy continues to work. There is no flag day.

Evidence required before approval can be considered discharged: a review of
the proposed code vocabulary for information disclosure; tests proving no
submitted value appears in any error response; tests proving `request_id`
carries no derivable meaning; a demonstration that an unmigrated endpoint
still behaves correctly in the console; and confirmation that no
operator-facing prose is emitted by any handler.

## Impact

- **Product scope:** no new capability. The decoy configuration form in
  `WCX-11` gains field-level errors; every other screen keeps its current
  behaviour with better internal fidelity.
- **Architecture:** no architectural change. `CP-04` REST/JSON with OpenAPI is
  unchanged; `RE-10` is unaffected because the console consumes generated
  types for the extended shape rather than hand-declaring it.
- **Contracts and data:** `StatusResponse` gains four optional fields, or a
  sibling schema is added and referenced by error responses. No successful
  response shape changes. No database migration is required. A new enumerated
  error-code vocabulary becomes a reviewed contract surface.
- **Security and trust boundary:** the trust boundary is unchanged. The new
  surface is information disclosure, addressed by constraints 2 through 5:
  closed vocabularies, no free text, no echoed values, and an opaque
  correlation identifier. Handled that way the change is neutral to slightly
  positive, because the console stops inferring semantics from status codes.
  Handled carelessly it is a regression, which is why the vocabulary review is
  named as required evidence.
- **Operations and release:** `request_id` should be correlatable with Control
  Plane logs, which is an operational benefit for the pilot. No deployment or
  configuration change.

## Rollout and failure behavior

Every added field is optional, so rollout is incremental by endpoint and
requires no compatibility layer. The console's `WCX-02` taxonomy already
treats these fields as absent-by-default, so a partially migrated backend
produces correct, if less specific, console behaviour throughout.

Recommended migration order, following where the value is highest:
environment and zone write endpoints first, since `WCX-09` and `WCX-11` depend
on conflict and validation fidelity there; then the Phase 2 decoy endpoints as
they are built, so they are born with the richer contract; then the remainder
opportunistically.

If the proposal is declined, `WCX-02` still ships the frontend taxonomy and
`WCX-11` ships with form-level errors only, which is recorded in that
package's error-presentation section as the unapproved-path behaviour. If the
change is approved and later proves harmful, the fields can be dropped from
responses without breaking any client, because the console never requires
them.

Failure behavior of the mechanism itself: an unrecognised `code` falls back to
the generic entry for its `status`; an unrecognised `field` in `field_errors`
is surfaced as a form-level error rather than silently discarded, so a
mismatch between backend and console can never hide a rejection reason.

## Owner decision record

- Decision: APPROVED
- Decided by: `@sinanganiz`
- Date: 2026-09-04
- Rationale: Option B approved: additive optional code, field_errors, retry_after, and request_id on the existing status body, subject to every constraint in the Recommendation section including the closed code vocabulary and the prohibition on free-text operator-facing messages.

Context for the decision: the Product Owner accepted `WC-D03` on 2026-09-03 as
Option A now plus Option B routed here. The strongest argument for approving
Option B promptly rather than later is that the repository development
compatibility policy currently permits forward-only change and the backend is
still early; the same change made after the Phase 3 incident endpoints exist
will touch considerably more surface. The strongest argument against is that
Option A has not yet been used in anger, so the field-level need is reasoned
rather than observed. Only `@sinanganiz` can record `OWNER DECISION` and
`APPROVED`.
