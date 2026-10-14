#!/bin/sh
# リリースのあとに、GitHub Releases に載ったファイルを確かめる。
#   sh tools/verify-release.sh v0.37.0
#   sh tools/verify-release.sh --keep v0.37.0   # 落としたファイルを消さずに残す（場所を出す）
#
# 新しい一時ディレクトリにリリースのファイルを落とし、次を確かめる。どれか 1 つでも合わなければ 1 で終わる。
#   1. ファイルの数と名前が、.goreleaser.yaml（と release.yml が足す出どころの証明のバンドル）から決まるものと同じ
#   2. 各ファイルの sha256 が checksums.txt と合い、checksums.txt がアーカイブと SBOM をすべて、それだけを並べている
#   3. gh があれば、各アーカイブの出どころの証明（gh attestation verify。install.sh と同じ条件）。なければ飛ばしたと言う
#
# 落とすのは HTTPS だけ（install.sh と同じく --proto '=https' --tlsv1.2）。落としたものは開かず、動かさない
# （アーカイブも展開しない）。ネットワークから来た名前は、決まった形のものだけを使う。
set -eu

REPO="MichinaoShimizu/kiroku"
export LC_ALL=C

die() { printf 'verify-release: %s\n' "$*" >&2; exit 2; }

main() {
  keep=
  if [ "${1:-}" = --keep ]; then
    keep=1
    shift
  fi
  [ $# -eq 1 ] || die "usage: sh tools/verify-release.sh [--keep] vX.Y.Z"
  version=$1
  printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$' || die "not a version: $version (want vX.Y.Z)"
  ver=${version#v}
  command -v curl >/dev/null 2>&1 || die "curl is required"
  root=$(cd "$(dirname "$0")/.." && pwd)

  tmp=$(mktemp -d 2>/dev/null || mktemp -d -t kiroku-verify)
  if [ -n "$keep" ]; then
    trap 'echo "kept the downloads in $tmp"' EXIT
  else
    trap 'rm -rf "$tmp"' EXIT
  fi
  trap 'exit 130' INT
  trap 'exit 143' TERM
  mkdir "$tmp/dl"

  failed=0
  pass() { printf 'ok    %s\n' "$*"; }
  bad() { printf 'FAIL  %s\n' "$*"; failed=1; }

  # 1. 期待するファイル。手元にタグがあれば、そのタグの .goreleaser.yaml と release.yml から（なければ今のファイルから）
  from="the working tree"
  if git -C "$root" rev-parse -q --verify "refs/tags/$version^{commit}" >/dev/null 2>&1; then
    mkdir -p "$tmp/cfg/.github/workflows"
    if git -C "$root" show "$version:.goreleaser.yaml" > "$tmp/cfg/.goreleaser.yaml" &&
      git -C "$root" show "$version:.github/workflows/release.yml" > "$tmp/cfg/.github/workflows/release.yml"; then
      root=$tmp/cfg
      from="the tag $version"
    fi
  fi
  expected "$root" "$ver" > "$tmp/want.raw" || exit 2
  sort -u "$tmp/want.raw" > "$tmp/want"
  echo "expected $(wc -l < "$tmp/want" | tr -d ' ') assets for $version (from .goreleaser.yaml and release.yml in $from)"

  # 載っているファイル（GitHub の API。名前は、このリリースの download の URL の形のものだけを使う）
  api="https://api.github.com/repos/$REPO/releases/tags/$version"
  curl --proto '=https' --tlsv1.2 -fsSL -H 'Accept: application/vnd.github+json' -o "$tmp/release.json" "$api" ||
    die "could not read the release $version from $api"
  prefix="https://github.com/$REPO/releases/download/$version/"
  grep -o '"browser_download_url": *"[^"]*"' "$tmp/release.json" | sed 's/^"browser_download_url": *"//; s/"$//' > "$tmp/urls"
  : > "$tmp/got"
  while IFS= read -r u; do
    case "$u" in
      "$prefix"*) name=${u#"$prefix"} ;;
      *) bad "an asset URL outside this release: $u"; continue ;;
    esac
    if printf '%s\n' "$name" | grep -Eq '^[A-Za-z0-9][A-Za-z0-9._-]*$'; then
      printf '%s\n' "$name" >> "$tmp/got"
    else
      bad "an asset with an unexpected name: $name"
    fi
  done < "$tmp/urls"
  sort -o "$tmp/got" "$tmp/got"
  if cmp -s "$tmp/want" "$tmp/got"; then
    pass "$(wc -l < "$tmp/got" | tr -d ' ') assets with the expected names"
  else
    comm -23 "$tmp/want" "$tmp/got" | sed 's/^/FAIL  missing: /'
    comm -13 "$tmp/want" "$tmp/got" | sed 's/^/FAIL  unexpected: /'
    failed=1
  fi

  # 落とす（期待していて、載っているものだけ。名前は自分で組み立てたもの）
  comm -12 "$tmp/want" "$tmp/got" > "$tmp/fetch"
  while IFS= read -r name; do
    curl --proto '=https' --tlsv1.2 -fsSL -o "$tmp/dl/$name" "$prefix$name" || { bad "could not download $name"; rm -f "$tmp/dl/$name"; }
  done < "$tmp/fetch"

  # 2. checksums.txt
  sums=$tmp/dl/checksums.txt
  if [ -f "$sums" ]; then
    grep -v -e '\.sigstore\.json$' -e '^checksums\.txt$' "$tmp/want" > "$tmp/summed"
    if ! awk 'NF != 2 || $1 !~ /^[0-9a-f]{64}$/ || $2 !~ /^[A-Za-z0-9][A-Za-z0-9._-]*$/ { bad = 1 } END { exit bad }' "$sums"; then
      bad "checksums.txt has a line that is not \"<sha256> <file>\""
    fi
    awk '{ print $2 }' "$sums" | sort > "$tmp/listed"
    if cmp -s "$tmp/summed" "$tmp/listed"; then
      pass "checksums.txt lists the $(wc -l < "$tmp/listed" | tr -d ' ') archives and SBOMs, and nothing else"
    else
      comm -23 "$tmp/summed" "$tmp/listed" | sed 's/^/FAIL  not in checksums.txt: /'
      comm -13 "$tmp/summed" "$tmp/listed" | sed 's/^/FAIL  in checksums.txt but not expected: /'
      failed=1
    fi
    n=0
    while IFS= read -r name; do
      [ -f "$tmp/dl/$name" ] || continue
      want=$(awk -v f="$name" '$2 == f { print $1; exit }' "$sums")
      got=$(sha256 "$tmp/dl/$name")
      if [ -n "$want" ] && [ "$got" = "$want" ]; then
        n=$((n + 1))
      else
        bad "sha256 of $name is $got, checksums.txt says ${want:-nothing}"
      fi
    done < "$tmp/summed"
    pass "sha256 matches checksums.txt for $n files"
  else
    bad "checksums.txt was not downloaded"
  fi

  # 3. 出どころの証明
  if ! command -v gh >/dev/null 2>&1; then
    echo "skip  build provenance: gh (GitHub CLI) is not installed, so gh attestation verify was not run"
  else
    bundle=$tmp/dl/kiroku_$ver.sigstore.json
    grep -E '\.(tar\.gz|zip)$' "$tmp/want" > "$tmp/archives"
    while IFS= read -r name; do
      [ -f "$tmp/dl/$name" ] || continue
      how=$(attest "$tmp/dl/$name" "$version" "$bundle" "$tmp/attest.log") && rc=0 || rc=$?
      case $rc in
        0) pass "build provenance of $name ($how)" ;;
        3) echo "skip  build provenance of $name: $how" ;;
        *) bad "build provenance of $name: $how"; sed 's/^/        /' "$tmp/attest.log" | head -n 5 ;;
      esac
    done < "$tmp/archives"
  fi

  echo
  if [ "$failed" -ne 0 ]; then
    echo "verify-release: $version has problems (see FAIL above)"
    exit 1
  fi
  echo "verify-release: $version looks right"
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    die "sha256sum or shasum is required"
  fi
}

