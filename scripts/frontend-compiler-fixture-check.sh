#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture_root="$repository_root/tests/fixtures/frontend-compiler"

fail() {
  printf 'frontend compiler fixture check: %s\n' "$1" >&2
  return 1
}

expect_compiler_diagnostic() {
  local relative_file="$1"
  local expected_code="$2"
  local temporary_directory config output status codes
  temporary_directory="$(mktemp -d "$fixture_root/.compiler-check.XXXXXX")"
  config="$temporary_directory/tsconfig.json"
  trap 'rm -rf "$temporary_directory"' RETURN
  printf '{\n  "extends": "%s/frontend/tsconfig.base.json",\n  "files": ["%s/%s"]\n}\n' \
    "$repository_root" "$fixture_root" "$relative_file" >"$config"

  set +e
  output="$(cd "$repository_root" && npx --prefix frontend vue-tsc --noEmit -p "$config" 2>&1)"
  status=$?
  set -e
  ((status != 0)) || { fail "$relative_file unexpectedly compiled"; return 1; }
  codes="$(printf '%s\n' "$output" | sed -n 's/.*error TS\([0-9][0-9]*\):.*/\1/p' | LC_ALL=C sort -u)"
  [[ "$codes" == "$expected_code" ]] || {
    printf '%s\n' "$output" >&2
    fail "$relative_file produced diagnostic codes ${codes:-none}; expected only TS$expected_code"
    return 1
  }
  printf 'frontend compiler fixture check: PASS: %s rejected with TS%s\n' "$relative_file" "$expected_code"
  rm -rf "$temporary_directory"
  trap - RETURN
}

expect_policy_diagnostic() {
  local relative_file="$1"
  local expected_code="$2"
  local pattern="$3"
  local file="$fixture_root/$relative_file"
  [[ -f "$file" && -r "$file" ]] || { fail "policy fixture is missing or unreadable: $relative_file"; return 1; }
  if ! LC_ALL=C rg --line-number --color never -- "$pattern" "$file" >/dev/null; then
    fail "$relative_file did not exercise $expected_code"
    return 1
  fi
  printf '%s: %s rejected by strict frontend source policy\n' "$expected_code" "$relative_file"
}

self_test() {
  command -v node >/dev/null 2>&1 || { fail 'required tool is missing: node'; return 1; }
  command -v npx >/dev/null 2>&1 || { fail 'required tool is missing: npx'; return 1; }
  command -v rg >/dev/null 2>&1 || { fail 'required tool is missing: rg'; return 1; }
  [[ -x "$repository_root/frontend/node_modules/.bin/vue-tsc" ]] || { fail 'vue-tsc is missing; run task deps:frontend'; return 1; }

  (cd "$repository_root" && npx --prefix frontend vue-tsc --noEmit -p tests/fixtures/frontend-compiler/tsconfig.valid.json)
  printf '%s\n' 'frontend compiler fixture check: PASS: strict TypeScript, strict Vue SFC, and application-owned declaration compiled'

  expect_compiler_diagnostic invalid/invalid-props.vue 2322
  expect_compiler_diagnostic invalid/invalid-emits.vue 2345
  expect_compiler_diagnostic invalid/invalid-unchecked-index.ts 2532
  expect_compiler_diagnostic invalid/invalid-exact-optional.ts 2375
  expect_policy_diagnostic invalid/invalid-any.ts FTS1001 '(^|[^[:alnum:]_])any([^[:alnum:]_]|$)'
  expect_policy_diagnostic invalid/invalid-suppression.ts FTS1002 '@ts-(nocheck|ignore|expect-error)'
  expect_policy_diagnostic invalid/invalid-cross-boundary-import.ts FTS1003 'frontend/overseer|overseer/src|\.\./\.\./\.\./\.\./frontend/overseer'
  printf '%s\n' 'frontend compiler fixture check self-test: PASS: valid fixtures compiled and all seven invalid categories were rejected exactly'
}

if [[ "${1:-}" == '--self-test' && "$#" == 1 ]]; then
  self_test
  exit
fi

fail 'usage: frontend-compiler-fixture-check.sh --self-test'
exit 2
