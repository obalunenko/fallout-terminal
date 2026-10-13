#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_root}"
export BUF_CACHE_DIR="${BUF_CACHE_DIR:-${TMPDIR:-/tmp}/fallout-terminal-buf-cache}"

sync_revision=false
case "${1-}" in
  '') ;;
  --sync-revision) sync_revision=true ;;
  *)
    printf 'usage: scripts/proto-generate.sh [--sync-revision]\n' >&2
    exit 1
    ;;
esac
if [[ "$#" -gt 1 ]]; then
  printf 'usage: scripts/proto-generate.sh [--sync-revision]\n' >&2
  exit 1
fi

schema_revision() {
  find proto/fallout/terminal -type f -name '*.proto' -print \
    | LC_ALL=C sort \
    | while IFS= read -r source; do shasum -a 256 "${source}"; done \
    | shasum -a 256 \
    | awk '{print $1}'
}

root_module_revision() {
  shasum -a 256 go.mod go.sum | shasum -a 256 | awk '{print $1}'
}

require_version() {
  local description="$1"
  local actual="$2"
  local expected="$3"
  if [[ "${actual}" != "${expected}" ]]; then
    printf '%s version is %s; expected %s\n' "${description}" "${actual}" "${expected}" >&2
    exit 1
  fi
}

root_module_before="$(root_module_revision)"
require_version "Buf" "$(go tool -modfile=tools/buf/go.mod buf --version)" "1.73.0"
require_version "protoc-gen-go" "$(go tool -modfile=tools/protoc-gen-go/go.mod protoc-gen-go --version)" "protoc-gen-go v1.36.12"
require_version "protoc-gen-connect-go" "$(go tool -modfile=tools/protoc-gen-connect-go/go.mod protoc-gen-connect-go --version)" "1.21.0"
require_version "protoc-gen-es" "$(node -p "require('./frontend/node_modules/@bufbuild/protoc-gen-es/package.json').version")" "2.16.0"

# Node 22+ exposes experimental Web Storage globals even when no persistence
# file is configured. TypeScript VFS probes localStorage while protoc-gen-es is
# loading, which otherwise emits a warning despite generation succeeding.
node_major="$(node -p "process.versions.node.split('.')[0]")"
if (( node_major >= 22 )) && [[ " ${NODE_OPTIONS:-} " != *" --no-experimental-webstorage "* ]]; then
  export NODE_OPTIONS="${NODE_OPTIONS:+${NODE_OPTIONS} }--no-experimental-webstorage"
fi

actual_revision="$(schema_revision)"
if [[ "${sync_revision}" == true ]]; then
  printf '%s\n' "${actual_revision}" > proto/schema-revision.txt
fi
expected_revision="$(tr -d '[:space:]' < proto/schema-revision.txt)"
if [[ "${actual_revision}" != "${expected_revision}" ]]; then
  printf 'schema revision is %s; expected %s\nUpdate proto/schema-revision.txt with reviewed schema changes.\n' "${actual_revision}" "${expected_revision}" >&2
  exit 1
fi

if [[ "$(grep -Ec '^    out: internal/gen$' proto/buf.gen.go.yaml)" -ne 2 ]]; then
  printf 'Go generation outputs must remain isolated under internal/gen\n' >&2
  exit 1
fi
if [[ "$(grep -Ec '^    out: frontend/client/gen$' proto/buf.gen.es.yaml)" -ne 1 ]]; then
  printf 'ECMAScript generation output must remain isolated under frontend/client/gen\n' >&2
  exit 1
fi

go tool -modfile=tools/buf/go.mod buf generate --template proto/buf.gen.go.yaml
go tool -modfile=tools/buf/go.mod buf generate --template proto/buf.gen.es.yaml

if [[ "$(schema_revision)" != "${expected_revision}" ]]; then
  printf 'generation modified schema inputs\n' >&2
  exit 1
fi
if [[ "$(root_module_revision)" != "${root_module_before}" ]]; then
  printf 'isolated protobuf generation modified root go.mod or go.sum\n' >&2
  exit 1
fi

printf 'Generated Go and ECMAScript contracts from schema revision %s\n' "${expected_revision}"
