#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture_root="$repository_root/tests/fixtures/frontend-runtime-inventory"

fail() {
  printf 'frontend runtime inventory check: %s\n' "$1" >&2
  return 1
}

validate_relative_path() {
  local relative="$1"
  local component basename
  [[ -n "$relative" ]] || { fail 'inventory contains an empty path'; return 1; }
  [[ "$relative" != /* ]] || { fail "absolute path is prohibited: $relative"; return 1; }
  [[ "$relative" != ./* && "$relative" != *//* && "$relative" != */ ]] || {
    fail "path is not normalized: $relative"
    return 1
  }
  IFS='/' read -r -a components <<<"$relative"
  for component in "${components[@]}"; do
    [[ "$component" != '.' ]] || { fail "path is not normalized: $relative"; return 1; }
    [[ "$component" != '..' ]] || { fail "traversal component is prohibited: $relative"; return 1; }
    case "$component" in
      candidate) fail "candidate component is prohibited: $relative"; return 1 ;;
      test-fixtures) fail "test-fixtures component is prohibited: $relative"; return 1 ;;
      overseer|bindings|binding|wails|native|private|internal)
        fail "privileged component is prohibited: $relative"; return 1 ;;
      dev|development|test|tests)
        fail "development or test component is prohibited: $relative"; return 1 ;;
      proto|protobuf|gen|generated)
        fail "protobuf or generated-source component is prohibited: $relative"; return 1 ;;
    esac
  done

  basename="${relative##*/}"
  if LC_ALL=C grep -Eqi '(^|[-_.])candidate([-_.]|$)' <<<"$basename"; then
    fail "candidate artifact is prohibited: $relative"
    return 1
  fi
  if LC_ALL=C grep -Eqi '(^|[-_.])(overseer|bindings?|wails|native|private|internal)([-_.]|$)' <<<"$basename"; then
    fail "privileged component is prohibited: $relative"
    return 1
  fi
  case "$basename" in
    client.js|sound.js|presentation-uplink.js)
      fail "removed Player bootstrap is prohibited: $relative"; return 1 ;;
    *.ts|*.tsx|*.vue)
      fail "authored source extension is prohibited: $relative"; return 1 ;;
    *.map)
      fail "source map is prohibited: $relative"; return 1 ;;
    *.proto|*_pb.js|*_pb.ts)
      fail "protobuf or generated-source component is prohibited: $relative"; return 1 ;;
  esac

  case "$relative" in
    .keep|index.html|assets/*.js|assets/*.css|assets/*.ttf|sounds/*/*.wav) return 0 ;;
    *) fail "unsupported runtime path: $relative"; return 1 ;;
  esac
}

expect_invalid_path() {
  local relative="$1"
  local expected="$2"
  local output status
  set +e
  output="$(validate_relative_path "$relative" 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *"$expected"* ]] || {
    printf '%s\n' "$output" >&2
    fail "invalid fixture did not fail with expected diagnostic '$expected': $relative"
  }
}

self_test() {
  local relative expected
  while IFS= read -r relative; do
    [[ -z "$relative" ]] || validate_relative_path "$relative"
  done <"$fixture_root/valid-relative-paths.txt"
  while IFS='|' read -r relative expected; do
    [[ -z "$relative" ]] || expect_invalid_path "$relative" "$expected"
  done <"$fixture_root/invalid-relative-paths.txt"
  printf '%s\n' 'frontend runtime inventory check self-test: PASS: every valid and invalid normalized relative-path fixture classified actionably'
}

check_inventory() {
  local dist_root="$1"
  local inventory="$2"
  local required_assets="$3"
  local relative absolute pattern source count matched
  local -a text_files

  [[ -d "$dist_root" && -r "$dist_root" ]] || { fail "dist root is missing or unreadable: $dist_root"; return 1; }
  [[ -f "$inventory" && -r "$inventory" && -s "$inventory" ]] || { fail "inventory is missing, unreadable, or empty: $inventory"; return 1; }
  [[ -f "$required_assets" && -r "$required_assets" && -s "$required_assets" ]] || {
    fail "required-assets file is missing, unreadable, or empty: $required_assets"
    return 1
  }

  text_files=()
  while IFS= read -r relative; do
    validate_relative_path "$relative"
    absolute="$dist_root/$relative"
    [[ -f "$absolute" && -r "$absolute" ]] || { fail "inventory entry is missing or unreadable: $relative"; return 1; }
    [[ -s "$absolute" || "$relative" == '.keep' ]] || { fail "runtime entry is empty: $relative"; return 1; }
    case "$relative" in *.html|*.js|*.css) text_files+=("$absolute") ;; esac
  done <"$inventory"

  [[ -s "$dist_root/index.html" ]] || { fail 'production index.html is missing or empty'; return 1; }
  grep -Eq '^assets/[^/]+\.js$' "$inventory" || { fail 'runtime JavaScript bundle is missing'; return 1; }
  grep -Eq '^assets/[^/]+\.css$' "$inventory" || { fail 'runtime CSS bundle is missing'; return 1; }

  while IFS='|' read -r pattern source; do
    [[ -n "$pattern" && -n "$source" ]] || { fail 'required-assets row is malformed'; return 1; }
    count=0
    matched=''
    while IFS= read -r relative; do
      case "$relative" in
        $pattern) count=$((count + 1)); matched="$relative" ;;
      esac
    done <"$inventory"
    [[ "$count" == 1 ]] || { fail "required asset pattern matched $count entries: $pattern"; return 1; }
    [[ -f "$repository_root/$source" && -r "$repository_root/$source" ]] || { fail "required source asset is missing: $source"; return 1; }
    cmp -s "$repository_root/$source" "$dist_root/$matched" || { fail "runtime asset differs from source: $matched"; return 1; }
    printf 'frontend runtime inventory check: PASS: byte-identical %s <- %s\n' "$matched" "$source"
  done <"$required_assets"

  ((${#text_files[@]} > 0)) || { fail 'no readable HTML/JS/CSS files were selected for content scan'; return 1; }
  "$repository_root/scripts/frontend-assert-no-match.sh" \
    'candidate([-_/.]*(main|player|index\.html))|player[-_/.]*candidate|test[-_/.]*fixtures|client\.js|sound\.js|presentation-uplink\.js|legacyPlayerRoot|playerLegacy|@wailsio/runtime|wailsjs|window\.desktopAPI|fallout/terminal/private' \
    "${text_files[@]}"
  printf 'frontend runtime inventory check: PASS: %d normalized runtime entries; text-scanned %d readable HTML/JS/CSS file(s), no binary assets\n' \
    "$(wc -l <"$inventory" | tr -d ' ')" "${#text_files[@]}"
}

case "${1:-}" in
  --self-test)
    [[ "$#" == 1 ]] || { fail 'usage: frontend-runtime-inventory-check.sh --self-test'; exit 2; }
    self_test
    ;;
  --dist-root)
    [[ "$#" == 6 && "$3" == '--inventory' && "$5" == '--required-assets' ]] || {
      fail 'usage: frontend-runtime-inventory-check.sh --dist-root ROOT --inventory FILE --required-assets FILE'
      exit 2
    }
    check_inventory "$2" "$4" "$6"
    ;;
  *)
    fail 'usage: frontend-runtime-inventory-check.sh --self-test | --dist-root ROOT --inventory FILE --required-assets FILE'
    exit 2
    ;;
esac
