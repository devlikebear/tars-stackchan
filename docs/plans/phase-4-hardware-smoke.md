# Phase 4: End-To-End Hardware Smoke

Goal: prove Claude/TARS can control the real Stack-chan through MCP without the mobile app.

## Status

Phase 4 is prepared but not hardware-verified yet.

Added:

- `scripts/dev/prepare-firmware-upload.sh`
- `scripts/dev/check-firmware-upload-ready.sh`
- `scripts/test/hardware-smoke.sh`
- `docs/hardware-smoke.md`

## Upload Flow

```bash
cd /Users/changheonshin/workspace/myworks/tars-stackchan
export TARS_STACKCHAN_TOKEN="<local-token>"
scripts/dev/prepare-firmware-upload.sh
```

The script prepares an ignored upstream checkout at `.work/stack-chan`, copies the bridge MOD, and patches the MOD manifest token when `TARS_STACKCHAN_TOKEN` is set.

Then, from the prepared upstream firmware checkout:

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
export TARS_STACKCHAN_BASE_URL=http://stackchan.local
export TARS_STACKCHAN_TOKEN="<local-token>"
scripts/test/hardware-smoke.sh
```

The script checks both raw HTTP endpoints and MCP tool calls:

- `stackchan_get_status`
- `stackchan_set_expression`
- `stackchan_move_head`
- `stackchan_set_led`
- `stackchan_run_motion`

## Upload Possible When

- `TARS_STACKCHAN_TOKEN` is set.
- Stack-chan is connected over USB.
- `npm install` and Moddable/ESP32 setup have completed in `.work/stack-chan/firmware`.
- The target is selected, normally `esp32/m5stack_cores3` for CoreS3/K151.
- `scripts/dev/check-firmware-upload-ready.sh` passes.

At that point, run the two `npm_config_target=esp32/m5stack_cores3 ...` upload commands above.
