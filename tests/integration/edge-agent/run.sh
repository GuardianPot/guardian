#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/../../.." && pwd)"
fixture_root="$(mktemp -d "${TMPDIR:-/tmp}/guardian-edge-p1-w7.XXXXXX")"
binary_path="$fixture_root/guardian-edge"
config_path="$fixture_root/config.json"
database_path="$fixture_root/state/edge.db"
spool_path="$fixture_root/spool"
identity_dir="$fixture_root/identity"
cert_path="$identity_dir/device.crt"
key_path="$identity_dir/device.key"
daemon_stdout="$fixture_root/daemon.stdout"
daemon_log="$fixture_root/daemon.log"
daemon_pid=""

umask 077

if [[ "$(id -u)" -eq 0 ]]; then
  echo 'P1-W7 integration must run the main daemon as an unprivileged user.' >&2
  exit 1
fi

cleanup() {
  if [[ -n "$daemon_pid" ]] && kill -0 "$daemon_pid" 2>/dev/null; then
    kill -TERM "$daemon_pid" 2>/dev/null || true
    wait "$daemon_pid" 2>/dev/null || true
  fi
  case "$fixture_root" in
    "${TMPDIR:-/tmp}"/guardian-edge-p1-w7.*) rm -rf -- "$fixture_root" ;;
    *) printf 'Refusing to remove unexpected fixture path: %s\n' "$fixture_root" >&2 ;;
  esac
}
trap cleanup EXIT

mkdir -p -- "$identity_dir"
chmod 0700 "$identity_dir"

GOWORK=off go -C "$repo_root/apps/edge-agent" build \
  -trimpath \
  -ldflags '-s -w -X main.version=p1-w7-integration' \
  -o "$binary_path" \
  ./cmd/edge-agent

cat >"$config_path" <<EOF
{
  "control_plane_endpoint": "127.0.0.1:7443",
  "database_path": "$database_path",
  "spool_directory": "$spool_path",
  "identity_certificate_path": "$cert_path",
  "identity_private_key_path": "$key_path",
  "shutdown_timeout_seconds": 5,
  "log_level": "info"
}
EOF
chmod 0600 "$config_path"

if "$binary_path" serve --config "$fixture_root/missing.json" >"$fixture_root/missing.stdout" 2>"$fixture_root/missing.log"; then
  echo 'Missing configuration unexpectedly started the daemon.' >&2
  exit 1
fi

if "$binary_path" serve --config "$config_path" >"$fixture_root/identity.stdout" 2>"$fixture_root/identity.log"; then
  echo 'Missing secure identity unexpectedly started the daemon.' >&2
  exit 1
fi
grep -Fq 'secure device identity is unavailable' "$fixture_root/identity.log"
if [[ -e "$database_path" ]]; then
  echo 'Identity failure unexpectedly created the database.' >&2
  exit 1
fi

openssl req -x509 -newkey rsa:2048 -sha256 -nodes -days 1 \
  -subj '/CN=guardian-edge-integration' \
  -keyout "$key_path" \
  -out "$cert_path" \
  >/dev/null 2>&1
chmod 0600 "$key_path"
chmod 0644 "$cert_path"

start_daemon() {
  : >"$daemon_stdout"
  : >"$daemon_log"
  "$binary_path" serve --config "$config_path" >"$daemon_stdout" 2>"$daemon_log" &
  daemon_pid=$!
  for _ in $(seq 1 100); do
    if "$binary_path" status --config "$config_path" --format json >"$fixture_root/status.json" 2>"$fixture_root/status.err"; then
      if grep -Fq '"name": "process"' "$fixture_root/status.json" && grep -Fq '"status": "healthy"' "$fixture_root/status.json"; then
        return
      fi
    fi
    if ! kill -0 "$daemon_pid" 2>/dev/null; then
      wait "$daemon_pid" || true
      echo 'Edge daemon exited before becoming healthy.' >&2
      sed -n '1,120p' "$daemon_log" >&2
      exit 1
    fi
    sleep 0.05
  done
  echo 'Edge daemon did not become healthy.' >&2
  exit 1
}

stop_daemon() {
  kill -TERM "$daemon_pid"
  wait "$daemon_pid"
  daemon_pid=""
}

