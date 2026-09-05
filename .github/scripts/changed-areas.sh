#!/usr/bin/env bash
# Decides which quality jobs a change needs.
#
# On anything that is not a pull request (`FULL=true`) every area runs, so a
# push to main, the nightly sweep, and a manual dispatch always exercise the
# complete gate. A pull request runs only the areas its diff touches; the fast
# gate always runs and is not selected here.
set -euo pipefail

emit() {
  echo "$1=$2" >>"${GITHUB_OUTPUT}"
  echo "$1=$2"
}

if [ "${FULL:-false}" = "true" ]; then
  for area in go web contracts container; do emit "${area}" true; done
  exit 0
fi

if ! git cat-file -e "${BASE}^{commit}" 2>/dev/null; then
  echo "base commit ${BASE} unavailable; running every area" >&2
  for area in go web contracts container; do emit "${area}" true; done
  exit 0
fi

changed="$(git diff --name-only "${BASE}" "${HEAD}")"
echo "changed files:"
echo "${changed}" | sed 's/^/  /'

matches() { echo "${changed}" | grep -Eq "$1"; }

# Shared inputs that can break any area.
if matches '^(package-lock\.json|package\.json|Taskfile\.yml|go\.work|\.github/(workflows|scripts)/)'; then
  echo "shared build input changed; running every area" >&2
  for area in go web contracts container; do emit "${area}" true; done
  exit 0
fi

matches '^(apps/(control-plane|edge-agent)/|pkg/|tests/integration/|tests/security/)' \
  && emit go true || emit go false

matches '^(apps/web-console/|tests/e2e/web-console/|openapi/)' \
  && emit web true || emit web false

matches '^(proto/|openapi/|schemas/|buf\.|redocly\.yaml|tools/check-buf)' \
  && emit contracts true || emit contracts false

matches '^(deploy/|decoys/|.*Dockerfile|tools/cowrie-fixture\.sh)' \
  && emit container true || emit container false
