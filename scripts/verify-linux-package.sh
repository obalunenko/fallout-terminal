#!/usr/bin/env bash

set -euo pipefail

verification_deadline_seconds=60
workspace=''
app_pid=''
window_id=''
player_probe_pid=''
window_manager_pid=''
app_log=''

fail() {
  printf 'verify-linux-package: %s\n' "$1" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

process_is_alive() {
  [[ "${app_pid}" =~ ^[0-9]+$ ]] && kill -0 "${app_pid}" 2>/dev/null
}

player_probe_is_alive() {
  [[ "${player_probe_pid}" =~ ^[0-9]+$ ]] && kill -0 "${player_probe_pid}" 2>/dev/null
}

window_manager_is_alive() {
  [[ "${window_manager_pid}" =~ ^[0-9]+$ ]] && kill -0 "${window_manager_pid}" 2>/dev/null
}

ensure_window_manager() {
  xprop -root _NET_SUPPORTING_WM_CHECK 2>/dev/null | grep -q 'window id #' && return

  require_command openbox
  openbox >"${workspace}/openbox.log" 2>&1 &
  window_manager_pid=$!
  local deadline=$((SECONDS + 10))
  while ((SECONDS <= deadline)); do
    window_manager_is_alive || fail 'window manager exited during startup'
    xprop -root _NET_SUPPORTING_WM_CHECK 2>/dev/null | grep -q 'window id #' && return
    sleep 0.1
  done
  fail 'window manager did not claim the X11 display within 10 seconds'
}

wait_for_process_exit() {
  local deadline=$((SECONDS + verification_deadline_seconds))
  local state=''

  while ((SECONDS <= deadline)); do
    if ! process_is_alive; then
      return 0
    fi
    state="$(ps -o stat= -p "${app_pid}" 2>/dev/null | tr -d '[:space:]')"
    [[ "${state}" == Z* ]] && return 0
    sleep 0.2
  done
  return 1
}

cleanup() {
  local status=$?
  trap - EXIT HUP INT TERM

  if ((status != 0)) && [[ -s "${app_log}" ]]; then
    printf '%s\n' 'verify-linux-package: application log (first 120 lines):' >&2
    head -n 120 "${app_log}" >&2
    printf '%s\n' 'verify-linux-package: application log (last 160 lines):' >&2
    tail -n 160 "${app_log}" >&2
  fi

  if player_probe_is_alive; then
    kill -TERM "${player_probe_pid}" 2>/dev/null || true
    wait "${player_probe_pid}" 2>/dev/null || true
  fi

  if process_is_alive; then
    kill -TERM "${app_pid}" 2>/dev/null || true
    local deadline=$((SECONDS + 5))
    while process_is_alive && ((SECONDS <= deadline)); do
      sleep 0.1
    done
    if process_is_alive; then
      kill -KILL "${app_pid}" 2>/dev/null || true
    fi
    wait "${app_pid}" 2>/dev/null || true
  fi

  if window_manager_is_alive; then
    kill -TERM "${window_manager_pid}" 2>/dev/null || true
    wait "${window_manager_pid}" 2>/dev/null || true
  fi

  if [[ -n "${workspace}" ]]; then
    case "${workspace}" in
      "${tmp_parent}"/fallout-linux-package.*) rm -rf -- "${workspace}" ;;
      *) printf 'verify-linux-package: refusing unexpected cleanup path\n' >&2 ;;
    esac
  fi
  exit "${status}"
}

wait_for_listener() {
  local expected="$1"
  local deadline=$((SECONDS + verification_deadline_seconds))

  while ((SECONDS <= deadline)); do
    if python3 - "${expected}" <<'PY'
import socket
import sys

expect_open = sys.argv[1] == "open"
sock = socket.socket()
sock.settimeout(0.2)
try:
    sock.connect(("127.0.0.1", 3690))
    is_open = True
except OSError:
    is_open = False
finally:
    sock.close()
raise SystemExit(0 if is_open == expect_open else 1)
PY
    then
      return 0
    fi
    sleep 0.2
  done
  return 1
}

