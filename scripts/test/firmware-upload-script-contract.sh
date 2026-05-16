#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
upload_script="$repo_root/scripts/dev/upload-firmware.sh"
prepare_script="$repo_root/scripts/dev/prepare-firmware-upload.sh"
ready_script="$repo_root/scripts/dev/check-firmware-upload-ready.sh"

assert_contains() {
  file="$1"
  pattern="$2"
  if ! grep -Fq "$pattern" "$file"; then
    echo "missing pattern in $file: $pattern" >&2
    exit 1
  fi
}

if [ ! -x "$upload_script" ]; then
  echo "missing executable upload script: $upload_script" >&2
  exit 1
fi

assert_contains "$upload_script" "uv run --with esptool esptool"
assert_contains "$upload_script" "erase-region"
assert_contains "$upload_script" "write-flash"
assert_contains "$upload_script" "TARS_STACKCHAN_MOD_OFFSET"
assert_contains "$upload_script" "TARS_STACKCHAN_MOD_SIZE"
assert_contains "$upload_script" "TARS_STACKCHAN_DEPLOY_HOST"
assert_contains "$upload_script" "scripts/test/hardware-smoke.sh"

assert_contains "$prepare_script" "scripts/dev/upload-firmware.sh"
assert_contains "$ready_script" "scripts/dev/upload-firmware.sh"
