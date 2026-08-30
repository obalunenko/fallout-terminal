#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() {
  printf 'frontend expected RED check: %s\n' "$1" >&2
  return 1
}

usage() {
  printf 'usage: %s --task ID --evidence PATH --expect EXACT_TEXT -- COMMAND [ARG...]\n' "$0" >&2
  return 2
}

run_expected_red() {
  local task_id="$1"
  local evidence="$2"
  local expected="$3"
  shift 3
  local output_file status signature_count assertion_count evidence_parent command_text

  [[ -n "$task_id" ]] || { fail 'task ID must not be empty'; return 1; }
  [[ "$evidence" != /* ]] || { fail 'evidence path must be repository-relative'; return 1; }
  [[ -n "$expected" ]] || { fail 'expected signature must not be empty'; return 1; }
  (($# > 0)) || { fail 'product command is missing'; return 1; }

  output_file="$(mktemp "${TMPDIR:-/tmp}/fallout-frontend-red.XXXXXX")"
  trap 'rm -f "$output_file"' RETURN

  set +e
  (cd "$repository_root" && "$@") >"$output_file" 2>&1
  status=$?
  set -e

  ((status != 0)) || { fail 'product command unexpectedly passed; RED was not demonstrated'; return 1; }

  if LC_ALL=C grep -Eqi \
    'command not found|no such file or directory|cannot find (module|package)|ERR_MODULE_NOT_FOUND|configuration error|config file.*(missing|not found)|fixture[- ]server.*fail|webserver.*fail|browser.*(failed|not found|executable)|executable doesn.t exist|connection refused|target page.*closed|timed? out|timeout exceeded|SyntaxError' \
    "$output_file"; then
    cat "$output_file" >&2
    fail 'product command failed because of infrastructure, configuration, browser, timeout, or syntax failure'
    return 1
  fi

  signature_count="$(LC_ALL=C grep -Fxc -- "$expected" "$output_file" || true)"
  [[ "$signature_count" == 1 ]] || {
    cat "$output_file" >&2
    fail "expected exactly one assertion signature: $expected"
    return 1
  }
  assertion_count="$(LC_ALL=C grep -Ec '(^|[[:space:]])AssertionError:' "$output_file" || true)"
  [[ "$assertion_count" == 1 ]] || {
    cat "$output_file" >&2
    fail 'an unrelated or duplicate assertion failure accompanied the expected RED'
    return 1
  }

  evidence_parent="$(dirname "$repository_root/$evidence")"
  [[ -d "$evidence_parent" ]] || { fail "evidence directory is missing: ${evidence_parent#"$repository_root/"}"; return 1; }
  command_text="$(printf '%q ' "$@")"
  {
    printf 'Task: %s\n' "$task_id"
    printf 'Expected assertion: %s\n' "$expected"
    printf 'Product command: %s\n' "${command_text% }"
    printf 'Product exit status: %s\n' "$status"
    printf '%s\n' '--- captured stdout/stderr ---'
    cat "$output_file"
  } >"$repository_root/$evidence"

  printf 'frontend expected RED check: PASS: %s produced only the expected assertion and evidence %s\n' \
    "$task_id" "$evidence"
  rm -f "$output_file"
  trap - RETURN
}

self_test() {
  local expected_evidence output status
  expected_evidence="specs/033-frontend-vue-typescript-migration/evidence/.T003-red-self-test.txt"
  trap 'rm -f "$repository_root/$expected_evidence"' RETURN

  run_expected_red SELFTEST "$expected_evidence" \
    'AssertionError: expected product behavior is missing' \
    tests/fixtures/frontend-expected-red/expected-product-failure.sh
  [[ -s "$repository_root/$expected_evidence" ]] || fail 'expected RED evidence was not written to the exact requested path'
  rm -f "$repository_root/$expected_evidence"

  set +e
  output="$(run_expected_red SELFTEST "$expected_evidence" \
    'AssertionError: expected product behavior is missing' \
    tests/fixtures/frontend-expected-red/unrelated-failure.sh 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'expected exactly one assertion signature'* ]] || fail 'unrelated assertion failure was not rejected'

  set +e
  output="$(run_expected_red SELFTEST "$expected_evidence" \
    'AssertionError: expected product behavior is missing' \
    tests/fixtures/frontend-expected-red/infrastructure-failure.sh 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'failed because of infrastructure'* ]] || fail 'infrastructure failure was not rejected'

  set +e
  output="$(run_expected_red SELFTEST "$expected_evidence" \
    'AssertionError: expected product behavior is missing' true 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'unexpectedly passed'* ]] || fail 'successful product command was not rejected'

  printf '%s\n' 'frontend expected RED check self-test: PASS: expected assertion accepted; unrelated, infrastructure, and zero-status results rejected'
  rm -f "$repository_root/$expected_evidence"
  trap - RETURN
}

if [[ "${1:-}" == '--self-test' ]]; then
  [[ "$#" == 1 ]] || usage
  self_test
  exit
fi

task_id=''
evidence=''
expected=''
while (($# > 0)); do
  case "$1" in
    --task) (($# >= 2)) || usage; task_id="$2"; shift 2 ;;
    --evidence) (($# >= 2)) || usage; evidence="$2"; shift 2 ;;
    --expect) (($# >= 2)) || usage; expected="$2"; shift 2 ;;
    --) shift; break ;;
    *) usage ;;
  esac
done
run_expected_red "$task_id" "$evidence" "$expected" "$@"
