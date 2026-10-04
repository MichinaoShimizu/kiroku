package report

import (
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// ブランチ・エージェントごとの配分：同時に動いていた時間は按分し、トークン・コスト・クレジットは期間の中の記録だけを足す。
func TestShares(t *testing.T) {
	ws := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	t0 := float64(ws.Add(10 * time.Hour).Unix())
	cost, before := 0.5, t0-86400*7
	main, feat := "main", "feat"
	a := &core.Session{ID: "a", Source: "Claude Code", Project: "app", Branch: &main, Start: t0, End: t0 + 3600,
		Segs: [][3]float64{{t0, t0 + 3600, 1}},
		UEv:  []core.Event{{T: &t0, U: core.Tokens{In: 100}, Cost: &cost}, {T: &before, U: core.Tokens{In: 999}}}}
	b := &core.Session{ID: "b", Source: "Kiro CLI", Project: "app", Branch: &feat, Start: t0 + 1800, End: t0 + 5400,
		Segs: [][3]float64{{t0 + 1800, t0 + 5400, 1}}, CEv: []core.Credit{{T: &t0, V: 2}}}
	c := &core.Session{ID: "c", Source: "Kiro CLI", Project: "web", Start: t0, End: t0 + 600, Segs: [][3]float64{{t0, t0 + 600, 1}}}
	w := Summarize([]*core.Session{a, b, c}, ws, ws.AddDate(0, 0, 7))
	if w == nil {
		t.Fatal("summary is nil")
	}
	got := map[string]Share{}
	for _, s := range w.Shares["branch"] {
		got[s.Key] = s
	}
	// a（60 分）は最初の 10 分を c と、後半 30 分を b と分け合う。b（60 分）は前半 30 分を a と分け合う
	if s := got["app · main"]; s.Tokens != 100 || s.Cost != 0.5 || s.Minutes != 40 {
		t.Errorf("app · main = %+v", s)
	}
	if s := got["app · feat"]; s.Credits != 2 || s.Minutes != 45 {
		t.Errorf("app · feat = %+v", s)
	}
	if s := got["web · —"]; s.Minutes != 5 {
		t.Errorf("web · — = %+v", s)
	}
	src := w.Shares["source"]
	if len(src) != 2 || src[0].Key != "Kiro CLI" || src[1].Key != "Claude Code" {
		t.Errorf("source = %+v", src)
	}
}
