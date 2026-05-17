#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
script="$repo_root/scripts/dev/discover-base-url.sh"

fail() {
  echo "$1" >&2
  exit 1
}

assert_contains() {
  file="$1"
  pattern="$2"
  if ! grep -Fq -- "$pattern" "$file"; then
    fail "missing pattern in $file: $pattern"
  fi
}

[ -x "$script" ] || fail "missing executable script: $script"
assert_contains "$script" "TARS_STACKCHAN_UPLOAD_PORT"
assert_contains "$script" "TARS_STACKCHAN_DISCOVER_BAUD"
assert_contains "$script" "TARS_STACKCHAN_DISCOVER_RESET"
assert_contains "$script" "python3"
assert_contains "$script" "/v1/status"

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

mkdir -p "$tmp_dir/bin"
cat >"$tmp_dir/bin/curl" <<'SH'
#!/usr/bin/env sh
set -eu
url=""
for arg in "$@"; do
  case "$arg" in
    http://*) url="$arg" ;;
  esac
done
case "$url" in
  http://stackchan.local/v1/status)
    printf '%s\n' '{"connected":true,"device":"stackchan-k151","ip":"192.168.10.20"}'
    ;;
  http://192.168.10.20/v1/status)
    printf '%s\n' '{"connected":true,"device":"stackchan-k151","ip":"192.168.10.20"}'
    ;;
  http://192.168.10.30/v1/status)
    printf '%s\n' '{"connected":true,"device":"stackchan-k151","ip":"192.168.10.30"}'
    ;;
  *)
    exit 7
    ;;
esac
SH
chmod +x "$tmp_dir/bin/curl"

from_mdns="$(
  PATH="$tmp_dir/bin:$PATH" \
  TARS_STACKCHAN_BASE_URL= \
  TARS_STACKCHAN_DISCOVER_HOSTS=http://stackchan.local \
  "$script" --url-only
)"
[ "$from_mdns" = "http://192.168.10.20" ] || fail "from_mdns = $from_mdns, want http://192.168.10.20"

cat >"$tmp_dir/serial.log" <<'EOF'
[wifi] connected
[tars-stackchan] local control API listening on port 80
[net] ip: 192.168.10.20
EOF

discovered="$(
  PATH="$tmp_dir/bin:$PATH" \
  TARS_STACKCHAN_BASE_URL= \
  TARS_STACKCHAN_DISCOVER_HOSTS=http://missing.local \
  TARS_STACKCHAN_DISCOVER_LOG_FILE="$tmp_dir/serial.log" \
  "$script" --url-only
)"
[ "$discovered" = "http://192.168.10.20" ] || fail "discovered = $discovered, want http://192.168.10.20"

explicit="$(
  PATH="$tmp_dir/bin:$PATH" \
  TARS_STACKCHAN_BASE_URL=http://192.168.10.30 \
  TARS_STACKCHAN_DISCOVER_LOG_FILE="$tmp_dir/serial.log" \
  "$script" --url-only
)"
[ "$explicit" = "http://192.168.10.30" ] || fail "explicit = $explicit, want http://192.168.10.30"

echo "discover-base-url-contract: OK"
