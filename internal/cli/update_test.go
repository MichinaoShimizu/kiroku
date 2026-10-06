package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"v0.1.1", "v0.1.0", 1}, {"v0.1.0", "v0.1.1", -1}, {"v0.10.0", "v0.9.9", 1}, {"0.1.1", "v0.1.1", 0},
		{"v1.0.0", "v1.0.0-rc.1", 1}, {"v1.0.0-rc.1", "v1.0.0", -1}, {"v1.0.0-rc.2", "v1.0.0-rc.1", 1},
	} {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// 偽の Releases を立てて、最新の版を読み、落として確かめ、自分自身を置きかえるところまで通す。
func TestSelfUpdate(t *testing.T) {
	newBin := []byte("#!/bin/sh\necho new kiroku\n")
	var arc bytes.Buffer
	gz := gzip.NewWriter(&arc)
	tw := tar.NewWriter(gz)
	for _, f := range []struct{ name, body string }{{"README.md", "readme"}, {"kiroku", string(newBin)}} {
		tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(f.body))
	}
	tw.Close()
	gz.Close()
	name := "kiroku_9.9.9_linux_amd64.tar.gz"
	sum := sha256.Sum256(arc.Bytes())
	sums := hex.EncodeToString(sum[:]) + "  " + name + "\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/v9.9.9", http.StatusFound)
	})
	mux.HandleFunc("/releases/download/v9.9.9/"+name, func(w http.ResponseWriter, r *http.Request) { w.Write(arc.Bytes()) })
	mux.HandleFunc("/releases/download/v9.9.9/checksums.txt", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, sums) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	defer func(b string) { releaseBase = b }(releaseBase)
	releaseBase = srv.URL

	if v, err := latestVersion(srv.Client()); err != nil || v != "v9.9.9" {
		t.Fatalf("latestVersion = %q, %v", v, err)
	}
	exe := filepath.Join(t.TempDir(), "kiroku")
	os.WriteFile(exe, []byte("old"), 0o755)
	if err := selfUpdate(srv.Client(), "v9.9.9", "linux", "amd64", exe); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); !bytes.Equal(got, newBin) {
		t.Fatalf("入れかわっていない: %q", got)
	}
	if st, _ := os.Stat(exe); runtime.GOOS != "windows" && st.Mode().Perm()&0o100 == 0 { // Windows には実行の権限ビットがない
		t.Error("実行できる権限が落ちた")
	}

	// チェックサムが合わなければ入れかえない
	sums = strings.Repeat("0", 64) + "  " + name + "\n"
	os.WriteFile(exe, []byte("old"), 0o755)
	if err := selfUpdate(srv.Client(), "v9.9.9", "linux", "amd64", exe); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("チェックサム違いを通した: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Fatal("失敗したのに元の kiroku が変わった")
	}
	// その版にファイルがなければエラー
	if err := selfUpdate(srv.Client(), "v9.9.9", "darwin", "arm64", exe); err == nil {
		t.Fatal("ないファイルでエラーにならない")
	}
}

// kiroku update のクライアントは、つながるまで・ヘッダーまでは短く待ち、遅い回線のダウンロードは途中で切らない。
func TestUpdateClientTimeouts(t *testing.T) {
	c := newUpdateClient()
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr.ResponseHeaderTimeout <= 0 || tr.ResponseHeaderTimeout > time.Minute || tr.TLSHandshakeTimeout <= 0 || tr.DialContext == nil {
		t.Fatalf("つながるまで・ヘッダーまでの上限がない: %+v", c.Transport)
	}
	if c.Timeout != 0 && c.Timeout < 10*time.Minute {
		t.Errorf("全体の上限 %v は遅い回線のダウンロードには短い", c.Timeout)
	}

	// 本体をゆっくり送っても読み切れる。最新の版の確認もこのクライアントで通る
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/v9.9.9", http.StatusFound)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 3; i++ {
			fmt.Fprint(w, "x")
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	defer func(b string) { releaseBase = b }(releaseBase)
	releaseBase = srv.URL
	if v, err := latestVersion(c); err != nil || v != "v9.9.9" {
		t.Fatalf("latestVersion = %q, %v", v, err)
	}
	if b, err := fetch(c, srv.URL+"/slow"); err != nil || string(b) != "xxx" {
		t.Fatalf("fetch = %q, %v", b, err)
	}
}