collect_descendants() {
  local root_pid="$1"
  local frontier=("${root_pid}")
  local next=()
  local parent=''
  local child=''

  while ((${#frontier[@]} > 0)); do
    next=()
    for parent in "${frontier[@]}"; do
      while read -r child; do
        [[ "${child}" =~ ^[0-9]+$ ]] || continue
        printf '%s\n' "${child}"
        next+=("${child}")
      done < <(ps -eo pid=,ppid= | awk -v parent="${parent}" '$2 == parent { print $1 }')
    done
    frontier=("${next[@]}")
  done
}

wait_for_descendants_exit() {
  local deadline=$((SECONDS + 15))
  local pid=''
  local alive=''

  while ((SECONDS <= deadline)); do
    alive=''
    for pid in "$@"; do
      if kill -0 "${pid}" 2>/dev/null; then
        alive="${pid}"
        break
      fi
    done
    [[ -z "${alive}" ]] && return 0
    sleep 0.2
  done
  return 1
}

start_player_probe() {
  local repository_root="$1"
  local evidence_path="$2"
  local probe_script="${workspace}/player connection probe.mjs"
  local connect_module="${repository_root}/frontend/node_modules/@connectrpc/connect/dist/esm/index.js"
  local transport_module="${repository_root}/frontend/node_modules/@connectrpc/connect-web/dist/esm/index.js"
  local service_module="${repository_root}/frontend/client/gen/fallout/terminal/player/v1/player_pb.js"

  for module in "${connect_module}" "${transport_module}" "${service_module}"; do
    [[ -f "${module}" ]] || fail 'generated player client dependencies are missing; run npm ci before native verification'
  done

  cat >"${probe_script}" <<'JS'
import { writeFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

const [connectPath, transportPath, servicePath, evidencePath] = process.argv.slice(2);
const [{ createClient }, { createConnectTransport }, { PlayerService }] = await Promise.all([
  import(pathToFileURL(connectPath).href),
  import(pathToFileURL(transportPath).href),
  import(pathToFileURL(servicePath).href),
]);
const controller = new AbortController();
for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => controller.abort());
}
const client = createClient(PlayerService, createConnectTransport({ baseUrl: "http://127.0.0.1:3690" }));
const iterator = client.subscribe(
  { clientInstanceId: "native-package-smoke" },
  { signal: controller.signal },
)[Symbol.asyncIterator]();
const first = await iterator.next();
if (first.done || first.value?.payload?.case !== "snapshot") {
  throw new Error("player subscription did not return a complete synchronized snapshot");
}
const snapshot = first.value.payload.value;
const candidate = snapshot?.playerState?.roster?.find((entry) => Number(entry.availability) === 1);
if (!snapshot?.recognitionHandle || !snapshot?.playerState?.broadcastId || !candidate?.characterId) {
  throw new Error("player snapshot did not expose an available synchronized control choice");
}
const selected = await client.selectCharacter({
  recognitionHandle: snapshot.recognitionHandle,
  requestId: "native-package-select-character",
  broadcastId: snapshot.playerState.broadcastId,
  characterId: candidate.characterId,
});
if (!selected?.accepted) throw new Error("player control action was rejected");
let synchronized = false;
while (!synchronized) {
  const next = await iterator.next();
  if (next.done) throw new Error("player subscription ended before synchronized control state");
  const state = next.value?.payload?.case === "update" ? next.value.payload.value?.playerState : null;
  synchronized = state?.assignedCharacter?.characterId === candidate.characterId;
}
await writeFile(evidencePath, "connected\ncontrol-accepted\nsynchronized\n", { encoding: "utf8", mode: 0o600 });
try {
  while (!(await iterator.next()).done) {
    // Keep the real subscription connected until the application is closed.
  }
} catch (error) {
  if (!controller.signal.aborted) throw error;
}
JS

  node "${probe_script}" "${connect_module}" "${transport_module}" "${service_module}" "${evidence_path}" \
    >"${workspace}/player-probe.stdout.log" 2>"${workspace}/player-probe.stderr.log" &
  player_probe_pid=$!
}

wait_for_player_evidence() {
  local evidence_path="$1"
  local deadline=$((SECONDS + 30))

  while ((SECONDS <= deadline)); do
    [[ -s "${evidence_path}" ]] && return 0
    player_probe_is_alive || return 1
    process_is_alive || return 1
    sleep 0.2
  done
  return 1
}

wait_for_window() {
  local deadline=$((SECONDS + verification_deadline_seconds))
  local candidate=''

  while ((SECONDS <= deadline)); do
    process_is_alive || return 1
    candidate="$(xdotool search --onlyvisible --pid "${app_pid}" --name 'Fallout Terminal.*Overseer Control' 2>/dev/null | head -n 1 || true)"
    if [[ "${candidate}" =~ ^[0-9]+$ ]]; then
      window_id="${candidate}"
      return 0
    fi
    sleep 0.2
  done
  return 1
}

wait_for_open_dialog() {
  local deadline=$((SECONDS + 15))
  local candidate=''

  while ((SECONDS <= deadline)); do
    process_is_alive || return 1
    candidate="$(xdotool search --onlyvisible --name '^Open Fallout Terminal Session$' 2>/dev/null | head -n 1 || true)"
    if [[ "${candidate}" =~ ^[0-9]+$ ]]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
    sleep 0.2
  done
  return 1
}

wait_for_session_open() {
  local log_path="$1"
  local deadline=$((SECONDS + verification_deadline_seconds))
  local succeeded='operation[=:][^[:space:]]*session\.open.*outcome[=:][^[:space:]]*succeeded|outcome[=:][^[:space:]]*succeeded.*operation[=:][^[:space:]]*session\.open'
  local failed='operation[=:][^[:space:]]*session\.open.*outcome[=:][^[:space:]]*(failed|rejected|cancelled)|outcome[=:][^[:space:]]*(failed|rejected|cancelled).*operation[=:][^[:space:]]*session\.open'

  while ((SECONDS <= deadline)); do
    process_is_alive || return 1
    grep -Eq "${succeeded}" "${log_path}" 2>/dev/null && return 0
    grep -Eq "${failed}" "${log_path}" 2>/dev/null && return 2
    sleep 0.2
  done
  return 1
}

wait_for_operation_success() {
  local log_path="$1"
  local operation="$2"
  local deadline=$((SECONDS + verification_deadline_seconds))
  local succeeded="operation[=:][^[:space:]]*${operation}.*outcome[=:][^[:space:]]*succeeded|outcome[=:][^[:space:]]*succeeded.*operation[=:][^[:space:]]*${operation}"

  while ((SECONDS <= deadline)); do
    process_is_alive || return 1
    grep -Eq "${succeeded}" "${log_path}" 2>/dev/null && return 0
    sleep 0.2
  done
  return 1
}

drive_demo_open() {
  local demo_path="$1"
  local dialog_id=''

  python3 "${repository_root}/scripts/native-ui-smoke.py" invoke --name 'ОТКРЫТЬ СЕССИЮ' || return 1
  dialog_id="$(wait_for_open_dialog)" || return 1
  xdotool windowactivate --sync "${dialog_id}"
  xdotool key --clearmodifiers --window "${dialog_id}" ctrl+l
  sleep 0.2
  xdotool type --clearmodifiers --window "${dialog_id}" --delay 1 -- "${demo_path}"
  xdotool key --clearmodifiers --window "${dialog_id}" Return
  sleep 0.5
  if xdotool search --onlyvisible --name '^Open Fallout Terminal Session$' >/dev/null 2>&1; then
    xdotool key --clearmodifiers --window "${dialog_id}" Return
  fi
}

[[ "$#" == 1 ]] || fail 'usage: scripts/verify-linux-package.sh ARCHIVE.tar.gz'
[[ "$(uname -s)" == Linux ]] || fail 'verification requires Linux'
[[ -n "${DISPLAY:-}" ]] || fail 'DISPLAY is required; start Xvfb and a window manager before verification'

for command in awk basename cat chmod cp dirname find gdbus go grep head ldd mkdir mktemp node pkg-config ps python3 readelf realpath rm sleep tail tar tr uname xdotool xdpyinfo xprop; do
  require_command "${command}"
done
xdpyinfo -display "${DISPLAY}" >/dev/null 2>&1 || fail "cannot connect to DISPLAY ${DISPLAY}"

[[ -f "$1" ]] || fail 'archive is missing'
archive="$(realpath "$1")"
[[ -s "${archive}" ]] || fail 'archive is empty'

case "$(uname -m)" in
  x86_64) expected_arch='amd64' ;;
  aarch64|arm64) expected_arch='arm64' ;;
  *) fail "unsupported native architecture: $(uname -m)" ;;
