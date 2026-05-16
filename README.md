# TARS Stack-chan Bridge

TARS Stack-chan Bridge lets local AI agents control an M5Stack Stack-chan K151 robot through MCP tools.

The MVP architecture is local-first:

```text
AI Agent / TARS / Claude
  -> MCP client
    -> tars-stackchan MCP server
      -> local HTTP API over Wi-Fi
        -> Stack-chan firmware bridge
          -> expression / head servo / LED / motion / speech
```

The server supports a mock bridge for local development and an HTTP bridge for real firmware control.

## Current Scope

Implemented MCP tools:

- `stackchan_get_status`
- `stackchan_set_expression`
- `stackchan_move_head`
- `stackchan_set_led`
- `stackchan_run_motion`
- `stackchan_speak`

Safety and validation included in the server:

- Head tilt is clamped to `5..85` degrees.
- Expressions are limited to a known allowlist.
- LED colors must use `#RRGGBB`.
- Speech text is required and capped at 240 characters.
- Tool arguments reject unknown JSON fields.

## Quickstart

```bash
cd mcp-server
go test ./...
go build ./cmd/tars-stackchan-mcp
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | go run ./cmd/tars-stackchan-mcp
```

The default bridge is the in-memory mock bridge. It reports a connected `stackchan-k151` device and records expression, head, LED, motion, and speech requests locally.

## Bridge Configuration

Mock mode is the default:

```bash
TARS_STACKCHAN_BRIDGE=mock go run ./cmd/tars-stackchan-mcp
```

HTTP mode calls the firmware-local `/v1` API:

```bash
TARS_STACKCHAN_BRIDGE=http \
TARS_STACKCHAN_BASE_URL=http://stackchan.local \
TARS_STACKCHAN_TOKEN="$TARS_STACKCHAN_TOKEN" \
go run ./cmd/tars-stackchan-mcp
```

The shared protocol is documented in [docs/protocol/local-control-api.md](docs/protocol/local-control-api.md).

## Example Tool Call

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"stackchan_move_head","arguments":{"pan_deg":45,"tilt_deg":120,"speed":0.6}}}' \
  | go run ./cmd/tars-stackchan-mcp
```

The response shows `tilt_deg` clamped to `85`.

## Local Control GUI

Run the browser-based control panel before connecting an AI client:

```bash
cd mcp-server
TARS_STACKCHAN_BASE_URL=http://stackchan.local \
TARS_STACKCHAN_TOKEN="$TARS_STACKCHAN_TOKEN" \
go run ./cmd/tars-stackchan-control
```

Open `http://127.0.0.1:8787`. The panel exposes status, expression, head, LED, motion, and speech controls through the same bridge contract as the MCP server.

For UI-only development without hardware:

```bash
TARS_STACKCHAN_BRIDGE=mock go run ./cmd/tars-stackchan-control
```

Override the listen address with `TARS_STACKCHAN_CONTROL_ADDR`, for example `TARS_STACKCHAN_CONTROL_ADDR=127.0.0.1:8790`.

## Repository Layout

```text
tars-stackchan/
  README.md
  docs/
    plans/
  mcp-server/
    cmd/tars-stackchan-mcp/
    internal/stackchan/
    internal/bridge/mock/
    examples/
      claude-desktop/
      tars/
  firmware/
  scripts/
```

## Current Phase

Phase 4 is prepared with upload and hardware smoke helpers. See [docs/hardware-smoke.md](docs/hardware-smoke.md).
