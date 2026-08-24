#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
default_app_path="${repository_root}/build/dev/Fallout Terminal.app"
default_fixture="${repository_root}/internal/testutil/testdata/session-v1-state-changing.json"
default_player_config="${repository_root}/sessions/demo-players.json"
player_probe="${repository_root}/scripts/state-changing-reset-native-player-smoke.mjs"
smoke_mode='reset'
if [[ "${1:-}" == '--presentation' ]]; then
  smoke_mode='presentation'
  shift
fi
app_path="${1:-${default_app_path}}"
fixture_path="${2:-${default_fixture}}"
player_config_fixture="${3:-${default_player_config}}"
local_url='http://127.0.0.1:3690/'
smoke_deadline_seconds=30
smoke_root=''
app_pid=''
player_probe_pid=''
smoke_succeeded=0

capture_diagnostics() {
  [[ -n "${smoke_root}" && -d "${smoke_root}" ]] || return 0
  {
    printf 'captured_at=%s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
    printf 'app_path=%s\n' "${app_path}"
    printf 'tracked_app_pid=%s\n' "${app_pid}"
    printf 'matching_process_ids=%s\n' "$(process_ids | tr '\n' ' ')"
    if [[ "${app_pid}" =~ ^[0-9]+$ ]]; then
      ps -p "${app_pid}" -o pid=,ppid=,stat=,etime=,command= 2>&1 || true
    fi
  } >"${smoke_root}/native-process.txt" 2>&1
  curl --silent --show-error --max-time 2 --dump-header "${smoke_root}/local-response.headers" \
    --output "${smoke_root}/local-response.html" "${local_url}" 2>"${smoke_root}/local-response.error" || true
  if [[ "${app_pid}" =~ ^[0-9]+$ ]] && kill -0 "${app_pid}" 2>/dev/null; then
    osascript - "${app_pid}" >"${smoke_root}/native-accessibility.txt" 2>&1 <<'APPLESCRIPT' || true
on run argv
  set processID to (item 1 of argv) as integer
  tell application "System Events" to tell first process whose unix id is processID
    try
      set snapshot to ""
      repeat with candidate in entire contents of window 1
        try
          set snapshot to snapshot & (role of candidate as text) & " | " & (name of candidate as text) & " | " & (description of candidate as text) & linefeed
        end try
      end repeat
      return snapshot
    on error messageText
      return "accessibility snapshot failed: " & messageText
    end try
  end tell
end run
APPLESCRIPT
  fi
}

fail() {
  capture_diagnostics
  printf 'state-changing-reset-native-smoke: FAIL: %s\n' "$1" >&2
  [[ -z "${smoke_root}" ]] || printf 'state-changing-reset-native-smoke: diagnostics preserved at %s\n' "${smoke_root}" >&2
  exit 1
}

pass() {
  printf 'state-changing-reset-native-smoke: PASS: %s\n' "$1"
}

