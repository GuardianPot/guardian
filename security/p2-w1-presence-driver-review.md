# P2-W1 routed presence driver security review

- Review date: 2026-09-08
- Work package: P2-W1
- Decisions: DC-12, SP-01, SP-02, ADR 0009
- Acceptance: AC-ON-004, plus P2-W1's reboot-reconcile and no-orphan-IP criteria
- Scope: `apps/edge-agent/internal/presence` — the reconciler and the
  unprivileged conflict probe. The netlink adapter is **not** in scope: it is
  not written, and section 5 says why.

## The risk this package carries

Guardian puts an address on a network the customer already depends on. The
failure that matters is not a decoy that does not work — it is a decoy that
takes an address a production host was using. A deception product that causes
the outage it was bought to prevent has failed in the worst available way, and
every rule below exists for that.

## The conflict probe, and why it needs no privilege

`AC-ON-004` requires a deployment onto an address another host answers on to be
refused fail-safe, with the existing host unaffected. Answering that question
looks like it needs an ARP probe, which needs a raw socket, which needs root and
a new operation on the privileged helper's contract.

It does not. The kernel already resolves neighbours on demand: sending one
datagram to an on-link address makes it ask, and the answer lands in
`/proc/net/arp`, which is world-readable. Complete entry with a hardware
address means something replied; an entry that never completed means nothing
did.

Verified on a live segment as **uid 1000 with `--cap-drop=ALL` and
`no-new-privileges`**: the default gateway reads in use, an unused address in
the same subnet reads free. `task presence:probe` reproduces it against the
production `NeighbourProbe` rather than a spike.

That this needs no privilege is the most valuable result in the package. The
alternative was a new raw-socket capability inside the root helper, which would
have widened the privileged surface permanently for one question.

### Is this scanning?

No, and the distinction is worth being precise about because the product
elsewhere forbids scanning outright.

The probe takes **one address per call** — an address the operator already chose
and the Control Plane already validated into a zone — sends **one datagram**,
and reads a local file. There is no range walk, no port list, no service
detection, and no code path that could become one: the function signature takes
a single `Address` and returns a single answer.

What it emits on the wire is one UDP datagram to the discard port, which is
less than a `ping`. The ARP request the kernel sends as a side effect is
broadcast traffic every host on the segment already produces constantly.

The alternative — placing an address without checking — is the behaviour that
would actually harm the network.

## Fail-safe direction

Three answers are possible and only one of them takes the address.

| Probe result | Reconciler |
|---|---|
| another host answered | refuse, `refused_conflict` |
| could not check | refuse, `refused_unverified` |
| checked, nothing answered | apply |

"Could not check" refusing is the load-bearing one. It covers a host with no
readable neighbour cache, a table read that failed, a datagram that could not be
sent, and a platform that is not Linux — a developer machine cannot claim an
address. Every one of those is asserted by test.

The default `ConflictProbe`, if a caller supplies none, is the one that cannot
check. A reconciler constructed carelessly refuses everything rather than
applying blind.

## No orphan address

The acceptance criterion is that a failed deploy leaves no orphan IP. The window
in which one can exist is between a partial apply and the next reconcile, so the
release is issued in the same pass rather than deferred. An apply the driver
will not confirm is treated as a failure and released too — the same rule the
decoy observed state follows: absence of confirmation is never a claim of
presence.

Reservations are persisted because an orphan outlives the process that made it.
A reconciler that kept its holdings in memory would, after a restart, be unable
to release an address it had applied, and that address would stay on the
interface until the host went down. Asserted by a test that runs a second
reconciler over the first one's reservations.

Removals run before applies, because the Control Plane permits an address to
move between decoys and applying the new holder first would put a duplicate on
the wire.

## What is not covered

**The netlink adapter is not written.** `EnsureAddress` in the privileged helper
is validated, allowlisted, audited, and idempotent, and its adapter is
`UnsupportedAdapter` — `P1-W8` built the boundary and left the implementation
out deliberately. Filling it is new root code that mutates host networking, and
`AGENTS.md` names that a stop-and-ask. No contract change is involved; the RPC
already exists.

Until it lands, this package decides correctly and applies nothing. The driver
reports `unsupported` in that state and never `present`, so nothing downstream
can mistake a decided address for a placed one.

**The probe is a point-in-time answer.** A host that is switched on after the
probe and before the apply would collide. Narrowing that window further needs
the kernel's own duplicate address detection, which is the adapter's business
when it exists — a note for `P2-W1`'s second half rather than a gap in this one.

**Routing and interface binding.** The roadmap lists them; they are the
adapter's, not the driver's, and the driver's `Address` already carries the
interface the adapter will bind to.

## Conclusion

No new privilege. The package's one externally visible behaviour is a single
datagram to an operator-chosen address, and it exists to avoid the far larger
harm of taking that address blind. The privileged surface is unchanged, and the
capability that looked like it would have to grow — a raw socket in the root
helper — turned out not to be needed at all.
