#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
base_url="${TARS_STACKCHAN_BASE_URL:-http://stackchan.local}"
token="${TARS_STACKCHAN_TOKEN:-}"
connect_timeout="${TARS_STACKCHAN_SMOKE_CONNECT_TIMEOUT:-3}"
max_time="${TARS_STACKCHAN_SMOKE_MAX_TIME:-8}"
status_only="${TARS_STACKCHAN_SMOKE_STATUS_ONLY:-0}"

if [ -z "$token" ]; then
  echo "TARS_STACKCHAN_TOKEN is required" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

base_host="$(printf '%s' "$base_url" | sed 's#^[^:]*://##; s#/.*$##; s#:.*$##')"
base_port="$(printf '%s' "$base_url" | sed -n 's#^[^:]*://[^:/]*:\([0-9][0-9]*\).*#\1#p')"
if [ -z "$base_port" ]; then
  case "$base_url" in
    https://*) base_port=443 ;;
    *) base_port=80 ;;
  esac
fi

diagnose_reachability() {
  echo "diagnostics:"
  echo "  base_url: $base_url"
  echo "  host: $base_host"
  echo "  port: $base_port"

  if command -v dscacheutil >/dev/null 2>&1; then
    echo "+ dscacheutil -q host -a name $base_host"
    dscacheutil -q host -a name "$base_host" || true
  fi

  if command -v route >/dev/null 2>&1; then
    echo "+ route -n get $base_host"
    route -n get "$base_host" || true
  fi

  if command -v ping >/dev/null 2>&1; then
    echo "+ ping -c 1 -W 1000 $base_host"
    ping -c 1 -W 1000 "$base_host" || true
  fi

  if command -v nc >/dev/null 2>&1; then
    echo "+ nc -vz -G $connect_timeout $base_host $base_port"
    nc -vz -G "$connect_timeout" "$base_host" "$base_port" || true
  fi

  if command -v arp >/dev/null 2>&1; then
    echo "+ arp -n $base_host"
    arp -n "$base_host" || true
  fi
}

run_curl() {
  echo "+ curl request"
  if ! curl --connect-timeout "$connect_timeout" --max-time "$max_time" "$@"; then
    echo "hardware smoke request failed" >&2
    diagnose_reachability
    return 1
  fi
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

if [ "$status_only" = "1" ]; then
  exit 0
fi

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
