# Patch: Install TARS Stack-chan Bridge MOD

This patch note describes how to apply the project-owned bridge overlay to the official Stack-chan firmware.

## Upstream Base

```text
repository: https://github.com/stack-chan/stack-chan
commit: 677224032e9ca25ac5c327b2eacd0034804b756f
```

The official firmware at this commit is a Moddable SDK project. It already connects Wi-Fi before loading a runtime MOD, and it exposes `HttpServerService` plus the `Robot` facade needed for face, servo, LED, and motion control.

## Files To Add

Copy:

```text
/Users/changheonshin/workspace/myworks/tars-stackchan/firmware/stackchan/mods/tars_stackchan_bridge
```

to the upstream checkout at:

```text
firmware/mods/tars_stackchan_bridge
```

## Configuration

Edit:

```text
firmware/mods/tars_stackchan_bridge/manifest.json
```

Set:

```json
{
  "config": {
    "tarsStackchan": {
      "token": "your-local-token",
      "port": 80,
      "device": "stackchan-k151",
      "firmware": "tars-stackchan-dev",
      "ledName": "a"
    }
  }
}
```

## Build And Flash

```bash
cd firmware
npm run setup
npm run setup -- --device=esp32
npm run deploy
npm run mod mods/tars_stackchan_bridge/manifest.json
```

For CoreS3/K151:

```bash
npm_config_target=esp32/m5stack_cores3 npm run deploy
npm_config_target=esp32/m5stack_cores3 npm run mod mods/tars_stackchan_bridge/manifest.json
```

## Verification

```bash
curl http://stackchan.local/v1/status

curl -X POST \
  -H "Authorization: Bearer $TARS_STACKCHAN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pan_deg":45,"tilt_deg":120,"speed":0.6}' \
  http://stackchan.local/v1/head
```

The head response must report `tilt_deg` as `85`, proving the firmware-side servo clamp is active.
