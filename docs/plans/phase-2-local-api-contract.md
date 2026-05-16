# Phase 2: Local HTTP Bridge And Protocol Docs

Goal: implement the MCP server side of the real bridge against a documented local firmware API.

## Status

- `docs/protocol/local-control-api.md` defines the shared `/v1` API.
- `TARS_STACKCHAN_BRIDGE=mock|http` selects the bridge.
- `TARS_STACKCHAN_BASE_URL` configures the Stack-chan firmware base URL.
- `TARS_STACKCHAN_TOKEN` configures the bearer token for mutating requests.
- Mock bridge remains the default.
- HTTP bridge implements:
  - `GET /v1/status`
  - `POST /v1/expression`
  - `POST /v1/head`
  - `POST /v1/leds`
  - `POST /v1/motion`

## Verification

```bash
cd /Users/changheonshin/workspace/myworks/tars-stackchan/mcp-server
go test ./...
TARS_STACKCHAN_BRIDGE=http TARS_STACKCHAN_BASE_URL=http://127.0.0.1:9999 go test ./...
```

## Phase 2 Boundary

Firmware implementation is intentionally out of scope for this phase. The HTTP bridge is tested with `httptest.Server`.
