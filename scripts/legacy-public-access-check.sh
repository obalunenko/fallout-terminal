#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() {
  printf 'legacy public-access check: FAIL: %s\n' "$1" >&2
  return 1
}

list_active_files() {
  local scan_root="$1"
  if git -C "${scan_root}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "${scan_root}" ls-files -co --exclude-standard -z
  else
    find "${scan_root}" -type f -print0
  fi
}

is_active_surface() {
  case "$1" in
    .git/*|build/*|node_modules/*|frontend/node_modules/*|frontend/client/node_modules/*|frontend/overseer/node_modules/*|tests/browser/node_modules/*|specs/*|docs/*rollback*.md|scripts/legacy-public-access-check.sh)
      return 1
      ;;
    *)
      return 0
      ;;
  esac
}

record_match() {
  local category="$1"
  local relative="$2"
  printf 'legacy public-access check: FOUND: %s [%s]\n' "${relative}" "${category}" >&2
}

scan_active_tree() {
  local scan_root="$1"
  local file relative
  local found=0
  local env_argument_pattern='NGROK_(BIN|ENABLED|DOMAIN|USERNAME|PASSWORD|BASIC_AUTH)|--ngrok([-=[:space:]]|$)'
  local process_pattern='NewProcessRunner|ProcessRunner|OwnedProcess|darwinOwnerGuardScript|tunnel-guardian|exec\.Command(Context)?\([^\n]*ngrok|LookPath\([^\n]*ngrok'
  local legacy_config_pattern='DefaultDomain|PolicyParent'
  local launch_pattern='NGROK_[A-Z_]+=.*(go run|Fallout Terminal\.app)|ngrok[[:space:]]+http([[:space:]]|$)'
  local root_tunnel_seam_pattern='TunnelEnabled|startTunnelLocked|tunnelStartupFailureMessage|tunnelAddressFailureMessage|public tunnel is enabled but not configured'

  while IFS= read -r -d '' file; do
    if [[ "${file}" = /* ]]; then
      relative="${file#"${scan_root}"/}"
    else
      relative="${file}"
      file="${scan_root}/${file}"
    fi
    is_active_surface "${relative}" || continue
    [[ -f "${file}" ]] || continue
    grep -Iq . "${file}" 2>/dev/null || continue

    if LC_ALL=C grep -Eq -- "${env_argument_pattern}" "${file}"; then
      record_match 'env-or-argument-runtime' "${relative}"
      found=1
    fi
    if LC_ALL=C grep -Eq -- "${process_pattern}" "${file}"; then
      record_match 'external-process-runtime' "${relative}"
      found=1
    fi
    if LC_ALL=C grep -Eq -- "${legacy_config_pattern}" "${file}"; then
      record_match 'legacy-config-or-shared-domain' "${relative}"
      found=1
    fi
    if LC_ALL=C grep -Eq -- "${launch_pattern}" "${file}"; then
      record_match 'active-launch-guidance' "${relative}"
      found=1
    fi
    if [[ "${relative}" == 'app.go' || "${relative}" == 'app_test.go' ]] && LC_ALL=C grep -Eq -- "${root_tunnel_seam_pattern}" "${file}"; then
      record_match 'root-startup-tunnel-seam' "${relative}"
      found=1
    fi
  done < <(list_active_files "${scan_root}")

  return "${found}"
}

scan_package() {
  local app_path="$1"
  local executable_path="${app_path}/Contents/MacOS/Fallout Terminal"
  local bundled_provider

  [[ -d "${app_path}" ]] || return 0
  bundled_provider="$(find "${app_path}/Contents" -type f \( -iname 'ngrok' -o -iname 'ngrok.exe' -o -iname 'ngrok-*' \) -print -quit 2>/dev/null || true)"
  if [[ -n "${bundled_provider}" ]]; then
    record_match 'bundled-provider-executable' "${bundled_provider#"${repository_root}"/}"
    return 1
  fi
  if [[ -x "${executable_path}" ]] && strings "${executable_path}" | grep -Eq 'NGROK_BIN|DefaultDomain|tunnel-guardian'; then
    record_match 'packaged-legacy-runtime-string' "${executable_path#"${repository_root}"/}"
    return 1
  fi
  return 0
}

check_tree() {
  local scan_root="$1"
  local app_path="${2:-}"
  local diagnostic_only="${3:-0}"
  local failed=0

  scan_active_tree "${scan_root}" || failed=1
  if [[ -n "${app_path}" ]]; then
    scan_package "${app_path}" || failed=1
  fi
  if [[ "${failed}" == 1 ]]; then
    if [[ "${diagnostic_only}" == 1 ]]; then
      printf 'Legacy public-access diagnostic completed: known pre-cutover findings remain; strict final gate was not run.\n'
      return 0
    fi
    fail 'legacy CLI/process/PATH runtime, root startup tunnel seam, or active launch guidance remains'
    return 1
  fi
  printf 'Legacy public-access check passed: one embedded production runtime; no CLI, process, PATH, root startup tunnel seam, shared-domain, active env/argument guidance, or bundled provider path remains.\n'
}

self_test() {
  local fixture
  fixture="$(mktemp -d "${TMPDIR:-/tmp}/fallout-legacy-public-access.XXXXXX")"
  trap 'rm -rf "${fixture}"' RETURN
  mkdir -p "${fixture}/internal/tunnel" "${fixture}/scripts" "${fixture}/specs/historical" "${fixture}/build/bin/Fallout Terminal.app/Contents/MacOS"
  printf 'module example.test/app\n' >"${fixture}/go.mod"
  printf 'package tunnel\n// embedded provider only\n' >"${fixture}/internal/tunnel/ngrok.go"
  printf '# Historical NGROK_BIN evidence\n' >"${fixture}/specs/historical/spec.md"
  check_tree "${fixture}"

  printf 'package tunnel\nfunc legacy() { _ = NewProcessRunner() }\n' >"${fixture}/internal/tunnel/process.go"
  if check_tree "${fixture}" >/dev/null 2>&1; then
    fail 'self-test accepted an external process runner'
  fi
  rm "${fixture}/internal/tunnel/process.go"

  printf 'NGROK_ENABLED=1 go run ./cmd/build dev\n' >"${fixture}/README.md"
  if check_tree "${fixture}" >/dev/null 2>&1; then
    fail 'self-test accepted active env launch guidance'
  fi
  rm "${fixture}/README.md"

  printf 'package main\ntype AppDependencies struct { TunnelEnabled bool }\n' >"${fixture}/app.go"
  if check_tree "${fixture}" >/dev/null 2>&1; then
    fail 'self-test accepted the dormant root startup tunnel seam'
  fi
  rm "${fixture}/app.go"

  printf '#!/bin/sh\n' >"${fixture}/build/bin/Fallout Terminal.app/Contents/MacOS/ngrok"
  if scan_package "${fixture}/build/bin/Fallout Terminal.app" >/dev/null 2>&1; then
    fail 'self-test accepted a bundled provider executable'
  fi

  printf 'Legacy public-access detector self-test passed.\n'
}

case "${1:-}" in
  --self-test)
    [[ "$#" -eq 1 ]] || { fail 'usage: legacy-public-access-check.sh [--self-test|--diagnose]'; exit 2; }
    self_test
    ;;
  --diagnose)
    [[ "$#" -eq 1 ]] || { fail 'usage: legacy-public-access-check.sh [--self-test|--diagnose]'; exit 2; }
    check_tree "${repository_root}" "${repository_root}/build/bin/Fallout Terminal.app" 1
    ;;
  '')
    check_tree "${repository_root}" "${repository_root}/build/bin/Fallout Terminal.app"
    ;;
  *)
    fail 'usage: legacy-public-access-check.sh [--self-test|--diagnose]'
    exit 2
    ;;
esac
