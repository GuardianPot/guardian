#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="$repo_root/deploy/lab/compose.yaml"
if command -v docker >/dev/null 2>&1 && docker version >/dev/null 2>&1; then
  docker_bin=docker
elif command -v docker.exe >/dev/null 2>&1; then
  docker_bin=docker.exe
else
  echo 'Docker CLI is required (docker or docker.exe).' >&2
  exit 1
fi
if [[ "$docker_bin" = "docker.exe" ]] && command -v wslpath >/dev/null 2>&1; then
  compose_file="$(wslpath -w "$compose_file")"
fi

"$docker_bin" compose -f "$compose_file" -p guardian-lab down --volumes --remove-orphans
"$docker_bin" compose -f "$compose_file" -p guardian-lab build --pull
"$docker_bin" compose -f "$compose_file" -p guardian-lab up --detach
