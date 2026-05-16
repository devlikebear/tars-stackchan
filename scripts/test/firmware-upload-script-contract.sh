#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
upload_script="$repo_root/scripts/dev/upload-firmware.sh"
prepare_script="$repo_root/scripts/dev/prepare-firmware-upload.sh"
ready_script="$repo_root/scripts/dev/check-firmware-upload-ready.sh"
tts_server="$repo_root/scripts/dev/tts-remote-server.py"
tts_runner="$repo_root/scripts/dev/run-local-tts.sh"
tts_contract="$repo_root/scripts/test/tts-remote-server-contract.sh"

assert_contains() {
  file="$1"
  pattern="$2"
  if ! grep -Fq -- "$pattern" "$file"; then
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
assert_contains "$upload_script" "TARS_STACKCHAN_SMOKE_RETRY_USB_RESET"
assert_contains "$upload_script" "reset_device_for_smoke_retry"
assert_contains "$upload_script" "scripts/test/hardware-smoke.sh"
assert_contains "$upload_script" "verify_prepared_mod_contract"
assert_contains "$upload_script" "tars-http-server-service"
assert_contains "$upload_script" "host firmware module collision"
assert_contains "$upload_script" "must not import the host headers module"

assert_contains "$prepare_script" "scripts/dev/upload-firmware.sh"
assert_contains "$prepare_script" "manifest_m5stackchan_cores3.json"
assert_contains "$prepare_script" "type: 'm5stackchan'"
assert_contains "$prepare_script" "led.head"
assert_contains "$prepare_script" "servoPower"
assert_contains "$prepare_script" "TARS_STACKCHAN_TTS_HOST"
assert_contains "$prepare_script" "TARS_STACKCHAN_TTS_VOLUME:-0.15"
assert_contains "$prepare_script" "type: 'remote'"
assert_contains "$prepare_script" "volume: Number(ttsVolume)"
assert_contains "$prepare_script" "speechPathPrefix"
assert_contains "$prepare_script" "encodeURIComponent(token)"
assert_contains "$ready_script" "scripts/dev/upload-firmware.sh"
assert_contains "$ready_script" "firmware bridge token and speech TTS token are configured"
assert_contains "$ready_script" "host manifest is patched for CoreS3/K151 servo, head LED, and remote TTS"
assert_contains "$ready_script" "tts.volume <= 0.2"
assert_contains "$tts_server" "gemini-3.1-flash-tts-preview"
assert_contains "$tts_server" "x-goog-api-key"
assert_contains "$tts_server" "TARS_STACKCHAN_TTS_TOKEN"
assert_contains "$tts_server" "hmac.compare_digest"
assert_contains "$tts_server" "redact_path"
assert_contains "$tts_server" "responseModalities"
assert_contains "$tts_server" "prebuiltVoiceConfig"
assert_contains "$tts_server" "wave.open"
assert_contains "$tts_runner" "TARS_STACKCHAN_TTS_TOKEN"
assert_contains "$tts_runner" "TARS_STACKCHAN_TOKEN"
assert_contains "$tts_runner" "gemini-3.1-flash-tts-preview"
assert_contains "$tts_runner" "TARS_STACKCHAN_TTS_VOICE:-Kore"
assert_contains "$tts_runner" "exec env TARS_STACKCHAN_TTS_TOKEN="
assert_contains "$tts_contract" "uv run python"
