#!/usr/bin/env bash
set -euo pipefail

image='cowrie/cowrie:3.0.12@sha256:3e4ce75576e4dffc3397ae3ad8dbb00afa00fe826b1531fea50d4fd9728326e1'
image_revision='ced855a5cda953eb4ad439d8ee8060afe4234fe4'
name='guardian-p0-w7-cowrie-fixture'
network='guardian-p0-w7-cowrie-network'
fixture_password='letmein'
hostile_marker='__GUARDIAN_HOSTILE__'
normalizer='tools/check-cowrie-events.mjs'
if command -v node >/dev/null 2>&1; then
  node_bin=node
elif [ -x '/mnt/c/Program Files/nodejs/node.exe' ]; then
  node_bin='/mnt/c/Program Files/nodejs/node.exe'
else
  echo 'Node.js 24 is required for Cowrie event normalization.' >&2
  exit 1
fi
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
if [ -d "/mnt/c/Users/${USER}/AppData/Local/Temp" ]; then
  client_dir="$(mktemp -d "/mnt/c/Users/${USER}/AppData/Local/Temp/guardian-p0-w7-cowrie.XXXXXX")"
else
  client_dir="$(mktemp -d)"
fi
client_bin="$client_dir/guardian-p0-w7-cowrie-client"
if command -v wslpath >/dev/null 2>&1; then
  client_bin_for_docker="$(wslpath -w "$client_bin")"
else
  client_bin_for_docker="$client_bin"
fi

if command -v docker >/dev/null 2>&1 && docker version >/dev/null 2>&1; then
  docker_bin=docker
elif command -v docker.exe >/dev/null 2>&1 && docker.exe version >/dev/null 2>&1; then
  docker_bin=docker.exe
else
  echo 'Docker CLI is required (docker or docker.exe).' >&2
  exit 1
fi

remove_fixture() {
  "$docker_bin" rm --force --volumes "$name" >/dev/null 2>&1 || true
  "$docker_bin" network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$client_dir"
}

trap remove_fixture EXIT
remove_fixture
mkdir -p "$client_dir"

"$docker_bin" pull "$image" >/dev/null
if command -v go >/dev/null 2>&1; then
  (cd tools/cowrie-client && GOWORK=off GOOS=linux GOARCH=amd64 go mod download && GOWORK=off GOOS=linux GOARCH=amd64 go build -o "$client_bin" .)
elif [ -x '/mnt/c/Program Files/Go/bin/go.exe' ] && [ -x '/mnt/c/WINDOWS/System32/WindowsPowerShell/v1.0/powershell.exe' ]; then
  repo_win="$(wslpath -w "$repo_root")"
  "/mnt/c/WINDOWS/System32/WindowsPowerShell/v1.0/powershell.exe" -NoProfile -Command "Set-Location -LiteralPath '$repo_win/tools/cowrie-client'; \$env:GOWORK='off'; \$env:GOOS='linux'; \$env:GOARCH='amd64'; go mod download; if (\$LASTEXITCODE -ne 0) { exit \$LASTEXITCODE }; go build -o '$client_bin_for_docker' ."
else
  echo 'Go 1.27 is required to build the disposable Cowrie SSH client.' >&2
  exit 1
fi

"$docker_bin" network create --internal --label guardian.work-package=P0-W7 "$network" >/dev/null
"$docker_bin" create \
  --name "$name" \
  --network "$network" \
  --read-only \
  --tmpfs '/tmp:rw,noexec,nosuid,nodev,size=16m,uid=999,gid=999' \
  --tmpfs '/cowrie/cowrie-git/var/log/cowrie:rw,nosuid,nodev,size=32m,uid=999,gid=999' \
  --tmpfs '/cowrie/cowrie-git/var/lib/cowrie:rw,nosuid,nodev,size=32m,uid=999,gid=999' \
  --tmpfs '/cowrie/cowrie-git/var/lib/cowrie/tty:rw,nosuid,nodev,size=32m,uid=999,gid=999' \
  --tmpfs '/cowrie/cowrie-git/var/run:rw,nosuid,nodev,size=8m,uid=999,gid=999' \
  --memory 256m \
  --memory-swap 256m \
  --cpus 0.50 \
  --pids-limit 128 \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --label guardian.work-package=P0-W7 \
  --label guardian.decoy.socket-mount=false \
  --label "guardian.cowrie.upstream=v3.0.12@$image_revision" \
  "$image" >/dev/null
