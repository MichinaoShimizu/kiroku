#!/bin/sh
# tools/verify-release.sh のテスト。ネットワークには出ない：偽の curl（作り物のリリースを一時ディレクトリから返す）と
# 偽の gh（attestation verify の結果を GH_MODE で選ぶ）で動かす。tools/check.sh から動かす。
#   sh tools/verify-release_test.sh
set -u
here=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
export LC_ALL=C

fails=0
fail() { printf 'FAIL: %s\n' "$*"; fails=$((fails + 1)); }
ok() { printf 'ok: %s\n' "$*"; }

version=v9.9.9
rel=$tmp/rel
bin=$tmp/bin
mkdir -p "$rel" "$bin"

# curl … -o FILE URL：API の URL には rel にあるファイルを並べた release.json を、download の URL には rel のファイルを返す
cat > "$bin/curl" <<'EOF'
#!/bin/sh
out="" url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out=$2; shift ;;
    -H) shift ;;
    http*) url=$1 ;;
  esac
  shift
done
case "$url" in
  https://api.github.com/*)
    {
      echo '{"assets": ['
      for f in "$FAKE_REL"/*; do
        [ -f "$f" ] || continue
        printf '{"browser_download_url": "https://github.com/MichinaoShimizu/kiroku/releases/download/%s/%s"},\n' "$FAKE_VERSION" "${f##*/}"
      done
      echo ']}'
    } > "$out"
    ;;
  *) cp "$FAKE_REL/${url##*/}" "$out" ;;
esac
EOF
# gh attestation verify <file> …：確かめたファイルの名前を GH_LOG に書き、GH_MODE の結果を返す
cat > "$bin/gh" <<'EOF'
#!/bin/sh
[ "$1 $2" = "attestation verify" ] || exit 2
printf '%s\n' "${3##*/}" >> "$GH_LOG"
case "$GH_MODE" in
  ok) exit 0 ;;
  net) echo 'Error: failed to fetch attestations: dial tcp: lookup api.github.com: no such host' >&2; exit 1 ;;
  # 証明書の値（ブランチ名・ワークフロー名）に「届かない」ときの言葉が入っていても、合わないものは合わない
  evilref) printf 'Error: expected SourceRepositoryRef to be refs/heads/main, got refs/heads/fix-timeout\n' >&2; exit 1 ;;
  evilsan) echo 'Error: no matching CertificateIdentity found, last error: expected SAN to match x, got x/ci-timeout.yml (dial tcp)' >&2; exit 1 ;;
  esc) printf 'Error: no matching CertificateIdentity found \033]0;pwned\007\033[2J\n' >&2; exit 1 ;;
esac
exit 3
EOF
chmod 755 "$bin/curl" "$bin/gh"
export FAKE_REL="$rel" FAKE_VERSION="$version" GH_LOG="$tmp/gh.log"

run() { PATH="$bin:$PATH" GH_MODE=$1 sh "$here/verify-release.sh" "$version" > "$tmp/out" 2>&1; }

# 載っているべきファイルの名前は、空のリリースで動かしたときの「missing:」から読む（.goreleaser.yaml の読み方はスクリプトに任せる）
run ok
sed -n 's/^FAIL  missing: //p' "$tmp/out" > "$tmp/names"
if [ ! -s "$tmp/names" ]; then
  echo "FAIL: could not read the expected asset names:"
  sed 's/^/    /' "$tmp/out"
  exit 1
fi
: > "$tmp/sums"
while IFS= read -r name; do
  printf 'dummy %s\n' "$name" > "$rel/$name"
  case "$name" in
    *.sigstore.json | checksums.txt) ;;
    *) printf '%s  %s\n' "$( (sha256sum "$rel/$name" 2>/dev/null || shasum -a 256 "$rel/$name") | cut -d' ' -f1)" "$name" >> "$tmp/sums" ;;
  esac
done < "$tmp/names"
cp "$tmp/sums" "$rel/checksums.txt"

# expect <mode> <exit code> <text in the output> <name>
expect() {
  : > "$GH_LOG"
  rc=0
  run "$1" || rc=$?
  if [ "$rc" -eq "$2" ] && grep -q "$3" "$tmp/out"; then
    ok "$4"
  else
    fail "$4: exit $rc (want $2), want \"$3\" in the output"
    sed 's/^/    /' "$tmp/out"
  fi
}

expect ok 0 'looks right' "every asset verifies"
# release.yml は SBOM と checksums.txt も証明するので、それも確かめる
for n in checksums.txt "$(grep -m1 '\.sbom\.json$' "$tmp/names")" "$(grep -m1 '\.tar\.gz$' "$tmp/names")"; do
  if grep -qxF "$n" "$GH_LOG"; then ok "attests $n"; else fail "did not attest $n"; fi
done
if grep -q '\.sigstore\.json$' "$GH_LOG"; then fail "attested the sigstore bundle itself"; else ok "skips the sigstore bundle"; fi

expect net 0 'skip  build provenance' "unreachable GitHub is a skip, not a failure"
expect evilref 1 'FAIL  build provenance' "a ref mismatch whose ref says timeout fails"
expect evilsan 1 'FAIL  build provenance' "a SAN mismatch whose name says dial tcp fails"
expect esc 1 'FAIL  build provenance' "a mismatch with control characters fails"
if LC_ALL=C grep -q "$(printf '[\001-\010\013-\037\177]')" "$tmp/out"; then
  fail "gh's output reached the terminal with control characters"
else
  ok "gh's output has no control characters"
fi

if [ "$fails" -ne 0 ]; then
  echo "verify-release_test: $fails failed"
  exit 1
fi
echo "verify-release_test: all passed"
