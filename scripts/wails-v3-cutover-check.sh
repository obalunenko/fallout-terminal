#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() {
  printf 'wails-v3 cutover check: %s\n' "$1" >&2
  return 1
}

task_block() {
  local taskfile="$1"
  local task_name="$2"

  awk -v task_name="$task_name" '
    $0 ~ "^  " task_name ":" { in_task = 1 }
    in_task && $0 ~ /^  [A-Za-z0-9:_-]+:/ && $0 !~ "^  " task_name ":" { exit }
    in_task { print }
  ' "$taskfile"
}

check_task_orchestration() {
  local root="$1"
  local taskfile="${root}/Taskfile.yml"
  local taskfiles task_name task_definition matches make_targets

  taskfiles="$(find "${root}" -type d \( -name .git -o -name node_modules \) -prune -o \
    -type f \( -name Taskfile.yml -o -name Taskfile.yaml \) -print)"
  [[ "${taskfiles}" == "${taskfile}" ]] || {
    printf '%s\n' "${taskfiles}" >&2
    fail 'root Taskfile.yml must be the only active Taskfile'
    return 1
  }
  grep -Eq "^[[:space:]]*version:[[:space:]]*['\"]?3['\"]?[[:space:]]*$" "${taskfile}" || {
    fail 'root Taskfile.yml does not declare schema version 3'
    return 1
  }

  for task_name in dev build package; do
    task_definition="$(task_block "${taskfile}" "${task_name}")"
    [[ -n "${task_definition}" ]] || {
      fail "root Taskfile.yml is missing the canonical ${task_name} task"
      return 1
    }
    printf '%s\n' "${task_definition}" | grep -Eq "(go|\{\{\.GO\}\})[[:space:]]+run[[:space:]]+\./cmd/build[[:space:]]+${task_name}([[:space:]\"']|$)" || {
      fail "canonical ${task_name} task does not delegate to cmd/build"
      return 1
    }
  done

  for task_name in build package; do
    task_definition="$(task_block "${taskfile}" "${task_name}")"
    printf '%s\n' "${task_definition}" | grep -Fq '{{.GOOS}}' &&
      printf '%s\n' "${task_definition}" | grep -Fq '{{.GOARCH}}' || {
      fail "canonical ${task_name} task does not translate Wails GOOS/GOARCH variables"
      return 1
    }
  done

  matches="$(LC_ALL=C grep -IEn '(^|[[:space:]`;&|])(wails3|wails)[[:space:]]+(dev|build|package|task)([[:space:]]|$)' "${taskfile}" 2>/dev/null || true)"
  [[ -z "${matches}" ]] || {
    printf '%s\n' "${matches}" >&2
    fail 'root Taskfile.yml can recurse through a high-level Wails command'
    return 1
  }

  [[ -f "${root}/Makefile" ]] || { fail 'Makefile tool bootstrap is missing'; return 1; }
  make_targets="$(LC_ALL=C grep -IEn '^[[:alnum:]_-]+:' "${root}/Makefile" 2>/dev/null | grep -Ev '^[0-9]+:(tools|help):' || true)"
  [[ -z "${make_targets}" ]] || {
    printf '%s\n' "${make_targets}" >&2
    fail 'Makefile contains a parallel workflow target'
    return 1
  }
  grep -Eq '^tools:' "${root}/Makefile" || { fail 'Makefile does not expose the tools bootstrap'; return 1; }
  grep -Eq '^help:' "${root}/Makefile" || { fail 'Makefile does not expose non-mutating help'; return 1; }
}

