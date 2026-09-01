#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() {
  printf 'secret leak check: %s\n' "$1" >&2
  return 1
}

list_active_files() {
  if git -C "$repository_root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$repository_root" ls-files -co --exclude-standard -z
  else
    find "$repository_root" -type f -print0
  fi
}

list_active_matching_files() {
  local pattern="$1"
  local file relative

  while IFS= read -r -d '' file; do
    if [[ "$file" = /* ]]; then
      relative="${file#"$repository_root"/}"
    else
      relative="$file"
      file="$repository_root/$file"
    fi
    case "$relative" in
      .git/*|specs/*|node_modules/*|frontend/node_modules/*|frontend/client/node_modules/*|frontend/overseer/node_modules/*|tests/*|*_test.go|scripts/secret-leak-check.sh)
        continue
        ;;
    esac
    [[ -f "$file" && -r "$file" ]] || continue
    if grep -IlE "$pattern" "$file" >/dev/null 2>&1; then
      printf '%s\n' "$relative"
    fi
  done < <(list_active_files)
}

scan_canary_file() {
  local canary_file="$1"
  local scan_root="${2:-$repository_root}"
  local file relative
  local leaked=0

  while IFS= read -r -d '' file; do
    [[ "$file" == "$canary_file" ]] && continue
    if [[ "$file" = /* ]]; then
      relative="${file#"$scan_root"/}"
    else
      relative="$file"
      file="$scan_root/$file"
    fi
    case "$relative" in
      .git/*|specs/*|node_modules/*|frontend/node_modules/*|frontend/client/node_modules/*|frontend/overseer/node_modules/*|tests/browser/node_modules/*|scripts/secret-leak-check.sh)
        continue
        ;;
    esac
    if grep -IlF -f "$canary_file" "$file" >/dev/null 2>&1; then
      printf 'secret leak check: canary detected in %s (value redacted)\n' "$relative" >&2
      leaked=1
    fi
  done < <(
    if [[ "$scan_root" == "$repository_root" ]]; then
      list_active_files
    else
      find "$scan_root" -type f -print0
    fi
  )
  [[ "$leaked" == 0 ]]
}

check_public_contracts() {
  local forbidden
  forbidden="$(grep -ERIl '(^|[[:space:]_])(authtoken|account_token|player_password|provider_token|credential|secret)[[:space:]]*=' \
    "$repository_root/proto/fallout/terminal/player" \
    "$repository_root/proto/fallout/terminal/persistence" 2>/dev/null || true)"
  [[ -z "$forbidden" ]] || fail 'public or persistent protobuf contract contains a secret field'

  grep -Fq 'optional string generated_password = 3;' \
    "$repository_root/proto/fallout/terminal/private/v1/public_access.proto" ||
    fail 'one-time generated-password result contract is missing'
  grep -Fq 'string replacement_provider_token = 5;' \
    "$repository_root/proto/fallout/terminal/private/v1/public_access.proto" ||
    fail 'narrow provider-token mutation input is missing'
  grep -Fq 'string replacement_player_password = 7;' \
    "$repository_root/proto/fallout/terminal/private/v1/public_access.proto" ||
    fail 'narrow player-password mutation input is missing'
}

check_active_sources() {
  local suspicious
  suspicious="$(list_active_matching_files \
    '(ngrok[_-]?authtoken|provider[_-]?token|player[_-]?password)[[:space:]]*[:=][[:space:]]*["'"'][^"'"']{8,}["'"']' || true)"
  [[ -z "$suspicious" ]] || {
    while IFS= read -r file; do
      [[ -n "$file" ]] && printf 'secret leak check: suspicious literal in %s (value redacted)\n' "$file" >&2
    done <<<"$suspicious"
    return 1
  }

  if grep -ER --include='*.ts' --include='*.vue' \
    '(localStorage|sessionStorage).*(providerToken|playerPassword|generatedPassword)|(providerToken|playerPassword|generatedPassword).*(localStorage|sessionStorage)' \
    "$repository_root/frontend/overseer/src" >/dev/null 2>&1; then
    fail 'Overseer frontend persists a public-access secret in browser storage'
  fi

  grep -Fq 'replacementProviderToken' "$repository_root/frontend/overseer/src/components/ProviderTokenDialog.vue" ||
    fail 'Overseer secret mutation flow is not implemented yet'
  grep -Fq "token.value = '';" "$repository_root/frontend/overseer/src/components/ProviderTokenDialog.vue" ||
    fail 'Overseer provider-token input is not cleared after mutation'
  grep -Fq 'generatedPassword' "$repository_root/frontend/overseer/src/components/PlayerCredentialsDialog.vue" ||
    fail 'one-time generated-password presentation is not implemented yet'
}

check_generated_password_scope() {
  local file unexpected=''

  while IFS= read -r file; do
    [[ -n "$file" ]] || continue
    case "$file" in
      app.go|app_contract.go|internal/tunnel/secret.go|proto/fallout/terminal/private/v1/public_access.proto|internal/gen/fallout/terminal/private/v1/public_access.pb.go|frontend/overseer/src/App.vue|frontend/overseer/src/components/GeneratedPasswordDialog.vue|frontend/overseer/src/components/PlayerCredentialsDialog.vue|frontend/overseer/src/controllers/overseer-controller.ts|frontend/overseer/src/overseer.css|frontend/overseer/bindings/github.com/obalunenko/Fallout-Terminal/v2/models.js)
        ;;
      *)
        unexpected="${unexpected}${file}"$'\n'
        ;;
    esac
  done < <(list_active_matching_files 'generated[_-]?password|generatedPassword|GeneratedPassword')

  if [[ -n "$unexpected" ]]; then
    while IFS= read -r file; do
      [[ -n "$file" ]] && printf 'secret leak check: generated-password surface outside one-time flow: %s\n' "$file" >&2
    done <<<"$unexpected"
    return 1
  fi

  if grep -ERIl 'generated[_-]?password|generatedPassword|GeneratedPassword' \
    "$repository_root/proto/fallout/terminal/player" \
    "$repository_root/proto/fallout/terminal/persistence" \
    "$repository_root/internal/session" \
    "$repository_root/internal/playerconfig" >/dev/null 2>&1; then
    fail 'generated password escaped into a public or persistent contract'
    return 1
  fi

  grep -Fq "oneTimeValue.value = '';" "$repository_root/frontend/overseer/src/components/GeneratedPasswordDialog.vue" ||
    fail 'one-time generated-password presentation is not cleared on completion'
  grep -Fq 'release?.();' "$repository_root/frontend/overseer/src/components/GeneratedPasswordDialog.vue" ||
    fail 'one-time generated-password callback retains its value after completion'
  grep -Fq 'afterGenerated.generatedPassword).toBeUndefined()' \
    "$repository_root/tests/browser/desktop-api.spec.mjs" ||
    fail 'browser contract does not prove generated-password non-readback'
  grep -Fq "locator('#generatedPasswordValue')).toHaveText('')" \
    "$repository_root/tests/browser/public-access-settings.spec.mjs" ||
    fail 'browser journey does not prove one-time presentation clearing'
}

check_development_override_scope() {
  local override="$repository_root/internal/tunnel/test_override.go"
  [[ -f "$override" ]] || { fail 'development/test public-access override is missing'; return 1; }

  local name
  for name in \
    FALLOUT_NGROK_AUTHTOKEN \
    FALLOUT_NGROK_RESERVED_DOMAIN \
    FALLOUT_PUBLIC_TEST_USERNAME \
    FALLOUT_PUBLIC_TEST_PASSWORD; do
    grep -Fq "$name" "$override" || { fail "approved development override name is missing: $name"; return 1; }
  done
  if grep -Fq 'os.Environ' "$override"; then
    fail 'development override enumerates the process environment'
    return 1
  fi
  grep -Fq 'publicAccessStoresForProfile(publicSettings, publicSecrets, packaged, os.LookupEnv)' \
    "$repository_root/main.go" || fail 'root composition does not explicitly gate the development override from packaged production'
}

check_native_secure_store_scope() {
  local provider
  for provider in keychain_windows.go keychain_linux.go; do
    provider="$repository_root/internal/platform/$provider"
    [[ -f "$provider" ]] || { fail "native secure credential provider is missing: ${provider##*/}"; return 1; }
    grep -Fq 'clear(' "$provider" || { fail "native credential provider does not clear temporary secret bytes: ${provider##*/}"; return 1; }
    if grep -Eq 'os\.(Create|OpenFile|WriteFile)|os\.(Getenv|LookupEnv)' "$provider"; then
      fail "native credential provider contains a file or environment fallback: ${provider##*/}"
      return 1
    fi
  done

  grep -Fq 'NewPlatformSecureCredentialStore(packaged)' "$repository_root/main.go" ||
    fail 'root composition does not select the platform-native secure credential store'

  local category
  for category in secret_store_locked secret_store_denied secret_store_unavailable; do
    grep -Fq "$category" "$repository_root/frontend/overseer/src/composables/usePublicAccess.ts" ||
      { fail "Overseer is missing platform-neutral secure-store wording for $category"; return 1; }
  done
  if grep -FiR 'Keychain' "$repository_root/frontend/overseer/src" >/dev/null 2>&1; then
    fail 'Overseer contains macOS-specific credential-store wording'
    return 1
  fi
}

check_player_bundle_boundary() {
  local bundle_root="$repository_root/frontend/client/dist"
  local emitted relative
  local leaked=0

  [[ -d "$bundle_root" && -f "$bundle_root/.keep" && -f "$bundle_root/index.html" ]] || {
    fail 'production Player bundle is incomplete'
    return 1
  }
  [[ "$(grep -Foc '<div id="playerApp"></div>' "$bundle_root/index.html")" == 1 ]] || {
    fail 'production Player bundle does not contain the sole Vue root'
    return 1
  }
  grep -Eq '<script type="module"[^>]+src="\./assets/index-[^"]+\.js"' "$bundle_root/index.html" || {
    fail 'production Player bundle does not select its emitted module'
    return 1
  }

  while IFS= read -r -d '' emitted; do
    relative="${emitted#"$bundle_root"/}"
    case "$relative" in
      *.ts|*.tsx|*.vue|*.map|*/src/*|src/*|*candidate*|*test-fixtures*|*/client.js|client.js|*/sound.js|sound.js|*/presentation-uplink.js|presentation-uplink.js)
        printf 'secret leak check: forbidden Player bundle path: %s\n' "$relative" >&2
        leaked=1
        ;;
    esac
    if grep -IlE \
      'candidate([-_/.]*(main|player|mount|bridge|selection|root|index))|test[-_/.]*fixtures|frontend/client/src|frontend/overseer|wailsjs|fallout/terminal/private|client\.js|sound\.js|presentation-uplink\.js|ngrok[_-]?authtoken|provider(Token|_token)|player(Password|_password)|generatedPassword' \
      "$emitted" >/dev/null 2>&1; then
      printf 'secret leak check: forbidden legacy, candidate, source, private, or credential surface in Player bundle: %s\n' "$relative" >&2
      leaked=1
    fi
  done < <(find "$bundle_root" -type f -print0)

  [[ "$leaked" == 0 ]] || return 1
}

