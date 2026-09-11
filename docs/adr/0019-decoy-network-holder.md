# ADR 0019: Decoy network holder and routed /32 delivery

- Status: Accepted
- Date: 2026-09-11
- Owner approval: `@sinanganiz`, 2026-09-11, on the recommendation recorded in
  `docs/work-packages/phase-2/P2-W3.md` section 7
- Supersedes in part: [ADR 0016](0016-routed-secondary-ip-placement.md) — where
  a decoy address lives. Conflict detection, reconciliation, cleanup, and the
  default-deny egress policy stand.
- Decision refs: `NW-01`, `NW-02`, `SP-01`, `SP-02`
- Acceptance refs: `AC-SEC-002`, `AC-SEC-003`, `AC-ON-004`

## Context

ADR 0016 binds a decoy's `/32` on the Edge's decoy-side interface, so the Edge
host owns the address. That was decided before decoys ran in containers. With
`P2-W3` a decoy is a container in its own network namespace, and an address the
host owns is terminated by the host: nothing reaches the container.

The privileged helper holds `CAP_NET_ADMIN` and nothing else, and the security
gate leaves roughly 0.2 of headroom under its exposure ceiling. Any design that
needs more is a decision in its own right.

## Options considered

1. **The helper creates each decoy's namespace.** Needs `CAP_SYS_ADMIN`, which
   is close to unrestricted root and exceeds the headroom. The helper's systemd
   sandbox also gives it a private mount namespace, so a namespace it pins by
   bind mount is invisible to containerd.
2. **The runtime creates the namespace and the helper configures it.** Tested:
   moving a veth into another namespace needs only `CAP_NET_ADMIN`, but
   configuring anything inside it needs `CAP_SYS_PTRACE` to open the namespace
   and `CAP_SYS_ADMIN` to join it. More privilege than option 1.
3. **The host keeps the address and DNATs to the container.** The container's
   traffic then leaves with an internal source address, outside the decoy
   ranges `P2-W2`'s egress policy is keyed on, and the control stops matching.
4. **A macvlan in the decoy's namespace.** Its traffic never crosses the host's
   netfilter, so egress cannot be enforced where a decoy cannot reach it.
5. **A network holder per decoy.** Chosen.

## Decision

Each decoy has a **holder**: a Guardian container that owns the decoy's network
namespace and does nothing else.

- The holder runs a small static Guardian binary with `CAP_NET_ADMIN` inside its
  own namespace only, as a non-root uid. It brings up `lo`, takes the veth peer,
  names it `eth0`, assigns the decoy's `/32`, installs a default route out of
  it, and then waits. It listens on nothing.
- Its root is an empty snapshot with the binary bind-mounted read-only from the
  Edge package. There is no holder image to build, sign, or pull; the binary
  has the same provenance as the helper. The decoy rule "no host-sourced mount"
  is unchanged for decoys — the holder is not attacker-facing, and a decoy does
  not share its mount namespace.
- The helper, in the host namespace with `CAP_NET_ADMIN`: creates a veth pair,
  moves the peer into the holder's namespace by pid, brings its end up with
  `proxy_arp`, routes the decoy's `/32` to it, and installs a proxy neighbour
  entry for the address on the allowlisted decoy-side interface. It enables
  forwarding on exactly those two interfaces, through netlink, because its
  `/proc/sys` is read-only.
- The decoy joins the holder's network namespace. That is the one namespace
  path Guardian's spec may set, and only to a holder the helper has verified is
  running before and after the join.
- The decoy's address therefore lives inside the decoy's namespace. The host
  owns nothing but a route, a neighbour-proxy entry, and a veth.

The default-deny egress policy gains one rule: forwarded traffic arriving on a
decoy-side interface is dropped unless it is addressed to a decoy.

## Consequences

- **The Edge becomes a router, for decoy addresses only.** Forwarding on the
  decoy-side interface is what lets the host deliver to a container, and on its
  own it would also let anything in that zone use the Edge as a gateway into
  whatever the Edge can reach. The new rule restores the pre-Guardian behaviour
  for everything that is not a decoy. The spike shows the risk with the rule
  absent and closed with it present.
- **Ordering.** Forwarding is never enabled until the egress policy, including
  the new rule, is installed. The egress ordering `P2-W3` already enforces for
  starting decoys applies to enabling forwarding too.
- **`P2-W1` changes what "present" means.** `EnsureAddress` stops adding a local
  address and installs the proxy neighbour instead. Its ownership marker moves
  from the IPv4 address label to the neighbour entry's protocol attribute; the
  refusal to touch what Guardian did not create stays. The conflict probe is
  unchanged.
- **A timing tell to remove.** The kernel delays proxy-ARP replies by up to
  0.8s by default; a real host answers at once. The helper sets the decoy-side
  interface's proxy delay to zero.
- **Holder and decoy restart together.** A decoy's namespace belongs to its
  holder. If the holder is replaced, the decoy is restarted into the new one.

## Security and failure behavior

- No new capability for the helper, no service directive change.
- The holder's `CAP_NET_ADMIN` is scoped to a namespace containing only the
  decoy; it cannot reach the host's network (asserted). A decoy gains nothing
  from the holder: separate pid and mount namespaces, no shared process.
- The pid the decoy joins is re-checked after the join and before the decoy
  starts. A pid reused by a host process between the two would otherwise put a
  decoy on the host's network; if the holder is not the same running task on
  both sides, the decoy task is deleted unstarted.
- Removal is the reverse order: decoy, host route and neighbour entry, veth,
  holder. Forwarding on the decoy-side interface is left enabled while any
  decoy uses it and restored when the last one goes.

## Evidence

`tests/security/network-holder/spike.sh`, run by `task privileged:network-spike`,
on a real kernel with each actor limited to its production capability set:

- the helper moves the veth peer into the decoy's namespace by pid with
  `CAP_NET_ADMIN` alone, and the holder configures the namespace from inside
  with `CAP_NET_ADMIN` alone;
- an attacker in the zone reaches the decoy, resolved to the Edge by proxy ARP,
  and the decoy sees the attacker's own address;
- before the policy, the zone can send into production through the Edge and a
  decoy can send into production — both shown with one-way datagrams, because
  a TCP handshake would fail on the return path whether or not the packet got
  in;
- with the policy, neither can, a decoy cannot send back into the zone, and the
  attacker still gets an answer;
- the decoy cannot change its own network, and the holder cannot move its
  interface into the host's.
