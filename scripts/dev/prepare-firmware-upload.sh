#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
lock_file="$repo_root/firmware/stackchan/upstream.lock"
overlay_dir="$repo_root/firmware/stackchan/mods/tars_stackchan_bridge"
host_overlay_dir="$repo_root/firmware/stackchan/host-overlay/imagein-camera-cores3"

upstream_repo="$(awk -F= '$1 == "repository" { print $2 }' "$lock_file")"
upstream_commit="$(awk -F= '$1 == "commit" { print $2 }' "$lock_file")"
target_dir="${TARS_STACKCHAN_UPSTREAM_DIR:-$repo_root/.work/stack-chan}"
token="${TARS_STACKCHAN_TOKEN:-}"
target="${TARS_STACKCHAN_TARGET:-esp32/m5stack_cores3}"
tts_host="${TARS_STACKCHAN_TTS_HOST:-}"
tts_port="${TARS_STACKCHAN_TTS_PORT:-18080}"
tts_volume="${TARS_STACKCHAN_TTS_VOLUME:-0.15}"

# Default to the project-stable mDNS hostname instead of a raw, DHCP-volatile
# IP. The Go relay (tars-stackchan-control tts serve) advertises this name via
# dns-sd, so the device resolves the current relay IP at boot without a
# re-flash. Keep this literal in sync with tts.DefaultTTSHostname
# (mcp-server/internal/tts/mdns.go); firmware-upload-script-contract.sh asserts
# it. Setting TARS_STACKCHAN_TTS_HOST=<ip-or-host> still wins as a fallback for
# networks where mDNS does not resolve.
if [ -z "$tts_host" ]; then
  tts_host="tars-stackchan-tts.local"
fi

if ! command -v git >/dev/null 2>&1; then
  echo "git is required" >&2
  exit 1
fi

if ! command -v node >/dev/null 2>&1; then
  echo "node is required to patch the MOD manifest token" >&2
  exit 1
fi

mkdir -p "$(dirname -- "$target_dir")"

if [ ! -d "$target_dir/.git" ]; then
  git clone "$upstream_repo" "$target_dir"
fi

git -C "$target_dir" fetch --depth 1 origin "$upstream_commit"
git -C "$target_dir" checkout --detach "$upstream_commit"

# Restore the upstream-tracked host manifest base. A prior patched state or an
# interrupted run can leave firmware/stackchan/manifest_local.json deleted in
# the worktree; the host firmware build then fails with
# "manifest_local.json: manifest not found!" and the TTS host bake below is
# silently skipped. It is tracked upstream (not gitignored), so restoring it
# gives the patch step a deterministic clean base on every run.
git -C "$target_dir" checkout -- firmware/stackchan/manifest_local.json 2>/dev/null || true

mkdir -p "$target_dir/firmware/mods"
rm -rf "$target_dir/firmware/mods/tars_stackchan_bridge"
cp -R "$overlay_dir" "$target_dir/firmware/mods/tars_stackchan_bridge"

# Spike S host overlay: project-vendored patched Moddable camera module that
# hands the shared I2C bus off before esp_camera_init (M5 In_I2C.release()
# pattern). Copied next to manifest_local.json so the host manifest can
# include it with a relative path.
rm -rf "$target_dir/firmware/stackchan/tars-imagein-camera"
cp -R "$host_overlay_dir" "$target_dir/firmware/stackchan/tars-imagein-camera"

manifest="$target_dir/firmware/mods/tars_stackchan_bridge/manifest.json"
host_manifest="$target_dir/firmware/stackchan/manifest_local.json"

if [ -n "$token" ]; then
  node - "$manifest" "$token" <<'NODE'