esac
expected_archive="Fallout-Terminal-linux-${expected_arch}.tar.gz"
[[ "$(basename "${archive}")" == "${expected_archive}" ]] || fail "archive name must be ${expected_archive}"

checksum_path="${archive}.sha256"
[[ -s "${checksum_path}" ]] || fail "checksum sidecar is missing: ${expected_archive}.sha256"

repository_root="$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")"

tmp_parent="${TMPDIR:-/tmp}"
tmp_parent="${tmp_parent%/}"
workspace="$(mktemp -d "${tmp_parent}/fallout-linux-package.XXXXXX")"
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

ensure_window_manager

canary_file="${workspace}/native credential canaries"
credential_log="${workspace}/native-credential-smoke.log"
if ! (cd "${repository_root}" && go run ./cmd/native-credential-smoke --canary-file "${canary_file}") >"${credential_log}" 2>&1; then
  head -n 80 "${credential_log}" >&2
  fail 'native Secret Service write/read/replace/delete acceptance failed'
fi

archive_entries="$(tar -tzf "${archive}")" || fail 'archive cannot be listed as tar.gz'
archive_details="$(tar -tvzf "${archive}")" || fail 'archive metadata cannot be read'
[[ -n "${archive_entries}" ]] || fail 'archive contains no entries'

