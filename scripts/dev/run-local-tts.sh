#!/usr/bin/env sh
set -eu

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

exec env TARS_STACKCHAN_TTS_TOKEN="$tts_token" uv run python scripts/dev/tts-remote-server.py --host 0.0.0.0 --port "$port" --model "$model" --voice "$voice"
