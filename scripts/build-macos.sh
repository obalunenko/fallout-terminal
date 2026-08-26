#!/bin/sh
set -eu
set -o pipefail

fail() {
  printf 'build-macos: %s\n' "$1" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

script_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repository_root=$(CDPATH= cd -- "$script_directory/.." && pwd)
case "$repository_root" in
  /|'') fail "refusing an unsafe repository root" ;;
esac

app_path="$repository_root/build/bin/Fallout Terminal.app"
app_executable="$app_path/Contents/MacOS/Fallout Terminal"
entitlements_path="$repository_root/build/darwin/entitlements.plist"
dmg_path="$repository_root/build/bin/Fallout-Terminal-arm64.dmg"
dmg_checksum_path="$dmg_path.sha256"
volume_name="Fallout Terminal"

preflight_only=false
case "${1-}" in
  '') ;;
  --preflight) preflight_only=true ;;
  *) fail "usage: scripts/build-macos.sh [--preflight]" ;;
esac
test "$#" -le 1 || fail "usage: scripts/build-macos.sh [--preflight]"

: "${DEVELOPER_ID_APPLICATION:?set DEVELOPER_ID_APPLICATION to an installed Developer ID Application identity or fingerprint}"
: "${NOTARYTOOL_KEYCHAIN_PROFILE:?set NOTARYTOOL_KEYCHAIN_PROFILE to a notarytool Keychain profile name}"

test "$(uname -s)" = "Darwin" || fail "release builds require macOS"
test "$(uname -m)" = "arm64" || fail "release builds require an Apple Silicon host"
test -f "$repository_root/go.mod" || fail "repository root is missing go.mod"
test -f "$repository_root/cmd/build/main.go" || fail "repository Go build command is missing"
test -f "$entitlements_path" || fail "missing entitlements: $entitlements_path"
/usr/bin/plutil -lint "$entitlements_path" >/dev/null || fail "invalid entitlements plist"

for command_name in go npm xcrun security codesign spctl hdiutil ditto lipo shasum grep awk basename mktemp install mkdir ln rm; do
  require_command "$command_name"
done

case "$(go version)" in
  *' go1.27.'*) ;;
  *) fail "Go 1.27.x is required" ;;
esac

xcrun --find notarytool >/dev/null 2>&1 || fail "xcrun could not find notarytool"
xcrun --find stapler >/dev/null 2>&1 || fail "xcrun could not find stapler"

identity_matches=$(security find-identity -v -p codesigning 2>/dev/null | grep -F -- "$DEVELOPER_ID_APPLICATION" || true)
case "$identity_matches" in
  *'Developer ID Application:'*) ;;
  *) fail "DEVELOPER_ID_APPLICATION does not reference an installed valid Developer ID Application identity" ;;
esac

# This reads the named Keychain profile and contacts the notary service, but it
# neither prints stored credentials nor creates a submission.
if ! xcrun notarytool history \
  --keychain-profile "$NOTARYTOOL_KEYCHAIN_PROFILE" \
  --output-format json >/dev/null; then
  fail "NOTARYTOOL_KEYCHAIN_PROFILE is unavailable or cannot authenticate"
fi

if [ "$preflight_only" = true ]; then
  printf '%s\n' "Release preflight passed; no build or notarization submission was performed."
  exit 0
fi

temporary_root=${TMPDIR:-/tmp}
temporary_root=$(CDPATH= cd -- "$temporary_root" && pwd)
temporary_directory=$(mktemp -d "$temporary_root/fallout-terminal-release.XXXXXX")
cleanup() {
  case "$temporary_directory" in
    "$temporary_root"/fallout-terminal-release.*)
      rm -rf -- "$temporary_directory"
      ;;
    *)
      printf 'build-macos: refusing to remove unexpected temporary path: %s\n' "$temporary_directory" >&2
      ;;
  esac
}
trap cleanup EXIT HUP INT TERM

export CGO_ENABLED=1
export MACOSX_DEPLOYMENT_TARGET=13.0

cd "$repository_root"
go tool -modfile=tools/task/go.mod task package

test -d "$app_path" || fail "Wails did not create the expected app: $app_path"
test -x "$app_executable" || fail "app executable is missing: $app_executable"
lipo -archs "$app_executable" | grep -qw arm64 || fail "application executable is not arm64"

# The Go package command installs every resource and ad-hoc signs the completed
# development candidate. Replace that signature with the release identity.
codesign --force --deep --options runtime --timestamp \
  --entitlements "$entitlements_path" \
  --sign "$DEVELOPER_ID_APPLICATION" \
  "$app_path"

codesign --verify --deep --strict --verbose=2 "$app_path"
codesign_details=$(codesign -d --verbose=4 "$app_path" 2>&1)
case "$codesign_details" in
  *flags=*runtime*) ;;
  *) fail "hardened runtime flag is missing" ;;
esac
case "$codesign_details" in
  *'Authority=Developer ID Application:'*) ;;
  *) fail "Developer ID Application signature is missing" ;;
esac

app_archive="$temporary_directory/Fallout-Terminal-arm64.zip"
ditto -c -k --sequesterRsrc --keepParent "$app_path" "$app_archive"

printf '%s\n' "Submitting signed app archive for notarization..."
xcrun notarytool submit "$app_archive" \
  --keychain-profile "$NOTARYTOOL_KEYCHAIN_PROFILE" \
  --wait --output-format json
xcrun stapler staple "$app_path"
xcrun stapler validate "$app_path"
spctl --assess --type execute --verbose=4 "$app_path"

dmg_stage="$temporary_directory/dmg-root"
mkdir -p "$dmg_stage"
ditto "$app_path" "$dmg_stage/Fallout Terminal.app"
ln -s /Applications "$dmg_stage/Applications"

unsigned_dmg="$temporary_directory/Fallout-Terminal-arm64.dmg"
hdiutil create \
  -volname "$volume_name" \
  -srcfolder "$dmg_stage" \
  -format UDZO \
  -imagekey zlib-level=9 \
  "$unsigned_dmg"

codesign --force --timestamp \
  --sign "$DEVELOPER_ID_APPLICATION" \
  "$unsigned_dmg"
codesign --verify --strict --verbose=2 "$unsigned_dmg"

printf '%s\n' "Submitting signed DMG for notarization..."
xcrun notarytool submit "$unsigned_dmg" \
  --keychain-profile "$NOTARYTOOL_KEYCHAIN_PROFILE" \
  --wait --output-format json
xcrun stapler staple "$unsigned_dmg"
xcrun stapler validate "$unsigned_dmg"
spctl --assess --type open --context context:primary-signature --verbose=4 "$unsigned_dmg"

install -m 0644 "$unsigned_dmg" "$dmg_path"
dmg_checksum=$(shasum -a 256 "$dmg_path" | awk '{print $1}')
printf '%s  %s\n' "$dmg_checksum" "$(basename "$dmg_path")" >"$dmg_checksum_path"
chmod 0644 "$dmg_checksum_path"

printf '\nRelease artifacts verified:\n'
printf '  App: %s\n' "$app_path"
printf '  DMG: %s\n' "$dmg_path"
printf '  Architectures: '
lipo -archs "$app_executable"
printf '  SHA-256 (app executable): '
shasum -a 256 "$app_executable" | awk '{print $1}'
printf '  SHA-256 (DMG): '
printf '%s\n' "$dmg_checksum"
printf '  SHA-256 sidecar: %s\n' "$dmg_checksum_path"
