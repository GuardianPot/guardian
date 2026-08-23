#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)"
compose_file="$repo_root/deploy/control-plane/compose.yaml"
project_name="guardian-p1-w1-test-$$"
temporary_dir="$(mktemp -d /tmp/guardian-p1-w1.XXXXXX)"
control_plane_pid=""

show_log() {
  if [[ -f "$temporary_dir/control-plane.log" ]]; then
    sed "s/${GUARDIAN_POSTGRES_PASSWORD}/[REDACTED]/g" "$temporary_dir/control-plane.log" >&2
  fi
}

cleanup() {
  if [[ -n "$control_plane_pid" ]] && kill -0 "$control_plane_pid" 2>/dev/null; then
    kill -TERM "$control_plane_pid" 2>/dev/null || true
    wait "$control_plane_pid" 2>/dev/null || true
  fi
  case "$project_name" in
    guardian-p1-w1-test-*)
      docker compose --file "$compose_file" --project-name "$project_name" down --volumes --remove-orphans >/dev/null 2>&1 || true
      ;;
    *)
      echo "refusing unexpected Compose project cleanup: $project_name" >&2
      ;;
  esac
  case "$temporary_dir" in
    /tmp/guardian-p1-w1.*) rm -rf -- "$temporary_dir" ;;
    *) echo "refusing unexpected temporary cleanup: $temporary_dir" >&2 ;;
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

export GUARDIAN_POSTGRES_DB=guardian
export GUARDIAN_POSTGRES_USER=guardian
export GUARDIAN_POSTGRES_PASSWORD="guardian-$(tr -d '-' </proc/sys/kernel/random/uuid)"
export GUARDIAN_POSTGRES_PORT="${GUARDIAN_POSTGRES_PORT:-55432}"

docker compose --file "$compose_file" --project-name "$project_name" up --detach --wait --wait-timeout 90

export GUARDIAN_TEST_DATABASE_URL="postgres://${GUARDIAN_POSTGRES_USER}:${GUARDIAN_POSTGRES_PASSWORD}@127.0.0.1:${GUARDIAN_POSTGRES_PORT}/${GUARDIAN_POSTGRES_DB}?sslmode=disable"
export GUARDIAN_DATABASE_URL="$GUARDIAN_TEST_DATABASE_URL"

GOWORK=off go -C "$repo_root/apps/control-plane" test -tags=integration -count=1 ./internal/database
GOWORK=off go -C "$repo_root/apps/control-plane" run ./cmd/control-plane migrate
GOWORK=off go -C "$repo_root/apps/control-plane" build -trimpath -o "$temporary_dir/guardian-control-plane" ./cmd/control-plane

http_port="$(free_port)"
GUARDIAN_HTTP_ADDRESS="127.0.0.1:${http_port}" "$temporary_dir/guardian-control-plane" serve >"$temporary_dir/control-plane.log" 2>&1 &
control_plane_pid=$!

ready="false"
for _ in $(seq 1 80); do
  if curl --fail --silent --show-error "http://127.0.0.1:${http_port}/readyz" >/dev/null 2>&1; then
    ready="true"
    break
  fi
  if ! kill -0 "$control_plane_pid" 2>/dev/null; then
    show_log
    echo "Control Plane exited before readiness" >&2
    exit 1
  fi
  sleep 0.25
done
if [[ "$ready" != "true" ]]; then
  show_log
  echo "Control Plane did not become ready" >&2
  exit 1
fi
curl --fail --silent --show-error "http://127.0.0.1:${http_port}/livez" >/dev/null

kill -TERM "$control_plane_pid"
wait "$control_plane_pid"
control_plane_pid=""

if grep -Fq "$GUARDIAN_POSTGRES_PASSWORD" "$temporary_dir/control-plane.log"; then
  echo "Control Plane log leaked the development database password" >&2
  exit 1
fi

echo "Control Plane PostgreSQL migration, restart persistence, failure, health, and shutdown evidence passed."
