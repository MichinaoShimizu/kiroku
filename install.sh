#!/bin/sh
# kiroku のインストーラー（macOS・Linux）。
#
#   curl -fsSL https://raw.githubusercontent.com/MichinaoShimizu/kiroku/main/install.sh | sh
#
# GitHub Releases から OS と CPU に合ったファイルを落とし、checksums.txt で確かめてから置く。
# GitHub CLI（gh）があれば、出どころの証明（artifact attestation）も確かめる。
# curl で落とすので macOS の「開発元を確認できない」警告（quarantine）は付かない。
#
# 環境変数:
#   KIROKU_VERSION      入れる版（例: v0.1.1）。なければ最新
#   KIROKU_INSTALL_DIR  置き場所。なければ /usr/local/bin（書き込めなければ ~/.local/bin）
#   KIROKU_SKIP_ATTESTATION=1  gh があっても、出どころの証明（gh attestation verify）を確かめない
#   KIROKU_REQUIRE_ATTESTATION=1  出どころの証明を必ず確かめる（gh がない・ログインしていない・GitHub に届かないときも止める）
#
# 中身はすべて main の中にあり、いちばん最後の行で呼ぶ。curl | sh で途中までしか落とせなかったときに、
# 途中までの行だけが動いてしまわないように。
set -eu

REPO="MichinaoShimizu/kiroku"

say() { printf '%s\n' "$*"; }
die() { printf 'kiroku: %s\n' "$*" >&2; exit 1; }

main() {
command -v curl >/dev/null 2>&1 || die "curl is required"
command -v tar >/dev/null 2>&1 || die "tar is required"

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) die "unsupported OS: $(uname -s). On Windows, download the zip from https://github.com/$REPO/releases" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) die "unsupported CPU: $(uname -m)" ;;
esac

# 最新の版は、releases/latest のリダイレクト先から読む（API の回数制限にかからない）
version="${KIROKU_VERSION:-}"
if [ -z "$version" ]; then
  url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") ||
    die "could not check the latest version"
  version="${url##*/}"
fi
case "$version" in
  v[0-9]*) ;;
  [0-9]*) version="v$version" ;;
  *) die "unexpected version format: $version" ;;
esac

