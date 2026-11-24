package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeGH は、gh の代わりに動く sh のスクリプトを置き、releaseRepo と ghCommand をそれに向ける。
// release view には latest を返し、release download には dir の中の --pattern のファイルを標準出力に書く。
// mode で失敗のしかたを選ぶ（GH_FAKE_MODE）。返す関数は、起動されたときの引数（1 回 1 行）。
func fakeGH(t *testing.T, latest, mode string) (dir string, calls func() []string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake gh is a shell script")
	}
	dir = t.TempDir()
	log := filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + log + `"
case "$GH_FAKE_MODE" in
  auth) echo "HTTP 404: Not Found (https://api.github.com/repos/corp/kiroku/releases/latest)" >&2; exit 1 ;;
  esc) printf 'bad \033[31mred\033[0m \342\200\256 thing\nsecond line\n' >&2; exit 1 ;;
  hangchild) sleep 30 & echo "` + latest + `"; exit 0 ;;
  hang) sleep 30; exit 0 ;;
  bigstderr) head -c 100000 /dev/zero | tr '\0' w >&2; echo "` + latest + `"; exit 0 ;;
esac
if [ "$1 $2" = "release view" ]; then
  [ -n "` + latest + `" ] || { echo "release not found" >&2; exit 1; }
  echo "` + latest + `"
  exit 0
fi
if [ "$1 $2" = "release download" ]; then
  pat=""
  while [ $# -gt 0 ]; do [ "$1" = --pattern ] && pat=$2; shift; done
  [ -f "` + dir + `/$pat" ] || { echo "no assets match the file pattern" >&2; exit 1; }
  if [ "$GH_FAKE_MODE" = big ]; then head -c 4096 /dev/zero; exit 0; fi
  cat "` + dir + `/$pat"
  exit 0
fi
exit 2
`
	gh := filepath.Join(dir, "gh")
	if err := os.WriteFile(gh, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	oldRepo, oldCmd := releaseRepo, ghCommand
	t.Cleanup(func() { releaseRepo, ghCommand = oldRepo, oldCmd })
	releaseRepo, ghCommand = "github.com/corp/kiroku", gh
	t.Setenv("GH_FAKE_MODE", mode)
	return dir, func() []string {
		b, _ := os.ReadFile(log)
		s := strings.TrimSpace(string(b))
		if s == "" {
			return nil
		}
		return strings.Split(s, "\n")
	}
}

// putRelease は、偽の gh が返す tag の linux/amd64 のアーカイブと checksums.txt を dir に置く。
func putRelease(t *testing.T, dir, tag, body string) string {
	t.Helper()
	name := assetName(tag, "linux", "amd64")
	arc := makeArchive(t, "linux", [2]string{"README.md", "readme"}, [2]string{"kiroku", body})
	sum := sha256.Sum256(arc)
	os.WriteFile(filepath.Join(dir, name), arc, 0o644)
	os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(hex.EncodeToString(sum[:])+"  "+name+"\n"), 0o644)
	return name
}

