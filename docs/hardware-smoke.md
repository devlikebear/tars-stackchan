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
- `TARS_STACKCHAN_TOKEN` is set and matches the firmware MOD manifest.

## Prepare Firmware Upload

```bash
cd /Users/changheonshin/workspace/myworks/tars-stackchan
export TARS_STACKCHAN_TOKEN="<local-token>"
scripts/dev/prepare-firmware-upload.sh
```

Then install/setup upstream dependencies and run the upload commands printed by the script.

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

Not run yet. Hardware and firmware upload are pending.

## Known Limitations

- The bridge uses a static local bearer token for MVP.
- Camera, microphone, speech, NFC, IR, OTA, and multi-device flows are out of scope.
- The firmware overlay is tested by contract tests locally; full validation requires a real Stack-chan device.
