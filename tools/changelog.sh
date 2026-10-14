#!/bin/sh
# changes/ の変更の断片（PR ごとに 1 つ以上のファイル。形式は changes/README.md）を、CHANGELOG.md の「## Unreleased」にまとめる。
# PR が CHANGELOG.md を直接書き換えないので、並んだ PR どうしがぶつからない。「## Unreleased」にすでにある項目もそのまま残し、
# 断片の項目はその後ろに足す。節（### Added など）は下の順に並べ直し、同じ節が 2 つあれば 1 つにする。
#
#   sh tools/changelog.sh                      # 断片を「## Unreleased」にまとめ、まとめた断片を消す
#   sh tools/changelog.sh --release v0.37.0    # まとめたうえで「## Unreleased」を「## v0.37.0 - <今日（UTC）>」にする（--date で日付を指定）。
#                                              # いちばん上がもう「## v0.37.0」なら、あとから増えた断片をその節に足す
#   sh tools/changelog.sh --print              # まとめた「## Unreleased」の中身を出すだけ（何も書き換えない。next-version.sh が使う）
#   sh tools/changelog.sh --check              # 断片の名前と中身を確かめるだけ
#   最後の引数で CHANGELOG.md を指定できる（断片はその隣の changes/ から読む）
#
# 何度動かしても同じ結果になる（まとめる断片がなく、--release でもなければ、CHANGELOG.md に触らない）。
# 名前か中身のおかしい断片が 1 つでもあれば、何も書き換えずに理由を出して 1 で終わる。
set -eu
export LC_ALL=C

# 節の順（Keep a Changelog の順）。断片の名前の <section> もこの 6 つ
sections="added changed deprecated removed fixed security"

die() { printf 'changelog: %s\n' "$*" >&2; exit 1; }
usage() { die "usage: sh tools/changelog.sh [--check | --print | --release vX.Y.Z [--date YYYY-MM-DD]] [CHANGELOG.md]"; }

mode=merge
version=
date=
file=
while [ $# -gt 0 ]; do
  case "$1" in
    --check) mode=check ;;
    --print) mode=print ;;
    --release) [ $# -ge 2 ] || usage; mode=release; version=$2; shift ;;
    --date) [ $# -ge 2 ] || usage; date=$2; shift ;;
    -*) usage ;;
    *) [ -z "$file" ] || usage; file=$1 ;;
  esac
  shift
done
[ -n "$file" ] || file=$(cd "$(dirname "$0")/.." && pwd)/CHANGELOG.md
[ -f "$file" ] || die "$file not found"
dir=$(dirname "$file")/changes

if [ "$mode" = release ]; then
  printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$' || die "not a version: $version (want vX.Y.Z)"
  [ -n "$date" ] || date=$(date -u +%Y-%m-%d)
