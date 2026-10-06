package cli

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// --serve 中に履歴が増えたら、読み直して /stamp と /data.json が新しくなる。
func TestLiveRefresh(t *testing.T) {
	home := t.TempDir()
	cli := filepath.Join(home, "sessions", "cli")
	if err := os.MkdirAll(cli, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join("testdata", "home", ".kiro", "sessions", "cli")
	add := func(id string) {
		for _, ext := range []string{".json", ".jsonl"} {
			copyFile(t, filepath.Join(src, id+ext), filepath.Join(cli, id+ext))
		}
	}
	add("0d612aac-99c4-4a54-bda2-76ad60b6ddf3")

	s := &source.KiroCLI{Home: home, CrewHome: filepath.Join(home, "crew")}
	load := func() snapshot {
		data, rep := collect([]source.Source{s}, map[string]bool{"kiro": true}, 15)
		return snapshot{data: data, weeks: report.AllWeeks(data), meta: map[string]any{}, rep: rep, gen: float64(time.Now().UnixNano()) / 1e9}
	}
	logw = io.Discard
	defer func() { logw = os.Stderr }()
	l := &live{load: load, paths: source.WatchPaths(s), print: io.Discard}
	if err := l.refresh(); err != nil {
		t.Fatal(err)
	}
	l.markReady()
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	get := func(path string) string {
		r, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return string(b)
	}
	if html := get("/"); !strings.Contains(html, "const LIVE = true") {
		t.Fatal("画面が live になっていない")
	}
	stamp := get("/stamp")
	if l.count() != 1 {
		t.Fatalf("最初のセッション数 = %d, want 1", l.count())
	}

	stop, done := make(chan struct{}), make(chan struct{})
	defer func() { close(stop); <-done }() // 読み直しの途中で次のテストへ進まない（time.Local を書きかえるテストと競合する）
	go func() { l.watch(20*time.Millisecond, stop); close(done) }()
	time.Sleep(60 * time.Millisecond)
	add("1e13c3c1-d7ae-41c2-a324-e6a440665d9a")
	deadline := time.Now().Add(5 * time.Second)
	for l.count() != 2 {
		if time.Now().After(deadline) {
			t.Fatalf("増えた履歴が反映されない（%d セッション）", l.count())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if get("/stamp") == stamp {
		t.Fatal("/stamp が変わっていない")
	}
	if js := get("/data.json"); !strings.Contains(js, "1e13c3c1") {
		t.Fatal("/data.json に新しいセッションがない")
	}
}

// よそのホスト名で来たリクエストは断る（DNS リバインディング対策）。0.0.0.0 で外に開いても、
// このコンピューターの IP・ホスト名と --allow-host で足した名前だけを通す。
func TestSameOrigin(t *testing.T) {
	old := localNames
	t.Cleanup(func() { localNames = old })
	localNames = func() []string { return []string{"192.168.0.5", "fe80::1", "mymac", "mymac.local"} }
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, c := range []struct {
		addr  string
		allow []string
		codes map[string]int
	}{
		{"127.0.0.1:8484", nil, map[string]int{"localhost:8484": 200, "127.0.0.1:8484": 200, "[::1]:8484": 200, "LocalHost:8484": 200,
			"evil.example:8484": 403, "localhost:9999": 403, "192.168.0.5:8484": 403}},
		{"0.0.0.0:8484", nil, map[string]int{"localhost:8484": 200, "192.168.0.5:8484": 200, "[fe80::1]:8484": 200, "mymac.local:8484": 200,
			"evil.example:8484": 403, "192.168.0.6:8484": 403, "192.168.0.5:9999": 403}},
		{"192.168.0.5:8484", nil, map[string]int{"192.168.0.5:8484": 200, "localhost:8484": 200, "mymac:8484": 403, "evil.example:8484": 403}},
		{"0.0.0.0:8484", []string{"kiroku.home"}, map[string]int{"kiroku.home:8484": 200, "evil.example:8484": 403}},
		{"0.0.0.0:80", nil, map[string]int{"mymac": 200, "mymac:80": 200, "evil.example": 403}},
	} {
		h := sameOrigin(c.addr, c.allow, ok)
		for host, want := range c.codes {
			r := httptest.NewRequest("GET", "/", nil)
			r.Host = host
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != want {
				t.Errorf("%s %v: Host %s → %d, want %d", c.addr, c.allow, host, w.Code, want)
			}
		}
	}
}

// 書き込みが続いている間は読み直さず、落ち着いたら 1 回だけ読み直す。続きすぎたら max で読み直す。
func TestSettler(t *testing.T) {
	t0 := time.Unix(0, 0)
	at := func(sec int) time.Time { return t0.Add(time.Duration(sec) * time.Second) }
	s := settler{settle: 10 * time.Second, max: 60 * time.Second}
	if s.step(false, at(0)) {
		t.Fatal("変化がないのに読み直した")
	}
	// 0〜20 秒は 5 秒ごとに変化、そのあと止まる → 30 秒で読み直し
	for _, sec := range []int{5, 10, 15, 20} {
		if s.step(true, at(sec)) {
			t.Fatalf("%d 秒: 書き込み中に読み直した", sec)
		}
	}
	if s.step(false, at(25)) {
		t.Fatal("25 秒: まだ落ち着いていないのに読み直した")
	}
	if !s.step(false, at(30)) {
		t.Fatal("30 秒: 落ち着いたのに読み直さない")
	}
	if s.step(false, at(35)) {
		t.Fatal("35 秒: 2 回読み直した")
	}
	// 100 秒から書き込みが続く → 最初の変化から 60 秒（160 秒）で読み直す
	for sec := 100; sec < 160; sec += 5 {
		if s.step(true, at(sec)) {
			t.Fatalf("%d 秒: max より前に読み直した", sec)
		}
	}
	if !s.step(true, at(160)) {
		t.Fatal("160 秒: 書き込みが続いても max で読み直すはず")
	}
}

// /history は、読み込んだセッションの履歴ファイルだけを返す。
func TestServeHistory(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s1.jsonl")
	os.WriteFile(p, []byte(`{"type":"user"}`+"\n"), 0o644)
	l := &live{snap: snapshot{data: []*core.Session{{ID: "s1", File: p}, {ID: "db", File: filepath.Join(dir, "data.sqlite3")}}}, ready: true, roots: []string{dir}}
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	get := func(q string) (int, string) {
		r, err := http.Get(srv.URL + "/history?" + q)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(b)
	}
	if code, body := get("id=s1"); code != 200 || !strings.Contains(body, `"type":"user"`) {
		t.Errorf("s1 = %d %q", code, body)
	}
	for _, q := range []string{"id=nope", "id=db", "id=../../etc/passwd", "path=" + p} {
		if code, _ := get(q); code != 404 {
			t.Errorf("%s = %d, want 404", q, code)
		}
	}
}

// /prompt は、HTML では切った依頼文の全文を返す。
func TestServePrompt(t *testing.T) {
	ts := 1000.0
	b := core.NewBuilder("Claude Code", "s1")
	b.Tick(&ts)
	b.Prompt(&ts, "短い依頼")
	b.Prompt(&ts, strings.Repeat("長", core.PromptRunes)+"最後")
	l := &live{snap: snapshot{data: []*core.Session{b.Finish(15)}}, ready: true}
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	get := func(q string) (int, string) {
		r, err := http.Get(srv.URL + "/prompt?" + q)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(body)
	}
	if code, body := get("id=s1&i=1"); code != 200 || !strings.HasSuffix(body, "最後") {
		t.Errorf("i=1 = %d %q", code, body)
	}
	if code, body := get("id=s1&i=0"); code != 200 || body != "短い依頼" {
		t.Errorf("i=0 = %d %q", code, body)
	}
	for _, q := range []string{"id=s1&i=2", "id=s1&i=-1", "id=s1", "id=nope&i=0"} {
		if code, _ := get(q); code != 404 {
			t.Errorf("%s = %d, want 404", q, code)
		}
	}
}

// /reply は、HTML では切った応答の全文を返す。応答のない依頼は 404。
func TestServeReply(t *testing.T) {
	ts, rt, next := 1000.0, 1100.0, 1200.0
	b := core.NewBuilder("Claude Code", "s1")
	b.Tick(&ts)
	b.Prompt(&ts, "長い応答がつく依頼")
	b.Reply(&rt, "m1", strings.Repeat("応", core.ReplyRunes)+"おわり")
	b.Prompt(&next, "応答のない依頼")
	l := &live{snap: snapshot{data: []*core.Session{b.Finish(15)}}, ready: true}
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	get := func(q string) (int, string) {
		r, err := http.Get(srv.URL + "/reply?" + q)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(body)
	}
	if code, body := get("id=s1&i=0"); code != 200 || !strings.HasSuffix(body, "おわり") {
		t.Errorf("i=0 = %d %q", code, body)
	}
	for _, q := range []string{"id=s1&i=1", "id=s1&i=2", "id=s1", "id=nope&i=0"} {
		if code, _ := get(q); code != 404 {
			t.Errorf("%s = %d, want 404", q, code)
		}
	}
}

// ポートだけを指定しても、手元だけで待ち受ける（同じネットワークのほかの人に履歴を見せない）。
func TestListenAddr(t *testing.T) {
	for in, want := range map[string]string{":8485": "127.0.0.1:8485", "127.0.0.1:9000": "127.0.0.1:9000", "0.0.0.0:8484": "0.0.0.0:8484", "[::1]:8484": "[::1]:8484"} {
		if got := listenAddr(in); got != want {
			t.Errorf("listenAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

// serve は先に待ち受け、最初の読み込みが終わるまでは読み込み中の画面と 503 を返す（落ちない）。
// 読み終わったら本物の画面と JSON になる。Host の確認は読み込み中もかかる。
func TestServeBeforeLoad(t *testing.T) {
	logw = io.Discard
	defer func() { logw = os.Stderr }()
	gate := make(chan struct{})
	load := func() snapshot {
		<-gate // 読み込みが長引いている間を作る
		ts := 1000.0
		b := core.NewBuilder("Claude Code", "s1")
		b.Tick(&ts)
		b.Prompt(&ts, "はじめの依頼")
		return snapshot{data: []*core.Session{b.Finish(15)}, weeks: map[string]*report.Week{}, meta: map[string]any{}, gen: 1234.5}
	}
	l := &live{load: load, print: io.Discard, keep: func() error { return nil }}
	srv := httptest.NewServer(sameOrigin("127.0.0.1:8484", nil, l.handler()))
	defer srv.Close()
	do := func(method, path, host string) (int, string) {
		r, _ := http.NewRequest(method, srv.URL+path, nil)
		r.Host = host
		r.Header.Set("X-Kiroku", "1")
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	const host = "localhost:8484"

	stop, done := make(chan struct{}), make(chan struct{})
	go func() { l.start(time.Hour, stop); close(done) }()
	defer func() { close(stop); <-done }()

	if code, body := do("GET", "/", host); code != 503 || !strings.Contains(body, "Reading your history") || strings.Contains(body, "const LIVE") {
		t.Errorf("読み込み中の / = %d %.80q", code, body)
	}
	for _, c := range []struct{ method, path string }{{"GET", "/stamp"}, {"GET", "/data.json"}, {"GET", "/history?id=s1"}, {"GET", "/prompt?id=s1&i=0"}, {"GET", "/reply?id=s1&i=0"}, {"POST", "/archive"}} {
		if code, _ := do(c.method, c.path, host); code != 503 {
			t.Errorf("読み込み中の %s %s = %d, want 503", c.method, c.path, code)
		}
	}
	if code, _ := do("GET", "/nope", host); code != 404 {
		t.Errorf("読み込み中の /nope = %d, want 404", code)
	}
	for _, p := range []string{"/", "/stamp", "/data.json"} {
		if code, body := do("GET", p, "evil.example:8484"); code != 403 || strings.Contains(body, "Reading") {
			t.Errorf("読み込み中によそのホストから %s = %d", p, code)
		}
	}

	close(gate)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if code, body := do("GET", "/stamp", host); code == 200 {
			if body != "1234.5" {
				t.Fatalf("/stamp = %q", body)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("読み終わらない")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if code, body := do("GET", "/", host); code != 200 || !strings.Contains(body, "const LIVE = true") {
		t.Errorf("読み終わったあとの / = %d", code)
	}
	if code, body := do("GET", "/data.json", host); code != 200 || !strings.Contains(body, `"s1"`) {
		t.Errorf("読み終わったあとの /data.json = %d %.80q", code, body)
	}
	if code, body := do("GET", "/prompt?id=s1&i=0", host); code != 200 || body != "はじめの依頼" {
		t.Errorf("読み終わったあとの /prompt = %d %q", code, body)
	}
	if code, _ := do("GET", "/", "evil.example:8484"); code != 403 {
		t.Errorf("よそのホストから / = %d, want 403", code)
	}
}

// 最初の読み込みに失敗しても止まらず、/stamp の 503 の本文でエラーを知らせる（読み込み中の画面がそれを出す）。
func TestServeLoadFails(t *testing.T) {
	logw = io.Discard
	defer func() { logw = os.Stderr }()
	// 画面に入れられない値（NaN）で JSON にできず、refresh が失敗する
	l := &live{load: func() snapshot { return snapshot{meta: map[string]any{"x": math.NaN()}} }, print: io.Discard}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() { l.start(time.Hour, stop); close(done) }()
	defer func() { close(stop); <-done }()
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		r, err := http.Get(srv.URL + "/stamp")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != 503 {
			t.Fatalf("/stamp = %d, want 503", r.StatusCode)
		}
		if len(b) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("失敗が /stamp に出ない")
		}
		time.Sleep(10 * time.Millisecond)
	}
	r, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 503 {
		t.Errorf("/ = %d, want 503（読み込み中の画面）", r.StatusCode)
	}
}

// ポートがふさがっていたら、履歴を読む前にすぐ止まる。
func TestServeBusyPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var loaded atomic.Bool
	load := func() snapshot { loaded.Store(true); return snapshot{} }
	errc := make(chan error, 1)
	go func() { errc <- serveLive(ln.Addr().String(), nil, time.Second, nil, load, nil, false) }()
	select {
	case err := <-errc:
		if err == nil || !strings.Contains(err.Error(), "could not listen on") || !strings.Contains(err.Error(), "kiroku serve :8485") {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ふさがったポートで止まらない")
	}
	if loaded.Load() {
		t.Fatal("待ち受けられないのに履歴を読んだ")
	}
}

// syncBuf は、ほかの goroutine から書かれても読める出力先（serve の端末への表示を見る）。
type syncBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// waitFor は cond が true になるまで待つ（5 秒たったら失敗）。
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s にならない", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// testCookie は fetchHost が付ける Cookie（kiroku serve の鍵）。
var testCookie *http.Cookie

// fetchHost は Host を指定して srv に送り、状態・本文・ヘッダーを返す。
func fetchHost(t *testing.T, method, url, host string) (int, string, http.Header) {
	t.Helper()
	r, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		r.Host = host
	}
	r.Header.Set("X-Kiroku", "1")
	if testCookie != nil {
		r.AddCookie(testCookie)
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b), res.Header
}

// kiroku serve をとおしで動かす: ポートだけなら手元で待ち受けて localhost の URL を出し、読み込み中の画面から本物の画面になる。
// Host の確認と --allow-host もかかり、止めたらエラーなしで終わる。
func TestServeLive(t *testing.T) {
	out := &syncBuf{}
	logw = out
	defer func() { logw = os.Stderr }()
	oldServe, oldStart := serveHTTP, startLive
	defer func() { serveHTTP, startLive = oldServe, oldStart }()
	srvc := make(chan *http.Server, 1)
	serveHTTP = func(s *http.Server, ln net.Listener) error {
		srvc <- s
		return s.Serve(ln)
	}
	gate := make(chan struct{})
	var openGate sync.Once
	stop, done := make(chan struct{}), make(chan struct{})
	var started atomic.Bool
	startLive = func(l *live, every time.Duration) {
		started.Store(true)
		go func() { l.start(every, stop); close(done) }()
	}
	// 見張りを止めてから次のテストへ進む（time.Local を書きかえるテストと競合する）
	defer func() {
		openGate.Do(func() { close(gate) })
		close(stop)
		if started.Load() {
			<-done
		}
	}()
	load := func() snapshot {
		<-gate
		ts := 1000.0
		b := core.NewBuilder("Claude Code", "s1")
		b.Tick(&ts)
		return snapshot{data: []*core.Session{b.Finish(15)}, weeks: map[string]*report.Week{}, meta: map[string]any{}, gen: 42}
	}
	errc := make(chan error, 1)
	go func() { errc <- serveLive(":0", []string{"kiroku.home"}, 0, nil, load, nil, false) }()
	var srv *http.Server
	select {
	case srv = <-srvc:
	case err := <-errc:
		t.Fatalf("serve が止まった: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("待ち受けない")
	}
	// ヘッダーを送らずにつなぎっぱなしにする相手で詰まらないよう、時間を区切る（大きな /history を返せるよう、書き出しは区切らない）
	if srv.ReadHeaderTimeout != 10*time.Second || srv.IdleTimeout != 120*time.Second || srv.MaxHeaderBytes != 64<<10 || srv.WriteTimeout != 0 || srv.ReadTimeout != 0 {
		t.Errorf("http.Server の時間の区切り = %+v", srv)
	}
	m := regexp.MustCompile(`^serving http://localhost:(\d+)/ \(reading history`).FindStringSubmatch(out.String())
	if m == nil {
		t.Fatalf("URL が出ていない: %q", out.String())
	}
	port := m[1]
	base := "http://127.0.0.1:" + port
	if !strings.Contains(out.String(), `"kiroku open" opens it with the key`) {
		t.Errorf("kiroku open を案内するはず: %q", out.String())
	}
	// 鍵のないブラウザ（同じコンピューターのほかのユーザーなど）には、履歴を見せない
	for _, path := range []string{"/", "/stamp", "/data.json"} {
		if code, body, _ := fetchHost(t, "GET", base+path, "localhost:"+port); code != 403 || !strings.Contains(body, "kiroku open") {
			t.Errorf("鍵なしの %s = %d %.80q, want 403", path, code, body)
		}
	}
	key, err := serveKey(false)
	if err != nil {
		t.Fatal(err)
	}
	testCookie = &http.Cookie{Name: "kiroku_key_" + port, Value: key}
	defer func() { testCookie = nil }()

	if code, body, _ := fetchHost(t, "GET", base+"/", "localhost:"+port); code != 503 || !strings.Contains(body, "Reading your history") {
		t.Errorf("読み込み中の / = %d %.80q", code, body)
	}
	if code, _, _ := fetchHost(t, "GET", base+"/stamp", "kiroku.home:"+port); code != 503 {
		t.Errorf("--allow-host の名前で /stamp = %d, want 503", code)
	}
	if code, _, _ := fetchHost(t, "GET", base+"/", "evil.example:"+port); code != 403 {
		t.Errorf("よそのホストから / = %d, want 403", code)
	}
	openGate.Do(func() { close(gate) })
	waitFor(t, "読み終わった表示", func() bool { return strings.Contains(out.String(), "1 session loaded in") })
	if !strings.Contains(out.String(), "checking for new history every 1s") {
		t.Errorf("1 秒より短い間隔は 1 秒にするはず: %q", out.String())
	}
	if code, body, _ := fetchHost(t, "GET", base+"/", "localhost:"+port); code != 200 || !strings.Contains(body, "const LIVE = true") {
		t.Errorf("読み終わったあとの / = %d", code)
	}
	if code, body, _ := fetchHost(t, "GET", base+"/stamp", "127.0.0.1:"+port); code != 200 || body != "42" {
		t.Errorf("/stamp = %d %q", code, body)
	}
	srv.Close()
	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("止めたら %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("止まらない")
	}
}

// 最初の読み込みに失敗したら読み込み中の画面にエラーを出し、履歴が変わったら読み直して画面を開けるようにする。
// そのあとの読み直しに失敗しても、前の集計のまま答え続ける。
func TestServeRetryAfterFailure(t *testing.T) {
	logw = io.Discard
	defer func() { logw = os.Stderr }()
	dir := t.TempDir()
	var failing atomic.Bool
	failing.Store(true)
	var n atomic.Int32
	load := func() snapshot {
		if failing.Load() {
			return snapshot{meta: map[string]any{"x": math.NaN()}} // 画面にできず refresh が失敗する
		}
		var data []*core.Session
		for i := int32(0); i <= n.Load(); i++ {
			ts := 1000.0
			b := core.NewBuilder("Claude Code", fmt.Sprintf("s%d", i))
			b.Tick(&ts)
			data = append(data, b.Finish(15))
		}
		return snapshot{data: data, weeks: map[string]*report.Week{}, meta: map[string]any{}, gen: float64(n.Load() + 1)}
	}
	out := &syncBuf{}
	l := &live{load: load, paths: []string{dir}, print: out}
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	stop, done := make(chan struct{}), make(chan struct{})
	go func() { l.start(20*time.Millisecond, stop); close(done) }()
	defer func() { close(stop); <-done }()
	touch := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	waitFor(t, "失敗の表示", func() bool { return strings.Contains(out.String(), "could not read history:") })
	if !strings.Contains(out.String(), "still serving; will try again when history changes") {
		t.Errorf("続けると言っていない: %q", out.String())
	}
	code, body, h := fetchHost(t, "GET", srv.URL+"/stamp", "")
	if code != 503 || body == "" || h.Get("Retry-After") != "1" || h.Get("Cache-Control") != "no-store" {
		t.Errorf("失敗したときの /stamp = %d %q %v", code, body, h)
	}
	if code, body, _ := fetchHost(t, "GET", srv.URL+"/", ""); code != 503 || !strings.Contains(body, "Reading your history") {
		t.Errorf("失敗したときの / = %d", code)
	}

	failing.Store(false)
	touch("a.jsonl")
	waitFor(t, "読み直して開ける", func() bool { code, _, _ := fetchHost(t, "GET", srv.URL+"/stamp", ""); return code == 200 })
	if code, body, _ := fetchHost(t, "GET", srv.URL+"/data.json", ""); code != 200 || !strings.Contains(body, `"s0"`) {
		t.Errorf("読み直したあとの /data.json = %d %.80q", code, body)
	}
	waitFor(t, "読み直した表示", func() bool { return strings.Contains(out.String(), "history changed → 1 sessions (+1)") })

	failing.Store(true)
	n.Store(1)
	touch("b.jsonl")
	waitFor(t, "読み直しの失敗の表示", func() bool { return strings.Contains(out.String(), "reload failed:") })
	if code, body, _ := fetchHost(t, "GET", srv.URL+"/stamp", ""); code != 200 || body != "1" {
		t.Errorf("読み直しに失敗したあとの /stamp = %d %q（前の集計のままのはず）", code, body)
	}
	if l.count() != 1 {
		t.Errorf("読み直しに失敗したのに集計が変わった: %d", l.count())
	}

	failing.Store(false)
	touch("c.jsonl")
	waitFor(t, "もう一度読み直す", func() bool { return l.count() == 2 })
	if code, body, _ := fetchHost(t, "GET", srv.URL+"/stamp", ""); code != 200 || body != "2" {
		t.Errorf("/stamp = %d %q", code, body)
	}
}

// 画面の「kiroku にコピーを残す」が失敗したら 500 でわけを返し、できない serve では 404。
func TestServeArchiveErrors(t *testing.T) {
	logw = io.Discard
	defer func() { logw = os.Stderr }()
	ok := func() snapshot { return snapshot{meta: map[string]any{}} }
	for _, c := range []struct {
		name string
		l    *live
		code int
		body string
	}{
		{"できない", &live{load: ok, ready: true}, 404, ""},
		{"オンにできない", &live{load: ok, ready: true, keep: func() error { return errors.New("disk full: /home/me/secret") }}, 500, "disk full: /home/me/secret"},
		{"読み直せない", &live{load: func() snapshot { return snapshot{meta: map[string]any{"x": math.NaN()}} }, ready: true, keep: func() error { return nil }}, 500, "NaN"},
	} {
		out := &syncBuf{}
		c.l.print = out
		srv := httptest.NewServer(c.l.handler())
		code, body, _ := fetchHost(t, "POST", srv.URL+"/archive", "")
		srv.Close()
		if code != c.code {
			t.Errorf("%s: %d, want %d", c.name, code, c.code)
		}
		// くわしいわけ（ファイルの場所など）は端末にだけ出し、画面には返さない
		if c.body != "" && (strings.Contains(body, c.body) || !strings.Contains(body, "see the terminal") || !strings.Contains(out.String(), c.body)) {
			t.Errorf("%s: 返した文 %q、端末 %q", c.name, body, out.String())
		}
	}
}

// 読み込んだあとで履歴ファイルが消えたら、/history は 404（落ちない）。
func TestServeHistoryGone(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s1.jsonl")
	l := &live{snap: snapshot{data: []*core.Session{{ID: "s1", File: p}}}, ready: true}
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	if code, _, _ := fetchHost(t, "GET", srv.URL+"/history?id=s1", ""); code != 404 {
		t.Errorf("消えた履歴 = %d, want 404", code)
	}
}

// autostart と doctor が見る probe は、読み込み中（503）・開ける（200）・kiroku でない・止まっているを見分ける。
func TestProbeStamp(t *testing.T) {
	l := &live{snap: snapshot{gen: 1}}
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")
	if got := probe(addr); got != loading {
		t.Errorf("読み込み中 = %d, want %d", got, loading)
	}
	l.markReady()
	if got := probe(addr); got != ready {
		t.Errorf("読み終わった = %d, want %d", got, ready)
	}
	other := httptest.NewServer(http.NotFoundHandler())
	defer other.Close()
	if got := probe(strings.TrimPrefix(other.URL, "http://")); got != noAnswer {
		t.Errorf("kiroku でないサーバー = %d, want %d", got, noAnswer)
	}
	srv.Close()
	if got := probe(addr); got != noAnswer {
		t.Errorf("止まっている = %d, want %d", got, noAnswer)
	}
}

// 読み込みにかかった時間は、1 秒未満はミリ秒、それより長いと 0.1 秒まで。
func TestTook(t *testing.T) {
	for in, want := range map[time.Duration]string{1234567 * time.Microsecond: "1.2s", 345678 * time.Microsecond: "346ms"} {
		if got := took(in).String(); got != want {
			t.Errorf("took(%v) = %s, want %s", in, got, want)
		}
	}
}