func TestUnknownSubcommand(t *testing.T) {
	if err := dispatch([]string{"updat"}); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("err = %v", err)
	}
}

// makeArchive は goos 向けのリリースのアーカイブ（Windows は zip、ほかは tar.gz）を作る。files は名前と中身（/ で終わる名前はフォルダ）。
func makeArchive(t *testing.T, goos string, files ...[2]string) []byte {
	t.Helper()
	var b bytes.Buffer
	if goos == "windows" {
		zw := zip.NewWriter(&b)
		for _, f := range files {
			w, err := zw.Create(f[0])
			if err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(f[1]))
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		h := &tar.Header{Name: f[0], Mode: 0o755, Size: int64(len(f[1])), Typeflag: tar.TypeReg}
		if strings.HasSuffix(f[0], "/") {
			h.Typeflag, h.Size = tar.TypeDir, 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(f[1]))
	}
	tw.Close()
	gz.Close()
	return b.Bytes()
}

func assetName(tag, goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("kiroku_%s_%s_%s%s", strings.TrimPrefix(tag, "v"), goos, goarch, ext)
}

// fakeReleases は偽の Releases を立てて releaseBase を向ける。latest が最新の版（空なら releases/latest は 404）で、
// tags のどれにも、この OS/arch のアーカイブ（中の kiroku は「kiroku <版>」と書いたファイル）と checksums.txt がある。
// 返す関数は、届いたリクエストのパスの一覧。
func fakeReleases(t *testing.T, latest string, tags ...string) func() []string {
	t.Helper()
	bin := "kiroku"
	if runtime.GOOS == "windows" {
		bin = "kiroku.exe"
	}
	files := map[string][]byte{}
	for _, tag := range tags {
		name := assetName(tag, runtime.GOOS, runtime.GOARCH)
		arc := makeArchive(t, runtime.GOOS, [2]string{"README.md", "readme"}, [2]string{bin, "kiroku " + tag})
		sum := sha256.Sum256(arc)
		files["/releases/download/"+tag+"/"+name] = arc
		files["/releases/download/"+tag+"/checksums.txt"] = []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	}
	var mu sync.Mutex
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.URL.Path)
		mu.Unlock()
		if r.URL.Path == "/releases/latest" && latest != "" {
			http.Redirect(w, r, "/releases/tag/"+latest, http.StatusFound)
			return
		}
		b, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	old := releaseBase
	t.Cleanup(func() { releaseBase = old })
	releaseBase = srv.URL
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), hits...)
	}
}

// captureOutput は fn のあいだの標準出力を返す（標準エラーは捨てる）。
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = f, null
	func() {
		defer func() { os.Stdout, os.Stderr = oldOut, oldErr; null.Close(); f.Close() }()
		fn()
	}()
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fakeExe は、kiroku update が置きかえる実行ファイルを一時フォルダの偽物にする（テストの実行ファイルは置きかえない）。
func fakeExe(t *testing.T) string {
	t.Helper()
	// macOS の一時フォルダ（/var/folders/…）は /private/var へのリンクで、kiroku update はリンクをたどった先を出すので、先にたどっておく
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "kiroku")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	oldExe, oldVersion := updateExe, version
	t.Cleanup(func() { updateExe, version = oldExe, oldVersion })
	updateExe = func() (string, error) { return exe, nil }
	return exe
}

