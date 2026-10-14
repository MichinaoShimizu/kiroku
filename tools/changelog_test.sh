#!/bin/sh
# tools/changelog.sh と tools/next-version.sh のテスト。tools/testdata/changelog の作り物の CHANGELOG.md と changes/ を
# 一時ディレクトリに写して動かし、結果を want*.md と比べる。tools/check.sh から動かす。
#   sh tools/changelog_test.sh
set -u
here=$(cd "$(dirname "$0")" && pwd)
fx=$here/testdata/changelog
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fails=0
fail() { printf 'FAIL: %s\n' "$*"; fails=$((fails + 1)); }
ok() { printf 'ok: %s\n' "$*"; }

# copy <fixture> → $tmp/<fixture>（毎回まっさらに）
copy() {
  rm -rf "${tmp:?}/$1"
  cp -R "$fx/$1" "$tmp/$1"
}

# expect_same <file> <want> <name>
expect_same() {
  if cmp -s "$1" "$2"; then ok "$3"; else fail "$3"; diff -u "$2" "$1" | sed 's/^/    /'; fi
}

# expect_out <want> <name> <command...>: 標準出力が want と一致する
expect_out() {
  want=$1 name=$2
  shift 2
  got=$("$@" 2>&1)
  if [ "$got" = "$want" ]; then ok "$name"; else fail "$name: got \"$got\", want \"$want\""; fi
}

cl() { sh "$here/changelog.sh" "$@"; }
nv() { sh "$here/next-version.sh" "$@"; }

# 1. 「## Unreleased」に項目がある CHANGELOG に断片をまとめる
copy merge
c=$tmp/merge/CHANGELOG.md
expect_out v0.37.0 "next-version counts the fragments (BREAKING in a fragment, 0.x → minor)" nv Unreleased "$c"
cl --check "$c" > /dev/null || fail "--check accepts the merge fixture"
cl --print "$c" > "$tmp/print.md"
cl "$c" > /dev/null || fail "merge exits 0"
expect_same "$c" "$fx/merge/want.md" "merge keeps the existing Unreleased entries, adds the fragments, orders the sections and joins duplicate headings"
left=$(cd "$tmp/merge/changes" && echo *)
if [ "$left" = README.md ]; then ok "merged fragments are deleted, README.md is kept"; else fail "left in changes/: $left"; fi
awk '/^## Unreleased/ { on = 1; next } /^## / { on = 0 } on' "$c" | sed '1{/^$/d;}' | sed '${/^$/d;}' > "$tmp/section.md"
expect_same "$tmp/print.md" "$tmp/section.md" "--print shows the section that merge writes"
expect_out v0.37.0 "next-version gives the same version after the merge" nv Unreleased "$c"
cp "$c" "$tmp/before.md"
cl "$c" > /dev/null || fail "a second merge exits 0"
expect_same "$c" "$tmp/before.md" "a second merge changes nothing (idempotent)"
# 正規化した形は変わらない：まとめた後に断片を足しても、既存の項目の並びはそのまま
printf -- '- One more fix\n' > "$tmp/merge/changes/later.fixed.md"
cl "$c" > /dev/null
awk '{ print } $0 == "- A crash on empty input" { print "- One more fix" }' "$fx/merge/want.md" > "$tmp/want-later.md"
expect_same "$c" "$tmp/want-later.md" "a later fragment is appended after the earlier ones"

# 2. 版にする（release.sh が使う）
copy merge
c=$tmp/merge/CHANGELOG.md
cl --release v0.37.0 --date 2026-10-14 "$c" > /dev/null || fail "--release exits 0"
expect_same "$c" "$fx/merge/want-release.md" "--release merges and names the section"
expect_out v0.37.0 "next-version of the released section matches" nv v0.37.0 "$c"
expect_out "changelog: \"## v0.37.0\" is already the top section and changes/ has no fragments" "--release again is a no-op" cl --release v0.37.0 --date 2026-10-14 "$c"
expect_same "$c" "$fx/merge/want-release.md" "--release again changes nothing"
if cl --release v0.37.1 "$c" > /dev/null 2>&1; then fail "--release with nothing to release fails"; else ok "--release with nothing to release fails"; fi
if cl --release 0.37.1 "$c" > /dev/null 2>&1; then fail "--release needs vX.Y.Z"; else ok "--release needs vX.Y.Z"; fi
if cl --release v0.36.0 "$c" > /dev/null 2>&1; then fail "--release of a version below the top fails"; else ok "--release of a version below the top fails"; fi
# リリースの PR を作ったあとに増えた断片は、その版の節に足す（見出しの日付はそのまま）
printf -- '- Late fix\n' > "$tmp/merge/changes/late.fixed.md"
cl --release v0.37.0 --date 2026-10-20 "$c" > /dev/null || fail "--release with a late fragment exits 0"
awk '{ print } $0 == "- A crash on empty input" { print "- Late fix" }' "$fx/merge/want-release.md" > "$tmp/want-late.md"
expect_same "$c" "$tmp/want-late.md" "--release adds a late fragment to the top version section"
copy merge
c=$tmp/merge/CHANGELOG.md
sed 's/^## v0.36.0 - 2026-10-14$/## v0.37.0 - 2026-10-14/' "$fx/merge/CHANGELOG.md" > "$c"
if cl --release v0.37.0 "$c" > /dev/null 2> "$tmp/err"; then fail "--release with the version below Unreleased fails"; fi
if grep -q 'already exists below the top section' "$tmp/err"; then ok "--release with the version below Unreleased is reported"; else fail "not reported: $(cat "$tmp/err")"; fi

