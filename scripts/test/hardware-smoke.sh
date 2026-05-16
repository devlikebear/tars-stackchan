#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
base_url="${TARS_STACKCHAN_BASE_URL:-http://stackchan.local}"
token="${TARS_STACKCHAN_TOKEN:-}"
connect_timeout="${TARS_STACKCHAN_SMOKE_CONNECT_TIMEOUT:-3}"
max_time="${TARS_STACKCHAN_SMOKE_MAX_TIME:-8}"
status_only="${TARS_STACKCHAN_SMOKE_STATUS_ONLY:-0}"
skip_auth_checks="${TARS_STACKCHAN_SMOKE_SKIP_AUTH_CHECKS:-0}"
last_response=""
last_status=""
tmp_dir="${TMPDIR:-/tmp}/tars-stackchan-smoke.$$"
mkdir -p "$tmp_dir"
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

if [ -z "$token" ]; then
  echo "TARS_STACKCHAN_TOKEN is required" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

assert_contains_text() {
  label="$1"
  text="$2"
  pattern="$3"
  case "$text" in
    *"$pattern"*) ;;
    *)
      echo "$label did not contain expected text: $pattern" >&2
      echo "$text" >&2
      return 1
      ;;
  esac
}

print_troubleshooting_hints() {
  echo "hints:"
  echo "  - If nc succeeds but curl receives 0 bytes, attach xsbug/serial2xsbug and inspect the JS stack."
  echo "  - A common cause is a MOD module-name collision with host firmware modules."
  echo "  - The bridge should import tars-http-server-service, not http-server-service."
  echo "  - If DNS fails for stackchan.local, retry with the IP printed in the firmware log."
}

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

  print_troubleshooting_hints
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

run_curl_capture() {
  response_file="$tmp_dir/response"
  echo "+ curl request"
  if ! curl --connect-timeout "$connect_timeout" --max-time "$max_time" -o "$response_file" "$@"; then
    echo "hardware smoke request failed" >&2
    diagnose_reachability
    return 1
  fi
  last_response="$(cat "$response_file")"
  printf '%s\n' "$last_response"
}

expect_http_status() {
  label="$1"
  expected="$2"
  shift 2
  response_file="$tmp_dir/response"
  echo "+ curl $label expecting HTTP $expected"
  last_status="$(curl --connect-timeout "$connect_timeout" --max-time "$max_time" -sS -o "$response_file" -w "%{http_code}" "$@" || true)"
  last_response="$(cat "$response_file" 2>/dev/null || true)"
  if [ -n "$last_response" ]; then
    printf '%s\n' "$last_response"
  fi
  echo "HTTP $last_status"
  if [ "$last_status" != "$expected" ]; then
    echo "$label returned HTTP $last_status, expected $expected" >&2
    diagnose_reachability
    return 1
  fi
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

run_curl_capture -fsS "$base_url/v1/status"
assert_contains_text "status response" "$last_response" '"connected":true'
assert_contains_text "status response" "$last_response" '"device":"stackchan-k151"'

if [ "$status_only" = "1" ]; then
  exit 0
fi

if [ "$skip_auth_checks" != "1" ]; then
  expect_http_status "missing token rejection" 401 -X POST \
    -H "Content-Type: application/json" \
    -d '{"emotion":"happy"}' \
    "$base_url/v1/expression"
  assert_contains_text "missing token response" "$last_response" "invalid token"

  expect_http_status "invalid token rejection" 401 -X POST \
    -H "Authorization: Bearer invalid-token" \
    -H "Content-Type: application/json" \
    -d '{"emotion":"happy"}' \
    "$base_url/v1/expression"
  assert_contains_text "invalid token response" "$last_response" "invalid token"
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
