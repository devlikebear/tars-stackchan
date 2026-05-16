# Firmware Bridge

This directory contains the firmware-side overlay for the TARS Stack-chan local control API.

The current official Stack-chan firmware is not an ESP-IDF-only C/C++ tree. It is a Moddable SDK firmware with TypeScript/JavaScript host code and MOD overlays. Because of that, Phase 3 is implemented as a small Stack-chan MOD instead of an ESP-IDF component rewrite.

## Upstream

The overlay targets:

```text
repository: https://github.com/stack-chan/stack-chan
commit: 677224032e9ca25ac5c327b2eacd0034804b756f
checked: 2026-05-16
```

The upstream lock is recorded in [stackchan/upstream.lock](stackchan/upstream.lock).

## Overlay

Project-owned firmware code lives at:

```text
firmware/stackchan/mods/tars_stackchan_bridge/
```

It exposes the Phase 2 local HTTP API:

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

The token is configured in `manifest.json` under `config.tarsStackchan.token`.
For K151/CoreS3, the bridge uses the `head` LED group from the upstream platform config.

## Apply To Upstream Firmware

Clone the upstream firmware and copy the overlay:

```bash
git clone https://github.com/stack-chan/stack-chan.git
cd stack-chan
git checkout 677224032e9ca25ac5c327b2eacd0034804b756f

cp -R /Users/changheonshin/workspace/myworks/tars-stackchan/firmware/stackchan/mods/tars_stackchan_bridge \
  firmware/mods/tars_stackchan_bridge
```

Edit `firmware/mods/tars_stackchan_bridge/manifest.json` and replace `replace-with-local-token`.

## Build And Flash

The project helper wraps the upstream Moddable setup and the reliable upload path:

```bash
cd /Users/changheonshin/workspace/myworks/tars-stackchan
export TARS_STACKCHAN_TOKEN="<local-token>"
export TARS_STACKCHAN_UPLOAD_PORT=/dev/cu.usbmodem1101
TARS_STACKCHAN_DEPLOY_HOST=1 scripts/dev/upload-firmware.sh all
```

For MOD-only iteration after the host firmware is already deployed:

```bash
scripts/dev/upload-firmware.sh mod
```

The helper intentionally avoids the observed macOS/CoreS3 `serial2xsbug -install` hang by direct-flashing the generated XSA archive into the ESP32 `xs` MOD partition:

```text
offset: 0xfa0000
size:   0x40000
```

Python tooling for this direct flash path runs through `uv`:

```bash
uv run --with esptool esptool ...
```

If the upstream partition table changes, override with `TARS_STACKCHAN_MOD_OFFSET` and `TARS_STACKCHAN_MOD_SIZE`.

For host deploys, `scripts/dev/prepare-firmware-upload.sh` also patches upstream `stackchan/manifest_local.json` for the CoreS3/K151 hardware. This is important because the pinned upstream default sets `driver.type` to `none`, which makes servo-backed head and motion requests return successfully without physical movement. It also points Stack-chan remote TTS at `TARS_STACKCHAN_TTS_HOST:TARS_STACKCHAN_TTS_PORT`; when the host is omitted, the helper uses the Mac's `en0` IP when available.

For local speech testing, run the Gemini 3.1 Flash TTS relay on the Mac before asking Stack-chan to speak:

```bash
export GEMINI_API_KEY="<google-ai-studio-api-key>"
scripts/dev/run-local-tts.sh
```

The helper runs a small `uv`-launched HTTP server that calls the Gemini REST TTS endpoint, wraps the returned 24 kHz mono PCM as WAV, and serves it at `/api/tts?text=...`. Defaults are `TARS_STACKCHAN_TTS_MODEL=gemini-3.1-flash-tts-preview` and `TARS_STACKCHAN_TTS_VOICE=Kore`; the API key can come from `GEMINI_API_KEY` or `TARS_STACKCHAN_GEMINI_API_KEY`.

If the local API is not reachable immediately after flashing, run smoke through the project helper:

```bash
TARS_STACKCHAN_BASE_URL=http://stackchan.local scripts/dev/upload-firmware.sh smoke
```

When USB is still connected, the helper performs one hard-reset retry before failing. Disable it with `TARS_STACKCHAN_SMOKE_RETRY_USB_RESET=0`.

## Local Verification

The local automated verification for the firmware bridge is the MOD contract test:

```bash
scripts/test/firmware-bridge-contract.sh
```

The helper also guards the launch/config issues found during hardware upload:

- MOD config must be read from `mod/config`, not `mc/config`.
- The bridge exports `onLaunch()` to bypass the default setup UI touch probe on K151/CoreS3.
- HTTP listener/service modules are shipped inside the MOD so the server lifetime is retained with the bridge.
- MOD-owned modules use `tars-*` names, such as `tars-http-server-service`, to avoid resolving to host firmware modules with the same generic names.

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
  -d '{"text":"hello stack-chan"}' \
  http://stackchan.local/v1/speech
```

The head response should report `tilt_deg` as `85`. Pan is also clamped to the firmware overlay's safe `-90..90` degree range.