# expected <root> <version without v> は、載っているべきファイルの名前を並べる（sort 済み）。
# .goreleaser.yaml の形が下で読める形と違えば、推し量らずに止まる（このスクリプトを直す合図）
expected() {
  g=$1/.goreleaser.yaml
  r=$1/.github/workflows/release.yml
  [ -f "$g" ] && [ -f "$r" ] || { echo "verify-release: $g or $r not found" >&2; return 1; }
  # 「key: [a, b]」の 1 行から a b を出す。ちょうど 1 行でなければ失敗
  list() {
    out=$(grep -E "^ *$1: *\[[^]]*\] *(#.*)?$" "$g") || { echo "verify-release: no \"$1: [...]\" line in .goreleaser.yaml" >&2; return 1; }
    [ "$(printf '%s\n' "$out" | wc -l)" -eq 1 ] || { echo "verify-release: more than one \"$1:\" line in .goreleaser.yaml" >&2; return 1; }
    printf '%s\n' "$out" | sed 's/^[^[]*\[//; s/\].*//; s/,/ /g'
  }
  project=$(sed -n 's/^project_name: *\([A-Za-z0-9_-]*\) *$/\1/p' "$g")
  oses=$(list goos) || return 1
  arches=$(list goarch) || return 1
  # アーカイブの形式（既定）と、OS ごとの上書き（format_overrides の「- goos: X」と次の行の「formats: [Y]」）
  formats=$(grep -E '^ *- formats: *\[[^]]*\] *$' "$g" | sed 's/^[^[]*\[//; s/\].*//') || true
  overrides=$(awk '/^ *- goos: / { os = $3; next } os != "" && /^ *formats: *\[/ { f = $0; sub(/^[^[]*\[/, "", f); sub(/\].*/, "", f); print os " " f; os = "" }' "$g")
  template=$(sed -n 's/^ *name_template: *"\(.*\)" *$/\1/p' "$g" | head -n 1)
  checksum=$(awk '/^checksum:/ { on = 1; next } on && /^ *name_template:/ { print $2; exit } /^[^ ]/ { on = 0 }' "$g")
  [ -n "$project" ] && [ -n "$oses" ] && [ -n "$arches" ] && [ -n "$checksum" ] ||
    { echo "verify-release: could not read project_name, goos, goarch or the checksum name from .goreleaser.yaml" >&2; return 1; }
  case "$formats" in
    tar.gz | zip) ;;
    *) echo "verify-release: expected one archive format (tar.gz or zip) in .goreleaser.yaml, got \"$formats\"" >&2; return 1 ;;
  esac
  [ "$template" = '{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}' ] ||
    { echo "verify-release: the archive name_template in .goreleaser.yaml changed (\"$template\"); update tools/verify-release.sh" >&2; return 1; }
  sbom=
  if grep -Eq '^ *- artifacts: *archive *$' "$g"; then sbom=1; fi
  for os in $oses; do
    for arch in $arches; do
      f=$formats
      o=$(printf '%s\n' "$overrides" | awk -v os="$os" '$1 == os { print $2 }')
      [ -z "$o" ] || f=$o
      a="${project}_${2}_${os}_${arch}.$f"
      echo "$a"
      [ -z "$sbom" ] || echo "$a.sbom.json"
    done
  done
  echo "$checksum"
  # release.yml が足す、出どころの証明のバンドル（kiroku_<版>.sigstore.json）
  if grep -q 'sigstore\.json' "$r"; then echo "${project}_$2.sigstore.json"; fi
}

