# P2-W4 decoy manifest schema security review

- Review date: 2026-09-08
- Work package: P2-W4
- Decisions: DC-01, DC-11, DC-12, INT-01, SP-02, ADR 0009
- Acceptance: P2-W4's two named criteria, plus AC-SEC-001, AC-SEC-002,
  AC-SEC-003
- Scope: `schemas/decoy/v1/decoy-manifest.schema.json`, the four committed pack
  manifests, and the loader in `apps/control-plane/internal/deception/manifest.go`

## What this package actually is

`P2-W15` gave the decoy record nowhere to put a runtime detail. No image,
command, argument, mount, capability, or port field exists in the API, on the
wire, or in the database, and the API test rejects each by name. That was the
right shape and it left a gap: Guardian knew a pack's family and interaction
level and nothing else, which is not enough to start a container with.

This package is that gap, filled deliberately. It is the door a runtime detail
comes through, so the review question is not "is the schema correct" but **what
can a manifest ask for, and what happens when it asks for something it should
not get**.

## The privilege vocabulary

One capability: `NET_BIND_SERVICE`. It is there because a decoy presenting SSH
on 22 or HTTP on 80 has no other way to bind the port that makes it convincing,
and a decoy on a random high port is not a decoy anyone will probe.

Everything else about the runtime baseline was settled by `ADR 0009` and the
`P0-W6` review — dropped capabilities, an exact image digest, no network, no
bind mounts, no runtime sockets, a read-only root, bounded resources, and
`no-new-privileges` — and **no manifest field can express a departure from any
of it, because no such field exists**. There is no free-form capability string,
no raw OCI spec passthrough, no host path, no volume, no `privileged` flag, and
no runtime-socket reference. `additionalProperties: false` at every level of the
schema, and `DisallowUnknownFields` in the loader, mean a manifest that carries
one is refused rather than having it quietly ignored.

That last point is the one worth stating twice. A request outside the set is
**rejected, not filtered**. Filtering would leave a pack author believing they
had been granted something they had not, and the pack would then fail at
runtime in a way that looks like the runtime's fault. The rejection also names
the capability it refused, because a pack author who cannot see which request
was the problem will guess.

`ErrManifestPrivilege` is its own sentinel for the same reason: a manifest
asking for authority Guardian will not grant is the one validation failure that
is a security event rather than a typo, and a caller should be able to treat it
differently.

Tested against `CAP_SYS_ADMIN`, `CAP_NET_ADMIN`, `CAP_NET_RAW`, `CAP_SYS_PTRACE`,
`CAP_DAC_OVERRIDE`, the unprefixed `SYS_ADMIN`, `ALL`, and the empty string, and
against the unknown properties `image`, `command`, `entrypoint`, `mounts`,
`privileged`, and `host_network`.

## Egress (AC-SEC-003)

`policy` has one value, `deny`, written out rather than assumed so a reader of
the manifest sees it and so that adding a second value is a schema change with
a review attached. Exceptions are enumerated, capped at four, and each must
name an RFC1918 destination, a port, a protocol, and a reason. A public
destination cannot be expressed — the pattern forbids it — and there is no
wildcard and no "allow all".

**The absence of a declaration means nothing is permitted, never everything.**
That inversion is where this kind of schema usually goes wrong.

Every committed manifest has an empty allow list, and a test asserts it for all
four rather than trusting review. `P2-W5`'s Cowrie is the reason this is not a
formality: it is the only medium-interaction pack in the MVP, it is real
software an attacker interacts with, and this field is what keeps it from
reaching the internet if it is subverted.

The manifest declares intent. `P2-W2` enforces it with nftables, and this
review does not claim otherwise: a declaration nobody applies is documentation.
The ordering in the agreed plan has `P2-W2` before every attacker-facing pack
for exactly that reason.

## The version mismatch

`ErrManifestVersion` is separate from every other validation failure, and the
version is checked **before anything else is read**. A manifest written against
a contract this build does not implement must not be partially interpreted:
every field below the version is read under rules that may not apply to it, and
reporting a field error for such a document sends the reader after the wrong
thing entirely.

Tested by setting the schema to a later version, an earlier one, a different
contract's identifier, and the empty string — each with a deliberately broken
port list, asserting the version error is what comes back rather than the field
error.

## Two things that could still go wrong

**The manifest is not verified.** It is a checked declaration. Signature
enforcement over pack artefacts is `AC-SEC-004`, a Phase 5 gate, and nothing
here verifies that a pack matches its manifest — there is no pack yet. The
integrity property this package does provide is that the manifest is committed
and reviewable, so a change to what a decoy is granted appears in a diff.

**Nothing consumes it yet.** `P2-W3` will build a runtime from these fields and
`P2-W14` will probe with them. Until then the schema and the loader are
enforced but unexercised by a runtime, and a mismatch between what the manifest
says and what the runtime does is a `P2-W3` risk this review cannot close.

## `pack_digest` stays null

Worth recording because both `decoys/README.md` and this package's name
suggested otherwise. The contract states the condition as two things: null
"until `P2-W4` defines a manifest **and a pack exists to hash**". This satisfies
the first only.

Hashing the manifest and putting it in `pack_digest` would record the digest of
the *description* rather than of the artifact — an identity claim about
something that has not been built. That is the same failure the field was
written to avoid, so the digest stays null and the README was corrected to say
which condition remains open.

## Conclusion

No new privilege, credential path, or network authority. The package narrows
rather than widens: before it, "what a pack may do" was undefined and therefore
unbounded in practice; after it, the answer is one capability, no egress, a
connection-only health probe, and required resource limits, with every other
possibility unrepresentable rather than merely disallowed.

The residual risk is that the vocabulary is small enough to be inconvenient,
and the pressure to widen it will come from a pack that does not work. That
pressure should arrive as a schema change with this review attached, which is
what the canonical-contract fixture is for: it pins the capability enum and the
egress policy, so widening either fails the lane rather than passing quietly.
