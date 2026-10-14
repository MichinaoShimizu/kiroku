#!/bin/sh
# スクリーンショットを撮り直したあと（sh tools/screenshots/run.sh）、コミットしてある版と比べて違いが小さなゆらぎだけの画像を
# コミットしてある版に戻す。本当に変わった画像だけがコミットに残る。比べるのは tools/imgdiff（Go の image パッケージ）。
#   sh tools/screenshots/keep-changed.sh               # docs/*.png と docs/demo.gif のうち、HEAD から変わったもの
#   sh tools/screenshots/keep-changed.sh docs/og.png   # 指定したものだけ
# PNG は画素で比べ、demo.gif は止まって見える場面どうしで比べる（くわしくは tools/imgdiff/main.go）。
# ダミーデータは「今」から作るので、日や時刻が違えば中身も変わる。同じ画像に戻せるのは、同じ KIROKU_FAKE_NOW で撮ったときだけ。
# 終了コード: 0 = 比べ終えた、1 = 読めない画像があった（その画像は撮り直したまま残す）
set -eu
root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
go build -o "$tmp/imgdiff" ./tools/imgdiff

if [ $# -eq 0 ]; then
  # 変わった画像（追跡しているものだけ。新しい画像は比べる相手がないので、そのまま残す）
  git diff --name-only HEAD -- 'docs/*.png' docs/demo.gif > "$tmp/list"
else
  printf '%s\n' "$@" > "$tmp/list"
fi

failed=0
restored=0
kept=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  case "$f" in
    *.png) ext=png ;;
    *.gif) ext=gif ;;
    *) echo "skip $f: only .png and .gif" >&2; failed=1; continue ;;
  esac
  if ! git cat-file -e "HEAD:$f" 2>/dev/null; then
    echo "new      $f (not committed yet)"
    kept=$((kept + 1))
    continue
  fi
  git show "HEAD:$f" > "$tmp/old.$ext"
  rc=0
  out=$("$tmp/imgdiff" "$tmp/old.$ext" "$f") || rc=$?
  case $rc in
    0)
      git checkout HEAD -- "$f"
      echo "restored $f: ${out#"$f": }"
      restored=$((restored + 1))
      ;;
    1)
      echo "changed  $f: ${out#"$f": }"
      kept=$((kept + 1))
      ;;
    *)
      echo "error    $f: could not compare, kept the new image" >&2
      failed=1
      ;;
  esac
done < "$tmp/list"

echo "$kept changed, $restored restored to the committed version"
exit "$failed"
