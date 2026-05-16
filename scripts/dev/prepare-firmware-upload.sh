#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
lock_file="$repo_root/firmware/stackchan/upstream.lock"
overlay_dir="$repo_root/firmware/stackchan/mods/tars_stackchan_bridge"

upstream_repo="$(awk -F= '$1 == "repository" { print $2 }' "$lock_file")"
upstream_commit="$(awk -F= '$1 == "commit" { print $2 }' "$lock_file")"
target_dir="${TARS_STACKCHAN_UPSTREAM_DIR:-$repo_root/.work/stack-chan}"
token="${TARS_STACKCHAN_TOKEN:-}"
target="${TARS_STACKCHAN_TARGET:-esp32/m5stack_cores3}"

if ! command -v git >/dev/null 2>&1; then
  echo "git is required" >&2
  exit 1
fi

if ! command -v node >/dev/null 2>&1; then
  echo "node is required to patch the MOD manifest token" >&2
  exit 1
fi

mkdir -p "$(dirname -- "$target_dir")"

if [ ! -d "$target_dir/.git" ]; then
  git clone "$upstream_repo" "$target_dir"
fi

git -C "$target_dir" fetch --depth 1 origin "$upstream_commit"
git -C "$target_dir" checkout --detach "$upstream_commit"

mkdir -p "$target_dir/firmware/mods"
rm -rf "$target_dir/firmware/mods/tars_stackchan_bridge"
cp -R "$overlay_dir" "$target_dir/firmware/mods/tars_stackchan_bridge"

manifest="$target_dir/firmware/mods/tars_stackchan_bridge/manifest.json"

if [ -n "$token" ]; then
  node - "$manifest" "$token" <<'NODE'
const fs = require('fs')
const [manifestPath, token] = process.argv.slice(2)
const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
manifest.config ??= {}
manifest.config.tarsStackchan ??= {}
manifest.config.tarsStackchan.token = token
fs.writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`)
NODE
else
  echo "warning: TARS_STACKCHAN_TOKEN is empty; manifest still contains replace-with-local-token" >&2
fi

cat <<EOF
Prepared upstream firmware checkout:
  $target_dir

Next upload commands:
  TARS_STACKCHAN_UPLOAD_PORT=/dev/cu.usbmodemXXXX \\
  TARS_STACKCHAN_DEPLOY_HOST=1 \\
  scripts/dev/upload-firmware.sh all

MOD-only direct flash after the host is already deployed:
  TARS_STACKCHAN_UPLOAD_PORT=/dev/cu.usbmodemXXXX \\
  scripts/dev/upload-firmware.sh mod

After upload, run:
  TARS_STACKCHAN_BASE_URL=http://stackchan.local \\
  TARS_STACKCHAN_TOKEN="\$TARS_STACKCHAN_TOKEN" \\
  "$repo_root/scripts/test/hardware-smoke.sh"
EOF
