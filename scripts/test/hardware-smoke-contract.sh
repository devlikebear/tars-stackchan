#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
smoke_script="$repo_root/scripts/test/hardware-smoke.sh"

assert_contains() {
  file="$1"
  pattern="$2"
  if ! grep -Fq -- "$pattern" "$file"; then
    echo "missing pattern in $file: $pattern" >&2
    exit 1
  fi
}

if [ ! -x "$smoke_script" ]; then
  echo "missing executable hardware smoke script: $smoke_script" >&2
  exit 1
fi

assert_contains "$smoke_script" "TARS_STACKCHAN_SMOKE_CONNECT_TIMEOUT"
assert_contains "$smoke_script" "TARS_STACKCHAN_SMOKE_MAX_TIME"
assert_contains "$smoke_script" "--connect-timeout"
assert_contains "$smoke_script" "--max-time"
assert_contains "$smoke_script" "diagnose_reachability"
assert_contains "$smoke_script" "assert_contains_text"
assert_contains "$smoke_script" "expect_http_status"
assert_contains "$smoke_script" "invalid token"
assert_contains "$smoke_script" "module-name collision"
