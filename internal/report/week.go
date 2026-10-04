// Package report は週ごと・月ごとの集計（サマリー）と、プロジェクト別のまとめ・配分（share.go）を作る。
// 時刻はすべてこのマシンのローカル時刻で数える。分単位で「誰が動いていたか」を並べて集計する。
package report

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
)

const (
	FocusMin    = 60 // これ以上続いたら「集中ブロック」
	FocusBridge = 5  // この分数までの切れ目はつながっているとみなす
	NightFrom   = 22 // 深夜の時間帯
	NightTo     = 6
)

type Block struct {
	T       float64 `json:"t"`
	Min     int     `json:"min"`
	Project string  `json:"project"`
	Share   float64 `json:"share"`
}

type Day struct {
	Active   int     `json:"active"`
	Night    int     `json:"night"`
	Switches int     `json:"switches"`
	Prompts  int     `json:"prompts"`
	Tokens   float64 `json:"tokens"`  // その日のトークン（入力・出力・キャッシュの合計）
	Cost     float64 `json:"cost"`    // その日の目安コスト（API 換算）
	Credits  float64 `json:"credits"` // その日の Kiro クレジット
	Commits  int     `json:"commits"` // その日の git のコミット（手元のリポジトリ）
}

type Friction struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Project string   `json:"project"`
	Start   float64  `json:"start"`
	Score   int      `json:"score"`
	Why     []string `json:"why"`
	WhyEn   []string `json:"whyEn"` // 英語表示のときの理由（Why と同じ順）
}

type Heavy struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Project   string  `json:"project"`
	Start     float64 `json:"start"`
	Cost      float64 `json:"cost"`
	Subagents int     `json:"subagents"`
}

type WeekUsage struct {
	Tokens    float64  `json:"tokens"`
	Out       float64  `json:"out"`
	Cost      float64  `json:"cost"`
	Credits   float64  `json:"credits"`
	CacheHit  *float64 `json:"cacheHit"`
	Models    [][4]any `json:"models"`   // [model, cost, tokens, msgs]
	Projects  [][2]any `json:"projects"` // [project, cost]
	Subagents int      `json:"subagents"`
	SubMin    float64  `json:"subMin"`
	SubTypes  [][2]any `json:"subTypes"`
	Heavy     []Heavy  `json:"heavy"`
	Unpriced  float64  `json:"unpriced"`       // 料金表にないモデルのトークン（目安コストに入らない）
	UnpricedM []string `json:"unpricedModels"` // そのモデル（トークンの多い順）
}

// Summary は 1 期間（週か月）の集計。
type Summary struct {
	Usage         WeekUsage          `json:"usage"`
	Start         string             `json:"start"`         // 期間の最初の日（YYYY-MM-DD）
	FixRate       *float64           `json:"fixRate"`       // 言い直し・中断のあった依頼の割合（%）。依頼がなければ nil
	CostPerAsk    *float64           `json:"costPerAsk"`    // 1 依頼あたりの目安コスト。トークンの記録がなければ nil
	Outputs       core.OutputTotal   `json:"outputs"`       // AI が実行したコミット・PR 作成・変更した行（成果の代理）
	OutSessions   int                `json:"outSessions"`   // コミットか PR 作成まで行ったセッションの数
	CostPerCommit *float64           `json:"costPerCommit"` // 1 コミットあたりの目安コスト。コミットかトークンの記録がなければ nil
	Git           GitTotal           `json:"git"`           // 手元の git リポジトリのコミット（gitlog）
	Sessions      int                `json:"sessions"`
	Prompts       int                `json:"prompts"`
	Active        int                `json:"active"`
	AI            int                `json:"ai"`
	Parallel      int                `json:"parallel"`
	MaxConc       int                `json:"maxConc"`
	Night         int                `json:"night"`
	Weekend       int                `json:"weekend"`
	Focus         []Block            `json:"focus"`
	SwitchesAvg   float64            `json:"switchesAvg"`
	SwitchesMax   int                `json:"switchesMax"`
	WaitMedian    *float64           `json:"waitMedian"`
	WaitP90       *float64           `json:"waitP90"`
	WaitCount     int                `json:"waitCount"`
	Projects      [][2]any           `json:"projects"`
	ProjectStats  []ProjectStat      `json:"projectStats"` // プロジェクト別のまとめ（project.go）
	Shares        map[string][]Share `json:"shares"`       // ブランチ・エージェントごとの配分（share.go）
	Days          []Day              `json:"days"`         // 期間の日ごと（週なら 7、月なら 28〜31）
	Friction      []Friction         `json:"friction"`
	Native        []NativeGroup      `json:"native"` // エージェント別の参考指標
	start, end    time.Time
}

