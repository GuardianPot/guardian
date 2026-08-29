#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)"
compose_file="$repo_root/deploy/control-plane/compose.yaml"
project_name="guardian-p1-w6-test-$$"
fixture_dir="$(mktemp -d /tmp/guardian-p1-w6-fixture.XXXXXX)"
control_pid=""

cleanup() {
  if [[ -n "$control_pid" ]] && kill -0 "$control_pid" >/dev/null 2>&1; then
    kill "$control_pid" >/dev/null 2>&1 || true
    wait "$control_pid" >/dev/null 2>&1 || true
  fi
  case "$project_name" in
    guardian-p1-w6-test-*)
      docker compose --file "$compose_file" --project-name "$project_name" down --volumes --remove-orphans >/dev/null 2>&1 || true
      ;;
    *) echo "refusing unexpected Compose project cleanup: $project_name" >&2 ;;
  esac
  case "$fixture_dir" in
    /tmp/guardian-p1-w6-fixture.*)
      chmod -R u+rwX "$fixture_dir" >/dev/null 2>&1 || true
      rm -rf -- "$fixture_dir"
      ;;
    *) echo "refusing unexpected fixture cleanup: $fixture_dir" >&2 ;;
  esac
}
trap cleanup EXIT

export GUARDIAN_POSTGRES_DB=guardian
export GUARDIAN_POSTGRES_USER=guardian
password_suffix="$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')"
if [[ ! "$password_suffix" =~ ^[0-9a-f]{32}$ ]]; then
  echo "failed to generate disposable PostgreSQL password" >&2
  exit 1
fi
export GUARDIAN_POSTGRES_PASSWORD="guardian-$password_suffix"
export GUARDIAN_POSTGRES_PORT="${GUARDIAN_P1_W6_POSTGRES_PORT:-55435}"

docker compose --file "$compose_file" --project-name "$project_name" up --detach --wait --wait-timeout 90

export GUARDIAN_TEST_DATABASE_URL="postgres://${GUARDIAN_POSTGRES_USER}:${GUARDIAN_POSTGRES_PASSWORD}@127.0.0.1:${GUARDIAN_POSTGRES_PORT}/${GUARDIAN_POSTGRES_DB}?sslmode=disable"
GOWORK=off go -C "$repo_root/apps/control-plane" test -tags=integration -count=1 \
  -run '^TestDesiredStatePublicationObservationAndAuditAreDurable$' ./internal/storage

export GUARDIAN_RECON_FIXTURE_DIR="$fixture_dir"
GOWORK=off go -C "$repo_root/apps/control-plane" test -tags=integration -count=1 \
  -run '^TestReconciliationControlFixture$' ./internal/reconciliation \
  >"$fixture_dir/control.log" 2>&1 &
control_pid=$!

for _ in $(seq 1 600); do
  if [[ -s "$fixture_dir/fixture.json" ]]; then
    break
  fi
  if ! kill -0 "$control_pid" >/dev/null 2>&1; then
    cat "$fixture_dir/control.log" >&2
    echo "Control Plane reconciliation fixture stopped before readiness" >&2
    exit 1
  fi
  sleep 0.05
done
if [[ ! -s "$fixture_dir/fixture.json" ]]; then
  cat "$fixture_dir/control.log" >&2
  echo "Control Plane reconciliation fixture readiness timed out" >&2
  exit 1
fi

if ! GOWORK=off go -C "$repo_root/apps/edge-agent" test -tags=integration -count=1 \
  -run '^TestReconciliationEdgeFixture$' ./internal/reconciliation \
  >"$fixture_dir/edge.log" 2>&1; then
  cat "$fixture_dir/control.log" >&2
  cat "$fixture_dir/edge.log" >&2
  exit 1
fi
if ! wait "$control_pid"; then
  cat "$fixture_dir/control.log" >&2
  cat "$fixture_dir/edge.log" >&2
  exit 1
fi
control_pid=""

run_isolated_tests() {
  local module_path="$1"
  local packages="$2"
  local source_path="$repo_root/$module_path"
  local module_cache
  module_cache="$(GOWORK=off go env GOMODCACHE)"
  if command -v cygpath >/dev/null 2>&1; then
    source_path="$(cygpath -w "$source_path")"
    module_cache="$(cygpath -w "$module_cache")"
  fi
  MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*' docker run --rm \
    --network=none \
    --read-only \
    --tmpfs /tmp:rw,exec,nosuid,size=1g \
    --cap-drop=ALL \
    --security-opt=no-new-privileges \
    --pids-limit=1024 \
    --env CGO_ENABLED=1 \
    --env GOWORK=off \
    --env GOMODCACHE=/go/pkg/mod \
    --env GOCACHE=/tmp/go-build \
    --env GOTMPDIR=/tmp/go-tmp \
    --env HOME=/tmp/home \
    --mount "type=bind,src=$source_path,dst=/src,readonly" \
    --mount "type=bind,src=$module_cache,dst=/go/pkg/mod,readonly" \
    --workdir /src \
    golang:1.27-bookworm@sha256:484ef6066fa69acb059fdfeda7ba2b8f7391f2ef6abc6f9b8411e669ebd56466 \
    sh -ec "mkdir -p /tmp/go-build /tmp/go-tmp /tmp/home && exec go test -mod=readonly -race -count=3 $packages"
}

run_isolated_tests apps/control-plane './internal/reconciliation'
run_isolated_tests apps/edge-agent './internal/reconciliation'

node -e 'JSON.parse(require("node:fs").readFileSync(process.argv[1], "utf8"))' \
  "$repo_root/schemas/device/v1/desired-state-snapshot.schema.json"

if rg -n --hidden --glob '!go.work.sum' \
  'BEGIN (EC |RSA |)PRIVATE KEY|authorization:[[:space:]]*bearer|enrollment_token[=:][A-Za-z0-9_-]{20,}' \
  "$repo_root/apps/control-plane/internal/reconciliation" \
  "$repo_root/apps/control-plane/internal/storage/reconciliation.go" \
  "$repo_root/apps/edge-agent/internal/reconciliation" \
  "$repo_root/apps/edge-agent/internal/storage/reconciliation.go" \
  "$repo_root/proto/guardian/device" \
  "$repo_root/schemas/device" \
  "$repo_root/docs/runbooks/reconciliation" \
  "$repo_root/security/p1-w6-reconciliation-review.md"; then
  echo "Reconciliation source/evidence scan found committed credential-shaped material" >&2
  exit 1
fi

echo "P1-W6 durable publication, real mTLS convergence, replay, restart, retry, bounds, race, and secret-scan evidence passed."
