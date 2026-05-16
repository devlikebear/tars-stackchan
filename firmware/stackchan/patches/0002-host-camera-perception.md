# Patch: Host Camera Perception (Embodied Bot Phase 1)

This patch note describes how the project adds on-device camera capture to the
official Stack-chan firmware for the Embodied Bot perception loop. It builds on
[`0001-install-tars-stackchan-bridge-mod.md`](./0001-install-tars-stackchan-bridge-mod.md).

## Upstream Base

```text
repository: https://github.com/stack-chan/stack-chan
commit: 677224032e9ca25ac5c327b2eacd0034804b756f
```

At this commit the device-side `stackchan/camera.ts` is a **stub**
(`capture()` returns `undefined`); the only real implementation is the
browser/WASM simulator path. There is no native ESP32 camera binding.

## Why This Is A Host Change (not a MOD)

The Moddable SDK ships an ECMA-419 camera module
`embedded:io/image/in/camera` (`$(MODDABLE)/modules/io/imagein/camera`) whose
ESP32 backend depends on the native IDF components `esp32-camera ^2.0.10` and
`esp_jpeg ^1.3.1`. Native code must be compiled into the **host firmware**; a
runtime MOD (XS bytecode loaded after boot) cannot add native components.
Therefore camera capture requires a **host deploy flash**, not a `mod`-only
flash.

No upstream stack-chan source files are modified. The upstream stub
`stackchan/camera.ts` is left untouched and unused; the bridge MOD imports
`embedded:io/image/in/camera` directly for the `/v1/camera/snapshot` handler.

## Applied By The Prepare Script

`scripts/dev/prepare-firmware-upload.sh` patches the upstream host manifest
`firmware/stackchan/manifest_local.json` for the `esp32/m5stack_cores3`
target. For the camera it adds:

```jsonc
{
  "include": [
    "./manifest_m5stackchan_cores3.json",
    "$(MODDABLE)/modules/io/imagein/camera/manifest.json",
    "$(MODDABLE)/modules/io/audioin/manifest.json"
  ],
  "config": {
    "camera": { "frameSize": "QVGA", "jpeg": { "quality": 12 } },
    "audioIn": { "sampleRate": 16000, "bitsPerSample": 16 }
  }
}
```

`embedded:io/audio/in` (native IDF `esp_driver_i2s`) is added the same way and
for the same reason as the camera: it is a host module. The bridge MOD records
a WAV clip from it for `/v1/audio/clip`. 16 kHz / 16-bit matches the protocol's
declared audio format. Upstream `stackchan/microphone.ts` is not used; the MOD
ships its own minimal WAV recorder modeled on it.

The GC0308 camera pin map (powerdown/reset/xclk/pclk/href/vsync/scl/sda/d0..d7)
and the base `camera.frameSize` are already supplied by the Moddable SDK
target manifest
`$(BUILD)/devices/esp32/targets/m5stack_cores3/manifest.json`, which the
CoreS3 platform manifest includes. esp32-camera autodetects the sensor over
SCCB, so only the module include and JPEG/framesize config are added here.

## MOD Usage

The bridge MOD (`firmware/stackchan/mods/tars_stackchan_bridge`) uses the
host-provided module for the snapshot endpoint (implemented in Phase 1.2):

```js
import Camera from "embedded:io/image/in/camera"
// GET /v1/camera/snapshot -> capture a JPEG frame -> image/jpeg response
```

The microphone path needs no host change: upstream `stackchan/microphone.ts`
already wraps `embedded:io/audio/in` and the CoreS3 build configures
`audioIn` at 16 kHz / 16-bit. The MOD records a clip for `/v1/audio/clip`.

## Build And Flash

Camera is in the host firmware, so a host deploy is required:

```bash
TARS_STACKCHAN_UPLOAD_PORT=/dev/cu.usbmodemXXXX \
TARS_STACKCHAN_DEPLOY_HOST=1 \
scripts/dev/upload-firmware.sh all
```

A `mod`-only flash after a host deploy that already includes the camera
module is sufficient for later MOD-only iterations.

## Verification

```bash
curl -H "Authorization: Bearer $TARS_STACKCHAN_TOKEN" \
  http://stackchan.local/v1/camera/snapshot -o /tmp/snap.jpg
file /tmp/snap.jpg   # expect: JPEG image data

curl -H "Authorization: Bearer $TARS_STACKCHAN_TOKEN" \
  "http://stackchan.local/v1/audio/clip?ms=1500" -o /tmp/clip.wav
file /tmp/clip.wav   # expect: RIFF (little-endian) WAVE audio
```

## Risks / Notes

- `esp32-camera` / `esp_jpeg` are resolved by the IDF component manager at
  build time (requires network on first build). Standard for Moddable camera
  examples (`$(MODDABLE)/examples/io/imagein/camera/camera-server-jpeg`).