scan_tree() {
  local root="$1"
  local matches

  for obsolete in wails.json build/darwin/postbuild.sh production_resources_bindings.go; do
    [[ ! -e "${root}/${obsolete}" ]] || { fail "obsolete active artifact remains: ${obsolete}"; return 1; }
  done
  check_task_orchestration "${root}" || return

  matches="$(find "${root}" \( -path '*/.git' -o -path '*/node_modules' -o -path '*/specs' \) -prune -o -type f -name '*.go' ! -name '*_test.go' \
    -exec grep -EnH 'github\.com/wailsapp/wails/v2|WAILS_V2|USE_WAILS_V2|legacyWails|dual.?runtime' {} + || true)"
  [[ -z "${matches}" ]] || { printf '%s\n' "${matches}" >&2; fail 'active Go source contains v2 or dual-runtime code'; return 1; }

  matches="$(find "${root}" \( -path '*/.git' -o -path '*/node_modules' -o -path '*/specs' -o -path '*/tools' \) -prune -o -type f -name '*.go' \
    -exec grep -EnH '^[[:space:]]*(import[[:space:]]+)?([._[:alnum:]]+[[:space:]]+)?"github\.com/obalunenko/Fallout-Terminal([^"]*)"' {} + 2>/dev/null | \
    grep -Ev '"github\.com/obalunenko/Fallout-Terminal/v2(/[^"]*)?"' || true)"
  [[ -z "${matches}" ]] || { printf '%s\n' "${matches}" >&2; fail 'active Go source contains an unsuffixed application import'; return 1; }

  if grep -En 'github\.com/wailsapp/wails/v2' "${root}/go.mod" "${root}/go.sum"; then
    fail 'application module still resolves Wails v2'
    return 1
  fi

  matches="$(grep -ERIn 'frontend/(overseer/)?wailsjs|window\.(go|runtime)|electronAPI|WAILS_V2|USE_WAILS_V2|legacyWails|dual.?runtime' \
    "${root}/frontend/overseer/src" "${root}/frontend/overseer/bindings" "${root}/frontend/overseer/dist" 2>/dev/null || true)"
  [[ -z "${matches}" ]] || { printf '%s\n' "${matches}" >&2; fail 'frontend source/generated/bundle contains a v2 global or dual-runtime fallback'; return 1; }

  matches="$(find "${root}/README.md" "${root}/scripts" "${root}/.github/workflows" -type f \
    ! -name 'wails-v3-cutover-check.sh' ! -name 'wails-bindings-check.sh' ! -name 'verify-macos-app.sh' \
    ! -name 'wails-v3-contract-check.sh' ! -name 'tool-modules-check.sh' ! -name 'frontend-policy-check.sh' \
    -exec grep -EnH \
      'go[[:space:]]+install[[:space:]]+github\.com/wailsapp/wails|go[[:space:]]+run[[:space:]]+\./cmd/build[[:space:]]+(dev|build|package)([[:space:]]|$)|(^|[[:space:]`;&|])make[[:space:]]+(dev|build|package)([[:space:]]|$)|(^|[[:space:]`;&|])wails[[:space:]]+(dev|build|generate)([[:space:]]|$)|@wailsio/runtime.*(latest|\^|~|\*)|github\.com/wailsapp/wails/v3@latest' \
      {} + 2>/dev/null || true)"
  [[ -z "${matches}" ]] || { printf '%s\n' "${matches}" >&2; fail 'active command/documentation bypasses Task or uses v2, global, or floating Wails resolution'; return 1; }

  [[ -f "${root}/specs/001-wails-v2-migration/spec.md" ]] || { fail 'historical Wails v2 spec is missing'; return 1; }
  [[ -f "${root}/docs/wails-migration-rollback.md" ]] || { fail 'historical Electron-to-Wails rollback record is missing'; return 1; }
  grep -Eq 'specs/006-wails-v3-migration/quickstart\.md' "${root}/README.md" || { fail 'README does not link the active Wails v3 quickstart'; return 1; }
  grep -Eq 'docs/wails-v3-migration-rollback\.md' "${root}/README.md" || { fail 'README does not link the active Wails v3 rollback record'; return 1; }
  grep -Eqi 'histor|истор' "${root}/README.md" || { fail 'README does not identify earlier migration records as history'; return 1; }
}

