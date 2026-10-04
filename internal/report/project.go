package report

import (
	"math"
	"sort"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
)

// ProjectStat は期間の中の 1 プロジェクトのまとめ:
// 何に（どのセッションに）何時間、どれだけのトークン・目安コスト・クレジットを、どのモデル中心に使い、何が重かったか。
type ProjectStat struct {
	Project   string           `json:"project"`
	Minutes   float64          `json:"minutes"`  // 作業していた時間（同時に動いていたプロジェクトとは按分）
	Sessions  int              `json:"sessions"` // この期間に動いていたセッション
	Prompts   int              `json:"prompts"`
	Tokens    float64          `json:"tokens"`
	Cost      float64          `json:"cost"`
	Credits   float64          `json:"credits"`
	Subagents int              `json:"subagents"`
	Outputs   core.OutputTotal `json:"outputs"` // AI が実行したコミット・PR 作成・変更した行
	Git       GitTotal         `json:"git"`     // 手元の git のコミット
	Models    []ModelShare     `json:"models"`  // トークンの多い順（上位 3）
	Top       []TopSession     `json:"top"`     // 長く動いていた順（上位 3）
	Heavy     *TopSession      `json:"heavy"`   // いちばん重かった（目安コスト、なければクレジット、なければトークン）
}

type ModelShare struct {
	Model  string  `json:"model"`
	Tokens float64 `json:"tokens"`
	Cost   float64 `json:"cost"`
	Turns  int     `json:"turns"` // 応答（Kiro などトークンのないエージェントはクレジットの記録）の数
}

type TopSession struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Source  string  `json:"source"`
	Start   float64 `json:"start"`
	Minutes float64 `json:"minutes"` // この期間の中で動いていた時間
	Tokens  float64 `json:"tokens"`
	Cost    float64 `json:"cost"`
	Credits float64 `json:"credits"`
}

// projectStats はプロジェクトごとのまとめ。minutes は Summarize で按分した作業時間、order はその多い順。
func projectStats(data []*core.Session, ws, we float64, minutes map[string]float64, order []string, commits []gitlog.Commit) []ProjectStat {
	type acc struct {
		st     ProjectStat
		models map[string]*ModelShare
		order  []string
		ses    []TopSession
	}
	by := map[string]*acc{}
	get := func(p string) *acc {
		a := by[p]
		if a == nil {
			a = &acc{st: ProjectStat{Project: p, Minutes: core.Round(minutes[p], 0)}, models: map[string]*ModelShare{}}
			by[p] = a
		}
		return a
	}
	in := func(t *float64, def float64) bool {
		v := def
		if t != nil && *t != 0 {
			v = *t
		}
		return ws <= v && v < we
	}
	for _, d := range data {
		if d.End < ws || d.Start >= we {
			continue
		}
		a := get(d.Project)
		ts := TopSession{ID: d.ID, Title: d.Title, Source: d.Source, Start: d.Start}
		for _, sg := range d.Segs {
			if o := math.Min(sg[1], we) - math.Max(sg[0], ws); o > 0 {
				ts.Minutes += o / 60
			}
		}
		for _, p := range d.Prompts {
			if p.T != nil && *p.T != 0 && ws <= *p.T && *p.T < we {
				a.st.Prompts++
			}
		}
		for _, e := range d.UEv {
			if !in(e.T, d.Start) {
				continue
			}
			tok := e.U.Total()
			ts.Tokens += tok
			name := e.Model
			if name == "" {
				name = "（不明）"
			}
			m := a.models[name]
			if m == nil {
				m = &ModelShare{Model: name}
				a.models[name] = m
				a.order = append(a.order, name)
			}
			m.Tokens += tok
			m.Turns++
			if e.Cost != nil {
				m.Cost += *e.Cost
				ts.Cost += *e.Cost
			}
		}
		for _, c := range d.CEv {
			if in(c.T, d.Start) {
				ts.Credits += c.V
			}
		}
		// トークンの記録がない（Kiro など）セッションは、モデル名だけ数える
		if len(d.UEv) == 0 && ts.Credits > 0 {
			for _, mm := range d.Models {
				name, _ := mm[0].(string)
				if name == "" {
					continue
				}
				m := a.models[name]
				if m == nil {
					m = &ModelShare{Model: name}
					a.models[name] = m
					a.order = append(a.order, name)
				}
				switch n := mm[1].(type) {
				case int:
					m.Turns += n
				case float64:
					m.Turns += int(n)
				}
			}
		}
		for _, sa := range d.Subagents {
			if sa.Start != nil && *sa.Start != 0 && ws <= *sa.Start && *sa.Start < we {
				a.st.Subagents++
			}
		}
		for _, o := range d.OEv {
			if in(o.T, d.Start) {
				a.st.Outputs.Add(o)
			}
		}
		if ts.Minutes == 0 && ts.Tokens == 0 && ts.Credits == 0 {
			continue
		}
		a.st.Sessions++
		a.st.Tokens += ts.Tokens
		a.st.Cost += ts.Cost
		a.st.Credits += ts.Credits
		a.ses = append(a.ses, ts)
	}
	for _, c := range commits {
		if ws <= c.T && c.T < we && by[c.Project] != nil {
			by[c.Project].st.Git.add(c)
		}
	}
	seen := map[string]bool{}
	var names []string
	for _, p := range order {
		if by[p] != nil {
			names = append(names, p)
			seen[p] = true
		}
	}
	var rest []string // 作業時間はないが使用量だけあるプロジェクト（Crew の裏方の処理など）
	for p := range by {
		if !seen[p] {
			rest = append(rest, p)
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		return by[rest[i]].st.Credits+by[rest[i]].st.Cost > by[rest[j]].st.Credits+by[rest[j]].st.Cost
	})
	names = append(names, rest...)

	out := []ProjectStat{}
	for _, p := range names {
		a := by[p]
		if a.st.Sessions == 0 {
			continue
		}
		sort.SliceStable(a.order, func(i, j int) bool {
			x, y := a.models[a.order[i]], a.models[a.order[j]]
			if x.Tokens != y.Tokens {
				return x.Tokens > y.Tokens
			}
			return x.Turns > y.Turns
		})
		for i, n := range a.order {
			if i == 3 {
				break
			}
			m := *a.models[n]
			m.Cost = core.Round(m.Cost, 4)
			a.st.Models = append(a.st.Models, m)
		}
		for i := range a.ses {
			s := &a.ses[i]
			s.Minutes, s.Cost, s.Credits = core.Round(s.Minutes, 0), core.Round(s.Cost, 4), core.Round(s.Credits, 2)
		}
		byTime := append([]TopSession(nil), a.ses...)
		sort.SliceStable(byTime, func(i, j int) bool { return byTime[i].Minutes > byTime[j].Minutes })
		if len(byTime) > 3 {
			byTime = byTime[:3]
		}
		a.st.Top = byTime
		heavy := append([]TopSession(nil), a.ses...)
		sort.SliceStable(heavy, func(i, j int) bool {
			x, y := heavy[i], heavy[j]
			if x.Cost != y.Cost {
				return x.Cost > y.Cost
			}
			if x.Credits != y.Credits {
				return x.Credits > y.Credits
			}
			return x.Tokens > y.Tokens
		})
		if h := heavy[0]; h.Cost > 0 || h.Credits > 0 || h.Tokens > 0 {
			a.st.Heavy = &h
		}
		a.st.Cost, a.st.Credits = core.Round(a.st.Cost, 4), core.Round(a.st.Credits, 2)
		out = append(out, a.st)
	}
	return out
}
