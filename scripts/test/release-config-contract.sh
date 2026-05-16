#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
ci_workflow="$repo_root/.github/workflows/ci.yml"
release_workflow="$repo_root/.github/workflows/release.yml"
goreleaser_config="$repo_root/.goreleaser.yaml"

assert_file() {
  if [ ! -f "$1" ]; then
    echo "missing file: $1" >&2
    exit 1
  fi
}

assert_contains() {
  file="$1"
  pattern="$2"
  if ! grep -Fq -- "$pattern" "$file"; then
    echo "missing pattern in $file: $pattern" >&2
    exit 1
  fi
}

assert_file "$ci_workflow"
assert_file "$release_workflow"
assert_file "$goreleaser_config"

assert_contains "$ci_workflow" "go test ./..."
assert_contains "$ci_workflow" "scripts/test/firmware-upload-script-contract.sh"
assert_contains "$ci_workflow" "scripts/test/release-config-contract.sh"

assert_contains "$release_workflow" "goreleaser/goreleaser-action"
assert_contains "$release_workflow" "TAP_GITHUB_TOKEN"
assert_contains "$release_workflow" "GITHUB_TOKEN"

assert_contains "$goreleaser_config" "tars-stackchan-mcp"
assert_contains "$goreleaser_config" "tars-stackchan-control"
assert_contains "$goreleaser_config" "devlikebear"
assert_contains "$goreleaser_config" "homebrew-tap"
assert_contains "$goreleaser_config" "TARS_STACKCHAN_REPO_ROOT"
assert_contains "$goreleaser_config" "stackchan_upload_firmware"
