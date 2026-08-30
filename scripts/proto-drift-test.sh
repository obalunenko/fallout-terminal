#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repository_root"

fail() {
  printf 'protobuf drift test: %s\n' "$1" >&2
  exit 1
}

usage() {
  fail 'usage: proto-drift-test.sh --target PATH --expect-diagnostic EXACT_TEXT'
}

target=''
expected_diagnostic=''
while (($# > 0)); do
  case "$1" in
    --target)
      (($# >= 2)) || usage
      target="$2"
      shift 2
      ;;
    --expect-diagnostic)
      (($# >= 2)) || usage
      expected_diagnostic="$2"
      shift 2
      ;;
    *)
      usage
      ;;
  esac
done

required_target='frontend/client/gen/fallout/terminal/player/v1/player_pb.ts'
[[ "$target" == "$required_target" ]] || fail "target must be exactly $required_target"
[[ "$expected_diagnostic" == "generated protobuf drift: $required_target" ]] \
  || fail 'expected diagnostic does not name the exact generated TypeScript target'
[[ -f "$target" && -r "$target" ]] || fail "target is missing or unreadable: $target"
[[ -w "$target" ]] || fail "target is not writable for the deliberate drift probe: $target"
[[ -x scripts/proto-check.sh ]] || fail 'required checker is missing or not executable: scripts/proto-check.sh'
[[ -r proto/buf.gen.es.yaml && -r proto/buf.gen.go.yaml ]] || fail 'protobuf generator configuration is missing or unreadable'
for tool in go node npm shasum git; do
  command -v "$tool" >/dev/null 2>&1 || fail "required tool is missing: $tool"
done

temporary="$(mktemp -d "${TMPDIR:-/tmp}/fallout-proto-drift.XXXXXX")"
backup="$temporary/player_pb.ts"
output_file="$temporary/proto-check.txt"
cp -p "$target" "$backup"

restore_target() {
  cp -p "$backup" "$target"
}

cleanup() {
  if [[ -f "$backup" ]]; then
    restore_target
  fi
  rm -rf "$temporary"
}
trap cleanup EXIT HUP INT TERM

owned_manifest() {
  local destination="$1"
  {
    find proto/fallout/terminal/player/v1 internal/gen frontend/client/gen -type f -print | LC_ALL=C sort
    printf '%s\n' proto/buf.gen.es.yaml proto/buf.gen.go.yaml proto/schema-revision.txt go.mod go.sum
  } | while IFS= read -r owned; do
    [[ -f "$owned" && -r "$owned" ]] || fail "owned protobuf input/output is missing or unreadable: $owned"
    printf '%s\t%s\n' "$owned" "$(shasum -a 256 "$owned" | awk '{print $1}')"
  done >"$destination"
}

owned_manifest "$temporary/before.manifest"
printf '\n// deliberate generated TypeScript drift fixture\n' >>"$target"

set +e
scripts/proto-check.sh >"$output_file" 2>&1
status=$?
set -e

((status != 0)) || fail 'proto-check unexpectedly accepted deliberate generated TypeScript drift'
if LC_ALL=C grep -Eqi \
  'command not found|no such file or directory|permission denied|operation not permitted|timed? out|timeout|cannot find (module|package)|configuration error|syntax error' \
  "$output_file"; then
  cat "$output_file" >&2
  fail 'proto-check failed because of infrastructure, permission, timeout, dependency, or configuration failure'
fi
if ! grep -Fq 'checked-in generated contracts drift from a clean pinned-Buf generation' "$output_file" \
  || ! grep -Fq "$target" "$output_file"; then
  cat "$output_file" >&2
  fail 'proto-check failed with an unrelated or wrong drift diagnostic'
fi

restore_target
cmp -s "$backup" "$target" || fail 'generated TypeScript target was not restored byte-for-byte'
owned_manifest "$temporary/after.manifest"
if ! cmp -s "$temporary/before.manifest" "$temporary/after.manifest"; then
  diff -u "$temporary/before.manifest" "$temporary/after.manifest" >&2 || true
  fail 'drift verification changed an unrelated protobuf input or generated output'
fi

printf '%s\n' "$expected_diagnostic"
printf '%s\n' 'protobuf drift test: PASS: exact generated TypeScript drift rejected and all owned bytes restored'

rm -rf "$temporary"
trap - EXIT HUP INT TERM
