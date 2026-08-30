#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
default_config="$repository_root/tests/browser/playwright.config.mjs"

fail() {
  printf 'frontend focused browser check: %s\n' "$1" >&2
  return 1
}

escaped_suffix() {
  node -e 'process.stdout.write(process.argv[1].replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))' "$1"
}

run_focused() {
  local spec="$1"
  local suffix="$2"
  local config="$3"
  local escaped list_output list_status count run_status

  [[ "$spec" != /* && "$spec" != *'..'* ]] || { fail 'spec path must be an exact repository-relative path without traversal'; return 1; }
  [[ "$spec" == *.spec.mjs ]] || { fail 'spec path must end in .spec.mjs'; return 1; }
  [[ -f "$repository_root/$spec" && -r "$repository_root/$spec" ]] || { fail "spec is missing or unreadable: $spec"; return 1; }
  [[ -n "$suffix" ]] || { fail 'literal title suffix must not be empty'; return 1; }
  [[ -f "$config" && -r "$config" ]] || { fail "list infrastructure failure: Playwright config is missing or unreadable: $config"; return 1; }
  [[ -x "$repository_root/tests/browser/node_modules/.bin/playwright" ]] || { fail 'list infrastructure failure: Playwright executable is missing'; return 1; }

  escaped="$(escaped_suffix "$suffix")"
  set +e
  list_output="$(cd "$repository_root" && tests/browser/node_modules/.bin/playwright test \
    --config "$config" --list --grep "${escaped}\$" "$spec" 2>&1)"
  list_status=$?
  set -e
  if ((list_status != 0)) && ! { [[ "$list_output" == *'No tests found.'* ]] && [[ "$list_output" == *'Total: 0 tests in 0 files'* ]]; }; then
    printf '%s\n' "$list_output" >&2
    fail "list infrastructure failure (status $list_status)"
    return 1
  fi

  count="$(printf '%s\n' "$list_output" | sed -n 's/^Total: \([0-9][0-9]*\) tests\{0,1\} in .*$/\1/p' | tail -n 1)"
  [[ "$count" =~ ^[0-9]+$ ]] || { printf '%s\n' "$list_output" >&2; fail 'list infrastructure failure: could not parse Playwright test count'; return 1; }
  [[ "$count" == 1 ]] || { printf '%s\n' "$list_output" >&2; fail "focused selector matched $count tests; expected exactly 1"; return 1; }

  printf '%s\n' "$list_output"
  set +e
  (cd "$repository_root" && tests/browser/node_modules/.bin/playwright test \
    --config "$config" --grep "${escaped}\$" "$spec")
  run_status=$?
  set -e
  return "$run_status"
}

self_test() {
  local temporary_directory temporary_config output status
  temporary_directory="$(mktemp -d "$repository_root/tests/browser/.frontend-focused-self-test.XXXXXX")"
  temporary_config="$temporary_directory/playwright.config.mjs"
  trap 'rm -rf "$temporary_directory"' RETURN
  printf '%s\n' \
    "export default { testDir: '../../fixtures/frontend-focused-browser', fullyParallel: false, workers: 1 };" \
    >"$temporary_config"

  run_focused tests/fixtures/frontend-focused-browser/one-match.spec.mjs \
    'focused browser exact target' "$temporary_config"

  set +e
  output="$(run_focused tests/fixtures/frontend-focused-browser/one-match.spec.mjs \
    'missing focused suffix' "$temporary_config" 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'matched 0 tests; expected exactly 1'* ]] || fail 'zero-match selector was not rejected'

  set +e
  output="$(run_focused tests/fixtures/frontend-focused-browser/multiple-match.spec.mjs \
    'shared focused suffix' "$temporary_config" 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'matched 2 tests; expected exactly 1'* ]] || fail 'multiple-match selector was not rejected'

  set +e
  output="$(run_focused tests/fixtures/frontend-focused-browser/one-match.spec.mjs \
    'focused browser exact target' "$temporary_config.missing" 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'list infrastructure failure'* ]] || fail 'list infrastructure failure was not classified'

  rm -rf "$temporary_directory"
  trap - RETURN
  printf '%s\n' 'frontend focused browser check self-test: PASS: one match executed; zero, multiple, and list-infrastructure cases rejected'
}

if [[ "${1:-}" == '--self-test' ]]; then
  [[ "$#" == 1 ]] || { fail 'usage: frontend-focused-browser-check.sh --self-test'; exit 2; }
  self_test
  exit
fi

[[ "$#" == 2 ]] || { fail 'usage: frontend-focused-browser-check.sh SPEC.spec.mjs LITERAL_TITLE_SUFFIX'; exit 2; }
run_focused "$1" "$2" "$default_config"
