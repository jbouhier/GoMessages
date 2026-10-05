#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "Marketing capture currently runs on macOS." >&2
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${1:-$repo_root/site/public/product}"
mkdir -p "$output_dir"

output_dir="$(cd "$output_dir" && pwd)"

cargo build --locked --manifest-path "$repo_root/Cargo.toml" --bin gomessages
capture_dir="$(mktemp -d "${TMPDIR:-/tmp}/gomessages-marketing.XXXXXX")"
app_pid=""
cleanup() {
  if [[ -n "$app_pid" ]]; then
    kill "$app_pid" 2>/dev/null || true
    wait "$app_pid" 2>/dev/null || true
  fi
  rm -rf "$capture_dir"
}
trap cleanup EXIT

clang -framework CoreGraphics -framework CoreFoundation \
  "$repo_root/scripts/capture-window.c" -o "$capture_dir/capture-window"

GOMESSAGES_MARKETING_DATA_DIR="$capture_dir/data" \
  "$repo_root/target/debug/gomessages" --marketing-demo &
app_pid=$!

"$capture_dir/capture-window" "$app_pid" "$capture_dir/desktop.png"
kill "$app_pid" 2>/dev/null || true
wait "$app_pid" 2>/dev/null || true
app_pid=""

GOMESSAGES_MARKETING_DATA_DIR="$capture_dir/data" \
  "$repo_root/target/debug/gomessages" --marketing-demo --marketing-settings &
app_pid=$!
"$capture_dir/capture-window" "$app_pid" "$capture_dir/settings.png" smallest

cp "$capture_dir/desktop.png" "$output_dir/desktop.png"
cp "$capture_dir/settings.png" "$output_dir/settings.png"
sips -g pixelWidth -g pixelHeight "$output_dir/desktop.png" "$output_dir/settings.png"
