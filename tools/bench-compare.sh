#!/bin/sh
# 読み取り・集計・描画のベンチマーク（internal/cli の BenchmarkLarge）を、比べる元（既定: origin/main）と今の作業ツリーで回し、比べて出す。
#   sh tools/bench-compare.sh              # origin/main と比べる
#   sh tools/bench-compare.sh v0.36.0      # タグ・ブランチ・コミットと比べる
# 環境変数:
#   KIROKU_BENCH_COUNT  それぞれ何回回すか（-count。既定 6。benchstat が差を確かめるには 6 回ほど要る）
#   KIROKU_BENCH_TIME   -benchtime（例: 3x。既定は go test の既定）
#   KIROKU_BENCH_PLAIN=1  benchstat を使わず、平均の表を出す
#   KIROKU_BENCH_DAYS・KIROKU_BENCH_PER_DAY  合成の履歴の大きさ（bench_test.go。両方に同じ値が渡る）
# 比べる元は、一時ディレクトリの git worktree で回し、終わったら消す。今のほうは、まだコミットしていない変更も含めて回す。
# 比べるのは benchstat（版を固定して go run で動かす。上げるときは下の benchstat= を書き換える）。動かせなければ、平均の表を出す。
set -eu

benchstat=golang.org/x/perf/cmd/benchstat@v0.0.0-20261009192801-be2c69fb417e

die() { printf 'bench-compare: %s\n' "$*" >&2; exit 1; }

[ $# -le 1 ] || die "usage: sh tools/bench-compare.sh [base-ref]"
base=${1:-origin/main}
count=${KIROKU_BENCH_COUNT:-6}
case "$count" in '' | *[!0-9]* | 0) die "KIROKU_BENCH_COUNT must be a positive number" ;; esac
case "${KIROKU_BENCH_TIME:-}" in '' | [0-9]*x | [0-9]*s | [0-9]*ms) ;; *) die "KIROKU_BENCH_TIME must look like 3x or 2s" ;; esac
root=$(git rev-parse --show-toplevel) || die "run this inside the kiroku repository"
cd "$root"
case "$base" in -*) die "the base ref must not start with -" ;; esac
commit=$(git rev-parse --verify --quiet "$base^{commit}") || die "unknown ref: $base (for origin/main, run git fetch origin main first)"

tmp=$(mktemp -d)
wt=$tmp/base
cleanup() {
  if [ -d "$wt" ]; then git worktree remove --force "$wt" >/dev/null 2>&1 || true; fi
  git worktree prune >/dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

run() { # run <dir> <out>
  set -- "$1" "$2" -run '^$' -bench Large -benchmem -count "$count"
  if [ -n "${KIROKU_BENCH_TIME:-}" ]; then set -- "$@" -benchtime "$KIROKU_BENCH_TIME"; fi
  (cd "$1" && shift 2 && go test ./internal/cli "$@") > "$2" 2>&1 || { cat "$2" >&2; return 1; }
  grep -q '^BenchmarkLarge' "$2" || { cat "$2" >&2; return 1; }
}

echo "== base: $base ($(git rev-parse --short "$commit")), $count runs"
git worktree add --quiet --detach "$wt" "$commit"
run "$wt" "$tmp/base.txt" || die "the benchmark failed on $base (does it have BenchmarkLarge in internal/cli?)"
git worktree remove --force "$wt"

dirty=
[ -z "$(git status --porcelain --untracked-files=no)" ] || dirty=" with uncommitted changes"
echo "== head: $(git rev-parse --short HEAD)$dirty, $count runs"
run "$root" "$tmp/head.txt" || die "the benchmark failed on the working tree"

echo
if [ -z "${KIROKU_BENCH_PLAIN:-}" ] && go run "$benchstat" -h >/dev/null 2>&1; then
  (cd "$tmp" && go run "$benchstat" base=base.txt head=head.txt)
  exit 0
fi
echo "(plain means of $count runs, no significance test; benchstat was not used)"
echo
# 平均の表：ベンチマークと単位ごとに、base と head の平均と変化
awk '
  FNR == 1 { side = (FILENAME ~ /base\.txt$/) ? "base" : "head" }
  /^BenchmarkLarge/ {
    name = $1; sub(/-[0-9]+$/, "", name)
    for (i = 3; i + 1 <= NF; i += 2) {
      k = name SUBSEP $(i + 1)
      if (!(k in seen)) { seen[k] = 1; keys[++n] = k }
      sum[side, k] += $i; cnt[side, k]++
    }
  }
  END {
    printf "%-28s %-12s %16s %16s %9s\n", "benchmark", "unit", "base", "head", "change"
    for (j = 1; j <= n; j++) {
      k = keys[j]; split(k, p, SUBSEP)
      b = cnt["base", k] ? sum["base", k] / cnt["base", k] : 0
      h = cnt["head", k] ? sum["head", k] / cnt["head", k] : 0
      d = (b != 0 && cnt["head", k]) ? sprintf("%+.1f%%", (h - b) / b * 100) : "-"
      printf "%-28s %-12s %16.4g %16.4g %9s\n", p[1], p[2], b, h, d
    }
  }' "$tmp/base.txt" "$tmp/head.txt"