"$docker_bin" start "$name" >/dev/null

for attempt in $(seq 1 30); do
  if "$docker_bin" logs "$name" 2>&1 | grep -F 'Ready to accept SSH connections' >/dev/null; then
    break
  fi
  if [ "$attempt" -eq 30 ]; then
    echo 'Cowrie fixture did not become SSH-ready.' >&2
    exit 1
  fi
  sleep 0.5
done

test "$("$docker_bin" inspect --format '{{.HostConfig.Privileged}}' "$name")" = false
test "$("$docker_bin" inspect --format '{{.HostConfig.ReadonlyRootfs}}' "$name")" = true
test "$("$docker_bin" inspect --format '{{.HostConfig.Memory}}' "$name")" = 268435456
test "$("$docker_bin" inspect --format '{{.HostConfig.NanoCpus}}' "$name")" = 500000000
test "$("$docker_bin" inspect --format '{{.HostConfig.PidsLimit}}' "$name")" = 128
test "$("$docker_bin" inspect --format '{{.HostConfig.NetworkMode}}' "$name")" = "$network"
printf '%s' "$("$docker_bin" inspect --format '{{json .HostConfig.CapDrop}}' "$name")" | grep -F 'ALL' >/dev/null
printf '%s' "$("$docker_bin" inspect --format '{{json .HostConfig.SecurityOpt}}' "$name")" | grep -F 'no-new-privileges:true' >/dev/null
test "$("$docker_bin" inspect --format '{{json .HostConfig.Binds}}' "$name")" = null
! "$docker_bin" inspect --format '{{json .Mounts}}' "$name" | grep -F '"Type":"bind"' >/dev/null
test "$("$docker_bin" network inspect --format '{{.Internal}}' "$network")" = true
test "$("$docker_bin" image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$image")" = "$image_revision"

"$docker_bin" cp "$client_bin_for_docker" "$name:/cowrie/cowrie-git/var/guardian-p0-w7-cowrie-client"
set +e
"$docker_bin" exec "$name" /cowrie/cowrie-git/var/guardian-p0-w7-cowrie-client -user root -password root -command id >/dev/null
bad_status=$?
set -e
test "$bad_status" -ne 0
"$docker_bin" exec "$name" /cowrie/cowrie-git/var/guardian-p0-w7-cowrie-client -user root -password "$fixture_password" -command "id; uname -a; printf \"<script>alert(1)</script>$hostile_marker\"" >/dev/null

raw_events="$("$docker_bin" exec "$name" /cowrie/cowrie-env/bin/python3 -c "from pathlib import Path; print(Path('var/log/cowrie/cowrie.json').read_text())")"
printf '%s\n{malformed-cowrie-event\n' "$raw_events" | "$node_bin" "$normalizer" --validate --forbidden "$fixture_password" --hostile "$hostile_marker" >/dev/null

set +e
"$docker_bin" exec "$name" /cowrie/cowrie-env/bin/python3 -c "import socket; socket.create_connection(('1.1.1.1', 443), 1); raise SystemExit('egress unexpectedly allowed')" >/dev/null
network_status=$?
set -e
test "$network_status" -ne 0

remove_fixture
trap - EXIT
echo 'P0-W7 Cowrie adapter passed: pinned provenance, SSH auth/session/command evidence, hostile-input boundary, malformed-event tolerance, internal network, and egress denial.'
