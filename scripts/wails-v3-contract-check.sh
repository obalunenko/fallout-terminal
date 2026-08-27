#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() {
  printf 'wails-v3 contract check: %s\n' "$1" >&2
  return 1
}

list_scan_files() {
  local scan_root="$1"

  if git -C "$scan_root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$scan_root" ls-files -co --exclude-standard -z
  else
    find "$scan_root" -type f -print0
  fi
}

scan_files() {
  local scan_root="$1"
  local pattern="$2"
  local exclude_tests="${3:-false}"
  local file
  local relative_file
  local file_matches

  while IFS= read -r -d '' file; do
    if [[ "$file" = /* ]]; then
      relative_file="${file#"$scan_root"/}"
    else
      relative_file="$file"
      file="$scan_root/$file"
    fi

    case "$relative_file" in
      scripts/wails-v3-contract-check.sh|scripts/wails-v3-cutover-check.sh|scripts/tool-modules-check.sh|specs/*|docs/wails-migration-rollback.md|node_modules/*|frontend/node_modules/*|frontend/client/node_modules/*|frontend/overseer/node_modules/*|tests/browser/node_modules/*)
        continue
        ;;
    esac
    if [[ "$exclude_tests" == true && "$relative_file" == *_test.go ]]; then
      continue
    fi

    file_matches="$(LC_ALL=C grep -IEn "$pattern" "$file" 2>/dev/null || true)"
    while IFS= read -r match; do
      [[ -n "$match" ]] && printf '%s:%s\n' "$relative_file" "$match"
    done <<<"$file_matches"
  done < <(list_scan_files "$scan_root")
}

scan_unqualified_commands() {
  local scan_root="$1"
  local matches

  matches="$(scan_files "$scan_root" '(^|[[:space:]`;&|])(wails3|wails)[[:space:]]+(dev|build|package|generate)([[:space:]]|$)' true)"
  matches="$(printf '%s\n' "$matches" | grep -Ev 'go tool -modfile=tools/wails/go\.mod wails3 ' || true)"
  [[ -z "$matches" ]] || {
    printf '%s\n' "$matches" >&2
    fail 'active files contain an unqualified Wails command'
  }
}

scan_floating_versions() {
  local scan_root="$1"
  local matches

  matches="$(scan_files "$scan_root" '(@wailsio/[A-Za-z0-9_./-]+"?[[:space:]]*:[[:space:]]*"(latest|\^|~|\*)|github\.com/wailsapp/wails(/v3)?@latest|go[[:space:]]+install[[:space:]]+github\.com/wailsapp/wails[^[:space:]]*@latest)')"
  [[ -z "$matches" ]] || {
    printf '%s\n' "$matches" >&2
    fail 'active files contain a floating Wails version'
  }
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

scan_build_orchestration() {
  local scan_root="$1"
  local taskfile="$scan_root/Taskfile.yml"
  local taskfiles task_name task_definition forbidden_commands make_targets

  taskfiles="$(find "$scan_root" \
    -type d \( -name node_modules -o -name .git \) -prune -o \
    -type f \( -name 'Taskfile.yml' -o -name 'Taskfile.yaml' \) -print)"
  [[ "$taskfiles" == "$taskfile" ]] || {
    printf '%s\n' "$taskfiles" >&2
    fail 'root Taskfile.yml must be the only active Taskfile'
    return 1
  }

  grep -Eq "^[[:space:]]*version:[[:space:]]*['\"]?3['\"]?[[:space:]]*$" "$taskfile" || {
    fail 'root Taskfile.yml does not declare schema version 3'
    return 1
  }

  for task_name in dev build package; do
    task_definition="$(task_block "$taskfile" "$task_name")"
    [[ -n "$task_definition" ]] || {
      fail "root Taskfile.yml is missing the canonical $task_name task"
      return 1
    }
    printf '%s\n' "$task_definition" | grep -Eq "(go|\{\{\.GO\}\})[[:space:]]+run[[:space:]]+\./cmd/build[[:space:]]+$task_name([[:space:]\"']|$)" || {
      fail "canonical $task_name task does not delegate to cmd/build"
      return 1
    }
  done

  for task_name in build package; do
    task_definition="$(task_block "$taskfile" "$task_name")"
    printf '%s\n' "$task_definition" | grep -Fq '{{.GOOS}}' &&
      printf '%s\n' "$task_definition" | grep -Fq '{{.GOARCH}}' || {
      fail "canonical $task_name task does not translate Wails GOOS/GOARCH variables"
      return 1
    }
  done

  forbidden_commands="$(LC_ALL=C grep -IEn '(^|[[:space:]`;&|])(wails3|wails)[[:space:]]+(dev|build|package|task)([[:space:]]|$)' "$taskfile" 2>/dev/null || true)"
  [[ -z "$forbidden_commands" ]] || {
    printf '%s\n' "$forbidden_commands" >&2
    fail 'root Taskfile.yml can recurse through a high-level Wails command'
    return 1
  }

  forbidden_commands="$(scan_files "$scan_root" 'go[[:space:]]+run[[:space:]]+\./cmd/build[[:space:]]+(dev|build|package)([[:space:]]|$)' true)"
  forbidden_commands="$(printf '%s\n' "$forbidden_commands" | grep -Ev '^Taskfile\.yml:' || true)"
  [[ -z "$forbidden_commands" ]] || {
    printf '%s\n' "$forbidden_commands" >&2
    fail 'active files bypass the canonical Task dev/build/package graph'
    return 1
  }

  [[ -f "$scan_root/Makefile" ]] || {
    fail 'Makefile tool bootstrap is missing'
    return 1
  }
  make_targets="$(LC_ALL=C grep -IEn '^[[:alnum:]_-]+:' "$scan_root/Makefile" 2>/dev/null | grep -Ev '^[0-9]+:(tools|help):' || true)"
  [[ -z "$make_targets" ]] || {
    printf '%s\n' "$make_targets" >&2
    fail 'Makefile contains a parallel workflow target'
    return 1
  }
  grep -Eq '^tools:' "$scan_root/Makefile" || {
    fail 'Makefile does not expose the tools bootstrap'
    return 1
  }
  grep -Eq '^help:' "$scan_root/Makefile" || {
    fail 'Makefile does not expose non-mutating help'
    return 1
  }

  forbidden_commands="$(scan_files "$scan_root" '(^|[[:space:]`;&|])make[[:space:]]+(dev|build|package)([[:space:]]|$)' true)"
  [[ -z "$forbidden_commands" ]] || {
    printf '%s\n' "$forbidden_commands" >&2
    fail 'active files advertise a superseded Make workflow command'
    return 1
  }
}

scan_root_module() {
  local scan_root="$1"
  local root_module="$scan_root/go.mod"

  [[ -f "$root_module" ]] || fail 'root go.mod is missing'

  if grep -En '^tool[[:space:]]*(\(|[^[:space:]])' "$root_module"; then
    fail 'root application go.mod contains a tool declaration'
    return 1
  fi

  if grep -En '^[[:space:]]*(github\.com/bufbuild/buf|google\.golang\.org/protobuf/cmd/protoc-gen-go|connectrpc\.com/connect/cmd/protoc-gen-connect-go)([[:space:]]|$)' "$root_module"; then
    fail 'root application go.mod contains a tool-only dependency'
    return 1
  fi
}

scan_lifecycle_schema() {
  local scan_root="$1"
  local runtime_schema="$scan_root/proto/fallout/terminal/private/v1/runtime.proto"

  [[ -f "$runtime_schema" ]] || fail 'private runtime schema is missing'
  if grep -En '(LifecyclePhase|lifecycle_phase|lifecyclePhase|^[[:space:]]*(optional[[:space:]]+)?(string|int32|int64|uint32|uint64|[A-Za-z_][A-Za-z0-9_.]*)[[:space:]]+phase[[:space:]]*=)' "$runtime_schema"; then
    fail 'private runtime schema contains a serialized lifecycle phase'
  fi

  local runtime_digest revision_digest baseline_digest
  runtime_digest="$(shasum -a 256 "$runtime_schema" | awk '{print $1}')"
  revision_digest="$(shasum -a 256 "$scan_root/proto/schema-revision.txt" | awk '{print $1}')"
  baseline_digest="$(shasum -a 256 "$scan_root/proto/compatibility-baseline.binpb" | awk '{print $1}')"
  [[ "$runtime_digest" == 4fd0b3ef31bd7ada1101ae36bfbd749acd36c53c4bc2da185d33dec4d4c669a9 ]] || {
    fail 'reviewed private runtime schema changed unexpectedly'
    return 1
  }
  [[ "$revision_digest" == 4211c6a2f25e6ebf72aa2b28c2b564d45b9d5df077ffa62f55c25f9de2864522 ]] || {
    fail 'reviewed schema revision record changed unexpectedly'
    return 1
  }
  [[ "$baseline_digest" == 50b88cc9e08a189012925e1a97094d1e097b223e591aca8acb856ba0daf099f3 ]] || {
    fail 'reviewed feature-007 compatibility baseline changed unexpectedly'
    return 1
  }
}

check_tree() {
  local scan_root="$1"

  scan_unqualified_commands "$scan_root" || return
  scan_build_orchestration "$scan_root" || return
  scan_floating_versions "$scan_root" || return
  scan_root_module "$scan_root" || return
  scan_lifecycle_schema "$scan_root" || return
}

self_test() {
  local fixture_root
  fixture_root="$(mktemp -d)"
  trap 'rm -rf "$fixture_root"' RETURN

  mkdir -p "$fixture_root/proto/fallout/terminal/private/v1" "$fixture_root/docs"
  printf 'module example.test/app\n\ngo 1.27.0\n' >"$fixture_root/go.mod"
  printf 'tools:\n\t@go install tool\nhelp:\n\t@printf '\''Run task --list.\\n'\''\n' >"$fixture_root/Makefile"
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
    '      - go run ./cmd/build package --target "{{.GOOS}}/{{.GOARCH}}"' >"$fixture_root/Taskfile.yml"
  cp "$repository_root/proto/fallout/terminal/private/v1/runtime.proto" "$fixture_root/proto/fallout/terminal/private/v1/runtime.proto"
  cp "$repository_root/proto/schema-revision.txt" "$fixture_root/proto/schema-revision.txt"
  cp "$repository_root/proto/compatibility-baseline.binpb" "$fixture_root/proto/compatibility-baseline.binpb"
  printf 'task dev\n' >"$fixture_root/docs/commands.md"
  check_tree "$fixture_root"

  printf 'go tool -modfile=tools/wails/go.mod wails3 dev\n' >"$fixture_root/docs/commands.md"
  check_tree "$fixture_root"

  printf 'wails3 dev\n' >"$fixture_root/docs/commands.md"
  if check_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted an unqualified Wails command'
  fi

  printf 'task dev\n"@wailsio/runtime": "latest"\n' >"$fixture_root/docs/commands.md"
  if check_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted a floating Wails version'
  fi

  printf 'task dev\n' >"$fixture_root/docs/commands.md"
  printf 'module example.test/app\n\ngo 1.27.0\n\ntool github.com/bufbuild/buf/cmd/buf\n' >"$fixture_root/go.mod"
  if check_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted a root tool declaration'
  fi

  printf 'module example.test/app\n\ngo 1.27.0\n' >"$fixture_root/go.mod"
  mkdir -p "$fixture_root/nested"
  printf "version: '3'\n" >"$fixture_root/nested/Taskfile.yml"
  if check_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted a parallel Taskfile'
  fi
  rm "$fixture_root/nested/Taskfile.yml"

  printf 'go run ./cmd/build package\n' >"$fixture_root/docs/commands.md"
  if check_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted a public direct-Go package command'
  fi
  printf 'task package GOOS=linux GOARCH=amd64\n' >"$fixture_root/docs/commands.md"

  cp "$fixture_root/Taskfile.yml" "$fixture_root/Taskfile.valid.yml"
  printf '\n      - go tool -modfile=tools/wails/go.mod wails3 package\n' >>"$fixture_root/Taskfile.yml"
  if check_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted Wails-to-Task recursion'
  fi
  mv "$fixture_root/Taskfile.valid.yml" "$fixture_root/Taskfile.yml"

  printf '\nmessage LifecycleFixture { string lifecycle_phase = 1; }\n' >>"$fixture_root/proto/fallout/terminal/private/v1/runtime.proto"
  if check_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted a serialized lifecycle phase'
  fi

  printf 'wails-v3 contract check self-test passed\n'
}

case "${1:-}" in
  --self-test)
    self_test
    ;;
  '')
    check_tree "$repository_root"
    printf 'Wails v3 migration contracts are qualified, pinned, tool-isolated, and schema-stable.\n'
    ;;
  *)
    fail "unknown argument: $1"
    ;;
esac
