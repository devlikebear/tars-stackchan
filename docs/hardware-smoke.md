# Hardware Smoke

Status: not run yet.

Date: TBD
Firmware upstream: `stack-chan/stack-chan@677224032e9ca25ac5c327b2eacd0034804b756f`
TARS Stack-chan commit: TBD
Device: M5Stack Stack-chan K151 / CoreS3
Base URL: `http://stackchan.local`

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

## Smoke Commands

```bash
export TARS_STACKCHAN_BASE_URL=http://stackchan.local
export TARS_STACKCHAN_TOKEN="<local-token>"
scripts/test/hardware-smoke.sh
```

## Checklist

- [ ] Firmware host deploy completed.
- [ ] `tars_stackchan_bridge` MOD upload completed.
- [ ] `scripts/dev/check-firmware-upload-ready.sh` passes before upload.
- [ ] Device joined Wi-Fi.
- [ ] `GET /v1/status` responds.
- [ ] Missing or invalid token is rejected for mutating requests.
- [ ] `stackchan_get_status` works through MCP.
- [ ] `stackchan_set_expression` changes expression.
- [ ] `stackchan_move_head` moves head and clamps unsafe tilt.
- [ ] `stackchan_set_led` changes LED.
- [ ] `stackchan_run_motion` runs `nod`.

## Results

Host firmware deploy and direct MOD flash have been verified on the connected CoreS3/K151.
The bridge boot log reached:

```text
[tars-stackchan] bypassing default setup launch
[tars-stackchan] local control API listening on port 80
```

HTTP smoke is still blocked until the Mac can route to the device IP.
The last observed device IP was `192.168.219.113`, but Mac-side TCP checks returned `No route to host`.

## Known Limitations

- The bridge uses a static local bearer token for MVP.
- Camera, microphone, speech, NFC, IR, OTA, and multi-device flows are out of scope.
- The firmware overlay is tested by contract tests locally; full validation requires a real Stack-chan device.
- Some routers isolate Wi-Fi clients. If the firmware log shows an IP but `curl` fails with `No route to host`, check Mac network attachment, VPN routes, and router client isolation before changing firmware code.