// Week は 1 週間ぶんの Summary（互換のための別名）。
type Week = Summary

// MondayOf はその時刻を含む週の月曜 0 時（ローカル時刻）。
func MondayOf(ts float64) time.Time {
	d := time.Unix(0, int64(ts*1e9)).In(time.Local)
	wd := (int(d.Weekday()) + 6) % 7
	return time.Date(d.Year(), d.Month(), d.Day()-wd, 0, 0, 0, 0, time.Local)
}

func unix(t time.Time) float64 { return float64(t.UnixNano()) / 1e9 }

type sp struct{ sid, project string }

func distinctProjects(xs []sp) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x.project] {
			seen[x.project] = true
			out = append(out, x.project)
		}
	}
	return out
}

func distinctSessions(xs []sp) int {
	seen := map[string]bool{}
	for _, x := range xs {
		seen[x.sid] = true
	}
	return len(seen)
}

func fptr(v float64) *float64 { return &v }

// MonthOf はその時刻を含む月の 1 日 0 時（ローカル時刻）。
func MonthOf(ts float64) time.Time {
	d := time.Unix(0, int64(ts*1e9)).In(time.Local)
	return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.Local)
}

// Stats は 1 週ぶんの集計。動いていた時間がなければ nil。
func Stats(data []*core.Session, wsT time.Time, commits ...gitlog.Commit) *Week {
	return Summarize(data, wsT, wsT.AddDate(0, 0, 7), commits...)
}

