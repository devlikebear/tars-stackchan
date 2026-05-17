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
bodyprovider_go="$repo_root/mcp-server/internal/bodyprovider/contract.go"
stackchan_adapter_go="$repo_root/mcp-server/internal/bodyprovider/stackchan_adapter.go"
loop_go="$repo_root/mcp-server/internal/perception/loop.go"
tars_config="$repo_root/mcp-server/examples/tars/tars.config.yaml"

# perceive subcommand is wired into the control CLI dispatch.
assert_contains "$main_go" 'case "perceive":'
assert_contains "$main_go" "runPerceive(args[1:], stderr)"

# Phase 3: enroll subcommand + identity in the TARS webhook payload.
assert_contains "$perceive_go" 'case "enroll":'
assert_contains "$perceive_go" "--reset"
assert_contains "$sink_go" '`json:"identity"`'

# Phase 4 embodiment provider contract: perception and actuation are exposed as
# body-provider-neutral types, while Stack-chan remains the first adapter.
assert_contains "$bodyprovider_go" "type Provider interface"
assert_contains "$bodyprovider_go" "Actuate(context.Context, BodyAction)"
assert_contains "$bodyprovider_go" "CapturePercept(context.Context, CaptureOptions)"
assert_contains "$stackchan_adapter_go" "func NewStackChanProvider"
assert_contains "$loop_go" "Bridge     bodyprovider.PerceptionBridge"
assert_contains "$sink_go" '`json:"x-embodiment"`'
assert_contains "$sink_go" '`json:"media_ref,omitempty"`'
assert_contains "$tars_config" "embodiment:"
assert_contains "$tars_config" "endpoint: tars-stackchan"
assert_contains "$tars_config" "- speech"

# Audio-only real-HW mode (Spike S workaround): camera-capture gate.
assert_contains "$repo_root/mcp-server/internal/perception/config.go" "CameraEnabled"
assert_contains "$loop_go" "if d.Config.CameraEnabled {"

# Perception package unit tests (trigger/debounce/rate-limit/sink/loop).
( cd "$repo_root/mcp-server" && go test ./internal/bodyprovider/... ./internal/perception/... )

echo "perception-loop contract OK"