- **First host build gotcha (root-caused 2026-05-16):** the first host
  builds fail with `fatal error: esp_camera.h: No such file or directory`.
  Moddable compiles module C (`cc camera.c.o`) *before* the ESP-IDF app
  phase, using include paths under
  `xsProj-<subclass>/managed_components/espressif__esp32-camera/...`. Those
  dirs are only populated when the IDF component manager solves
  `main/idf_component.yml`. Because the Moddable build dies at `cc
  camera.c.o` before its own `idf.py reconfigure` runs, `managed_components/`
  stays empty and **simply re-running the host deploy does NOT fix it**
  (verified: two identical failures).

  Verified recipe (2026-05-16) — three steps, once per build tree:

  1. Run the host deploy once. It fails at `cc camera.c.o` but generates
     `xsProj-<subclass>/main/idf_component.yml` with both deps.
  2. Fetch the managed components with the IDF component manager, e.g.
     `cd "$MODDABLE/build/tmp/esp32/m5stack_cores3/<debug|release>/stackchan/xsProj-<subclass>"`,
     `. "$IDF_PATH/export.sh"`, then `idf.py update-dependencies` (or
     `reconfigure`). **Either command also runs CMake with the wrong/default
     IDF target and no Moddable `-D` defines, poisoning
     `xsProj-<subclass>/build/`** (symptoms seen: `Target 'esp32s3' not
     consistent with 'esp32'`; `main/CMakeLists.txt` release branch →
     `Cannot find source file: debugger_none.c`; `ninja: 'xs_.a' missing`).
  3. **Therefore delete the poisoned build dir before rebuilding:**
     `rm -rf "$XP/build"` (keep `managed_components/`, `dependencies.lock`,
     `main/idf_component.yml` — they live at the xsProj root, not under
     `build/`). Then `TARS_STACKCHAN_DEPLOY_HOST=1 ... upload-firmware.sh all`
     succeeds: Moddable regenerates `build/` with its own correct `-D`
     defines, the component manager reuses `dependencies.lock` (no
     re-download), `esp_camera.h` resolves, and the host links + flashes.

  Net: a debug **and** a release tree each cost roughly 1 (fail) + fetch +
  rm build + 1 (succeed) host build. Not a regression; inherent to layering a
  new IDF managed component under Moddable's build ordering.
- The camera framebuffer needs PSRAM; CoreS3 has 8 MB PSRAM and JPEG QVGA is
  small, so the budget is comfortable.

## Spike S fix — camera SCCB shares Moddable's I2C bus (2026-05-17)

Root cause (Spike S): CoreS3 routes the camera SCCB onto the same internal
I2C bus (GPIO12 SDA / GPIO11 SCL) that Moddable already owns via the IDF
**new `i2c_master`** driver (AXP2101 / AW9523 / touch / audio codecs, IDF
port 0). esp32-camera creating its own bus on those pins hard-resets the
device.

Fix is **config-only** (no SDK/component source patch), applied by the
prepare script's CoreS3 host-manifest patch:

```jsonc
"config": { "camera": { "frameSize": "QVGA", "jpeg": { "quality": 12 },
                          "sda": -1, "scl": -1, "i2c_port": 0 } }
```

Why it works (esp32-camera 2.1.6, `CONFIG_SCCB_HARDWARE_I2C_DRIVER_NEW=y`):
`esp_camera.c` takes the `SCCB_Use_Port()` branch when `pin_sccb_sda == -1`
instead of `SCCB_Init()`; `sccb-ng.c::SCCB_Use_Port` sets
`sccb_owns_i2c_port=false` and `SCCB_Install_Device` then calls
`i2c_master_get_bus_handle(sccb_i2c_port)` — reusing the existing bus on that
port rather than creating one. `i2c_port:0` matches Moddable's internal bus
(`_i2c.c` default `I2C_NUM_0`). DVP pins (d0..d7/xclk/pclk/href/vsync) keep
the SDK target values via manifest deep-merge. Config-only; no source patch.

