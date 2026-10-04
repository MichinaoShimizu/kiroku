#!/bin/sh
# CHANGELOG.md の節の中身から、次の版の番号を出す（セマンティック バージョニング）。
#   BREAKING を含む → 1.0 以降は major、0.x の間は minor
#   ### Added がある → minor
#   それ以外（Changed / Fixed / Removed だけ）→ patch
# Usage: sh tools/next-version.sh [節の名前（既定: Unreleased）] [CHANGELOG.md]
#        前の版は、その節より下にある最初の版の節（なければ最新のタグ）
set -eu
sec=${1:-Unreleased}
file=${2:-CHANGELOG.md}
body=$(awk -v v="$sec" '/^## / { if (on) exit; on = ($2 == v); next } on' "$file")
if [ -z "$(printf '%s' "$body" | tr -d '[:space:]')" ]; then
  echo "next-version: \"## $sec\" in $file is missing or empty" >&2
  exit 1
fi
prev=$(awk -v v="$sec" '/^## / { if (seen && $2 ~ /^v[0-9]/) { print $2; exit } if ($2 == v) seen = 1 }' "$file")
[ -n "$prev" ] || prev=$(git tag --list 'v*' --sort=-v:refname 2>/dev/null | head -1)
[ -n "$prev" ] || prev=v0.0.0
IFS=. read -r ma mi pa <<VER
$(printf '%s' "${prev#v}" | sed 's/-.*//')
VER
if printf '%s\n' "$body" | grep -q 'BREAKING'; then
  if [ "$ma" -ge 1 ]; then ma=$((ma + 1)); mi=0; else mi=$((mi + 1)); fi
  pa=0
elif printf '%s\n' "$body" | grep -q '^### Added'; then
  mi=$((mi + 1)); pa=0
else
  pa=$((pa + 1))
fi
echo "v$ma.$mi.$pa"
