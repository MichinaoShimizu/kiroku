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

// 鍵を知っている画面にだけ見せる（ADR 0031）。鍵は X-Kiroku-Key ヘッダーでだけ受け付け、Cookie では通さない
// （Cookie はポートを区別しないので、同じコンピューターのほかのポートのサーバーにも送られ、鍵が盗まれる）。
// ?key= で開くと、鍵をページに入れない中継のページを返し、鍵の Cookie は渡さず、以前の版の Cookie は消させる。
// GET / だけは鍵なしで通す（履歴を入れない画面の骨組みを返すところ。TestServeShellHasNoHistory）。
func TestRequireKey(t *testing.T) {
	key := strings.Repeat("ab", 32)
	h := requireKey("127.0.0.1:8484", key, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("history")) }))
	do := func(method, target string, hdr string, c *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, nil)
		if hdr != "" {
			r.Header.Set("X-Kiroku-Key", hdr)
		}
		if c != nil {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	oldCookie := &http.Cookie{Name: "kiroku_key_8484", Value: key}
	for _, c := range []struct {
		name   string
		method string
		target string
		hdr    string
		cookie *http.Cookie
	}{
		{"鍵なし", "GET", "/data.json", "", nil},
		{"違う鍵", "GET", "/?key=" + strings.Repeat("cd", 32), "", nil},
		{"空の鍵", "GET", "/?key=", "", nil},
		{"違うヘッダー", "GET", "/data.json", strings.Repeat("cd", 32), nil},
		{"短いヘッダー", "GET", "/data.json", key[:63], nil},
		{"以前の版の Cookie だけ（もう通さない）", "GET", "/data.json", "", oldCookie},
		{"以前の版の Cookie だけで /stamp", "GET", "/stamp", "", oldCookie},
		{"以前の版の Cookie だけで POST", "POST", "/archive", "", oldCookie},
		{"POST の ?key=", "POST", "/archive?key=" + key, "", nil},
		{"POST /", "POST", "/", "", nil},
		{"/ 以外は鍵なしで通さない", "GET", "/history?id=s1", "", nil},
	} {
		w := do(c.method, c.target, c.hdr, c.cookie)
		if w.Code != 403 || w.Body.String() == "history" || !strings.Contains(w.Body.String(), "kiroku open") {
			t.Errorf("%s: %d %q", c.name, w.Code, w.Body.String())
		}
		for _, ck := range w.Result().Cookies() {
			if ck.Value != "" || ck.MaxAge >= 0 {
				t.Errorf("%s: 鍵の Cookie を渡した: %+v", c.name, ck)
			}
		}
	}
	// ヘッダーに鍵があれば通す（POST も。X-Kiroku と同じオリジンの確かめは、そのあとの /archive と /ai/run がする）
	for _, m := range []string{"GET", "POST"} {
		if w := do(m, "/data.json", key, nil); w.Code != 200 || w.Body.String() != "history" {
			t.Errorf("ヘッダーつきの %s = %d %q", m, w.Code, w.Body.String())
		}
	}
	// GET / は鍵なしで通す（返すのは骨組みだけ）
	if w := do("GET", "/", "", nil); w.Code != 200 {
		t.Errorf("鍵なしの GET / = %d", w.Code)
	}
	// 以前の版の Cookie が送られてきたら、どの答えでも消させる
	for _, target := range []string{"/", "/data.json"} {
		w := do("GET", target, "", oldCookie)
		cs := w.Result().Cookies()
		if len(cs) != 1 || cs[0].Name != "kiroku_key_8484" || cs[0].Value != "" || cs[0].MaxAge >= 0 || cs[0].Path != "/" {
			t.Errorf("%s: 以前の版の Cookie を消させていない: %+v", target, cs)
		}
	}
	// ?key= は中継のページ。鍵はページに入れず（アドレスから読む）、localStorage にしまうスクリプトだけを CSP で許す
	w := do("GET", "/history?id=s1&key="+key, "", oldCookie)
	body := w.Body.String()
	if w.Code != 200 || strings.Contains(body, key) || !strings.Contains(body, `<a id="to" href="/history?id=s1">`) ||
		!strings.Contains(body, `localStorage.setItem("kiroku:key", k)`) || !strings.Contains(body, "location.replace(") ||
		!strings.Contains(body, `<meta name="referrer" content="no-referrer">`) || strings.Contains(body, "http-equiv=\"refresh\"") {
		t.Fatalf("?key= = %d %q", w.Code, body)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("中継のページの Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
	if !strings.Contains(body, `content="`+hopKeyCSP+`"`) || !strings.Contains(hopKeyCSP, "default-src 'none'") || !strings.Contains(hopKeyCSP, cspHash(hopKeyScript)) ||
		strings.Contains(hopKeyCSP, "unsafe-inline") || strings.Contains(hopKeyCSP, "connect-src") {
		t.Errorf("中継のページの CSP = %q", hopKeyCSP)
	}
	if strings.Count(body, "<script>") != 1 || !strings.Contains(body, "<script>"+hopKeyScript+"</script>") {
		t.Errorf("中継のページのスクリプトが hopKeyScript と違う（CSP のハッシュが合わない）: %q", body)
	}
	cs := w.Result().Cookies()
	if len(cs) != 1 || cs[0].Value != "" || cs[0].MaxAge >= 0 {
		t.Fatalf("?key= で鍵の Cookie を渡した、または以前の版の Cookie を消させていない: %+v", cs)
	}
	if w := do("GET", "/?key="+key, "", nil); len(w.Result().Cookies()) != 0 || !strings.Contains(w.Body.String(), `<a id="to" href="/">`) {
		t.Errorf("Cookie のない ?key= = %+v %q", w.Result().Cookies(), w.Body.String())
	}
	// 移る先はこのサイトの中だけ（//evil.example のような URL にはしない）。エスケープもする
	for _, target := range []string{"//evil.example/x?key=" + key, `/\evil.example/x?key=` + key, `/%5Cevil.example/x?key=` + key} {
		if w := do("GET", target, "", nil); !strings.Contains(w.Body.String(), `href="/evil.example/x"`) {
			t.Errorf("%s への移動 = %q", target, w.Body.String())
		}
	}
	if w := do("GET", `/?a=%22%3E%3Cscript%3E&key=`+key, "", nil); strings.Count(w.Body.String(), "<script>") != 1 {
		t.Errorf("移る先をエスケープしていない: %q", w.Body.String())
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

// 中継のあとに移る先は、いつもこのサイトの中（ブラウザがタブ・改行を読み飛ばしたり \ を / と読んだりしても // にならない）
func TestHopTargetStaysOnSite(t *testing.T) {
	for _, p := range []string{"/", "//evil", "/\\evil", "/\t/evil", "/\r/evil", "/\\\t\\evil", "/\x7f/evil", "/a/\tb", "/x y"} {
		to := hopTarget(p)
		if !strings.HasPrefix(to, "/") || strings.HasPrefix(to, "//") || strings.HasPrefix(to, "/\\") || strings.ContainsFunc(to, func(c rune) bool { return c < 0x20 || c == 0x7f || c == '\\' }) {
			t.Errorf("hopTarget(%q) = %q", p, to)
		}
		// ブラウザと同じく、タブと改行を除いてもこのサイトの中
		if s := strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(to); strings.HasPrefix(s, "//") {
			t.Errorf("hopTarget(%q) = %q becomes %q in a browser", p, to, s)
		}
	}
	if got := hopTarget("/a/b"); got != "/a/b" {
		t.Errorf("plain path = %q", got)
	}
}
