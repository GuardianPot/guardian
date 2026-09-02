#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)"
compose_file="$repo_root/deploy/control-plane/compose.yaml"
project_name="guardian-p1-w9-test-$$"

cleanup() {
  case "$project_name" in
    guardian-p1-w9-test-*)
      docker compose --file "$compose_file" --project-name "$project_name" down --volumes --remove-orphans >/dev/null 2>&1 || true
      ;;
    *) echo "refusing unexpected Compose project cleanup: $project_name" >&2 ;;
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
export GUARDIAN_POSTGRES_PORT="${GUARDIAN_P1_W9_POSTGRES_PORT:-55439}"

docker compose --file "$compose_file" --project-name "$project_name" up --detach --wait --wait-timeout 90
export GUARDIAN_TEST_DATABASE_URL="postgres://${GUARDIAN_POSTGRES_USER}:${GUARDIAN_POSTGRES_PASSWORD}@127.0.0.1:${GUARDIAN_POSTGRES_PORT}/${GUARDIAN_POSTGRES_DB}?sslmode=disable"

GOWORK=off go -C "$repo_root/apps/control-plane" test -tags=integration -count=1 \
  -run '^TestHealthProjectionPersistsAckDuplicateDisconnectAndRecovery$' ./internal/storage
GOWORK=off go -C "$repo_root/apps/control-plane" test -count=1 \
  ./internal/health ./internal/devicechannel ./internal/api

source_path="$repo_root/apps/edge-agent"
module_cache="$(GOWORK=off go env GOMODCACHE)"
if command -v cygpath >/dev/null 2>&1; then
  source_path="$(cygpath -w "$source_path")"
  module_cache="$(cygpath -w "$module_cache")"
fi
MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*' docker run --rm \
  --network=none \
  --read-only \
  --tmpfs /tmp:rw,exec,nosuid,size=1g \
  --tmpfs /run:rw,nosuid,nodev,size=16m \
  --cap-drop=ALL \
  --security-opt=no-new-privileges \
  --pids-limit=1024 \
  --env CGO_ENABLED=1 \
  --env GOWORK=off \
  --env GUARDIAN_RUNTIME_PROBE_FIXTURE=1 \
  --env GOMODCACHE=/go/pkg/mod \
  --env GOCACHE=/tmp/go-build \
  --env GOTMPDIR=/tmp/go-tmp \
  --env HOME=/tmp/home \
  --mount "type=bind,src=$source_path,dst=/src,readonly" \
  --mount "type=bind,src=$module_cache,dst=/go/pkg/mod,readonly" \
  --workdir /src \
  golang:1.27-bookworm@sha256:484ef6066fa69acb059fdfeda7ba2b8f7391f2ef6abc6f9b8411e669ebd56466 \
  sh -ec "mkdir -p /tmp/go-build /tmp/go-tmp /tmp/home && exec go test -mod=readonly -race -count=3 -tags=integration -run '^(TestReporter|TestHealthReport|TestSchemaTwo|TestMissingHelper|TestHelperStop|TestFixedContainerd)' ./internal/health ./internal/storage ./internal/privileged ./internal/privclient"

node "$repo_root/tests/integration/health/check-contracts.mjs"

# model_test.go intentionally contains bounded credential-shaped hostile input
# to prove that canonical messages reject it; it is not committed evidence.
# POSIX grep keeps the identical pattern runnable on any CI image; an unusable
# scanner is treated as a failure rather than as a passing scan.
set +e
scan_output="$(grep -REn --exclude=model_test.go --exclude=go.work.sum \
  'BEGIN (EC |RSA |)PRIVATE KEY|authorization:[[:space:]]*bearer|token[=:][A-Za-z0-9_-]{20,}' \
  "$repo_root/apps/control-plane/internal/health" \
  "$repo_root/apps/control-plane/internal/storage/health.go" \
  "$repo_root/apps/edge-agent/internal/health" \
  "$repo_root/apps/edge-agent/internal/storage/health.go" \
  "$repo_root/apps/edge-agent/internal/privileged/runtime.go" \
  "$repo_root/docs/runbooks/health" \
  "$repo_root/security/p1-w9-health-review.md" 2>&1)"
scan_status=$?
set -e
if [ "$scan_status" -eq 0 ]; then
  printf '%s\n' "$scan_output" >&2
  echo "Health source/evidence scan found committed credential-shaped material" >&2
  exit 1
fi
if [ "$scan_status" -ne 1 ]; then
  printf '%s\n' "$scan_output" >&2
  echo "Health source/evidence scan could not complete (status $scan_status)" >&2
  exit 1
fi

echo "P1-W9 PostgreSQL/SQLite durability, ACK/replay, disconnect, helper/runtime failure recovery, race, and secret-scan evidence passed."
