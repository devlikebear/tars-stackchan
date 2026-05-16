#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"

node --test "$repo_root/firmware/stackchan/mods/tars_stackchan_bridge/bridge-core.test.mjs"
