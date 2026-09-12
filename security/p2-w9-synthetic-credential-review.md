# P2-W9 synthetic credential domain security review

- Review date: 2026-09-10
- Work package: P2-W9
- Decisions: DC-11, EV-02, EV-03, CS-06
- Acceptance: P2-W9's two criteria
- Scope: `apps/control-plane/internal/credential`

## Why this package needs a review even though nothing here is secret

The material this package mints is worthless by construction. That is exactly
what makes it easy to handle carelessly, and there are three ways carelessness
here would produce a real problem.

**A synthetic credential that works somewhere real.** The failure is obvious and
the mitigation is structural: the material is 16 bytes of `crypto/rand` with a
fixed prefix, so it is not derived from, does not resemble, and cannot collide
with anything an operator uses. Nothing in this package reads an existing
credential, and there is no input from which one could be seeded.

**A synthetic credential Guardian cannot recognise as its own.** If a planted
credential turned up in a production password manager and looked like any other
leaked secret, somebody would spend a night rotating things that never needed
rotating. The `gdn-decoy-` prefix is what prevents that.

**Plaintext kept after the operator has planted it.** The acceptance criterion
says the UI must not reveal a reusable secret after the intended workflow. This
package makes that stronger than a UI rule: the stored type has no field for a
plaintext secret, so there is nothing for a screen, a log line, or a query
result to reveal.

## The marker is a trade, and it is the one judgement call here

`gdn-decoy-` tells an attacker who has found a credential that they are inside a
deception system. That is a genuine cost and it should be recorded as accepted
rather than overlooked.

It is accepted because the two halves land at different times. The
identifiability benefit arrives when somebody *finds* the credential — in a
paste, a log, a repository, a password manager — and can tell immediately that
it grants nothing. The disclosure cost arrives when somebody *types* it, and by
then the evidence the credential exists to produce has already been produced.

The alternative — unmarked material indistinguishable from a real leaked
secret — is worse in the case that actually costs a customer something.

If the Product Owner wants the opposite trade, the prefix is one constant and
one test.

## What is stored

| Stored | Not stored |
|---|---|
| SHA-256 of the raw secret bytes | the secret |
| decoy, environment, kind, username | anything derived from a real credential |
| lifecycle timestamps | the plaintext, at any point after `New` returns |

SHA-256 matches the enrollment-token handling `P1-W4` already reviewed. A slow
KDF would be the right answer for a password; it is the wrong answer here,
because the value has 128 bits of entropy and the comparison sits on a decoy's
authentication path where an attacker controls the request rate.

The raw bytes are zeroed after hashing. That is hygiene rather than a control —
Go's garbage collector makes no promises — but it costs nothing and narrows the
window.

## Matching is constant-time, and the reason is not the obvious one

Guessing a synthetic credential gains an attacker nothing: the thing grants no
access, and a guess that landed would only manufacture evidence against
themselves.

`subtle.ConstantTimeCompare` is used anyway because this comparison runs on the
same code path as every other credential check a decoy performs. A timing side
channel that exists "only in the honeypot branch" is one somebody will
eventually copy into a branch where it counts.

## Strict base64, found by a test

The first version used lenient decoding. A test asserting that a near-miss does
not match failed, and the reason was real: the final character of a 22-character
base64 encoding of 16 bytes carries spare bits, so several distinct strings
decode to the same value and therefore to the same hash.

That made "the attacker typed the value we planted" imprecise — a variant
spelling would have triggered. `RawURLEncoding.Strict()` gives one credential
exactly one spelling.

## Honesty properties

These are security properties in this product, because misreporting coverage is
the failure mode a deception system has to avoid.

- **A credential nobody planted is not live.** `pending` exists between minting
  and the operator confirming placement, and `Live()` is false for it. Counting
  unplanted bait as coverage would overstate what the product is watching.
- **Triggering is idempotent and keeps the first use.** An attacker returning to
  a decoy reports the same credential again; overwriting the timestamp would
  move "first seen" forward every visit and mislead an investigator.
- **Revoking stops new evidence and erases none.** `CS-06` forbids silent
  deletion. A revoked credential stops raising alerts — which is the point, for
  an operator who tore down the location they planted it in — and keeps the
  record that it was used.

## Untrusted content

Two operator-authored strings are attacker-visible by design: the `username` an
attacker will see offered, and the `placement_note` recording where it was put.
Both are NFC-trimmed, control-character-rejected, and bounded on the same rules
as a decoy display name, and both remain untrusted to any renderer.

The placement note deserves one caution for whoever builds the console screen:
it describes a real location in a real system. It is operator-authored, it is
not attacker-visible, and it should not be rendered anywhere a decoy's own
content is rendered, because a note reading "left in /home/svc/.netrc on
fs-prod-01" names a production host.

## Deliberately not covered

- **Delivery to the decoy.** Decided 2026-09-13 and not yet implemented: the
  hash travels in the root-installed workload definition, with no public
  contract change (`P2-W9` section 5). The review that implementation needs is
  of that path: the definition is world-readable on the Edge, which is
  acceptable only because it carries a hash of 128-bit worthless material and
  never the secret, and the Edge's recognition rule must be the Control
  Plane's exactly.
- **Storage and API.** No table, no migration, no REST surface. All three follow
  the delivery decision, since it may change what is stored.
- **Pack recognition.** `P2-W5` and `P2-W7` present and recognise the
  credentials; each is new attacker-facing code and gets its own review.
- **Automatic placement.** The roadmap specifies manual pilot placement. An
  automated planter writing into real systems would be the most dangerous
  component in this product, and nothing here moves toward one.

## Conclusion

No new privilege, no network reach, and no handling of any real secret. The
package's security rests on the stored type having nowhere to put a plaintext,
the material being unmistakably Guardian's, and the lifecycle refusing to claim
coverage that does not exist.

The residual risk is entirely in what happens next: the moment a decoy can
recognise these credentials, the hash has to reach the Edge, and that path is
the thing to review carefully rather than this one.
