# Firmware Bridge

This directory contains the firmware-side overlay for the TARS Stack-chan local control API.

The current official Stack-chan firmware is a Moddable SDK TypeScript/JavaScript firmware with MOD overlays, not an ESP-IDF-only C/C++ tree. The TARS bridge is therefore implemented as a small Stack-chan MOD overlay instead of a host firmware rewrite.

## Upstream

The overlay targets:

```text
repository: https://github.com/stack-chan/stack-chan
commit: 677224032e9ca25ac5c327b2eacd0034804b756f
checked: 2026-05-16
```

The upstream lock is recorded in [stackchan/upstream.lock](stackchan/upstream.lock).

Project-owned firmware code lives at:

```text
firmware/stackchan/mods/tars_stackchan_bridge/
```

## Local API

The MOD exposes the `/v1` local HTTP API used by the MCP server:

```text
GET  /v1/status
POST /v1/expression
POST /v1/head
POST /v1/leds
POST /v1/motion
POST /v1/speech
```

Mutating requests require:

```http
Authorization: Bearer <token>
```

The token is configured in `manifest.json` under `config.tarsStackchan.token`. The prepare helper patches this from `TARS_STACKCHAN_TOKEN`.

The protocol contract is documented in [../docs/protocol/local-control-api.md](../docs/protocol/local-control-api.md).

## Build And Upload

The project helper wraps upstream checkout preparation, dependency setup, MOD build, direct flash, and smoke testing:

```bash
cd /Users/changheonshin/workspace/myworks/tars-stackchan
export TARS_STACKCHAN_TOKEN="<local-token>"
export TARS_STACKCHAN_UPLOAD_PORT=/dev/cu.usbmodem1101
scripts/dev/upload-firmware.sh mod
```

For a fresh device or when the host firmware also needs to be rebuilt:

```bash
TARS_STACKCHAN_DEPLOY_HOST=1 scripts/dev/upload-firmware.sh all
```

Available modes:

- `prepare`: clone/pin upstream firmware, copy the bridge MOD, patch manifests.
- `deps`: prepare and install upstream dependencies/toolchain when needed.
- `host`: deploy the upstream host firmware.
- `mod`: build and direct-flash the TARS bridge MOD.
- `smoke`: run the hardware smoke check.
- `all`: prepare, install deps, optionally deploy host, flash MOD, and smoke.

Check readiness any time:

```bash
scripts/dev/check-firmware-upload-ready.sh
```

## Direct Flash Path

The helper avoids the observed macOS/CoreS3 `serial2xsbug -install` hang by direct-flashing the generated XSA archive into the ESP32 `xs` MOD partition:

```text
offset: 0xfa0000
size:   0x40000
```

Python tooling for this flash path runs through `uv`:

```bash
uv run --with esptool esptool ...
```

If the upstream partition table changes, override with:

```bash
export TARS_STACKCHAN_MOD_OFFSET=0xfa0000
export TARS_STACKCHAN_MOD_SIZE=0x40000
```

## K151/CoreS3 Host Patch

For host deploys, `scripts/dev/prepare-firmware-upload.sh` patches upstream `stackchan/manifest_local.json` for the CoreS3/K151 hardware:

- `driver.type` is set to `m5stackchan`.
- Servo power is set to the `py32` path used by K151/CoreS3.
- The head LED group is configured.
- Remote TTS is pointed at the local TTS relay.
- Default host TTS volume is `0.15` for close-range testing.

This matters because the pinned upstream default can use `driver.type: none`, which makes head and motion calls return success without physical servo movement.

## Speech TTS Relay

Start the Gemini 3.1 Flash TTS relay on the Mac before asking Stack-chan to speak:

```bash
export GEMINI_API_KEY="<google-ai-studio-api-key>"
scripts/dev/run-local-tts.sh
```

Defaults:

- `TARS_STACKCHAN_TTS_MODEL=gemini-3.1-flash-tts-preview`
- `TARS_STACKCHAN_TTS_VOICE=Kore`
- `TARS_STACKCHAN_TTS_VOLUME=0.15`
- `TARS_STACKCHAN_TTS_PORT=18080`

The relay calls the Gemini REST TTS endpoint, wraps the returned 24 kHz mono PCM as WAV, and serves it at `/api/tts?token=...&text=...`.

The relay requires a token so other LAN clients cannot spend the Gemini quota. `scripts/dev/run-local-tts.sh` uses the first available value from:

- `TARS_STACKCHAN_TTS_TOKEN`
- `TARS_STACKCHAN_TOKEN`
- the prepared MOD manifest token

The prepare helper patches the MOD speech path to include that token, and the relay redacts tokens from request logs.

## MCP Firmware Tool

The MCP server can expose firmware upload through `stackchan_upload_firmware`, but only when explicitly enabled:

```bash
TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS=1 tars-stackchan-mcp
```

Recommended install for Claude Code:

```bash
tars-stackchan-mcp install --target claude-code --firmware-tools
```

The Homebrew formula sets `TARS_STACKCHAN_REPO_ROOT` for the installed wrapper so the MCP firmware tool can find packaged `scripts/dev/upload-firmware.sh` and `firmware/`.

Before giving an agent upload access, run:

```bash
tars-stackchan-mcp doctor
```

## Verification

Local contract tests:

```bash
scripts/test/firmware-bridge-contract.sh
scripts/test/firmware-upload-script-contract.sh
scripts/test/hardware-smoke-contract.sh
scripts/test/tts-remote-server-contract.sh
```

Hardware smoke:

```bash
export TARS_STACKCHAN_BASE_URL=http://stackchan.local
export TARS_STACKCHAN_TOKEN="<local-token>"
scripts/test/hardware-smoke.sh
```

When `scripts/dev/upload-firmware.sh smoke` cannot reach the local API and a USB port is available, it performs one USB hard-reset retry before failing. Disable that retry with:

```bash
export TARS_STACKCHAN_SMOKE_RETRY_USB_RESET=0
```

## Manual HTTP Smoke

```bash
curl http://stackchan.local/v1/status

curl -X POST \
  -H "Authorization: Bearer $TARS_STACKCHAN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"emotion":"happy"}' \
  http://stackchan.local/v1/expression

curl -X POST \
  -H "Authorization: Bearer $TARS_STACKCHAN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pan_deg":45,"tilt_deg":120,"speed":0.6}' \
  http://stackchan.local/v1/head

curl -X POST \
  -H "Authorization: Bearer $TARS_STACKCHAN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text":"hello stack-chan","volume":0.15}' \
  http://stackchan.local/v1/speech
```

The head response should report `tilt_deg` as `85`. Pan is also clamped to the firmware overlay's safe `-90..90` degree range.
