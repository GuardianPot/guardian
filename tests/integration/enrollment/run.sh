#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)"
compose_file="$repo_root/deploy/control-plane/compose.yaml"
project_name="guardian-p1-w4-test-$$"
temporary_dir="$(mktemp -d /tmp/guardian-p1-w4.XXXXXX)"

cleanup() {
  case "$project_name" in
    guardian-p1-w4-test-*)
      docker compose --file "$compose_file" --project-name "$project_name" down --volumes --remove-orphans >/dev/null 2>&1 || true
      ;;
    *) echo "refusing unexpected Compose project cleanup: $project_name" >&2 ;;
  esac
  case "$temporary_dir" in
    /tmp/guardian-p1-w4.*) rm -rf -- "$temporary_dir" ;;
    *) echo "refusing unexpected temporary cleanup: $temporary_dir" >&2 ;;
  esac
}
trap cleanup EXIT

export GUARDIAN_POSTGRES_DB=guardian
export GUARDIAN_POSTGRES_USER=guardian
export GUARDIAN_POSTGRES_PASSWORD="guardian-$(tr -d '-' </proc/sys/kernel/random/uuid)"
export GUARDIAN_POSTGRES_PORT="${GUARDIAN_P1_W4_POSTGRES_PORT:-55433}"

docker compose --file "$compose_file" --project-name "$project_name" up --detach --wait --wait-timeout 90

export GUARDIAN_TEST_DATABASE_URL="postgres://${GUARDIAN_POSTGRES_USER}:${GUARDIAN_POSTGRES_PASSWORD}@127.0.0.1:${GUARDIAN_POSTGRES_PORT}/${GUARDIAN_POSTGRES_DB}?sslmode=disable"
GOWORK=off go -C "$repo_root/apps/control-plane" test -tags=integration -count=1 \
  -run '^TestDurableEnrollmentReplayReenrollmentRotationAndRevocation$' ./internal/storage
GOWORK=off go -C "$repo_root/apps/edge-agent" test -count=1 ./internal/enrollment ./internal/identity

if rg -n --hidden --glob '!go.work.sum' \
  'BEGIN (EC |RSA |)PRIVATE KEY|GUARDIAN_MASTER_KEY=[A-Za-z0-9+/=_-]{20,}|"token"[[:space:]]*:[[:space:]]*"[A-Za-z0-9_-]{43}"' \
  "$repo_root/apps/control-plane/internal/api" \
  "$repo_root/apps/control-plane/internal/devicepki" \
  "$repo_root/apps/control-plane/internal/devices" \
  "$repo_root/apps/control-plane/internal/secretstore" \
  "$repo_root/apps/control-plane/internal/storage" \
  "$repo_root/apps/edge-agent/cmd" \
  "$repo_root/apps/edge-agent/internal/app" \
  "$repo_root/apps/edge-agent/internal/config" \
  "$repo_root/apps/edge-agent/internal/enrollment" \
  "$repo_root/apps/edge-agent/internal/identity" \
  "$repo_root/deploy/device-pki" \
  "$repo_root/docs/runbooks/enrollment" \
  "$repo_root/security/p1-w4-device-enrollment-review.md"; then
  echo "Enrollment source/evidence scan found committed secret-shaped material" >&2
  exit 1
fi

echo "P1-W4 durable enrollment, replay, rotation, re-enrollment, revocation, Edge TLS, permission, and secret-scan evidence passed."