declare -A seen_entries=()
while IFS= read -r entry; do
  normalized="${entry%/}"
  [[ -n "${normalized}" ]] || fail 'archive contains an empty entry name'
  case "${normalized}" in
    /*|../*|*/../*|*/..) fail 'archive contains an absolute or parent-traversal entry' ;;
  esac
  [[ -z "${seen_entries[${normalized}]+present}" ]] || fail "archive contains duplicate entry: ${normalized}"
  seen_entries["${normalized}"]=1
  case "${normalized}" in
    'Fallout Terminal'|'Fallout Terminal/'*) ;;
    *) fail "archive entry is outside the Fallout Terminal root: ${normalized}" ;;
  esac
done <<<"${archive_entries}"

while IFS= read -r detail; do
  case "${detail:0:1}" in
    -|d) ;;
    *) fail 'archive contains a link, device, socket, or other non-regular entry' ;;
  esac
done <<<"${archive_details}"

python3 - "${archive}" "${checksum_path}" "${expected_arch}" <<'PY'
import hashlib
import json
import pathlib
import re
import sys
import tarfile

archive_path = pathlib.Path(sys.argv[1])
checksum_path = pathlib.Path(sys.argv[2])
expected_arch = sys.argv[3]
expected_archive = f"Fallout-Terminal-linux-{expected_arch}.tar.gz"
expected_entries = [
    "Fallout Terminal/Fallout Terminal",
    "Fallout Terminal/artifact-manifest.json",
    "Fallout Terminal/resources/THIRD_PARTY_NOTICES.md",
    "Fallout Terminal/resources/appicon.png",
    "Fallout Terminal/resources/sessions/demo-players.json",
    "Fallout Terminal/resources/sessions/demo.json",
]
expected_files = [entry.removeprefix("Fallout Terminal/") for entry in expected_entries if not entry.endswith("artifact-manifest.json")]

