#!/bin/sh
# Rebuild a released kiroku binary from its tag and compare it, byte for byte,
# with the binary in the release archive on GitHub.
#
# Usage: sh tools/reproduce.sh <version> <os> <arch>
#   e.g. sh tools/reproduce.sh v0.13.3 linux amd64
#        os: darwin | linux | windows   arch: amd64 | arm64
#
# Needs git, go (any version that can download the toolchain named in the
# tag's go.mod), curl, tar (unzip for windows) and sha256sum or shasum.
# The build mirrors .goreleaser.yaml: CGO_ENABLED=0, -trimpath,
# -ldflags "-s -w -X main.version=<version without v>", from a clean clone
# of the tag (Go stamps the module version and commit into the binary).
# Environment: KIROKU_REPO_URL to clone from somewhere else (default: GitHub).
set -eu

REPO="MichinaoShimizu/kiroku"

die() { printf 'reproduce: %s\n' "$*" >&2; exit 1; }

[ $# -eq 3 ] || die "usage: sh tools/reproduce.sh <version> <os> <arch>"
version=$1 os=$2 arch=$3
case "$version" in
  v[0-9]*) ;;
  [0-9]*) version="v$version" ;;
  *) die "unexpected version format: $version" ;;
esac
case "$os" in darwin | linux | windows) ;; *) die "unsupported os: $os" ;; esac
case "$arch" in amd64 | arm64) ;; *) die "unsupported arch: $arch" ;; esac
for c in git go curl tar; do
  command -v "$c" >/dev/null 2>&1 || die "$c is required"
done

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    die "sha256sum or shasum is required"
  fi
}

ver=${version#v}
exe=kiroku
ext=tar.gz
if [ "$os" = windows ]; then
  exe=kiroku.exe
  ext=zip
  command -v unzip >/dev/null 2>&1 || die "unzip is required"
fi
file="kiroku_${ver}_${os}_${arch}.${ext}"
base="https://github.com/$REPO/releases/download/$version"

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t kiroku-reproduce)
trap 'rm -rf "$tmp"' EXIT INT TERM
mkdir "$tmp/release" "$tmp/build"

# 1. The released binary, checked against the release's checksums.txt.
curl -fsSL -o "$tmp/release/$file" "$base/$file" || die "could not download $base/$file"
curl -fsSL -o "$tmp/release/checksums.txt" "$base/checksums.txt" || die "could not download checksums.txt"
want=$(awk -v f="$file" '$2 == f { print $1 }' "$tmp/release/checksums.txt")
[ -n "$want" ] || die "$file is not in checksums.txt"
[ "$(sha256 "$tmp/release/$file")" = "$want" ] || die "$file does not match checksums.txt"
if [ "$ext" = zip ]; then
  unzip -p "$tmp/release/$file" "$exe" >"$tmp/release/$exe"
else
  tar -xzf "$tmp/release/$file" -C "$tmp/release" "$exe"
fi

# 2. The same binary, rebuilt from a clean clone of the tag with the
#    toolchain the tag's go.mod names.
git clone --quiet --branch "$version" "${KIROKU_REPO_URL:-https://github.com/$REPO.git}" "$tmp/src" 2>/dev/null ||
  die "could not clone $version"
toolchain=$(awk '$1 == "toolchain" { print $2; exit }' "$tmp/src/go.mod")
[ -n "$toolchain" ] || toolchain="go$(awk '$1 == "go" { print $2; exit }' "$tmp/src/go.mod")"
# GOENV=off and an empty GOFLAGS keep local Go settings out of the build.
(
  cd "$tmp/src"
  env GOENV=off GOFLAGS= GOTOOLCHAIN="$toolchain" CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    GOAMD64=v1 GOARM64=v8.0 GOEXPERIMENT= \
    go build -trimpath -ldflags "-s -w -X main.version=$ver" -o "$tmp/build/$exe" .
)

released=$(sha256 "$tmp/release/$exe")
rebuilt=$(sha256 "$tmp/build/$exe")
printf 'released %s  %s/%s\n' "$released" "$file" "$exe"
printf 'rebuilt  %s  %s (%s, %s/%s)\n' "$rebuilt" "$exe" "$toolchain" "$os" "$arch"
if [ "$released" = "$rebuilt" ]; then
  printf 'OK: %s %s/%s is reproducible\n' "$version" "$os" "$arch"
else
  die "MISMATCH: the rebuilt binary differs from the released one"
fi
