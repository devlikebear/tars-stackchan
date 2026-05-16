#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
target_dir="${TARS_STACKCHAN_UPSTREAM_DIR:-$repo_root/.work/stack-chan}"
firmware_dir="$target_dir/firmware"
target="${TARS_STACKCHAN_TARGET:-esp32/m5stack_cores3}"
mode="${1:-all}"
bridge_manifest="mods/tars_stackchan_bridge/manifest.json"
prepared_manifest="$firmware_dir/$bridge_manifest"
mod_name="tars_stackchan_bridge"
mod_offset="${TARS_STACKCHAN_MOD_OFFSET:-0xfa0000}"
mod_size="${TARS_STACKCHAN_MOD_SIZE:-0x40000}"
chip="${TARS_STACKCHAN_CHIP:-esp32s3}"
baud="${TARS_STACKCHAN_BAUD:-460800}"
skip_setup="${TARS_STACKCHAN_SKIP_SETUP:-0}"
force_setup="${TARS_STACKCHAN_FORCE_SETUP:-0}"
deploy_host="${TARS_STACKCHAN_DEPLOY_HOST:-0}"
skip_smoke="${TARS_STACKCHAN_SKIP_SMOKE:-0}"
base_url="${TARS_STACKCHAN_BASE_URL:-http://stackchan.local}"
upload_port="${TARS_STACKCHAN_UPLOAD_PORT:-}"

usage() {
  cat <<EOF
Usage: scripts/dev/upload-firmware.sh [all|prepare|deps|host|mod|smoke]

Modes:
  all      prepare checkout, install deps, optionally deploy host, direct-flash MOD, run smoke
  prepare  prepare the pinned upstream checkout and copy the bridge MOD
  deps     prepare checkout and install/setup upstream firmware dependencies
  host     deploy the upstream Stack-chan host firmware
  mod      build the bridge MOD and flash it directly to the ESP32 xs partition
  smoke    run scripts/test/hardware-smoke.sh

Important environment:
  TARS_STACKCHAN_TOKEN       local bearer token patched into the MOD manifest
  TARS_STACKCHAN_UPLOAD_PORT USB serial device, defaults to first /dev/cu.usbmodem*
  TARS_STACKCHAN_BASE_URL    hardware smoke URL, defaults to http://stackchan.local
  TARS_STACKCHAN_DEPLOY_HOST set to 1 for mode=all to also deploy host firmware
  TARS_STACKCHAN_SKIP_SETUP  set to 1 to skip upstream npm run setup
  TARS_STACKCHAN_FORCE_SETUP set to 1 to rerun upstream setup even when mcconfig exists
  TARS_STACKCHAN_MOD_OFFSET  ESP32 xs MOD partition offset, defaults to 0xfa0000
  TARS_STACKCHAN_MOD_SIZE    ESP32 xs MOD partition size, defaults to 0x40000
EOF
}

die() {
  echo "error: $1" >&2
  exit 1
}

