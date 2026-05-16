#!/usr/bin/env sh
set -eu

# Contract test for the Go Gemini TTS relay (tars-stackchan-control tts serve).
# Asserts the offline-observable behavior only (no Gemini network calls):
#   - GET /health           -> 200 "ok"
#   - GET /api/tts no token  -> 401
# This mirrors scripts/test/tts-remote-server-contract.* for the Python relay.

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
host="127.0.0.1"
port="18099"
token="contract-token"

bin_dir="$(mktemp -d)"
bin="$bin_dir/tars-stackchan-control"

cleanup() {
  if [ -n "${server_pid:-}" ]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$bin_dir"
}
trap cleanup EXIT INT TERM

( cd "$repo_root/mcp-server" && go build -o "$bin" ./cmd/tars-stackchan-control )

TARS_STACKCHAN_TTS_CACHE="$bin_dir/cache" \
  "$bin" tts serve --host "$host" --port "$port" --token "$token" --api-key "unused" &
server_pid=$!

# Wait for the listener to come up (max ~10s).
ready=0
i=0
while [ "$i" -lt 50 ]; do
  if curl -fsS "http://$host:$port/health" >/dev/null 2>&1; then
    ready=1
    break
  fi
  i=$((i + 1))
  sleep 0.2
done
if [ "$ready" -ne 1 ]; then
  echo "relay did not become ready on http://$host:$port" >&2
  exit 1
fi

health="$(curl -fsS "http://$host:$port/health")"
if [ "$health" != "ok" ]; then
  echo "GET /health body = '$health', want 'ok'" >&2
  exit 1
fi

code="$(curl -s -o /dev/null -w '%{http_code}' "http://$host:$port/api/tts?text=hi")"
if [ "$code" != "401" ]; then
  echo "GET /api/tts without token -> $code, want 401" >&2
  exit 1
fi

echo "tts-relay-go-contract: OK"