digest = hashlib.sha256(archive_path.read_bytes()).hexdigest()
expected_sidecar = f"{digest}  {expected_archive}\n".encode()
assert checksum_path.read_bytes() == expected_sidecar, "checksum sidecar is malformed or does not match the archive"

with tarfile.open(archive_path, "r:gz") as source:
    members = source.getmembers()
    names = [member.name for member in members]
    assert names == expected_entries, "archive inventory/order differs from the schema-v1 contract"
    assert len(names) == len(set(names)), "archive contains duplicate entries"
    payload = {}
    modes = {}
    for member in members:
        assert member.isfile(), f"archive entry is not a regular file: {member.name}"
        assert member.mtime == 315532800, f"archive timestamp is not normalized: {member.name}"
        assert member.uid == 0 and member.gid == 0 and not member.uname and not member.gname, f"archive owner metadata is not normalized: {member.name}"
        stream = source.extractfile(member)
        assert stream is not None, f"archive entry cannot be read: {member.name}"
        payload[member.name.removeprefix("Fallout Terminal/")] = stream.read()
        modes[member.name.removeprefix("Fallout Terminal/")] = f"{member.mode & 0o7777:04o}"

manifest = json.loads(payload["artifact-manifest.json"])
assert set(manifest) == {"schemaVersion", "product", "sourceRevision", "target", "runtime", "files"}, "manifest fields differ from schema v1"
assert manifest["schemaVersion"] == 1 and manifest["product"] == "Fallout Terminal", "manifest identity is invalid"
assert re.fullmatch(r"(?:[0-9a-f]{40}|[0-9a-f]{64})", manifest["sourceRevision"]), "manifest source revision is invalid"
assert manifest["target"] == {"os": "linux", "arch": expected_arch}, "manifest target does not match the native runner"
assert manifest["runtime"] == "GTK4/WebKitGTK 6.0 and Secret Service", "manifest runtime identity is invalid"
records = manifest["files"]
assert [record.get("path") for record in records] == expected_files, "manifest inventory/order is invalid"
assert len({record["path"] for record in records}) == len(expected_files), "manifest contains duplicate paths"
for record in records:
    assert set(record) == {"path", "size", "mode", "sha256"}, f"manifest file fields are invalid: {record.get('path')}"
    path = record["path"]
    contents = payload[path]
    expected_mode = "0755" if path == "Fallout Terminal" else "0444"
    assert record["size"] == len(contents), f"manifest size mismatch: {path}"
    assert record["mode"] == expected_mode == modes[path], f"manifest/archive mode mismatch: {path}"
    assert record["sha256"] == hashlib.sha256(contents).hexdigest(), f"manifest hash mismatch: {path}"
assert modes["artifact-manifest.json"] == "0444", "artifact manifest archive mode must be 0444"
PY

extract_root="${workspace}/extracted package"
mkdir -p "${extract_root}"
tar --extract --gzip --file "${archive}" --directory "${extract_root}" --no-same-owner --no-same-permissions
package_root="${extract_root}/Fallout Terminal"
executable="${package_root}/Fallout Terminal"
resources="${package_root}/resources"
demo="${resources}/sessions/demo.json"
players="${resources}/sessions/demo-players.json"
artifact_manifest="${package_root}/artifact-manifest.json"

[[ -d "${package_root}" ]] || fail 'archive root directory is missing'
[[ -x "${executable}" && -f "${executable}" ]] || fail 'Linux application executable is missing or not executable'
for required in "${artifact_manifest}" "${resources}/appicon.png" "${resources}/THIRD_PARTY_NOTICES.md" "${demo}" "${players}"; do
  [[ -s "${required}" && -f "${required}" ]] || fail 'a required packaged resource is missing or empty'
done
unexpected_special="$(find "${package_root}" -mindepth 1 ! -type d ! -type f -print -quit)"
[[ -z "${unexpected_special}" ]] || fail 'extracted package contains a link or other special file'

