# TARS Stack-chan Bridge

TARS Stack-chan Bridge lets local AI agents control an M5Stack Stack-chan K151 robot through MCP tools.

The MVP architecture is local-first:

```text
AI Agent / TARS / Claude
  -> MCP client
    -> tars-stackchan MCP server
      -> local HTTP API over Wi-Fi
        -> Stack-chan firmware bridge
          -> expression / head servo / LED / motion
```

Phase 1 contains a Go MCP server with a mock bridge, so it can be tested without hardware or firmware changes.

## Current Scope

Implemented MCP tools:

- `stackchan_get_status`
- `stackchan_set_expression`
- `stackchan_move_head`
- `stackchan_set_led`
- `stackchan_run_motion`

Safety and validation included in the server:

- Head tilt is clamped to `5..85` degrees.
- Expressions are limited to a known allowlist.
- LED colors must use `#RRGGBB`.
- Tool arguments reject unknown JSON fields.

## Quickstart

```bash
cd mcp-server
go test ./...
go build ./cmd/tars-stackchan-mcp
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | go run ./cmd/tars-stackchan-mcp
```

The default bridge is the in-memory mock bridge. It reports a connected `stackchan-k151` device and records expression, head, LED, and motion requests locally.

## Example Tool Call

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"stackchan_move_head","arguments":{"pan_deg":45,"tilt_deg":120,"speed":0.6}}}' \
  | go run ./cmd/tars-stackchan-mcp
```

The response shows `tilt_deg` clamped to `85`.

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

## Next Phase

Phase 2 will add the real HTTP bridge and the shared local firmware API protocol document. Firmware work starts after that protocol is locked.