// kiroku update をとおしで動かす: --check の出力、手元のビルド、最新のとき、--to、--force、ない版。
func TestRunUpdate(t *testing.T) {
	hits := fakeReleases(t, "v9.9.9", "v9.9.9", "v9.9.8")
	exe := fakeExe(t)
	for _, c := range []struct {
		name    string
		version string
		args    []string
		out     string // 標準出力に含まれるはずの文
		err     string // エラーに含まれるはずの文（空ならエラーなし）
		want    string // あとの kiroku の中身
	}{
		{"新しい版があるか確かめるだけ", "0.1.0", []string{"--check"}, `a new version is available: v0.1.0 → v9.9.9 (run "kiroku update")`, "", "old"},
		{"最新なら最新と言うだけ", "9.9.9", []string{"--check"}, "already up to date (v9.9.9)", "", "old"},
		{"手元のビルドは最新の版だけ知らせる", "dev", []string{"--check"}, "latest is v9.9.9 (this kiroku is a dev build from source)", "", "old"},
		{"--to の版と比べる", "9.9.8", []string{"--check", "--to", "v9.9.9"}, "a new version is available: v9.9.8 → v9.9.9", "", "old"},
		{"手元のビルドは置きかえない", "dev", nil, "", "dev build from source", "old"},
		{"最新なら落とさない", "9.9.9", nil, "already up to date (v9.9.9)", "", "old"},
		{"新しい版に置きかえる", "0.1.0", nil, "updated v0.1.0 → v9.9.9 (" + exe + ")", "", "kiroku v9.9.9"},
		{"--to で古い版も入れられる（v はなくてもよい）", "9.9.9", []string{"--to", "9.9.8"}, "updated v9.9.9 → v9.9.8", "", "kiroku v9.9.8"},
		{"--force なら手元のビルドも置きかえる", "dev", []string{"--force"}, "updated vdev → v9.9.9", "", "kiroku v9.9.9"},
		{"--force なら最新でも入れ直す", "9.9.9", []string{"--force"}, "updated v9.9.9 → v9.9.9", "", "kiroku v9.9.9"},
		{"ない版はエラーで、元のまま", "0.1.0", []string{"--to", "v1.2.3"}, "downloading kiroku v1.2.3", "could not download kiroku_1.2.3_", "old"},
		{"版を位置引数で書いたらエラー（黙って最新を入れない）", "0.1.0", []string{"v9.9.8"}, "", "too many arguments: v9.9.8", "old"},
		{"位置引数の後ろの --check も読む（確かめるだけのつもりで置きかえない）", "0.1.0", []string{"now", "--check"}, "", "too many arguments: now", "old"},
		{"知らないオプション", "0.1.0", []string{"--nope"}, "", "flag provided but not defined", "old"},
		{"--help はエラーにしない", "0.1.0", []string{"--help"}, "", "", "old"},
	} {
		if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
		version = c.version
		var err error
		out := captureOutput(t, func() { err = dispatch(append([]string{"update"}, c.args...)) })
		if c.err == "" && err != nil || c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.err)
		}
		if !strings.Contains(out, c.out) {
			t.Errorf("%s: 出力 %q に %q がない", c.name, out, c.out)
		}
		if got, _ := os.ReadFile(exe); string(got) != c.want {
			t.Errorf("%s: kiroku の中身 = %q, want %q", c.name, got, c.want)
		}
	}
	for _, h := range hits() {
		if strings.Contains(h, "/download/") && !strings.Contains(h, "v9.9.") && !strings.Contains(h, "v1.2.3") {
			t.Errorf("頼んでいない版を落とした: %s", h)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(exe), ".kiroku-new-*")); len(left) > 0 {
		t.Errorf("書きかけのファイルが残った: %v", left)
	}
}

// 最新の版を確かめられないときは、何も落とさずにわけを言う。
func TestRunUpdateLatestFails(t *testing.T) {
	hits := fakeReleases(t, "") // リリースがない（releases/latest が 404）
	exe := fakeExe(t)
	os.WriteFile(exe, []byte("old"), 0o755)
	version = "0.1.0"
	var err error
	captureOutput(t, func() { err = dispatch([]string{"update"}) })
	if err == nil || !strings.Contains(err.Error(), "could not check the latest version") || !strings.Contains(err.Error(), "release not found (404") {
		t.Fatalf("err = %v", err)
	}
	if h := hits(); len(h) != 1 {
		t.Errorf("最新の版を確かめるだけのはずが %v", h)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Errorf("kiroku が変わった: %q", got)
	}
}