// GitTotal は期間の git のコミットのまとめ。
type GitTotal struct {
	Commits int `json:"commits"`
	AI      int `json:"ai"` // うちエージェントが実行したもの
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

func (g *GitTotal) add(c gitlog.Commit) {
	g.Commits++
	if c.AI {
		g.AI++
	}
	g.Added += c.Added
	g.Removed += c.Removed
}

// Summarize は [from, to) の集計。from と to はローカル時刻の 0 時。動いていた時間がなければ nil。
// commits は手元の git のコミット（なくてもよい）。
func Summarize(data []*core.Session, wsT, weT time.Time, commits ...gitlog.Commit) *Summary {
	ws, we := unix(wsT), unix(weT)
	n := int(math.Floor((we - ws) / 60))
	// 日の境目（夏時間でも日付どおりに分ける）
	var dayStart []float64
	var dayWeekend []bool
	for d := wsT; d.Before(weT); d = d.AddDate(0, 0, 1) {
		dayStart = append(dayStart, unix(d))
		dayWeekend = append(dayWeekend, d.Weekday() == time.Saturday || d.Weekday() == time.Sunday)
	}
	nd := len(dayStart)
	mins := make([][]sp, n)
	ai := 0
	for _, d := range data {
		if d.End < ws || d.Start >= we {
			continue
		}
		for _, sg := range d.Segs {
			m0 := int(math.Max(0, math.Floor((sg[0]-ws)/60)))
			m1 := int(math.Min(float64(n), math.Ceil((sg[1]-ws)/60)))
			if m1 > m0 {
				ai += m1 - m0
			}
			for m := m0; m < m1; m++ {
				mins[m] = append(mins[m], sp{d.ID, d.Project})
			}
		}
	}
	var active []int
	for m := 0; m < n; m++ {
		if len(mins[m]) > 0 {
			active = append(active, m)
		}
	}
	if len(active) == 0 {
		return nil
	}
	projects := map[string]float64{}
	var projOrder []string
	parallel, maxConc := 0, 0
	dayOf := func(m int) int {
		t := ws + float64(m)*60
		return max(0, sort.Search(nd, func(i int) bool { return dayStart[i] > t })-1)
	}
	isNight := func(m int) bool {
		h := int((ws + float64(m)*60 - dayStart[dayOf(m)]) / 3600)
		return h >= NightFrom || h < NightTo
	}
	night, weekend := 0, 0
	for _, m := range active {
		names := distinctProjects(mins[m])
		for _, p := range names {
			if _, ok := projects[p]; !ok {
				projOrder = append(projOrder, p)
			}
			projects[p] += 1 / float64(len(names))
		}
		c := distinctSessions(mins[m])
		if c >= 2 {
			parallel++
		}
		maxConc = max(maxConc, c)
		if isNight(m) {
			night++
		}
		if dayWeekend[dayOf(m)] {
			weekend++
		}
	}
	// 集中ブロック: 途切れ（FocusBridge 分まで）を許して続いた作業
	blocks := []Block{}
	start, last := active[0], active[0]
	flush := func() {
		if last-start+1 < FocusMin {
			return
		}
		share := map[string]int{}
		var order []string
		for k := start; k <= last; k++ {
			for _, p := range distinctProjects(mins[k]) {
				if _, ok := share[p]; !ok {
					order = append(order, p)
				}
				share[p]++
			}
		}
		top := order[0]
		for _, p := range order {
			if share[p] > share[top] {
				top = p
			}
		}
		blocks = append(blocks, Block{T: ws + float64(start)*60, Min: last - start + 1, Project: top,
			Share: core.Round(float64(share[top])/float64(last-start+1), 2)})
	}
	for _, m := range active[1:] {
		if m-last <= FocusBridge+1 {
			last = m
			continue
		}
		flush()
		start, last = m, m
	}
	flush()

	// 切り替え: 依頼を出した順に並べて、前の依頼とプロジェクトが変わった回数（日ごと）
	type tp struct {
		t float64
		p string
	}
	var prompts []tp
	for _, d := range data {
		for _, p := range d.Prompts {
			if p.T != nil && *p.T != 0 && ws <= *p.T && *p.T < we {
				prompts = append(prompts, tp{*p.T, d.Project})
			}
		}
	}
	sort.Slice(prompts, func(i, j int) bool {
		if prompts[i].t != prompts[j].t {
			return prompts[i].t < prompts[j].t
		}
		return prompts[i].p < prompts[j].p
	})
	days := make([]Day, nd)
	for _, m := range active {
		days[dayOf(m)].Active++
		if isNight(m) {
			days[dayOf(m)].Night++
		}
	}
	prevDay, prevProj := -1, ""
	for _, x := range prompts {
		di := dayOf(int(math.Floor((x.t - ws) / 60)))
		days[di].Prompts++
		if prevDay == di && prevProj != x.p {
			days[di].Switches++
		}
		prevDay, prevProj = di, x.p
	}
	// 日ごとの使用量（トークン・目安コスト・クレジット）は、記録された時刻の日に入れる
	dayAt := func(t float64) int { return max(0, sort.Search(nd, func(i int) bool { return dayStart[i] > t })-1) }
	for _, d := range data {
		for _, e := range d.UEv {
			t := d.Start
			if e.T != nil && *e.T != 0 {
				t = *e.T
			}
			if ws <= t && t < we {
				x := &days[dayAt(t)]
				x.Tokens += e.U.Total()
				if e.Cost != nil {
					x.Cost += *e.Cost
				}
			}
		}
		for _, c := range d.CEv {
			t := d.Start
			if c.T != nil && *c.T != 0 {
				t = *c.T
			}
			if ws <= t && t < we {
				days[dayAt(t)].Credits += c.V
			}
		}
	}
	var gitTot GitTotal
	for _, c := range commits {
		if ws <= c.T && c.T < we {
			days[dayAt(c.T)].Commits++
			gitTot.add(c)
		}
	}
	for i := range days {
		days[i].Cost, days[i].Credits = core.Round(days[i].Cost, 4), core.Round(days[i].Credits, 2)
	}
	activeDays, swSum, swMax := 0, 0, 0
	for _, d := range days {
		if d.Active > 0 {
			activeDays++
			swSum += d.Switches
		}
		swMax = max(swMax, d.Switches)
	}

	var waits []float64
	for _, d := range data {
		for _, w := range d.Waits {
			if ws <= w[0] && w[0] < we {
				waits = append(waits, w[1])
			}
		}
	}
	sort.Float64s(waits)
	pick := func(q float64) *float64 {
		if len(waits) == 0 {
			return nil
		}
		return fptr(waits[min(len(waits)-1, int(float64(len(waits))*q))])
	}

	friction := []Friction{}
	for _, d := range data {
		if !(ws <= d.Start && d.Start < we) {
			continue
		}
		why, whyEn, score := []string{}, []string{}, 0
		if d.Corrections > 0 {
			why = append(why, fmt.Sprintf("言い直し %d 回", d.Corrections))
			whyEn = append(whyEn, plural(d.Corrections, "correction"))
			score += 2 * d.Corrections
		}
		if d.Interrupts > 0 {
			why = append(why, fmt.Sprintf("中断 %d 回", d.Interrupts))
			whyEn = append(whyEn, plural(d.Interrupts, "interruption"))
			score += 2 * d.Interrupts
		}
		if d.NPrompts >= 15 {
			why = append(why, fmt.Sprintf("依頼 %d 回", d.NPrompts))
			whyEn = append(whyEn, plural(d.NPrompts, "prompt"))
			score += (d.NPrompts-15)/5 + 1
		}
		if score > 0 {
			friction = append(friction, Friction{d.ID, d.Title, d.Project, d.Start, score, why, whyEn})
		}
	}
	sort.SliceStable(friction, func(i, j int) bool { return friction[i].Score > friction[j].Score })
	if len(friction) > 3 {
		friction = friction[:3]
	}

	usage := weekUsage(data, ws, we)

	sessions := 0
	for _, d := range data {
		if d.End >= ws && d.Start < we {
			sessions++
		}
	}
	projList := [][2]any{}
	sort.SliceStable(projOrder, func(i, j int) bool { return projects[projOrder[i]] > projects[projOrder[j]] })
	for _, p := range projOrder {
		projList = append(projList, [2]any{p, core.Round(projects[p], 0)})
	}

	fixes := 0
	for _, d := range data {
		for _, t := range d.Fix {
			if ws <= t && t < we {
				fixes++
			}
		}
	}
	np := len(prompts)
	var fixRate, costPer *float64
	if np > 0 {
		fixRate = fptr(core.Round(float64(fixes)*100/float64(np), 1))
		if usage.Tokens > 0 {
			costPer = fptr(core.Round(usage.Cost/float64(np), 3))
		}
	}
	var outs core.OutputTotal
	outSes := 0
	for _, d := range data {
		shipped := false
		for _, o := range d.OEv {
			t := d.Start
			if o.T != nil && *o.T != 0 {
				t = *o.T
			}
			if ws <= t && t < we {
				outs.Add(o)
				shipped = shipped || o.Kind == "commit" || o.Kind == "pr"
			}
		}
		if shipped {
			outSes++
		}
	}
	var costPerCommit *float64
	if outs.Commits > 0 && usage.Tokens > 0 {
		costPerCommit = fptr(core.Round(usage.Cost/outs.Commits, 2))
	}
	return &Summary{
		Outputs: outs, OutSessions: outSes, CostPerCommit: costPerCommit, Git: gitTot,
		Native:       nativeGroups(data, ws, we),
		ProjectStats: projectStats(data, ws, we, projects, projOrder, commits),
		Shares:       map[string][]Share{"branch": shares(data, ws, we, mins, ShareKeys["branch"]), "source": shares(data, ws, we, mins, ShareKeys["source"])},
		FixRate:      fixRate, CostPerAsk: costPer, Usage: usage, Start: wsT.Format("2006-01-02"), Sessions: sessions, Prompts: np,
		Active: len(active), AI: ai, Parallel: parallel, MaxConc: maxConc, Night: night, Weekend: weekend,
		Focus: blocks, SwitchesAvg: core.Round(float64(swSum)/float64(activeDays), 1), SwitchesMax: swMax,
		WaitMedian: pick(0.5), WaitP90: pick(0.9), WaitCount: len(waits), Projects: projList, Days: days,
		Friction: friction, start: wsT, end: weT,
	}
}

// NativeGroup は 1 つのエージェントの参考指標。
type NativeGroup struct {
	Source   string             `json:"source"`
	Sessions int                `json:"sessions"`
	Values   []core.NativeValue `json:"values"`
}

// nativeGroups はその週の観測をエージェントごとにまとめる。観測のないエージェントは出さない。
func nativeGroups(data []*core.Session, ws, we float64) []NativeGroup {
	by := map[string][]core.Measure{}
	sess := map[string]int{}
	var order []string
	for _, d := range data {
		used := false
		for _, m := range d.Meas {
			t := d.Start
			if m.T != nil && *m.T != 0 {
				t = *m.T
			}
			if ws <= t && t < we {
				if _, ok := by[d.Source]; !ok {
					order = append(order, d.Source)
				}
				by[d.Source] = append(by[d.Source], m)
				used = true
			}
		}
		if used {
			sess[d.Source]++
		}
	}
	out := []NativeGroup{}
	for _, src := range order {
		if vs := core.AggregateNative(src, by[src]); len(vs) > 0 {
			out = append(out, NativeGroup{Source: src, Sessions: sess[src], Values: vs})
		}
	}
	return out
}

// weekUsage は AI の使い方: トークン・目安コスト・クレジット・モデル・サブエージェント。
func weekUsage(data []*core.Session, ws, we float64) WeekUsage {
	type mm struct{ tokens, cost, msgs, unpriced float64 }
	models := map[string]*mm{}
	var modelOrder []string
	byProj := map[string]float64{}
	var projOrder []string
	heavy := map[string]float64{}
	var tot core.Tokens
	cost, credits, unpriced := 0.0, 0.0, 0.0
	addProj := func(p string, v float64) {
		if _, ok := byProj[p]; !ok {
			projOrder = append(projOrder, p)
		}
		byProj[p] += v
	}
	for _, d := range data {
		for _, e := range d.UEv {
			t := d.Start
			if e.T != nil && *e.T != 0 {
				t = *e.T
			}
			if !(ws <= t && t < we) {
				continue
			}
			tot.In += e.U.In
			tot.Out += e.U.Out
			tot.CW += e.U.CW
			tot.CW1h += e.U.CW1h
			tot.CR += e.U.CR
			name := e.Model
			if name == "" {
				name = "（不明）"
			}
			if models[name] == nil {
				models[name] = &mm{}
				modelOrder = append(modelOrder, name)
			}
			c := 0.0
			if e.Cost != nil {
				c = *e.Cost
			} else {
				unpriced += e.U.Total()
				models[name].unpriced += e.U.Total()
			}
			models[name].tokens += e.U.Total()
			models[name].msgs++
			models[name].cost += c
			cost += c
			addProj(d.Project, c)
			heavy[d.ID] += c
		}
		for _, c := range d.CEv {
			t := d.Start
			if c.T != nil && *c.T != 0 {
				t = *c.T
			}
			if ws <= t && t < we {
				credits += c.V
				addProj(d.Project, 0)
			}
		}
	}
	subTypes := map[string]int{}
	var typeOrder []string
	nSubs, subSec := 0, 0.0
	for _, d := range data {
		for _, a := range d.Subagents {
			if a.Start == nil || *a.Start == 0 || !(ws <= *a.Start && *a.Start < we) {
				continue
			}
			nSubs++
			if _, ok := subTypes[a.Type]; !ok {
				typeOrder = append(typeOrder, a.Type)
			}
			subTypes[a.Type]++
			end := *a.Start
			if a.End != nil && *a.End != 0 {
				end = *a.End
			}
			subSec += math.Max(0, end-*a.Start)
		}
	}
	reads := tot.CR + tot.CW + tot.CW1h + tot.In
	var cacheHit *float64
	if reads > 0 {
		cacheHit = fptr(core.Round(tot.CR/reads, 3))
	}
	mrows := [][4]any{}
	sort.SliceStable(modelOrder, func(i, j int) bool {
		a, b := models[modelOrder[i]], models[modelOrder[j]]
		ca, cb := core.Round(a.cost, 2), core.Round(b.cost, 2)
		if ca != cb {
			return ca > cb
		}
		return a.tokens > b.tokens
	})
	for _, m := range modelOrder {
		v := models[m]
		mrows = append(mrows, [4]any{m, core.Round(v.cost, 2), v.tokens, v.msgs})
	}
	unpricedM := []string{}
	for _, m := range modelOrder {
		if models[m].unpriced > 0 {
			unpricedM = append(unpricedM, m)
		}
	}
	sort.SliceStable(unpricedM, func(i, j int) bool { return models[unpricedM[i]].unpriced > models[unpricedM[j]].unpriced })
	prows := [][2]any{}
	sort.SliceStable(projOrder, func(i, j int) bool { return byProj[projOrder[i]] > byProj[projOrder[j]] })
	for _, p := range projOrder {
		if byProj[p] != 0 {
			prows = append(prows, [2]any{p, core.Round(byProj[p], 2)})
		}
	}
	trows := [][2]any{}
	sort.SliceStable(typeOrder, func(i, j int) bool { return subTypes[typeOrder[i]] > subTypes[typeOrder[j]] })
	for _, t := range typeOrder {
		trows = append(trows, [2]any{t, subTypes[t]})
	}
	var hs []*core.Session
	for _, d := range data {
		if heavy[d.ID] != 0 {
			hs = append(hs, d)
		}
	}
	sort.SliceStable(hs, func(i, j int) bool { return heavy[hs[i].ID] > heavy[hs[j].ID] })
	heavyRows := []Heavy{}
	for i, d := range hs {
		if i == 3 {
			break
		}
		heavyRows = append(heavyRows, Heavy{d.ID, d.Title, d.Project, d.Start, core.Round(heavy[d.ID], 2), len(d.Subagents)})
	}
	return WeekUsage{Tokens: tot.Total(), Out: tot.Out, Cost: core.Round(cost, 2), Credits: core.Round(credits, 2),
		CacheHit: cacheHit, Models: mrows, Projects: prows, Subagents: nSubs, SubMin: core.Round(subSec/60, 0),
		SubTypes: trows, Heavy: heavyRows, Unpriced: unpriced, UnpricedM: unpricedM}
}

// AllWeeks は記録のあるすべての週を集計する。キーは月曜の日付。
func AllWeeks(data []*core.Session, commits ...gitlog.Commit) map[string]*Week {
	out := map[string]*Week{}
	seen := map[string]bool{}
	ps := newPeriods(data)
	for _, d := range data {
		w := MondayOf(d.Start)
		for unix(w) <= d.End {
			k := w.Format("2006-01-02")
			if !seen[k] {
				seen[k] = true
				if st := Stats(ps.within(unix(w), unix(w.AddDate(0, 0, 7))), w, commits...); st != nil {
					out[k] = st
				}
			}
			w = w.AddDate(0, 0, 7)
		}
	}
	return out
}

// AllMonths は記録のあるすべての月を集計する。キーは YYYY-MM。
func AllMonths(data []*core.Session, commits ...gitlog.Commit) map[string]*Summary {
	out := map[string]*Summary{}
	seen := map[string]bool{}
	ps := newPeriods(data)
	for _, d := range data {
		m := MonthOf(d.Start)
		for unix(m) <= d.End {
			k := m.Format("2006-01")
			if !seen[k] {
				seen[k] = true
				if st := Summarize(ps.within(unix(m), unix(m.AddDate(0, 1, 0))), m, m.AddDate(0, 1, 0), commits...); st != nil {
					out[k] = st
				}
			}
			m = m.AddDate(0, 1, 0)
		}
	}
	return out
}

// plural は英語表示のための「1 correction」「2 corrections」。
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
