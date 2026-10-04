#!/bin/sh
# ダミーデータを作り、kiroku の HTML を出力して、docs のスクリーンショットを撮り直す。
#   sh tools/screenshots/run.sh
# 必要なもの: Go、Python 3、git、Node.js と Playwright（このディレクトリで npm i playwright）
set -eu
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
python3 "$here/gen.py" "$work"
python3 "$here/mkgit.py" "$work"
(cd "$root" && go run . html --no-open --sources claude --root "$work/home/.claude/projects" -o "$work/kiroku.html")
node "$here/capture.mjs" "$work/kiroku.html" "$root/docs"
echo "docs/screenshot.png と docs/summary.png を更新しました"
