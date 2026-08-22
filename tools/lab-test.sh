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

forwarding="$("${compose[@]}" exec -T edge-agent sh -ec 'cat /proc/sys/net/ipv4/ip_forward')"
test "$forwarding" = "1"
"${compose[@]}" exec -T edge-agent sh -ec 'if ip route show default | grep -q .; then exit 1; fi; ip route show; ping -c 1 -W 1 172.30.0.20 >/dev/null'
"${compose[@]}" exec -T attacker sh -ec 'if ip route show default | grep -q .; then exit 1; fi; ip route show; ping -c 1 -W 1 172.30.20.10 >/dev/null; curl --fail --silent http://172.30.20.10:8080/ | grep -F guardian-lab-test-host >/dev/null'

if "${compose[@]}" exec -T attacker sh -ec 'ping -c 1 -W 1 172.30.0.20 >/dev/null'; then
  echo 'attacker unexpectedly reached the management-only control-plane address' >&2
  exit 1
fi

echo 'P0-W4 lab test passed: forwarding, routed access, HTTP reachability, and management isolation.'
