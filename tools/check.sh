#!/bin/sh
# CI（.github/workflows/ci.yml）と同じチェックを手元でまとめて回す。PR を出す前に使う。
#   sh tools/check.sh          # gofmt・go vet・staticcheck・govulncheck・go test・go build・shellcheck
#   sh tools/check.sh --e2e    # それに加えて、画面の e2e（smoke.mjs と xss.mjs。Node と Playwright が必要）
# staticcheck と govulncheck の版は ci.yml から読む（版を上げるときは ci.yml だけを書き換える）。
# 途中で失敗しても最後まで回し、失敗したものを最後に並べて 1 で終わる。
set -u
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root" || exit 1

e2e=
case "${1:-}" in
  --e2e) e2e=1 ;;
  "") ;;
  *) echo "usage: sh tools/check.sh [--e2e]" >&2; exit 2 ;;
esac

ci=.github/workflows/ci.yml
staticcheck=$(grep -o 'honnef.co/go/tools/cmd/staticcheck@[^ ]*' "$ci" | head -n 1)
govulncheck=$(grep -o 'golang.org/x/vuln/cmd/govulncheck@[^ ]*' "$ci" | head -n 1)
if [ -z "$staticcheck" ] || [ -z "$govulncheck" ]; then
  echo "could not read the staticcheck or govulncheck version from $ci" >&2
  exit 1
fi
toolchain=$(go env GOVERSION) || exit 1

failed=
step() {
  name=$1
  shift
  printf '\n== %s\n' "$name"
  if "$@"; then
    echo "ok"
  else
    echo "FAIL: $name"
    failed="$failed $name"
  fi
}

gofmt_clean() {
  out=$(gofmt -l .) || return 1
  [ -z "$out" ] && return 0
  echo "$out"
  return 1
}

shellcheck_scripts() {
  if ! command -v shellcheck >/dev/null 2>&1; then
    echo "shellcheck is not installed: skipped (CI runs it)"
    return 0
  fi
  shellcheck install.sh tools/*.sh .claude/hooks/*.sh
}

build() {
  out=$(mktemp -d) || return 1
  go build -o "$out/kiroku" .
  rc=$?
  rm -rf "$out"
  return $rc
}

# CI の e2e ジョブと同じ流れ。CHROMIUM があればその Chromium を使う（Playwright の Chromium を取ってこられない環境向け）
e2e_run() {
  if [ ! -d tools/screenshots/node_modules/playwright ]; then
    echo "Playwright is not installed: run (cd tools/screenshots && npm ci --ignore-scripts && npx playwright install chromium)"
    return 1
  fi
  work=$(mktemp -d) || return 1
  sh tools/screenshots/run.sh --html "$work/kiroku.html" &&
    node tools/screenshots/smoke.mjs "$work/kiroku.html" &&
    python3 -I tools/screenshots/hostile.py "$work/hostile" &&
    TZ=UTC go run . html --no-open --sources claude --claude-root "$work/hostile/fx/projects" --week 2026-10-05 -o "$work/xss.html" &&
    node tools/screenshots/xss.mjs "$work/xss.html"
  rc=$?
  rm -rf "$work"
  return $rc
}

step gofmt gofmt_clean
step vet go vet ./...
step staticcheck env GOTOOLCHAIN="$toolchain" go run "$staticcheck" ./...
step govulncheck env GOTOOLCHAIN="$toolchain" go run "$govulncheck" ./...
step test go test ./...
step build build
step shellcheck shellcheck_scripts
if [ -n "$e2e" ]; then
  step e2e e2e_run
fi

echo
if [ -n "$failed" ]; then
  echo "failed:$failed"
  exit 1
fi
echo "all checks passed"
