# Phase 3: Firmware Local API Bridge

Goal: expose the Phase 2 `/v1` local HTTP API from Stack-chan firmware.

## Implementation Note

The current official Stack-chan firmware at `677224032e9ca25ac5c327b2eacd0034804b756f` is a Moddable SDK TypeScript/JavaScript firmware, not a plain ESP-IDF C/C++ component tree. Phase 3 is therefore implemented as a Stack-chan MOD overlay:

```text
firmware/stackchan/mods/tars_stackchan_bridge/
```

This keeps upstream recognizable and avoids rewriting the host firmware.

## Status

- Upstream source is referenced in `firmware/stackchan/upstream.lock`.
- Required patch/application steps are documented in `firmware/stackchan/patches/0001-install-tars-stackchan-bridge-mod.md`.
- The MOD starts `HttpServerService` from `onRobotCreated`, which upstream calls after Wi-Fi setup.
- Static bearer-token check is implemented for mutating endpoints.
- Implemented endpoints:
  - `GET /v1/status`
  - `POST /v1/expression`
  - `POST /v1/head`
  - `POST /v1/leds`
  - `POST /v1/motion`
- Firmware-side `tilt_deg` clamp is `5..85`.
- Firmware-side `pan_deg` clamp is `-90..90`.
- Unsafe `speed`, `brightness`, color, expression, and motion values are rejected before hardware calls.

## Verification

Local contract test:

```bash
node --test firmware/stackchan/mods/tars_stackchan_bridge/bridge-core.test.mjs
```

MCP server regression:

```bash
cd mcp-server
go test ./...
go build ./cmd/tars-stackchan-mcp
```

## Hardware Verification Pending

Full firmware build/flash requires the upstream Moddable/ESP32 toolchain and hardware:

```bash
cd stack-chan/firmware
npm run build
npm run deploy
npm run mod mods/tars_stackchan_bridge/manifest.json
```

`idf.py build` was not run locally because `idf.py` is not installed and the targeted upstream firmware does not use a direct `firmware/stackchan/idf.py` project layout.
