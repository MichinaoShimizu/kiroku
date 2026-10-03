package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	stop := make(chan struct{})
	defer close(stop)
	go l.watch(20*time.Millisecond, stop)
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

// 手元だけで待ち受けているときは、よそのホスト名で来たリクエストを断る（DNS リバインディング対策）。
func TestSameOrigin(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h := sameOrigin("127.0.0.1:8484", ok)
	for host, want := range map[string]int{"localhost:8484": 200, "127.0.0.1:8484": 200, "[::1]:8484": 200, "evil.example:8484": 403, "localhost:9999": 403} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("Host %s → %d, want %d", host, w.Code, want)
		}
	}
	// 0.0.0.0 で開いたときは本人の選択なので通す
	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "192.168.0.5:8484"
	w := httptest.NewRecorder()
	sameOrigin("0.0.0.0:8484", ok).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Errorf("0.0.0.0 → %d", w.Code)
	}
}
