package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
	"github.com/MichinaoShimizu/kiroku/internal/web"
)

// testSources は、testdata のすべての合成データと、ベンチマークと同じ合成の Claude Code の履歴（root）。
func testSources(t *testing.T, root string) []source.Source {
	t.Helper()
	h := filepath.Join("testdata", "home")
	all := source.All(source.Options{
		ClaudeRoot: filepath.Join(h, ".claude", "projects"),
		KiroHome:   filepath.Join(h, ".kiro"),
		CrewHome:   filepath.Join("testdata", "crew"),
		KiroCLIDB:  filepath.Join("testdata", "sqlite", "kiro-cli.sqlite3"),
		CodexHome:  codexHomeOnce(t),
	})
	return append(all, &source.Claude{Root: root})
}

var codexOnce string

func codexHomeOnce(t *testing.T) string {
	if codexOnce == "" {
		codexOnce = codexHome(t)
	}
	return codexOnce
}

// viewOf は、画面が受け取るもの（セッション・週・月の JSON）と、kiroku serve が全文として見せるもの。
func viewOf(t *testing.T, data []*core.Session) ([]byte, []string) {
	t.Helper()
	parts, err := web.Marshal(data, report.AllWeeks(data), report.AllMonths(data), map[string]any{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fulls []string
	for _, d := range data {
		for _, p := range d.Prompts {
			fulls = append(fulls, p.Full())
			if p.Reply != nil {
				fulls = append(fulls, p.Reply.Full())
			}
		}
	}
	return parts.JSON(), fulls
}

// 前に起動したときにディスクに残した結果から読んでも、全部を読み直したときと同じものが見え、
// 変わっていない会話のファイルは読み直さない。ファイルは本人だけが読める。
func TestDiskCacheSameView(t *testing.T) {
	setup(t)
	logw = io.Discard
	root := t.TempDir()
	writeClaudeHistory(t, root, 20, 4)
	dir := filepath.Join(t.TempDir(), "cache")
	want := map[string]bool{"claude": true, "kiro": true, "codex": true}

	fresh, _ := collect(testSources(t, root), want, 15)
	wantJS, wantFull := viewOf(t, fresh)

	c1 := newLoadCache()
	c1.disk = openDiskCache(dir, 15, "", t.Logf)
	if c1.disk == nil {
		t.Fatal("no disk cache")
	}
	first, _ := collectCached(testSources(t, root), want, 15, c1)
	c1.saveToDisk()
	c1.disk.wait()
	if js, _ := viewOf(t, first); !bytes.Equal(js, wantJS) {
		t.Fatal("first load differs from collect")
	}
	if runtime.GOOS != "windows" {
		for p, mode := range map[string]os.FileMode{dir: 0o700, filepath.Join(dir, diskCacheFile): 0o600} {
			st, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if st.Mode().Perm() != mode {
				t.Errorf("%s: mode %o, want %o", p, st.Mode().Perm(), mode)
			}
		}
	}

	// 新しいプロセスのつもりで、Source も作り直す
	var reads atomic.Int64
	srcs := testSources(t, root)
	for i, s := range srcs {
		if c, ok := s.(*source.Claude); ok {
			srcs[i] = &diskCountClaude{Claude: c, n: &reads}
		}
	}
	c2 := newLoadCache()
	c2.disk = openDiskCache(dir, 15, "", t.Logf)
	second, _ := collectCached(srcs, want, 15, c2)
	gotJS, gotFull := viewOf(t, second)
	if !bytes.Equal(gotJS, wantJS) {
		t.Errorf("view from the disk cache differs (%d bytes, want %d)", len(gotJS), len(wantJS))
		for i := range min(len(gotJS), len(wantJS)) {
			if gotJS[i] != wantJS[i] {
				t.Logf("first difference at %d: got %q want %q", i, gotJS[max(0, i-80):min(len(gotJS), i+80)], wantJS[max(0, i-80):min(len(wantJS), i+80)])
				break
			}
		}
	}
	if !reflect.DeepEqual(gotFull, wantFull) {
		t.Error("full prompts and replies differ")
	}
	for i := range fresh {
		if i < len(second) && !reflect.DeepEqual(fresh[i], second[i]) {
			t.Errorf("session %s differs from a fresh read", fresh[i].ID)
			break
		}
	}
	if n := reads.Load(); n != 0 {
		t.Errorf("read %d Claude Code conversations again, want 0", n)
	}

	// 1 つ書き換えると、それだけ読み直す
	files, _ := filepath.Glob(filepath.Join(root, "*", "*.jsonl"))
	f, err := os.OpenFile(files[0], os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"type":"user","timestamp":"2025-10-30T10:00:00Z","cwd":"/Users/me/web-app","message":{"role":"user","content":"one more"}}` + "\n")
	f.Close()
	reads.Store(0)
	c3 := newLoadCache()
	c3.disk = openDiskCache(dir, 15, "", t.Logf)
	collectCached(srcs, want, 15, c3)
	if n := reads.Load(); n != 1 {
		t.Errorf("read %d conversations after one changed, want 1", n)
	}
}

type diskCountClaude struct {
	*source.Claude
	n *atomic.Int64
}

func (c *diskCountClaude) LoadUnit(u source.Unit, emit func(*core.Builder)) error {
	c.n.Add(1)
	return c.Claude.LoadUnit(u, emit)
}

// ほかの版の kiroku・ほかの --gap・ほかの人も読めるファイルのものは使わない。
func TestDiskCacheRejects(t *testing.T) {
	setup(t)
	logw = io.Discard
	root := t.TempDir()
	writeClaudeHistory(t, root, 2, 2)
	dir := filepath.Join(t.TempDir(), "cache")
	want := map[string]bool{"claude": true}
	srcs := []source.Source{&source.Claude{Root: root}}
	c := newLoadCache()
	c.disk = openDiskCache(dir, 15, "", nil)
	collectCached(srcs, want, 15, c)
	c.saveToDisk()
	c.disk.wait()
	has := func(d *diskCache) bool { return d != nil && len(d.loaded) > 0 }
	if !has(openDiskCache(dir, 15, "", nil)) {
		t.Fatal("saved cache not read back")
	}
	if has(openDiskCache(dir, 20, "", nil)) {
		t.Error("used a cache saved with another --gap")
	}
	prices := filepath.Join(t.TempDir(), "p.json")
	os.WriteFile(prices, []byte(`{}`), 0o600)
	if has(openDiskCache(dir, 15, prices, nil)) {
		t.Error("used a cache saved without --prices")
	}
	// 別の実行ファイルのもの
	b, _ := os.ReadFile(filepath.Join(dir, diskCacheFile))
	other := &diskCache{path: filepath.Join(dir, diskCacheFile), build: "other", gap: 15}
	if _, err := other.read(); err == nil {
		t.Error("used a cache saved by another kiroku")
	}
	if runtime.GOOS != "windows" {
		os.Chmod(filepath.Join(dir, diskCacheFile), 0o644)
		if has(openDiskCache(dir, 15, "", nil)) {
			t.Error("used a cache file other users can read")
		}
	}
	// 壊れたファイルは使わない（落ちない）
	os.WriteFile(filepath.Join(dir, diskCacheFile), b[:len(b)/2], 0o600)
	if has(openDiskCache(dir, 15, "", nil)) {
		t.Error("used a truncated cache")
	}
}

// --no-cache は残した結果を消す。
func TestRemoveDiskCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KIROKU_CACHE_DIR", dir)
	p := filepath.Join(dir, diskCacheFile)
	os.WriteFile(p, []byte("x"), 0o600)
	if err := removeDiskCache(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("cache not removed")
	}
	if err := removeDiskCache(); err != nil {
		t.Error(err)
	}
	t.Setenv("KIROKU_CACHE_DIR", "relative")
	if _, err := cacheDir(); err == nil {
		t.Error("accepted a relative KIROKU_CACHE_DIR")
	}
}

// 中身を残さない Kiro Crew の会話の写し（Claude Code・Codex の記録）は、ディスクに残さない。
// Kiro を選ばないとき（--sources claude,codex）も同じ。次の起動でも隠したまま出す。
func TestDiskCacheSkipsCrewPrivate(t *testing.T) {
	quietLog(t)
	for _, want := range []map[string]bool{{"kiro": true, "claude": true, "codex": true}, {"claude": true, "codex": true}} {
		all := crewPrivateSources(t)
		dir := filepath.Join(t.TempDir(), "cache")
		c := newLoadCache()
		c.disk = openDiskCache(dir, 15, "", nil)
		collectCached(all, want, 15, c)
		c.saveToDisk()
		c.disk.wait()
		b, err := os.ReadFile(filepath.Join(dir, diskCacheFile))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte("SECRET")) {
			t.Errorf("sources %v: the disk cache holds a private Crew conversation", want)
		}
		if !bytes.Contains(b, []byte("VISIBLE-CLAUDE")) {
			t.Errorf("sources %v: the disk cache lacks an ordinary conversation", want)
		}
		c2 := newLoadCache()
		c2.disk = openDiskCache(dir, 15, "", nil)
		data, _ := collectCached(all, want, 15, c2)
		assertPrivate(t, data, "Claude Code", "c-priv")
		assertVisible(t, data, "Claude Code", "c-pub", "VISIBLE-CLAUDE")
	}
}