// 最新の版が読めないとき（リダイレクトしない・つながらない）はエラー。渡したクライアントは書きかえない。
func TestLatestVersionErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html>no redirect</html>")
	}))
	defer func(b string) { releaseBase = b }(releaseBase)
	releaseBase = srv.URL
	c := &http.Client{}
	if v, err := latestVersion(c); err == nil || !strings.Contains(err.Error(), "release not found (200 OK)") {
		t.Errorf("リダイレクトなし = %q, %v", v, err)
	}
	if c.Timeout != 0 || c.CheckRedirect != nil {
		t.Errorf("渡したクライアントが書きかわった: %+v", c)
	}
	srv.Close()
	if v, err := latestVersion(c); err == nil {
		t.Errorf("止めたサーバーから %q が読めた", v)
	}
}

// 落とせない・checksums.txt にない・中身がちがう・つながらないときは、元の kiroku をそのまま残す。
func TestSelfUpdateFailures(t *testing.T) {
	good := makeArchive(t, "linux", [2]string{"kiroku", "new"})
	noBin := makeArchive(t, "linux", [2]string{"README.md", "readme"})
	broken := []byte("not a gzip")
	name := assetName("v9.9.9", "linux", "amd64")
	sumOf := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	var mu sync.Mutex
	arc, sums, sumsCode := good, "", 200
	set := func(a []byte, s string, code int) { mu.Lock(); arc, sums, sumsCode = a, s, code; mu.Unlock() }
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/download/v9.9.9/"+name, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Write(arc)
	})
	mux.HandleFunc("/releases/download/v9.9.9/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.WriteHeader(sumsCode)
		fmt.Fprint(w, sums)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	defer func(b string) { releaseBase = b }(releaseBase)
	releaseBase = srv.URL
	dir := t.TempDir()
	exe := filepath.Join(dir, "kiroku")

	for _, c := range []struct {
		name string
		arc  []byte
		sums string
		code int
		want string
	}{
		{"checksums.txt がない", good, "", 404, "could not download checksums.txt: 404 Not Found"},
		{"checksums.txt に載っていない", good, sumOf(good) + "  kiroku_9.9.9_darwin_arm64.tar.gz\n", 200, name + " is not listed in checksums.txt"},
		{"形のちがう行は読まない", good, sumOf(good) + " " + name + " extra\n", 200, "is not listed in checksums.txt"},
		{"中身がちがう", good, sumOf(noBin) + "  " + name + "\n", 200, "checksum mismatch, not updating (" + name + ")"},
		{"アーカイブに kiroku がない", noBin, sumOf(noBin) + "  " + name + "\n", 200, "kiroku not found in the archive"},
		{"アーカイブが壊れている", broken, sumOf(broken) + "  " + name + "\n", 200, "gzip"},
	} {
		os.WriteFile(exe, []byte("old"), 0o755)
		set(c.arc, c.sums, c.code)
		if err := selfUpdate(srv.Client(), "v9.9.9", "linux", "amd64", exe); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
		if got, _ := os.ReadFile(exe); string(got) != "old" {
			t.Errorf("%s: 失敗したのに kiroku が変わった: %q", c.name, got)
		}
	}
	// Windows の改行（CRLF）の checksums.txt でも読める
	set(good, strings.Repeat("0", 64)+"  other.zip\r\n"+sumOf(good)+"  "+name+"\r\n", 200)
	if err := selfUpdate(srv.Client(), "v9.9.9", "linux", "amd64", exe); err != nil {
		t.Fatalf("CRLF の checksums.txt: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "new" {
		t.Errorf("入れかわっていない: %q", got)
	}

	// その版がない・つながらない
	if err := selfUpdate(srv.Client(), "v1.0.0", "linux", "amd64", exe); err == nil || !strings.Contains(err.Error(), "could not download kiroku_1.0.0_linux_amd64.tar.gz: 404 Not Found") {
		t.Errorf("ない版: %v", err)
	}
	srv.Close()
	os.WriteFile(exe, []byte("old"), 0o755)
	if err := selfUpdate(srv.Client(), "v9.9.9", "linux", "amd64", exe); err == nil || !strings.Contains(err.Error(), "could not download "+name) {
		t.Errorf("つながらない: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Errorf("つながらないのに kiroku が変わった: %q", got)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".kiroku-new-*")); len(left) > 0 {
		t.Errorf("書きかけのファイルが残った: %v", left)
	}
}

// Windows 向けは zip から kiroku.exe を取り出す（同じ zip の kiroku は使わない）。
func TestSelfUpdateZip(t *testing.T) {
	arc := makeArchive(t, "windows", [2]string{"README.md", "readme"}, [2]string{"kiroku", "not this"}, [2]string{"kiroku.exe", "new exe"})
	name := assetName("v9.9.9", "windows", "arm64")
	sum := sha256.Sum256(arc)
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/download/v9.9.9/"+name, func(w http.ResponseWriter, r *http.Request) { w.Write(arc) })
	mux.HandleFunc("/releases/download/v9.9.9/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	defer func(b string) { releaseBase = b }(releaseBase)
	releaseBase = srv.URL
	exe := filepath.Join(t.TempDir(), "kiroku.exe")
	os.WriteFile(exe, []byte("old"), 0o755)
	if err := selfUpdate(srv.Client(), "v9.9.9", "windows", "arm64", exe); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "new exe" {
		t.Errorf("入れかわっていない: %q", got)
	}
}

// アーカイブから kiroku を取り出す: フォルダの中にあってもよく、同じ名前のフォルダは飛ばす。なければ・壊れていればエラー。
func TestExtract(t *testing.T) {
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	w.Write([]byte(strings.Repeat("x", 100))) // gzip としては読めるが、tar としては途中で切れている
	w.Close()
	for _, c := range []struct {
		name, ext, bin string
		arc            []byte
		want, err      string
	}{
		{"tar.gz", ".tar.gz", "kiroku", makeArchive(t, "linux", [2]string{"kiroku/", ""}, [2]string{"kiroku_9.9.9/kiroku", "bin"}), "bin", ""},
		{"zip", ".zip", "kiroku.exe", makeArchive(t, "windows", [2]string{"kiroku_9.9.9/kiroku.exe", "exe"}), "exe", ""},
		{"tar.gz にない", ".tar.gz", "kiroku", makeArchive(t, "linux", [2]string{"kiroku.exe", "x"}), "", "kiroku not found in the archive"},
		{"zip にない", ".zip", "kiroku.exe", makeArchive(t, "windows", [2]string{"kiroku", "x"}), "", "kiroku.exe not found in the archive"},
		{"gzip でない", ".tar.gz", "kiroku", []byte("this is not a gzip file"), "", "gzip"},
		{"zip でない", ".zip", "kiroku.exe", []byte("plain"), "", "zip"},
		{"tar が途中で切れている", ".tar.gz", "kiroku", gz.Bytes(), "", "EOF"},
	} {
		got, err := extract(c.arc, c.ext, c.bin)
		if c.err == "" && (err != nil || string(got) != c.want) || c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s: %q, %v", c.name, got, err)
		}
	}
}

// 置きかえ: 書けないフォルダならわけを言って何もしない。元の権限に実行の権限を足す。Windows は元を .old に逃がす。
func TestReplaceExe(t *testing.T) {
	dir := t.TempDir()
	if err := replaceExe(filepath.Join(dir, "no-such-dir", "kiroku"), []byte("new")); err == nil || !strings.Contains(err.Error(), "cannot write to") {
		t.Errorf("書けないフォルダ: %v", err)
	}
	exe := filepath.Join(dir, "kiroku")
	if err := os.WriteFile(exe, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := replaceExe(exe, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "new" {
		t.Errorf("入れかわっていない: %q", got)
	}
	if runtime.GOOS == "windows" { // Windows には実行の権限ビットがない
		if got, _ := os.ReadFile(exe + ".old"); string(got) != "old" {
			t.Errorf(".old に元の kiroku がない: %q", got)
		}
	} else if st, err := os.Stat(exe); err != nil || st.Mode().Perm() != 0o711 {
		t.Errorf("権限 = %v, %v", st.Mode().Perm(), err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".kiroku-new-*")); len(left) > 0 {
		t.Errorf("書きかけのファイルが残った: %v", left)
	}
}

// kiroku version は版と OS/arch を出す。
func TestRunVersion(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "1.2.3"
	out := captureOutput(t, func() {
		if err := dispatch([]string{"--version"}); err != nil {
			t.Error(err)
		}
	})
	if want := "kiroku 1.2.3 (" + runtime.GOOS + "/" + runtime.GOARCH + ")\n"; out != want {
		t.Errorf("= %q, want %q", out, want)
	}
}
