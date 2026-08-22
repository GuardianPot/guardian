#!/usr/bin/env bash
set -euo pipefail

image='debian:13-slim@sha256:3a39a0592364683e6bab97937b72cad5a8fa6dcbbee90edb3bb48c7f8e94f258'
name='guardian-p0-w6-decoy-fixture'

if command -v docker >/dev/null 2>&1 && docker version >/dev/null 2>&1; then
  docker_bin=docker
elif command -v docker.exe >/dev/null 2>&1 && docker.exe version >/dev/null 2>&1; then
  docker_bin=docker.exe
else
  echo 'Docker CLI is required (docker or docker.exe).' >&2
  exit 1
fi

remove_fixture() {
  "$docker_bin" rm --force "$name" >/dev/null 2>&1 || true
}

cycle() {
  remove_fixture
  "$docker_bin" create \
    --name "$name" \
    --runtime runc \
    --network none \
    --read-only \
    --tmpfs '/tmp:rw,noexec,nosuid,nodev,size=16m' \
    --memory 64m \
    --memory-swap 64m \
    --cpus 0.25 \
    --pids-limit 64 \
    --cap-drop ALL \
    --security-opt no-new-privileges:true \
    --user 65532:65532 \
    --label guardian.work-package=P0-W6 \
    --label guardian.decoy.socket-mount=false \
    "$image" sleep infinity >/dev/null
  "$docker_bin" start "$name" >/dev/null

  runtime="$($docker_bin inspect --format '{{.HostConfig.Runtime}}' "$name" | tr -d '\r')"
  privileged="$($docker_bin inspect --format '{{.HostConfig.Privileged}}' "$name" | tr -d '\r')"
  memory="$($docker_bin inspect --format '{{.HostConfig.Memory}}' "$name" | tr -d '\r')"
  nano_cpus="$($docker_bin inspect --format '{{.HostConfig.NanoCpus}}' "$name" | tr -d '\r')"
  pids_limit="$($docker_bin inspect --format '{{.HostConfig.PidsLimit}}' "$name" | tr -d '\r')"
  network_mode="$($docker_bin inspect --format '{{.HostConfig.NetworkMode}}' "$name" | tr -d '\r')"
  security_opt="$($docker_bin inspect --format '{{json .HostConfig.SecurityOpt}}' "$name" | tr -d '\r')"
  cap_drop="$($docker_bin inspect --format '{{json .HostConfig.CapDrop}}' "$name" | tr -d '\r')"
  binds="$($docker_bin inspect --format '{{json .HostConfig.Binds}}' "$name" | tr -d '\r')"

  test "$runtime" = runc
  test "$privileged" = false
  test "$memory" = 67108864
  test "$nano_cpus" = 250000000
  test "$pids_limit" = 64
  test "$network_mode" = none
  printf '%s' "$security_opt" | grep -F 'no-new-privileges:true' >/dev/null
  printf '%s' "$cap_drop" | grep -F 'ALL' >/dev/null
  test "$binds" = 'null'

  "$docker_bin" exec "$name" sh -ec 'for socket in /run/containerd/containerd.sock /run/containerd/s/containerd.sock /var/run/docker.sock /run/docker.sock; do test ! -S "$socket"; done; if touch /etc/guardian-w6-write-test 2>/dev/null; then exit 1; fi; touch /tmp/guardian-w6-write-test; test -f /tmp/guardian-w6-write-test'
  "$docker_bin" kill "$name" >/dev/null
  running="$($docker_bin inspect --format '{{.State.Running}}' "$name" | tr -d '\r')"
  test "$running" = false
  remove_fixture
  if "$docker_bin" container inspect "$name" >/dev/null 2>&1; then
    echo 'decoy cleanup left a container behind' >&2
    exit 1
  fi
}

trap remove_fixture EXIT
remove_fixture
"$docker_bin" pull "$image" >/dev/null
runtime_json="$($docker_bin info --format '{{json .Runtimes}}')"
printf '%s' "$runtime_json" | grep -F '"runc"' >/dev/null
cycle
cycle
echo 'P0-W6 decoy lifecycle passed: digest pin, runc, limits, socket isolation, failure cleanup, and repeatability.'
