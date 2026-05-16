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
```

Mutating requests require:

```http
Authorization: Bearer <token>
```

The token is configured in `manifest.json` under `config.tarsStackchan.token`.

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

Follow the upstream Moddable setup first:

```bash
cd firmware
npm run setup
npm run setup -- --device=esp32
```

Deploy the host firmware:

```bash
npm run deploy
```

Then flash this bridge as a MOD:

```bash
npm run mod mods/tars_stackchan_bridge/manifest.json
```

For a CoreS3/K151 target, use the upstream target override if needed:

```bash
npm_config_target=esp32/m5stack_cores3 npm run deploy
npm_config_target=esp32/m5stack_cores3 npm run mod mods/tars_stackchan_bridge/manifest.json
```

## Local Verification

The host machine used for this implementation does not currently have `idf.py` installed, and this upstream firmware is Moddable-based. The local automated verification for the firmware bridge is therefore the MOD contract test:

```bash
node --test firmware/stackchan/mods/tars_stackchan_bridge/bridge-core.test.mjs
```

Run the full firmware build on a machine with the upstream Moddable/ESP32 toolchain installed:

```bash
cd stack-chan/firmware
npm run build
npm run mod mods/tars_stackchan_bridge/manifest.json
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
```

The head response should report `tilt_deg` as `85`. Pan is also clamped to the firmware overlay's safe `-90..90` degree range.