elf_header="$(readelf -h "${executable}")" || fail 'Linux executable does not have a readable ELF header'
grep -Eq '^  Type:[[:space:]]+(DYN|EXEC)' <<<"${elf_header}" || fail 'Linux executable is not an executable ELF image'
case "${expected_arch}" in
  amd64) grep -Eq '^  Machine:[[:space:]]+Advanced Micro Devices X86-64' <<<"${elf_header}" || fail 'ELF machine is not AMD64' ;;
  arm64) grep -Eq '^  Machine:[[:space:]]+AArch64' <<<"${elf_header}" || fail 'ELF machine is not ARM64' ;;
esac
elf_dependencies="$(ldd "${executable}" 2>&1)" || fail 'Linux dynamic-loader inspection failed; verify the native runtime installation'
grep -Fq 'not found' <<<"${elf_dependencies}" && fail 'Linux executable has unresolved shared-library prerequisites'
pkg-config --exists gtk4 || fail 'GTK4 development/runtime metadata is unavailable; install gtk4'
pkg-config --exists webkitgtk-6.0 || fail 'WebKitGTK 6.0 development/runtime metadata is unavailable; install webkitgtk-6.0'
[[ -n "${DBUS_SESSION_BUS_ADDRESS:-}" ]] || fail 'a D-Bus session with Secret Service is required'
secret_service_owner="$(gdbus call --session --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus --method org.freedesktop.DBus.NameHasOwner org.freedesktop.secrets 2>/dev/null)" || fail 'could not query the D-Bus session for Secret Service'
[[ "${secret_service_owner}" == '(true,)' ]] || fail 'Secret Service is unavailable; start an unlocked native Secret Service test session'

python3 - "${demo}" "${players}" <<'PY'
import json
import sys

demo_path, players_path = sys.argv[1:]
with open(demo_path, encoding="utf-8") as source:
    demo = json.load(source)
assert demo.get("version") == 1, "bundled demo version must be 1"
assert demo.get("name") == "Убежище 76 — День Возрождения", "bundled demo identity is incorrect"
assert sum(
    terminal.get("name") == "Терминал смотрителя — Убежище 76"
    for terminal in demo.get("terminals", [])
) == 1, "bundled demo must contain the reviewed Overseer terminal"
assert demo.get("playerConfig") == "demo-players.json", "bundled demo player config reference is incorrect"
with open(players_path, encoding="utf-8") as source:
    players = json.load(source)
assert players.get("version") == 1, "bundled player config version must be 1"
PY

acceptance_root="${workspace}/acceptance fixtures"
launch_cwd="${workspace}/unrelated working directory"
mkdir -p "${acceptance_root}" "${launch_cwd}"
demo_copy="${acceptance_root}/demo.json"
players_copy="${acceptance_root}/demo-players.json"
cp "${demo}" "${demo_copy}"
cp "${players}" "${players_copy}"
chmod u+w "${demo_copy}" "${players_copy}"
app_log="${workspace}/application.log"

(cd "${launch_cwd}" && exec "${executable}" >"${app_log}" 2>&1) &
app_pid=$!
wait_for_window || fail 'a real Overseer window did not appear within 60 seconds'
wait_for_listener open || fail 'the local player listener did not become reachable on 127.0.0.1:3690 within 60 seconds'

drive_demo_open "${demo_copy}" || fail 'could not drive the native OpenSession dialog'
if wait_for_session_open "${app_log}"; then
  :
else
  session_status=$?
  if [[ "${session_status}" == 2 ]]; then
    fail 'the application rejected the packaged demo through OpenSession'
  fi
  fail 'the application did not confirm packaged demo loading within 60 seconds'
fi
process_is_alive || fail 'application exited after loading the packaged demo'
xdotool search --onlyvisible --pid "${app_pid}" --name 'Fallout Terminal.*Overseer Control' >/dev/null 2>&1 || fail 'Overseer window disappeared after loading the packaged demo'