start_daemon
"$binary_path" diagnostics --config "$config_path" --format json >"$fixture_root/diagnostics.json"
if [[ "$(wc -c <"$fixture_root/diagnostics.json")" -gt 65536 ]]; then
  echo 'Diagnostics exceeded the bounded output limit.' >&2
  exit 1
fi
if grep -Fq "$key_path" "$fixture_root/diagnostics.json" || grep -Fq 'BEGIN PRIVATE KEY' "$fixture_root/diagnostics.json"; then
  echo 'Diagnostics exposed private-key data.' >&2
  exit 1
fi
stop_daemon

"$binary_path" status --config "$config_path" --format json >"$fixture_root/stopped.json"
grep -Fq '"status": "stopped"' "$fixture_root/stopped.json"

start_daemon
stop_daemon

wal_database="$fixture_root/wal-fixture.db"
set +e
"$binary_path" --w8-fixture crash "$wal_database" >"$fixture_root/wal-crash.log" 2>&1
wal_exit=$?
set -e
if [[ "$wal_exit" -ne 42 ]]; then
  printf 'WAL crash exit code was %d, expected 42.\n' "$wal_exit" >&2
  exit 1
fi
sleep 0.15
"$binary_path" --w8-fixture recover "$wal_database" >"$fixture_root/wal-recover.log" 2>&1
grep -Fq 'delivered exactly once' "$fixture_root/wal-recover.log"

rm -f -- "$database_path-wal" "$database_path-shm"
printf 'corrupt-edge-db-marker' >"$database_path"
corrupt_hash="$(sha256sum "$database_path" | awk '{print $1}')"
if "$binary_path" status --config "$config_path" --format json >"$fixture_root/corrupt.stdout" 2>"$fixture_root/corrupt.log"; then
  echo 'Corrupt database unexpectedly reported status.' >&2
  exit 1
fi
grep -Fq 'edge database is corrupt' "$fixture_root/corrupt.log"
if [[ "$(sha256sum "$database_path" | awk '{print $1}')" != "$corrupt_hash" ]]; then
  echo 'Corrupt database changed without explicit recovery.' >&2
  exit 1
fi
if "$binary_path" recover-db --config "$config_path" >"$fixture_root/unconfirmed.stdout" 2>"$fixture_root/unconfirmed.log"; then
  echo 'Unconfirmed database recovery unexpectedly succeeded.' >&2
  exit 1
fi
grep -Fq 'recovery confirmation is required' "$fixture_root/unconfirmed.log"
if [[ "$(sha256sum "$database_path" | awk '{print $1}')" != "$corrupt_hash" ]]; then
  echo 'Unconfirmed recovery changed the corrupt database.' >&2
  exit 1
fi
"$binary_path" recover-db --config "$config_path" --confirm-reset-development-data >"$fixture_root/recovery.log"
grep -Fq 'development database recovered' "$fixture_root/recovery.log"
if ! compgen -G "$database_path.corrupt-*" >/dev/null; then
  echo 'Explicit recovery did not quarantine the corrupt database.' >&2
  exit 1
fi
"$binary_path" status --config "$config_path" --format json >"$fixture_root/recovered.json"

start_daemon
stop_daemon

[[ "$(stat -c '%a' "$config_path")" == '600' ]]
[[ "$(stat -c '%a' "$key_path")" == '600' ]]
[[ "$(stat -c '%a' "$database_path")" == '600' ]]
[[ "$(stat -c '%a' "$spool_path")" == '700' ]]

service_file="$repo_root/deploy/edge-agent/guardian-edge.service"
grep -Fxq 'User=guardian-edge' "$service_file"
grep -Fxq 'Group=guardian-edge' "$service_file"
grep -Fxq 'NoNewPrivileges=yes' "$service_file"
grep -Fxq 'CapabilityBoundingSet=' "$service_file"
grep -Fxq 'ProtectSystem=strict' "$service_file"
grep -Fxq 'RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6' "$service_file"
grep -Eq '^u guardian-edge - .* /nonexistent /usr/sbin/nologin$' "$repo_root/deploy/edge-agent/guardian-edge.sysusers"

if grep -Fq "$key_path" "$daemon_log" || grep -Fq 'BEGIN PRIVATE KEY' "$daemon_log"; then
  echo 'Daemon logs exposed private-key data.' >&2
  exit 1
fi

echo 'P1-W7 Edge daemon lifecycle, restart, corruption, recovery, permissions, and redaction fixture passed.'
