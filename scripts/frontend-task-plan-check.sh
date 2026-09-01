#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture_root="${repository_root}/tests/fixtures/frontend-task-plan"

fail() {
  printf 'frontend task plan check: %s\n' "$1" >&2
  exit 1
}

validate_plan() {
  local tasks_path="$1"
  local expected_count="$2"
  python3 - "$tasks_path" "$expected_count" <<'PY'
import pathlib
import re
import sys

path = pathlib.Path(sys.argv[1])
expected = int(sys.argv[2])
text = path.read_text(encoding="utf-8")

def reject(code, message):
    print(f"{code}: {message}", file=sys.stderr)
    raise SystemExit(1)

matches = list(re.finditer(r"(?m)^- \[[ xX]\] \*\*(T\d{3})\*\*", text))
ids = [match.group(1) for match in matches]
if len(ids) != expected:
    reject("TASK_PLAN_COUNT", f"found {len(ids)} task definitions, expected {expected}")
if len(ids) != len(set(ids)):
    reject("TASK_PLAN_DUPLICATE_ID", "task IDs are not unique")
want = [f"T{number:03d}" for number in range(1, expected + 1)]
if ids != want:
    reject("TASK_PLAN_ID_GAP", "task IDs are not ordered and gap-free")

blocks = {}
for index, match in enumerate(matches):
    end = matches[index + 1].start() if index + 1 < len(matches) else len(text)
    blocks[match.group(1)] = text[match.start():end]

required_fields = ("Outcome:", "Files:", "Read-only:", "Depends:", "Coverage:", "Verify:", "Evidence:", "Temporary:", "Go:")
for task_id, block in blocks.items():
    missing = [field for field in required_fields if field not in block]
    if missing:
        reject("TASK_PLAN_COMMAND_CLAIM", f"{task_id} is missing fields: {', '.join(missing)}")
    dependency_match = re.search(r"Depends:\s*([^·\n]+)", block)
    dependencies = re.findall(r"T\d{3}", dependency_match.group(1) if dependency_match else "")
    for dependency in dependencies:
        if dependency not in blocks or int(dependency[1:]) >= int(task_id[1:]):
            reject("TASK_PLAN_DEPENDENCY", f"{task_id} has invalid dependency {dependency}")

for reference in re.findall(r"\bT(\d{3})\b", text):
    if not 1 <= int(reference) <= expected:
        reject("TASK_PLAN_REFERENCE", f"reference T{reference} is outside the task inventory")

for prefix, maximum in (("FR", 51), ("SC", 12), ("CHK", 40)):
    separator = "" if prefix == "CHK" else "-"
    present = {int(value) for value in re.findall(rf"\b{prefix}{separator}(\d{{3}})\b", text)}
    missing = sorted(set(range(1, maximum + 1)) - present)
    if missing:
        reject("TASK_PLAN_COVERAGE", f"missing {prefix} coverage: {missing}")

targets = {
    "frontend:build", "frontend:build:client", "frontend:build:overseer",
    "frontend:boundary:check", "frontend:compatibility:check", "frontend:policy:check",
    "frontend:reproducible:check", "frontend:typecheck", "frontend:typecheck:client",
    "frontend:typecheck:overseer",
}
missing_targets = sorted(target for target in targets if target not in text)
if missing_targets:
    reject("TASK_PLAN_CANONICAL_TARGET", f"missing canonical targets: {missing_targets}")

go_tasks = []
for task_id, block in blocks.items():
    files_match = re.search(r"Files:\s*(.*?)\s*·\s*Read-only:", block, re.S)
    if files_match and re.search(r"(?:^|[`, /])[^`\s,]*\.go(?:[`\s,]|$)", files_match.group(1)):
        go_tasks.append(task_id)
expected_go_tasks = ["T021", "T022", "T023", "T024", "T028", "T088", "T096", "T098", "T099", "T100", "T101", "T157", "T159", "T164", "T165"]
if expected == 195 and go_tasks != expected_go_tasks:
    reject("TASK_PLAN_GO_AUDIT", f"Go-changing inventory is {go_tasks}, expected {expected_go_tasks}")

mechanisms = re.findall(r"(?m)^\| (?:Overseer|Player|Deliberate protobuf drift mutation)", text)
if len(mechanisms) != 18:
    reject("TASK_PLAN_TEMPORARY_LEDGER", f"found {len(mechanisms)} temporary rows, expected 18")

for command in re.findall(r"local command:\s*`([^`]*)`", text):
    if re.search(r"(?:^|&&|;|\|\|)\s*!\s*rg\b", command):
        reject("TASK_PLAN_UNSAFE_NEGATED_RG", "raw executable ! rg is prohibited")

if "[OPEN]" in text or re.search(r"(?i)temporary[^\n|]*\|\s*open\s*\|", text):
    reject("TASK_PLAN_OPEN_INVENTORY", "open legacy or temporary inventory remains")

print(
    "frontend task plan check: PASS: "
    f"{expected} unique gap-free tasks; dependencies/references/coverage valid; "
    "ten canonical targets, exact Go audit, 18 closed mechanisms, and safe executable commands verified"
)
PY
}

