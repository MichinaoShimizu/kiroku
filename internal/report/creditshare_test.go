package report

import (
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// サマリーの Kiro credits のグラフは、期間の shares の source からエージェントごとの credits を積み上げる。
// エージェントごとの credits を足すと期間の credits になり、期間の外にしか記録のないエージェントは入らない。
func TestCreditsBySource(t *testing.T) {
	ws := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	we := ws.AddDate(0, 0, 7)
	t0 := float64(ws.Add(10 * time.Hour).Unix())
	t1 := t0 + 7200
	cost := 0.5
	a := &core.Session{ID: "a", Source: "Claude Code", Project: "app", Start: t0, End: t0 + 3600,
		Segs: [][3]float64{{t0, t0 + 3600, 1}}, UEv: []core.Event{{T: &t0, U: core.Tokens{In: 100}, Cost: &cost}}}
	b := &core.Session{ID: "b", Source: "Kiro CLI (V2)", Project: "app", Start: t1, End: t1 + 1800,
		Segs: [][3]float64{{t1, t1 + 1800, 1}}, CEv: []core.Credit{{T: &t1, V: 2}}}
	t2 := t1 + 3600
	c := &core.Session{ID: "c", Source: "Kiro IDE / CLI (V3)", Project: "app", Start: t2, End: t2 + 600,
		Segs: [][3]float64{{t2, t2 + 600, 1}}, CEv: []core.Credit{{T: &t2, V: 3.5}}}
	old := t0 - 86400*14
	d := &core.Session{ID: "d", Source: "Kiro Crew", Project: "app", Start: old, End: old + 600,
		Segs: [][3]float64{{old, old + 600, 1}}, CEv: []core.Credit{{T: &old, V: 9}}}

	w := Summarize([]*core.Session{a, b, c, d}, ws, we)
	if w == nil {
		t.Fatal("no summary")
	}
	got, sum := map[string]float64{}, 0.0
	for _, x := range w.Shares["source"] {
		got[x.Key] = x.Credits
		sum += x.Credits
	}
	if got["Kiro CLI (V2)"] != 2 || got["Kiro IDE / CLI (V3)"] != 3.5 || got["Claude Code"] != 0 {
		t.Errorf("credits by source = %v", got)
	}
	if _, ok := got["Kiro Crew"]; ok {
		t.Errorf("an agent with no records in the period is listed: %v", got)
	}
	if sum != w.Usage.Credits {
		t.Errorf("credits by source add up to %v, usage has %v", sum, w.Usage.Credits)
	}
}
