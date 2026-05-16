# Hardware Smoke

Status: passed.

Date: 2026-05-16
Firmware upstream: `stack-chan/stack-chan@677224032e9ca25ac5c327b2eacd0034804b756f`
TARS Stack-chan commit: Phase 4 smoke fix commit
Device: M5Stack Stack-chan K151 / CoreS3
Base URL: `http://192.168.219.113`

## Upload Prerequisites

- Stack-chan is connected over USB.
- Stack-chan can join a 2.4 GHz Wi-Fi network.
- Upstream Moddable/ESP32 toolchain is installed.
- `uv` is installed; the upload helper uses `uv run --with esptool esptool` for ESP32 flash operations.
- `TARS_STACKCHAN_TOKEN` is set and matches the firmware MOD manifest.

## Prepare Firmware Upload

```bash
cd /Users/changheonshin/workspace/myworks/tars-stackchan
export TARS_STACKCHAN_TOKEN="<local-token>"
scripts/dev/upload-firmware.sh prepare
```

## One-command Upload

For a fresh device or after rebuilding the host firmware:

```bash
export TARS_STACKCHAN_TOKEN="<local-token>"
export TARS_STACKCHAN_UPLOAD_PORT=/dev/cu.usbmodem1101
export TARS_STACKCHAN_BASE_URL=http://192.168.219.113
TARS_STACKCHAN_DEPLOY_HOST=1 scripts/dev/upload-firmware.sh all
```

For the common iteration loop where the host firmware is already deployed:

```bash
export TARS_STACKCHAN_TOKEN="<local-token>"
export TARS_STACKCHAN_UPLOAD_PORT=/dev/cu.usbmodem1101
scripts/dev/upload-firmware.sh mod
```

The MOD upload uses direct ESP32 partition flashing instead of `npm run mod`.
The observed macOS/CoreS3 failure mode was `serial2xsbug -install` hanging after `Installing mod`.
The working path is:

1. Build the MOD archive with `mcrun -dl -m -p esp32/m5stack_cores3 -t build`.
2. Erase only the ESP32 `xs` MOD partition.
3. Flash the generated `.xsa` to the `xs` partition with `uv run --with esptool esptool`.

Defaults for the pinned CoreS3/K151 build:

```text
TARS_STACKCHAN_MOD_OFFSET=0xfa0000
TARS_STACKCHAN_MOD_SIZE=0x40000
```

Check readiness any time:

```bash
scripts/dev/check-firmware-upload-ready.sh
```

The bridge MOD must avoid module names that collide with host firmware modules.
The Stack-chan host already ships `http-server-service`; the bridge maps its retained service as `tars-http-server-service` and imports that name so the MOD does not accidentally load the host copy.
If the device does not answer HTTP immediately after direct flash, a USB hard reset can bring Wi-Fi and the bridge API back without reflashing.

## Smoke Commands

```bash
export TARS_STACKCHAN_BASE_URL=http://stackchan.local
export TARS_STACKCHAN_TOKEN="<local-token>"
scripts/test/hardware-smoke.sh
```

When `scripts/dev/upload-firmware.sh smoke` fails to reach the local API and a USB port is available, it hard-resets the device once with `uv run --with esptool esptool ... chip-id`, waits, and retries the smoke. Set `TARS_STACKCHAN_SMOKE_RETRY_USB_RESET=0` to disable this retry.

## Checklist

- [x] Firmware host deploy completed.
- [x] `tars_stackchan_bridge` MOD upload completed.
- [x] `scripts/dev/check-firmware-upload-ready.sh` passes before upload.
- [x] Device joined Wi-Fi.
- [x] `GET /v1/status` responds.
- [x] Missing or invalid token is rejected for mutating requests.
- [x] `stackchan_get_status` works through MCP.
- [x] `stackchan_set_expression` changes expression.
- [x] `stackchan_move_head` moves head and clamps unsafe tilt.
- [x] `stackchan_set_led` changes LED.
- [x] `stackchan_run_motion` runs `nod`.
- [x] `stackchan_speak` sends a short speech request.

## Results

Direct MOD flash and end-to-end hardware smoke have been verified on the connected CoreS3/K151.
The bridge boot log reached:

```text
[tars-stackchan] bypassing default setup launch
[tars-stackchan] local control API listening on port 80
```

Successful HTTP status response:

```json
{"connected":true,"device":"stackchan-k151","firmware":"tars-stackchan-dev","ip":"192.168.219.113","capabilities":["expression","head","leds","motion","speech"]}
```

Successful hardware smoke command:

```bash
TARS_STACKCHAN_BASE_URL=http://192.168.219.113 scripts/dev/upload-firmware.sh smoke
```

The smoke covered raw HTTP requests and MCP tool calls for status, expression, head movement, LED, motion, and speech. The unsafe `tilt_deg=120` input was clamped to `tilt_deg=85` by the firmware response.

The speech endpoint returns as soon as the request is accepted. It starts `robot.say(...)` in the background so HTTP and MCP smoke tests do not block on TTS generation or playback.

Missing and invalid bearer tokens were rejected with HTTP 401:

```text
{"error":"invalid token"}
HTTP 401
```

## Known Limitations

- The bridge uses a static local bearer token for MVP.
- Camera, microphone, NFC, IR, OTA, and multi-device flows are out of scope.
- The firmware overlay is tested by contract tests locally; full validation requires a real Stack-chan device.
- `stackchan.local` did not resolve in this environment during the smoke; use the IP from the firmware log or status response until mDNS is verified.
