# P2-W1 routed presence driver security review

- Review date: 2026-09-08, extended 2026-09-11 for the netlink adapter
- Work package: P2-W1
- Decisions: DC-12, SP-01, SP-02, ADR 0009
- Acceptance: AC-ON-004, plus P2-W1's reboot-reconcile and no-orphan-IP criteria
- Scope: `apps/edge-agent/internal/presence` (reconciler, conflict probe, helper
  driver), `apps/edge-agent/internal/privileged` (netlink address adapter), and
  `deploy/edge-agent/guardian-edge-privd.service`

The second review date is the important one. The first pass covered code that
held no privilege at all; this one covers root code that changes a customer's
network, and a service profile that had to be widened to let it.

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

## The netlink adapter: what the privilege actually grew by

This is the part of the package that carries real risk, and the honest summary
is that the service profile changed more than the code did.

### The profile was not merely restrictive, it was prohibitive

The shipped helper ran as root with an **empty capability bounding set**,
`PrivateNetwork=yes`, and `RestrictAddressFamilies=AF_UNIX`. Each of those
independently makes a netlink address change impossible: no `CAP_NET_ADMIN`, no
view of the host's interfaces, and no `AF_NETLINK` socket. Filling in the
adapter without touching the unit would have shipped dead code that reported
`netlink-unavailable` forever.

The widening is exactly three directives:

| Was | Is | Why |
|---|---|---|
| `CapabilityBoundingSet=` | `CapabilityBoundingSet=CAP_NET_ADMIN` | The only capability an address change needs |
| `PrivateNetwork=yes` | `PrivateNetwork=no` | The host's interfaces are the thing being changed |
| `RestrictAddressFamilies=AF_UNIX` | `… AF_UNIX AF_NETLINK` | The transport |

`systemd-analyze security --offline=yes` moves from **1.3 to 1.8**, measured
rather than estimated, and `PrivateMounts=yes` was added to keep headroom under
the gate's ceiling of 2.0. Every one of the three lines is asserted verbatim by
`tests/security/privileged-helper/run.sh`, which now also rejects a *second*
occurrence of any of them — a duplicate directive unions with the first, so an
exact match on one line is only a real bound if there is exactly one line.

`PrivateNetwork=no` is the largest single concession and deserves naming: the
helper can now see the host's network. What stops it using that is
`IPAddressDeny=any`, a newly added `SocketBindDeny=any`, and the address-family
restriction, which together leave it able to change addressing while unable to
send a packet. `AF_INET`, `AF_INET6`, and `AF_PACKET` remain forbidden, and the
gate fails if any of them appears.

### No process execution, at the cost of encoding netlink by hand

The obvious implementation is `ip addr add`. It was rejected: an `exec` path
inside a process holding `CAP_NET_ADMIN` is one unchecked string away from being
an arbitrary-command path, and the boundary test greps production code for that
primitive. The adapter therefore encodes `RTM_NEWADDR` and `RTM_DELADDR`
directly.

The message parser is written in this repository rather than taken from
`syscall`, because that one reinterprets the receive buffer through unsafe
pointer casts. This one checks every kernel-supplied length against what
actually arrived before using it as a bound, and refuses the datagram otherwise.
A truncated read, an oversized declared length, and a zero-length attribute are
each asserted.

Replies are filtered on three things: the sending port must be 0 (the kernel),
and the sequence number and port ID must be the ones asked about. The socket
carries a receive timeout, so a kernel that never answers stops a root process
rather than parking it.

### The refusal that matters: Guardian never removes an address it did not add

Every address the adapter places carries the IPv4 label `<interface>:gdn`. The
label is the kernel's own ownership marker, visible in `ip -4 addr show`, and it
is checked in both directions: an address on the interface without it is the
host's, and both placing over it and removing it are refused with
`address-held-by-host`.

This is what stands between a misconfigured `--allow-address-range` and a
customer outage. The allowlist is the primary control and it is unchanged; the
label is the second one, and it is enforced by the kernel's own record rather
than by any state this helper keeps — which matters, because the helper is
deliberately stateless across restarts.

The trade is a length limit. A label is capped at 15 characters and convention
requires it to start with the interface name, so an interface name longer than
11 characters cannot carry one. Rather than place an unlabelled address, the
adapter refuses with `interface-name-too-long-to-label`. A loud refusal on an
unusual interface name is better than an address Guardian could not later prove
was its own; every conventional name (`eth0`, `ens192`, `enp0s31f6`, `br-decoy`)
fits.

IPv6 is refused for the same reason: labels are an IPv4 mechanism, so an IPv6
decoy address would be unmarked. `presence.Address` already accepted only
private IPv4, so this narrows nothing in practice.

### One residual risk, and it is the operator's to avoid

Deleting the *primary* address of an IPv4 subnet makes the kernel remove the
secondaries in that subnet with it. Guardian's addresses are added after the
host's and are therefore secondaries, so removing one cascades to nothing. The
exception is a host address added to an interface *after* Guardian's, in a
subnet the operator also allowlisted as a decoy range — a configuration that
already requires putting production addressing inside a decoy range. The host
mitigation is `net.ipv4.conf.<if>.promote_secondaries=1`. It is recorded here
rather than coded around, because no check inside the helper can distinguish
that case from a legitimate one.

### Evidence

`task presence:netlink` runs the adapter against a real kernel in a container
holding `CAP_NET_ADMIN` and nothing else, in a throwaway network namespace: an
address is added and observed carrying Guardian's label, adding it again reports
no change, it is removed, removing it again reports no change, and an address
staged with a foreign label is refused in both directions and survives intact.
The interface's pre-existing addresses are compared before and after, and the
placement is confirmed a second time through `net.Interface.Addrs`, which
reaches the kernel by a netlink implementation this repository did not write.

## What is not covered

**The probe is a point-in-time answer.** A host switched on between the probe
and the apply would collide. The kernel's own duplicate address detection would
narrow the window; the adapter does not use it, and this stays a known
limitation rather than a solved problem.

**Nothing calls the reconciler yet.** `NewHelperDriver` joins the two halves and
is asserted to satisfy the real client's signature, but the Edge has no source
of desired decoy addresses. The chain is complete and idle. Nothing places an
address today without a caller that does not yet exist, which is worth knowing
when reading the risk above.

**Routing.** The adapter attaches the broadcast address iproute2 would and
nothing else. Decoy reachability beyond the local subnet is not this package's.

**The other three operations.** `ApplyNftablesPolicy`, `ReconcileContainer`, and
`EnsureNetworkNamespace` still report `phase-2-adapter-not-implemented`. Each is
a separate privileged surface with its own review, and the capability report is
per-operation precisely so that filling one does not imply the others.

## Conclusion

The unprivileged half of this package is unchanged and still needs no privilege.
The privileged half grants exactly one capability, and the profile that carries
it has been measured, tightened where it could be, and pinned line by line.

The failure this package exists to prevent — Guardian taking an address a
production host was using — now has two independent controls: the operator's
allowlist, and the kernel's own record of which addresses Guardian labelled. The
second one is new, and it is the reason the adapter is safer than the RPC
boundary alone made it.

The profile has roughly 0.2 of headroom under the security gate's ceiling. That
is deliberate: `P2-W2` and `P2-W3` each want another capability, and neither
will slip through without a decision.
