package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// kiroku serve の鍵は、本人しか読めないファイル（0600、フォルダは 0700）に作り、次からは同じ鍵を使う。
// ほかのユーザーが読める鍵や壊れた鍵は使わずに作り直す。
func TestServeKey(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "kiroku")
	t.Setenv("KIROKU_CONFIG_DIR", dir)
	if _, err := serveKey(false); err != errNoKey {
		t.Fatalf("鍵がないのに err = %v", err)
	}
	key, err := serveKey(true)
	if err != nil || len(key) != 64 {
		t.Fatalf("key = %q, %v", key, err)
	}
	if again, _ := serveKey(true); again != key {
		t.Error("2 回目に別の鍵を作った")
	}
	if got, _ := serveKey(false); got != key {
		t.Error("作った鍵を読めない")
	}
	path := filepath.Join(dir, keyFile)
	if runtime.GOOS != "windows" {
		for p, want := range map[string]os.FileMode{path: 0o600, dir: 0o700} {
			if st, _ := os.Stat(p); st.Mode().Perm() != want {
				t.Errorf("%s のモード = %v, want %v", p, st.Mode().Perm(), want)
			}
		}
		os.Chmod(path, 0o644)
		if _, err := serveKey(false); err != errNoKey {
			t.Error("ほかのユーザーが読める鍵を使った")
		}
		fresh, err := serveKey(true)
		if err != nil || fresh == key {
			t.Errorf("読める鍵を作り直さない: %q %v", fresh, err)
		}
		if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
			t.Errorf("作り直した鍵のモード = %v", st.Mode().Perm())
		}
	}
	os.WriteFile(path, []byte("short\n"), 0o600)
	if k, err := serveKey(true); err != nil || k == "short" || len(k) != 64 {
		t.Errorf("壊れた鍵を作り直さない: %q %v", k, err)
	}
}

// 鍵を知っているブラウザにだけ見せる。?key= で開くと Cookie を渡し、鍵を消した URL へ移す。
func TestRequireKey(t *testing.T) {
	key := strings.Repeat("ab", 32)
	h := requireKey("127.0.0.1:8484", key, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("history")) }))
	do := func(method, target string, c *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, nil)
		if c != nil {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, c := range []struct {
		name   string
		target string
		cookie *http.Cookie
	}{
		{"鍵なし", "/data.json", nil},
		{"違う鍵", "/?key=" + strings.Repeat("cd", 32), nil},
		{"違う Cookie", "/", &http.Cookie{Name: "kiroku_key_8484", Value: strings.Repeat("cd", 32)}},
		{"ほかのポートの Cookie", "/", &http.Cookie{Name: "kiroku_key_9999", Value: key}},
		{"空の鍵", "/?key=", nil},
	} {
		w := do("GET", c.target, c.cookie)
		if w.Code != 403 || w.Body.String() == "history" || len(w.Result().Cookies()) != 0 {
			t.Errorf("%s: %d %q", c.name, w.Code, w.Body.String())
		}
	}
	if w := do("POST", "/archive?key="+key, nil); w.Code != 403 {
		t.Errorf("POST の ?key= で通した: %d", w.Code)
	}
	// Strict の Cookie はよそから来た移動には付かないので、このポートのページから鍵のない URL へ移る
	w := do("GET", "/history?id=s1&key="+key, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `content="0;url=/history?id=s1"`) || strings.Contains(w.Body.String(), key) {
		t.Fatalf("?key= = %d %q", w.Code, w.Body.String())
	}
	cs := w.Result().Cookies()
	if len(cs) != 1 || cs[0].Name != "kiroku_key_8484" || cs[0].Value != key || !cs[0].HttpOnly || cs[0].SameSite != http.SameSiteStrictMode || cs[0].Path != "/" || cs[0].MaxAge <= 0 {
		t.Fatalf("Cookie = %+v", cs)
	}
	// 移る先はこのサイトの中だけ（//evil.example のような URL にはしない）
	if w := do("GET", "//evil.example/x?key="+key, nil); !strings.Contains(w.Body.String(), `url=/evil.example/x"`) {
		t.Errorf("//host への移動 = %q", w.Body.String())
	}
	if w := do("GET", "/data.json", cs[0]); w.Code != 200 || w.Body.String() != "history" {
		t.Errorf("Cookie つき = %d %q", w.Code, w.Body.String())
	}
}

