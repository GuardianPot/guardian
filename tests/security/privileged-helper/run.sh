#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$repo_root"
verify_log="$(mktemp)"
trap 'rm -f "$verify_log"' EXIT

GOWORK=off go -C apps/edge-agent vet ./internal/privileged ./internal/privclient ./cmd/edge-privd
GOWORK=off go -C apps/edge-agent test -count=1 ./internal/privileged ./internal/privclient ./cmd/edge-privd

if find apps/edge-agent/cmd/edge-privd apps/edge-agent/internal/privileged apps/edge-agent/internal/privclient \
  -type f -name '*.go' ! -name '*_test.go' -print0 | \
  xargs -0 grep -En '"os/exec"|exec\.Command|syscall\.Exec|unix\.Exec'; then
  echo 'Privileged-helper production code contains a process or shell execution primitive.' >&2
  exit 1
fi

if grep -En '^[[:space:]]*(string|bytes)[[:space:]]+(command|executable|path|script|socket|ruleset)[[:space:]]*=' \
  proto/guardian/privileged/v1/privileged.proto; then
  echo 'Privileged-helper RPC contract exposes a forbidden command/path/raw-ruleset field.' >&2
  exit 1
fi

service_file="$repo_root/deploy/edge-agent/guardian-edge-privd.service"
grep -Fxq 'User=root' "$service_file"
grep -Fxq 'Group=guardian-edge' "$service_file"
grep -Fxq 'NoNewPrivileges=yes' "$service_file"
grep -Fxq 'CapabilityBoundingSet=' "$service_file"
grep -Fxq 'AmbientCapabilities=' "$service_file"
grep -Fxq 'PrivateNetwork=yes' "$service_file"
grep -Fxq 'RestrictAddressFamilies=AF_UNIX' "$service_file"
grep -Fxq 'SystemCallFilter=@system-service' "$service_file"
grep -Fxq 'IPAddressDeny=any' "$service_file"

systemd-analyze verify \
  deploy/edge-agent/guardian-edge-privd.service \
  deploy/edge-agent/guardian-edge.service 2>"$verify_log" || true
if grep -Ev '^guardian-edge(-privd)?\.service: Command /usr/(libexec/guardian-edge/guardian-edge-privd|bin/guardian-edge) is not executable: No such file or directory$' \
  "$verify_log" | grep -q .; then
  cat "$verify_log" >&2
  exit 1
fi
security_report="$(SYSTEMD_COLORS=0 systemd-analyze security --offline=yes \
  deploy/edge-agent/guardian-edge-privd.service || true)"
grep -Eq 'Overall exposure level .*: [0-2](\.[0-9]+)? OK' <<<"$security_report"
grep -E 'Overall exposure level' <<<"$security_report"

echo 'P1-W8 privileged-helper typed boundary, abuse, and service-profile checks passed.'
