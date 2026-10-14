#!/bin/sh
# CI（.github/workflows/ci.yml）と同じチェックを手元でまとめて回す。PR を出す前に使う。
#   sh tools/check.sh          # gofmt・go vet・staticcheck・govulncheck・go test・changes/ の断片・go build・shellcheck
#   sh tools/check.sh --e2e    # それに加えて、画面の e2e（smoke.mjs・xss.mjs・serve.mjs。Node と Playwright が必要）
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
  # CI（ci.yml の install-script）と同じ引数。CI はランナーに入っている shellcheck（ubuntu-24.04 では 0.9.0）を使う。
  # 0.11 は SC2015（A && B || C）などを出さないことがあるので、版が違えば CI で初めて落ちることがある
  ver=$(shellcheck --version | sed -n 's/^version: //p')
  case "$ver" in
    0.9.* | 0.10.*) ;;
    *) echo "note: shellcheck $ver here; CI's older shellcheck can report more (for example SC2015)" ;;
  esac
  shellcheck install.sh tools/*.sh tools/*/*.sh .claude/hooks/*.sh
}

# changes/ の断片の名前と中身（CI の release-dry-run と同じ）と、それをまとめる tools/changelog.sh・next-version.sh のテスト
changelog() {
  sh tools/changelog.sh --check || return 1
  if out=$(sh tools/changelog_test.sh 2>&1); then
    printf '%s\n' "$out" | tail -n 1
    return 0
  fi
  printf '%s\n' "$out" | grep -v '^ok: '
  return 1
}

build() {
  out=$(mktemp -d) || return 1
  go build -o "$out/kiroku" .
  rc=$?
  rm -rf "$out"
  return $rc
}

# CI の e2e ジョブと同じ流れ。CHROMIUM があればその Chromium を使う（Playwright の Chromium を取ってこられない環境向け）。
# ダミーデータとブラウザの時計は、CI と同じ決まった時刻（KIROKU_FAKE_NOW。ci.yml から読む）を「今」にする（走らせた日付や時刻で結果が変わらないように）。
# ほかの時刻で試すときは KIROKU_FAKE_NOW を渡す
e2e_run() {
  if [ ! -d tools/screenshots/node_modules/playwright ]; then
    echo "Playwright is not installed: run (cd tools/screenshots && npm ci --ignore-scripts && npx playwright install chromium)"
    return 1
  fi
  if [ -z "${KIROKU_FAKE_NOW:-}" ]; then
    KIROKU_FAKE_NOW=$(sed -n 's/^ *KIROKU_FAKE_NOW: *"\{0,1\}\([^" ]*\)"\{0,1\} *$/\1/p' "$ci" | head -n 1)
  fi
  if [ -z "$KIROKU_FAKE_NOW" ]; then
    echo "could not read KIROKU_FAKE_NOW from $ci"
    return 1
  fi
  export KIROKU_FAKE_NOW
  echo "KIROKU_FAKE_NOW=$KIROKU_FAKE_NOW"
  work=$(mktemp -d) || return 1
  sh tools/screenshots/run.sh --html "$work/kiroku.html" &&
    node tools/screenshots/smoke.mjs "$work/kiroku.html" &&
    python3 -I tools/screenshots/hostile.py "$work/hostile" &&
    TZ=UTC go run . html --no-open --sources claude --claude-root "$work/hostile/fx/projects" --week 2026-10-05 -o "$work/xss.html" &&
    node tools/screenshots/xss.mjs "$work/xss.html" &&
    go build -o "$work/kiroku" . &&
    python3 -I tools/screenshots/gen.py "$work/gen" >/dev/null &&
    node tools/screenshots/serve.mjs "$work/kiroku" "$work/gen"
  rc=$?
  rm -rf "$work"
  return $rc
}

step gofmt gofmt_clean
step vet go vet ./...
step staticcheck env GOTOOLCHAIN="$toolchain" go run "$staticcheck" ./...
step govulncheck env GOTOOLCHAIN="$toolchain" go run "$govulncheck" ./...
step test go test ./...
step changelog changelog
step build build
step seclint go run ./tools/seclint
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
