#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
port="${TARS_STACKCHAN_TTS_PORT:-18080}"
model="${TARS_STACKCHAN_TTS_MODEL:-gemini-3.1-flash-tts-preview}"
voice="${TARS_STACKCHAN_TTS_VOICE:-Kore}"

cd "$repo_root"
exec uv run python scripts/dev/tts-remote-server.py --host 0.0.0.0 --port "$port" --model "$model" --voice "$voice"
