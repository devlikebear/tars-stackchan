#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
target_dir="${TARS_STACKCHAN_UPSTREAM_DIR:-$repo_root/.work/stack-chan}"
firmware_dir="$target_dir/firmware"
manifest="$firmware_dir/mods/tars_stackchan_bridge/manifest.json"
host_manifest="$firmware_dir/stackchan/manifest_local.json"
moddable_dir="${MODDABLE:-$HOME/.local/share/moddable}"
ready=true

fail() {
  echo "not ready: $1" >&2
  ready=false
}

pass() {
  echo "ready: $1"
}

if [ -d "$firmware_dir" ]; then
  pass "prepared upstream firmware checkout exists at $firmware_dir"
else
  fail "prepared upstream checkout missing; run scripts/dev/prepare-firmware-upload.sh"
fi

if command -v node >/dev/null 2>&1; then
  pass "node is available"
else
  fail "node is required"
fi

if [ -f "$manifest" ]; then
  token_status="$(node - "$manifest" <<'NODE'
const manifest = require(process.argv[2])
const config = manifest.config?.tarsStackchan || {}
const token = config.token || ''
const prefix = config.speechPathPrefix || ''
if (!token || token === 'replace-with-local-token') {
  process.stdout.write('placeholder')
} else if (!prefix.includes('/api/tts?token=') || !prefix.includes('&text=')) {
  process.stdout.write('speech-prefix-missing-token')
} else {
  process.stdout.write('configured')
}
NODE
)"
  if [ "$token_status" = "configured" ]; then
    pass "firmware bridge token and speech TTS token are configured in prepared manifest"
  else
    fail "firmware bridge token is not ready ($token_status); rerun prepare script with TARS_STACKCHAN_TOKEN"
  fi
else
  fail "firmware bridge manifest missing in prepared checkout"
fi

if [ -f "$host_manifest" ]; then
  host_status="$(node - "$host_manifest" <<'NODE'
const manifest = require(process.argv[2])
const includes = manifest.include || []
const driver = manifest.config?.driver || {}
const led = manifest.config?.led || {}
const tts = manifest.config?.tts || {}
const ok = includes.includes('./manifest_m5stackchan_cores3.json') &&
  driver.type === 'm5stackchan' &&
  driver.servoPower?.type === 'py32' &&
  led.head?.type === 'py32' &&
  tts.type === 'remote' &&
  typeof tts.host === 'string' &&
  typeof tts.port === 'number' &&
  typeof tts.volume === 'number' &&
  tts.volume <= 0.2
process.stdout.write(ok ? 'ready' : `driver=${driver.type || 'missing'} ledHead=${led.head?.type || 'missing'} tts=${tts.type || 'missing'}:${tts.host || 'missing'}:${tts.port || 'missing'} volume=${tts.volume ?? 'missing'}`)
NODE
)"
  if [ "$host_status" = "ready" ]; then
    pass "host manifest is patched for CoreS3/K151 servo, head LED, and remote TTS"
  else
    fail "host manifest is not patched for CoreS3/K151 ($host_status); rerun prepare"
  fi
else
  fail "host manifest missing in prepared checkout"
fi

if command -v npm >/dev/null 2>&1; then
  pass "npm is available"
else
  fail "npm is required"
fi

if command -v uv >/dev/null 2>&1; then
  pass "uv is available for Python/esptool"
else
  fail "uv is required for direct ESP32 MOD flash"
fi

if [ -x "$firmware_dir/node_modules/.bin/xs-dev" ]; then
  pass "upstream npm dependencies are installed"
else
  fail "upstream npm dependencies missing; run npm install in $firmware_dir"
fi

mcconfig_path="$(command -v mcconfig 2>/dev/null || true)"
if [ -z "$mcconfig_path" ] && [ -x "$moddable_dir/build/bin/mac/release/mcconfig" ]; then
  mcconfig_path="$moddable_dir/build/bin/mac/release/mcconfig"
fi

if [ -n "$mcconfig_path" ]; then
  pass "mcconfig is available at $mcconfig_path"
else
  fail "mcconfig is not available; run npm run setup from $firmware_dir"
fi

serial_ports="$(ls /dev/cu.usbmodem* /dev/cu.usbserial* /dev/cu.SLAB_USBtoUART* 2>/dev/null || true)"
if [ -n "$serial_ports" ]; then
  pass "USB serial device candidate found"
  echo "$serial_ports"
else
  fail "no USB serial device candidate found; connect Stack-chan over USB"
fi

if [ "$ready" = true ]; then
  cat <<EOF

Firmware upload can start now:
  scripts/dev/upload-firmware.sh mod

For a full one-command host deploy + MOD direct flash + smoke:
  TARS_STACKCHAN_DEPLOY_HOST=1 scripts/dev/upload-firmware.sh all
EOF
  exit 0
fi

exit 1
