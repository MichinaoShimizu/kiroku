package report

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
)

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// cacheSess は ws から off 時間後に始まり min 分動いたセッション。
func cacheSess(id, source, project string, ws time.Time, off, min float64) *core.Session {
	t0 := float64(ws.Unix()) + off*3600
	return &core.Session{ID: id, Source: source, Project: project, Title: id, Start: t0, End: t0 + min*60,
		Segs: [][3]float64{{t0, t0 + min*60, 1}}, Prompts: []core.Prompt{}}
}

// cacheData は 10 週ぶんのセッション（週の境目をまたぐものを含む）とコミット。
func cacheData() ([]*core.Session, []gitlog.Commit) {
	ws := time.Date(2026, 7, 6, 0, 0, 0, 0, time.Local) // 月曜
	var data []*core.Session
	var commits []gitlog.Commit
	for d := 0; d < 70; d += 2 {
		s := cacheSess("s"+time.Duration(d).String(), "Claude Code", []string{"app", "web"}[d%4/2], AddDays(ws, d), 10, 45)
		s.Corrections = d % 3
		data = append(data, s)
		commits = append(commits, gitlog.Commit{Hash: s.ID, T: s.Start + 600, Project: s.Project, Added: d, AI: d%4 == 0})
	}
	span := cacheSess("span", "Claude Code", "app", AddDays(ws, 13), 23.5, 60) // 日曜の 23:30 から 1 時間
	data = append(data, span)
	return data, commits
}

// Cache は、AllWeeks・AllMonths と同じ結果を返し、何も変わっていなければ前回の集計をそのまま返す。
func TestCacheSameAsFresh(t *testing.T) {
	data, commits := cacheData()
	c := NewCache()
	w1, m1 := c.AllWeeks(data, commits...), c.AllMonths(data, commits...)
	if jsonOf(t, w1) != jsonOf(t, AllWeeks(data, commits...)) || jsonOf(t, m1) != jsonOf(t, AllMonths(data, commits...)) {
		t.Fatal("Cache の結果が AllWeeks・AllMonths と違う")
	}
	if len(w1) < 10 || len(m1) < 3 {
		t.Fatalf("週 %d・月 %d", len(w1), len(m1))
	}
	w2, m2 := c.AllWeeks(data, commits...), c.AllMonths(data, commits...)
	for k := range w1 {
		if w1[k] != w2[k] {
			t.Errorf("変わっていない週 %s を集計し直した", k)
		}
	}
	for k := range m1 {
		if m1[k] != m2[k] {
			t.Errorf("変わっていない月 %s を集計し直した", k)
		}
	}
}

// 読み直したセッション（新しいポインタ）やコミットが変わった期間だけを集計し直す。セッションがなくなった期間は消える。
func TestCacheRecomputesOnlyChangedPeriods(t *testing.T) {
	data, commits := cacheData()
	c := NewCache()
	before := c.AllWeeks(data, commits...)
	beforeM := c.AllMonths(data, commits...)

	// 1 つのセッションを読み直した（中身が変わり、ポインタも新しい）
	i := 5
	changed := *data[i]
	changed.Corrections = 7
	data = append(append([]*core.Session{}, data[:i]...), append([]*core.Session{&changed}, data[i+1:]...)...)
	wk := MondayOf(changed.Start).Format("2006-01-02")
	mo := MonthOf(changed.Start).Format("2006-01")
	after, afterM := c.AllWeeks(data, commits...), c.AllMonths(data, commits...)
	if jsonOf(t, after) != jsonOf(t, AllWeeks(data, commits...)) || jsonOf(t, afterM) != jsonOf(t, AllMonths(data, commits...)) {
		t.Fatal("読み直したあとの結果が AllWeeks・AllMonths と違う")
	}
	for k := range after {
		if (k == wk) == (after[k] == before[k]) {
			t.Errorf("週 %s: 集計し直した = %v, want %v", k, after[k] != before[k], k == wk)
		}
	}
	for k := range afterM {
		if (k == mo) == (afterM[k] == beforeM[k]) {
			t.Errorf("月 %s: 集計し直した = %v, want %v", k, afterM[k] != beforeM[k], k == mo)
		}
	}

	// コミットの行数が変わった週だけを集計し直す
	before = after
	commits = append([]gitlog.Commit{}, commits...)
	commits[0].Added += 100
	cw := MondayOf(commits[0].T).Format("2006-01-02")
	after = c.AllWeeks(data, commits...)
	if after[cw] == before[cw] || after[cw].Git.Added != before[cw].Git.Added+100 {
		t.Errorf("コミットが変わった週 %s を集計し直していない", cw)
	}
	for k := range after {
		if k != cw && after[k] != before[k] {
			t.Errorf("コミットの変わっていない週 %s を集計し直した", k)
		}
	}

	// 最初の週のセッションがなくなると、その週は結果から消える
	first := MondayOf(data[0].Start).Format("2006-01-02")
	var rest []*core.Session
	for _, d := range data {
		if MondayOf(d.Start).Format("2006-01-02") != first {
			rest = append(rest, d)
		}
	}
	if len(rest) == len(data) {
		t.Fatal("最初の週のセッションを抜けていない")
	}
	if got := c.AllWeeks(rest, commits...); got[first] != nil || jsonOf(t, got) != jsonOf(t, AllWeeks(rest, commits...)) {
		t.Errorf("セッションがなくなった週 %s が残った", first)
	}
	if _, ok := c.weeks[first]; ok {
		t.Error("なくなった週を覚えたまま")
	}
}

// 1 年分の集計を、全部し直すときと、1 つのセッションだけ読み直したとき（kiroku serve の読み直し）で比べる。
//
//	go test ./internal/report -run '^$' -bench Cache
func BenchmarkCache(b *testing.B) {
	ws := time.Date(2025, 10, 6, 0, 0, 0, 0, time.Local)
	var data []*core.Session
	for d := 0; d < 365; d++ {
		for k := 0; k < 8; k++ {
			s := cacheSess("s", "Claude Code", []string{"app", "web", "docs"}[k%3], AddDays(ws, d), 8+float64(k), 50)
			s.ID = s.ID + time.Duration(d*8+k).String()
			data = append(data, s)
		}
	}
	b.Run("full", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			AllWeeks(data)
			AllMonths(data)
		}
	})
	b.Run("one-session-changed", func(b *testing.B) {
		c := NewCache()
		c.AllWeeks(data)
		c.AllMonths(data)
		for i := 0; i < b.N; i++ {
			last := *data[len(data)-1] // 今いちばん新しいセッションに追記があった
			data[len(data)-1] = &last
			c.AllWeeks(data)
			c.AllMonths(data)
		}
	})
}
