#!/usr/bin/env bash

set -euo pipefail

export LC_ALL=C

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tool_check_go="${TOOL_MODULES_CHECK_GO:-go}"
tool_module_files=()

fail() {
  printf 'tool module check: %s\n' "$1" >&2
  return 1
}

discover_tool_modules() {
  local scan_root="$1"
  local module_file

  tool_module_files=()
  shopt -s nullglob
  for module_file in "$scan_root"/tools/*/go.mod; do
    tool_module_files+=("$module_file")
  done
  shopt -u nullglob

  ((${#tool_module_files[@]} > 0)) || {
    fail 'no tool modules found at tools/*/go.mod'
    return 1
  }
}

check_tool_module_contract() {
  local scan_root="$1"
  local module_file="$2"
  local expected_module_prefix="$3"
  local relative_module="${module_file#"$scan_root"/}"
  local directory="${relative_module#tools/}"
  local actual_module
  local sum_file="${module_file%/go.mod}/go.sum"
  local tool_count

  directory="${directory%/go.mod}"
  actual_module="$(awk '$1 == "module" && NF == 2 { print $2 }' "$module_file")"
  [[ "$actual_module" == "$expected_module_prefix/$directory" ]] || {
    fail "$relative_module must retain independent identity $expected_module_prefix/$directory"
    return 1
  }
  [[ -s "$sum_file" ]] || { fail "missing or empty ${relative_module%/go.mod}/go.sum"; return 1; }
  grep -Eq '^go[[:space:]]+[0-9]+\.[0-9]+([.][0-9]+)?$' "$module_file" || {
    fail "$relative_module has no explicit Go version"
    return 1
  }
  tool_count="$(awk '$1 == "tool" && NF == 2 && $2 != "(" { count++ } END { print count + 0 }' "$module_file")"
  [[ "$tool_count" == 1 ]] || {
    fail "$relative_module must declare exactly one direct tool"
    return 1
  }
  if grep -Eq '^tool[[:space:]]*\(' "$module_file"; then
    fail "$relative_module must use one direct tool directive, not a tool block"
    return 1
  fi
}

check_tool_pin() {
  local scan_root="$1"
  local directory="$2"
  local command_package="$3"
  local parent_module="$4"
  local version="$5"
  local module_file="$scan_root/tools/$directory/go.mod"

  [[ -f "$module_file" ]] || { fail "missing tools/$directory/go.mod"; return 1; }
  grep -Eq "^tool[[:space:]]+${command_package//\//\\/}$" "$module_file" || {
    fail "tools/$directory/go.mod does not own $command_package"
    return 1
  }
  grep -Eq "^[[:space:]]*(require[[:space:]]+)?${parent_module//\//\\/}[[:space:]]+${version//./\\.}([[:space:]]+// indirect)?$" "$module_file" || {
    fail "tools/$directory/go.mod does not pin $parent_module $version"
    return 1
  }
}

check_root_module() {
  local scan_root="$1"
  local expected_module="$2"
  local root_module="$scan_root/go.mod"
  local actual_module

  [[ -f "$root_module" ]] || { fail 'root go.mod is missing'; return 1; }
  actual_module="$(awk '$1 == "module" && NF == 2 { print $2 }' "$root_module")"
  [[ "$actual_module" == "$expected_module" ]] || {
    fail "root go.mod must declare $expected_module"
    return 1
  }
  if grep -En '^tool[[:space:]]*(\(|[^[:space:]])|github\.com/bufbuild/buf|github\.com/caarlos0/svu|github\.com/go-task/task/v3/cmd/task|github\.com/golangci/golangci-lint|github\.com/goreleaser/goreleaser|github\.com/wailsapp/wails/v3/cmd/wails3|google\.golang\.org/protobuf/cmd/protoc-gen-go|connectrpc\.com/connect/cmd/protoc-gen-connect-go|oras\.land/oras' "$root_module"; then
    fail 'root application go.mod contains a tool declaration or tool-only dependency'
    return 1
  fi
}

check_make_bootstrap() {
  local scan_root="$1"
  local makefile="$scan_root/Makefile"
  local targets

  [[ -f "$makefile" ]] || { fail 'root Makefile is missing'; return 1; }
  grep -Fqx 'TOOL_MODULES := $(sort $(wildcard tools/*/go.mod))' "$makefile" || {
    fail 'Makefile must discover immediate tool modules in deterministic path order'
    return 1
  }
  grep -Fqx '.DEFAULT_GOAL := tools' "$makefile" || {
    fail 'Makefile default goal must be tools'
    return 1
  }
  [[ "$(grep -Fxc '.PHONY: tools help' "$makefile" || true)" == 1 ]] || {
    fail 'Makefile must declare tools and help as phony'
    return 1
  }
  grep -Fq 'module_dir="$${module_file%/go.mod}"' "$makefile" || {
    fail 'Makefile tools target must enter each discovered module'
    return 1
  }
  grep -Fq '(cd "$$module_dir" && $(GO) install tool)' "$makefile" || {
    fail 'Makefile tools target must run go install tool in each discovered module'
    return 1
  }

  targets="$(awk -F: '/^[[:alnum:]][[:alnum:]_.-]*:/ && $0 !~ /^[^:]+:=/ { print $1 }' "$makefile")"
  [[ "$targets" == $'tools\nhelp' ]] || {
    fail 'Makefile must expose only the tools and help targets'
    return 1
  }
  grep -Fq 'task --list' "$makefile" || {
    fail 'Makefile help must direct maintainers to the Task workflow list'
    return 1
  }
}

file_signature() {
  local file="$1"

  if [[ -f "$file" ]]; then
    cksum "$file"
  else
    printf '<missing>\n'
  fi
}

check_tool_resolution_preserves_root() {
  local scan_root="$1"
  local root_module="$scan_root/go.mod"
  local root_sum="$scan_root/go.sum"
  local module_file
  local relative_module
  local tool_package
  local module_before
  local sum_before
  local resolution_failed=0

  module_before="$(file_signature "$root_module")"
  sum_before="$(file_signature "$root_sum")"

  for module_file in "${tool_module_files[@]}"; do
    relative_module="${module_file#"$scan_root"/}"
    tool_package="$(awk '$1 == "tool" && NF == 2 && $2 != "(" { print $2 }' "$module_file")"
    if ! (cd "$scan_root" && "$tool_check_go" list -mod=readonly -modfile="$relative_module" "$tool_package" >/dev/null); then
      fail "failed to resolve the tool owned by $relative_module"
      resolution_failed=1
      break
    fi
  done

  if [[ "$(file_signature "$root_module")" != "$module_before" || "$(file_signature "$root_sum")" != "$sum_before" ]]; then
    fail 'tool resolution modified the root go.mod or go.sum'
    return 1
  fi
  ((resolution_failed == 0))
}

list_scan_files() {
  local scan_root="$1"

  if git -C "$scan_root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$scan_root" ls-files -co --exclude-standard -z
  else
    find "$scan_root" -type f -print0
  fi
}

check_active_commands() {
  local scan_root="$1"
  local file
  local relative_file
  local file_matches
  local matches=''
  local forbidden_pattern='(go[[:space:]]+install[[:space:]]+(github\.com/wailsapp/wails|github\.com/caarlos0/svu/v3|github\.com/go-task/task/v3/cmd/task|github\.com/bufbuild/buf/cmd/buf|github\.com/golangci/golangci-lint/v2/cmd/golangci-lint|github\.com/goreleaser/goreleaser/v2|google\.golang\.org/protobuf/cmd/protoc-gen-go|connectrpc\.com/connect/cmd/protoc-gen-connect-go|oras\.land/oras/cmd/oras)|go[[:space:]]+run[[:space:]]+(github\.com/caarlos0/svu/v3|github\.com/golangci/golangci-lint/v2/cmd/golangci-lint|github\.com/goreleaser/goreleaser/v2|oras\.land/oras/cmd/oras)|go[[:space:]]+tool[[:space:]]+(wails3|svu|buf|golangci-lint|goreleaser|protoc-gen-go|protoc-gen-connect-go|oras)([[:space:]]|$)|(^|[[:space:]`;&|])(wails3|svu|buf|golangci-lint|goreleaser|oras)[[:space:]]+(current|next|major|minor|patch|prerelease|dev|build|package|generate|format|lint|breaking|run|release|check|login|push)([[:space:]]|$))'
	local allowed_pattern='go tool -modfile=tools/(wails|svu|buf|goreleaser|protoc-gen-go|protoc-gen-connect-go)/go\.mod (wails3|svu|buf|goreleaser|protoc-gen-go|protoc-gen-connect-go)([[:space:]]|$)'

  while IFS= read -r -d '' file; do
    if [[ "$file" = /* ]]; then
      relative_file="${file#"$scan_root"/}"
    else
      relative_file="$file"
      file="$scan_root/$file"
    fi

    case "$relative_file" in
      scripts/tool-modules-check.sh|specs/*|docs/wails-migration-rollback.md|node_modules/*|frontend/node_modules/*|frontend/client/node_modules/*|frontend/overseer/node_modules/*|tests/browser/node_modules/*)
        continue
        ;;
    esac

    file_matches="$(LC_ALL=C grep -IEn "$forbidden_pattern" "$file" 2>/dev/null || true)"
    file_matches="$(printf '%s\n' "$file_matches" | grep -Ev "$allowed_pattern" || true)"
    if [[ -n "$file_matches" ]]; then
      matches+="${relative_file}:${file_matches}"$'\n'
    fi
  done < <(list_scan_files "$scan_root")

  [[ -z "$matches" ]] || {
    printf '%s' "$matches" >&2
    fail 'active files contain a global, bare, or root-module Go tool invocation'
    return 1
  }
}

check_tree() {
  local scan_root="$1"
  local expected_root_module="${2:-github.com/obalunenko/Fallout-Terminal/v2}"
  local expected_tool_module_prefix="${3:-github.com/obalunenko/Fallout-Terminal/tools}"
  local module_file

	discover_tool_modules "$scan_root" || return
	if [[ -e "$scan_root/tools/oras" ]]; then
		fail 'tools/oras must be absent; GitHub Packages publication is not supported'
		return
  fi
  for module_file in "${tool_module_files[@]}"; do
    check_tool_module_contract "$scan_root" "$module_file" "$expected_tool_module_prefix" || return
  done
  check_tool_pin "$scan_root" wails github.com/wailsapp/wails/v3/cmd/wails3 github.com/wailsapp/wails/v3 v3.0.0-beta.15 || return
  check_tool_pin "$scan_root" task github.com/go-task/task/v3/cmd/task github.com/go-task/task/v3 v3.53.1 || return
  check_tool_pin "$scan_root" buf github.com/bufbuild/buf/cmd/buf github.com/bufbuild/buf v1.72.0 || return
  check_tool_pin "$scan_root" golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint github.com/golangci/golangci-lint/v2 v2.13.1 || return
  check_tool_pin "$scan_root" protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go google.golang.org/protobuf v1.36.11 || return
  check_tool_pin "$scan_root" protoc-gen-connect-go connectrpc.com/connect/cmd/protoc-gen-connect-go connectrpc.com/connect v1.20.0 || return
	check_tool_pin "$scan_root" goreleaser github.com/goreleaser/goreleaser/v2 github.com/goreleaser/goreleaser/v2 v2.18.0 || return
  check_tool_pin "$scan_root" svu github.com/caarlos0/svu/v3 github.com/caarlos0/svu/v3 v3.4.1 || return
  check_root_module "$scan_root" "$expected_root_module" || return
  check_make_bootstrap "$scan_root" || return
  check_active_commands "$scan_root" || return
  check_tool_resolution_preserves_root "$scan_root" || return
}

write_fixture_module() {
  local scan_root="$1"
  local directory="$2"
  local command_package="$3"
  local parent_module="$4"
  local version="$5"
  local module_identity="${6:-example.test/tools/$directory}"

  mkdir -p "$scan_root/tools/$directory"
  printf 'module %s\n\ngo 1.27.0\n\ntool %s\n\nrequire %s %s\n' \
    "$module_identity" "$command_package" "$parent_module" "$version" >"$scan_root/tools/$directory/go.mod"
  printf '%s %s/go.mod h1:fixture\n' "$parent_module" "$version" >"$scan_root/tools/$directory/go.sum"
}

write_fixture_makefile() {
  local scan_root="$1"

  printf '%s\n' \
    'GO ?= go' \
    '' \
    'TOOL_MODULES := $(sort $(wildcard tools/*/go.mod))' \
    '' \
    '.DEFAULT_GOAL := tools' \
    '.PHONY: tools help' \
    '' \
    'tools:' >"$scan_root/Makefile"
  printf '\t%s\n' \
    '@set -eu; \' \
    'for module_file in $(TOOL_MODULES); do \' \
    'module_dir="$${module_file%/go.mod}"; \' \
    '(cd "$$module_dir" && $(GO) install tool); \' \
    'done' >>"$scan_root/Makefile"
  printf '%s\n' \
    '' \
    'help:' \
    $'\t@printf '\''Run task --list for project workflows.\\n'\''' >>"$scan_root/Makefile"
}

write_fixture_go() {
  local scan_root="$1"

  mkdir -p "$scan_root/bin"
  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'if [[ -f .mutate-root-module ]]; then' \
    "  printf '\\n// simulated tool-resolution drift\\n' >>go.mod" \
    'fi' >"$scan_root/bin/go"
  chmod +x "$scan_root/bin/go"
}

check_fixture_tree() {
  check_tree "$1" example.test/app example.test/tools
}

self_test() {
  local fixture_root
  local tool_check_go
  fixture_root="$(mktemp -d)"
  trap 'rm -rf "$fixture_root"' RETURN

  printf 'module example.test/app\n\ngo 1.27.0\n' >"$fixture_root/go.mod"
  printf 'example.test/runtime v1.0.0/go.mod h1:fixture\n' >"$fixture_root/go.sum"
  mkdir -p "$fixture_root/docs"
  write_fixture_makefile "$fixture_root"
  write_fixture_go "$fixture_root"
  tool_check_go="$fixture_root/bin/go"
  printf 'go tool -modfile=tools/wails/go.mod wails3 generate bindings -clean ./...\n' >"$fixture_root/docs/commands.md"
  printf 'go run -modfile=tools/golangci-lint/go.mod github.com/golangci/golangci-lint/v2/cmd/golangci-lint run\n' >>"$fixture_root/docs/commands.md"
  write_fixture_module "$fixture_root" wails github.com/wailsapp/wails/v3/cmd/wails3 github.com/wailsapp/wails/v3 v3.0.0-beta.15
  write_fixture_module "$fixture_root" task github.com/go-task/task/v3/cmd/task github.com/go-task/task/v3 v3.53.1
  write_fixture_module "$fixture_root" buf github.com/bufbuild/buf/cmd/buf github.com/bufbuild/buf v1.72.0
  write_fixture_module "$fixture_root" golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint github.com/golangci/golangci-lint/v2 v2.13.1
  write_fixture_module "$fixture_root" protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go google.golang.org/protobuf v1.36.11
  write_fixture_module "$fixture_root" protoc-gen-connect-go connectrpc.com/connect/cmd/protoc-gen-connect-go connectrpc.com/connect v1.20.0
  write_fixture_module "$fixture_root" goreleaser github.com/goreleaser/goreleaser/v2 github.com/goreleaser/goreleaser/v2 v2.18.0
  write_fixture_module "$fixture_root" svu github.com/caarlos0/svu/v3 github.com/caarlos0/svu/v3 v3.4.1
  check_fixture_tree "$fixture_root"

  printf 'module example.test/v2\n\ngo 1.27.0\n' >"$fixture_root/go.mod"
  if check_fixture_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted an unexpected root module identity'
  fi
  printf 'module example.test/app\n\ngo 1.27.0\n' >"$fixture_root/go.mod"

  write_fixture_module "$fixture_root" wails github.com/wailsapp/wails/v3/cmd/wails3 \
    github.com/wailsapp/wails/v3 v3.0.0-beta.15 example.test/v2/tools/wails
  if check_fixture_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted a tool module migrated with the root v2 identity'
  fi
  write_fixture_module "$fixture_root" wails github.com/wailsapp/wails/v3/cmd/wails3 github.com/wailsapp/wails/v3 v3.0.0-beta.15

  printf '\ntool github.com/bufbuild/buf/cmd/buf\n' >>"$fixture_root/go.mod"
  if check_fixture_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted a root tool declaration'
  fi
  printf 'module example.test/app\n\ngo 1.27.0\n' >"$fixture_root/go.mod"

  printf 'go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run\n' >"$fixture_root/docs/commands.md"
  if check_fixture_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted an unpinned golangci-lint invocation'
  fi

  printf 'go install github.com/go-task/task/v3/cmd/task@latest\n' >"$fixture_root/docs/commands.md"
  if check_fixture_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted a global tool installation'
  fi

  printf 'go tool -modfile=tools/wails/go.mod wails3 generate bindings -clean ./...\n' >"$fixture_root/docs/commands.md"
  printf 'go run -modfile=tools/golangci-lint/go.mod github.com/golangci/golangci-lint/v2/cmd/golangci-lint run\n' >>"$fixture_root/docs/commands.md"

  write_fixture_module "$fixture_root" task github.com/go-task/task/v3/cmd/task github.com/go-task/task/v3 v3.53.0
  if check_fixture_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted the wrong Task version'
  fi
  write_fixture_module "$fixture_root" task github.com/go-task/task/v3/cmd/task github.com/go-task/task/v3 v3.53.1

  write_fixture_module "$fixture_root" future example.test/future/cmd/future example.test/future v1.0.0
  check_fixture_tree "$fixture_root"
  : >"$fixture_root/tools/future/go.sum"
  if check_fixture_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted an empty sum for a discovered tool module'
  fi
  write_fixture_module "$fixture_root" future example.test/future/cmd/future example.test/future v1.0.0
  printf '\ntool example.test/second-tool\n' >>"$fixture_root/tools/future/go.mod"
  if check_fixture_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted multiple tools in a discovered module'
  fi
  rm -rf "$fixture_root/tools/future"

  printf '\ncheck:\n\t@true\n' >>"$fixture_root/Makefile"
  if check_fixture_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted an additional Make workflow target'
  fi
  write_fixture_makefile "$fixture_root"

  printf '.DEFAULT_GOAL := tools\n.PHONY: tools\ntools:\n\t@true\n' >"$fixture_root/Makefile"
  if check_fixture_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted a Make bootstrap without module discovery'
  fi
  write_fixture_makefile "$fixture_root"

  : >"$fixture_root/.mutate-root-module"
  if check_fixture_tree "$fixture_root" >/dev/null 2>&1; then
    fail 'self-test accepted root-module drift during tool resolution'
  fi
  rm -f "$fixture_root/.mutate-root-module"
  printf 'module example.test/app\n\ngo 1.27.0\n' >"$fixture_root/go.mod"

  printf 'tool module check self-test passed\n'
}

case "${1:-}" in
  --self-test)
    self_test
    ;;
  '')
    check_tree "$repository_root"
    printf 'Go development tools are isolated, exactly pinned, and invoked through their owning modules.\n'
    ;;
  *)
    fail "unknown argument: $1"
    ;;
esac
