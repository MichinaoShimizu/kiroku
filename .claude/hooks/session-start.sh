#!/bin/bash
# Claude Code のクラウドのセッションが始まるときに、tools/check.sh と画面の e2e に要るものを先に入れておく。
# 手元の Claude Code では何もしない。何度動かしても同じ結果になる。版はすべて固定（ci.yml・package-lock.json）。
set -euo pipefail

if [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0
fi

cd "${CLAUDE_PROJECT_DIR:-$(dirname "$0")/../..}"

# Go のモジュールと、go.mod の toolchain の版の Go
go mod download
toolchain=$(go env GOVERSION)

# staticcheck と govulncheck を、tools/check.sh と同じ版・同じ toolchain で一度ビルドしてキャッシュに載せる
ci=.github/workflows/ci.yml
for tool in "$(grep -o 'honnef.co/go/tools/cmd/staticcheck@[^ ]*' "$ci" | head -n 1)" \
  "$(grep -o 'golang.org/x/vuln/cmd/govulncheck@[^ ]*' "$ci" | head -n 1)"; do
  GOTOOLCHAIN="$toolchain" go run "$tool" -version >/dev/null
done

# テストのビルドもキャッシュに載せる（動かしはしない）
go test -run '^$' ./... >/dev/null

# tools/check.sh の shellcheck を入れる（CI は Ubuntu のものを使う）。入らなくてもセッションは止めない
if ! command -v shellcheck >/dev/null 2>&1; then
  pip install --quiet 'shellcheck-py==0.11.0.1' || echo "session-start: could not install shellcheck" >&2
fi

# 画面の e2e とスクショ用の Playwright。インストール時のスクリプトは動かさない
(cd tools/screenshots && npm ci --ignore-scripts --no-audit --no-fund --silent)

# Playwright の Chromium は取ってこられないので、コンテナに入っているものを使う
if [ -n "${CLAUDE_ENV_FILE:-}" ] && [ -x /opt/pw-browsers/chromium ]; then
  echo 'export CHROMIUM=/opt/pw-browsers/chromium' >>"$CLAUDE_ENV_FILE"
fi
