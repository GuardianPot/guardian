#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)"
compose_file="$repo_root/deploy/control-plane/compose.yaml"
project_name="guardian-p1-w3-test-$$"

cleanup() {
  case "$project_name" in
    guardian-p1-w3-test-*)
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
export GUARDIAN_POSTGRES_PASSWORD="guardian-environment-$(random_secret)"
export GUARDIAN_POSTGRES_PORT="${GUARDIAN_P1_W3_POSTGRES_PORT:-$(free_port)}"

docker compose --file "$compose_file" --project-name "$project_name" up --detach --wait --wait-timeout 90

export GUARDIAN_TEST_DATABASE_URL="postgres://${GUARDIAN_POSTGRES_USER}:${GUARDIAN_POSTGRES_PASSWORD}@127.0.0.1:${GUARDIAN_POSTGRES_PORT}/${GUARDIAN_POSTGRES_DB}?sslmode=disable"

GOWORK=off go -C "$repo_root/apps/control-plane" test -tags=integration -count=1 \
  -run '^(TestEnvironmentPostgreSQLContract|TestEnvironmentHTTPSWithRealOwnerSession)$' ./internal/storage

if rg -n \
  'net\.Dial|DialContext|ListenPacket|exec\.Command|os/exec|nmap|masscan|iptables|nftables|firewall|ip[[:space:]]+route|netlink|RawConn' \
  "$repo_root/apps/control-plane/internal/environment" \
  "$repo_root/apps/control-plane/internal/storage/environment.go" \
  "$repo_root/apps/control-plane/internal/api/environment.go"; then
  echo "P1-W3 save path contains a prohibited scan, probe, route, or firewall primitive" >&2
  exit 1
fi

if rg -n --hidden \
  'BEGIN (EC |RSA |)PRIVATE KEY|bootstrap_token=[A-Za-z0-9_-]{43}|"session_token"[[:space:]]*:|"csrf_token"[[:space:]]*:[[:space:]]*"[A-Za-z0-9_-]{43}"' \
  "$repo_root/apps/control-plane/internal/environment" \
  "$repo_root/apps/control-plane/internal/api/environment.go" \
  "$repo_root/apps/control-plane/internal/storage/environment.go" \
  "$repo_root/docs/runbooks/environment" \
  "$repo_root/security/p1-w3-environment-review.md"; then
  echo "P1-W3 source/evidence scan found committed secret-shaped material" >&2
  exit 1
fi

echo "P1-W3 PostgreSQL 18, HTTPS owner authorization, CIDR, concurrency, rollback, FK, restart, ETag, bounds, redaction, forbidden-method, and no-scan evidence passed."
