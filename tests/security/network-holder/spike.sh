#!/usr/bin/env bash
# The network-holder spike (ADR 0019).
#
# Proves the decoy datapath on a real kernel before any production code
# depends on it, with every actor limited to the capabilities it will have:
#
#   helper  CAP_NET_ADMIN, in the host's namespace
#   holder  CAP_NET_ADMIN, inside the decoy's namespace only
#   decoy   CAP_NET_BIND_SERVICE, uid 10001, in the holder's namespace
#
# Four namespaces stand in for the real topology: `zone` is the routed decoy
# segment an attacker is on, `edge` is the Edge host, `prod` is a production
# host behind the Edge's other interface, and `holder` is a decoy's network.
#
# Runs inside a throwaway privileged container; nothing here touches the host.
set -euo pipefail

apt-get update -qq >/dev/null 2>&1
apt-get install -y -qq iproute2 nftables socat procps >/dev/null 2>&1

decoy=172.30.20.99
pass=0
check() {
  local name="$1"; shift
  if "$@"; then echo "PASS  $name"; pass=$((pass + 1)); else echo "FAIL  $name" >&2; exit 1; fi
}
refuse() {
  local name="$1"; shift
  if "$@" 2>/dev/null; then echo "FAIL  $name" >&2; exit 1; else echo "PASS  $name"; pass=$((pass + 1)); fi
}
as_helper() { ip netns exec edge setpriv --bounding-set=-all,+net_admin --inh-caps=-all "$@"; }
as_holder() { ip netns exec holder setpriv --bounding-set=-all,+net_admin --inh-caps=-all "$@"; }

# --- the network before Guardian ---------------------------------------------
for ns in zone edge prod holder; do ip netns add "$ns"; done
ip link add zone0 netns zone type veth peer name edge0 netns edge
ip link add prod0 netns prod type veth peer name mgmt0 netns edge
ip -n zone addr add 172.30.20.1/24 dev zone0
ip -n edge addr add 172.30.20.10/24 dev edge0
ip -n edge addr add 10.50.0.10/24 dev mgmt0
ip -n prod addr add 10.50.0.20/24 dev prod0
for pair in zone:zone0 edge:edge0 edge:mgmt0 prod:prod0; do
  ip -n "${pair%%:*}" link set "${pair##*:}" up
  ip -n "${pair%%:*}" link set lo up
done
ip -n prod route add default via 10.50.0.10
# A server that is not a router. Whether a new namespace inherits forwarding
# depends on the host (IPv4 copies the initial namespace's setting by default),
# so the model sets it explicitly rather than assuming it.
ip netns exec edge sysctl -qw net.ipv4.conf.all.forwarding=0
check "the edge does not forward before Guardian" \
  test "$(ip netns exec edge sysctl -n net.ipv4.conf.edge0.forwarding)" = 0

# A process that owns the holder namespace, standing in for the holder task
# whose pid containerd reports.
ip netns exec holder sleep 600 &
holder_pid=$!
sleep 0.3

# --- the helper: host side, CAP_NET_ADMIN only ----------------------------------
as_helper ip link add gdnh0 type veth peer name gdnp0
check "the helper moves the veth peer into the decoy namespace by pid" \
  as_helper ip link set gdnp0 netns "$holder_pid"
as_helper ip link set gdnh0 up
as_helper ip route add "$decoy/32" dev gdnh0
as_helper ip neigh add proxy "$decoy" dev edge0
# Production sets these through RTM_SETLINK / IFLA_INET_CONF, because the
# helper's /proc/sys is read-only. sysctl here needs the same capability.
as_helper sysctl -qw net.ipv4.conf.edge0.forwarding=1
as_helper sysctl -qw net.ipv4.conf.gdnh0.forwarding=1
as_helper sysctl -qw net.ipv4.conf.gdnh0.proxy_arp=1

# --- the holder: inside the decoy namespace, CAP_NET_ADMIN only ------------------
as_holder ip link set lo up
as_holder ip link set gdnp0 name eth0
as_holder ip addr add "$decoy/32" dev eth0
as_holder ip link set eth0 up
check "the holder configures the decoy namespace from inside" \
  as_holder ip route add default dev eth0

