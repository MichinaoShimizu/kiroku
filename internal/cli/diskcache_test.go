package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
	if _, _, err := other.read(); err == nil {
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
	if removed, err := removeDiskCache(); err != nil || !removed {
		t.Fatal(removed, err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("cache not removed")
	}
	if removed, err := removeDiskCache(); err != nil || removed {
		t.Error(removed, err)
	}
	t.Setenv("KIROKU_CACHE_DIR", "relative")
	if _, err := cacheDir(); err == nil {
		t.Error("accepted a relative KIROKU_CACHE_DIR")
	}
	if _, err := removeDiskCache(); err == nil {
		t.Error("--no-cache with a relative KIROKU_CACHE_DIR said nothing")
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

// あとから中身を残さないことにした（persistent から incognito にした）Crew の会話の写しは、前に残したファイルからも消える。
// 隠すものを調べきれないとき（session_map.json が壊れている）は、前に残したファイルを消す。
func TestDiskCacheDropsLaterPrivate(t *testing.T) {
	quietLog(t)
	all := crewPrivateSources(t)
	crew := all[1].(*source.KiroCLI).CrewHome
	transcript := filepath.Join(crew, "sessions", "dashboard_chat-1-100.jsonl")
	orig, err := os.ReadFile(transcript)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(transcript, bytes.Replace(orig, []byte(`"incognito"`), []byte(`"persistent"`), 1), 0o600)
	// ほかの中身を残さない会話（chat-4）は残さないので、起動のたびに読み直してファイルを書き直してしまう。
	// 変えた会話のことだけを確かめるため、中身を残すことにする
	other := filepath.Join(crew, "sessions", "dashboard_chat-4-400.jsonl")
	b4, _ := os.ReadFile(other)
	os.WriteFile(other, bytes.Replace(b4, []byte(`"incognito"`), []byte(`"persistent"`), 1), 0o600)
	dir := filepath.Join(t.TempDir(), "cache")
	file := filepath.Join(dir, diskCacheFile)
	want := map[string]bool{"kiro": true, "claude": true, "codex": true}
	run := func() {
		t.Helper()
		c := newLoadCache()
		c.disk = openDiskCache(dir, 15, "", nil)
		data, _ := collectCached(all, want, 15, c)
		c.saveToDisk()
		c.disk.wait()
		_ = data
	}
	run()
	if b, _ := os.ReadFile(file); !bytes.Contains(b, []byte("SECRET-CLAUDE-CURRENT")) {
		t.Fatal("a persistent conversation was not saved (the test checks nothing)")
	}
	os.WriteFile(transcript, orig, 0o600) // incognito にした。Claude Code の履歴は変わらない
	run()
	if b, _ := os.ReadFile(file); bytes.Contains(b, []byte("SECRET-CLAUDE-CURRENT")) {
		t.Error("the disk cache still holds a conversation made private after it was saved")
	}

	os.WriteFile(transcript, bytes.Replace(orig, []byte(`"incognito"`), []byte(`"persistent"`), 1), 0o600)
	run()
	os.WriteFile(filepath.Join(crew, "session_map.json"), []byte(`{"dashboard:chat-1-100": {"sid": "c-priv"`), 0o600)
	run()
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Error("kept the disk cache although kiroku could not check which conversations are private")
	}
}

// 裏で書くものは、読み込みを終えたときの結果と隠した会話の組。読み込みの途中（エージェントの 1 つを読み直して、
// 隠すものを調べ直す前）に書いても、新しく現れた中身を残さない会話の写しは入らない。
func TestDiskSnapshotIsConsistent(t *testing.T) {
	quietLog(t)
	all := crewPrivateSources(t)
	claudeSrc, crew := all[0].(*source.Claude), all[1].(*source.KiroCLI).CrewHome
	want := map[string]bool{"kiro": true, "claude": true}
	c := newLoadCache()
	collectCached(all, want, 15, c)
	writeUnder(t, crew, map[string]string{
		"session_map.json":                    `{"dashboard:chat-1-100": {"sid": "c-priv", "provider": "claude_code", "cwd": "/w"}, "dashboard:chat-5-500": {"sid": "c-new", "provider": "claude_code", "cwd": "/w"}}`,
		"sessions/dashboard_chat-5-500.jsonl": `{"_type": "metadata", "title": "t", "memory_mode": "incognito"}` + "\n",
	})
	writeUnder(t, claudeSrc.Root, map[string]string{"-w/c-new.jsonl": claudeTranscript("SECRET-NEW")})
	c.load(claudeSrc, 15) // 隠すものを調べ直す前
	has := func() bool {
		df, err := c.diskSnapshot("b")
		if err != nil {
			t.Fatal(err)
		}
		for _, ds := range df.sources {
			for _, u := range ds.units {
				for _, o := range u.Outs {
					if o.Sess != nil && strings.Contains(o.Sess.ID, "c-new") {
						return true
					}
				}
			}
		}
		return false
	}
	if has() {
		t.Error("the snapshot holds a new private copy before it was checked")
	}
	collectCached(all, want, 15, c)
	if has() {
		t.Error("the snapshot holds a private copy")
	}
}

// 時間帯が変わったら使わない（時間帯のない時刻はローカル時刻として読むので）。前からあるフォルダは、書かないときも 0700 にする。
// 画面の JSON にできない値の入った Unit は使わない。
func TestDiskCacheZoneDirAndValues(t *testing.T) {
	setup(t)
	logw = io.Discard
	root := t.TempDir()
	writeClaudeHistory(t, root, 1, 2)
	dir := filepath.Join(t.TempDir(), "cache")
	c := newLoadCache()
	c.disk = openDiskCache(dir, 15, "", nil)
	collectCached([]source.Source{&source.Claude{Root: root}}, map[string]bool{"claude": true}, 15, c)
	c.saveToDisk()
	c.disk.wait()
	old := time.Local
	time.Local = time.UTC
	if d := openDiskCache(dir, 15, "", nil); d != nil && len(d.loaded) > 0 {
		t.Error("used a cache saved in another time zone")
	}
	time.Local = old
	if runtime.GOOS != "windows" {
		os.Chmod(dir, 0o755)
		openDiskCache(dir, 15, "", nil)
		if st, _ := os.Stat(dir); st.Mode().Perm() != 0o700 {
			t.Errorf("cache folder left %o", st.Mode().Perm())
		}
	}
	bad := &core.Session{Tools: [][2]any{{"Bash", complex(1, 2)}}}
	if _, ok := restoreUnit(diskUnit{Outs: []diskLoaded{{Sess: bad}}}); ok {
		t.Error("accepted a value the view can't show")
	}
	good := &core.Session{Tools: [][2]any{{"Bash", 2}}}
	if _, ok := restoreUnit(diskUnit{Outs: []diskLoaded{{Sess: good}}}); !ok {
		t.Error("rejected an ordinary session")
	}
}
