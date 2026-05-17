#!/usr/bin/env sh
set -eu

connect_timeout="${TARS_STACKCHAN_DISCOVER_CONNECT_TIMEOUT:-1}"
max_time="${TARS_STACKCHAN_DISCOVER_MAX_TIME:-2}"
serial_timeout="${TARS_STACKCHAN_DISCOVER_TIMEOUT:-8}"
serial_baud="${TARS_STACKCHAN_DISCOVER_BAUD:-115200}"
reset_before_read="${TARS_STACKCHAN_DISCOVER_RESET:-0}"
chip="${TARS_STACKCHAN_CHIP:-esp32s3}"
upload_port="${TARS_STACKCHAN_UPLOAD_PORT:-}"
hosts="${TARS_STACKCHAN_DISCOVER_HOSTS:-http://stackchan.local}"
log_file="${TARS_STACKCHAN_DISCOVER_LOG_FILE:-}"
mode="url"

usage() {
  cat <<EOF
Usage: scripts/dev/discover-base-url.sh [--url-only|--export]

Discovers a reachable Stack-chan local API URL. It probes explicit
TARS_STACKCHAN_BASE_URL first, then stackchan.local, then IPs parsed from a
serial log or USB serial output. Candidate IPs are accepted only when
GET /v1/status succeeds.

Environment:
  TARS_STACKCHAN_BASE_URL              explicit URL to probe first
  TARS_STACKCHAN_UPLOAD_PORT           USB serial port to read, otherwise auto-detect
  TARS_STACKCHAN_DISCOVER_HOSTS        space-separated mDNS/host candidates
  TARS_STACKCHAN_DISCOVER_LOG_FILE     parse IPs from this file before USB
  TARS_STACKCHAN_DISCOVER_BAUD         USB serial baud, default 115200
  TARS_STACKCHAN_DISCOVER_TIMEOUT      seconds to read USB serial, default 8
  TARS_STACKCHAN_DISCOVER_RESET=1      reset over esptool before reading serial
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --url-only)
      mode="url"
      ;;
    --export)
      mode="export"
      ;;
    -h|--help|help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
  shift
done

log() {
  echo "discover-base-url: $1" >&2
}

normalize_url() {
  url="$(printf '%s' "$1" | sed 's/[[:space:]]//g; s#/*$##')"
  if [ -z "$url" ]; then
    return 0
  fi
  case "$url" in
    http://*|https://*) printf '%s\n' "$url" ;;
    *) printf 'http://%s\n' "$url" ;;
  esac
}

valid_ipv4() {
  printf '%s\n' "$1" | awk -F. '
    NF != 4 { exit 1 }
    {
      for (i = 1; i <= 4; i++) {
        if ($i !~ /^[0-9]+$/ || $i < 0 || $i > 255) exit 1
      }
      if ($1 == 0 || $1 == 127 || $1 == 255) exit 1
      print $0
    }
  '
}

extract_status_ip() {
  if command -v node >/dev/null 2>&1; then
    node -e "
let input = ''
process.stdin.setEncoding('utf8')
process.stdin.on('data', (chunk) => { input += chunk })
process.stdin.on('end', () => {
  try {
    const parsed = JSON.parse(input)
    if (typeof parsed.ip === 'string') process.stdout.write(parsed.ip)
  } catch (_error) {}
})
"
  else
    sed -n 's/.*"ip"[[:space:]]*:[[:space:]]*"\([0-9.][0-9.]*\)".*/\1/p' | head -n 1
  fi
}

probe_url() {
  candidate="$(normalize_url "$1")"
  [ -n "$candidate" ] || return 1

  status_url="$candidate/v1/status"
  body="$(curl -fsS --connect-timeout "$connect_timeout" --max-time "$max_time" "$status_url" 2>/dev/null)" || return 1
  case "$body" in
    *'"connected":true'*|*'"connected": true'*) ;;
    *) return 1 ;;
  esac

  status_ip="$(printf '%s' "$body" | extract_status_ip || true)"
  if [ -n "$status_ip" ] && valid_ipv4 "$status_ip" >/dev/null 2>&1; then
    printf 'http://%s\n' "$status_ip"
  else
    printf '%s\n' "$candidate"
  fi
}

emit_url() {
  case "$mode" in
    export) printf 'export TARS_STACKCHAN_BASE_URL=%s\n' "$1" ;;
    *) printf '%s\n' "$1" ;;
  esac
}

tmp_dir="$(mktemp -d)"
candidates="$tmp_dir/candidates"
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM
: >"$candidates"

add_candidate() {
  candidate="$(normalize_url "$1")"
  if [ -n "$candidate" ] && ! grep -Fxq "$candidate" "$candidates"; then
    printf '%s\n' "$candidate" >>"$candidates"
  fi
}