# 3. 「## Unreleased」がない（リリースの直後）
copy fresh
c=$tmp/fresh/CHANGELOG.md
expect_out v0.36.1 "next-version with only Changed and Fixed fragments → patch" nv Unreleased "$c"
cl "$c" > /dev/null
expect_same "$c" "$fx/fresh/want.md" "merge creates ## Unreleased above the last release"
expect_out v0.36.1 "next-version after creating ## Unreleased" nv Unreleased "$c"
printf -- '- A new thing\n' > "$tmp/fresh/changes/thing.added.md"
expect_out v0.37.0 "an Added fragment with an existing Unreleased → minor" nv Unreleased "$c"

# 断片も「## Unreleased」もないとき
copy fresh
rm "$tmp/fresh/changes/"*.md
c=$tmp/fresh/CHANGELOG.md
if nv Unreleased "$c" > /dev/null 2>&1; then fail "next-version with nothing unreleased fails"; else ok "next-version with nothing unreleased fails"; fi
cp "$c" "$tmp/before.md"
cl "$c" > /dev/null
expect_same "$c" "$tmp/before.md" "merge without fragments leaves CHANGELOG.md alone"
rmdir "$tmp/fresh/changes"
cl --check "$c" > /dev/null || fail "--check without changes/ exits 0"

# 4. 名前か中身のおかしい断片
copy bad
c=$tmp/bad/CHANGELOG.md
if cl "$c" > "$tmp/out" 2> "$tmp/err"; then fail "bad fragments make merge fail"; else ok "bad fragments make merge fail"; fi
for want in "changes/Upper.fixed.md: the name" "changes/fix.bugfix.md: the name" "changes/notes.txt: the name" \
  "changes/heading.added.md:1: each line" "changes/empty.fixed.md: has no" "changes/prose.changed.md:1: each line" \
  "changes/orphan.removed.md:1: each line"; do
  if grep -qF "$want" "$tmp/err"; then ok "reports $want"; else fail "does not report \"$want\": $(cat "$tmp/err")"; fi
done
if grep -qF ok.added.md "$tmp/err"; then fail "a valid fragment is reported"; fi
expect_same "$c" "$fx/bad/CHANGELOG.md" "bad fragments leave CHANGELOG.md alone"
if [ -f "$tmp/bad/changes/ok.added.md" ]; then ok "bad fragments delete nothing"; else fail "a fragment was deleted"; fi
if cl --check "$c" > /dev/null 2>&1; then fail "--check fails on bad fragments"; else ok "--check fails on bad fragments"; fi
if nv Unreleased "$c" > /dev/null 2>&1; then fail "next-version fails on bad fragments"; else ok "next-version fails on bad fragments"; fi

# CRLF、リンク、ディレクトリ、隠しファイル（git に入れにくいので、ここで作る）
for case in crlf link dir hidden; do
  copy fresh
  ch=$tmp/fresh/changes
  case $case in
    crlf) printf -- '- Windows line endings\r\n' > "$ch/win.fixed.md"; want="changes/win.fixed.md:1: has a carriage return" ;;
    link) ln -s "$ch/typo.fixed.md" "$ch/link.fixed.md"; want="changes/link.fixed.md: not a regular file" ;;
    dir) mkdir "$ch/sub.fixed.md"; want="changes/sub.fixed.md: not a regular file" ;;
    hidden) printf -- '- Hidden\n' > "$ch/.hidden.fixed.md"; want="changes/.hidden.fixed.md: the name" ;;
  esac
  if cl --check "$tmp/fresh/CHANGELOG.md" > /dev/null 2> "$tmp/err"; then fail "$case: --check passes"; fi
  if grep -qF "$want" "$tmp/err"; then ok "reports $want"; else fail "$case: does not report \"$want\": $(cat "$tmp/err")"; fi
done

# 5. CHANGELOG.md の形がおかしいとき
copy fresh
c=$tmp/fresh/CHANGELOG.md
printf '\n## Unreleased\n\n### Fixed\n\n- Misplaced\n' >> "$c"
if cl "$c" > /dev/null 2> "$tmp/err"; then
  fail "a misplaced ## Unreleased passes"
elif grep -q 'must be the first "## " section' "$tmp/err"; then
  ok "a misplaced ## Unreleased is reported"
else
  fail "a misplaced ## Unreleased is not reported: $(cat "$tmp/err")"
fi

echo
if [ "$fails" -gt 0 ]; then
  echo "changelog_test: $fails failed"
  exit 1
fi
echo "changelog_test: all passed"
