# P2-W2 nftables egress policy security review

- Review date: 2026-09-11
- Work package: P2-W2
- Decisions: NW-01, NW-02, SP-01, ADR 0010, ADR 0016
- Acceptance: AC-SEC-003, plus P2-W2's persistence and no-bypass criteria
- Scope: `apps/edge-agent/internal/privileged` — the nftables egress adapter

## What this control is for

Every other security control in this product protects Guardian. This one
protects the customer *from* Guardian: it is the reason deploying medium-
interaction decoy software is a defensible decision rather than a reckless one.
Cowrie runs code an attacker typed. If a decoy can open an outbound connection,
Guardian has installed a foothold.

So the failure to avoid is not "the policy is imperfect". It is "the policy is
reported as applied and is not enforcing anything".

## The privilege did not grow

This is the result worth stating first. `ADR 0016` had already decided on
default-deny `forward`/`output` policy, and those are host-namespace chains.
That mattered more than it appears:

Applying rules *inside* each decoy's network namespace would require `setns`,
which requires **`CAP_SYS_ADMIN`** — a capability that is close to unrestricted
root, and one the helper's service profile has no headroom for (`P2-W1` left
roughly 0.2 under the security gate's ceiling; `CAP_SYS_ADMIN` alone costs more
than that). Host-side enforcement needs only the `CAP_NET_ADMIN` that `P2-W1`
already granted.

Measured, not assumed: `systemd-analyze security --offline=yes` is unchanged at
**1.8**, and `RestrictAddressFamilies=AF_UNIX AF_NETLINK` already covers
`NETLINK_NETFILTER`. No directive in the unit changed for this package.

Host placement is also the stronger choice on its own merits. Rules inside a
namespace can be flushed by anything holding `CAP_NET_ADMIN` in that namespace;
rules in the host cannot be reached from inside a decoy at all. That is what
satisfies "decoy cannot bypass by changing container process": the enforcement
point is not in the container's reach, whatever the container becomes.

## What the policy is keyed on, and why not the obvious thing

The RPC carries a namespace name. The policy does not use it to match.

Matching on the decoy's namespace or veth interface would have required
inventing a naming convention that `P2-W3` must then honour. If `P2-W3` named
things differently, the rules would match nothing — and the adapter would still
report success, because the rules would be present and correct-looking. That is
precisely the silent-failure shape this control cannot have.

The policy is keyed instead on the `--allow-address-range` prefixes, which are:

- root-controlled startup arguments, never RPC input, so no caller can widen or
  narrow what is denied;
- the **same compiled set** that gates `EnsureAddress`, so an address Guardian
  is able to place is necessarily an address this policy covers. The two cannot
  drift apart, because they are one list.

The namespace argument still has to be allowlisted for the RPC to be accepted;
what it selects is *when* the profile is asserted, never what it says.

## Deny-shaped, so it composes

Base chains carry policy `accept` and the denial lives in the rules. This is
deliberate and is the opposite of what looks safest at a glance.

An nftables chain policy applies to every packet reaching that hook. A Guardian
table with a `drop` policy on `forward` would drop the host's own forwarded
traffic — a control that takes the customer's routing down the first time it
runs. Because a drop in *any* table wins, expressing denial in scoped rules
loses nothing: the policy still cannot be overridden by another firewall's
accept, and it no longer has an opinion about traffic that is not a decoy's.

The lab asserts both halves: the decoy source is refused, and the host's own
source to the same destination is not.

## Evidence is a packet, not a ruleset

A test that checked which rules exist would pass on a ruleset that matches
nothing, which is the characteristic failure of hand-encoded netlink. Two
conventions of `nf_tables` make it easy to produce one: integer attributes are
big-endian unlike the rest of netlink, and register contents are not — an
address loaded from a packet is raw network bytes while `ct state` is a
host-order word. Getting either backwards yields a ruleset the kernel accepts
and that never matches.

So the lab sends datagrams, with only the source address varying between the
test and control. After the policy is applied the decoy source is refused with
`EPERM` by the kernel itself. Deleting the table restores egress, is detected as
unconverged, and the following apply denies again.

## Honesty properties

- **A transaction is not a policy.** The ruleset is read back from the kernel
  after the batch is acknowledged, and a mismatch is an error rather than a
  success. The kernel saying "ok" and the rules being present are two facts.
- **A removed policy reads as removed.** Every rule carries a marker naming the
  policy version, a digest of the ranges, and its position. A flushed table, a
  partially flushed chain, an appended rule, and a different range set are each
  asserted to compare as not-applied.
- **An empty policy is never applied.** With no decoy ranges the capability
  reports `unsupported` with the cause, rather than installing rules that match
  nothing. This is the one place where a cheerful answer would be actively
  dangerous.
- **The batch is atomic.** The table is created, deleted, and recreated in one
  transaction, so replacing the policy never leaves a window with the table
  present and its rules gone.

## Residual risks

**Reboot.** nftables state is kernel state. Between boot and the first reconcile
pass there is no policy. Nothing in this package can close that, because nothing
in this package starts a decoy — it becomes an ordering requirement on `P2-W3`:
a decoy must not run before the policy is applied. Recorded here so that it is
reviewed as part of `P2-W3` rather than discovered later.

**A misconfigured allowlist.** A range containing production addresses would
have its egress denied. Same error and same answer as `P2-W1`'s review:
`--allow-address-range` takes decoy ranges only.

**Conntrack.** `ct state` requires `nf_conntrack`. On a kernel without it the
rule is refused at apply time and the adapter reports failure rather than
partial enforcement, because the batch is atomic.

**Hand-encoded protocol.** Roughly four hundred lines of netlink encoding in a
root daemon is a real maintenance surface. The alternative was a third-party
nftables library — a new supply-chain dependency in the most privileged
component in the product — or shelling out to `nft`, which the boundary forbids
outright and which a security test greps for. Hand-encoding was chosen as the
option that changes neither the dependency set nor the execution surface, and
the packet-level lab is what makes it verifiable. If the Product Owner would
rather carry the dependency, this is the trade to revisit.

## An inconsistency found while doing this, in P2-W1

`ADR 0016` says decoy addresses are bound as **`/32` identities**. The
`presence` package documents the opposite — "the prefix length is the zone's" —
and the address adapter applies whatever prefix length it is given.

A zone-length secondary creates a connected route for the whole subnet on that
interface. Where the interface already carries that subnet this is harmless;
where it does not, Guardian would claim routing for addresses it does not own.
`/32` avoids it entirely, which is what the ADR decided.

This is not a P2-W2 defect and is not changed here: it alters `presence.Address`
semantics, the reconciler, the helper driver, and both labs, and bundling it
into this commit would make both harder to review. It is currently inert because
nothing calls the reconciler. It should be corrected before anything does.

## Conclusion

No new privilege, no new dependency, no execution surface, and no contract
change. The enforcement point is outside the decoy's reach by construction, and
the policy's scope is the same list that decides what Guardian may place, so the
two cannot disagree.

The control is verified by the kernel refusing a packet, which is the only form
of evidence `AC-SEC-003` actually asks for.
