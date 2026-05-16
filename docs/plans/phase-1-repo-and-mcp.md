# Phase 1: Repository And MCP Server

Goal: create a standalone repository with a working Go MCP server that can run without hardware through a mock bridge.

## Status

- Repository scaffold exists.
- Go module path is `github.com/devlikebear/tars-stackchan/mcp-server`, matching the configured GitHub remote owner.
- Mock bridge is the default runtime bridge.
- MCP stdio server supports:
  - `initialize`
  - `notifications/initialized`
  - `ping`
  - `tools/list`
  - `tools/call`
- Tool argument validation is covered by tests.

## Verification

```bash
cd /Users/changheonshin/workspace/myworks/tars-stackchan/mcp-server
go test ./...
go build ./cmd/tars-stackchan-mcp
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | go run ./cmd/tars-stackchan-mcp
```

## Phase 1 Boundary

No firmware or hardware is required in this phase. The HTTP bridge and firmware-local API begin in Phase 2.