// releaseRepo が埋め込まれたビルドは、gh で最新の版を確かめて落とし、checksums.txt で確かめてから入れかえる。
// gh はシェルを通さず、決まった引数だけで起動する。
func TestGHUpdate(t *testing.T) {
	dir, calls := fakeGH(t, "v9.9.9", "")
	name := putRelease(t, dir, "v9.9.9", "kiroku v9.9.9")

	if v, err := latestVersion(newUpdateClient()); err != nil || v != "v9.9.9" {
		t.Fatalf("latestVersion = %q, %v", v, err)
	}
	exe := filepath.Join(t.TempDir(), "kiroku")
	os.WriteFile(exe, []byte("old"), 0o755)
	if err := selfUpdate(newUpdateClient(), "v9.9.9", "linux", "amd64", exe); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "kiroku v9.9.9" {
		t.Fatalf("入れかわっていない: %q", got)
	}
	want := []string{
		"release view --repo github.com/corp/kiroku --json tagName --jq .tagName",
		"release download v9.9.9 --repo github.com/corp/kiroku --pattern " + name + " --output -",
		"release download v9.9.9 --repo github.com/corp/kiroku --pattern checksums.txt --output -",
	}
	if got := calls(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("gh の引数:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// チェックサムが合わなければ入れかえない
	os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(strings.Repeat("0", 64)+"  "+name+"\n"), 0o644)
	os.WriteFile(exe, []byte("old"), 0o755)
	if err := selfUpdate(newUpdateClient(), "v9.9.9", "linux", "amd64", exe); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("チェックサム違いを通した: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Fatal("失敗したのに元の kiroku が変わった")
	}
}

// kiroku update をとおしで: gh で最新を確かめて入れかえ、--check は案内だけ。
func TestGHRunUpdate(t *testing.T) {
	dir, _ := fakeGH(t, "v9.9.9", "")
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("the fake release has only linux/amd64")
	}
	putRelease(t, dir, "v9.9.9", "kiroku v9.9.9")
	exe := fakeExe(t)
	os.WriteFile(exe, []byte("old"), 0o755)
	version = "1.0.0"
	out := captureOutput(t, func() {
		if err := dispatch([]string{"update", "--check"}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "v1.0.0 → v9.9.9") {
		t.Errorf("--check の出力: %q", out)
	}
	captureOutput(t, func() {
		if err := dispatch([]string{"update"}); err != nil {
			t.Error(err)
		}
	})
	if got, _ := os.ReadFile(exe); string(got) != "kiroku v9.9.9" {
		t.Fatalf("入れかわっていない: %q", got)
	}
}

// gh がない・ログインしていない・版やファイルがない・おかしな版を返すときは、元の kiroku を残し、どうすればよいかを言う。
func TestGHFailures(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "kiroku")
	check := func(t *testing.T, err error, want string) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want %q", err, want)
		}
		if got, _ := os.ReadFile(exe); string(got) != "old" {
			t.Errorf("元の kiroku が変わった: %q", got)
		}
	}
	os.WriteFile(exe, []byte("old"), 0o755)

	t.Run("no gh", func(t *testing.T) {
		fakeGH(t, "v9.9.9", "")
		ghCommand = filepath.Join(t.TempDir(), "gh") // ない
		_, err := latestVersion(newUpdateClient())
		check(t, err, "needs the GitHub CLI (gh)")
		check(t, selfUpdate(newUpdateClient(), "v9.9.9", "linux", "amd64", exe), "needs the GitHub CLI (gh)")
	})
	t.Run("not logged in or no access", func(t *testing.T) {
		fakeGH(t, "v9.9.9", "auth")
		_, err := latestVersion(newUpdateClient())
		check(t, err, "gh auth status")
		check(t, selfUpdate(newUpdateClient(), "v9.9.9", "linux", "amd64", exe), "gh auth status")
	})
	t.Run("no release", func(t *testing.T) {
		fakeGH(t, "", "")
		_, err := latestVersion(newUpdateClient())
		check(t, err, "release not found")
	})
	t.Run("no file for this version", func(t *testing.T) {
		fakeGH(t, "v9.9.9", "")
		check(t, selfUpdate(newUpdateClient(), "v9.9.9", "linux", "amd64", exe), "that version may not exist; see https://github.com/corp/kiroku/releases")
	})
	t.Run("odd tag", func(t *testing.T) {
		for _, tag := range []string{"v1.2.3/../x", "latest", "-v1.2.3", "--repo=evil/x", "../../etc", "v1.1.0\\nv9.9.9"} {
			fakeGH(t, tag, "")
			if v, err := latestVersion(newUpdateClient()); err == nil || !strings.Contains(err.Error(), "unexpected release version") {
				t.Errorf("%q: latestVersion = %q, %v", tag, v, err)
			}
		}
	})
	t.Run("too large", func(t *testing.T) {
		dir, _ := fakeGH(t, "v9.9.9", "big")
		putRelease(t, dir, "v9.9.9", "kiroku v9.9.9")
		old := ghMaxStdout
		t.Cleanup(func() { ghMaxStdout = old })
		ghMaxStdout = 1024
		check(t, selfUpdate(newUpdateClient(), "v9.9.9", "linux", "amd64", exe), "larger than")
	})
}

