#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_root}"

application_bundle="build/bin/Fallout Terminal.app"
application_executable="${application_bundle}/Contents/MacOS/Fallout Terminal"
temporary="$(mktemp -d "${TMPDIR:-/tmp}/fallout-reproducible-build.XXXXXX")"
trap 'rm -rf "${temporary}"' EXIT HUP INT TERM

fail() {
  printf 'reproducible build check: %s\n' "$1" >&2
  exit 1
}

tree_digest() {
  local path="$1"
  [[ -e "${path}" ]] || fail "missing build output: ${path}"
  find "${path}" -type f -print0 \
    | LC_ALL=C sort -z \
    | while IFS= read -r -d '' file; do
        printf '%s\0' "${file#${path}/}"
        stat -f '%Lp' "${file}"
        shasum -a 256 "${file}"
      done \
    | shasum -a 256 \
    | awk '{print $1}'
}

tracked_state() {
  local destination="$1"
  {
    git status --porcelain=v1 --untracked-files=all
    git diff --no-ext-diff --binary -- .
  } >"${destination}"
}

run_once() {
  local run="$1"

  go tool -modfile=tools/task/go.mod task package
  tree_digest internal/gen >"${temporary}/${run}.internal-gen"
  tree_digest frontend/client/gen >"${temporary}/${run}.client-gen"
  tree_digest frontend/overseer/bindings >"${temporary}/${run}.bindings"
  tree_digest frontend/client/dist >"${temporary}/${run}.client-dist"
  tree_digest frontend/overseer/dist >"${temporary}/${run}.overseer-dist"
  shasum -a 256 "${application_executable}" | awk '{print $1}' >"${temporary}/${run}.native"
  scripts/verify-macos-app.sh "${application_bundle}"
  scripts/hash-macos-app.sh "${application_bundle}" >"${temporary}/${run}.app"
}

scripts/tool-modules-check.sh
scripts/wails-v3-contract-check.sh
scripts/dependency-license-check.sh
scripts/secret-leak-check.sh --self-test
scripts/secret-leak-check.sh
scripts/legacy-public-access-check.sh --self-test
scripts/legacy-public-access-check.sh --diagnose
tracked_state "${temporary}/before"

run_once first
run_once second

for output in internal-gen client-gen bindings client-dist overseer-dist native app; do
  cmp "${temporary}/first.${output}" "${temporary}/second.${output}" \
    || fail "two clean runs produced different ${output} output"
done

tracked_state "${temporary}/after"
cmp "${temporary}/before" "${temporary}/after" \
  || fail 'the repeated build changed tracked or untracked repository state'

printf 'Two complete protobuf, player, binding, native, and inspected unsigned app builds were byte-reproducible with exact dependency/license and leak gates and zero repository drift.\n'
