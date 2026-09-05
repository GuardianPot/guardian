#!/usr/bin/env bash
set -euo pipefail
trap 'echo "P1-W11 runner failed at line $LINENO" >&2' ERR

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)"
compose_file="$repo_root/deploy/control-plane/compose.yaml"
project_name="guardian-p1-w11-test-$$"
fixture_dir="$(mktemp -d /tmp/guardian-p1-w11.XXXXXX)"
control_pid=""

cleanup() {
  if [[ -n "$control_pid" ]] && kill -0 "$control_pid" >/dev/null 2>&1; then
    kill "$control_pid" >/dev/null 2>&1 || true
    wait "$control_pid" >/dev/null 2>&1 || true
  fi
  case "$project_name" in
    guardian-p1-w11-test-*) docker compose --file "$compose_file" --project-name "$project_name" down --volumes --remove-orphans >/dev/null 2>&1 || true ;;
    *) echo "refusing unexpected Compose project cleanup: $project_name" >&2 ;;
  esac
  case "$fixture_dir" in
    /tmp/guardian-p1-w11.*) chmod -R u+rwX "$fixture_dir" >/dev/null 2>&1 || true; rm -rf -- "$fixture_dir" ;;
    *) echo "refusing unexpected fixture cleanup: $fixture_dir" >&2 ;;
  esac
}
trap cleanup EXIT

free_port() {
  node -e 'const server=require("node:net").createServer();server.listen(0,"127.0.0.1",()=>{process.stdout.write(String(server.address().port));server.close();});'
}

umask 077
http_port="$(free_port)"
device_port="$(free_port)"
export GUARDIAN_POSTGRES_DB=guardian
export GUARDIAN_POSTGRES_USER=guardian
export GUARDIAN_POSTGRES_PASSWORD="guardian-$(openssl rand -hex 16)"
export GUARDIAN_POSTGRES_PORT="$(free_port)"
export GUARDIAN_DATABASE_URL="postgres://${GUARDIAN_POSTGRES_USER}:${GUARDIAN_POSTGRES_PASSWORD}@127.0.0.1:${GUARDIAN_POSTGRES_PORT}/${GUARDIAN_POSTGRES_DB}?sslmode=disable"
master_key_path="$fixture_dir/master.key"
tls_certificate_path="$fixture_dir/server.crt"
tls_key_path="$fixture_dir/server.key"
export GUARDIAN_PUBLIC_ORIGIN="https://127.0.0.1:${http_port}"
export GUARDIAN_HTTP_ADDRESS="0.0.0.0:${http_port}"
export GUARDIAN_DEVICE_CHANNEL_ADDRESS="0.0.0.0:${device_port}"

head -c 32 /dev/urandom >"$master_key_path"
printf '%s\n' '[req]' 'distinguished_name=dn' 'prompt=no' '[dn]' 'CN=Guardian E2E Root' >"$fixture_dir/ca.cnf"
printf '%s\n' '[req]' 'distinguished_name=dn' 'prompt=no' '[dn]' 'CN=127.0.0.1' >"$fixture_dir/server.cnf"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -config "$fixture_dir/ca.cnf" \
  -keyout "$fixture_dir/server-ca.key" -out "$fixture_dir/server-ca.crt" >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -config "$fixture_dir/server.cnf" \
  -keyout "$tls_key_path" -out "$fixture_dir/server.csr" >/dev/null 2>&1
printf '%s\n' 'subjectAltName=IP:127.0.0.1,DNS:localhost,DNS:host.docker.internal' 'extendedKeyUsage=serverAuth' 'keyUsage=digitalSignature,keyEncipherment' >"$fixture_dir/server.ext"
openssl x509 -req -days 1 -sha256 -in "$fixture_dir/server.csr" \
  -CA "$fixture_dir/server-ca.crt" -CAkey "$fixture_dir/server-ca.key" -CAcreateserial \
  -extfile "$fixture_dir/server.ext" -out "$tls_certificate_path" >/dev/null 2>&1

