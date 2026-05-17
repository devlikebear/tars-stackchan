#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
upload_script="$repo_root/scripts/dev/upload-firmware.sh"
prepare_script="$repo_root/scripts/dev/prepare-firmware-upload.sh"
ready_script="$repo_root/scripts/dev/check-firmware-upload-ready.sh"

assert_contains() {
  file="$1"
  pattern="$2"
  if ! grep -Fq -- "$pattern" "$file"; then
    echo "missing pattern in $file: $pattern" >&2
    exit 1
  fi
}

assert_not_contains() {
  file="$1"
  pattern="$2"
  if grep -Fq -- "$pattern" "$file"; then
    echo "unexpected pattern in $file: $pattern" >&2
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
# Embodied Bot camera S2: the CoreS3 host manifest must use the
# project-vendored esp_video/V4L2 camera overlay, matching M5Stack StackChan
# and Espressif m5stack_core_s3 BSP. This avoids the real-hardware reset seen
# in the Moddable esp32-camera path.
assert_contains "$prepare_script" "host_overlay_dir="
assert_contains "$prepare_script" 'cp -R "$host_overlay_dir" "$target_dir/firmware/stackchan/tars-imagein-camera"'
assert_contains "$prepare_script" "sdkconfig-combined"
assert_contains "$prepare_script" "./tars-imagein-camera/manifest.json"
assert_contains "$prepare_script" '$(MODDABLE)/modules/io/audioin/manifest.json'
assert_contains "$prepare_script" "manifest.config.camera = {"
assert_contains "$prepare_script" "frameSize: 'QVGA'"
assert_not_contains "$prepare_script" "manifest.build.SDKCONFIGPATH = './tars-imagein-camera/sdkconfig'"
assert_contains "$prepare_script" "xclk: -1"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/manifest.json" '"SDKCONFIGPATH": "./sdkconfig-combined"'
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/manifest.json" '"esp_video"'
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/manifest.json" '"esp32-camera"'
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/manifest.json" '"driver/include"'
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/manifest.json" '"esp_jpeg"'
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/manifest.json" '"*": "$(MODDABLE)/modules/io/common/builtinCommon"'
assert_not_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/manifest.json" '$(MODDABLE)/modules/io/imagein/camera/manifest.json'
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/sdkconfig/sdkconfig.defaults" "CONFIG_ESP_VIDEO_ENABLE_DVP_VIDEO_DEVICE=y"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/sdkconfig/sdkconfig.defaults" "CONFIG_ESP_VIDEO_ENABLE_SPI_VIDEO_DEVICE=n"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/sdkconfig/sdkconfig.defaults" "CONFIG_ESP_VIDEO_ENABLE_USB_UVC_VIDEO_DEVICE=n"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/sdkconfig/sdkconfig.defaults" "CONFIG_CAMERA_GC0308_DVP_RGB565_BE_320X240_20FPS=y"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/camera.c" "esp_video_init"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/camera.c" "esp_video_init_with_flags"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/camera.c" "DVP-only layout"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/camera.c" "esp_video_init_sccb_config_t"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/camera.c" "VIDIOC_DQBUF"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/camera.c" "image_to_jpeg"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/camera.c" "camera init failed at"
assert_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/camera.c" "falling back to current V4L2 format"
assert_not_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/camera.c" "esp_camera_init"
assert_not_contains "$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3/camera.c" "tarsReleaseSharedI2C"
assert_contains "$prepare_script" "manifest.config.audioIn = {"
assert_contains "$prepare_script" "sampleRate: 16000"
assert_contains "$prepare_script" "type: 'm5stackchan'"
assert_contains "$prepare_script" "led.head"
assert_contains "$prepare_script" "servoPower"
assert_contains "$prepare_script" "TARS_STACKCHAN_TTS_HOST"
# Phase 2: default to the mDNS hostname (not a DHCP-volatile IP); the env
# override must still win. Keep this literal in sync with
# tts.DefaultTTSHostname (mcp-server/internal/tts/mdns.go).
assert_contains "$prepare_script" 'tts_host="tars-stackchan-tts.local"'
assert_contains "$prepare_script" 'tts_host="${TARS_STACKCHAN_TTS_HOST:-}"'
assert_contains "$prepare_script" "is an mDNS name; the relay must advertise it"
# The host manifest base must be restored after the upstream checkout so the
# host build + TTS host bake are deterministic (regression: a clean prepare
# left it deleted and the host firmware build failed).
assert_contains "$prepare_script" "checkout -- firmware/stackchan/manifest_local.json"
assert_contains "$prepare_script" "TARS_STACKCHAN_TTS_VOLUME:-0.15"
assert_contains "$prepare_script" "type: 'remote'"
assert_contains "$prepare_script" "volume: Number(ttsVolume)"
assert_contains "$prepare_script" "speechPathPrefix"
assert_contains "$prepare_script" "encodeURIComponent(token)"
assert_contains "$ready_script" "scripts/dev/upload-firmware.sh"
assert_contains "$ready_script" "firmware bridge token and speech TTS token are configured"
assert_contains "$ready_script" "host manifest is patched for CoreS3/K151 servo, head LED, and remote TTS"
assert_contains "$ready_script" "tts.volume <= 0.2"
# The Gemini TTS relay is now the Go binary; its behavior is covered by
# scripts/test/tts-relay-go-contract.sh and mcp-server/internal/tts tests.
# The Python relay (tts-remote-server.py / run-local-tts.sh) was removed in
# Phase 4 of the dynamic TTS discovery epic.
