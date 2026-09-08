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

# The boundary greps below are the evidence, not a formality, and `if <tool> ...`
# reads a missing tool's 127 as "no match" — which would pass every one of them
# silently. They use `git grep`, which is present wherever this repository is,
# and the tools they do need are required up front.
for tool in docker git go; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "required tool not found: $tool" >&2
    exit 1
  fi
done

random_secret() {
  # od and /dev/urandom rather than /proc/sys/kernel/random/uuid, which exists
  # only on Linux. This runner should work on any developer machine.
  head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n'
}

export GUARDIAN_POSTGRES_DB=guardian
export GUARDIAN_POSTGRES_USER=guardian
export GUARDIAN_POSTGRES_PASSWORD="guardian-decoy-$(random_secret)"
# 0 asks Docker for an ephemeral port and we read back what it chose. This
# needs no Python and, unlike picking a free port first, has no window in which
# something else can take the port between the probe and the bind.
export GUARDIAN_POSTGRES_PORT="${GUARDIAN_P2_W15_POSTGRES_PORT:-0}"

docker compose --file "$compose_file" --project-name "$project_name" up --detach --wait --wait-timeout 90

if [ "$GUARDIAN_POSTGRES_PORT" = "0" ]; then
  published="$(docker compose --file "$compose_file" --project-name "$project_name" port postgres 5432)"
  GUARDIAN_POSTGRES_PORT="${published##*:}"
  if ! [ "$GUARDIAN_POSTGRES_PORT" -gt 0 ] 2>/dev/null; then
    echo "could not read the published PostgreSQL port: $published" >&2
    exit 1
  fi
fi

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
#
# Comment lines are excluded. `runtime.go` names containerd in prose, because
# describing the seam P2-W3 will fill is exactly what that comment is for, and
# a package is not reaching for an authority by explaining which one it lacks.
# The check is about imports and calls.
if git -C "$repo_root" grep -n -E \
  'internal/identity|internal/devicepki|internal/privileged|internal/privclient|os/exec|containerd' \
  -- 'apps/edge-agent/internal/decoy' ':(exclude)*_test.go' \
  | grep -vE ':[0-9]+:[[:space:]]*//'; then
  echo "P2-W15 Edge decoy package reached identity, PKI, privileged, or runtime authority" >&2
  exit 1
fi

# No decoy save path may scan, probe, route, or mutate a firewall. Placement is
# a Control Plane decision about configuration, not a network action.
if git -C "$repo_root" grep -n -E \
  'net\.Dial|DialContext|ListenPacket|exec\.Command|os/exec|nmap|masscan|iptables|nftables|firewall|ip[[:space:]]+route|netlink|RawConn' \
  -- 'apps/control-plane/internal/deception' \
  'apps/control-plane/internal/storage/decoy.go' \
  'apps/control-plane/internal/api/decoy.go'; then
  echo "P2-W15 decoy save path contains a prohibited scan, probe, route, or firewall primitive" >&2
  exit 1
fi

# The API surface must offer no field that could carry a runtime artifact.
if git -C "$repo_root" grep -n -E \
  '"image"|"command"|"args"|"entrypoint"|"mount"|"volume"|"capabilit|"privileged"|"port"' \
  -- 'apps/control-plane/internal/api/decoy.go'; then
  echo "P2-W15 decoy API exposed a runtime artifact field" >&2
  exit 1
fi

if git -C "$repo_root" grep -n -E \
  'BEGIN (EC |RSA |)PRIVATE KEY|bootstrap_token=[A-Za-z0-9_-]{43}|"session_token"[[:space:]]*:|"csrf_token"[[:space:]]*:[[:space:]]*"[A-Za-z0-9_-]{43}"' \
  -- 'apps/control-plane/internal/deception' \
  'apps/edge-agent/internal/decoy' \
  'docs/runbooks/decoy' \
  'security/p2-w15-decoy-domain-review.md'; then
  echo "P2-W15 source/evidence scan found committed secret-shaped material" >&2
  exit 1
fi

echo "P2-W15 decoy domain, placement, lifecycle, audit, observed-state separation, SEC-06 unmanaged, channel bound, and boundary evidence passed."
