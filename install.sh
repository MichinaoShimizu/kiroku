#!/bin/sh
# kiroku のインストーラー（macOS・Linux）。
#
#   curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh
#
# GitHub Releases から OS と CPU に合ったファイルを落とし、checksums.txt で確かめてから置く。
# curl で落とすので macOS の「開発元を確認できない」警告（quarantine）は付かない。
#
# 環境変数:
#   KIROKU_VERSION      入れる版（例: v0.1.1）。なければ最新
#   KIROKU_INSTALL_DIR  置き場所。なければ /usr/local/bin（書き込めなければ ~/.local/bin）
set -eu

REPO="MichinaoShimizu/kiroku"

say() { printf '%s\n' "$*"; }
die() { printf 'kiroku: %s\n' "$*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || die "curl が必要です"
command -v tar >/dev/null 2>&1 || die "tar が必要です"

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) die "この OS には対応していません: $(uname -s)。Windows は https://github.com/$REPO/releases から zip を落としてください" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) die "この CPU には対応していません: $(uname -m)" ;;
esac

# 最新の版は、releases/latest のリダイレクト先から読む（API の回数制限にかからない）
version="${KIROKU_VERSION:-}"
if [ -z "$version" ]; then
  url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") ||
    die "最新の版を調べられませんでした"
  version="${url##*/}"
fi
case "$version" in
  v[0-9]*) ;;
  [0-9]*) version="v$version" ;;
  *) die "版の形が読めません: $version" ;;
esac

file="kiroku_${version#v}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$version"

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t kiroku)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "kiroku ${version}（$os/${arch}）を落としています…"
curl -fsSL -o "$tmp/$file" "$base/$file" || die "$file を落とせませんでした（${base}）"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || die "checksums.txt を落とせませんでした"

want=$(awk -v f="$file" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || die "checksums.txt に $file がありません"
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$file" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
  got=$(shasum -a 256 "$tmp/$file" | awk '{ print $1 }')
else
  die "sha256sum か shasum が必要です"
fi
[ "$got" = "$want" ] || die "チェックサムが合いません（${file}）"

tar -xzf "$tmp/$file" -C "$tmp" kiroku || die "展開できませんでした"

dir="${KIROKU_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
    dir=/usr/local/bin
  else
    dir="$HOME/.local/bin"
  fi
fi
mkdir -p "$dir" || die "$dir を作れませんでした"
mv -f "$tmp/kiroku" "$dir/kiroku" || die "$dir に置けませんでした（KIROKU_INSTALL_DIR で場所を変えられます）"
chmod +x "$dir/kiroku"
if [ "$os" = darwin ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$dir/kiroku" 2>/dev/null || true
fi

say "入れました: $dir/kiroku（$("$dir/kiroku" --version)）"
case ":$PATH:" in
  *":$dir:"*) say "kiroku serve で始まります（kiroku help で使い方）" ;;
  *) say "$dir が PATH に入っていません。シェルの設定に次の 1 行を足してください:"
     say "  export PATH=\"$dir:\$PATH\""
     say "そのあと kiroku serve で始まります" ;;
esac
