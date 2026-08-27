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

service=$(find "$first" -type f -name desktopservice.js -print)
test "$(printf '%s\n' "$service" | sed '/^$/d' | wc -l | tr -d ' ')" = 1

actual="$temporary/actual-methods"
expected="$temporary/expected-methods"
sed -n 's/^export function \([A-Za-z0-9_]*\)(.*/\1/p' "$service" | LC_ALL=C sort >"$actual"
printf '%s\n' \
    AddCharacter AssignCharacter CopyDemo DeleteCharacter EndBroadcast ForceHackSuccess \
    GeneratePlayerPassword GetPublicAccess GetRuntimeStatus LoadReferencedPlayerConfig MoveCharacter NewPlayerConfig NewSession \
    OpenPlayerConfig OpenSession OpenURL ReleaseCharacter RenameLogicalSession ReplaceTerminalGroups \
    RequestTerminalActivation RequestTerminalClear ResetCommandState ResetFailedHack ResetTerminalCommandStates ResolveCommandExecution ResolveTerminalNavigation ResolveTerminalSwitch \
    SavePublicAccessSettings SaveSession SetActiveController StartBroadcast StartPublicAccess \
    StopPublicAccess UpdateCharacter UpdateLiveTerminal | LC_ALL=C sort >"$expected"
diff -u "$expected" "$actual"
test "$(wc -l <"$actual" | tr -d ' ')" = 35

for forbidden in Start Shutdown ServiceStartup ServiceShutdown Dispatch Call Capabilities \
    ReadFile WriteFile Exec Environment OpenDialog Browser PlayerService Subscribe; do
    ! grep -qx "$forbidden" "$actual"
done

! grep -ERn 'github\.com/wailsapp/wails/v2|frontend/wailsjs|window\.(go|runtime)|generic.?dispatch' "$first"
test -f "$first/github.com/obalunenko/Fallout-Terminal/desktopservice.js"

event_types="$first/github.com/wailsapp/wails/v3/internal/eventdata.d.ts"
test -f "$event_types"
for event in server-info client-count hack-state coordination-state session-state public-access-status; do
    grep -q "\"$event\"" "$event_types"
done
test "$(grep -E '^[[:space:]]+"(server-info|client-count|hack-state|coordination-state|session-state|public-access-status)"' "$event_types" | wc -l | tr -d ' ')" = 6

echo "Wails bindings are deterministic and expose exactly 35 accepted desktop methods."