python3 "${repository_root}/scripts/native-ui-smoke.py" invoke --name '+ ПАПКА' || fail 'could not perform a session-authoring action'
wait_for_operation_success "${app_log}" 'session\.save' || fail 'the opened JSON session was not durably saved'

xdotool windowclose "${window_id}"
wait_for_process_exit || fail 'application did not exit after saving the session copy'
if wait "${app_pid}"; then
  :
else
  exit_status=$?
  app_pid=''
  fail "application exited with status ${exit_status} after saving the session copy"
fi
app_pid=''
wait_for_listener closed || fail 'local player listener remained reachable before reopening the saved copy'
grep -Fq 'application shutdown completed' "${app_log}" || fail 'first application lifecycle did not release its resources'

app_log="${workspace}/application-reopen.log"
window_id=''
(cd "${launch_cwd}" && exec "${executable}" >"${app_log}" 2>&1) &
app_pid=$!
wait_for_window || fail 'a real Overseer window did not reappear within 60 seconds'
wait_for_listener open || fail 'the local player listener did not restart before reopening the saved copy'
drive_demo_open "${demo_copy}" || fail 'could not reopen the saved JSON copy through the native dialog'
wait_for_session_open "${app_log}" || fail 'the saved JSON copy did not reopen successfully'

python3 "${repository_root}/scripts/native-ui-smoke.py" invoke --name 'НАЧАТЬ ТРАНСЛЯЦИЮ' || fail 'could not start the demo broadcast'
python3 "${repository_root}/scripts/native-ui-smoke.py" invoke --name 'http://127.0.0.1:' --prefix || fail 'could not exercise the allowlisted HTTP external link'
python3 "${repository_root}/scripts/native-ui-smoke.py" assert-canaries-absent --canary-file "${canary_file}" || fail 'credential canary reached public accessibility state'

player_evidence="${workspace}/player connection.evidence"
start_player_probe "${repository_root}" "${player_evidence}"
wait_for_player_evidence "${player_evidence}" || fail 'one local player did not complete a synchronized control action within 30 seconds'
grep -Fq 'control-accepted' "${player_evidence}" || fail 'player control action was not accepted'
grep -Fq 'synchronized' "${player_evidence}" || fail 'player control state was not synchronized'
grep -Fq 'player server ready' "${app_log}" || fail 'application log does not confirm player-listener readiness'
grep -Fq 'application ready' "${app_log}" || fail 'application did not reach local-ready state with native Secret Service available'

mapfile -t application_descendants < <(collect_descendants "${app_pid}")

xdotool windowclose "${window_id}"
wait_for_process_exit || fail 'application did not exit after the Overseer window closed'
if wait "${app_pid}"; then
  :
else
  exit_status=$?
  app_pid=''
  fail "application exited with status ${exit_status}"
fi
app_pid=''

wait_for_listener closed || fail 'local player listener remained reachable after application exit'
if ((${#application_descendants[@]} > 0)); then
  wait_for_descendants_exit "${application_descendants[@]}" || fail 'an application-owned child process remained after shutdown'
fi
grep -Fq 'application shutdown completed' "${app_log}" || fail 'application log does not confirm complete resource release'

"${repository_root}/scripts/secret-leak-check.sh" --canary-file "${canary_file}" --scan-root "${workspace}"
linux_settings_root="${XDG_CONFIG_HOME:-${HOME}/.config}/com.vaulttec.fallout-terminal"
if [[ -d "${linux_settings_root}" ]]; then
  "${repository_root}/scripts/secret-leak-check.sh" --canary-file "${canary_file}" --scan-root "${linux_settings_root}"
fi

if player_probe_is_alive; then
  kill -TERM "${player_probe_pid}" 2>/dev/null || true
  wait "${player_probe_pid}" 2>/dev/null || true
fi
player_probe_pid=''

printf 'Verified %s: ELF/runtime identity, exact manifest/checksum, native JSON open/save, HTTP link, player control synchronization, protected credential round trip/leak scan, and complete resource release.\n' "${expected_archive}"
