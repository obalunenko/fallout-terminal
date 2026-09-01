#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
evidence_path='specs/033-frontend-vue-typescript-migration/evidence/T003-node-red.txt'

fail() {
  printf 'frontend task contract check: %s\n' "$1" >&2
  return 1
}

node_policy_product_test() {
  local temporary_root fake_node real_node output status
  temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/fallout-node-policy.XXXXXX")"
  trap 'rm -rf "$temporary_root"' RETURN
  fake_node="$temporary_root/node"
  real_node="$(command -v node)"
  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    '[[ "${1:-}" == "-e" && "$#" == 2 ]] || exit 64' \
    'exec "$FRONTEND_REAL_NODE" -e '\''Object.defineProperty(process.versions, "node", { value: process.env.FRONTEND_FAKE_NODE_VERSION }); Object.defineProperty(process, "version", { value: `v${process.env.FRONTEND_FAKE_NODE_VERSION}` }); eval(process.argv[1]);'\'' "$2"' \
    >"$fake_node"
  chmod +x "$fake_node"

  FRONTEND_REAL_NODE="$real_node" FRONTEND_FAKE_NODE_VERSION=26.8.1 \
    task -d "$repository_root" node:check NODE="$fake_node" >/dev/null 2>&1 || {
      fail 'expected Node.js 26.8.1 to be accepted'
      return 1
    }

  set +e
  output="$(FRONTEND_REAL_NODE="$real_node" FRONTEND_FAKE_NODE_VERSION=26.8.0 \
    task -d "$repository_root" node:check NODE="$fake_node" 2>&1)"
  status=$?
  set -e
  ((status != 0)) && [[ "$output" == *'Node.js 26.8.1 is required; found v26.8.0'* ]] || {
    printf '%s\n' "$output" >&2
    fail 'expected Node.js 26.8.0 to be rejected actionably'
    return 1
  }

  set +e
  output="$(FRONTEND_REAL_NODE="$real_node" FRONTEND_FAKE_NODE_VERSION=26.8.2 \
    task -d "$repository_root" node:check NODE="$fake_node" 2>&1)"
  status=$?
  set -e
  if ((status == 0)); then
    printf '%s\n' 'AssertionError: expected Node.js 26.8.2 to be rejected'
    return 1
  fi
  [[ "$output" == *'Node.js 26.8.1 is required; found v26.8.2'* ]] || {
    printf '%s\n' "$output" >&2
    fail 'Node.js 26.8.2 rejection was not actionable'
    return 1
  }

  rm -rf "$temporary_root"
  trap - RETURN
}

node_version_self_test() {
  local red_evidence="$repository_root/$evidence_path"
  [[ -r "$red_evidence" ]] || { fail "missing T003 RED evidence: $evidence_path"; return 1; }
  grep -Fxq 'Expected assertion: AssertionError: expected Node.js 26.8.2 to be rejected' "$red_evidence" || {
    fail 'T003 RED evidence does not contain the exact newer-version assertion'
    return 1
  }
  node_policy_product_test
  printf '%s\n' 'frontend task contract check: PASS: Node.js 26.8.1 accepted; 26.8.0 and 26.8.2 rejected actionably'
}

summary_commands() {
  task -d "$repository_root" --summary "$1" | sed -n '/^commands:$/,$p'
}

require_summary() {
  local target="$1"
  local expected="$2"
  local actual
  actual="$(summary_commands "$target")"
  [[ "$actual" == "$expected" ]] || {
    printf 'expected summary for %s:\n%s\nactual:\n%s\n' "$target" "$expected" "$actual" >&2
    fail "$target does not delegate through the required isolated command graph"
    return 1
  }
}

check_target_inventory() {
  local expected_count="$1"
  local inventory expected_inventory actual_count
  inventory="$(task -d "$repository_root" --list-all --json | node -e '
    let input = "";
    process.stdin.on("data", (chunk) => { input += chunk; });
    process.stdin.on("end", () => {
      const tasks = JSON.parse(input).tasks
        .map(({ name }) => name)
        .filter((name) => name.startsWith("frontend:"))
        .sort();
      process.stdout.write(tasks.join("\n"));
    });
  ')"
  expected_inventory="$(printf '%s\n' \
    frontend:build \
    frontend:build:client \
    frontend:build:overseer \
    frontend:boundary:check \
    frontend:compatibility:check \
    frontend:policy:check \
    frontend:reproducible:check \
    frontend:typecheck \
    frontend:typecheck:client \
    frontend:typecheck:overseer | LC_ALL=C sort)"
  actual_count="$(wc -l <<<"$inventory" | tr -d ' ')"
  [[ "$actual_count" == "$expected_count" ]] || { fail "expected $expected_count frontend targets; found $actual_count"; return 1; }
  [[ "$inventory" == "$expected_inventory" ]] || {
    printf 'expected frontend targets:\n%s\nactual:\n%s\n' "$expected_inventory" "$inventory" >&2
    fail 'frontend target inventory differs from the wave-a contract'
    return 1
  }
}