export GUARDIAN_MASTER_KEY_FILE="$master_key_path"
export GUARDIAN_TLS_CERT_FILE="$tls_certificate_path"
export GUARDIAN_TLS_KEY_FILE="$tls_key_path"
export GUARDIAN_WEB_CONSOLE_DIR="$repo_root/apps/web-console/dist"
if command -v cygpath >/dev/null 2>&1; then
  export GUARDIAN_MASTER_KEY_FILE="$(cygpath -w "$master_key_path")"
  export GUARDIAN_TLS_CERT_FILE="$(cygpath -w "$tls_certificate_path")"
  export GUARDIAN_TLS_KEY_FILE="$(cygpath -w "$tls_key_path")"
  export GUARDIAN_WEB_CONSOLE_DIR="$(cygpath -w "$repo_root/apps/web-console/dist")"
fi

docker compose --file "$compose_file" --project-name "$project_name" up --detach --wait --wait-timeout 90 postgres
npm run build --workspace @guardianpot/web-console >/dev/null
GOWORK=off go -C "$repo_root/apps/control-plane" build -trimpath -o "$fixture_dir/guardian-control-plane" ./cmd/control-plane
edge_runtime=host
if [[ "$(uname -s)" == MINGW* || "$(uname -s)" == MSYS* || "$(uname -s)" == CYGWIN* ]]; then
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOWORK=off go -C "$repo_root/apps/edge-agent" build -trimpath -o "$fixture_dir/guardian-edge" ./cmd/edge-agent
  edge_runtime=container
else
  GOWORK=off go -C "$repo_root/apps/edge-agent" build -trimpath -o "$fixture_dir/guardian-edge" ./cmd/edge-agent
fi
GOWORK=off "$fixture_dir/guardian-control-plane" migrate >/dev/null
GOWORK=off "$fixture_dir/guardian-control-plane" init-device-ca >/dev/null
bootstrap_output="$(GOWORK=off "$fixture_dir/guardian-control-plane" create-bootstrap-token)"
bootstrap_token="$(printf '%s\n' "$bootstrap_output" | sed -n 's/^bootstrap_token=//p')"
unset bootstrap_output
if [[ ! "$bootstrap_token" =~ ^[A-Za-z0-9_-]{43}$ ]]; then
  echo 'bootstrap token contract failed' >&2
  exit 1
fi

"$fixture_dir/guardian-control-plane" serve >"$fixture_dir/control-plane.log" 2>&1 &
control_pid=$!
for _ in $(seq 1 300); do
  if curl --silent --fail --cacert "$fixture_dir/server-ca.crt" "$GUARDIAN_PUBLIC_ORIGIN/readyz" >/dev/null; then break; fi
  if ! kill -0 "$control_pid" >/dev/null 2>&1; then
    sed "s/${GUARDIAN_POSTGRES_PASSWORD}/[REDACTED]/g" "$fixture_dir/control-plane.log" >&2
    echo 'Control Plane stopped before Web Console readiness' >&2
    exit 1
  fi
  sleep 0.1
done
if ! curl --silent --fail --cacert "$fixture_dir/server-ca.crt" "$GUARDIAN_PUBLIC_ORIGIN/readyz" >/dev/null; then
  echo 'Control Plane Web Console readiness timed out' >&2
  exit 1
fi

export GUARDIAN_E2E_USERNAME=owner
export GUARDIAN_E2E_PASSWORD="Guardian-$(openssl rand -hex 16)!"
bootstrap_response="$(node -e 'process.stdout.write(JSON.stringify({username:process.env.GUARDIAN_E2E_USERNAME,password:process.env.GUARDIAN_E2E_PASSWORD}))' |
  curl --silent --show-error --fail --cacert "$fixture_dir/server-ca.crt" \
    --request POST --header "Authorization: Bearer ${bootstrap_token}" --header 'Content-Type: application/json' \
    --data-binary @- "$GUARDIAN_PUBLIC_ORIGIN/v1/auth/bootstrap")"
