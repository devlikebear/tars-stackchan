# TARS Stack-chan Bridge

TARS Stack-chan Bridge lets local AI agents control an M5Stack Stack-chan K151/CoreS3 through MCP tools, a local HTTP firmware bridge, and an optional browser control console.

The project is local-first:

```text
AI agent / TARS / Claude Code
  -> stdio MCP server
    -> local HTTP API over Wi-Fi
      -> Stack-chan firmware MOD
        -> expression / head / LED / motion / speech
```

The MCP server can run against a mock bridge for development or an HTTP bridge for real hardware.

## Install

Released macOS/Linux binaries are published through GitHub Releases and Homebrew.

```bash
brew tap devlikebear/tap
brew install tars-stackchan
```

Set the local firmware bearer token before connecting a real device:

```bash
export TARS_STACKCHAN_TOKEN="<local-token>"
```

Connect Claude Code:

```bash
tars-stackchan-mcp install --target claude-code
```

For a copyable command or config snippet instead of writing client config:

```bash
tars-stackchan-mcp config --target claude-code
tars-stackchan-mcp config --target claude-desktop
tars-stackchan-mcp config --target tars
```

Check the local setup:

```bash
tars-stackchan-mcp doctor
```

### Device address

Examples below use `http://stackchan.local`, but `*.local` mDNS does not
resolve in many networks. If a probe fails, set `TARS_STACKCHAN_BASE_URL` to
the device IP printed in the firmware boot log or returned by
`GET /v1/status` (DHCP addresses can change between boots). `doctor` prints an
actionable hint when the probe fails, and warns when it is running against the
mock bridge instead of real hardware.

The firmware bearer token is flashed into the bridge MOD and is not
recoverable afterwards. `GET /v1/status` is unauthenticated, so `doctor` can
report `device: connected` while mutating calls still fail with HTTP 401 if
`TARS_STACKCHAN_TOKEN` does not match the flashed token. Re-flash the MOD with
a known token to resync.

## MCP Tools

The default MCP tools are:

- `stackchan_get_status`
- `stackchan_set_expression`
- `stackchan_move_head`
- `stackchan_set_led`
- `stackchan_run_motion`
- `stackchan_speak`

Safety rules enforced by the server:

- Head tilt is clamped to `5..85` degrees.
- Expressions and LED patterns use allowlists.
- LED colors must use `#RRGGBB`.
- Speech text is required and capped at 240 characters.
- Speech volume is optional and normalized to `0.0..1.0`.
- Tool arguments reject unknown JSON fields.

## Firmware Upload Tool

The firmware build/upload tool is intentionally opt-in because it can flash connected hardware.

```bash
tars-stackchan-mcp install --target claude-code --firmware-tools
```

That exposes:

- `stackchan_upload_firmware`

Supported modes:

- `prepare`
- `deps`
- `host`
- `mod`
- `smoke`
- `all`

The default mode is `mod`, which prepares the pinned upstream firmware checkout, builds the TARS bridge MOD, and direct-flashes the ESP32 `xs` MOD partition through `scripts/dev/upload-firmware.sh`.

Manual run example:

```bash
export TARS_STACKCHAN_TOKEN="<local-token>"
export TARS_STACKCHAN_UPLOAD_PORT=/dev/cu.usbmodem1101
scripts/dev/upload-firmware.sh mod
```

For a full host firmware deploy plus MOD flash:

```bash
TARS_STACKCHAN_DEPLOY_HOST=1 scripts/dev/upload-firmware.sh all
```

See [firmware/README.md](firmware/README.md) for firmware and TTS details, and [docs/hardware-smoke.md](docs/hardware-smoke.md) for the validated hardware smoke flow.

## Local Development

Run the Go test suite:

```bash
cd mcp-server
go test ./...
```

List tools with the mock bridge:

```bash
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' \
  | go run ./cmd/tars-stackchan-mcp
```

Run against real firmware:

```bash
TARS_STACKCHAN_BRIDGE=http \
TARS_STACKCHAN_BASE_URL=http://stackchan.local \
TARS_STACKCHAN_TOKEN="$TARS_STACKCHAN_TOKEN" \
go run ./cmd/tars-stackchan-mcp
```

Expose the optional firmware upload tool locally:

```bash
TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS=1 \
TARS_STACKCHAN_BRIDGE=http \
TARS_STACKCHAN_BASE_URL=http://stackchan.local \
TARS_STACKCHAN_TOKEN="$TARS_STACKCHAN_TOKEN" \
go run ./cmd/tars-stackchan-mcp
```

The shared firmware protocol is documented in [docs/protocol/local-control-api.md](docs/protocol/local-control-api.md).

## Local Control Console

Run the browser-based control panel before connecting an AI client:

```bash
cd mcp-server
TARS_STACKCHAN_BASE_URL=http://stackchan.local \
TARS_STACKCHAN_TOKEN="$TARS_STACKCHAN_TOKEN" \
go run ./cmd/tars-stackchan-control
```

Open `http://127.0.0.1:8787`. The console exposes status, expression, head, LED, motion, speech text, and speech volume through the same bridge contract as the MCP server.

It also exposes two Embodied Bot endpoints for external callers (claude code / TARS / scripts):

