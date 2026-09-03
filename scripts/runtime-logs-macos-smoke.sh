#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  printf 'SKIP: packaged runtime-log smoke requires macOS\n'
  exit 0
fi

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
executable="${repository_root}/build/bin/Fallout Terminal.app/Contents/MacOS/Fallout Terminal"
log_directory="${HOME}/Library/Application Support/com.vaulttec.fallout-terminal/logs"
if [[ ! -x "${executable}" ]]; then
  printf 'SKIP: build the packaged application before running this optional smoke\n'
  exit 0
fi

printf 'NOT RUN: Overseer log-access UI interaction and two-interaction/ten-second SC-008 evidence require matching-host UI automation\n'

"${executable}" &
application_pid=$!
cleanup() {
  if kill -0 "${application_pid}" 2>/dev/null; then
    kill "${application_pid}" 2>/dev/null || true
    for _ in {1..20}; do
      if ! kill -0 "${application_pid}" 2>/dev/null; then
        break
      fi
      sleep 0.1
    done
    if kill -0 "${application_pid}" 2>/dev/null; then
      kill -KILL "${application_pid}" 2>/dev/null || true
    fi
  fi
  wait "${application_pid}" 2>/dev/null || true
}
trap cleanup EXIT

for _ in {1..20}; do
  if compgen -G "${log_directory}/application-*.log" >/dev/null; then
    printf 'PASS: packaged application retained logs in %s\n' "${log_directory}"
    exit 0
  fi
  sleep 0.5
done

printf 'FAIL: packaged application did not create a retained log in %s\n' "${log_directory}" >&2
exit 1
