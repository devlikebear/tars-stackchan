#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"

uv run python "$repo_root/scripts/test/tts-remote-server-contract.py"