```bash
# One call -> a monitor "emotion" (expression + LED + optional motion preset)
curl -X POST -H 'Content-Type: application/json' \
  -d '{"emotion":"happy"}' http://127.0.0.1:8787/api/emotion
# allowed: happy sad angry surprised sleepy neutral blink excited calm
# add {"motion":false} to suppress the preset motion

# Read-only perception/owner/TARS visibility (no secrets)
curl http://127.0.0.1:8787/api/perceive/status
```

## Embodied Bot (perception loop)

The `perceive` subcommand turns Stack-chan into a sensory front-end: it polls
the device, captures audio (and camera, hardware permitting), labels the
moment against an enrolled owner, summarizes it, and posts a compact
observation to TARS (the brain). Run order:

```bash
# 1. (optional) fingerprint the owner so the bot tells owner from stranger
tars-stackchan-control perceive enroll --name me

# 2. run the loop (audio-only on real CoreS3 until Spike S is fixed)
TARS_STACKCHAN_BRIDGE=http TARS_STACKCHAN_BASE_URL=http://<device-ip> \
TARS_STACKCHAN_PERCEIVE_CAMERA=off \
TARS_STACKCHAN_TARS_BASE_URL=http://127.0.0.1:43180 \
TARS_STACKCHAN_TARS_WEBHOOK_CHANNEL=stackchan \
tars-stackchan-control perceive serve
```

Architecture (role split): tars-stackchan is the body (sensors + actuation),
TARS is the brain (LLM + memory + persona). See `docs/plans/embodied-bot-roadmap.md`.

> **Camera status:** on real M5Stack CoreS3, camera capture currently resets
> the device — the camera SCCB shares the internal I2C bus already owned by
> the Moddable peripheral driver (`docs/plans/embodied-bot-phase-1-spike.md`,
> Spike S). The microphone (I2S) is unaffected and verified on hardware, so
> run with `TARS_STACKCHAN_PERCEIVE_CAMERA=off` for audio-only operation.

For UI-only development without hardware:

```bash
TARS_STACKCHAN_BRIDGE=mock go run ./cmd/tars-stackchan-control
```

Override the listen address with `TARS_STACKCHAN_CONTROL_ADDR`, for example:

```bash
TARS_STACKCHAN_CONTROL_ADDR=127.0.0.1:8790 go run ./cmd/tars-stackchan-control
```

## Speech

Speech uses Stack-chan remote TTS pointed at the local Gemini relay. The relay
is the Go binary and runs as a Homebrew service; it advertises
`tars-stackchan-tts.local` over mDNS so the device finds it without a re-flash
when the Mac's DHCP address changes:

launchd does not read your shell profile, so set the secrets in the launchd
session (a plain `export` is not enough for the service):

```bash
launchctl setenv GEMINI_API_KEY "<google-ai-studio-api-key>"
launchctl setenv TARS_STACKCHAN_TOKEN "<local-token>"
brew services restart tars-stackchan
tars-stackchan-control tts status   # relay + mDNS health

# dev alternative (foreground, reads shell env): tars-stackchan-control tts serve
```

`launchctl setenv` does not survive a reboot; run those two lines from a
login item (e.g. a per-user LaunchAgent) to persist them.

Defaults:

- `TARS_STACKCHAN_TTS_MODEL=gemini-3.1-flash-tts-preview`
- `TARS_STACKCHAN_TTS_VOICE=Kore`
- `TARS_STACKCHAN_TTS_PORT=18080`

The relay requires `TARS_STACKCHAN_TTS_TOKEN` or `TARS_STACKCHAN_TOKEN` and
redacts tokens from logs. If mDNS does not resolve on your network, re-flash
with `TARS_STACKCHAN_TTS_HOST=<mac-ip>`. Verify with `tars-stackchan-mcp doctor`.

## Verification

Useful local checks:

```bash
cd mcp-server && go test ./...
scripts/test/firmware-bridge-contract.sh
scripts/test/firmware-upload-script-contract.sh
scripts/test/hardware-smoke-contract.sh
scripts/test/tts-relay-go-contract.sh
scripts/test/release-config-contract.sh
```

Build and release configuration:

```bash
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean --skip=publish
```

## Release

CI runs on `main` and pull requests. Tags matching `v*` run GoReleaser, publish GitHub release archives, and update `devlikebear/homebrew-tap`.

Required repository secrets:

- `GITHUB_TOKEN`: provided by GitHub Actions for release asset publishing.
- `TAP_GITHUB_TOKEN`: token with write access to `devlikebear/homebrew-tap`.

The Homebrew formula installs both binaries:

- `tars-stackchan-mcp`
- `tars-stackchan-control`

It also packages `firmware/` and `scripts/` under the formula share directory so `stackchan_upload_firmware` can find the upload helper after Homebrew installation.

## Repository Layout

```text
tars-stackchan/
  README.md
  .github/workflows/
  .goreleaser.yaml
  docs/
    hardware-smoke.md
    protocol/local-control-api.md
  firmware/
    README.md
    stackchan/
  mcp-server/
    cmd/tars-stackchan-mcp/
    cmd/tars-stackchan-control/
    internal/
  scripts/
    dev/
    test/
```
