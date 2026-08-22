#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="$repo_root/deploy/lab/compose.yaml"
if command -v docker >/dev/null 2>&1 && docker version >/dev/null 2>&1; then
  docker_bin=docker
elif command -v docker.exe >/dev/null 2>&1; then
  docker_bin=docker.exe
else
  echo 'Docker CLI is required (docker or docker.exe).' >&2
  exit 1
fi
if [[ "$docker_bin" = "docker.exe" ]] && command -v wslpath >/dev/null 2>&1; then
  compose_file="$(wslpath -w "$compose_file")"
fi
compose=("$docker_bin" compose -f "$compose_file" -p guardian-lab)
secondary_address=172.30.20.99/32
secondary_ip=172.30.20.99

cleanup() {
  "${compose[@]}" down --volumes --remove-orphans >/dev/null
}
trap cleanup EXIT

"$repo_root/tools/lab-reset.sh" >/dev/null
edge_zone_b="$("${compose[@]}" exec -T edge-agent sh -ec 'ip -4 route show 172.30.20.0/24' | awk '{print $3}' | head -n1)"
test_host_zone_b="$("${compose[@]}" exec -T test-host sh -ec 'ip -4 route show 172.30.20.0/24' | awk '{print $3}' | head -n1)"
test -n "$edge_zone_b"
test -n "$test_host_zone_b"

"${compose[@]}" exec -T edge-agent sh -ec "ip addr del $secondary_address dev $edge_zone_b 2>/dev/null || true"
"${compose[@]}" exec -T edge-agent sh -ec "arping -D -I $edge_zone_b -c 2 -w 2 $secondary_ip >/dev/null 2>&1"
"${compose[@]}" exec -T edge-agent sh -ec "ip addr add $secondary_address dev $edge_zone_b"

placed_count="$("${compose[@]}" exec -T edge-agent sh -ec "ip -o -4 addr show dev $edge_zone_b" | grep -F -c "$secondary_address" || true)"
test "$placed_count" -eq 1
"${compose[@]}" exec -T attacker sh -ec "ping -c 1 -W 1 $secondary_ip >/dev/null"

"${compose[@]}" exec -T edge-agent sh -ec "ip addr replace $secondary_address dev $edge_zone_b"
"${compose[@]}" exec -T edge-agent sh -ec "ip addr replace $secondary_address dev $edge_zone_b"
reconciled_count="$("${compose[@]}" exec -T edge-agent sh -ec "ip -o -4 addr show dev $edge_zone_b" | grep -F -c "$secondary_address" || true)"
test "$reconciled_count" -eq 1

if "${compose[@]}" exec -T test-host sh -ec "arping -D -I $test_host_zone_b -c 2 -w 2 $secondary_ip >/dev/null 2>&1"; then
  echo 'duplicate-address probe did not detect the placed secondary address' >&2
  exit 1
fi

"${compose[@]}" exec -T edge-agent sh -ec "ip addr del $secondary_address dev $edge_zone_b"
cleaned_count="$("${compose[@]}" exec -T edge-agent sh -ec "ip -o -4 addr show dev $edge_zone_b" | grep -F -c "$secondary_address" || true)"
test "$cleaned_count" -eq 0
"${compose[@]}" exec -T test-host sh -ec "arping -D -I $test_host_zone_b -c 2 -w 2 $secondary_ip >/dev/null 2>&1"

"${compose[@]}" exec -T edge-agent sh -ec 'if ip route show default | grep -q .; then exit 1; fi; nft list chain inet guardian_lab output | grep -F "policy drop" >/dev/null; if curl --connect-timeout 1 --silent http://198.51.100.1 >/dev/null 2>&1; then exit 1; fi'

echo 'P0-W5 secondary-IP spike passed: placement, routed reachability, reconcile, conflict detection, cleanup, and egress denial.'
