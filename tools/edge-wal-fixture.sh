#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
fixture_root="$(mktemp -d "$repo_root/.guardian-w8.XXXXXX")"
database_path="$fixture_root/edge.db"
binary_path="$fixture_root/edge-agent-fixture.exe"

go_bin="go"
if ! command -v "$go_bin" >/dev/null 2>&1; then
  if command -v go.exe >/dev/null 2>&1; then
    go_bin="go.exe"
  else
    echo 'Go is required for the W8 fixture.' >&2
    exit 1
  fi
fi
binary_output="$binary_path"
database_output="$database_path"
if [[ "$go_bin" == "go.exe" ]] && command -v wslpath >/dev/null 2>&1; then
  binary_output="$(wslpath -w "$binary_path")"
  database_output="$(wslpath -w "$database_path")"
fi

cleanup() {
  rm -rf -- "$fixture_root"
}
trap cleanup EXIT

cd "$repo_root"
"$go_bin" build -o "$binary_output" ./apps/edge-agent/cmd/edge-agent
set +e
"$binary_path" --w8-fixture crash "$database_output"
crash_exit_code=$?
set -e
if [[ "$crash_exit_code" -ne 42 ]]; then
  echo "W8 crash phase exit code was $crash_exit_code, expected 42." >&2
  exit 1
fi

sleep 0.25
"$binary_path" --w8-fixture recover "$database_output"
echo 'W8 crash/restart fixture passed.'
