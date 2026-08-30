#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() {
  printf 'frontend no-match check: %s\n' "$1" >&2
  return 1
}

check_no_match() {
  local pattern="$1"
  shift
  local path output status

  command -v rg >/dev/null 2>&1 || { fail 'required tool is missing: rg'; return 1; }
  [[ -n "$pattern" ]] || { fail 'pattern must not be empty'; return 1; }
  (($# > 0)) || { fail 'at least one retained file is required'; return 1; }
  for path in "$@"; do
    [[ -f "$path" ]] || { fail "retained file is missing: $path"; return 1; }
    [[ -r "$path" ]] || { fail "retained file is unreadable: $path"; return 1; }
  done

  set +e
  output="$(LC_ALL=C rg --line-number --color never -- "$pattern" "$@" 2>&1)"
  status=$?
  set -e
  case "$status" in
    0)
      printf '%s\n' "$output" >&2
      fail "forbidden match found for pattern: $pattern"
      return 1
      ;;
    1)
      printf 'frontend no-match check: PASS: pattern absent from %s readable file(s)\n' "$#"
      ;;
    *)
      printf '%s\n' "$output" >&2
      fail "rg failed with status $status for pattern: $pattern"
      return 1
      ;;
  esac
}

self_test() {
  local output status unreadable
  check_no_match 'forbidden-token' tests/fixtures/frontend-no-match/no-match.txt

  set +e
  output="$(check_no_match 'forbidden-token' tests/fixtures/frontend-no-match/forbidden-match.txt 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'forbidden match found'* ]] || fail 'forbidden-match fixture was not rejected'

  set +e
  output="$(check_no_match 'forbidden-token' tests/fixtures/frontend-no-match/missing.txt 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'retained file is missing'* ]] || fail 'missing-file fixture was not rejected'

  unreadable="$(mktemp "${TMPDIR:-/tmp}/fallout-frontend-unreadable.XXXXXX")"
  trap 'chmod 600 "$unreadable" 2>/dev/null || true; rm -f "$unreadable"' RETURN
  chmod 000 "$unreadable"
  set +e
  output="$(check_no_match 'forbidden-token' "$unreadable" 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'retained file is unreadable'* ]] || fail 'unreadable-file fixture was not rejected'

  set +e
  output="$(check_no_match '[' tests/fixtures/frontend-no-match/no-match.txt 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'rg failed with status'* ]] || fail 'invalid regular expression was not rejected as a tool error'

  chmod 600 "$unreadable"
  rm -f "$unreadable"
  trap - RETURN
  printf '%s\n' 'frontend no-match check self-test: PASS: no-match, match, missing, unreadable, and invalid-regex cases classified'
}

if [[ "${1:-}" == '--self-test' ]]; then
  [[ "$#" == 1 ]] || { fail 'usage: frontend-assert-no-match.sh --self-test'; exit 2; }
  self_test
  exit
fi

(($# >= 2)) || { fail 'usage: frontend-assert-no-match.sh PATTERN FILE [FILE...]'; exit 2; }
check_no_match "$@"