fi
if [ -n "$date" ]; then
  printf '%s\n' "$date" | grep -Eq '^[0-9]{4}-[0-9]{2}-[0-9]{2}$' || die "not a date: $date (want YYYY-MM-DD)"
  [ "$mode" = release ] || usage
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# 1. 断片を確かめ、節ごとに項目を集める（$tmp/<section>）。まとめた断片の一覧は $tmp/merged
: > "$tmp/merged"
for s in $sections; do : > "$tmp/$s"; done
bad=0
if [ -d "$dir" ]; then
  for f in "$dir"/* "$dir"/.[!.]* "$dir"/..?*; do
    [ -e "$f" ] || [ -L "$f" ] || continue # 何もないときの glob そのもの
    name=${f##*/}
    [ "$name" != README.md ] || continue
    if ! printf '%s\n' "$name" | grep -Eq '^[a-z0-9][a-z0-9-]*\.(added|changed|deprecated|removed|fixed|security)\.md$'; then
      printf 'changelog: changes/%s: the name must be <short-name>.<section>.md, with a short name of lowercase letters, digits and "-", and a section of %s\n' "$name" "$(echo "$sections" | sed 's/ /, /g')" >&2
      bad=1
      continue
    fi
    if [ -L "$f" ] || [ ! -f "$f" ]; then
      printf 'changelog: changes/%s: not a regular file\n' "$name" >&2
      bad=1
      continue
    fi
    # 中身は「- 」で始まる項目だけ。項目の続きの行は空白 2 つ以上で字下げする。空の行は消す（項目は詰めて並べる）
    if ! awk -v n="changes/$name" '
      /\r/ { printf "changelog: %s:%d: has a carriage return (use LF line endings)\n", n, NR > "/dev/stderr"; bad = 1; next }
      /^[ \t]*$/ { next }
      /^- [^ \t]/ { items++; print; next }
      /^  +[^ \t]/ && items { print; next }
      { printf "changelog: %s:%d: each line must start a \"- \" bullet or continue one (indented by two spaces): %s\n", n, NR, $0 > "/dev/stderr"; bad = 1 }
      END {
        if (!items && !bad) { printf "changelog: %s: has no \"- \" bullet\n", n > "/dev/stderr"; bad = 1 }
        exit bad
      }' "$f" > "$tmp/one"; then
      bad=1
      continue
    fi
    sec=${name%.md}
    sec=${sec##*.}
    cat "$tmp/one" >> "$tmp/$sec"
    printf '%s\n' "$f" >> "$tmp/merged"
  done
fi
[ "$bad" -eq 0 ] || die "fix the files above in changes/ (see changes/README.md)"
if [ "$mode" = check ]; then
  echo "changelog: $(grep -c . "$tmp/merged" || true) fragment(s) in changes/ are valid"
  exit 0
fi

# 2. CHANGELOG.md を「前置き」「まとめる節の中身」「残り」に分け、まとめる節の中身と断片を節ごとにまとめる。
#    まとめる節は「## Unreleased」。--release vX.Y.Z で、いちばん上の節がもう「## vX.Y.Z」なら（リリースの PR を作ったあとに
#    断片が増えたとき）、その節に足す（見出しの日付はそのまま）
awk -v tmp="$tmp" -v order="$sections" -v heading="${version:+$version - $date}" -v ver="$version" -v mode="$mode" '
  function fail(msg) { print "changelog: " msg > "/dev/stderr"; err = 1; exit }
  BEGIN {
    n = split(order, known, " ")
    for (i = 1; i <= n; i++) { title = toupper(substr(known[i], 1, 1)) substr(known[i], 2); names[i] = title; isknown[title] = 1 }
    part = "head"; cur = ""
  }
  /^## / {
    if ($2 == "Unreleased" || (ver != "" && $2 == ver)) {
      if ($2 == ver && part != "head") fail("\"## " ver "\" already exists below the top section")
      if (part == "unrel") fail("more than one \"## Unreleased\" heading")
      if (part == "rest") fail("\"## Unreleased\" must be the first \"## \" section")
      if ($2 == ver) keep = $0
      part = "unrel"; cur = ""; next
    }
    part = "rest"
  }
  part == "head" { head[++nh] = $0; next }
  part == "rest" { rest[++nr] = $0; next }
  # まとめる節の中
  /^### / {
    cur = substr($0, 5); sub(/[ \t]+$/, "", cur)
    if (!(cur in seen)) { seen[cur] = 1; if (!(cur in isknown)) extra[++ne] = cur }
    next
  }
  /^[ \t]*$/ { next }
  {
    if (cur == "") pre[++np] = $0
    else body[cur] = body[cur] $0 "\n"
  }
  END {
    if (err) exit 2
    # 断片の項目を、同じ節の既存の項目の後ろに足す
    for (i = 1; i <= n; i++) {
      f = tmp "/" known[i]
      while ((getline line < f) > 0) body[names[i]] = body[names[i]] line "\n"
      close(f)
    }
    out = ""
    for (i = 1; i <= np; i++) out = out pre[i] "\n"
    if (np) out = out "\n"
    for (i = 1; i <= n; i++) if (body[names[i]] != "") out = out "### " names[i] "\n\n" body[names[i]] "\n"
    for (i = 1; i <= ne; i++) if (body[extra[i]] != "") out = out "### " extra[i] "\n\n" body[extra[i]] "\n"
    sub(/\n+$/, "\n", out)
    if (mode == "print") { printf "%s", out; exit 0 }
    # 前置きの終わりの空の行は 1 つにそろえる
    while (nh > 0 && head[nh] ~ /^[ \t]*$/) nh--
    for (i = 1; i <= nh; i++) print head[i]
    if (nh) print ""
    if (out != "") {
      print (keep != "" ? keep : "## " (heading != "" ? heading : "Unreleased"))
      print ""
      printf "%s", out
      if (nr) print ""
    }
    for (i = 1; i <= nr; i++) print rest[i]
    print (out != "" ? "content" : "") > (tmp "/content")
    print keep > (tmp "/keep")
  }
' "$file" > "$tmp/new" || exit 1

if [ "$mode" = print ]; then
  cat "$tmp/new"
  exit 0
fi

content=$(cat "$tmp/content")
keep=$(cat "$tmp/keep")
merged=$(grep -c . "$tmp/merged" || true)
if [ "$mode" = release ]; then
  if [ "$merged" -eq 0 ] && [ -n "$keep" ]; then
    echo "changelog: \"## $version\" is already the top section and changes/ has no fragments"
    exit 0
  fi
  [ -n "$content" ] || die "nothing to release: \"## Unreleased\" is missing or empty and changes/ has no fragments"
elif [ "$merged" -eq 0 ]; then
  echo "changelog: no fragments in changes/; $file is unchanged"
  exit 0
fi

if ! cmp -s "$tmp/new" "$file"; then
  cat "$tmp/new" > "$file" # cat で書く（mv だと、mktemp の 0600 の権限になる）
fi
while IFS= read -r f; do
  rm -f "$f"
done < "$tmp/merged"
if [ -n "$keep" ]; then
  echo "changelog: merged $merged fragment(s) into \"$keep\""
elif [ "$mode" = release ]; then
  echo "changelog: merged $merged fragment(s) into \"## $version - $date\""
else
  echo "changelog: merged $merged fragment(s) into \"## Unreleased\""
fi
