package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// countingClaude は、読んだ会話の数を数える Claude Code の Source。
// 会話は並べて読むので、数えるところは mu で守る。
type countingClaude struct {
	*source.Claude
	mu    sync.Mutex
	units int
}

func (c *countingClaude) LoadUnit(u source.Unit, emit func(*core.Builder)) error {
	c.mu.Lock()
	c.units++
	c.mu.Unlock()
	return c.Claude.LoadUnit(u, emit)
}

// countingSource は、Load を呼んだ回数を数える（Unit に分けられない Source）。
type countingSource struct {
	source.Source
	loads int
}

func (c *countingSource) Load(emit func(*core.Builder)) error {
	c.loads++
	return c.Source.Load(emit)
}

func (c *countingSource) Watch() []string { return source.WatchPaths(c.Source) }

// touch は、ファイルの中身を変えずに更新時刻を進める（エージェントが追記したときの代わり）。
func touch(t *testing.T, p string, d time.Duration) {
	t.Helper()
	tm := time.Now().Add(d)
	if err := os.Chtimes(p, tm, tm); err != nil {
		t.Fatal(err)
	}
}

func jsonOf(t *testing.T, data []*core.Session, rep []source.Report) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"sessions": data, "report": rep})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// kiroku serve の読み直しは、変わった会話のファイルだけを読み直し、結果は全部を読み直したときと同じになる。
func TestLoadCacheRereadsOnlyChangedFiles(t *testing.T) {
	setup(t)
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("testdata", "home", ".claude", "projects"))); err != nil {
		t.Fatal(err)
	}
	cl := &countingClaude{Claude: &source.Claude{Root: root}}
	cx := &countingSource{Source: &source.Codex{Home: codexHome(t)}}
	all := []source.Source{cl, cx}
	want := map[string]bool{"claude": true, "codex": true}
	cache := newLoadCache()
	fresh := func() string { d, r := collectAll(all, want); return jsonOf(t, d, r) }

	data, rep := collectCached(all, want, 15, cache)
	n := len(cl.Units())
	if cl.units != n || cx.loads != 1 {
		t.Fatalf("1 回目: 会話 %d / %d、Codex %d 回", cl.units, n, cx.loads)
	}
	if got := jsonOf(t, data, rep); got != fresh() {
		t.Fatal("1 回目の結果が、キャッシュなしと違う")
	}

	// 何も変わっていなければ、何も読まない
	cl.units, cx.loads = 0, 0
	data, rep = collectCached(all, want, 15, cache)
	if cl.units != 0 || cx.loads != 0 {
		t.Errorf("変化なし: 会話 %d、Codex %d 回読んだ", cl.units, cx.loads)
	}
	if got := jsonOf(t, data, rep); got != fresh() {
		t.Error("変化なしの結果が違う")
	}

	// 1 つの会話に追記すると、その会話だけを読み直す（Codex は読まない）
	u := cl.Units()[0]
	f, err := os.OpenFile(u.Files[0], os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"type":"user","timestamp":"2026-09-30T09:59:00Z","message":{"role":"user","content":"追記した依頼"}}` + "\n")
	f.Close()
	touch(t, u.Files[0], time.Minute)
	cl.units, cx.loads = 0, 0
	data, rep = collectCached(all, want, 15, cache)
	if cl.units != 1 || cx.loads != 0 {
		t.Errorf("1 つ追記: 会話 %d、Codex %d 回読んだ, want 1 と 0", cl.units, cx.loads)
	}
	if got := jsonOf(t, data, rep); got != fresh() {
		t.Error("追記したあとの結果が、キャッシュなしと違う")
	}

	// サブエージェントのファイルが増えても、その会話を読み直す
	var withSub source.Unit
	for _, x := range cl.Units() {
		if len(x.Files) > 1 {
			withSub = x
			break
		}
	}
	if withSub.Key == "" {
		t.Fatal("サブエージェントのファイルがある会話が、合成データにない")
	}
	sub := filepath.Join(filepath.Dir(withSub.Files[1]), "agent-added.jsonl")
	b, _ := os.ReadFile(withSub.Files[1])
	os.WriteFile(sub, b, 0o644)
	cl.units = 0
	data, rep = collectCached(all, want, 15, cache)
	if cl.units != 1 {
		t.Errorf("サブエージェントのファイルが増えた: 会話 %d を読んだ, want 1", cl.units)
	}
	if got := jsonOf(t, data, rep); got != fresh() {
		t.Error("サブエージェントのファイルが増えたあとの結果が違う")
	}

	// 会話のファイルを消すと、そのセッションは消える
	before := len(data)
	os.Remove(u.Files[0])
	cl.units = 0
	data, rep = collectCached(all, want, 15, cache)
	if cl.units != 0 || len(data) != before-1 {
		t.Errorf("1 つ消した: 会話 %d を読んだ、セッション %d → %d", cl.units, before, len(data))
	}
	if got := jsonOf(t, data, rep); got != fresh() {
		t.Error("消したあとの結果が違う")
	}

	// Unit に分けられない Source は、見張る場所が変わったら全部を読み直す
	touch(t, filepath.Join(cx.Source.(*source.Codex).Home, "session_index.jsonl"), 2*time.Minute)
	cl.units, cx.loads = 0, 0
	data, rep = collectCached(all, want, 15, cache)
	if cl.units != 0 || cx.loads != 1 {
		t.Errorf("Codex が変わった: 会話 %d、Codex %d 回, want 0 と 1", cl.units, cx.loads)
	}
	if got := jsonOf(t, data, rep); got != fresh() {
		t.Error("Codex が変わったあとの結果が違う")
	}

	// 区切りの分数が変われば、全部を読み直す
	cl.units = 0
	collectCached(all, want, 30, cache)
	if cl.units != len(cl.Units()) {
		t.Errorf("区切りを変えた: 会話 %d / %d", cl.units, len(cl.Units()))
	}
}

// countingCodex は、読んだまとまりの数を数える Codex の Source。
type countingCodex struct {
	*source.Codex
	mu    sync.Mutex
	units int
}

func (c *countingCodex) LoadUnit(u source.Unit, emit func(*core.Builder)) error {
	c.mu.Lock()
	c.units++
	c.mu.Unlock()
	return c.Codex.LoadUnit(u, emit)
}

// Codex: ログなど会話でないファイルが変わっても読み直さず、追記されたスレッド（とその親子）だけを読み直す。
func TestLoadCacheCodexUnits(t *testing.T) {
	setup(t)
	home := codexHome(t)
	cx := &countingCodex{Codex: &source.Codex{Home: home}}
	all := []source.Source{cx}
	want := map[string]bool{"codex": true}
	cache := newLoadCache()
	fresh := func() string {
		d, r := collect([]source.Source{&source.Codex{Home: home}}, want, 15)
		return jsonOf(t, d, r)
	}
	check := func(what string, wantUnits int) {
		t.Helper()
		cx.units = 0
		data, rep := collectCached(all, want, 15, cache)
		if cx.units != wantUnits {
			t.Errorf("%s: まとまり %d を読んだ, want %d", what, cx.units, wantUnits)
		}
		if got := jsonOf(t, data, rep); got != fresh() {
			t.Errorf("%s: 結果が、キャッシュなしと違う", what)
		}
	}
	appendLine := func(p, line string, d time.Duration) {
		t.Helper()
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.WriteString(line + "\n")
		f.Close()
		touch(t, p, d)
	}

	check("1 回目", len(cx.Units()))

	// ログや history.jsonl が書かれても、何も読まない
	os.MkdirAll(filepath.Join(home, "log"), 0o755)
	appendLine(filepath.Join(home, "log", "codex-tui.log"), "INFO something", time.Minute)
	appendLine(filepath.Join(home, "history.jsonl"), `{"session_id":"thr-main","text":"x"}`, time.Minute)
	check("ログが変わった", 0)

	// サブエージェントのファイルに追記すると、親と一緒の 1 まとまりだけを読み直す（圧縮ファイルは読まない）
	sub := filepath.Join(home, "sessions", "2026", "09", "29", "rollout-2026-09-29T10-01-00-thr-sub.jsonl")
	appendLine(sub, `{"timestamp": "2026-09-29T01:03:00.000Z", "type": "event_msg", "payload": {"type": "user_message", "message": "追記した依頼"}}`, 2*time.Minute)
	check("サブエージェントに追記", 1)

	// スレッド名が変わると、そのスレッドだけを読み直す
	appendLine(filepath.Join(home, "session_index.jsonl"), `{"id": "thr-main", "thread_name": "新しいタイトル"}`, 3*time.Minute)
	check("スレッド名が変わった", 1)
	data, _ := collectCached(all, want, 15, cache)
	for _, s := range data {
		if s.ID == "thr-main" && s.Title != "新しいタイトル" {
			t.Errorf("タイトル = %q", s.Title)
		}
	}
}

// collectAll は、キャッシュを使わずに全部を読む（数を数える Source の数は戻す）。
func collectAll(all []source.Source, want map[string]bool) ([]*core.Session, []source.Report) {
	saved := []int{}
	for _, s := range all {
		switch c := s.(type) {
		case *countingClaude:
			saved = append(saved, c.units)
		case *countingSource:
			saved = append(saved, c.loads)
		}
	}
	data, rep := collect(all, want, 15)
	for i, s := range all {
		switch c := s.(type) {
		case *countingClaude:
			c.units = saved[i]
		case *countingSource:
			c.loads = saved[i]
		}
	}
	return data, rep
}

// kiroku serve の読み直しで、週と月の集計（report.Cache）は全部を集計し直したときと同じになる。
// report.Cache が頼る「変わっていない会話のセッションは同じポインタのまま」も確かめる。
func TestReportCacheFollowsLoadCache(t *testing.T) {
	setup(t)
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("testdata", "home", ".claude", "projects"))); err != nil {
		t.Fatal(err)
	}
	cl := &countingClaude{Claude: &source.Claude{Root: root}}
	all := []source.Source{cl}
	want := map[string]bool{"claude": true}
	cache, rcache := newLoadCache(), report.NewCache()
	aggregate := func(data []*core.Session) string {
		b, _ := json.Marshal([]any{rcache.AllWeeks(data), rcache.AllMonths(data)})
		return string(b)
	}
	fresh := func() string {
		d, _ := collectAll(all, want)
		b, _ := json.Marshal([]any{report.AllWeeks(d), report.AllMonths(d)})
		return string(b)
	}

	first, _ := collectCached(all, want, 15, cache)
	if got := aggregate(first); got != fresh() {
		t.Fatal("1 回目の集計が、キャッシュなしと違う")
	}
	u := cl.Units()[0]
	f, err := os.OpenFile(u.Files[0], os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"type":"user","timestamp":"2026-09-30T09:59:00Z","message":{"role":"user","content":"いや、そうじゃない"}}` + "\n")
	f.Close()
	touch(t, u.Files[0], time.Minute)
	second, _ := collectCached(all, want, 15, cache)
	if got := aggregate(second); got != fresh() {
		t.Error("追記したあとの集計が、キャッシュなしと違う")
	}
	prev := map[string]*core.Session{}
	for _, s := range first {
		prev[s.ID] = s
	}
	reused := 0
	for _, s := range second {
		if prev[s.ID] == s {
			reused++
		} else if s.File != u.Files[0] {
			t.Errorf("読み直していない会話 %s のセッションが新しくなった", s.ID)
		}
	}
	if reused == 0 || reused == len(second) {
		t.Errorf("使い回したセッション %d / %d", reused, len(second))
	}
}
