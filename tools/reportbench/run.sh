#!/bin/sh
# 報告プロンプトの試験の材料を作る（/report-eval のスキル用）。
#   sh tools/reportbench/run.sh <作業ディレクトリ>
# 合成の履歴と git リポジトリ（fixture.py）→ kiroku の HTML → 日報・週報・月報のプロンプト（prompts.mjs）を <作業ディレクトリ>/prompts に。
# 自分の履歴を読まないよう、HOME と kiroku の読む場所はすべて作業ディレクトリに向ける。
set -eu
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
work=${1:?作業ディレクトリを指定してください}
rm -rf "$work"
mkdir -p "$work"
work=$(cd "$work" && pwd)
python3 -I "$here/fixture.py" "$work" >/dev/null
(cd "$root" && go build -o "$work/kiroku" .)
(cd "$work" && HOME="$work/home" TZ=Asia/Tokyo GIT_CONFIG_GLOBAL=/dev/null ./kiroku html --no-open --sources claude \
  --claude-root "$work/home/.claude/projects" --archive-dir "$work/archive" -o "$work/kiroku.html" >/dev/null 2>&1)
node "$here/prompts.mjs" "$work/kiroku.html" "$work/prompts"