not_run() {
  capture_diagnostics
  printf 'state-changing-reset-native-smoke: NOT RUN: %s\n' "$1"
  [[ -z "${smoke_root}" ]] || printf 'state-changing-reset-native-smoke: diagnostics preserved at %s\n' "${smoke_root}" >&2
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

process_ids() {
  pgrep -f -- "${app_path}/Contents/MacOS/Fallout Terminal" 2>/dev/null || true
}

process_is_alive() {
  [[ "${app_pid}" =~ ^[0-9]+$ ]] && kill -0 "${app_pid}" 2>/dev/null
}

wait_for_single_process() {
  local deadline=$((SECONDS + smoke_deadline_seconds))
  local ids=''
  local count='0'
  while (( SECONDS <= deadline )); do
    ids="$(process_ids)"
    if [[ -n "${ids}" ]]; then
      count="$(printf '%s\n' "${ids}" | wc -l | tr -d ' ')"
      if [[ "${count}" == 1 ]] && kill -0 "${ids}" 2>/dev/null; then
        app_pid="${ids}"
        return 0
      fi
    fi
    sleep 0.1
  done
  return 1
}

wait_for_local() {
  local deadline=$((SECONDS + smoke_deadline_seconds))
  while (( SECONDS <= deadline )); do
    if [[ -n "${app_pid}" ]] && ! process_is_alive; then
      return 2
    fi
    if curl --fail --silent --show-error --max-time 1 "${local_url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

wait_for_file_reset() {
  local session_path="$1"
  local terminal_id="$2"
  local deadline=$((SECONDS + smoke_deadline_seconds))
  while (( SECONDS <= deadline )); do
    if python3 -c 'import json,sys; d=json.load(open(sys.argv[1], encoding="utf-8")); t=next(x for x in d["terminals"] if x["id"]==sys.argv[2]); raise SystemExit(0 if not t.get("commandStates") else 1)' "${session_path}" "${terminal_id}"; then
      return 0
    fi
    sleep 0.05
  done
  return 1
}

wait_for_marker() {
  local marker_path="$1"
  local log_path="$2"
  local deadline=$((SECONDS + smoke_deadline_seconds))
  while (( SECONDS <= deadline )); do
    [[ -f "${marker_path}" ]] && return 0
    if [[ "${player_probe_pid}" =~ ^[0-9]+$ ]] && ! kill -0 "${player_probe_pid}" 2>/dev/null; then
      [[ -f "${log_path}" ]] && tail -20 "${log_path}" >&2
      return 1
    fi
    sleep 0.05
  done
  [[ -f "${log_path}" ]] && tail -20 "${log_path}" >&2
  return 1
}

wait_for_process_exit() {
  local deadline=$((SECONDS + smoke_deadline_seconds))
  while (( SECONDS <= deadline )); do
    [[ -z "$(process_ids)" ]] && return 0
    sleep 0.1
  done
  return 1
}

cleanup() {
  if [[ "${player_probe_pid}" =~ ^[0-9]+$ ]]; then
    kill -TERM "${player_probe_pid}" 2>/dev/null || true
  fi
  if [[ "${app_pid}" =~ ^[0-9]+$ ]]; then
    kill -TERM "${app_pid}" 2>/dev/null || true
  fi
  if [[ "${smoke_succeeded}" == 1 && -n "${smoke_root}" ]]; then
    case "${smoke_root}" in
      /private/tmp/fallout-native-reset.*|/tmp/fallout-native-reset.*) rm -rf -- "${smoke_root}" ;;
      *) printf 'state-changing-reset-native-smoke: refusing unexpected cleanup path: %s\n' "${smoke_root}" >&2 ;;
    esac
  fi
}

validate_command_only_fixture() {
  python3 -c '
import json, sys
document = json.load(open(sys.argv[1], encoding="utf-8"))
for terminal in document.get("terminals", []):
    nodes = {}
    stack = [terminal.get("root", {})]
    while stack:
        node = stack.pop()
        nodes[node.get("id")] = node.get("type")
        stack.extend(node.get("children", []))
    for command_id in terminal.get("commandStates", {}):
        if nodes.get(command_id) != "command":
            raise SystemExit(f"commandStates[{command_id!r}] does not reference a command node")
' "$1"
}

run_self_test() {
  validate_command_only_fixture "${default_fixture}"
  node --check "${player_probe}"
  grep -Fq "resetConfirmationDialog.id = 'resetConfirmationDialog';" "${repository_root}/frontend/overseer/src/overseer.js" ||
    fail 'overseer reset confirmation dialog is missing'
  grep -Fq 'desktopAPI.resetTerminalCommandStates({ terminalId: term.id })' "${repository_root}/frontend/overseer/src/overseer.js" ||
    fail 'overseer reset-all control is not wired to the generated desktop facade'
  grep -Fq 'resetTerminalCommandStates: desktopService.ResetTerminalCommandStates' "${repository_root}/frontend/overseer/src/desktop-api.js" ||
    fail 'desktop facade is not wired to the generated Wails binding'
  grep -Fq 'saveStatus.dataset.sessionStateRevision' "${repository_root}/frontend/overseer/src/overseer.js" ||
    fail 'overseer session-state evidence is missing'
  grep -Fq 'screen.dataset.runtimeRevision' "${repository_root}/frontend/client/client.js" ||
    fail 'player runtime revision evidence is missing'
  if grep -F 'window.confirm(`Сбросить' "${repository_root}/frontend/overseer/src/overseer.js" >/dev/null; then
    fail 'command-state reset still depends on unsupported native window.confirm'
  fi
  pass 'command-only fixture, native overseer evidence, generated Wails chain, and two-player probe are present'
}

