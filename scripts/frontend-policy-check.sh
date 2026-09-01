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

reject_overseer_source_match() {
  local description="$1"
  local pattern="$2"
  shift 2

  local output status
  set +e
  output="$(LC_ALL=C grep -Eni -- "$pattern" "$@" 2>&1)"
  status=$?
  set -e

  case "$status" in
    0)
      fail "Overseer source contains prohibited ${description}: ${output%%$'\n'*}"
      return 1
      ;;
    1)
      return 0
      ;;
    *)
      fail "Overseer ${description} scan failed: $output"
      return 1
      ;;
  esac
}

check_overseer_values() {
  local source_path="$1"
  local source="$2"

  if [[ "$source_path" == frontend/overseer/src/*.js ]]; then
    fail "Overseer authored JavaScript is prohibited: $source_path"
    return 1
  fi
  if LC_ALL=C grep -Eqi \
    'legacy[-_ ]?overseer|legacyOverseer|candidate-main|overseer[-_/.]*candidate|test[-_/.]*fixtures|overseerVueLeaves|typedCoexistence|coexistenceBridge|legacyToVue|vueToLegacy|mountOverseerLeaves' \
    <<<"$source"; then
    fail 'Overseer source contains a prohibited legacy, candidate, root, or bridge identifier'
    return 1
  fi
  if LC_ALL=C grep -Eqi \
    'document\.(querySelector|getElementById)\([^)]*(legacyOverseerRoot|overseerVueLeaves|playerApp)' \
    <<<"$source"; then
    fail 'Overseer source contains prohibited cross-application DOM ownership'
    return 1
  fi
  if LC_ALL=C grep -Eqi \
    '(^|[^[:alnum:]_])any([^[:alnum:]_]|$)|@ts-(nocheck|ignore|expect-error)|as[[:space:]]+unknown[[:space:]]+as|as[[:space:]]+any|<any>|eslint-disable|biome-ignore|@vue-ignore' \
    <<<"$source"; then
    fail 'Overseer source contains a prohibited broad assertion or suppression'
    return 1
  fi
}

check_overseer_repository() {
  local source_root="$repository_root/frontend/overseer/src"
  local bindings_root="$repository_root/frontend/overseer/bindings"
  local -a authored_javascript source_files
  local file

  authored_javascript=()
  while IFS= read -r file; do authored_javascript+=("$file"); done \
    < <(find "$source_root" -type f -name '*.js' -print | LC_ALL=C sort)
  if ((${#authored_javascript[@]} > 0)); then
    fail "Overseer authored JavaScript is prohibited: ${authored_javascript[0]#"$repository_root/"}"
    return 1
  fi
  [[ -f "$bindings_root/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js" ]] || {
    fail 'path-exact Wails binding exclusion target is missing'
    return 1
  }
  source_files=()
  while IFS= read -r file; do source_files+=("$file"); done \
    < <(find "$source_root" -type f \
      \( -name '*.ts' -o -name '*.vue' -o -name '*.html' \) -print | LC_ALL=C sort)
  ((${#source_files[@]} > 0)) || { fail 'Overseer source scan found no readable inputs'; return 1; }
  reject_overseer_source_match 'legacy, candidate, root, or bridge identifier' \
    'legacy[-_ ]?overseer|legacyOverseer|candidate-main|overseer[-_/.]*candidate|test[-_/.]*fixtures|overseerVueLeaves|typedCoexistence|coexistenceBridge|legacyToVue|vueToLegacy|mountOverseerLeaves' \
    "${source_files[@]}"
  reject_overseer_source_match 'cross-application DOM ownership' \
    'document\.(querySelector|getElementById)\([^)]*(legacyOverseerRoot|overseerVueLeaves|playerApp)' \
    "${source_files[@]}"
  reject_overseer_source_match 'broad assertion or suppression' \
    '(^|[^[:alnum:]_])any([^[:alnum:]_]|$)|@ts-(nocheck|ignore|expect-error)|as[[:space:]]+unknown[[:space:]]+as|as[[:space:]]+any|<any>|eslint-disable|biome-ignore|@vue-ignore' \
    "${source_files[@]}"
  printf 'frontend policy check: PASS: final Overseer source has no authored JavaScript, temporary ownership, cross-DOM edge, broad escape, or suppression; only %s is excluded path-exactly\n' \
    "${bindings_root#"$repository_root/"}/**"
}

check_overseer_fixture() {
  local fixture="$1"
  check_overseer_values \
    "$(fixture_value path "$fixture")" \
    "$(fixture_value source "$fixture")"
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
  check_overseer_repository
  check_player_repository
  printf '%s\n' 'frontend policy check: PASS: repository manifests, lockfile, compiler policy, and Player dependency boundary satisfy the wave-a contract'
}

reject_player_foundation_match() {
  local description="$1"
  local pattern="$2"
  shift 2

  local output status
  set +e
  output="$(LC_ALL=C grep -Eni -- "$pattern" "$@" 2>&1)"
  status=$?
  set -e

  case "$status" in
    0)
      fail "Player foundation contains prohibited ${description}: ${output%%$'\n'*}"
      return 1
      ;;
    1)
      return 0
      ;;
    *)
      fail "Player foundation ${description} scan failed: $output"
      return 1
      ;;
  esac
}

check_player_foundation_files() {
  local label="$1"
  shift
  local file

  [[ "$#" -gt 0 ]] || { fail 'Player foundation scan requires at least one file'; return 1; }
  for file in "$@"; do
    [[ -f "$file" && -r "$file" ]] || {
      fail "Player foundation file is missing or unreadable: ${file#"$repository_root/"}"
      return 1
    }
  done

  reject_player_foundation_match 'Overseer import' \
    'frontend/overseer|(^|[^[:alnum:]_])overseer/' "$@"
  reject_player_foundation_match 'Wails runtime capability' \
    '@wailsio/runtime|(^|[^[:alnum:]_])wails([^[:alnum:]_]|$)' "$@"
  reject_player_foundation_match 'generated binding import' \
    '(^|[/._-])bindings?([/._-]|$)' "$@"
  reject_player_foundation_match 'native or filesystem capability' \
    'node:fs|filesystem|(^|[^[:alnum:]_])(internal|native)/' "$@"
  reject_player_foundation_match 'private protobuf capability' \
    '(^|[/._-])private([/._-]|$)|private_pb' "$@"
  reject_player_foundation_match 'cross-application type contract' \
    'cross[-_ ]?app|shared/application-types|shared-app-types' "$@"
  reject_player_foundation_match 'shared application store' \
    'shared[-_/ ]?(application[-_/ ]?)?(state|store)|createSharedStore' "$@"
  reject_player_foundation_match 'ConnectRPC or subscription behavior' \
    '@connectrpc|createClient|(^|[^[:alnum:]_])(Subscribe|subscription)([^[:alnum:]_]|$)' "$@"
  reject_player_foundation_match 'production DOM lookup or selection' \
    'document\.querySelector|document\.getElementById\([^)]*(screen|connOverlay|crt)|replaceChildren|productionSelection|legacyPlayerRoot|client\.js' "$@"

  printf 'frontend policy check: PASS: %s contains no privileged, cross-application, business-behavior, or production-DOM edge across %d readable file(s)\n' \
    "$label" "$#"
}

check_player_foundation() {
  check_player_foundation_files 'real Player foundation' \
    "$repository_root/frontend/client/src/env.d.ts" \
    "$repository_root/frontend/client/src/models/player-view-state.ts" \
    "$repository_root/frontend/client/src/ports/player-transport.ts" \
    "$repository_root/frontend/client/src/App.vue" \
    "$repository_root/frontend/client/src/mount.ts" \
    "$repository_root/frontend/client/src/main.ts"
}

check_player_source_values() {
  local source_path="$1"
  local source="$2"

  if [[ "$source_path" == frontend/client/*.js || "$source_path" == frontend/client/src/*.js ||
    "$source_path" == frontend/client/src/**/*.js ]]; then
    fail "Player authored JavaScript is prohibited: $source_path"
    return 1
  fi
  if LC_ALL=C grep -Eqi \
    "from[[:space:]]+['\"][^'\"]*(overseer|@wailsio|wails|bindings?|native|private|internal|shared[-_/ ]?(application[-_/ ]?)?(state|store))|import[[:space:]]*\\([^)]*(overseer|@wailsio|wails|bindings?|native|private|internal)" \
    <<<"$source"; then
    fail 'Player source contains a prohibited privileged or cross-application import edge'
    return 1
  fi
  if LC_ALL=C grep -Eqi \
    "candidate([-_/.]*(main|player|mount|bridge|selection|root|index))|player[-_/.]*candidate|test[-_/.]*fixtures|legacyPlayer|playerLegacy|staging(Mount|Bridge)|temporaryMount|client\\.js([\"']|$)|(^|[/\"'])sound\\.js([\"']|$)|presentation-uplink\\.js" \
    <<<"$source"; then
    fail 'Player source contains a prohibited legacy, alternate-root, or staging identifier'
    return 1
  fi
  if LC_ALL=C grep -Eqi \
    '(^|[^[:alnum:]_])any([^[:alnum:]_]|$)|@ts-(nocheck|ignore|expect-error)|as[[:space:]]+unknown[[:space:]]+as|as[[:space:]]+any|<any>|eslint-disable|biome-ignore|@vue-ignore' \
    <<<"$source"; then
    fail 'Player source contains a prohibited broad assertion or suppression'
    return 1
  fi
}