self_test() {
  local fixture
  fixture="$(mktemp -d "${TMPDIR:-/tmp}/fallout-cutover-check.XXXXXX")"
  trap 'rm -rf "${fixture}"' RETURN
  mkdir -p "${fixture}/build/darwin" "${fixture}/frontend/overseer/src" "${fixture}/frontend/overseer/bindings" "${fixture}/frontend/overseer/dist" \
    "${fixture}/internal/app" "${fixture}/scripts" "${fixture}/.github/workflows" \
    "${fixture}/specs/001-wails-v2-migration" "${fixture}/specs/099-completed" \
    "${fixture}/tools/helper" "${fixture}/docs"
  printf 'module example.test/app\n\ngo 1.27.0\n\nrequire github.com/wailsapp/wails/v3 v3.0.0-beta.15\n' >"${fixture}/go.mod"
  : >"${fixture}/go.sum"
  printf 'tools:\n\t@go install tool\nhelp:\n\t@printf '\''Run task --list.\\n'\''\n' >"${fixture}/Makefile"
  printf '%s\n' \
    "version: '3'" \
    'tasks:' \
    '  dev:' \
    '    cmds:' \
    '      - go run ./cmd/build dev' \
    '  build:' \
    '    cmds:' \
    '      - go run ./cmd/build build --target "{{.GOOS}}/{{.GOARCH}}"' \
    '  package:' \
    '    cmds:' \
    '      - go run ./cmd/build package --target "{{.GOOS}}/{{.GOARCH}}"' >"${fixture}/Taskfile.yml"
  printf 'package main\nimport (\n\t_ "github.com/obalunenko/Fallout-Terminal/v2/internal/domain"\n\t_ "github.com/wailsapp/wails/v3/pkg/application"\n)\nconst repository = "https://github.com/obalunenko/Fallout-Terminal"\n' >"${fixture}/main.go"
  printf 'package main\nimport _ "github.com/obalunenko/Fallout-Terminal/internal/toolfixture"\n' >"${fixture}/tools/helper/main.go"
  printf 'package history\nimport _ "github.com/obalunenko/Fallout-Terminal/internal/history"\n' >"${fixture}/specs/099-completed/example.go"
  printf 'export const ready = true;\n' >"${fixture}/frontend/overseer/src/app.js"
  printf 'export const generated = true;\n' >"${fixture}/frontend/overseer/bindings/service.js"
  printf '<!doctype html>\n' >"${fixture}/frontend/overseer/dist/index.html"
  printf 'Active: specs/006-wails-v3-migration/quickstart.md and docs/wails-v3-migration-rollback.md. Earlier records are historical evidence.\n' >"${fixture}/README.md"
  printf '%s\n' \
    "forbidden='@wailsio/runtime.*latest'" \
    'printf "%s\\n" "$forbidden" >/dev/null' >"${fixture}/scripts/frontend-policy-check.sh"
  printf '# Historical v2 spec\n' >"${fixture}/specs/001-wails-v2-migration/spec.md"
  printf '# Historical rollback\n' >"${fixture}/docs/wails-migration-rollback.md"
  scan_tree "${fixture}"

  printf 'package app\nimport _ "github.com/wailsapp/wails/v2"\n' >"${fixture}/internal/app/app.go"
  if scan_tree "${fixture}" >/dev/null 2>&1; then fail 'self-test accepted an active v2 Go import'; return 1; fi
  printf 'package app\n' >"${fixture}/internal/app/app.go"

  printf 'package app\nimport _ "github.com/obalunenko/Fallout-Terminal/internal/domain"\n' >"${fixture}/internal/app/app.go"
  if scan_tree "${fixture}" >/dev/null 2>&1; then fail 'self-test accepted an unsuffixed active application import'; return 1; fi
  printf 'package app\n' >"${fixture}/internal/app/app.go"

  printf '{}\n' >"${fixture}/wails.json"
  if scan_tree "${fixture}" >/dev/null 2>&1; then fail 'self-test accepted wails.json'; return 1; fi
  rm "${fixture}/wails.json"

  printf 'window.go.main.App();\n' >"${fixture}/frontend/overseer/src/app.js"
  if scan_tree "${fixture}" >/dev/null 2>&1; then fail 'self-test accepted a generated v2 global'; return 1; fi
  printf 'export const ready = true;\n' >"${fixture}/frontend/overseer/src/app.js"

  mkdir -p "${fixture}/nested"
  printf "version: '3'\n" >"${fixture}/nested/Taskfile.yml"
  if scan_tree "${fixture}" >/dev/null 2>&1; then fail 'self-test accepted a parallel Taskfile'; return 1; fi
  rm "${fixture}/nested/Taskfile.yml"

  cp "${fixture}/Taskfile.yml" "${fixture}/Taskfile.valid.yml"
  printf '\n      - go tool -modfile=tools/wails/go.mod wails3 package\n' >>"${fixture}/Taskfile.yml"
  if scan_tree "${fixture}" >/dev/null 2>&1; then fail 'self-test accepted Wails-to-Task recursion'; return 1; fi
  mv "${fixture}/Taskfile.valid.yml" "${fixture}/Taskfile.yml"

  printf '\ngo run ./cmd/build package\n' >>"${fixture}/README.md"
  if scan_tree "${fixture}" >/dev/null 2>&1; then fail 'self-test accepted a direct-Go public package command'; return 1; fi
  printf 'Active: specs/006-wails-v3-migration/quickstart.md and docs/wails-v3-migration-rollback.md. Earlier records are historical evidence.\n' >"${fixture}/README.md"

  printf 'wails build\n' >>"${fixture}/README.md"
  if scan_tree "${fixture}" >/dev/null 2>&1; then fail 'self-test accepted a bare v2 Wails command'; return 1; fi

  printf 'Wails v3 cutover scan self-test passed.\n'
}

case "${1:-}" in
  --self-test)
    [[ "$#" -eq 1 ]] || { fail 'usage: scripts/wails-v3-cutover-check.sh [--self-test]'; exit 1; }
    self_test
    ;;
  '')
    scan_tree "${repository_root}"
    "${repository_root}/scripts/tool-modules-check.sh"
    "${repository_root}/scripts/wails-bindings-check.sh"
    git -C "${repository_root}" diff --exit-code -- specs/001-wails-v2-migration docs/wails-migration-rollback.md
    if go -C "${repository_root}" list -m all | grep -En '^github\.com/wailsapp/wails/v2([[:space:]]|$)'; then
      fail 'resolved module graph still contains Wails v2'
      exit 1
    fi
    printf 'Wails v3 cutover scan passed: no active v2, dual-runtime, floating-tool, generated-global, dependency, bundle, script, CI, or operating-document surface remains.\n'
    ;;
  *)
    fail 'usage: scripts/wails-v3-cutover-check.sh [--self-test]'
    exit 1
    ;;
esac