launch_app() {
  open -n "${app_path}"
  wait_for_single_process || fail 'expected exactly one live native application process after launch'
  if ! wait_for_local; then
    process_is_alive || fail 'native application exited before local mode became ready'
    fail 'application did not make local mode ready'
  fi
}

drive_native_setup() {
  local session_path="$1"
  local player_config_path="$2"
  osascript - "${app_pid}" "${session_path}" "${player_config_path}" >>"${smoke_root}/native-automation.log" 2>&1 <<'APPLESCRIPT'
on findNamedButton(containerElement, buttonName)
  tell application "System Events"
    try
      if role of containerElement is "AXButton" and name of containerElement is buttonName then return containerElement
      set childElements to UI elements of containerElement
    on error
      set childElements to {}
    end try
  end tell
  repeat with childElement in childElements
    set candidate to my findNamedButton(childElement, buttonName)
    if candidate is not missing value then return candidate
  end repeat
  return missing value
end findNamedButton

on clickButton(processID, buttonName, attempts)
  repeat attempts times
    tell application "System Events" to tell first process whose unix id is processID
      set frontmost to true
      try
        set candidate to my findNamedButton(window 1, buttonName)
        if candidate is not missing value and enabled of candidate then
          click candidate
          return true
        end if
      end try
    end tell
    delay 0.1
  end repeat
  return false
end clickButton

on chooseFile(processID, buttonName, filePath)
  if not my clickButton(processID, buttonName, 150) then error buttonName & " button not found"
  delay 0.4
  tell application "System Events" to tell first process whose unix id is processID
    set frontmost to true
    keystroke "g" using {command down, shift down}
    delay 0.2
    keystroke filePath
    key code 36
    delay 0.4
    key code 36
  end tell
  delay 0.6
end chooseFile

on run argv
  set processID to (item 1 of argv) as integer
  set sessionPath to item 2 of argv
  set playerConfigPath to item 3 of argv
  my chooseFile(processID, "ОТКРЫТЬ СЕССИЮ", sessionPath)
  if playerConfigPath is not "" then my chooseFile(processID, "ВЫБРАТЬ ФАЙЛ", playerConfigPath)
  if not my clickButton(processID, "НАЧАТЬ ТРАНСЛЯЦИЮ", 150) then error "start-broadcast button not found"
  if not my clickButton(processID, "СДЕЛАТЬ АКТИВНЫМ", 150) then error "make-live button not found"
end run
APPLESCRIPT
}

drive_native_reset() {
  osascript - "${app_pid}" >>"${smoke_root}/native-automation.log" 2>&1 <<'APPLESCRIPT'
on findNamedButton(containerElement, buttonName)
  tell application "System Events"
    try
      if role of containerElement is "AXButton" and name of containerElement is buttonName then return containerElement
      set childElements to UI elements of containerElement
    on error
      set childElements to {}
    end try
  end tell
  repeat with childElement in childElements
    set candidate to my findNamedButton(childElement, buttonName)
    if candidate is not missing value then return candidate
  end repeat
  return missing value
end findNamedButton

on clickButton(processID, buttonName, attempts)
  repeat attempts times
    tell application "System Events" to tell first process whose unix id is processID
      set frontmost to true
      try
        set candidate to my findNamedButton(window 1, buttonName)
        if candidate is not missing value and enabled of candidate then
          click candidate
          return true
        end if
      end try
    end tell
    delay 0.1
  end repeat
  return false
end clickButton

on clickResetConfirmation(processID)
  tell application "System Events" to tell first process whose unix id is processID
    set frontmost to true
    set windowPosition to position of window 1
    set windowSize to size of window 1
    set confirmX to (item 1 of windowPosition) + ((item 1 of windowSize) div 2) - 130
    set confirmY to (item 2 of windowPosition) + ((item 2 of windowSize) div 2) + 52
    click at {confirmX, confirmY}
  end tell
end clickResetConfirmation

on run argv
  set processID to (item 1 of argv) as integer
  if not my clickButton(processID, "СБРОСИТЬ ВСЕ СОСТОЯНИЯ", 150) then error "reset-all button not found"
  delay 0.4
  my clickResetConfirmation(processID)
end run
APPLESCRIPT
}