file="kiroku_${version#v}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$version"

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t kiroku)
new=""
cleanup() {
  rm -rf "$tmp"
  if [ -n "$new" ]; then rm -f "$new"; fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

say "downloading kiroku ${version} ($os/${arch})…"
curl -fsSL -o "$tmp/$file" "$base/$file" || die "could not download $file (${base})"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || die "could not download checksums.txt"

want=$(awk -v f="$file" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || die "$file is not listed in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$file" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
  got=$(shasum -a 256 "$tmp/$file" | awk '{ print $1 }')
else
  die "sha256sum or shasum is required"
fi
[ "$got" = "$want" ] || die "checksum mismatch (${file})"

# gh があれば、このリポジトリの release.yml で作られたファイルか（artifact attestation）も確かめる。
# 出どころの証明は v0.12.0 から。gh にログインしていない・GitHub に届かないときは、checksums.txt は
# 合っているので警告だけにする（KIROKU_REQUIRE_ATTESTATION=1 なら止める）。証明が見つからない・合わないときは止める
require=${KIROKU_REQUIRE_ATTESTATION:-}
skipped="checksums.txt matched, so installing anyway (set KIROKU_SKIP_ATTESTATION=1 to skip this check, or KIROKU_REQUIRE_ATTESTATION=1 to make it required)"
# 確かめられなかったとき：ふだんは警告だけ、KIROKU_REQUIRE_ATTESTATION=1 なら止める
unchecked() {
  if [ "$require" = 1 ]; then
    die "$1, and KIROKU_REQUIRE_ATTESTATION=1 is set"
  fi
  say "warning: $1; $skipped"
}
attest=yes
if [ "${KIROKU_SKIP_ATTESTATION:-}" = 1 ]; then
  [ "$require" != 1 ] || die "KIROKU_SKIP_ATTESTATION=1 and KIROKU_REQUIRE_ATTESTATION=1 cannot be used together"
  attest=no
elif ! command -v gh >/dev/null 2>&1; then
  attest=no
  [ "$require" != 1 ] || die "the GitHub CLI (gh) is needed to check the build provenance, and KIROKU_REQUIRE_ATTESTATION=1 is set (https://cli.github.com/)"
else
  v=${version#v}
  major=${v%%.*}
  rest=${v#*.}
  minor=${rest%%.*}
  case "$major$minor" in
    *[!0-9]* | "") ;;
    *) if [ "$major" -eq 0 ] && [ "$minor" -lt 12 ]; then attest=no; fi ;;
  esac
  [ "$attest" = yes ] || [ "$require" != 1 ] || die "$version has no build provenance (it starts at v0.12.0), and KIROKU_REQUIRE_ATTESTATION=1 is set"
fi
if [ "$attest" = yes ]; then
  # 作ったのは release.yml（tag.yml から呼ばれても、証明書に載るのは呼ばれた側の release.yml）。GitHub のランナーで
  # 動いたものだけを認める。もとにしたのは、tag.yml が main で動いたときは main、タグを手で push したときはそのタグ
  status=1
  for ref in refs/heads/main "refs/tags/$version"; do
    status=0
    gh attestation verify "$tmp/$file" --repo "$REPO" \
      --signer-workflow "$REPO/.github/workflows/release.yml" \
      --source-ref "$ref" --deny-self-hosted-runners >/dev/null 2>"$tmp/attest.log" || status=$?
    # 合わなかったのが「もとにした ref」だけなら、もう一方で試す
    if [ "$status" -eq 0 ] || ! grep -q 'SourceRepositoryRef' "$tmp/attest.log"; then break; fi
  done
  if [ "$status" -eq 0 ]; then
    say "verified the build provenance with gh"
  elif [ "$status" -eq 4 ]; then
    # 4 は gh にログインしていないとき
    unchecked "gh is not logged in (\"gh auth login\"), so the build provenance was not checked"
  elif grep -Eqi 'unknown (command|flag)' "$tmp/attest.log"; then
    # gh attestation（や --source-ref などの旗）がない古い gh。どの gh が使われたのか分かるように版も見せる
    # （PATH の前のほうに古い gh があることもある）
    ghver=$(gh --version 2>/dev/null | awk 'NR == 1 { print $3 }')
    unchecked "this gh ${ghver:+($ghver) }is too old to check the build provenance (update it from your package manager or https://cli.github.com/)"
  elif grep -Eqi 'HTTP (401|403|429|5[0-9][0-9])|Sigstore verifier|dial tcp|no such host|connection (refused|reset)|timeout|TLS handshake|network is unreachable' "$tmp/attest.log"; then
    # GitHub に届かない・回数制限など。証明が合わないのとは別
    grep . "$tmp/attest.log" | head -n 3 | sed 's/^/         /' >&2
    unchecked "gh could not reach GitHub, so the build provenance was not checked"
  else
    # 証明が見つからない（HTTP 404）・合わない
    grep . "$tmp/attest.log" >&2 || true
    die "the build provenance of $file does not match $REPO (gh attestation verify failed; set KIROKU_SKIP_ATTESTATION=1 to skip this check)"
  fi
fi

tar -xzf "$tmp/$file" -C "$tmp" kiroku || die "could not extract the archive"

dir="${KIROKU_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
    dir=/usr/local/bin
  else
    dir="$HOME/.local/bin"
  fi
fi
mkdir -p "$dir" || die "could not create $dir"
# いったん同じ場所の .kiroku.new に写してから名前を変える（同じファイルシステムの中なので、入れ替えは一瞬）。
# 別のファイルシステムへの mv は、古いのを消してから写すので、途中で止まると壊れた kiroku が残るため
new="$dir/.kiroku.new"
if ! cp "$tmp/kiroku" "$new" 2>/dev/null || ! chmod +x "$new"; then
  die "could not install into $dir (set KIROKU_INSTALL_DIR to choose another place)"
fi
if [ "$os" = darwin ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$new" 2>/dev/null || true
fi
mv -f "$new" "$dir/kiroku" || die "could not install into $dir (set KIROKU_INSTALL_DIR to choose another place)"
new=""

say "installed $("$dir/kiroku" --version) to $dir/kiroku"

# PATH の前のほうに別の kiroku があると、そちらが動く（/usr/local/bin の古い版など）
real_dir=$(cd "$dir" && pwd -P)
first=""
old_ifs=$IFS
IFS=:
set -f
for p in $PATH; do
  [ -n "$p" ] || continue
  if [ -f "$p/kiroku" ] && [ -x "$p/kiroku" ]; then
    first=$p
    break
  fi
done
IFS=$old_ifs
set +f
if [ -n "$first" ] && [ "$(cd "$first" 2>/dev/null && pwd -P)" != "$real_dir" ]; then
  say "warning: another kiroku comes first in your PATH, so \"kiroku\" runs $first/kiroku, not the one just installed."
  say "         Remove it, or put $dir before $first in your PATH"
fi
case ":$PATH:" in
  *":$dir:"*) say "run \"kiroku serve\" to start (\"kiroku help\" for usage)" ;;
  *)
     # 書き足す先と書き方はシェルごとに違う（fish に export PATH はない）。分からないときは今までどおりの言い方
     line="export PATH=\"$dir:\$PATH\""
     # ~ は見せるためのもので、展開はしない（shellcheck の SC2088 はそのため）
     # shellcheck disable=SC2088
     case "${SHELL:-}" in
       */fish) rc="~/.config/fish/config.fish"; line="fish_add_path \"$dir\"" ;;
       */zsh) rc="~/.zshrc" ;;
       # bash がログイン時に読むのは、macOS では ~/.bash_profile、Linux では ~/.bashrc
       */bash) if [ "$os" = darwin ]; then rc="~/.bash_profile"; else rc="~/.bashrc"; fi ;;
       *) rc="your shell config (e.g. ~/.zshrc or ~/.bashrc)" ;;
     esac
     say "$dir is not in your PATH. Add this line to $rc:"
     say "  $line"
     say "then open a new terminal and run \"kiroku serve\" to start (\"kiroku help\" for usage)"
     say "or start it now without opening one: \"$dir/kiroku\" serve"
     ;;
esac
say "tip: Claude Code deletes conversations older than 30 days by default. To keep more history for kiroku, set \"cleanupPeriodDays\": 3650 in ~/.claude/settings.json"
say "     https://code.claude.com/docs/en/settings-reference#cleanupperioddays"
}

main "$@"