// gh のエラーの文は、端末を操作する文字を取り除き、最初の行だけを出す。
func TestGHErrorIsTerminalSafe(t *testing.T) {
	fakeGH(t, "v9.9.9", "esc")
	_, err := latestVersion(newUpdateClient())
	if err == nil {
		t.Fatal("no error")
	}
	msg := err.Error()
	if strings.ContainsAny(msg, "\x1b\u202e\n") || strings.Contains(msg, "second line") || !strings.Contains(msg, "bad [31mred[0m  thing") {
		t.Errorf("err = %q", msg)
	}
}

// 埋め込まれた releaseRepo が HOST/OWNER/REPO の形でなければ、gh を起動しない（- で始まる部分はオプションと取り違えうる）。
func TestGHRejectsOddRepo(t *testing.T) {
	for _, repo := range []string{"-R/x/y", "github.com/-x/y", "github.com/corp/kiroku;rm", "corp/kiroku", "github.com/corp/kiroku/extra", "github.com/corp/kiroku\n", "github.com/corp/ kiroku", "github.com/a/b\x1b[31m"} {
		_, calls := fakeGH(t, "v9.9.9", "")
		releaseRepo = repo
		if v, err := latestVersion(newUpdateClient()); err == nil || !strings.Contains(err.Error(), "not of the form HOST/OWNER/REPO") || strings.ContainsAny(err.Error(), "\x1b\n") {
			t.Errorf("%q: latestVersion = %q, %v", repo, v, err)
		}
		if c := calls(); len(c) != 0 {
			t.Errorf("%q: gh was run: %v", repo, c)
		}
	}
}

// gh が終わっても、gh から起動されたものが出力をつかんだままなら、待ち続けない（成功した gh の出力は使う）。
// gh 自身が終わらないときは、締め切りで止まる。
func TestGHDoesNotHang(t *testing.T) {
	fakeGH(t, "v9.9.9", "hangchild")
	start := time.Now()
	if v, err := ghLatest(20 * time.Second); err != nil || v != "v9.9.9" {
		t.Errorf("ghLatest = %q, %v", v, err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("took %v", d)
	}

	fakeGH(t, "v9.9.9", "hang")
	start = time.Now()
	if v, err := ghLatest(time.Second); err == nil || !strings.Contains(err.Error(), "did not finish in time") {
		t.Errorf("ghLatest = %q, %v", v, err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("took %v", d)
	}
}

// 成功した gh の標準エラーが長くても（デバッグの出力など）、失敗にしない。
func TestGHLongStderr(t *testing.T) {
	fakeGH(t, "v9.9.9", "bigstderr")
	if v, err := latestVersion(newUpdateClient()); err != nil || v != "v9.9.9" {
		t.Errorf("latestVersion = %q, %v", v, err)
	}
}

// releaseRepo が空（公開のリポジトリのビルド）なら、gh は使わない。
func TestNoGHByDefault(t *testing.T) {
	if releaseRepo != "" || viaGH() {
		t.Fatalf("releaseRepo = %q", releaseRepo)
	}
	if got := releasesPage(); got != releaseBase+"/releases" {
		t.Errorf("releasesPage = %q", got)
	}
}

// 画面のいちばん下のリンク先は、埋め込まれたリポジトリ（形が正しいときだけ）、なければ公開のリポジトリ。
func TestProjectURL(t *testing.T) {
	old := releaseRepo
	t.Cleanup(func() { releaseRepo = old })
	for repo, want := range map[string]string{
		"":                         "https://github.com/MichinaoShimizu/kiroku",
		"github.com/corp/kiroku":   "https://github.com/corp/kiroku",
		"github.com/corp/kiroku;x": "https://github.com/MichinaoShimizu/kiroku",
		"-x/corp/kiroku":           "https://github.com/MichinaoShimizu/kiroku",
	} {
		releaseRepo = repo
		if got := projectURL(); got != want {
			t.Errorf("projectURL(%q) = %q, want %q", repo, got, want)
		}
	}
}