check_dispatch_contracts() {
  local temporary_root fake_npm log output status
  temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/fallout-frontend-task-contract.XXXXXX")"
  fake_npm="$temporary_root/npm"
  log="$temporary_root/npm.log"
  trap 'rm -rf "$temporary_root"' RETURN
  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'printf "%s\n" "$*" >>"$FRONTEND_TASK_LOG"' \
    'if [[ -n "${FRONTEND_FAIL_COMMAND:-}" && "$*" == "$FRONTEND_FAIL_COMMAND" ]]; then exit 23; fi' \
    >"$fake_npm"
  chmod +x "$fake_npm"

  local target expected
  while IFS='|' read -r target expected; do
    : >"$log"
    FRONTEND_TASK_LOG="$log" task -d "$repository_root" "$target" NPM="$fake_npm" >/dev/null
    [[ "$(<"$log")" == "$expected" ]] || { fail "$target crossed an application or install boundary"; return 1; }
  done <<'CASES'
frontend:typecheck:overseer|run typecheck:overseer --prefix frontend
frontend:typecheck:client|run typecheck:client --prefix frontend
frontend:build:overseer|run build:overseer --prefix frontend
frontend:build:client|run build:client --prefix frontend
CASES

  : >"$log"
  FRONTEND_TASK_LOG="$log" task -d "$repository_root" frontend:typecheck NPM="$fake_npm" >/dev/null
  [[ "$(<"$log")" == $'run typecheck:overseer --prefix frontend\nrun typecheck:client --prefix frontend' ]] || {
    fail 'aggregate typecheck dependency order or isolation is incorrect'
    return 1
  }

  : >"$log"
  set +e
  output="$(FRONTEND_TASK_LOG="$log" FRONTEND_FAIL_COMMAND='run build:overseer --prefix frontend' \
    task -d "$repository_root" frontend:build NPM="$fake_npm" 2>&1)"
  status=$?
  set -e
  ((status != 0)) || { fail 'aggregate build swallowed a focused build failure'; return 1; }
  [[ "$(<"$log")" == $'ci --prefix frontend\nrun build:overseer --prefix frontend' ]] || {
    printf '%s\n' "$output" >&2
    fail 'aggregate build did not install once, stop on failure, and preserve build order'
    return 1
  }

  rm -rf "$temporary_root"
  trap - RETURN
}

full_self_test() {
  local expected_target_count="$1"
  node_version_self_test
  check_target_inventory "$expected_target_count"
  require_summary frontend:typecheck $'commands:\n - Task: frontend:typecheck:overseer\n - Task: frontend:typecheck:client'
  require_summary frontend:build $'commands:\n - Task: deps:frontend\n - Task: frontend:build:overseer\n - Task: frontend:build:client'
  require_summary frontend:boundary:check $'commands:\n - npm test --prefix tests/browser -- frontend-boundary-manifest.spec.mjs'
  require_summary frontend:compatibility:check $'commands:\n - scripts/frontend-focused-browser-check.sh tests/browser/persistence-compatibility.spec.mjs '\''legacy/current session and player config round-trip through migrated boundary'\'''
  require_summary frontend:policy:check $'commands:\n - scripts/frontend-policy-check.sh'
  require_summary frontend:reproducible:check $'commands:\n - scripts/reproducible-build-check.sh --frontend'
  check_dispatch_contracts
  printf 'frontend task contract check self-test: PASS: exact %s-target inventory, isolation, install ownership, order, and failure propagation verified\n' \
    "$expected_target_count"
}

if [[ "${1:-}" == '--source-only' ]]; then
  return 0 2>/dev/null || exit 0
fi

case "${1:-}" in
  --node-version-self-test)
    [[ "$#" == 1 ]] || { fail 'usage: frontend-task-contract-check.sh --node-version-self-test'; exit 2; }
    node_version_self_test
    ;;
  --self-test)
    [[ "$#" == 3 && "${2:-}" == '--expected-target-count' && "${3:-}" =~ ^[0-9]+$ ]] || {
      fail 'usage: frontend-task-contract-check.sh --self-test --expected-target-count COUNT'
      exit 2
    }
    full_self_test "$3"
    ;;
  *)
    fail 'usage: frontend-task-contract-check.sh --node-version-self-test | --self-test --expected-target-count COUNT'
    exit 2
    ;;
esac