check_tree() {
  local canary_file="${1:-}"
  local scan_root="${2:-$repository_root}"
  check_public_contracts
  check_active_sources
  check_generated_password_scope
  check_development_override_scope
  check_native_secure_store_scope
  check_player_bundle_boundary
  if [[ -n "$canary_file" ]]; then
    [[ -s "$canary_file" ]] || { fail 'canary file is missing or empty'; return 1; }
    [[ -d "$scan_root" ]] || { fail 'canary scan root is missing or not a directory'; return 1; }
    scan_canary_file "$canary_file" "$scan_root"
  fi
  printf 'Secret-bearing fields remain confined to narrow private inputs/results; no forbidden leak was detected.\n'
}

self_test() {
  local fixture_root canary_file surface
  local -a surfaces=(
    'errors/start-error.txt'
    'application-update/errors/check.txt'
    'application-update/errors/download.txt'
    'application-update/errors/verify.txt'
    'application-update/errors/stage.txt'
    'application-update/errors/apply.txt'
    'logs/application.log'
    'application-update/logs/helper.log'
    'events/public-access-status.json'
    'application-update/events/application-update-status.json'
    'protobuf/private-result.bin'
    'application-update/protobuf/update-command-result.bin'
    'config/public-access.json'
    'application-update/recovery/update-recovery.json'
    'application-update/environment/helper.env'
    'Application Support/Fallout Terminal/public-access.json'
    'Windows/Credentials/credential.bin'
    'Secret Service/login/item.bin'
    'sessions/session-v1.json'
    'player-config/player-config-v1.json'
    'args/process.args'
    'fixtures/public-access.json'
    'frontend/local-storage.json'
    'package/Fallout Terminal.app/Contents/Resources/diagnostic.json'
    'package/Fallout-Terminal-windows-amd64/resources/diagnostic.json'
    'package/Fallout-Terminal-linux-arm64/resources/diagnostic.json'
  )
  fixture_root="$(mktemp -d)"
  trap 'rm -rf "$fixture_root"' RETURN
  canary_file="$fixture_root/.canaries"
  mkdir -p "$fixture_root/surfaces"
  {
    printf 'https://updates.example.invalid/archive.zip?token=github_pat_'
    LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 96 || true
    printf '\n'
    printf 'github_pat_'
    LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 112 || true
    printf '\n'
    printf '/Users/update-canary-account/Downloads/'
    LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 128 || true
    printf '/Fallout-Terminal.zip\n'
  } >"$canary_file"

  for surface in "${surfaces[@]}"; do
    mkdir -p "$fixture_root/surfaces/$(dirname "$surface")"
    cp "$canary_file" "$fixture_root/surfaces/$surface"
    if scan_canary_file "$canary_file" "$fixture_root/surfaces" >/dev/null 2>&1; then
      fail "self-test accepted a canary on $surface"
      return 1
    fi
    : >"$fixture_root/surfaces/$surface"
  done

  scan_canary_file "$canary_file" "$fixture_root/surfaces"
  printf 'Secret leak detector self-test passed across update URLs, tokens, paths, errors, logs, events, protobuf, recovery, helper environment, config, native credential stores, session, player-config, args, fixtures, frontend, and packaged resources.\n'
}

case "${1:-}" in
  '')
    check_tree
    ;;
  --canary-file)
    if [[ "$#" == 2 ]]; then
      check_tree "$2"
    elif [[ "$#" == 4 && "$3" == --scan-root ]]; then
      check_tree "$2" "$4"
    else
      fail 'usage: secret-leak-check.sh --canary-file PATH [--scan-root DIRECTORY]'
      exit 2
    fi
    ;;
  --self-test)
    self_test
    ;;
  *)
    fail 'usage: secret-leak-check.sh [--canary-file PATH [--scan-root DIRECTORY]|--self-test]'
    ;;
esac