unset bootstrap_token
mapfile -t activation_material < <(printf '%s' "$bootstrap_response" | node "$repo_root/tests/e2e/web-console/bootstrap-material.mjs")
if [[ ${#activation_material[@]} -ne 2 ]] || [[ ! "${activation_material[1]}" =~ ^[0-9]{6}$ ]]; then
  echo 'bootstrap MFA material contract failed' >&2
  exit 1
fi
export GUARDIAN_E2E_RECOVERY_CODES="${activation_material[0]}"
export GUARDIAN_E2E_TOTP_CODE="${activation_material[1]}"
unset bootstrap_response
node -e 'process.stdout.write(JSON.stringify({username:process.env.GUARDIAN_E2E_USERNAME,password:process.env.GUARDIAN_E2E_PASSWORD,totp_code:process.env.GUARDIAN_E2E_TOTP_CODE}))' |
  curl --silent --show-error --fail --output /dev/null --cacert "$fixture_dir/server-ca.crt" \
    --request POST --header 'Content-Type: application/json' --data-binary @- "$GUARDIAN_PUBLIC_ORIGIN/v1/auth/login"
unset GUARDIAN_E2E_TOTP_CODE
unset activation_material

host_repo_root="$repo_root"
host_fixture_dir="$fixture_dir"
host_tls_ca="$fixture_dir/server-ca.crt"
host_edge_bin="$fixture_dir/guardian-edge"
if command -v cygpath >/dev/null 2>&1; then
  host_repo_root="$(cygpath -w "$repo_root")"
  host_fixture_dir="$(cygpath -w "$fixture_dir")"
  host_tls_ca="$(cygpath -w "$fixture_dir/server-ca.crt")"
  host_edge_bin="$(cygpath -w "$fixture_dir/guardian-edge")"
fi
export GUARDIAN_E2E_REPO_ROOT="$host_repo_root"
export GUARDIAN_E2E_BASE_URL="$GUARDIAN_PUBLIC_ORIGIN"
export GUARDIAN_E2E_DEVICE_CHANNEL="127.0.0.1:${device_port}"
export GUARDIAN_E2E_TLS_CA="$host_tls_ca"
export GUARDIAN_E2E_EDGE_BIN="$host_edge_bin"
export GUARDIAN_E2E_EDGE_RUNTIME="$edge_runtime"
export GUARDIAN_E2E_EDGE_CONTROL_PLANE="127.0.0.1:${http_port}"
export GUARDIAN_E2E_EDGE_DEVICE_CHANNEL="127.0.0.1:${device_port}"
if [[ "$edge_runtime" == container ]]; then
  export GUARDIAN_E2E_EDGE_CONTROL_PLANE="host.docker.internal:${http_port}"
  export GUARDIAN_E2E_EDGE_DEVICE_CHANNEL="host.docker.internal:${device_port}"
fi
export GUARDIAN_E2E_FIXTURE_DIR="$host_fixture_dir"
output_dir="${GUARDIAN_E2E_ARTIFACT_DIR:-$fixture_dir/evidence}"
results_dir="$fixture_dir/playwright-results"
mkdir -p "$output_dir"
export GUARDIAN_E2E_OUTPUT_DIR="$output_dir"
export GUARDIAN_E2E_RESULTS_DIR="$results_dir"
if command -v cygpath >/dev/null 2>&1; then
  export GUARDIAN_E2E_OUTPUT_DIR="$(cygpath -w "$output_dir")"
  export GUARDIAN_E2E_RESULTS_DIR="$(cygpath -w "$results_dir")"
fi

# A pull request proves the flow in one engine; main, the nightly sweep, and a
# manual dispatch prove it in all three. GUARDIAN_E2E_PROJECTS selects which.
e2e_projects="${GUARDIAN_E2E_PROJECTS:-chromium firefox webkit}"
project_args=()
for project in $e2e_projects; do
  case "$project" in
    chromium|firefox|webkit) project_args+=(--project "$project") ;;
    *) echo "unsupported Playwright project: $project" >&2; exit 1 ;;
  esac
done
if [[ ${#project_args[@]} -eq 0 ]]; then
  echo 'no Playwright project selected' >&2
  exit 1
fi

npx playwright test --config "$repo_root/tests/e2e/web-console/playwright.config.ts" "${project_args[@]}"

if find "$output_dir" -type f \( -name '*.json' -o -name '*.txt' -o -name '*.zip' \) -print -quit | grep -q .; then
  echo 'unexpected text or trace artifact was produced' >&2
  exit 1
fi
screenshot_count="$(find "$output_dir/screenshots" -type f -name 'onboarding-complete-*.png' | wc -l | tr -d ' ')"
expected_screenshots=$(( ${#project_args[@]} ))
if [[ "$screenshot_count" != "$expected_screenshots" ]]; then
  echo "expected $expected_screenshots post-dismissal screenshots, found $screenshot_count" >&2
  exit 1
fi

echo "Real onboarding, enrollment, health, accessibility, reduced-motion, and secret-lifetime evidence passed in: $e2e_projects"