add_ip_candidate() {
  ip="$(valid_ipv4 "$1" 2>/dev/null || true)"
  [ -n "$ip" ] || return 0
  add_candidate "http://$ip"
}

add_ips_from_text_file() {
  file="$1"
  [ -f "$file" ] || return 0
  grep -Eo '([0-9]{1,3}\.){3}[0-9]{1,3}' "$file" 2>/dev/null | while IFS= read -r ip; do
    add_ip_candidate "$ip"
  done
}

probe_candidates() {
  while IFS= read -r candidate; do
    [ -n "$candidate" ] || continue
    log "probing $candidate"
    if discovered="$(probe_url "$candidate")"; then
      emit_url "$discovered"
      exit 0
    fi
  done <"$candidates"
}

serial_ports() {
  if [ -n "$upload_port" ]; then
    printf '%s\n' "$upload_port"
    return 0
  fi

  for candidate in /dev/cu.usbmodem* /dev/cu.usbserial* /dev/cu.SLAB_USBtoUART*; do
    if [ -c "$candidate" ]; then
      printf '%s\n' "$candidate"
    fi
  done
}

reset_device() {
  port="$1"
  [ "$reset_before_read" = "1" ] || return 0
  log "resetting $port before serial IP discovery"
  if command -v uv >/dev/null 2>&1; then
    uv run --with esptool esptool --chip "$chip" --port "$port" chip-id >/dev/null 2>&1 || true
  elif command -v esptool >/dev/null 2>&1; then
    esptool --chip "$chip" --port "$port" chip-id >/dev/null 2>&1 || true
  else
    log "esptool is not available; reading serial without reset"
  fi
}

read_serial_ips() {
  port="$1"
  if ! command -v python3 >/dev/null 2>&1; then
    log "python3 is required for USB serial discovery"
    return 1
  fi

  TARS_STACKCHAN_DISCOVER_BAUD="$serial_baud" \
  TARS_STACKCHAN_DISCOVER_TIMEOUT="$serial_timeout" \
  python3 - "$port" <<'PY'
import os
import re
import select
import sys
import termios
import time

port = sys.argv[1]
baud = int(os.environ.get("TARS_STACKCHAN_DISCOVER_BAUD", "115200"))
timeout = float(os.environ.get("TARS_STACKCHAN_DISCOVER_TIMEOUT", "8"))
speed = getattr(termios, f"B{baud}", termios.B115200)
pattern = re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b")

def valid(ip):
    try:
        parts = [int(part) for part in ip.split(".")]
    except ValueError:
        return False
    return len(parts) == 4 and all(0 <= part <= 255 for part in parts) and parts[0] not in (0, 127, 255)

seen = set()
fd = os.open(port, os.O_RDONLY | os.O_NOCTTY | os.O_NONBLOCK)
try:
    attrs = termios.tcgetattr(fd)
    attrs[0] = 0
    attrs[1] = 0
    attrs[2] = (attrs[2] | termios.CLOCAL | termios.CREAD) & ~termios.CSIZE
    attrs[2] |= termios.CS8
    attrs[3] = 0
    attrs[4] = speed
    attrs[5] = speed
    termios.tcsetattr(fd, termios.TCSANOW, attrs)

    deadline = time.time() + timeout
    while time.time() < deadline:
        readable, _, _ = select.select([fd], [], [], 0.2)
        if not readable:
            continue
        try:
            chunk = os.read(fd, 4096)
        except BlockingIOError:
            continue
        text = chunk.decode("utf-8", errors="ignore")
        for ip in pattern.findall(text):
            if valid(ip) and ip not in seen:
                seen.add(ip)
                print(ip, flush=True)
finally:
    os.close(fd)
PY
}

if [ -n "${TARS_STACKCHAN_BASE_URL:-}" ]; then
  add_candidate "$TARS_STACKCHAN_BASE_URL"
fi
for host in $hosts; do
  add_candidate "$host"
done

probe_candidates

if [ -n "$log_file" ]; then
  log "reading IP candidates from $log_file"
  add_ips_from_text_file "$log_file"
  probe_candidates
fi

ports="$(serial_ports || true)"
if [ -z "$ports" ]; then
  log "no USB serial device found; set TARS_STACKCHAN_UPLOAD_PORT or TARS_STACKCHAN_BASE_URL"
else
  printf '%s\n' "$ports" | while IFS= read -r port; do
    [ -n "$port" ] || continue
    if [ ! -c "$port" ]; then
      log "skipping missing serial device $port"
      continue
    fi
    reset_device "$port"
    log "reading USB serial IP candidates from $port at ${serial_baud} baud"
    read_serial_ips "$port" | while IFS= read -r ip; do
      add_ip_candidate "$ip"
    done
  done
  probe_candidates
fi

log "could not discover a reachable Stack-chan URL"
exit 1