// ブラウザには鍵つきの URL をそのまま渡さず（プロセスの引数はほかのユーザーにも見える）、本人だけが読める HTML から移る。
func TestOpenWithKey(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "kiroku")
	t.Setenv("KIROKU_CONFIG_DIR", dir)
	var opened []string
	old := openBrowser
	openBrowser = func(target string) { opened = append(opened, target) }
	defer func() { openBrowser = old }()
	key := strings.Repeat("ab", 32)
	if err := openWithKey("http://localhost:8484/", key); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || strings.Contains(opened[0], key) {
		t.Fatalf("ブラウザに渡したもの = %q", opened)
	}
	b, err := os.ReadFile(opened[0])
	if err != nil || !strings.Contains(string(b), `content="0;url=http://localhost:8484/?key=`+key+`"`) {
		t.Fatalf("開く HTML = %q %v", b, err)
	}
	if st, _ := os.Stat(opened[0]); runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Errorf("開く HTML のモード = %v", st.Mode().Perm())
	}
}

// kiroku open: 鍵がなければ案内し、--print は鍵つきの URL を出す。開くのは答える kiroku serve があるときだけで、
// 開くときも鍵はブラウザのコマンドに渡さない。
func TestCmdOpen(t *testing.T) {
	t.Setenv("KIROKU_CONFIG_DIR", filepath.Join(t.TempDir(), "kiroku"))
	var opened []string
	oldOpen, oldProbe := openBrowser, probe
	defer func() { openBrowser, probe = oldOpen, oldProbe }()
	openBrowser = func(target string) { opened = append(opened, target) }
	if err := cmdOpen([]string{":18999"}); err != errNoKey {
		t.Fatalf("鍵がないのに err = %v", err)
	}
	key, _ := serveKey(true)
	if out := captureOutput(t, func() {
		if err := cmdOpen([]string{"--print", ":18999"}); err != nil {
			t.Fatal(err)
		}
	}); !strings.HasPrefix(out, "http://localhost:18999/?key="+key+"\n") {
		t.Errorf("--print = %q", out)
	}
	probe = func(addr string) int {
		if addr != "127.0.0.1:18999" {
			t.Errorf("probe(%q)", addr)
		}
		return noAnswer
	}
	if err := cmdOpen([]string{"0.0.0.0:18999"}); err == nil || !strings.Contains(err.Error(), "nothing answers") || len(opened) != 0 {
		t.Fatalf("動いていないのに開いた: %v %q", err, opened)
	}
	probe = func(string) int { return ready }
	captureOutput(t, func() {
		if err := cmdOpen([]string{":18999"}); err != nil {
			t.Fatal(err)
		}
	})
	if len(opened) != 1 || strings.Contains(opened[0], key) {
		t.Errorf("開いたもの = %q", opened)
	}
}

// autostart の確かめ（probe）は鍵を付けて問い合わせる（鍵がないと 403 しか返らない）。
func TestProbeSendsKey(t *testing.T) {
	t.Setenv("KIROKU_CONFIG_DIR", filepath.Join(t.TempDir(), "kiroku"))
	key, _ := serveKey(true)
	srv := httptest.NewServer(nil)
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")
	srv.Config.Handler = requireKey(addr, key, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if got := probe(addr); got != ready {
		t.Errorf("probe = %d, want ready", got)
	}
}

// 中継のページは、移るまでの一瞬に白い背景や青いリンクを見せない（背景と文字はローディング画面の背景の色）。
// 移る先はエスケープし、open.html は Referer に鍵つきの URL を出さない。
func TestHopPage(t *testing.T) {
	p := hopPage(`/x?a="><script>`, "")
	for _, want := range []string{`name="color-scheme" content="light dark"`, "background:var(--bg);color:var(--bg)", "a{color:inherit;animation:show .3s 2s forwards}", "prefers-color-scheme:dark"} {
		if !strings.Contains(p, want) {
			t.Errorf("中継のページに %q がない: %s", want, p)
		}
	}
	if strings.Contains(p, "<script>") || !strings.Contains(p, `url=/x?a=&#34;&gt;&lt;script&gt;"`) {
		t.Errorf("移る先をエスケープしていない: %s", p)
	}
	dir := filepath.Join(t.TempDir(), "kiroku")
	t.Setenv("KIROKU_CONFIG_DIR", dir)
	old := openBrowser
	openBrowser = func(string) {}
	defer func() { openBrowser = old }()
	if err := openWithKey("http://localhost:8484/", strings.Repeat("ab", 32)); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "open.html"))
	if !strings.Contains(string(b), `<meta name="referrer" content="no-referrer">`) || !strings.Contains(string(b), "color:var(--bg)") {
		t.Errorf("open.html = %s", b)
	}
}
