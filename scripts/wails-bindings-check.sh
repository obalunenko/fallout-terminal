#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

temporary=$(mktemp -d "${TMPDIR:-/tmp}/fallout-wails-bindings.XXXXXX")
trap 'rm -rf "$temporary"' EXIT HUP INT TERM
first="$temporary/first"
second="$temporary/second"

generate() {
    destination=$1
    GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/fallout-terminal-go-cache} \
        go tool -modfile=tools/wails/go.mod wails3 generate bindings \
        -clean -d "$destination" ./... >/dev/null
}

generate "$first"
generate "$second"
diff -ru "$first" "$second"
diff -ru frontend/overseer/bindings "$first"

service=$(find "$first" -type f -name desktopservice.js -print)
test "$(printf '%s\n' "$service" | sed '/^$/d' | wc -l | tr -d ' ')" = 1

actual="$temporary/actual-methods"
expected="$temporary/expected-methods"
sed -n 's/^export function \([A-Za-z0-9_]*\)(.*/\1/p' "$service" | LC_ALL=C sort >"$actual"
printf '%s\n' \
    AddCharacter AssignCharacter CopyDemo CopyPublicAccessCredentials DeleteCharacter EndBroadcast ForceHackSuccess \
    GeneratePlayerPassword GetApplicationUpdateStatus GetPublicAccess GetRuntimeStatus LoadReferencedPlayerConfig MoveCharacter NewPlayerConfig NewSession \
    OpenPlayerConfig OpenSession OpenURL ReleaseCharacter RenameLogicalSession ReplaceTerminalGroups \
    RequestTerminalActivation RequestTerminalClear ResetCommandState ResetFailedHack ResetTerminalCommandStates ResolveApplicationUpdateOffer ResolveApplicationUpdateRestart ResolveCommandExecution ResolveTerminalNavigation ResolveTerminalSwitch \
    SavePublicAccessSettings SaveSession SetActiveController StartBroadcast StartPublicAccess \
    StopPublicAccess UpdateCharacter UpdateLiveTerminal | LC_ALL=C sort >"$expected"
diff -u "$expected" "$actual"
test "$(wc -l <"$actual" | tr -d ' ')" = 39

for forbidden in Start Shutdown ServiceStartup ServiceShutdown Dispatch Call Capabilities \
    ReadFile WriteFile Exec Environment OpenDialog Browser PlayerService Subscribe; do
    ! grep -qx "$forbidden" "$actual"
done

! grep -ERn 'github\.com/wailsapp/wails/v2|frontend/wailsjs|window\.(go|runtime)|generic.?dispatch' "$first"
! grep -ERn 'ApplicationUpdate|fallout\.terminal\.private' frontend/client/gen
test -f "$first/github.com/obalunenko/Fallout-Terminal/v2/desktopservice.js"

event_types="$first/github.com/wailsapp/wails/v3/internal/eventdata.d.ts"
test -f "$event_types"

alias_declaration=frontend/overseer/src/adapters/wails-service.d.ts
test -f "$alias_declaration"

alias_methods="$temporary/alias-methods"
sed -n 's/^[[:space:]]*export function \([A-Za-z0-9_]*\)(.*/\1/p' "$alias_declaration" | LC_ALL=C sort >"$alias_methods"
diff -u "$actual" "$alias_methods"
test "$(wc -l <"$alias_methods" | tr -d ' ')" = 39

actual_events="$temporary/actual-events"
alias_events="$temporary/alias-events"
sed -n 's/^[[:space:]]*"\([^"]*\)".*/\1/p' "$event_types" | LC_ALL=C sort >"$actual_events"
sed -n "s/^[[:space:]]*readonly '\([^']*\)'.*/\1/p" "$alias_declaration" | LC_ALL=C sort >"$alias_events"
diff -u "$actual_events" "$alias_events"
test "$(wc -l <"$actual_events" | tr -d ' ')" = 7

for event in server-info client-count hack-state coordination-state session-state public-access-status application-update-status; do
    grep -qx "$event" "$actual_events"
done

alias_owners="$temporary/alias-owners"
rg -l "#wails-service" frontend/overseer | LC_ALL=C sort >"$alias_owners"
while IFS= read -r owner; do
    case "$owner" in
        frontend/overseer/src/adapters/desktop-api.ts|frontend/overseer/src/adapters/wails-service.d.ts|frontend/overseer/vite.config.ts) ;;
        *) echo "unexpected Wails service alias owner: $owner" >&2; exit 1 ;;
    esac
done <"$alias_owners"
grep -qx frontend/overseer/src/adapters/wails-service.d.ts "$alias_owners"
grep -qx frontend/overseer/vite.config.ts "$alias_owners"
! rg -n "#wails-service|@wailsio/runtime|frontend/overseer/bindings" frontend/client

typed_adapter=frontend/overseer/src/adapters/desktop-api.ts
if test -f "$typed_adapter"; then
    adapter_methods="$temporary/adapter-methods"
    adapter_inventory="$temporary/adapter-inventory"
    sed -n 's/.*desktopService\.\([A-Za-z0-9_]*\).*/\1/p' "$typed_adapter" | grep -v '^WailsDesktopEventMap$' | LC_ALL=C sort -u >"$adapter_methods"
    { cat "$adapter_methods"; printf '%s\n' CopyDemo; } | LC_ALL=C sort -u >"$adapter_inventory"
    diff -u "$actual" "$adapter_inventory"
    test "$(wc -l <"$adapter_methods" | tr -d ' ')" = 38
fi

echo "Wails bindings and the Overseer-only untrusted alias expose exactly 39 accepted desktop methods and seven named events."
