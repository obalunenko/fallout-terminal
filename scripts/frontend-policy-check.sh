#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture_root="$repository_root/tests/fixtures/frontend-policy"

fail() {
  printf 'frontend policy check: %s\n' "$1" >&2
  return 1
}

fixture_value() {
  local key="$1"
  local fixture="$2"
  local count value
  count="$(LC_ALL=C grep -c "^${key}=" "$fixture" || true)"
  [[ "$count" == 1 ]] || { fail "fixture must declare $key exactly once: ${fixture#"$repository_root/"}"; return 1; }
  value="$(sed -n "s/^${key}=//p" "$fixture")"
  [[ -n "$value" ]] || { fail "fixture value must not be empty: $key"; return 1; }
  printf '%s\n' "$value"
}

check_fixture() {
  local fixture="$1"
  local pins workspaces lockfiles install exclusions player_dependencies source
  [[ -f "$fixture" && -r "$fixture" ]] || { fail "fixture is missing or unreadable: ${fixture#"$repository_root/"}"; return 1; }

  pins="$(fixture_value pins "$fixture")"
  workspaces="$(fixture_value workspaces "$fixture")"
  lockfiles="$(fixture_value lockfiles "$fixture")"
  install="$(fixture_value install "$fixture")"
  exclusions="$(fixture_value exclusions "$fixture")"
  player_dependencies="$(fixture_value player_dependencies "$fixture")"
  source="$(fixture_value source "$fixture")"

  [[ "$pins" == 'typescript:6.0.3,@vitejs/plugin-vue:6.0.8,vue:3.5.42' ]] || { fail 'pin contract mismatch'; return 1; }
  [[ "$workspaces" == 'client,overseer' ]] || { fail 'workspace contract must be exactly client,overseer'; return 1; }
  [[ "$lockfiles" == 'frontend/package-lock.json' ]] || { fail 'frontend/package-lock.json must be the only frontend lockfile'; return 1; }
  [[ "$install" == 'task frontend:build>deps:frontend>npm ci --prefix frontend' ]] || { fail 'frontend install path must be owned only by task frontend:build'; return 1; }
  [[ "$exclusions" == 'frontend/overseer/bindings/**,frontend/client/gen/**,frontend/node_modules/**,frontend/client/dist/**,frontend/overseer/dist/**' ]] || {
    fail 'policy exclusions must be path-exact'
    return 1
  }
  if LC_ALL=C grep -Eqi '(@wailsio/runtime|wails|overseer|(^|[,/])(internal|native|private)([,/]|$))' <<<"$player_dependencies"; then
    fail 'Player dependency boundary includes a privileged dependency'
    return 1
  fi
  if LC_ALL=C grep -Eqi '(^|[^[:alnum:]_])any([^[:alnum:]_]|$)|@ts-(nocheck|ignore|expect-error)|as[[:space:]]+unknown[[:space:]]+as' <<<"$source"; then
    fail 'production source contains a prohibited type escape'
    return 1
  fi

  printf 'frontend policy check: PASS: fixture %s satisfies pins, workspace, lockfile, install, exclusion, Player-boundary, and type-safety contracts\n' \
    "${fixture#"$repository_root/"}"
}

check_repository() {
  node - "$repository_root" <<'NODE'
const fs = require('node:fs');
const path = require('node:path');

const root = process.argv[2];
const readJSON = (relativePath) => JSON.parse(fs.readFileSync(path.join(root, relativePath), 'utf8'));
const workspace = readJSON('frontend/package.json');
const player = readJSON('frontend/client/package.json');
const overseer = readJSON('frontend/overseer/package.json');

const fail = (message) => {
  console.error(`frontend policy check: ${message}`);
  process.exit(1);
};
const exact = (actual, expected, message) => {
  if (actual !== expected) fail(`${message}; expected ${expected}, found ${actual ?? 'missing'}`);
};

if (JSON.stringify(workspace.workspaces) !== JSON.stringify(['client', 'overseer'])) {
  fail('workspace contract must be exactly client,overseer');
}
exact(workspace.devDependencies?.typescript, '6.0.3', 'TypeScript pin mismatch');
exact(workspace.devDependencies?.['@vitejs/plugin-vue'], '6.0.8', 'Vue Vite plugin pin mismatch');
exact(workspace.devDependencies?.['vue-tsc'], '3.3.11', 'vue-tsc pin mismatch');
exact(player.dependencies?.vue, '3.5.42', 'Player Vue pin mismatch');
exact(overseer.dependencies?.vue, '3.5.42', 'Overseer Vue pin mismatch');

for (const [name, manifest] of [['workspace', workspace], ['Player', player], ['Overseer', overseer]]) {
  for (const [script, command] of Object.entries(manifest.scripts ?? {})) {
    if (/\bnpm\s+(ci|install)\b/.test(command)) {
      fail(`${name} manifest script ${script} creates an ungoverned install path`);
    }
  }
}

for (const dependency of Object.keys(player.dependencies ?? {})) {
  if (/wails|overseer|^(internal|native|private)(\/|$)/i.test(dependency)) {
    fail(`Player dependency boundary includes privileged dependency ${dependency}`);
  }
}
NODE

  [[ -f "$repository_root/frontend/package-lock.json" ]] || { fail 'frontend/package-lock.json is missing'; return 1; }
  [[ ! -e "$repository_root/frontend/client/package-lock.json" ]] || { fail 'frontend/client/package-lock.json is prohibited'; return 1; }
  [[ ! -e "$repository_root/frontend/overseer/package-lock.json" ]] || { fail 'frontend/overseer/package-lock.json is prohibited'; return 1; }
  "$repository_root/scripts/frontend-assert-no-match.sh" \
    'allowJs|checkJs|@ts-(nocheck|ignore|expect-error)|(^|[^[:alnum:]_])any([^[:alnum:]_]|$)' \
    frontend/tsconfig.base.json frontend/client/tsconfig.json frontend/overseer/tsconfig.json
  printf '%s\n' 'frontend policy check: PASS: repository manifests, lockfile, compiler policy, and Player dependency boundary satisfy the wave-a contract'
}

expect_fixture_failure() {
  local fixture="$1"
  local expected="$2"
  local output status
  set +e
  output="$(check_fixture "$fixture" 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *"$expected"* ]] || {
    printf '%s\n' "$output" >&2
    fail "fixture did not fail with expected diagnostic: $expected"
    return 1
  }
}

self_test() {
  check_repository
  check_fixture "$fixture_root/valid-policy.txt"
  expect_fixture_failure "$fixture_root/invalid-pin.txt" 'pin contract mismatch'
  expect_fixture_failure "$fixture_root/invalid-lockfile.txt" 'frontend/package-lock.json must be the only frontend lockfile'
  expect_fixture_failure "$fixture_root/invalid-player-dependency.txt" 'Player dependency boundary includes a privileged dependency'
  expect_fixture_failure "$fixture_root/invalid-type-escape.txt" 'production source contains a prohibited type escape'
  printf '%s\n' 'frontend policy check self-test: PASS: valid policy accepted and pin, lockfile, Player-boundary, and type-escape violations rejected actionably'
}

case "${1:-}" in
  --self-test)
    [[ "$#" == 1 ]] || { fail 'usage: frontend-policy-check.sh [--self-test]'; exit 2; }
    self_test
    ;;
  '')
    check_repository
    ;;
  *)
    fail 'usage: frontend-policy-check.sh [--self-test]'
    exit 2
    ;;
esac
