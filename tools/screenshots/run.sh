#!/bin/sh
# ダミーデータを作り、kiroku の HTML を出力して、docs のスクリーンショットを撮り直す。
#   sh tools/screenshots/run.sh
#   sh tools/screenshots/run.sh --html <出力先.html>   # ダミーデータの HTML だけを作る（git のリポジトリは一時ディレクトリごと消えるので、コミットのリンクは開けない）
# 必要なもの: Go、Python 3、git、Node.js と Playwright（このディレクトリで npm i playwright）
set -eu
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
python3 "$here/gen.py" "$work"
python3 "$here/mkgit.py" "$work"
(cd "$root" && go run . html --no-open --sources claude --root "$work/home/.claude/projects" -o "$work/kiroku.html")
if [ "${1:-}" = "--html" ]; then # ダミーデータの HTML だけを作る（利用者目線のテストなどに使う）
  cp "$work/kiroku.html" "${2:?出力先の HTML を指定してください}"
  echo "${2} を作りました"
  exit 0
fi
node "$here/capture.mjs" "$work/kiroku.html" "$root/docs"
echo "docs/screenshot.png と docs/summary.png を更新しました"
