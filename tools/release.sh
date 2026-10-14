#!/bin/sh
# リリースの PR を手元で用意する。push もタグ付けもしない（PR をマージすると tag.yml がタグを打ってリリースする）。
#   sh tools/release.sh
#   sh tools/release.sh --no-screenshots   # スクリーンショットを撮り直さない
#   KIROKU_RELEASE_REMOTE=<リモートかパス>   # origin の代わりに、そこの main から作る（スクリプトを試すとき）
#
# 流れ:
#   1. 作業ツリーに変更があれば止まる（コミットするか、別のブランチに置いてから。素の git stash は使わない）
#   2. origin の main を取ってきて、tools/next-version.sh で次の版 vX.Y.Z を決め、そこから claude/release-vX.Y.Z を作る
#   3. tools/changelog.sh --release で、changes/ の断片と「## Unreleased」を「## vX.Y.Z - <今日（UTC）>」にまとめる
#   4. Chromium があれば（CHROMIUM か Playwright の Chromium）、スクリーンショットを撮り直し、tools/screenshots/keep-changed.sh で
#      ゆらぎだけの画像を戻す。ダミーデータの「今」は KIROKU_FAKE_NOW（なければ ci.yml の e2e と同じ決まった時刻）。
#      同じ時刻で撮るので、画面が変わっていなければ同じ画像になり、本当に変わった画像だけが残る。今の時刻で撮るなら KIROKU_FAKE_NOW= （空）
#   5. sh tools/check.sh と sh tools/release-notes.sh vX.Y.Z を回し、コミットして、次にすること（push と PR）を出す
#
# 中身はすべて main の中にあり、いちばん最後の行で呼ぶ（途中で git switch がこのファイルを書き換えても、読み終えたものを動かす）。
set -eu

die() { printf 'release: %s\n' "$*" >&2; exit 1; }

# Chromium で撮れるか：CHROMIUM の指す実行ファイルか、Playwright が入れた Chromium
have_chromium() {
  command -v node >/dev/null 2>&1 || return 1
  [ -d tools/screenshots/node_modules/playwright ] || return 1
  if [ -n "${CHROMIUM:-}" ]; then
    [ -x "$CHROMIUM" ]
    return
  fi
  (cd tools/screenshots && node -e 'const { chromium } = require("playwright"); process.exit(require("fs").existsSync(chromium.executablePath()) ? 0 : 1)') 2>/dev/null
}

main() {
  shots=1
  case "${1:-}" in
    --no-screenshots) shots= ;;
    "") ;;
    *) die "usage: sh tools/release.sh [--no-screenshots]" ;;
  esac
  [ $# -le 1 ] || die "usage: sh tools/release.sh [--no-screenshots]"
  root=$(git rev-parse --show-toplevel) || die "run this inside the kiroku repository"
  cd "$root"

  # 1. 作業ツリー
  if [ -n "$(git status --porcelain --untracked-files=normal)" ]; then
    git status --short >&2
    die "the working tree has changes. Commit them or move them to another branch first (don't use a bare \"git stash\")"
  fi

  # 2. 版とブランチ
  remote=${KIROKU_RELEASE_REMOTE:-origin}
  echo "== fetching main from $remote"
  git fetch --quiet "$remote" main
  # ブランチを作る前に止まったら、もとのブランチに戻す
  orig=$(git symbolic-ref --quiet --short HEAD || git rev-parse HEAD)
  back() {
    git switch --quiet "$orig" 2>/dev/null || git switch --quiet --detach "$orig"
    die "$1"
  }
  git switch --quiet --detach FETCH_HEAD
  v=$(sh tools/next-version.sh) || back "nothing to release (no fragments in changes/ and no \"## Unreleased\" in CHANGELOG.md)"
  printf '%s\n' "$v" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || back "next-version.sh printed \"$v\""
  branch=claude/release-$v
  if git show-ref --verify --quiet "refs/heads/$branch"; then
    back "the branch $branch already exists. Delete it (git branch -D $branch) or finish that release"
  fi
  if git ls-remote --exit-code --heads "$remote" "$branch" >/dev/null 2>&1; then
    back "$remote already has the branch $branch; finish that pull request first"
  fi
  if git ls-remote --exit-code --tags "$remote" "refs/tags/$v" >/dev/null 2>&1; then
    back "the tag $v already exists on $remote"
  fi
  git switch --quiet -c "$branch"
  echo "== $branch (from main of $remote, $(git rev-parse --short HEAD))"

  # 3. CHANGELOG
  today=$(date -u +%Y-%m-%d)
  sh tools/changelog.sh --release "$v" --date "$today"
  want=$(sh tools/next-version.sh "$v")
  [ "$want" = "$v" ] || die "the merged section says $want, not $v (sh tools/next-version.sh $v)"

  # 4. スクリーンショット
  if [ -z "$shots" ]; then
    echo "== screenshots: skipped (--no-screenshots)"
  elif ! have_chromium; then
    echo "== screenshots: skipped, no Chromium. Install Playwright (cd tools/screenshots && npm ci --ignore-scripts && npx playwright install chromium) or set CHROMIUM"
    shots=
  else
    if [ -z "${KIROKU_FAKE_NOW+set}" ]; then
      KIROKU_FAKE_NOW=$(sed -n 's/^ *KIROKU_FAKE_NOW: *"\{0,1\}\([^" ]*\)"\{0,1\} *$/\1/p' .github/workflows/ci.yml | head -n 1)
      [ -n "$KIROKU_FAKE_NOW" ] || die "could not read KIROKU_FAKE_NOW from .github/workflows/ci.yml"
    fi
    export KIROKU_FAKE_NOW
    echo "== screenshots (KIROKU_FAKE_NOW=${KIROKU_FAKE_NOW:-now})"
    sh tools/screenshots/run.sh
    sh tools/screenshots/keep-changed.sh
  fi

  # 5. チェック、リリースノート、コミット
  echo "== sh tools/check.sh"
  sh tools/check.sh || die "tools/check.sh failed. Fix it on $branch, commit, and continue from \"Next steps\" in docs/development.md"
  echo "== sh tools/release-notes.sh $v"
  notes=$(sh tools/release-notes.sh "$v")
  printf '%s\n' "$notes"

  git add -A -- CHANGELOG.md changes
  if [ -n "$shots" ]; then
    git add -- docs
  fi
  git commit --quiet -m "Release $v"
  echo
  echo "== committed $(git rev-parse --short HEAD) on $branch:"
  git show --stat --format='   %s' HEAD | sed 's/^/  /'
  cat <<EOF

Next steps (this script pushes and tags nothing):
  1. Read the diff: git show HEAD
  2. Push:          git push -u $remote $branch
  3. Open the pull request to main, titled "Release $v", with the release notes above as its body:
                    sh tools/release-notes.sh $v | gh pr create --base main --title "Release $v" --body-file -
  4. When it is merged, .github/workflows/tag.yml tags $v and runs Release. Then check what was published:
                    sh tools/verify-release.sh $v
EOF
}

main "$@"
