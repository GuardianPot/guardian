#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="$repo_root/deploy/lab/compose.yaml"

docker compose -f "$compose_file" -p guardian-lab down --volumes --remove-orphans
docker compose -f "$compose_file" -p guardian-lab build --pull
docker compose -f "$compose_file" -p guardian-lab up --detach
