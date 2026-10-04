package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// countingClaude は、読んだ会話の数を数える Claude Code の Source。
type countingClaude struct {
	*source.Claude
	units int
}

func (c *countingClaude) LoadUnit(u source.Unit, emit func(*core.Builder)) error {
	c.units++
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