log() {
  echo "==> $1"
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

read_prepared_token() {
  if [ -f "$prepared_manifest" ] && command -v node >/dev/null 2>&1; then
    node -e "const m=require(process.argv[1]); const t=m.config?.tarsStackchan?.token || ''; process.stdout.write(t === 'replace-with-local-token' ? '' : t)" "$prepared_manifest"
  fi
}

ensure_token() {
  if [ -z "${TARS_STACKCHAN_TOKEN:-}" ]; then
    prepared_token="$(read_prepared_token || true)"
    if [ -n "$prepared_token" ]; then
      export TARS_STACKCHAN_TOKEN="$prepared_token"
    fi
  fi
  [ -n "${TARS_STACKCHAN_TOKEN:-}" ] || die "TARS_STACKCHAN_TOKEN is required"
}

ensure_upload_port() {
  if [ -n "$upload_port" ]; then
    [ -c "$upload_port" ] || die "TARS_STACKCHAN_UPLOAD_PORT is not a character device: $upload_port"
    return
  fi

  for candidate in /dev/cu.usbmodem* /dev/cu.usbserial* /dev/cu.SLAB_USBtoUART*; do
    if [ -c "$candidate" ]; then
      upload_port="$candidate"
      return
    fi
  done

  die "no USB serial device found; connect Stack-chan or set TARS_STACKCHAN_UPLOAD_PORT"
}

setup_toolchain_env() {
  export MODDABLE="${MODDABLE:-$HOME/.local/share/moddable}"
  export PATH="$MODDABLE/build/bin/mac/release:$PATH"

  idf_path="${IDF_PATH:-$HOME/.local/share/esp32/esp-idf}"
  if [ -f "$idf_path/export.sh" ]; then
    # Keep ESP-IDF's Python environment scoped to this shell; esptool flash uses uv below.
    . "$idf_path/export.sh" >/tmp/tars-stackchan-idf-export.log
  fi
}

run_prepare() {
  ensure_token
  log "Preparing pinned upstream firmware checkout"
  "$repo_root/scripts/dev/prepare-firmware-upload.sh"
}

install_deps() {
  require_cmd npm
  [ -d "$firmware_dir" ] || die "missing firmware checkout; run prepare first"

  log "Installing upstream npm dependencies when needed"
  if [ ! -d "$firmware_dir/node_modules" ]; then
    (cd "$firmware_dir" && npm install)
  fi

  mcconfig_path="${MODDABLE:-$HOME/.local/share/moddable}/build/bin/mac/release/mcconfig"
  if [ "$skip_setup" = "1" ]; then
    log "Skipping upstream setup because TARS_STACKCHAN_SKIP_SETUP=1"
  elif [ "$force_setup" = "1" ] || [ ! -x "$mcconfig_path" ]; then
    log "Running upstream Moddable/ESP32 setup"
    (cd "$firmware_dir" && npm run setup && npm run setup -- --device=esp32)
  else
    log "Skipping upstream setup because mcconfig already exists at $mcconfig_path"
  fi
}

deploy_host_firmware() {
  setup_toolchain_env
  ensure_upload_port
  log "Deploying Stack-chan host firmware to $upload_port"
  (cd "$firmware_dir" && npm_config_target="$target" npm run build && npm_config_target="$target" npm run deploy)
}

build_mod() {
  setup_toolchain_env
  [ -f "$prepared_manifest" ] || die "missing prepared MOD manifest; run prepare first"
  log "Building bridge MOD archive"
  (cd "$firmware_dir" && mcrun -dl -m -p "$target" -t build "$bridge_manifest")
}

run_esptool() {
  require_cmd uv
  uv run --with esptool esptool "$@"
}

flash_mod_direct() {
  setup_toolchain_env
  ensure_upload_port
  xsa="$MODDABLE/build/bin/esp32/debug/$mod_name/$mod_name.xsa"
  [ -f "$xsa" ] || die "missing MOD archive: $xsa"

  log "Direct-flashing bridge MOD archive to xs partition"
  echo "    port:   $upload_port"
  echo "    chip:   $chip"
  echo "    offset: $mod_offset"
  echo "    size:   $mod_size"
  run_esptool --chip "$chip" --port "$upload_port" erase-region "$mod_offset" "$mod_size"
  run_esptool --chip "$chip" --port "$upload_port" --baud "$baud" write-flash "$mod_offset" "$xsa"
}

run_smoke() {
  ensure_token
  log "Running hardware smoke against $base_url"
  TARS_STACKCHAN_BASE_URL="$base_url" \
    TARS_STACKCHAN_TOKEN="$TARS_STACKCHAN_TOKEN" \
    "$repo_root/scripts/test/hardware-smoke.sh"
}

case "$mode" in
  -h|--help|help)
    usage
    ;;
  prepare)
    run_prepare
    ;;
  deps)
    run_prepare
    install_deps
    ;;
  host)
    run_prepare
    install_deps
    deploy_host_firmware
    ;;
  mod)
    run_prepare
    install_deps
    build_mod
    flash_mod_direct
    ;;
  smoke)
    run_smoke
    ;;
  all)
    run_prepare
    install_deps
    if [ "$deploy_host" = "1" ]; then
      deploy_host_firmware
    else
      log "Skipping host deploy; set TARS_STACKCHAN_DEPLOY_HOST=1 to include it"
    fi
    build_mod
    flash_mod_direct
    if [ "$skip_smoke" = "1" ]; then
      log "Skipping smoke because TARS_STACKCHAN_SKIP_SMOKE=1"
    else
      run_smoke
    fi
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
