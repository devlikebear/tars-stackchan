#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"

assert_contains() {
  if ! grep -Fq -- "$2" "$1"; then
    echo "missing pattern in $1: $2" >&2
    exit 1
  fi
}

main_go="$repo_root/mcp-server/cmd/tars-stackchan-control/main.go"

perceive_go="$repo_root/mcp-server/cmd/tars-stackchan-control/perceive.go"
sink_go="$repo_root/mcp-server/internal/perception/sink_tars.go"

# perceive subcommand is wired into the control CLI dispatch.
assert_contains "$main_go" 'case "perceive":'
assert_contains "$main_go" "runPerceive(args[1:], stderr)"

# Phase 3: enroll subcommand + identity in the TARS webhook payload.
assert_contains "$perceive_go" 'case "enroll":'
assert_contains "$perceive_go" "--reset"
assert_contains "$sink_go" '`json:"identity"`'

# Audio-only real-HW mode (Spike S workaround): camera-capture gate.
assert_contains "$repo_root/mcp-server/internal/perception/config.go" "CameraEnabled"
assert_contains "$repo_root/mcp-server/internal/perception/loop.go" "if d.Config.CameraEnabled {"

# Perception package unit tests (trigger/debounce/rate-limit/sink/loop).
( cd "$repo_root/mcp-server" && go test ./internal/perception/... )

echo "perception-loop contract OK"
