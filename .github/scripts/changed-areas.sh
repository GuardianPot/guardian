#!/usr/bin/env bash
# Decides which quality jobs a change needs.
#
# On anything that is not a pull request (`FULL=true`) every area runs, so a
# push to main, the nightly sweep, and a manual dispatch always exercise the
# complete gate. A pull request runs only the areas its diff touches; the fast
# gate always runs and is not selected here.
#
# Only a change to the CI definition itself forces every area. Task and package
# manifests select the areas that actually consume them, so a console change
# that also edits Taskfile.yml no longer drags in the Go and container suites.
set -euo pipefail

emit() {
  echo "$1=$2" >>"${GITHUB_OUTPUT}"
  echo "$1=$2"
}

run_everything() {
  echo "$1" >&2
  for area in go web contracts container; do emit "${area}" true; done
  exit 0
}

if [ "${FULL:-false}" = "true" ]; then
  run_everything "not a pull request; running every area"
fi

if ! git cat-file -e "${BASE}^{commit}" 2>/dev/null; then
  run_everything "base commit ${BASE} unavailable; running every area"
fi

changed="$(git diff --name-only "${BASE}" "${HEAD}")"
echo "changed files:"
echo "${changed}" | sed 's/^/  /'

matches() { echo "${changed}" | grep -Eq "$1"; }

# The workflow and its selection script decide what runs at all, so a change to
# either is verified against everything.
if matches '^\.github/(workflows|scripts)/'; then
  run_everything "CI definition changed; running every area"
fi

# Node manifests drive the web and contract tooling only. Go and container
# builds do not read them.
node_manifest=false
if matches '^(package\.json|package-lock\.json|Taskfile\.yml)$'; then
  node_manifest=true
fi

select_area() {
  local area="$1" pattern="$2" manifest_sensitive="$3"
  if matches "${pattern}"; then
    emit "${area}" true
  elif [ "${manifest_sensitive}" = true ] && [ "${node_manifest}" = true ]; then
    emit "${area}" true
  else
    emit "${area}" false
  fi
}

select_area go '^(apps/(control-plane|edge-agent)/|pkg/|tests/integration/|tests/security/|go\.work)' false
select_area web '^(apps/web-console/|tests/e2e/web-console/|openapi/)' true
select_area contracts '^(proto/|openapi/|schemas/|buf\.|redocly\.yaml|tools/check-buf)' true
select_area container '^(deploy/|decoys/|.*Dockerfile|tools/cowrie-fixture\.sh)' false