# attest <file> <version> <bundle> <log> は gh attestation verify を回し、どう確かめたかを出す。
# 終了コード: 0 = 合った、1 = 合わない・見つからない、3 = 確かめられなかった（gh にログインしていない・古い・GitHub に届かない）
# 条件は install.sh と同じ：このリポジトリの release.yml が GitHub のランナーで作ったもの。もとにしたのは main（tag.yml から）かタグ
attest() {
  file=$1 version=$2 bundle=$3 log=$4
  for src in api bundle; do
    if [ "$src" = bundle ]; then
      [ -f "$bundle" ] || break
      set -- --bundle "$bundle"
    else
      set --
    fi
    for ref in refs/heads/main "refs/tags/$version"; do
      st=0
      gh attestation verify "$file" --repo "$REPO" "$@" \
        --signer-workflow "$REPO/.github/workflows/release.yml" \
        --source-ref "$ref" --deny-self-hosted-runners >/dev/null 2>"$log" || st=$?
      if [ "$st" -eq 0 ]; then
        if [ "$src" = api ]; then echo "GitHub attestation API, built from $ref"; else echo "release's sigstore bundle, built from $ref"; fi
        return 0
      fi
      grep -q 'SourceRepositoryRef' "$log" || break # 合わなかったのが「もとにした ref」だけなら、もう一方で試す
    done
    # GitHub の API で確かめられない（ログインしていない・届かない）ときだけ、リリースのバンドルで試す
    if [ "$st" -eq 4 ] || grep -Eqi 'HTTP (401|403|429|5[0-9][0-9])|dial tcp|no such host|connection (refused|reset)|timeout|TLS handshake|network is unreachable' "$log"; then
      continue
    fi
    if grep -Eqi 'unknown (command|flag)' "$log"; then
      echo "this gh ($(gh --version 2>/dev/null | awk 'NR == 1 { print $3 }')) is too old for gh attestation verify"
      return 3
    fi
    echo "gh attestation verify failed"
    return 1
  done
  echo "gh could not reach GitHub or is not logged in (gh auth login), and the bundle did not verify it either"
  return 3
}

main "$@"
