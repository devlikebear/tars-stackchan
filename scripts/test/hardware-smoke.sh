#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
base_url="${TARS_STACKCHAN_BASE_URL:-http://stackchan.local}"
token="${TARS_STACKCHAN_TOKEN:-}"

if [ -z "$token" ]; then
  echo "TARS_STACKCHAN_TOKEN is required" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

run_curl() {
  echo "+ curl request"
  curl "$@"
  echo
}

call_tool() {
  tool_name="$1"
  arguments="$2"
  request="{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"$tool_name\",\"arguments\":$arguments}}"
  echo "+ MCP $tool_name"
  printf '%s\n' "$request" | \
    TARS_STACKCHAN_BRIDGE=http \
    TARS_STACKCHAN_BASE_URL="$base_url" \
    TARS_STACKCHAN_TOKEN="$token" \
    go run ./cmd/tars-stackchan-mcp
}

run_curl -fsS "$base_url/v1/status"

run_curl -fsS -X POST \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"emotion":"happy"}' \
  "$base_url/v1/expression"

run_curl -fsS -X POST \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"pan_deg":45,"tilt_deg":120,"speed":0.6}' \
  "$base_url/v1/head"

run_curl -fsS -X POST \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"pattern":"solid","color":"#00AEEF","brightness":0.5}' \
  "$base_url/v1/leds"

run_curl -fsS -X POST \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"name":"nod"}' \
  "$base_url/v1/motion"

cd "$repo_root/mcp-server"

call_tool stackchan_get_status '{}'
call_tool stackchan_set_expression '{"emotion":"happy"}'
call_tool stackchan_move_head '{"pan_deg":45,"tilt_deg":120,"speed":0.6}'
call_tool stackchan_set_led '{"pattern":"solid","color":"#00AEEF","brightness":0.5}'
call_tool stackchan_run_motion '{"name":"nod"}'
