package cli

import (
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	l := &live{snap: snapshot{data: []*core.Session{{ID: "s1", File: p}, {ID: "db", File: filepath.Join(dir, "data.sqlite3")}}}, ready: true}
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
	for _, c := range []struct{ method, path string }{{"GET", "/stamp"}, {"GET", "/data.json"}, {"GET", "/history?id=s1"}, {"GET", "/prompt?id=s1&i=0"}, {"POST", "/archive"}} {
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
