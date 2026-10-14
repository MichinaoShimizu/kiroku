#!/bin/sh
# Print the CHANGELOG.md section for a version, for use as GitHub release notes.
# Usage: sh tools/release-notes.sh v0.1.6 [CHANGELOG.md]
set -eu
ver=${1:?usage: release-notes.sh <tag> [file]}
file=${2:-CHANGELOG.md}
notes=$(awk -v v="$ver" '
  /^## / { if (on) exit; on = ($2 == v) ; next }
  on
' "$file" | sed '/./,$!d')
if [ -z "$notes" ]; then
  echo "release-notes: no \"## $ver\" section in $file. Make it with \"sh tools/release.sh\" (or \"sh tools/changelog.sh --release $ver\") before tagging." >&2
  exit 1
fi
printf '%s\n' "$notes"
