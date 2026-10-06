package cli

import (
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

var wantHeaders = map[string]string{
	"X-Frame-Options":              "DENY",
	"Content-Security-Policy":      "frame-ancestors 'none'",
	"X-Content-Type-Options":       "nosniff",
	"Referrer-Policy":              "no-referrer",
	"Cross-Origin-Opener-Policy":   "same-origin",
	"Cross-Origin-Resource-Policy": "same-origin",
}

func checkHeaders(t *testing.T, what string, h http.Header) {
	t.Helper()
	for k, v := range wantHeaders {
		if got := h.Get(k); got != v {
			t.Errorf("%s: %s = %q, want %q", what, k, got, v)
		}
	}
}

// kiroku serve のすべての答え（読み込み中・読み終わったあと・エラー・よそのホスト）に、守りのヘッダーがつく。
func TestServeSecurityHeaders(t *testing.T) {
	logw = io.Discard
	defer func() { logw = os.Stderr }()
	dir := t.TempDir()
	p := filepath.Join(dir, "s1.jsonl")
	os.WriteFile(p, []byte(`{"type":"user"}`+"\n"), 0o600)
	ts := 1000.0
	b := core.NewBuilder("Claude Code", "s1")
	b.Tick(&ts)
	b.Prompt(&ts, "依頼")
	s := b.Finish(15)
	s.File = p
	l := &live{load: func() snapshot { return snapshot{data: []*core.Session{s}, meta: map[string]any{}, gen: 1} },
		print: io.Discard, roots: []string{dir}, keep: func() error { return nil }}
	srv := httptest.NewServer(secureHeaders(sameOrigin("127.0.0.1:8484", nil, l.handler())))
	defer srv.Close()
	const host = "localhost:8484"
	paths := []struct{ method, path string }{{"GET", "/"}, {"GET", "/data.json"}, {"GET", "/stamp"}, {"GET", "/history?id=s1"}, {"GET", "/prompt?id=s1&i=0"}, {"POST", "/archive"}, {"GET", "/nope"}}
	for _, c := range paths { // 読み込み中（503 と読み込み中の画面）
		_, _, h := fetchHost(t, c.method, srv.URL+c.path, host)
		checkHeaders(t, "読み込み中の "+c.path, h)
	}
	if err := l.refresh(); err != nil {
		t.Fatal(err)
	}
	l.markReady()
	for _, c := range paths {
		code, _, h := fetchHost(t, c.method, srv.URL+c.path, host)
		if c.path != "/nope" && code != 200 {
			t.Errorf("%s = %d", c.path, code)
		}
		checkHeaders(t, c.path, h)
	}
	code, _, h := fetchHost(t, "GET", srv.URL+"/", "evil.example:8484")
	if code != 403 {
		t.Errorf("よそのホスト = %d", code)
	}
	checkHeaders(t, "よそのホスト", h)
	code, _, h = fetchHost(t, "GET", srv.URL+"/archive", host)
	if code != 403 {
		t.Errorf("GET /archive = %d", code)
	}
	checkHeaders(t, "GET /archive", h)
}

// 最初の読み込みに失敗したときの 503 の本文には、くわしいわけ（ファイルの場所など）を出さず、端末に出す。
func TestServeLoadFailsHidesDetails(t *testing.T) {
	logw = io.Discard
	defer func() { logw = os.Stderr }()
	out := &syncBuf{}
	l := &live{load: func() snapshot { return snapshot{meta: map[string]any{"/home/me/secret": math.NaN()}} }, print: out}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() { l.start(time.Hour, stop); close(done) }()
	defer func() { close(stop); <-done }()
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	waitFor(t, "失敗の表示", func() bool { return strings.Contains(out.String(), "could not read history:") })
	code, body, _ := fetchHost(t, "GET", srv.URL+"/stamp", "")
	if code != 503 || !strings.Contains(body, "see the terminal") || strings.Contains(body, "NaN") {
		t.Errorf("/stamp = %d %q", code, body)
	}
	if !strings.Contains(out.String(), "NaN") {
		t.Errorf("端末にわけが出ていない: %q", out.String())
	}
}

// /history は、読んでいる履歴の場所（と kiroku archive のコピーの場所）の下のファイルだけを見せる。
// セッションのファイル名がほかの場所を指していても、シンボリックリンクでほかの場所へ抜けても見せない。
func TestServeHistoryOnlyUnderRoots(t *testing.T) {
	base := t.TempDir()
	root, outside := filepath.Join(base, "root"), filepath.Join(base, "outside")
	for _, d := range []string{root, outside} {
		os.MkdirAll(d, 0o700)
	}
	in := filepath.Join(root, "in.jsonl")
	out := filepath.Join(outside, "creds.json")
	os.WriteFile(in, []byte("inside\n"), 0o600)
	os.WriteFile(out, []byte("secret\n"), 0o600)
	data := []*core.Session{{ID: "in", File: in}, {ID: "out", File: out}, {ID: "dots", File: filepath.Join(root, "..", "outside", "creds.json")}}
	if runtime.GOOS != "windows" { // シンボリックリンクを作るには権限がいる
		link := filepath.Join(root, "link.json")
		if err := os.Symlink(out, link); err != nil {
			t.Fatal(err)
		}
		data = append(data, &core.Session{ID: "link", File: link})
	}
	l := &live{snap: snapshot{data: data}, ready: true, roots: []string{root}}
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	if code, body, _ := fetchHost(t, "GET", srv.URL+"/history?id=in", ""); code != 200 || body != "inside\n" {
		t.Errorf("in = %d %q", code, body)
	}
	for _, id := range []string{"out", "dots", "link"} {
		if code, body, _ := fetchHost(t, "GET", srv.URL+"/history?id="+id, ""); code != 404 || strings.Contains(body, "secret") {
			t.Errorf("%s = %d %q, want 404", id, code, body)
		}
	}
	// roots がなければ何も見せない
	l.roots = nil
	if code, _, _ := fetchHost(t, "GET", srv.URL+"/history?id=in", ""); code != 404 {
		t.Errorf("roots なし = %d, want 404", code)
	}
}

// 監査で見つかった再現: Kiro IDE（v1.0 より前）の sessions.json に sessionId "../../outside/creds" と書くと、
// 履歴の場所の外の creds.json を /history で読めた。いまは読み込みで捨て、/history も 404。
func TestServeKiroLegacyTraversal(t *testing.T) {
	logw = io.Discard
	defer func() { logw = os.Stderr }()
	base := t.TempDir()
	gs := filepath.Join(base, "storage", "kiro.kiroagent")
	ws := filepath.Join(gs, "workspace-sessions", "d3M=")
	os.MkdirAll(ws, 0o700)
	os.MkdirAll(filepath.Join(gs, "outside"), 0o700)
	os.WriteFile(filepath.Join(ws, "sessions.json"), []byte(`[{"sessionId": "../../outside/creds", "dateCreated": 1759100000000}, {"sessionId": "ok", "dateCreated": 1759100000000}]`), 0o600)
	os.WriteFile(filepath.Join(ws, "ok.json"), []byte(`{"history": []}`), 0o600)
	os.WriteFile(filepath.Join(gs, "outside", "creds.json"), []byte(`{"history": [{"message": {"role": "user", "content": "secret"}}]}`), 0o600)
	picked := []source.Source{&source.KiroIDELegacy{Storages: []string{gs}}}
	var data []*core.Session
	err := picked[0].Load(func(b *core.Builder) { data = append(data, b.Finish(15)) })
	if err == nil || !strings.Contains(err.Error(), "unsafe sessionId") {
		t.Errorf("Load のエラー = %v", err)
	}
	l := &live{snap: snapshot{data: data}, ready: true, roots: historyRoots(picked)}
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	if code, body, _ := fetchHost(t, "GET", srv.URL+"/history?id=../../outside/creds", ""); code != 404 || strings.Contains(body, "secret") {
		t.Errorf("../../outside/creds = %d %q, want 404", code, body)
	}
	if code, _, _ := fetchHost(t, "GET", srv.URL+"/history?id=ok", ""); code != 200 {
		t.Errorf("ok = %d, want 200", code)
	}
}

// /history で見せてよい場所は、Source が読む場所と kiroku archive のコピーの場所。
func TestHistoryRoots(t *testing.T) {
	picked := source.All(source.Options{ClaudeRoot: "/c/projects", KiroHome: "/k", KiroStorages: []string{"/legacy"}, CodexHome: "/x", Archive: "/arch"})
	got := strings.Join(historyRoots(picked), "|")
	for _, want := range []string{"/c/projects", filepath.Join("/arch", "claude"), filepath.Join("/k", "sessions"), "/legacy", filepath.Join("/x", "sessions"), filepath.Join("/arch", "crew", "sessions", "archive")} {
		if !strings.Contains(got, want) {
			t.Errorf("%q がない: %s", want, got)
		}
	}
	if !within("/a/b", "/a/b/c") || !within("/a/b", "/a/b") || within("/a/b", "/a/bc") || within("/a/b", "/a") {
		t.Error("within がおかしい")
	}
}
