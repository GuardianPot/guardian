#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)"
compose_file="$repo_root/deploy/control-plane/compose.yaml"
project_name="guardian-p1-w10-audit-$$"

cleanup() {
  case "$project_name" in
    guardian-p1-w10-audit-*)
      docker compose --file "$compose_file" --project-name "$project_name" down --volumes --remove-orphans >/dev/null 2>&1 || true
      ;;
    *)
      echo "refusing unexpected Compose project cleanup: $project_name" >&2
      ;;
  esac
}
trap cleanup EXIT

free_port() {
  local candidate=""
  if command -v python3 >/dev/null 2>&1; then
    candidate="$(
      python3 - 2>/dev/null <<'PY'
import socket
with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
    )" || candidate=""
    candidate="${candidate//$'\r'/}"
    if [[ "$candidate" =~ ^[0-9]+$ ]] && (( candidate >= 1024 && candidate <= 65535 )); then
      printf '%s\n' "$candidate"
      return 0
    fi
  fi

  if command -v powershell.exe >/dev/null 2>&1; then
    candidate="$(powershell.exe -NoProfile -Command '$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0); $listener.Start(); $port = ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port; $listener.Stop(); $port' 2>/dev/null)" || candidate=""
    candidate="${candidate//$'\r'/}"
    if [[ "$candidate" =~ ^[0-9]+$ ]] && (( candidate >= 1024 && candidate <= 65535 )); then
      printf '%s\n' "$candidate"
      return 0
    fi
  fi

  echo "unable to allocate a free local TCP port" >&2
  return 1
}

random_secret() {
  local candidate=""
  if [[ -r /proc/sys/kernel/random/uuid ]]; then
    candidate="$(tr -d '-' </proc/sys/kernel/random/uuid)"
  elif command -v openssl >/dev/null 2>&1; then
    candidate="$(openssl rand -hex 24 2>/dev/null)" || candidate=""
  elif command -v powershell.exe >/dev/null 2>&1; then
    candidate="$(powershell.exe -NoProfile -Command "[guid]::NewGuid().ToString('N')" 2>/dev/null)" || candidate=""
    candidate="${candidate//$'\r'/}"
  fi
  if [[ ! "$candidate" =~ ^[[:xdigit:]]{32,}$ ]]; then
    echo "unable to generate a disposable database secret" >&2
    return 1
  fi
  printf '%s\n' "$candidate"
}

export GUARDIAN_POSTGRES_DB=guardian
export GUARDIAN_POSTGRES_USER=guardian
export GUARDIAN_POSTGRES_PASSWORD="guardian-audit-$(random_secret)"
export GUARDIAN_POSTGRES_PORT="$(free_port)"

docker compose --file "$compose_file" --project-name "$project_name" up --detach --wait --wait-timeout 90

export GUARDIAN_TEST_DATABASE_URL="postgres://${GUARDIAN_POSTGRES_USER}:${GUARDIAN_POSTGRES_PASSWORD}@127.0.0.1:${GUARDIAN_POSTGRES_PORT}/${GUARDIAN_POSTGRES_DB}?sslmode=disable"

GOWORK=off go -C "$repo_root/apps/control-plane" test -count=1 ./internal/audit ./internal/api
GOWORK=off go -C "$repo_root/apps/control-plane" test -tags=integration -count=1 ./internal/storage

echo "P1-W10 audit domain, authorization, append-only, transaction, pagination, restart, and concurrency evidence passed."
