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

if [[ "${1:-}" == '--source-only' ]]; then
  return 0 2>/dev/null || exit 0
fi

case "${1:-}" in
  --node-version-self-test)
    [[ "$#" == 1 ]] || { fail 'usage: frontend-task-contract-check.sh --node-version-self-test'; exit 2; }
    node_version_self_test
    ;;
  *)
    fail 'usage: frontend-task-contract-check.sh --node-version-self-test'
    exit 2
    ;;
esac