verify_native_overseer_reset() {
  local terminal_id="$1"
  osascript - "${app_pid}" "${terminal_id}" >>"${smoke_root}/native-automation.log" 2>&1 <<'APPLESCRIPT'
on findNamedButton(containerElement, buttonName)
  tell application "System Events"
    try
      if role of containerElement is "AXButton" and name of containerElement is buttonName then return containerElement
      set childElements to UI elements of containerElement
    on error
      set childElements to {}
    end try
  end tell
  repeat with childElement in childElements
    set candidate to my findNamedButton(childElement, buttonName)
    if candidate is not missing value then return candidate
  end repeat
  return missing value
end findNamedButton

on findEvidence(containerElement, terminalID)
  tell application "System Events"
    try
      set elementName to name of containerElement as text
      set elementDescription to description of containerElement as text
      if (elementName contains "Wails command ResetTerminalCommandStates ok" or elementDescription contains "Wails command ResetTerminalCommandStates ok") and ¬
        (elementName contains ("terminal " & terminalID) or elementDescription contains ("terminal " & terminalID)) and ¬
        (elementName contains "document revision" or elementDescription contains "document revision") and ¬
        (elementName contains "session-state revision" or elementDescription contains "session-state revision") then return containerElement
      set childElements to UI elements of containerElement
    on error
      set childElements to {}
    end try
  end tell
  repeat with childElement in childElements
    set candidate to my findEvidence(childElement, terminalID)
    if candidate is not missing value then return candidate
  end repeat
  return missing value
end findEvidence

on run argv
  set processID to (item 1 of argv) as integer
  set terminalID to item 2 of argv
  repeat 300 times
    tell application "System Events" to tell first process whose unix id is processID
      try
        set resetButton to my findNamedButton(window 1, "СБРОСИТЬ ВСЕ СОСТОЯНИЯ")
        set evidence to my findEvidence(window 1, terminalID)
        if resetButton is not missing value and not (enabled of resetButton) and evidence is not missing value then return true
      end try
    end tell
    delay 0.1
  end repeat
  error "overseer DOM did not expose canonical Wails/session-state reset evidence"
end run
APPLESCRIPT
}

verify_native_overseer_reopen() {
  osascript - "${app_pid}" >>"${smoke_root}/native-automation.log" 2>&1 <<'APPLESCRIPT'
on findNamedButton(containerElement, buttonName)
  tell application "System Events"
    try
      if role of containerElement is "AXButton" and name of containerElement is buttonName then return containerElement
      set childElements to UI elements of containerElement
    on error
      set childElements to {}
    end try
  end tell
  repeat with childElement in childElements
    set candidate to my findNamedButton(childElement, buttonName)
    if candidate is not missing value then return candidate
  end repeat
  return missing value
end findNamedButton

on run argv
  set processID to (item 1 of argv) as integer
  repeat 300 times
    tell application "System Events" to tell first process whose unix id is processID
      try
        set resetButton to my findNamedButton(window 1, "СБРОСИТЬ ВСЕ СОСТОЯНИЯ")
        if resetButton is not missing value and not (enabled of resetButton) then return true
      end try
    end tell
    delay 0.1
  end repeat
  error "reopened overseer DOM did not remain INITIAL"
end run
APPLESCRIPT
}

start_player_probe() {
  local mode="$1"
  local ready_path="$2"
  local trigger_path="$3"
  local result_path="$4"
  local log_path="$5"
  node "${player_probe}" "${mode}" "${local_url}" "${ready_path}" "${trigger_path}" "${result_path}" >"${log_path}" 2>&1 &
  player_probe_pid=$!
}

