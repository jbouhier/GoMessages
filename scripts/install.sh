#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "GoMessages installation targets macOS." >&2
  exit 1
fi

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tools_dir="$project_dir/bin/tools"
wails3_bin="$tools_dir/wails3"
version="$(awk '$1 == "github.com/wailsapp/wails/v3" { print $2; exit }' "$project_dir/go.mod")"
if [[ -z "$version" ]]; then
  echo "Cannot find the Wails version in go.mod." >&2
  exit 1
fi

if [[ ! -x "$wails3_bin" ]]; then
  mkdir -p "$tools_dir"
  GOBIN="$tools_dir" go install "github.com/wailsapp/wails/v3/cmd/wails3@$version"
fi

export PATH="$tools_dir:$PATH"
cd "$project_dir"
"$wails3_bin" package

source_app="$project_dir/bin/gomessages.app"
target_app="/Applications/GoMessages.app"
if [[ ! -d "$source_app" ]]; then
  echo "Build did not produce $source_app" >&2
  exit 1
fi
codesign --verify --deep --strict "$source_app"

stage_root="$(mktemp -d /Applications/.gomessages-install.XXXXXX)"
stage_app="$stage_root/GoMessages.app"
backup_app="$stage_root/previous.app"
cleanup() {
  if [[ -d "$backup_app" && ! -e "$target_app" ]]; then
    mv "$backup_app" "$target_app"
  fi
  rm -rf "$stage_root"
}
trap cleanup EXIT

ditto "$source_app" "$stage_app"
codesign --verify --deep --strict "$stage_app"
if [[ -e "$target_app" ]]; then
  mv "$target_app" "$backup_app"
fi
mv "$stage_app" "$target_app"
if ! codesign --verify --deep --strict "$target_app"; then
  rm -rf "$target_app"
  exit 1
fi

echo "Installed: $target_app"
