#!/usr/bin/env sh
set -eu

# DEPRECATED dev wrapper. The Gemini TTS relay is now the Go binary
# `tars-stackchan-control tts serve`, normally run as a Homebrew service:
#
#   brew services start tars-stackchan
#
# This script remains only as a thin shim for local development and is
# removed in Phase 4. It resolves the relay token, then execs the Go relay
# (preferring an installed binary, falling back to `go run`).

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
port="${TARS_STACKCHAN_TTS_PORT:-18080}"
model="${TARS_STACKCHAN_TTS_MODEL:-gemini-3.1-flash-tts-preview}"
voice="${TARS_STACKCHAN_TTS_VOICE:-Kore}"
tts_token="${TARS_STACKCHAN_TTS_TOKEN:-${TARS_STACKCHAN_TOKEN:-}}"

cd "$repo_root"

if [ -z "$tts_token" ] && command -v node >/dev/null 2>&1 && [ -f ".work/stack-chan/firmware/mods/tars_stackchan_bridge/manifest.json" ]; then
  tts_token="$(node -e "const m=require('./.work/stack-chan/firmware/mods/tars_stackchan_bridge/manifest.json'); const t=m.config?.tarsStackchan?.token || ''; process.stdout.write(t === 'replace-with-local-token' ? '' : t)")"
fi

if [ -z "$tts_token" ]; then
  echo "error: TARS_STACKCHAN_TTS_TOKEN or TARS_STACKCHAN_TOKEN is required" >&2
  exit 1
fi

echo "deprecated: run-local-tts.sh -> use 'brew services start tars-stackchan' (running the Go relay via shim)" >&2

if command -v tars-stackchan-control >/dev/null 2>&1; then
  relay="tars-stackchan-control"
else
  relay="go run ./cmd/tars-stackchan-control"
  cd "$repo_root/mcp-server"
fi

# shellcheck disable=SC2086
exec env TARS_STACKCHAN_TTS_TOKEN="$tts_token" $relay tts serve \
  --host 0.0.0.0 --port "$port" --model "$model" --voice "$voice" "$@"