# --- the decoy: uid 10001, NET_BIND_SERVICE only, in the holder's namespace ---------
ip netns exec holder setpriv --reuid=10001 --regid=10001 --clear-groups \
  --bounding-set=-all,+net_bind_service --inh-caps=-all,+net_bind_service \
  --ambient-caps=-all,+net_bind_service \
  socat TCP-LISTEN:22,fork,reuseaddr SYSTEM:'echo "SSH-2.0-decoy peer=$SOCAT_PEERADDR"' &
sleep 0.5

# --- what an attacker in the zone sees -----------------------------------------------
banner="$(ip netns exec zone timeout 3 socat -T2 - "TCP:$decoy:22" </dev/null || true)"
check "an attacker in the zone reaches the decoy" grep -q '^SSH-2.0-decoy' <<<"$banner"
check "the decoy sees the attacker's own address, not the edge's" grep -q 'peer=172.30.20.1$' <<<"$banner"
edge_mac="$(ip -n edge -br link show edge0 | awk '{print $3}')"
check "the attacker resolved the decoy to the edge by proxy ARP" \
  grep -q "$edge_mac" <<<"$(ip -n zone neigh show "$decoy")"

# --- one-way reachability ---------------------------------------------------------
# TCP would prove nothing here: production's replies come back through mgmt0,
# which does not forward, so a handshake fails whether or not a packet got in.
# A datagram that arrives is the honest test, because an attack does not need a
# reply to do damage.
ip netns exec prod socat -u UDP-RECV:9 OPEN:/tmp/prod.udp,creat,append &
ip netns exec zone socat -u UDP-RECV:9 OPEN:/tmp/zone.udp,creat,append &
sleep 0.3
# Polls for three seconds. The first datagram to a new destination waits on
# proxy ARP, which the kernel delays by about 0.8s by default, and a negative
# result is only worth anything after the full wait.
arrived() {
  for _ in $(seq 30); do
    grep -q "$2" "$1" 2>/dev/null && return 0
    sleep 0.1
  done
  return 1
}
send_from_zone() { ip netns exec zone socat -u - "UDP-SENDTO:$1:9" <<<"$2"; }
send_from_decoy() {
  ip netns exec holder setpriv --reuid=10001 --regid=10001 --clear-groups \
    --bounding-set=-all --inh-caps=-all socat -u - "UDP-SENDTO:$1:9" <<<"$2"
}
ip -n zone route add 10.50.0.0/24 via 172.30.20.10

# --- the two risks, shown before they are closed -------------------------------------
send_from_zone 10.50.0.20 zone-before
check "without a restriction, forwarding lets the zone send into production" \
  arrived /tmp/prod.udp zone-before
send_from_decoy 10.50.0.20 decoy-before
check "without the egress policy, a decoy can send into production" \
  arrived /tmp/prod.udp decoy-before

# --- the policy: P2-W2's egress deny plus the forwarding restriction -------------------
ip netns exec edge nft -f - <<NFT
table ip guardian_decoy {
  chain guardian_egress {
    ct state established,related accept
    drop
  }
  chain guardian_forward {
    type filter hook forward priority 0; policy accept;
    ip saddr $decoy/32 jump guardian_egress
    iifname "edge0" ip daddr != $decoy/32 drop
  }
}
NFT

send_from_zone 10.50.0.20 zone-after
refuse "the zone can no longer send into production through the edge" \
  arrived /tmp/prod.udp zone-after
send_from_decoy 10.50.0.20 decoy-after
refuse "the decoy cannot send into production" arrived /tmp/prod.udp decoy-after
send_from_decoy 172.30.20.1 decoy-to-zone
refuse "the decoy cannot send back into the zone" arrived /tmp/zone.udp decoy-to-zone
banner="$(ip netns exec zone timeout 3 socat -T2 - "TCP:$decoy:22" </dev/null || true)"
check "the attacker still reaches the decoy, and gets an answer, with the policy in force" \
  grep -q '^SSH-2.0-decoy' <<<"$banner"

# --- what the holder and decoy cannot do ----------------------------------------------
refuse "the decoy cannot change its own network" \
  ip netns exec holder setpriv --reuid=10001 --regid=10001 --clear-groups \
  --bounding-set=-all,+net_bind_service --inh-caps=-all ip addr add 172.30.20.98/32 dev eth0
refuse "the holder cannot move its interface into the host's network" \
  as_holder ip link set eth0 netns 1

echo "network-holder spike: $pass properties hold"