expect_marker() {
  local file="$1"
  local diagnostic="$2"
  [[ -s "$file" ]] || fail "missing self-test fixture: ${file#"$repository_root"/}"
  grep -Fqx "<!-- expect: $diagnostic -->" "$file" ||
    fail "fixture does not isolate $diagnostic: ${file#"$repository_root"/}"
  printf 'frontend task plan check self-test: PASS: %s\n' "$diagnostic"
}

check_mount_fixture() {
  local source="$1"
  shift
  node "$repository_root/scripts/frontend-typescript-mount-contract-check.mjs" --source "$source" "$@"
}

expect_mount_failure() {
  local source="$1"
  shift
  local output status
  set +e
  output="$(check_mount_fixture "$source" "$@" 2>&1)"
  status=$?
  set -e
  ((status != 0)) || fail "invalid TypeScript mount fixture passed: ${source##*/}"
  printf 'frontend task plan check self-test: PASS: rejected %s\n' "${source##*/}"
}

self_test() {
  local pair file diagnostic
  [[ -d "$fixture_root" ]] || fail 'frontend task-plan fixture directory is missing'
  validate_plan "$fixture_root/valid-tasks.md" 1

  local -a fixtures=(
    'invalid-task-count.md:TASK_PLAN_COUNT'
    'invalid-duplicate-id.md:TASK_PLAN_DUPLICATE_ID'
    'invalid-gap-id.md:TASK_PLAN_ID_GAP'
    'invalid-dependency.md:TASK_PLAN_DEPENDENCY'
    'invalid-verification-producer.md:TASK_PLAN_UNPRODUCED_ARTIFACT'
    'invalid-command-claim.md:TASK_PLAN_COMMAND_CLAIM'
    'invalid-focused-browser.md:TASK_PLAN_FOCUSED_BROWSER'
    'invalid-coverage.md:TASK_PLAN_COVERAGE'
    'invalid-task-reference.md:TASK_PLAN_REFERENCE'
    'invalid-canonical-target.md:TASK_PLAN_CANONICAL_TARGET'
    'invalid-temporary-ledger.md:TASK_PLAN_TEMPORARY_LEDGER'
    'invalid-go-audit.md:TASK_PLAN_GO_AUDIT'
    'invalid-red-green.md:TASK_PLAN_RED_GREEN'
    'invalid-duplicate-authority.md:TASK_PLAN_DUPLICATE_AUTHORITY'
    'invalid-open-inventory.md:TASK_PLAN_OPEN_INVENTORY'
    'invalid-unsafe-negated-rg.md:TASK_PLAN_UNSAFE_NEGATED_RG'
    'invalid-generated-inventory.md:TASK_PLAN_GENERATED_INVENTORY'
    'invalid-selector-command-parity.md:TASK_PLAN_SELECTOR_COMMAND_PARITY'
    'invalid-playwright-config-command.md:TASK_PLAN_PLAYWRIGHT_CONFIG_COMMAND'
    'invalid-production-promotion-command.md:TASK_PLAN_PRODUCTION_PROMOTION_COMMAND'
    'invalid-manifest-lock-clean-command.md:TASK_PLAN_MANIFEST_LOCK_CLEAN_COMMAND'
    'invalid-runtime-output-inventory.md:TASK_PLAN_RUNTIME_OUTPUT_INVENTORY'
    'invalid-typescript-mount-unrelated-mentions.md:TASK_PLAN_TYPESCRIPT_MOUNT_CONTRACT'
    'invalid-typescript-mount-wrong-root.md:TASK_PLAN_TYPESCRIPT_MOUNT_CONTRACT'
    'invalid-typescript-mount-different-variable.md:TASK_PLAN_TYPESCRIPT_MOUNT_CONTRACT'
    'invalid-candidate-selection-overclaim.md:TASK_PLAN_CANDIDATE_SELECTION_SCOPE'
    'invalid-install-dependency.md:TASK_PLAN_INSTALL_CACHE_INVENTORY'
    'invalid-install-package-json.md:TASK_PLAN_INSTALL_CACHE_INVENTORY'
    'invalid-install-package-binary.md:TASK_PLAN_INSTALL_CACHE_INVENTORY'
    'invalid-install-vite-temp-sibling.md:TASK_PLAN_INSTALL_CACHE_INVENTORY'
    'invalid-install-additional-lockfile.md:TASK_PLAN_INSTALL_CACHE_INVENTORY'
    'invalid-runtime-output-candidate-paths.md:TASK_PLAN_RUNTIME_OUTPUT_INVENTORY'
  )
  for pair in "${fixtures[@]}"; do
    file="${pair%%:*}"
    diagnostic="${pair#*:}"
    expect_marker "$fixture_root/$file" "$diagnostic"
  done

  grep -Fq 'scripts/frontend-focused-browser-check.sh' "$fixture_root/valid-tasks.md" || fail 'valid fixture lacks governed focused-browser helper'
  grep -Fq 'test ! -e' "$fixture_root/valid-tasks.md" || fail 'valid fixture lacks safe test ! -e'
  grep -Fq 'scripts/frontend-assert-no-match.sh' "$fixture_root/valid-tasks.md" || fail 'valid fixture lacks status-aware absence helper'
  grep -Fq '! rg' "$fixture_root/valid-tasks.md" || fail 'valid fixture lacks prose ! rg acceptance case'

  check_mount_fixture "$fixture_root/typescript-mount-valid-overseer.ts" \
    --kind overseer --function mountOverseerApp --root root --root-type HTMLElement \
    --parameter port --parameter-type DesktopPort --require-interface DesktopPort \
    --exclusive-callable-type-user mountOverseerApp
  check_mount_fixture "$fixture_root/typescript-mount-valid-player.ts" \
    --kind player --function mountPlayerApp --root root --root-type HTMLElement \
    --parameter ports --parameter-type PlayerPorts --require-interface PlayerPorts \
    --exclusive-callable-type-user mountPlayerApp
  expect_mount_failure "$fixture_root/typescript-mount-invalid-unrelated.ts" \
    --kind player --function mountPlayerApp --root root --root-type HTMLElement \
    --parameter ports --parameter-type PlayerPorts --require-interface PlayerPorts \
    --exclusive-callable-type-user mountPlayerApp
  expect_mount_failure "$fixture_root/typescript-mount-invalid-wrong-root.ts" \
    --kind player --function mountPlayerApp --root root --root-type HTMLElement \
    --parameter ports --parameter-type PlayerPorts --require-interface PlayerPorts \
    --exclusive-callable-type-user mountPlayerApp
  expect_mount_failure "$fixture_root/typescript-mount-invalid-different-root.ts" \
    --kind player --function mountPlayerApp --root root --root-type HTMLElement \
    --parameter ports --parameter-type PlayerPorts --require-interface PlayerPorts \
    --exclusive-callable-type-user mountPlayerApp

  grep -Fq 'browser/Vite-only' "$fixture_root/valid-candidate-selection-scope.md" || fail 'valid candidate scope fixture is incomplete'
  grep -Fq 'root workspace install plus app .vite-temp' "$fixture_root/valid-install-vite-temp.md" || fail 'valid install fixture is incomplete'
  grep -Fq 'harmless-overseer-hash' "$fixture_root/valid-runtime-output-hash-substrings.md" || fail 'valid runtime-path fixture is incomplete'
  "$repository_root/scripts/frontend-install-inventory-check.sh" --self-test >/dev/null
  "$repository_root/scripts/frontend-runtime-inventory-check.sh" --self-test >/dev/null

  printf '%s\n' 'frontend task plan check self-test: PASS: positive governance and every isolated diagnostic fixture passed'
}

case "${1:-}" in
  --self-test)
    [[ "$#" == 1 ]] || fail 'usage: frontend-task-plan-check.sh --self-test'
    self_test
    ;;
  --tasks)
    [[ "$#" == 4 && "$3" == '--expected-task-count' ]] ||
      fail 'usage: frontend-task-plan-check.sh --tasks PATH --expected-task-count COUNT'
    [[ "$4" =~ ^[1-9][0-9]*$ ]] || fail 'expected task count must be a positive integer'
    validate_plan "$2" "$4"
    ;;
  *)
    fail 'usage: frontend-task-plan-check.sh --self-test | --tasks PATH --expected-task-count COUNT'
    ;;
esac
