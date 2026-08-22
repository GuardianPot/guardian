#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
go_bin="go"
if ! command -v "$go_bin" >/dev/null 2>&1; then
  if command -v go.exe >/dev/null 2>&1; then
    go_bin="go.exe"
  else
    echo 'Go is required for the W9 device PKI fixture.' >&2
    exit 1
  fi
fi

cd "$repo_root"
"$go_bin" test ./apps/control-plane/internal/devicepki -run TestDevicePKI -count=1 -v
echo 'W9 device PKI fixture passed.'
