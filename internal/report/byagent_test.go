package report

import (
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
)

// エージェントごとの集計：2 つ以上のエージェントが動いた期間だけ Agents を作り、各エージェントの集計はそのセッションとコミットだけ。
// 1 つだけなら Only にその名前。期間の外にしか記録のないエージェントは数えない。
func TestSummarizeByAgent(t *testing.T) {
	ws := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	we := ws.AddDate(0, 0, 7)
	t0 := float64(ws.Add(10 * time.Hour).Unix())
	cost := 0.5
	a := &core.Session{ID: "a", Source: "Claude Code", Project: "app", Start: t0, End: t0 + 3600,
		Segs: [][3]float64{{t0, t0 + 3600, 1}}, UEv: []core.Event{{T: &t0, U: core.Tokens{In: 100}, Cost: &cost}}}
	b := &core.Session{ID: "b", Source: "Kiro CLI (V2)", Project: "app", Start: t0 + 7200, End: t0 + 9000,
		Segs: [][3]float64{{t0 + 7200, t0 + 9000, 1}}, CEv: []core.Credit{{T: &t0, V: 2}}}
	old := t0 - 86400*14
	c := &core.Session{ID: "c", Source: "Codex", Project: "app", Start: old, End: old + 600, Segs: [][3]float64{{old, old + 600, 1}}}
	cs := []gitlog.Commit{{T: t0 + 60, Project: "app", AI: true, Session: "a", Added: 3}, {T: t0 + 120, Project: "app", Added: 5}}

	w := SummarizeByAgent([]*core.Session{a, b, c}, ws, we, cs...)
	if w == nil || w.Only != "" || len(w.Agents) != 2 {
		t.Fatalf("agents = %v, only = %q", w.Agents, w.Only)
	}
	cc, kr := w.Agents["Claude Code"], w.Agents["Kiro CLI (V2)"]
	if cc == nil || kr == nil {
		t.Fatalf("agents = %v", w.Agents)
	}
	if cc.Usage.Cost != 0.5 || cc.Usage.Credits != 0 || kr.Usage.Credits != 2 || kr.Usage.Cost != 0 {
		t.Errorf("usage: claude %+v, kiro %+v", cc.Usage, kr.Usage)
	}
	if cc.Active+kr.Active != w.Active {
		t.Errorf("active: %d + %d != %d", cc.Active, kr.Active, w.Active)
	}
	// 手でのコミットは、どのエージェントにも入れない
	if cc.Git.Commits != 1 || kr.Git.Commits != 0 || w.Git.Commits != 2 {
		t.Errorf("git: claude %+v, kiro %+v, all %+v", cc.Git, kr.Git, w.Git)
	}
	if cc.Agents != nil || cc.Only != "" {
		t.Errorf("nested agents = %v, only = %q", cc.Agents, cc.Only)
	}

	one := SummarizeByAgent([]*core.Session{a, c}, ws, we)
	if one == nil || one.Only != "Claude Code" || one.Agents != nil {
		t.Errorf("one agent: only = %q, agents = %v", one.Only, one.Agents)
	}
}
