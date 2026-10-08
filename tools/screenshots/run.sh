#!/bin/sh
# ダミーデータを作り、kiroku の HTML を出力して、docs のスクリーンショットを撮り直す。
#   sh tools/screenshots/run.sh
#   sh tools/screenshots/run.sh --html <出力先.html>   # ダミーデータの HTML だけを作る（git のリポジトリは一時ディレクトリごと消えるので、コミットのリンクは開けない）
# 必要なもの: Go、Python 3、git、Node.js と Playwright（このディレクトリで npm ci --ignore-scripts。版は package.json で固定）
set -eu
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
python3 "$here/gen.py" "$work"
python3 "$here/mkgit.py" "$work"
# gen.py は日本時間で作るので、日ごとの集計も日本時間で区切る。--html（デモ）では、どこから開いてもこの時計で見せる
if [ "${1:-}" = "--html" ]; then demo=1; else demo=; fi
# 読むのはダミーデータだけ。自分の履歴や kiroku archive の写しを混ぜないよう、読む場所をすべて一時ディレクトリに向け、
# 場所を変えるオプションのない Kiro IDE（v1.0 より前）のためにホームも替える（go のキャッシュはそのままにしたいので、先にビルドする）
(cd "$root" && go build -o "$work/kiroku" .)
(cd "$work" && HOME="$work/home" TZ=Asia/Tokyo KIROKU_DEMO=$demo ./kiroku html --no-open --sources claude,codex,kiro \
  --claude-root "$work/home/.claude/projects" --codex-home "$work/home/.codex" --kiro-home "$work/home/.kiro" \
  --crew-home "$work/home/.kiro/crew" --kiro-cli-db "$work/none.sqlite3" --archive-dir "$work/archive" -o "$work/kiroku.html")
if [ "${1:-}" = "--html" ]; then # ダミーデータの HTML だけを作る（利用者目線のテストなどに使う）
  cp "$work/kiroku.html" "${2:?出力先の HTML を指定してください}"
  echo "${2} を作りました"
  exit 0
fi
node "$here/capture.mjs" "$work/kiroku.html" "$root/docs"
echo "docs/screenshot.png・docs/summary.png・docs/worth.png・docs/report.png・docs/session.png・docs/og.png・docs/year.png を更新しました"
