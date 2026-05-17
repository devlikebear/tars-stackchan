#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
makefile="$repo_root/Makefile"
make_cmd="${MAKE:-make}"

fail() {
  echo "$1" >&2
  exit 1
}

assert_file() {
  [ -f "$1" ] || fail "missing file: $1"
}

assert_contains() {
  file="$1"
  pattern="$2"
  if ! grep -Fq -- "$pattern" "$file"; then
    fail "missing pattern in $file: $pattern"
  fi
}

assert_not_contains() {
  file="$1"
  pattern="$2"
  if grep -Fq -- "$pattern" "$file"; then
    fail "unexpected pattern in $file: $pattern"
  fi
}

assert_text_contains() {
  text="$1"
  pattern="$2"
  case "$text" in
    *"$pattern"*) ;;
    *) fail "help output missing: $pattern" ;;
  esac
}

required_targets="
help
fmt
lint
test
test-go
test-contracts
test-firmware
test-release
build
build-mcp
build-control
build-host
run-mcp
run-mcp-http
run-mcp-firmware
run-control
run-control-mock
host-serve
host-probe
doctor
doctor-http
discover-base-url
tts-serve
tts-status
perceive-serve
perceive-enroll
install-claude
config-claude
config-claude-desktop
config-tars
firmware-check
firmware-prepare
firmware-deps
firmware-host
firmware-mod
firmware-smoke
firmware-all
hardware-smoke
release-check
release-snapshot
clean
"

assert_file "$makefile"
assert_contains "$makefile" ".DEFAULT_GOAL := help"
assert_contains "$makefile" ".PHONY:"
assert_contains "$makefile" "scripts/test/firmware-bridge-contract.sh"
assert_contains "$makefile" "scripts/test/discover-base-url-contract.sh"
assert_contains "$makefile" "scripts/test/tts-relay-go-contract.sh"
assert_contains "$makefile" "scripts/test/hostbody-smoke-contract.sh"
assert_contains "$makefile" "scripts/dev/discover-base-url.sh"
assert_contains "$makefile" "scripts/dev/upload-firmware.sh"
assert_contains "$makefile" "TARS_STACKCHAN_AUTO_BASE_URL"
assert_contains "$makefile" "TARS_STACKCHAN_DISCOVER_RESET ?= 1"
assert_contains "$makefile" "RESOLVE_STACKCHAN_BASE_URL"
assert_contains "$makefile" "auto base URL discovery failed"
assert_contains "$makefile" "exit 1"
assert_not_contains "$makefile" 'using $$base_url'
assert_contains "$makefile" "export TARS_STACKCHAN_TOKEN"
assert_not_contains "$makefile" 'TARS_STACKCHAN_TOKEN="$(TARS_STACKCHAN_TOKEN)"'
assert_contains "$makefile" "release --snapshot --clean"
assert_contains "$makefile" "TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS=1"
assert_contains "$makefile" "tools/list"
assert_contains "$makefile" "tars-stackchan-host"

for target in $required_targets; do
  if ! "$make_cmd" -C "$repo_root" -n "$target" >/dev/null; then
    fail "make target is not runnable: $target"
  fi
done

help_output="$("$make_cmd" -C "$repo_root" help)"
assert_text_contains "$help_output" "MCP server"
assert_text_contains "$help_output" "Control UI"
assert_text_contains "$help_output" "Firmware"
assert_text_contains "$help_output" "Release"

echo "makefile-contract: OK"