finish_player_probe() {
  local result_path="$1"
  local log_path="$2"
  wait_for_marker "${result_path}" "${log_path}" || fail 'player probe did not produce its result'
  if ! wait "${player_probe_pid}"; then
    tail -20 "${log_path}" >&2 || true
    fail 'player probe failed'
  fi
  player_probe_pid=''
}

close_app() {
  process_is_alive || fail 'native application exited before the requested full close'
  osascript -e "tell application \"System Events\" to tell first process whose unix id is ${app_pid} to keystroke \"q\" using {command down}" >>"${smoke_root}/native-automation.log" 2>&1
  wait_for_process_exit || fail 'native application did not fully close'
  app_pid=''
}

if [[ "${1:-}" == '--self-test' ]]; then
  run_self_test
  exit 0
fi

[[ "$#" -le 3 ]] || fail 'usage: scripts/state-changing-reset-native-smoke.sh [APP_PATH] [SESSION_FIXTURE] [PLAYER_CONFIG]'
[[ "$(uname -s)" == Darwin ]] || fail 'native overseer-click smoke requires macOS'
for command in curl grep mktemp node open osascript pgrep python3; do
  require_command "${command}"
done
[[ -d "${app_path}" ]] || fail 'application bundle is missing'
[[ -x "${app_path}/Contents/MacOS/Fallout Terminal" ]] || fail 'application executable is missing'
[[ -f "${fixture_path}" ]] || fail 'session fixture is missing'
[[ -f "${player_config_fixture}" ]] || fail 'player config fixture is missing'
[[ -f "${player_probe}" ]] || fail 'player probe is missing'
[[ -d "${repository_root}/tests/browser/node_modules/playwright" ]] || fail 'Playwright is not installed; run npm ci --prefix tests/browser'
app_path="$(cd "$(dirname "${app_path}")" && pwd)/$(basename "${app_path}")"
fixture_path="$(cd "$(dirname "${fixture_path}")" && pwd)/$(basename "${fixture_path}")"
player_config_fixture="$(cd "$(dirname "${player_config_fixture}")" && pwd)/$(basename "${player_config_fixture}")"
validate_command_only_fixture "${fixture_path}"
[[ -z "$(process_ids)" ]] || fail 'a matching application process is already running'

smoke_root="$(mktemp -d /private/tmp/fallout-native-reset.XXXXXX)"
session_path="${smoke_root}/session.json"
player_config_path="${smoke_root}/players.json"
cp "${fixture_path}" "${session_path}"
cp "${player_config_fixture}" "${player_config_path}"
chmod u+w "${session_path}" "${player_config_path}"
terminal_id="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1], encoding="utf-8")); print(next(t["id"] for t in d["terminals"] if t.get("commandStates")))' "${session_path}")"
before_selected_states="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1], encoding="utf-8")); t=next(x for x in d["terminals"] if x["id"]==sys.argv[2]); print(len(t.get("commandStates", {})))' "${session_path}" "${terminal_id}")"
before_other_states="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1], encoding="utf-8")); print(sum(len(t.get("commandStates", {})) for t in d["terminals"] if t["id"] != sys.argv[2]))' "${session_path}" "${terminal_id}")"
[[ "${before_selected_states}" =~ ^[1-9][0-9]*$ ]] || fail 'selected fixture terminal has no completed command snapshots'

trap cleanup EXIT HUP INT TERM
launch_app
if ! drive_native_setup "${session_path}" "${player_config_path}"; then
  process_is_alive || fail 'native application exited while Accessibility automation prepared the window'
  not_run 'Accessibility automation could not prepare the native window; grant Accessibility access to the invoking terminal and rerun'
  exit 2
fi

if [[ "${smoke_mode}" == presentation ]]; then
  presentation_ready="${smoke_root}/presentation-ready.json"
  presentation_trigger="${smoke_root}/presentation-trigger"
  presentation_result="${smoke_root}/presentation-result.json"
  presentation_log="${smoke_root}/presentation-player.log"
  start_player_probe presentation "${presentation_ready}" "${presentation_trigger}" "${presentation_result}" "${presentation_log}"
  finish_player_probe "${presentation_result}" "${presentation_log}"
  python3 -c '