check_player_repository() {
  local source_root="$repository_root/frontend/client/src"
  local -a authored_javascript source_files
  local file relative source

  authored_javascript=()
  while IFS= read -r file; do authored_javascript+=("$file"); done \
    < <(find "$repository_root/frontend/client" \
      \( -path "$repository_root/frontend/client/dist" -o -path "$repository_root/frontend/client/gen" -o \
        -path "$repository_root/frontend/client/node_modules" \) -prune -o \
      -type f -name '*.js' -print | LC_ALL=C sort)
  if ((${#authored_javascript[@]} > 0)); then
    fail "Player authored JavaScript is prohibited: ${authored_javascript[0]#"$repository_root/"}"
    return 1
  fi

  source_files=(
    "$repository_root/frontend/client/index.html"
    "$repository_root/frontend/client/tsconfig.json"
    "$repository_root/frontend/client/vite.config.ts"
  )
  while IFS= read -r file; do source_files+=("$file"); done \
    < <(find "$source_root" -type f \( -name '*.ts' -o -name '*.vue' \) -print | LC_ALL=C sort)
  for file in "${source_files[@]}"; do
    [[ -f "$file" && -r "$file" ]] || { fail "Player source is missing or unreadable: ${file#"$repository_root/"}"; return 1; }
    relative="${file#"$repository_root/"}"
    source="$(<"$file")"
    check_player_source_values "$relative" "$source"
  done
  printf 'frontend policy check: PASS: final Player source has no authored JavaScript, privileged/cross-application edge, alternate root, staging mechanism, broad escape, or suppression across %d readable file(s)\n' \
    "${#source_files[@]}"
}

check_player_source_fixture() {
  local fixture="$1"
  check_player_source_values \
    "$(fixture_value path "$fixture")" \
    "$(fixture_value source "$fixture")"
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

expect_player_foundation_failure() {
  local fixture="$1"
  local expected="$2"
  local output status
  set +e
  output="$(check_player_foundation_files "fixture ${fixture#"$repository_root/"}" "$fixture" 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *"$expected"* ]] || {
    printf '%s\n' "$output" >&2
    fail "Player foundation fixture did not fail with expected diagnostic: $expected"
    return 1
  }
}

expect_overseer_fixture_failure() {
  local fixture="$1"
  local expected="$2"
  local output status
  set +e
  output="$(check_overseer_fixture "$fixture" 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *"$expected"* ]] || {
    printf '%s\n' "$output" >&2
    fail "Overseer fixture did not fail with expected diagnostic: $expected"
    return 1
  }
}

expect_player_source_fixture_failure() {
  local fixture="$1"
  local expected="$2"
  local output status
  set +e
  output="$(check_player_source_fixture "$fixture" 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *"$expected"* ]] || {
    printf '%s\n' "$output" >&2
    fail "Player source fixture did not fail with expected diagnostic: $expected"
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
  check_player_foundation_files 'valid Player foundation fixture' "$fixture_root/valid-player-foundation.txt"
  expect_player_foundation_failure "$fixture_root/invalid-player-wails.txt" 'prohibited Wails runtime capability'
  expect_player_foundation_failure "$fixture_root/invalid-player-bindings.txt" 'prohibited generated binding import'
  expect_player_foundation_failure "$fixture_root/invalid-player-native-filesystem.txt" 'prohibited native or filesystem capability'
  expect_player_foundation_failure "$fixture_root/invalid-player-private.txt" 'prohibited private protobuf capability'
  expect_player_foundation_failure "$fixture_root/invalid-cross-app-type.txt" 'prohibited cross-application type contract'
  expect_player_foundation_failure "$fixture_root/invalid-cross-app-store.txt" 'prohibited shared application store'
  expect_player_foundation_failure "$fixture_root/invalid-player-connectrpc.txt" 'prohibited ConnectRPC or subscription behavior'
  expect_player_foundation_failure "$fixture_root/invalid-player-production-dom.txt" 'prohibited production DOM lookup or selection'
  expect_overseer_fixture_failure "$fixture_root/invalid-overseer-legacy.txt" 'prohibited legacy, candidate, root, or bridge identifier'
  expect_overseer_fixture_failure "$fixture_root/invalid-generated-name-disguise.txt" 'Overseer authored JavaScript is prohibited'
  expect_player_source_fixture_failure "$fixture_root/invalid-player-legacy.txt" 'prohibited legacy, alternate-root, or staging identifier'
  expect_player_source_fixture_failure "$fixture_root/invalid-player-transitive-wails.txt" 'prohibited privileged or cross-application import edge'
  expect_player_source_fixture_failure "$fixture_root/invalid-player-candidate.txt" 'prohibited legacy, alternate-root, or staging identifier'
  printf '%s\n' 'frontend policy check self-test: PASS: general, Player-foundation, final Player, and final Overseer boundaries accept valid inputs and reject exact violations actionably'
}

case "${1:-}" in
  --self-test)
    [[ "$#" == 1 ]] || { fail 'usage: frontend-policy-check.sh [--self-test|--check-player-foundation]'; exit 2; }
    self_test
    ;;
  --check-player-foundation)
    [[ "$#" == 1 ]] || { fail 'usage: frontend-policy-check.sh [--self-test|--check-player-foundation]'; exit 2; }
    check_player_foundation
    ;;
  '')
    check_repository
    ;;
  *)
    fail 'usage: frontend-policy-check.sh [--self-test|--check-player-foundation]'
    exit 2
    ;;
esac
