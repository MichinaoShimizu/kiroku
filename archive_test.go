package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/archive"
	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// テストで、使っている人の kiroku archive の保存場所を読み書きしない。
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "kiroku-archive-")
	if err != nil {
		panic(err)
	}
	os.Setenv("KIROKU_ARCHIVE_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func writeClaude(t *testing.T, root, id string) string {
	t.Helper()
	dir := filepath.Join(root, "-Users-me-app")
	os.MkdirAll(dir, 0o755)
	p := filepath.Join(dir, id+".jsonl")
	os.WriteFile(p, []byte(`{"type":"user","timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":"直して"}}`+"\n"+
		`{"type":"assistant","timestamp":"2026-09-30T01:01:00Z","message":{"id":"m1","model":"claude-sonnet-5-5","content":[{"type":"text","text":"ok"}]}}`+"\n"), 0o644)
	return p
}

// kiroku archive on で保存を始め、Claude Code が会話を消しても読める。off は保存をやめる（コピーは残して読む）。
func TestArchiveCommand(t *testing.T) {
	root, dir := t.TempDir(), filepath.Join(t.TempDir(), "archive")
	p := writeClaude(t, root, "s1")
	flags := []string{"--root", root, "--sources", "claude", "--archive-dir", dir}
	if err := dispatch(append([]string{"archive"}, flags...)); err != nil || archive.Enabled(dir) {
		t.Fatalf("状態を見るだけでオンにした: %v", err)
	}
	if err := dispatch(append([]string{"archive", "on"}, flags...)); err != nil || !archive.Enabled(dir) {
		t.Fatalf("on: %v", err)
	}
	if files, _ := archive.Usage(dir); files != 1 {
		t.Fatalf("on のときに今ある履歴を残していない: %d", files)
	}
	writeClaude(t, root, "s2") // オンのあいだは、読むたびに残す
	os.Remove(p)
	fs := newFS("t", "")
	c := addCommon(fs)
	if err := fs.Parse(flags); err != nil {
		t.Fatal(err)
	}
	_, load, _ := c.loader()
	snap := load()
	if len(snap.data) != 2 || snap.rep[0].Archived != 1 {
		t.Fatalf("セッション %d・コピーから %d, want 2 と 1", len(snap.data), snap.rep[0].Archived)
	}
	if a := snap.meta["archive"].(map[string]any); a["on"] != true || a["files"] != 2 {
		t.Errorf("meta.archive = %v", a)
	}
	if err := dispatch(append([]string{"archive", "off"}, flags...)); err != nil || archive.Enabled(dir) {
		t.Fatalf("off: %v", err)
	}
	if files, _ := archive.Usage(dir); files != 2 {
		t.Errorf("off で（聞かずに）コピーを消した: %d", files)
	}
	if snap := load(); len(snap.data) != 2 {
		t.Errorf("止めたあとも、残したコピーは読む: %d", len(snap.data))
	}
	if err := dispatch(append([]string{"archive", "maybe"}, flags...)); err == nil {
		t.Error("知らない引数でエラーにならない")
	}
}

// 画面の「kiroku にコピーを残す」は、画面からの POST（X-Kiroku 付き）だけを受け付ける。
func TestServeArchive(t *testing.T) {
	enabled := 0
	l := &live{load: func() snapshot { return snapshot{meta: map[string]any{"archive": map[string]any{"on": enabled > 0}}} },
		keep: func() error { enabled++; return nil }, ready: true}
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	post := func(method string, h map[string]string) int {
		req, _ := http.NewRequest(method, srv.URL+"/archive", nil)
		for k, v := range h {
			req.Header.Set(k, v)
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	for name, c := range map[string]struct {
		m string
		h map[string]string
	}{
		"GET":      {"GET", map[string]string{"X-Kiroku": "1"}},
		"ヘッダーなし":   {"POST", nil},
		"ほかのサイトから": {"POST", map[string]string{"X-Kiroku": "1", "Origin": "https://evil.example"}},
	} {
		if code := post(c.m, c.h); code != 403 {
			t.Errorf("%s = %d, want 403", name, code)
		}
	}
	if enabled != 0 {
		t.Fatal("断ったのにオンにした")
	}
	if code := post("POST", map[string]string{"X-Kiroku": "1", "Origin": srv.URL}); code != 200 || enabled != 1 {
		t.Errorf("画面から = %d（オン %d 回）", code, enabled)
	}
	if !strings.Contains(string(l.json), `"on":true`) {
		t.Errorf("読み直していない: %s", l.json)
	}
}

// /history は、kiroku archive のコピー（.jsonl.zst）をほどいて見せる。
func TestServeHistoryArchived(t *testing.T) {
	root, dir := t.TempDir(), t.TempDir()
	writeClaude(t, root, "s1")
	if _, err := archive.Sync(root, dir); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "-Users-me-app", "s1.jsonl.zst")
	l := &live{snap: snapshot{data: []*core.Session{{ID: "s1", File: p}}}, ready: true}
	srv := httptest.NewServer(l.handler())
	defer srv.Close()
	r, err := http.Get(srv.URL + "/history?id=s1")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	if r.StatusCode != 200 || !strings.Contains(string(b), "直して") {
		t.Errorf("= %d %q", r.StatusCode, b)
	}
}
