#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)"
compose_file="$repo_root/deploy/control-plane/compose.yaml"
project_name="guardian-p1-w2-test-$$"
temporary_dir="$(mktemp -d /tmp/guardian-p1-w2.XXXXXX)"

cleanup() {
  case "$project_name" in
    guardian-p1-w2-test-*)
      docker compose --file "$compose_file" --project-name "$project_name" down --volumes --remove-orphans >/dev/null 2>&1 || true
      ;;
    *) echo "refusing unexpected Compose project cleanup: $project_name" >&2 ;;
  esac
  case "$temporary_dir" in
    /tmp/guardian-p1-w2.*) rm -rf -- "$temporary_dir" ;;
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
export GUARDIAN_POSTGRES_PORT="${GUARDIAN_P1_W2_POSTGRES_PORT:-$(free_port)}"

docker compose --file "$compose_file" --project-name "$project_name" up --detach --wait --wait-timeout 90

export GUARDIAN_TEST_DATABASE_URL="postgres://${GUARDIAN_POSTGRES_USER}:${GUARDIAN_POSTGRES_PASSWORD}@127.0.0.1:${GUARDIAN_POSTGRES_PORT}/${GUARDIAN_POSTGRES_DB}?sslmode=disable"
export GUARDIAN_DATABASE_URL="$GUARDIAN_TEST_DATABASE_URL"
export GUARDIAN_MASTER_KEY_FILE="$temporary_dir/master.key"
umask 077
head -c 32 /dev/urandom >"$GUARDIAN_MASTER_KEY_FILE"

GOWORK=off go -C "$repo_root/apps/control-plane" run ./cmd/control-plane migrate >/dev/null
bootstrap_output="$(GOWORK=off go -C "$repo_root/apps/control-plane" run ./cmd/control-plane create-bootstrap-token)"
mapfile -t bootstrap_lines <<<"$bootstrap_output"
if [[ ${#bootstrap_lines[@]} -ne 2 ]] ||
  [[ ! "${bootstrap_lines[0]}" =~ ^bootstrap_token=[A-Za-z0-9_-]{43}$ ]] ||
  [[ ! "${bootstrap_lines[1]}" =~ ^expires_at=[0-9TZ:+-]+$ ]]; then
  echo "Bootstrap CLI did not emit the bounded token/expiry contract" >&2
  exit 1
fi
unset bootstrap_output
unset bootstrap_lines

GOWORK=off go -C "$repo_root/apps/control-plane" test -tags=integration -count=1 \
  -run '^TestLocalAuthenticationPostgreSQLAndTLSContract$' ./internal/storage

if rg -n --hidden --glob '!go.work.sum' \
  'BEGIN (EC |RSA |)PRIVATE KEY|bootstrap_token=[A-Za-z0-9_-]{43}|"session_token"[[:space:]]*:|"recovery_codes"[[:space:]]*:[[:space:]]*\[[[:space:]]*"[A-Za-z0-9_-]{22}"' \
  "$repo_root/apps/control-plane/internal/auth" \
  "$repo_root/apps/control-plane/internal/api" \
  "$repo_root/apps/control-plane/internal/storage" \
  "$repo_root/docs/runbooks/auth" \
  "$repo_root/security/p1-w2-local-auth-review.md"; then
  echo "Authentication source/evidence scan found committed secret-shaped material" >&2
  exit 1
fi

echo "P1-W2 PostgreSQL, TLS, bootstrap, MFA, recovery, session, throttle, audit, and redaction evidence passed."
