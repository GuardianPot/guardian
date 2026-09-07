#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)"
compose_file="$repo_root/deploy/control-plane/compose.yaml"
project_name="guardian-p2-w15-test-$$"

cleanup() {
  case "$project_name" in
    guardian-p2-w15-test-*)
      docker compose --file "$compose_file" --project-name "$project_name" down --volumes --remove-orphans >/dev/null 2>&1 || true
      ;;
    *) echo "refusing unexpected Compose project cleanup: $project_name" >&2 ;;
  esac
}
trap cleanup EXIT

free_port() {
  python3 - <<'PY'
import socket
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
}

random_secret() {
  tr -d '-' </proc/sys/kernel/random/uuid
}

export GUARDIAN_POSTGRES_DB=guardian
export GUARDIAN_POSTGRES_USER=guardian
export GUARDIAN_POSTGRES_PASSWORD="guardian-decoy-$(random_secret)"
export GUARDIAN_POSTGRES_PORT="${GUARDIAN_P2_W15_POSTGRES_PORT:-$(free_port)}"

docker compose --file "$compose_file" --project-name "$project_name" up --detach --wait --wait-timeout 90

export GUARDIAN_TEST_DATABASE_URL="postgres://${GUARDIAN_POSTGRES_USER}:${GUARDIAN_POSTGRES_PASSWORD}@127.0.0.1:${GUARDIAN_POSTGRES_PORT}/${GUARDIAN_POSTGRES_DB}?sslmode=disable"

GOWORK=off go -C "$repo_root/apps/control-plane" test -tags=integration -count=1 \
  -run '^(TestDecoyLifecycleObservedStateAndAudit|TestDecoyObservationsAreScopedToTheReportingDeviceEnvironment|TestDesiredStateSnapshotCarriesDecoysWithinTheChannelBound)$' \
  ./internal/storage

# The Edge decoy component and the domain vocabularies do not need a database.
GOWORK=off go -C "$repo_root/apps/control-plane" test -count=1 ./internal/deception
GOWORK=off GOOS=linux go -C "$repo_root/apps/edge-agent" vet ./internal/decoy
if [ "$(uname -s)" = "Linux" ]; then
  GOWORK=off go -C "$repo_root/apps/edge-agent" test -count=1 ./internal/decoy
fi

# AC-SEC-002 in the direction this package can prove: the decoy packages reach
# neither device identity nor a privileged operation. The Go boundary test
# asserts the same thing; this is the grep an auditor can run by hand.
if rg -n \
  'internal/identity|internal/devicepki|internal/privileged|internal/privclient|os/exec|containerd' \
  "$repo_root/apps/edge-agent/internal/decoy" \
  --glob '!*_test.go'; then
  echo "P2-W15 Edge decoy package reached identity, PKI, privileged, or runtime authority" >&2
  exit 1
fi

# No decoy save path may scan, probe, route, or mutate a firewall. Placement is
# a Control Plane decision about configuration, not a network action.
if rg -n \
  'net\.Dial|DialContext|ListenPacket|exec\.Command|os/exec|nmap|masscan|iptables|nftables|firewall|ip[[:space:]]+route|netlink|RawConn' \
  "$repo_root/apps/control-plane/internal/deception" \
  "$repo_root/apps/control-plane/internal/storage/decoy.go" \
  "$repo_root/apps/control-plane/internal/api/decoy.go"; then
  echo "P2-W15 decoy save path contains a prohibited scan, probe, route, or firewall primitive" >&2
  exit 1
fi

# The API surface must offer no field that could carry a runtime artifact.
if rg -n \
  '"image"|"command"|"args"|"entrypoint"|"mount"|"volume"|"capabilit|"privileged"|"port"' \
  "$repo_root/apps/control-plane/internal/api/decoy.go"; then
  echo "P2-W15 decoy API exposed a runtime artifact field" >&2
  exit 1
fi

if rg -n --hidden \
  'BEGIN (EC |RSA |)PRIVATE KEY|bootstrap_token=[A-Za-z0-9_-]{43}|"session_token"[[:space:]]*:|"csrf_token"[[:space:]]*:[[:space:]]*"[A-Za-z0-9_-]{43}"' \
  "$repo_root/apps/control-plane/internal/deception" \
  "$repo_root/apps/edge-agent/internal/decoy" \
  "$repo_root/docs/runbooks/decoy" \
  "$repo_root/security/p2-w15-decoy-domain-review.md"; then
  echo "P2-W15 source/evidence scan found committed secret-shaped material" >&2
  exit 1
fi

echo "P2-W15 decoy domain, placement, lifecycle, audit, observed-state separation, SEC-06 unmanaged, channel bound, and boundary evidence passed."
