#!/usr/bin/env bash

set -euo pipefail

fail() {
  printf 'frontend install inventory check: %s\n' "$1" >&2
  return 1
}

relative_to_repository() {
  local value="$1"
  printf '%s\n' "${value#./}"
}

require_readable_file() {
  local value="$1"
  [[ -f "$value" && -r "$value" ]] || fail "required file is missing or unreadable: $(relative_to_repository "$value")"
}

validate_vite_temp() {
  local cache_root="$1"
  local entry relative
  while IFS= read -r entry; do
    relative="${entry#"$cache_root"/}"
    [[ "$relative" != "$entry" ]] || continue
    if [[ -d "$entry" ]]; then
      [[ "$relative" != node_modules && "$relative" != node_modules/* ]] || {
        fail "installed dependency payload is forbidden inside Vite config cache: $(relative_to_repository "$entry")"
        return 1
      }
      continue
    fi
    [[ -f "$entry" && -r "$entry" ]] || {
      fail "Vite config cache entry is unreadable or unsupported: $(relative_to_repository "$entry")"
      return 1
    }
    case "${entry##*/}" in
      *.timestamp-*-*.mjs) ;;
      *)
        fail "unexpected Vite config cache artifact: $(relative_to_repository "$entry")"
        return 1
        ;;
    esac
  done < <(find "$cache_root" -mindepth 1 -print | LC_ALL=C sort)
}

validate_app_node_modules() {
  local app_root="$1"
  local cache_root="$app_root/node_modules/.vite-temp"
  local child
  [[ ! -e "$app_root/node_modules" || -d "$app_root/node_modules" ]] || {
    fail "app-local node_modules is not a directory: $(relative_to_repository "$app_root/node_modules")"
    return 1
  }
  [[ ! -d "$app_root/node_modules" ]] && return 0
  while IFS= read -r child; do
    [[ "${child##*/}" == '.vite-temp' && -d "$child" ]] || {
      fail "unexpected app-local dependency or cache sibling: $(relative_to_repository "$child")"
      return 1
    }
  done < <(find "$app_root/node_modules" -mindepth 1 -maxdepth 1 -print | LC_ALL=C sort)
  [[ ! -d "$cache_root" ]] || validate_vite_temp "$cache_root"
}

write_inventory() {
  local frontend_root="$1"
  local output_prefix="$2"
  local require_workspace_install="$3"
  local repository_prefix="${frontend_root#./}"
  local install_inventory="${output_prefix}-dependency-install-roots.txt"
  local cache_inventory="${output_prefix}-vite-cache-paths.txt"
  local lock_inventory="${output_prefix}-lockfiles.txt"
  local manifest_inventory="${output_prefix}-manifests.txt"
  local app_root lockfile manifest node_modules_path

  [[ -d "$frontend_root" && -r "$frontend_root" ]] || {
    fail "frontend root is missing or unreadable: $repository_prefix"
    return 1
  }
  for manifest in "$frontend_root/package.json" "$frontend_root/client/package.json" "$frontend_root/overseer/package.json"; do
    require_readable_file "$manifest" || return 1
  done
  require_readable_file "$frontend_root/package-lock.json" || return 1
  mkdir -p "$(dirname "$output_prefix")"

  : >"$install_inventory"
  if [[ -d "$frontend_root/node_modules" ]]; then
    printf '%s\n' "$repository_prefix/node_modules" >"$install_inventory"
  elif [[ "$require_workspace_install" == true ]]; then
    fail "required workspace dependency installation is absent: $repository_prefix/node_modules"
    return 1
  fi

  for app_root in "$frontend_root/client" "$frontend_root/overseer"; do
    validate_app_node_modules "$app_root" || return 1
  done

  while IFS= read -r node_modules_path; do
    [[ "$node_modules_path" == "$frontend_root/node_modules" ||
       "$node_modules_path" == "$frontend_root/client/node_modules" ||
       "$node_modules_path" == "$frontend_root/overseer/node_modules" ]] && continue
    fail "additional dependency installation root is forbidden: $(relative_to_repository "$node_modules_path")"
    return 1
  done < <(find "$frontend_root" -path "$frontend_root/node_modules" -prune -o \
    -type d -name node_modules -print | LC_ALL=C sort)

  : >"$cache_inventory"
  for app_root in "$frontend_root/client" "$frontend_root/overseer"; do
    if [[ -d "$app_root/node_modules/.vite-temp" ]]; then
      printf '%s\n' "${app_root#./}/node_modules/.vite-temp" >>"$cache_inventory"
    fi
  done

  find "$frontend_root" -type f -name package-lock.json -print | while IFS= read -r lockfile; do
    printf '%s\n' "${lockfile#./}"
  done | LC_ALL=C sort >"$lock_inventory"
  [[ "$(<"$lock_inventory")" == "$repository_prefix/package-lock.json" ]] || {
    fail 'lockfile inventory must contain exactly the frontend workspace lockfile'
    return 1
  }

  find "$frontend_root" -path '*/node_modules' -prune -o -type f -name package.json -print | while IFS= read -r manifest; do
    printf '%s\n' "${manifest#./}"
  done | LC_ALL=C sort >"$manifest_inventory"
  [[ "$(<"$manifest_inventory")" == "$(printf '%s\n' \
    "$repository_prefix/client/package.json" \
    "$repository_prefix/overseer/package.json" \
    "$repository_prefix/package.json" | LC_ALL=C sort)" ]] || {
    fail 'manifest inventory must contain exactly the frontend workspace and two application manifests'
    return 1
  }

  printf 'frontend install inventory check: PASS: dependency roots, Vite caches, lockfiles, and manifests written to %s-*\n' \
    "$output_prefix"
}