const fs = require('fs')
const [manifestPath, token] = process.argv.slice(2)
const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
manifest.config ??= {}
manifest.config.tarsStackchan ??= {}
manifest.config.tarsStackchan.token = token
manifest.config.tarsStackchan.speechPathPrefix = `/api/tts?token=${encodeURIComponent(token)}&text=`
fs.writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`)
NODE
else
  echo "warning: TARS_STACKCHAN_TOKEN is empty; manifest still contains replace-with-local-token" >&2
fi

if [ -f "$host_manifest" ]; then
  node - "$host_manifest" "$target" "$tts_host" "$tts_port" "$tts_volume" <<'NODE'
const fs = require('fs')
const [manifestPath, target, ttsHost, ttsPort, ttsVolume] = process.argv.slice(2)
const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
manifest.config ??= {}

if (target === 'esp32/m5stack_cores3') {
  // Perception (Embodied Bot Phase 1): pull the Moddable ECMA-419 camera
  // module into the HOST firmware build. esp32-camera + esp_jpeg are native
  // IDF components, so they must be compiled into the host (a runtime MOD
  // cannot add native code) -- this is why camera capture requires a host
  // deploy flash, not `mod`-only. The CoreS3 SDK target manifest already
  // supplies the GC0308 pin map and QVGA framesize; this only adds the
  // module so the bridge MOD can `import "embedded:io/image/in/camera"`.
  // No upstream stack-chan source files are modified.
  // Spike S fix (I2C hand-off): include the project-vendored camera module
  // overlay instead of the stock SDK one. The overlay manifest itself
  // includes the SDK imagein/camera manifest (esp32-camera/esp_jpeg deps +
  // include dirs) and only overrides the esp32 native module with a patched
  // camera.c that releases the shared internal I2C bus immediately before
  // esp_camera_init (mirrors M5Unified's In_I2C.release()). Copied to
  // ./tars-imagein-camera by the overlay copy step above.
  manifest.include = [
    './manifest_m5stackchan_cores3.json',
    './tars-imagein-camera/manifest.json',
    '$(MODDABLE)/modules/io/audioin/manifest.json',
  ]
  // Camera SCCB now CREATES its bus on the real pins AFTER the hand-off
  // releases Moddable's bus, so keep the SDK target's GC0308 pin map (sda
  // 12 / scl 11 / i2c_port 1 / d0..d7) via manifest deep-merge — only set
  // framesize/jpeg here. (The earlier sda/scl=-1+i2c_port=0 "share the bus
  // concurrently" config did not work; CoreS3 needs the temporal hand-off.)
  manifest.config.camera = {
    frameSize: 'QVGA',
    jpeg: { quality: 12 },
  }
  // Microphone for /v1/audio/clip. embedded:io/audio/in is also a host
  // (native IDF: esp_driver_i2s) module, so it ships with the host build
  // alongside the camera. 16 kHz / 16-bit matches the protocol's WAV format.
  manifest.config.audioIn = {
    sampleRate: 16000,
    bitsPerSample: 16,
  }
  manifest.config.driver = {
    type: 'm5stackchan',
    panId: 1,
    tiltId: 2,
    yawZeroPosition: 460,
    pitchZeroPosition: 620,
    serial: {
      transmit: 6,
      receive: 7,
      port: 1,
      baud: 1000000,
    },
    servoPower: {
      type: 'py32',
      pin: 0,
      address: 111,
    },
  }
  manifest.config.led ??= {}
  manifest.config.led.head = {
    type: 'py32',
    length: 12,
    ledPin: 13,
    address: 111,
  }
  if (ttsHost) {
    manifest.config.tts = {
      type: 'remote',
      host: ttsHost,
      port: Number(ttsPort),
      sampleRate: 24000,
      volume: Number(ttsVolume),
    }
  }
}

fs.writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`)
NODE
  echo "ready: patched host manifest for $target"
  echo "ready: patched host TTS remote server to $tts_host:$tts_port at volume $tts_volume"
  case "$tts_host" in
    *.local)
      echo "note: $tts_host is an mDNS name; the relay must advertise it (brew services start tars-stackchan / tars-stackchan-control tts serve). Set TARS_STACKCHAN_TTS_HOST=<ip> to bake a fixed IP fallback." ;;
  esac
else
  echo "warning: host manifest not found: $host_manifest" >&2
fi

cat <<EOF
Prepared upstream firmware checkout:
  $target_dir

Next upload commands:
  TARS_STACKCHAN_UPLOAD_PORT=/dev/cu.usbmodemXXXX \\
  TARS_STACKCHAN_DEPLOY_HOST=1 \\
  scripts/dev/upload-firmware.sh all

MOD-only direct flash after the host is already deployed:
  TARS_STACKCHAN_UPLOAD_PORT=/dev/cu.usbmodemXXXX \\
  scripts/dev/upload-firmware.sh mod

After upload, run:
  TARS_STACKCHAN_BASE_URL=http://stackchan.local \\
  TARS_STACKCHAN_TOKEN="\$TARS_STACKCHAN_TOKEN" \\
  "$repo_root/scripts/test/hardware-smoke.sh"
EOF
