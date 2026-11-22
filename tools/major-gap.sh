#!/bin/sh
# 新しいメジャー版を、前のメジャー版から日を空けずに出さないための確かめ（docs/compatibility.md「Breaking changes」）。
# v2.0.0 以降の X.0.0 で、CHANGELOG.md の前のメジャー版（vX-1.0.0 から下の最初の「## vN.0.0 - 日付」）の日付から
# KIROKU_MAJOR_GAP_DAYS（既定 30）日たっていなければ、理由を出して 1 で終わる。
# セキュリティの修正のために急ぐときは KIROKU_MAJOR_OK=1 で通す（そのときも PR に理由を書く）。
# Usage: sh tools/major-gap.sh vX.Y.Z [CHANGELOG.md] [今日（YYYY-MM-DD、既定は UTC の今日）]
set -eu
v=${1:?usage: sh tools/major-gap.sh vX.Y.Z [CHANGELOG.md] [YYYY-MM-DD]}
file=${2:-CHANGELOG.md}
today=${3:-$(date -u +%Y-%m-%d)}
gap=${KIROKU_MAJOR_GAP_DAYS:-30}
case "$v" in
  v[0-9]*.0.0) ;;
  *) exit 0 ;;
esac
ma=${v#v}; ma=${ma%%.*}
[ "$ma" -ge 2 ] || exit 0
prev=$(awk -v v="$v" '/^## v[0-9]+\.0\.0 - [0-9]{4}-[0-9]{2}-[0-9]{2}/ && $2 != v { print $2, $4; exit }' "$file")
[ -n "$prev" ] || exit 0
pv=${prev% *} pd=${prev#* }
# 日付を通算の日数にする（GNU と BSD の date に頼らない）
days() {
  IFS=- read -r y m d <<D
$1
D
  y=${y#0} m=${m#0} d=${d#0}
  [ "$m" -le 2 ] && { y=$((y - 1)); m=$((m + 12)); }
  echo $((365 * y + y / 4 - y / 100 + y / 400 + (153 * (m - 3) + 2) / 5 + d))
}
since=$(( $(days "$today") - $(days "$pd") ))
if [ "$since" -lt "$gap" ]; then
  if [ "${KIROKU_MAJOR_OK:-}" = 1 ]; then
    echo "major-gap: $v comes $since days after $pv ($pd); allowed by KIROKU_MAJOR_OK=1" >&2
    exit 0
  fi
  echo "major-gap: $v would come only $since days after $pv ($pd); the minimum is $gap days." >&2
  echo "  Mark the removal as Deprecated in a minor release and hold the BREAKING fragment until then (docs/compatibility.md)," >&2
  echo "  or set KIROKU_MAJOR_OK=1 for a security fix and say why in the pull request." >&2
  exit 1
fi
