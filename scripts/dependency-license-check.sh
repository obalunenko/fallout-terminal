#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
module_file="$repository_root/go.mod"
task_module_file="$repository_root/tools/task/go.mod"
wails_module_file="$repository_root/tools/wails/go.mod"
notice_file="$repository_root/THIRD_PARTY_NOTICES.md"

readonly ngrok_module='golang.ngrok.com/ngrok/v2'
readonly ngrok_version='v2.1.4'
readonly keychain_module='github.com/keybase/go-keychain'
readonly keychain_version='v0.0.1'
readonly wincred_module='github.com/danieljoos/wincred'
readonly wincred_version='v1.2.3'
readonly dbus_module='github.com/godbus/dbus/v5'
readonly dbus_version='v5.2.2'
readonly wails_module='github.com/wailsapp/wails/v3'
readonly wails_version='v3.0.0-beta.15'
readonly task_module='github.com/go-task/task/v3'
readonly task_version='v3.53.1'
readonly shipped_targets=(
  'darwin/arm64'
  'windows/amd64'
  'windows/arm64'
  'linux/amd64'
  'linux/arm64'
)

fail() {
  printf 'dependency/license check: %s\n' "$1" >&2
  return 1
}

require_exact_pin() {
  local source_file="$1"
  local module="$2"
  local expected="$3"
  local actual

  [[ -f "$source_file" ]] || {
    fail "${source_file#"$repository_root"/} is missing"
    return 1
  }

  actual="$(awk -v wanted="$module" '$1 == wanted { print $2 }' "$source_file")"
  [[ "$actual" == "$expected" ]] || {
    fail "$module must be pinned exactly at $expected in ${source_file#"$repository_root"/} (found ${actual:-missing})"
    return 1
  }
}

resolved_runtime_modules() {
  local target
  local target_os
  local target_arch

  for target in "${shipped_targets[@]}"; do
    target_os="${target%/*}"
    target_arch="${target#*/}"
    (
      cd "$repository_root"
      env GOOS="$target_os" GOARCH="$target_arch" CGO_ENABLED=0 \
        go list -mod=readonly -deps \
        -f '{{with .Module}}{{if .Version}}{{.Path}}@{{.Version}}{{end}}{{end}}' \
        .
    )
  done | sed '/^$/d' | sort -u
}

resolved_reviewed_modules() {
  {
    resolved_runtime_modules
    printf '%s@%s\n' "$task_module" "$task_version"
  } | sort -u
}

check_pins() {
  local pin_file

  for pin_file in "$module_file" "$task_module_file" "$wails_module_file"; do
    [[ -f "$pin_file" ]] || {
      fail "${pin_file#"$repository_root"/} is missing"
      return 1
    }
    if grep -En '(@latest|[[:space:]]latest([[:space:]]|$))' "$pin_file"; then
      fail "${pin_file#"$repository_root"/} contains a floating dependency version"
      return 1
    fi
  done

  require_exact_pin "$module_file" "$ngrok_module" "$ngrok_version"
  require_exact_pin "$module_file" "$keychain_module" "$keychain_version"
  require_exact_pin "$module_file" "$wincred_module" "$wincred_version"
  require_exact_pin "$module_file" "$dbus_module" "$dbus_version"
  require_exact_pin "$module_file" "$wails_module" "$wails_version"
  require_exact_pin "$task_module_file" "$task_module" "$task_version"
  require_exact_pin "$wails_module_file" "$wails_module" "$wails_version"
}

check_notices() {
  local module_version
  local missing=0
  local reviewed_list="$1"

  [[ -f "$notice_file" ]] || {
    fail 'THIRD_PARTY_NOTICES.md is missing'
    return 1
  }

  while IFS= read -r module_version; do
    [[ -n "$module_version" ]] || continue
    if ! grep -Fq -- "- $module_version —" "$notice_file"; then
      printf 'dependency/license check: missing reviewed notice inventory entry for %s\n' \
        "$module_version" >&2
      missing=1
    fi
  done <"$reviewed_list"

  [[ "$missing" == 0 ]] || return 1
  grep -Fq '## ngrok-go' "$notice_file" || fail 'missing ngrok-go notice text'
  grep -Fq '## go-keychain' "$notice_file" || fail 'missing go-keychain notice text'
  grep -Fq '## Windows Credential Manager (wincred)' "$notice_file" || fail 'missing wincred notice text'
  grep -Fq '## D-Bus / Secret Service (godbus/dbus)' "$notice_file" || fail 'missing godbus/dbus notice text'
  grep -Fq '## Go Task' "$notice_file" || fail 'missing Go Task notice text'
  grep -Fq '## MIT License terms' "$notice_file" || fail 'missing MIT license terms'
  grep -Fq '## BSD-2-Clause License terms' "$notice_file" || fail 'missing BSD-2-Clause license terms'
  grep -Fq '## BSD-3-Clause License terms' "$notice_file" || fail 'missing BSD-3-Clause license terms'
  grep -Fq '## Apache License 2.0 terms' "$notice_file" || fail 'missing Apache-2.0 license terms'
}

check_tree() {
  local scratch
  scratch="$(mktemp -d)"
  trap 'rm -rf "$scratch"' RETURN

  check_pins

  resolved_reviewed_modules >"$scratch/reviewed-modules"
  check_notices "$scratch/reviewed-modules"

  printf 'Shipped target runtime and build-tool dependency inventory:\n'
  sed 's/^/  /' "$scratch/reviewed-modules"
  printf 'Dependency pins and reviewed notices are complete.\n'
}

case "${1:-}" in
  '')
    check_tree
    ;;
  --list)
    check_pins
    resolved_reviewed_modules
    ;;
  *)
    fail "unknown argument: $1"
    ;;
esac