**Verification result (2026-05-17): did NOT resolve the reset.** With this
config applied and flashed, `/v1/camera/snapshot` still hard-resets the real
CoreS3 (xsbug reboot banner; device recovers; audio path unaffected). The
SCCB-bus-share theory was sound and the cheapest fix to try first, but is
either incomplete or not the (only) cause. Remaining unverifiable-without-
backtrace suspects: a fault in the `cam_hal`/DVP path (LCD_CAM peripheral,
PSRAM DMA, or XCLK on hardcoded `LEDC_TIMER_0/CHANNEL_0`) which
`esp_camera_init` brings up independently of SCCB; or Moddable's internal
I2C port/lifetime differing from the `I2C_NUM_0` assumption. A real fix now
needs esp32-camera/Moddable SDK **source instrumentation** (outside this
repo's overlay model) or JTAG — backtrace is unobtainable on USB-CDC CoreS3.
**Camera remains a documented OPEN limitation; the audio path is verified
working on real hardware** (`TARS_STACKCHAN_PERCEIVE_CAMERA=off`). The
config is left in place: it is correct/harmless and a prerequisite for any
deeper fix.

### Web research (2026-05-17) — authoritative solution pattern found

The official M5Stack CoreS3 camera example
(`docs.m5stack.com/en/arduino/m5cores3/camera`) initializes the GC0308 as:

1. **`M5.In_I2C.release()`** — release the internal I2C bus from host control
2. `esp_camera_init(&camera_config)` (config notably uses `i2c_port = -1`,
   `xclk = -1`, SDA 12 / SCL 11, RGB565/QVGA, fb_count 2)

i.e. on CoreS3 the camera SCCB and the host's internal I2C **cannot be
co-owned**; the host must **hand the bus off** (release) before
`esp_camera_init`, then the camera owns it. This is exactly why the
config-only "share the bus concurrently" attempt failed.

**Missing piece in Moddable:** `modules/io/imagein/camera/esp32/camera.c`
`#include "_i2c.h"` but performs **no** I2C release/deactivate before
`esp_camera_init` (line 217). Moddable's `modules/io/i2c/esp32/_i2c.c`
already has the teardown primitives (`i2c_del_master_bus`, `modI2CUninit`
"make pins release bus", `modI2CDeactivate`), so the fix is implementable:
release the Moddable internal I2C bus immediately before `esp_camera_init`
in `cameraLoop()` (and decide bus ownership afterward — camera owns SCCB; if
other peripherals need the bus again later, re-init or coordinate).

**Ownership/scope:** this is a Moddable SDK source change, outside this
repo's overlay model. Implemented as a project-vendored patched camera
module (`firmware/stackchan/host-overlay/imagein-camera-cores3/`) remapped
via the prepare host-manifest patch.

### Handoff attempt result (2026-05-17) — also did NOT resolve the reset

The vendored `camera.c` calls `tarsReleaseSharedI2C()` (public IDF
`i2c_master_get_bus_handle` + `i2c_del_master_bus` on ports 0/1)
immediately before `esp_camera_init`, mirroring M5Unified's
`In_I2C.release()`. Built and flashed successfully (patched camera.c
compiles); **`/v1/camera/snapshot` still hard-resets the real CoreS3**
(xsbug reboot banner; device recovers; audio path unaffected).

**Two reasoned fixes have now failed** — (1) config-only concurrent bus
share (`SCCB_Use_Port`), (2) M5-pattern temporal handoff. Remaining,
mutually-indistinguishable-without-a-backtrace suspects: a fault in
`cam_hal`/DVP (LCD_CAM peripheral, PSRAM DMA, XCLK on hardcoded
`LEDC_TIMER_0/CHANNEL_0`); `i2c_del_master_bus` failing because Moddable's
attached devices were not removed first; or Moddable's static `gBus`
dangling after deletion. A backtrace is unobtainable on USB-CDC CoreS3
(debug serial owned by xsbug; release build separately broken) and no JTAG
is available, so further attempts are blind multi-flash cycles.

**Decision: camera/vision on real CoreS3 is a documented OPEN limitation.**
Root cause and two attempted fixes are recorded for a future effort that has
a hardware debugger. The vendored overlay + the prepare wiring are left in
place (harmless: `tarsReleaseSharedI2C` only runs if a snapshot is
requested; audio-only operation never triggers it). The audio perception
path is verified working on real hardware — run with
`TARS_STACKCHAN_PERCEIVE_CAMERA=off`.

## Build succeeds, but camera capture is an OPEN DEFECT (2026-05-16) — superseded by the Spike S fix above

The host firmware **builds, flashes, boots, joins Wi-Fi**, and
`/v1/status` (advertising `camera`,`microphone`), `/v1/sensors`, and bearer
auth all verified on real M5Stack CoreS3. **However `GET /v1/camera/snapshot`
hard-resets the device** (reproduced on debug build: status/sensors keep
working, snapshot → ESP-ROM reboot). `/v1/audio/clip` unverified (blocked).

Root cause (code-grounded; backtrace unobtainable — CoreS3 is USB-CDC only,
debug build's serial is owned by xsbug, release build's USB console drops the
panic on reset, no JTAG): native esp32-camera ↔ GC0308 bring-up on a board
Moddable does **not** officially support. Disproven cheap hypotheses: power
rails (AXP2101 `0x90=0xbf` already enables ALDO1-4/BLDO1-2/DLDO1) and
JS Camera options (`dimensionsToFrameSize(320,·)` → valid QVGA; an option
error would surface as a catchable JS error / HTTP 400, not a reset).

Remaining suspects for a future dedicated spike: AW9523-gated camera
reset/enable line distinct from the AXP rails; LEDC timer/channel conflict
(camera.c hardcodes `LEDC_TIMER_0`/`LEDC_CHANNEL_0`, which CoreS3 may already
use for backlight/buzzer); GC0308-specific clock/DMA. Tracked as a separate
"CoreS3 camera native bring-up" spike; Phase 2 proceeds against the mock
bridge until it is resolved.
