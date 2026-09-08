# P2-W10 and P2-W11 event pipeline security review

- Review date: 2026-09-08
- Work packages: P2-W10 (canonical event envelope), P2-W11 (Edge normalization)
- Decisions: EV-02, EV-03, EV-04, EV-05, SRC-07
- Acceptance: AC-EV-001, AC-EV-002, AC-EV-003, AC-SEC-008
- Scope: `schemas/event/v1/canonical-event.schema.json`,
  `apps/control-plane/internal/event`, `apps/edge-agent/internal/normalize`

## What this pipeline is exposed to

Everything in it has been shaped, directly or indirectly, by whoever is
attacking the decoy. A Cowrie log records what they typed; an HTTP access line
records the path they asked for; a session identifier is whatever the pack
generated while talking to them. The pipeline is written on that basis.

Two failure modes are worth naming because they are the ones that would matter.
An attacker who can crash the Edge has blinded the sensor watching them, which
is worth more to them than any single decoy. An attacker whose input reaches a
console, an index, or a log line unescaped has turned Guardian's own reporting
into their delivery mechanism.

## The envelope carries no attacker bytes

The strongest control here is structural. The canonical event carries metadata
and a **reference** to raw evidence, never the evidence. `raw_ref` names a blob,
its media type, its length, and whether it was truncated or quarantined.

That keeps an attacker's transcript in one bounded place instead of in every
index, log line, and console view that touches an event. `EV-03` already
required raw payloads to be bounded and selectively retained; this makes the
envelope unable to hold one at all.

The exceptions are deliberate and bounded: `auth.identity` (256 bytes) and
`session_id` (128 bytes) are attacker-influenced strings the model genuinely
needs, and both are length-capped at both ends of the pipeline. They remain
untrusted to any renderer — `WCX-06`'s components are what the console must use
for them.

## There is nowhere to put a credential

`EV-03` requires credential material to be protected or redacted. Rather than a
rule someone has to remember at each adapter, neither event type has a field a
secret could live in. `auth` carries the identity offered and the result;
`synthetic_credential_id` names a credential **Guardian planted**, so a match is
provable without storing what was typed.

Enforced three ways: a Go test walks the serialised shape looking for any key
whose name could hold a secret; the canonical-contract fixture checks the same
on the schema; and the cross-module parity check greps both Go types with
comments stripped. The third exists because the comments explaining this rule
necessarily use the words the rule forbids.

## The trust boundary inside the envelope

Two fields are the Control Plane's, and an Edge asserting either is a boundary
violation rather than a bad value.

**`ingested_time`.** An Edge that could assert when its event was accepted could
backdate one past a retention boundary. An event arriving with it set is refused
with its own sentinel, and the Edge's own struct has no field for it — a
producer that cannot express the value cannot send it.

**`edge_id`** is taken from the authenticated device identity at ingest, so one
Edge cannot attribute an event to another. The envelope enforces the shape; the
ingest path enforces the match, and that half belongs to `P2-W12`.

**`provenance.source_path`** is the same idea one layer down. An adapter cannot
say which path it came through: the pipeline overwrites it after the adapter
returns. An adapter that could label a parsed log line `native_uds` would be
inflating the confidence a reader places in it, and `SRC-07` is about exactly
that.

## AC-SEC-008, taken literally

The acceptance is that an oversized or malformed payload does not crash either
process. The pipeline's ordering is the control:

| Stage | Why it is there |
|---|---|
| size check before parsing | an oversized payload costs a length comparison, not an allocation |
| adapter inside a `recover` | an adapter indexes into attacker-written text; a slice bug becomes `adapter_failed`, not a dead Edge |
| validation before dedup | a malformed identifier never enters the dedup window |
| dedup before rate limiting | a retry storm does not eat the budget a real burst needs |
| bounded scanner on log lines | a 100 KB line is refused without being assembled |

Asserted against oversized, truncated, non-JSON, empty, wrong-typed, deeply
nested, and invalid-UTF-8 inputs, and against an adapter that panics on purpose.

The `recover` deserves a note, because catching panics is usually a smell. It is
here because an adapter is the one place in this design that parses hostile text
with per-pack code, and `P2-W5` through `P2-W8` will each add one. Containing
that blast radius is worth more than the crash being loud, and the reason code
keeps it visible rather than silent: `adapter_failed` means a Guardian bug, and
it is counted.

## Bounded everywhere an attacker chooses the size

Every quantity an attacker influences has a ceiling, and the ceiling is enforced
before the thing it bounds is read: event bytes, batch size, quarantined bytes,
dedup window, identity and session length, raw-reference length, and the token
bucket itself.

The dedup window evicting its oldest entry is a deliberate trade. An unbounded
window is a memory-exhaustion path reachable by anyone who can make a decoy
noisy, which is what a decoy is for; a duplicate arriving after the window has
turned over is caught by the durable constraint instead.

Quarantine is bounded and marked truncated for the same reason: keeping every
byte of a malformed payload would let the attacker choose how much disk a
malformed event costs. A rate-limited event is not quarantined at all — it is
well-formed and unremarkable, and keeping every one is how a burst becomes a
disk problem.

## Drift between two modules

The event type is declared twice because two Go modules cannot share a package,
and that is exactly the arrangement that drifts. `tests/integration/event`
declares the vocabularies itself and asserts all three — schema, Control Plane,
Edge — agree with it, so agreeing with each other is not enough. Verified by
adding a value to one side and watching the check fail.

## Deliberately not covered

- **Durable idempotency.** `P2-W12` owns the unique constraint. The
  deduplicators here make a duplicate detectable; the database makes it
  impossible, and both packages say so rather than implying the weaker one is
  the guarantee.
- **Blob and quarantine storage.** Interfaces here; `P2-W13` owns where bytes
  land, their retention, and their access control.
- **Pack parsers.** `P2-W5` through `P2-W8` write them and inherit these bounds.
  Each new adapter is new attacker-facing parsing and should be reviewed as such.
- **Console rendering.** `WCX-12` and `WCX-13` read evidence, through the
  `WCX-06` untrusted components.

## Conclusion

No new privilege and no new network reach. The pipeline's security rests on
three structural properties rather than on care: the envelope cannot hold an
attacker's bytes, it cannot hold a credential, and it cannot be made to allocate
before it has checked a bound.

The residual risk sits in the adapters that do not exist yet. Every one of them
is new code parsing hostile input, and the containment here — bounds, recover,
drop reasons — is what decides whether a bug in one of them is an incident or a
counter.
