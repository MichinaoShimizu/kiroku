package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// 新しい版のお知らせ：1 日に 1 回まで releases/latest を見て、端末でだけ聞き、y でだけ更新する。
func TestUpdateNotice(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/releases/latest" || r.URL.RawQuery != "" || r.ContentLength > 0 {
			t.Errorf("unexpected request %s %s (only the latest version may be asked, with nothing sent)", r.Method, r.URL)
		}
		w.Header().Set("Location", "/MichinaoShimizu/kiroku/releases/tag/v1.9.0")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	oldBase, oldVer, oldOn, oldNow := releaseBase, version, updateCheckOn, updateNow
	t.Cleanup(func() { releaseBase, version, updateCheckOn, updateNow = oldBase, oldVer, oldOn, oldNow })
	releaseBase, version = srv.URL, "1.0.5"
	updateCheckOn = func() bool { return true }
	dir := t.TempDir()
	t.Setenv("KIROKU_CONFIG_DIR", dir)
	var updated []string
	updateNow = func(v string) error { updated = append(updated, v); return nil }

	run := func(cmd, answer string) string {
		var out bytes.Buffer
		startUpdateCheck(cmd).offer(strings.NewReader(answer), &out)
		return out.String()
	}
	// 何も答えない（Enter だけ）なら更新しない。既定は No
	if out := run("stats", "\n"); !strings.Contains(out, "v1.9.0 is available (this is v1.0.5)") || !strings.Contains(out, "Skipped") || len(updated) != 0 {
		t.Fatalf("out = %q, updated = %v", out, updated)
	}
	// 結果は残し、24 時間のうちはもう聞きに行かない。ファイルはほかのユーザーが読めない
	if fi, err := os.Stat(filepath.Join(dir, updateCheckFile)); err != nil {
		t.Fatal(err)
	} else if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("cache mode = %v, want 0600", fi.Mode().Perm())
	}
	if out := run("html", "y\n"); !strings.Contains(out, "Update now?") || len(updated) != 1 || updated[0] != "v1.9.0" {
		t.Fatalf("out = %q, updated = %v", out, updated)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("asked GitHub %d times, want once a day", n)
	}
	// 更新と版とヘルプのコマンドでは確かめない。端末でない・止められているときも
	if startUpdateCheck("update") != nil || startUpdateCheck("version") != nil || startUpdateCheck("help") != nil {
		t.Error("update, version and help must not check")
	}
	updateCheckOn = oldOn
	t.Setenv("KIROKU_NO_UPDATE_CHECK", "1")
	if startUpdateCheck("stats") != nil {
		t.Error("KIROKU_NO_UPDATE_CHECK must turn it off")
	}
	// 同じか古い版なら、何も言わない
	updateCheckOn = func() bool { return true }
	t.Setenv("KIROKU_NO_UPDATE_CHECK", "")
	version = "1.9.0"
	if out := run("stats", "y\n"); out != "" || len(updated) != 1 {
		t.Errorf("up to date: out = %q", out)
	}
}

// 残した結果が壊れている・未来の時刻・版の形でないなら、使わずに聞き直す
func TestUpdateCacheIgnoresBadEntries(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Location", "/x/releases/tag/v2.0.0")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	oldBase := releaseBase
	t.Cleanup(func() { releaseBase = oldBase })
	releaseBase = srv.URL
	dir := t.TempDir()
	t.Setenv("KIROKU_CONFIG_DIR", dir)
	for i, bad := range []string{`not json`, `{"checked":99999999999,"latest":"v9.9.9"}`, `{"checked":1,"latest":"v1; rm -rf /"}`} {
		if err := os.WriteFile(filepath.Join(dir, updateCheckFile), []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if v := cachedOrFetchLatest(); v != "v2.0.0" {
			t.Errorf("%q: latest = %q, want a fresh v2.0.0", bad, v)
		}
		if int(hits.Load()) != i+1 {
			t.Errorf("%q: did not ask again", bad)
		}
	}
}

// 残した結果がリンク（/dev/zero を指す）・FIFO・大きなファイルなら、読まずに聞き直す（際限なく読んだり止まったりしない）。
// 書くときは、決まった名前の一時ファイルに置かれたリンクをたどらず、0600 のふつうのファイルで置きかえる。
func TestUpdateCacheFileSafety(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks and FIFOs need a Unix system")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/x/releases/tag/v2.0.0")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	oldBase := releaseBase
	t.Cleanup(func() { releaseBase = oldBase })
	releaseBase = srv.URL
	dir := t.TempDir()
	t.Setenv("KIROKU_CONFIG_DIR", dir)
	cache := filepath.Join(dir, updateCheckFile)
	for name, mk := range map[string]func() error{
		"symlink to /dev/zero": func() error { return os.Symlink("/dev/zero", cache) },
		"fifo":                 func() error { return syscallMkfifo(cache) },
		"big file":             func() error { return os.WriteFile(cache, bytes.Repeat([]byte(" "), 1<<20), 0o600) },
	} {
		os.Remove(cache)
		if err := mk(); err != nil {
			t.Fatal(name, err)
		}
		done := make(chan string, 1)
		go func() { done <- cachedOrFetchLatest() }()
		select {
		case v := <-done:
			if v != "v2.0.0" {
				t.Errorf("%s: latest = %q", name, v)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: reading the cache hangs", name)
		}
		if fi, err := os.Lstat(cache); err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm() != 0o600 {
			t.Errorf("%s: cache is not replaced by a 0600 regular file (%v, %v)", name, fi, err)
		}
	}
	// 以前の版が使った決まった名前の一時ファイルにリンクを置かれても、その先は変えない
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(cache)
	if err := os.Symlink(victim, cache+".tmp"); err != nil {
		t.Fatal(err)
	}
	cachedOrFetchLatest()
	if b, _ := os.ReadFile(victim); string(b) != "keep" {
		t.Errorf("the symlink target was overwritten: %q", b)
	}
	// /dev/null は端末ではない（cron の </dev/null >/dev/null で、見に行ったり聞いたりしない）
	if f, err := os.Open(os.DevNull); err == nil {
		if isTerminal(f) {
			t.Error("/dev/null must not count as a terminal")
		}
		f.Close()
	}
}
