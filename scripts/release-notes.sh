#!/usr/bin/env bash
# Prints the CHANGELOG.md section for VERSION (e.g. 0.4.0): the GitHub
# release notes.
set -euo pipefail
version="${1:?usage: release-notes.sh VERSION}"
notes="$(awk -v v="$version" '
  /^## / { if (found) exit; found = index($0, "[" v "]") > 0; next }
  found
' "$(dirname "$0")/../CHANGELOG.md")"
if [[ -z "${notes//[[:space:]]/}" ]]; then
  echo "No CHANGELOG.md section for $version" >&2
  exit 1
fi
printf '%s\n' "$notes"
