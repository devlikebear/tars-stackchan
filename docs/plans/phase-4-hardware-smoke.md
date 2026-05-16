# Phase 4: End-To-End Hardware Smoke

Goal: prove Claude/TARS can control the real Stack-chan through MCP without the mobile app.

## Status

Phase 4 passed against the connected CoreS3/K151 on 2026-05-16.

Added:

- `scripts/dev/prepare-firmware-upload.sh`
- `scripts/dev/check-firmware-upload-ready.sh`
- `scripts/dev/upload-firmware.sh`
- `scripts/test/hardware-smoke.sh`
- `scripts/test/hardware-smoke-contract.sh`
- `docs/hardware-smoke.md`

## Upload Flow

```bash
cd /Users/changheonshin/workspace/myworks/tars-stackchan
export TARS_STACKCHAN_TOKEN="<local-token>"
scripts/dev/prepare-firmware-upload.sh
```

The script prepares an ignored upstream checkout at `.work/stack-chan`, copies the bridge MOD, and patches the MOD manifest token when `TARS_STACKCHAN_TOKEN` is set.

For the current automated flow:

```bash
scripts/dev/check-firmware-upload-ready.sh
scripts/dev/upload-firmware.sh mod
```

For a fresh host deploy plus MOD flash:

```bash
TARS_STACKCHAN_DEPLOY_HOST=1 scripts/dev/upload-firmware.sh all
```

The helper builds the MOD with `mcrun`, then flashes the ESP32 `xs` partition directly with `uv run --with esptool esptool`.
This avoids the observed `serial2xsbug -install` hang on macOS/CoreS3.

Legacy manual flow from the prepared upstream firmware checkout:

```bash
cd .work/stack-chan/firmware
npm install
npm run setup
npm run setup -- --device=esp32
npm_config_target=esp32/m5stack_cores3 npm run deploy
npm_config_target=esp32/m5stack_cores3 npm run mod mods/tars_stackchan_bridge/manifest.json
```

## Smoke Flow

```bash
export TARS_STACKCHAN_BASE_URL=http://192.168.219.113
export TARS_STACKCHAN_TOKEN="<local-token>"
scripts/test/hardware-smoke.sh
```

The script checks both raw HTTP endpoints and MCP tool calls:

- `stackchan_get_status`
- `stackchan_set_expression`
- `stackchan_move_head`
- `stackchan_set_led`
- `stackchan_run_motion`

## Verified Results

- `GET /v1/status` responded from `http://192.168.219.113`.
- Missing and invalid bearer tokens returned HTTP 401.
- All five MCP tools succeeded in HTTP bridge mode.
- `stackchan_move_head` clamped `tilt_deg=120` to `tilt_deg=85`.
- The bridge MOD now uses `tars-http-server-service` and `tars-listen` to avoid collisions with host firmware modules.

## Upload Possible When

- `TARS_STACKCHAN_TOKEN` is set.
- Stack-chan is connected over USB.
- `npm install` and Moddable/ESP32 setup have completed in `.work/stack-chan/firmware`.
- The target is selected, normally `esp32/m5stack_cores3` for CoreS3/K151.
- `scripts/dev/check-firmware-upload-ready.sh` passes.

At that point, run `scripts/dev/upload-firmware.sh mod`, or use `TARS_STACKCHAN_DEPLOY_HOST=1 scripts/dev/upload-firmware.sh all` when the host firmware also needs to be rebuilt.