expect_failure() {
  local expected="$1"
  shift
  local output status
  set +e
  output="$("$@" 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *"$expected"* ]] || {
    printf '%s\n' "$output" >&2
    fail "self-test expected failure containing: $expected"
    return 1
  }
}

make_fixture() {
  local root="$1"
  mkdir -p "$root/client/node_modules/.vite-temp" "$root/overseer" "$root/node_modules"
  printf '{}\n' >"$root/package.json"
  printf '{}\n' >"$root/client/package.json"
  printf '{}\n' >"$root/overseer/package.json"
  printf '{}\n' >"$root/package-lock.json"
  printf 'export default {};\n' >"$root/client/node_modules/.vite-temp/vite.config.ts.timestamp-1-a.mjs"
}

self_test() {
  local temporary_root fixture output_prefix
  temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/fallout-frontend-install-inventory.XXXXXX")"
  trap 'rm -rf "$temporary_root"' RETURN

  fixture="$temporary_root/valid/frontend"
  make_fixture "$fixture"
  output_prefix="$temporary_root/valid/inventory"
  write_inventory "$fixture" "$output_prefix" true >/dev/null

  fixture="$temporary_root/dependency/frontend"
  make_fixture "$fixture"
  mkdir -p "$fixture/client/node_modules/vue"
  expect_failure 'unexpected app-local dependency or cache sibling' write_inventory "$fixture" "$temporary_root/dependency/inventory" false

  fixture="$temporary_root/package-manifest/frontend"
  make_fixture "$fixture"
  printf '{}\n' >"$fixture/client/node_modules/package.json"
  expect_failure 'unexpected app-local dependency or cache sibling' write_inventory "$fixture" "$temporary_root/package-manifest/inventory" false

  fixture="$temporary_root/package-binary/frontend"
  make_fixture "$fixture"
  mkdir -p "$fixture/client/node_modules/.bin"
  expect_failure 'unexpected app-local dependency or cache sibling' write_inventory "$fixture" "$temporary_root/package-binary/inventory" false

  fixture="$temporary_root/sibling/frontend"
  make_fixture "$fixture"
  mkdir -p "$fixture/client/node_modules/.cache"
  expect_failure 'unexpected app-local dependency or cache sibling' write_inventory "$fixture" "$temporary_root/sibling/inventory" false

  fixture="$temporary_root/lockfile/frontend"
  make_fixture "$fixture"
  printf '{}\n' >"$fixture/client/package-lock.json"
  expect_failure 'lockfile inventory must contain exactly' write_inventory "$fixture" "$temporary_root/lockfile/inventory" false

  fixture="$temporary_root/payload/frontend"
  make_fixture "$fixture"
  printf '{}\n' >"$fixture/client/node_modules/.vite-temp/package.json"
  expect_failure 'unexpected Vite config cache artifact' write_inventory "$fixture" "$temporary_root/payload/inventory" false

  fixture="$temporary_root/missing/frontend"
  make_fixture "$fixture"
  rm "$fixture/overseer/package.json"
  expect_failure 'required file is missing or unreadable' write_inventory "$fixture" "$temporary_root/missing/inventory" false

  printf '%s\n' 'frontend install inventory check self-test: PASS: workspace install plus .vite-temp accepted; dependency, manifest, binary, sibling, lockfile, payload, and missing-path violations rejected actionably'
  rm -rf "$temporary_root"
  trap - RETURN
}

case "${1:-}" in
  --self-test)
    [[ "$#" == 1 ]] || { fail 'usage: frontend-install-inventory-check.sh --self-test'; exit 2; }
    self_test
    ;;
  --frontend-root)
    [[ "$#" == 4 || "$#" == 5 ]] || {
      fail 'usage: frontend-install-inventory-check.sh --frontend-root ROOT --output-prefix PREFIX [--require-workspace-install]'
      exit 2
    }
    [[ "${3:-}" == '--output-prefix' ]] || {
      fail 'expected --output-prefix after frontend root'
      exit 2
    }
    require_workspace_install=false
    if [[ "$#" == 5 ]]; then
      [[ "$5" == '--require-workspace-install' ]] || { fail "unknown option: $5"; exit 2; }
      require_workspace_install=true
    fi
    write_inventory "$2" "$4" "$require_workspace_install"
    ;;
  *)
    fail 'usage: frontend-install-inventory-check.sh --self-test | --frontend-root ROOT --output-prefix PREFIX [--require-workspace-install]'
    exit 2
    ;;
esac
