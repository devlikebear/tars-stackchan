#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cmd_dir="$repo_root/mcp-server/cmd/tars-stackchan-host"
hostbody_dir="$repo_root/mcp-server/internal/hostbody"
goreleaser_config="$repo_root/.goreleaser.yaml"

assert_file() {
  if [ ! -f "$1" ]; then
    echo "missing file: $1" >&2
    exit 1
  fi
}

assert_contains() {
  if ! grep -Fq -- "$2" "$1"; then
    echo "missing pattern in $1: $2" >&2
    exit 1
  fi
}

assert_file "$cmd_dir/main.go"
assert_file "$hostbody_dir/tools.go"
assert_file "$hostbody_dir/capture.go"
assert_file "$hostbody_dir/speak.go"
assert_file "$hostbody_dir/sink.go"
assert_file "$hostbody_dir/mcp.go"

assert_contains "$cmd_dir/main.go" "tars-stackchan-host"
assert_contains "$cmd_dir/main.go" "host_speak"
assert_contains "$hostbody_dir/tools.go" "ProbeTools"
assert_contains "$hostbody_dir/capture.go" "CaptureAudio"
assert_contains "$hostbody_dir/capture.go" "CaptureCamera"
assert_contains "$hostbody_dir/speak.go" "TARS_STACKCHAN_HOST_TTS_BASE_URL"
assert_contains "$hostbody_dir/sink.go" "/v1/embodiment/percept/"
assert_contains "$hostbody_dir/mcp.go" "ToolHostSpeak"
assert_contains "$goreleaser_config" "tars-stackchan-host"

( cd "$repo_root/mcp-server" && go test ./internal/hostbody/... ./cmd/tars-stackchan-host )

echo "hostbody-smoke contract OK"