import json, sys
result = json.load(open(sys.argv[1], encoding="utf-8"))
assert result["feedbackElapsedMilliseconds"] <= 100, result
assert result["convergenceElapsedMilliseconds"] <= 1000, result
assert result["controller"] == result["observer"] == "Тревога отключена", result
assert result["observerInputSuppressed"] is True, result
print("presentation feedback {}ms; controller/observer convergence {}ms".format(
    result["feedbackElapsedMilliseconds"], result["convergenceElapsedMilliseconds"]))
' "${presentation_result}"
  close_app
  smoke_succeeded=1
  pass 'packaged controller presentation converged to the observer within 1s and observer input stayed inert'
  exit 0
fi

reset_ready="${smoke_root}/reset-ready.json"
reset_trigger="${smoke_root}/reset-trigger"
reset_result="${smoke_root}/reset-result.json"
reset_log="${smoke_root}/reset-player.log"
start_player_probe reset "${reset_ready}" "${reset_trigger}" "${reset_result}" "${reset_log}"
wait_for_marker "${reset_ready}" "${reset_log}" || fail 'controller and observer did not reach the completed pre-reset screen'

drive_native_reset || fail 'native overseer reset click or confirmation failed'
touch "${reset_trigger}"
wait_for_file_reset "${session_path}" "${terminal_id}" ||
  fail 'native overseer click did not remove canonical commandStates within the deadline'
verify_native_overseer_reset "${terminal_id}" || fail 'overseer did not expose Wails result and session-state evidence'
finish_player_probe "${reset_result}" "${reset_log}"

after_selected_states="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1], encoding="utf-8")); t=next(x for x in d["terminals"] if x["id"]==sys.argv[2]); print(len(t.get("commandStates", {})))' "${session_path}" "${terminal_id}")"
after_other_states="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1], encoding="utf-8")); print(sum(len(t.get("commandStates", {})) for t in d["terminals"] if t["id"] != sys.argv[2]))' "${session_path}" "${terminal_id}")"
[[ "${after_selected_states}" == 0 ]] || fail 'native reset left a completed command snapshot in the selected terminal'
[[ "${after_other_states}" == "${before_other_states}" ]] || fail 'native reset changed command snapshots in another terminal'
validate_command_only_fixture "${session_path}"

close_app
launch_app
drive_native_setup "${session_path}" '' || fail 'native application could not reopen the same session and referenced player config'

reopen_ready="${smoke_root}/reopen-ready.json"
reopen_trigger="${smoke_root}/reopen-trigger"
reopen_result="${smoke_root}/reopen-result.json"
reopen_log="${smoke_root}/reopen-player.log"
start_player_probe reopen "${reopen_ready}" "${reopen_trigger}" "${reopen_result}" "${reopen_log}"
finish_player_probe "${reopen_result}" "${reopen_log}"
verify_native_overseer_reopen || fail 'overseer DOM did not remain initial after full reopen'

python3 -c '
import json, sys
reset = json.load(open(sys.argv[1], encoding="utf-8"))
reopened = json.load(open(sys.argv[2], encoding="utf-8"))
assert reset["afterRevision"] > reset["beforeRevision"], reset
assert reset["convergenceElapsedMilliseconds"] <= 1000, reset
assert reset["controller"] == reset["observer"] == "INITIAL", reset
assert reset["staleResultAfterNavigation"] is False, reset
assert reopened["controller"] == reopened["observer"] == "INITIAL", reopened
assert reopened["reopened"] is True, reopened
print("runtime {}->{} in {}ms; reopen runtime {}".format(
    reset["beforeRevision"], reset["afterRevision"],
    reset["convergenceElapsedMilliseconds"], reopened["runtimeRevision"]))
' "${reset_result}" "${reopen_result}"

smoke_succeeded=1
pass "native click traversed generated Wails reset for terminal ${terminal_id}; selected commandStates ${before_selected_states}->${after_selected_states}, other terminal preserved ${before_other_states}; overseer/controller/observer reached INITIAL within 1s, navigation stayed initial, and full reopen preserved it"
