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

file_mode_and_size() {
  local file="$1"
  if stat -f '%Lp %z' "$file" >/dev/null 2>&1; then
    stat -f '%Lp %z' "$file"
  else
    stat -c '%a %s' "$file"
  fi
}

tree_manifest() {
  local path="$1"
  local destination="$2"
  local file relative mode_size digest
  [[ -d "$path" && -r "$path" ]] || fail "missing or unreadable build output: $path"
  : >"$destination"
  while IFS= read -r -d '' file; do
    relative="${file#"$path/"}"
    mode_size="$(file_mode_and_size "$file")"
    digest="$(shasum -a 256 "$file" | awk '{print $1}')"
    printf '%s\t%s\t%s\n' "$relative" "$mode_size" "$digest" >>"$destination"
  done < <(find "$path" -type f -print0 | LC_ALL=C sort -z)
  [[ -s "$destination" ]] || fail "build output contains no files: $path"
}

compare_trees() {
  local first="$1"
  local second="$2"
  local label="$3"
  local first_manifest="$temporary/${label}.first.manifest"
  local second_manifest="$temporary/${label}.second.manifest"
  tree_manifest "$first" "$first_manifest"
  tree_manifest "$second" "$second_manifest"
  if ! cmp -s "$first_manifest" "$second_manifest"; then
    diff -u "$first_manifest" "$second_manifest" >&2 || true
    fail "frontend reproducibility mismatch in $label"
  fi
  printf 'reproducible build check: PASS: %s manifest %s\n' \
    "$label" "$(shasum -a 256 "$first_manifest" | awk '{print $1}')"
}

capture_frontend_build() {
  local destination="$1"
  npm run build:overseer --prefix frontend
  npm run build:client --prefix frontend
  mkdir -p "$destination/overseer" "$destination/player"
  cp -R frontend/overseer/dist/. "$destination/overseer/"
  cp -R frontend/client/dist/. "$destination/player/"
}

check_frontend_reproducibility() {
  local scratch="$temporary/frontend"
  mkdir -p "$scratch/first" "$scratch/second"
  capture_frontend_build "$scratch/first"
  capture_frontend_build "$scratch/second"
  compare_trees "$scratch/first/overseer" "$scratch/second/overseer" overseer-vite
  compare_trees "$scratch/first/player" "$scratch/second/player" player-vite
  rm -rf "$scratch"
}

frontend_self_test() {
  local scratch="$temporary/self-test"
  local output status
  check_frontend_reproducibility

  mkdir -p "$scratch/expected" "$scratch/matching" "$scratch/mismatched"
  cp tests/fixtures/frontend-reproducibility/expected-tree.txt "$scratch/expected/tree.txt"
  cp tests/fixtures/frontend-reproducibility/expected-tree.txt "$scratch/matching/tree.txt"
  cp tests/fixtures/frontend-reproducibility/mismatched-tree.txt "$scratch/mismatched/tree.txt"
  compare_trees "$scratch/expected" "$scratch/matching" fixture-equal

  set +e
  output="$(compare_trees "$scratch/expected" "$scratch/mismatched" fixture-mismatch 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'frontend reproducibility mismatch in fixture-mismatch'* ]] \
    || fail 'deliberate copied-tree mismatch was not rejected actionably'

  rm -rf "$scratch"
  [[ ! -e "$scratch" ]] || fail 'self-test scratch directory was not removed'
  printf '%s\n' 'reproducible build check self-test: PASS: two Vite builds matched and a deliberate copied-tree mismatch failed actionably'
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

case "${1:-}" in
  --frontend)
    [[ "$#" == 1 ]] || fail 'usage: reproducible-build-check.sh [--frontend|--self-test]'
    check_frontend_reproducibility
    exit
    ;;
  --self-test)
    [[ "$#" == 1 ]] || fail 'usage: reproducible-build-check.sh [--frontend|--self-test]'
    frontend_self_test
    exit
    ;;
  '')
    ;;
  *)
    fail 'usage: reproducible-build-check.sh [--frontend|--self-test]'
    ;;
esac

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
