#!/usr/bin/env bash
# Sets the app version (e.g. 0.5.0) in build/config.yml and every
# platform file generated from it.
set -euo pipefail
new="${1:?usage: bump.sh VERSION}"
cd "$(dirname "$0")/.."
old="$(awk -F'"' '/^  version:/ { print $2; exit }' build/config.yml)"
[[ "$old" == "$new" ]] && { echo "Already $new"; exit 0; }
files=(
  build/config.yml
  build/darwin/Info.plist
  build/darwin/Info.dev.plist
  build/linux/nfpm/nfpm.yaml
  build/windows/info.json
  build/windows/wails.exe.manifest
  build/windows/nsis/wails_tools.nsh
  build/windows/msix/app_manifest.xml
  build/windows/msix/template.xml
)
for f in "${files[@]}"; do
  sed -i.bak -e "s/$old\.0/$new.0/g" -e "s/\"$old\"/\"$new\"/g" -e "s#>$old<#>$new<#g" "$f"
  rm "$f.bak"
done
grep -q "\"$new\"" build/config.yml
echo "$old -> $new"
