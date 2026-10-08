package report

import (
	"reflect"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
)

// sess は ws から off 時間後に始まり min 分動いたセッション。
func sess(id, source, project string, ws time.Time, off, min float64) *core.Session {
	t0 := float64(ws.Unix()) + off*3600
	return &core.Session{ID: id, Source: source, Project: project, Title: id, Start: t0, End: t0 + min*60,
		Segs: [][3]float64{{t0, t0 + min*60, 1}}, Prompts: []core.Prompt{}}
}

// 期間の中の git のコミットだけを、日ごと・合計（AI が実行したもの・追加・削除）に数える。
func TestSummarizeGitCommits(t *testing.T) {
	ws := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local) // 月曜
	// 日付と時刻から作る（夏時間の始まる日は 23 時間しかなく、0 時からの時間では日がずれる）
	at := func(day, h, m int) float64 { return float64(time.Date(2026, 9, 28+day, h, m, 0, 0, time.Local).Unix()) }
	commits := []gitlog.Commit{
		{Hash: "a", T: at(0, 10, 0), Project: "app", Added: 10, Removed: 2, AI: true},
		{Hash: "b", T: at(0, 23, 59), Project: "app", Added: 1},
		{Hash: "c", T: at(2, 9, 0), Project: "web", Removed: 5},
		{Hash: "d", T: at(-1, 23, 59), Project: "app", Added: 100}, // 前の週
		{Hash: "e", T: at(7, 0, 0), Project: "app", Added: 100},    // 次の週の月曜 0 時
	}
	w := Stats([]*core.Session{sess("s1", "Claude Code", "app", ws, 10, 30)}, ws, commits...)
	if w == nil {
		t.Fatal("summary is nil")
	}
	if want := (GitTotal{Commits: 3, AI: 1, Added: 11, Removed: 7}); w.Git != want {
		t.Errorf("Git = %+v, want %+v", w.Git, want)
	}
	var perDay []int
	for _, d := range w.Days {
		perDay = append(perDay, d.Commits)
	}
	if want := []int{2, 0, 1, 0, 0, 0, 0}; !reflect.DeepEqual(perDay, want) {
		t.Errorf("日ごとのコミット = %v, want %v", perDay, want)
	}
}

// 言い直し・中断・多い依頼から点数を付け、高い順に 3 件まで。理由は日本語と英語（単数・複数）で同じ並び。
func TestFriction(t *testing.T) {
	ws := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	calm := sess("calm", "Claude Code", "app", ws, 1, 10)
	one := sess("one", "Claude Code", "app", ws, 2, 10)
	one.Corrections = 1
	mixed := sess("mixed", "Claude Code", "app", ws, 3, 10)
	mixed.Corrections, mixed.Interrupts = 2, 1
	long := sess("long", "Kiro CLI", "web", ws, 4, 10)
	long.NPrompts = 25 // 15 回から数え、5 回ごとに 1 点足す: (25-15)/5+1 = 3
	worst := sess("worst", "Claude Code", "web", ws, 5, 10)
	worst.Interrupts = 4
	before := sess("before", "Claude Code", "app", ws, -2, 10) // 前の週に始まった
	before.Corrections = 9
	w := Stats([]*core.Session{calm, one, mixed, long, worst, before}, ws)
	if w == nil {
		t.Fatal("summary is nil")
	}
	var ids []string
	for _, f := range w.Friction {
		ids = append(ids, f.ID)
	}
	if want := []string{"worst", "mixed", "long"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("Friction = %v, want %v", ids, want)
	}
	byID := map[string]Friction{}
	for _, f := range w.Friction {
		byID[f.ID] = f
	}
	if f := byID["mixed"]; f.Score != 6 || !reflect.DeepEqual(f.Why, []string{"言い直し 2 回", "中断 1 回"}) ||
		!reflect.DeepEqual(f.WhyEn, []string{"2 corrections", "1 interruption"}) {
		t.Errorf("mixed = %+v", f)
	}
	if f := byID["long"]; f.Score != 3 || !reflect.DeepEqual(f.WhyEn, []string{"25 prompts"}) {
		t.Errorf("long = %+v", f)
	}
	// 3 件に入らなくても、1 回の言い直しは単数で書く
	w = Stats([]*core.Session{one}, ws)
	if len(w.Friction) != 1 || !reflect.DeepEqual(w.Friction[0].WhyEn, []string{"1 correction"}) {
		t.Errorf("one = %+v", w.Friction)
	}
}

// エージェント別の参考指標は、期間の中の観測だけをまとめる。時刻のない観測はセッションの開始で数え、
// 観測のないエージェントは出さない。
func TestNativeGroups(t *testing.T) {
	ws := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	we := AddDays(ws, 7)
	in, out := float64(ws.Unix())+3600, float64(we.Unix())+60
	a := sess("a", "Claude Code", "app", ws, 1, 30)
	a.Meas = []core.Measure{{Key: "responses", V: 3}, {Key: "responses", T: &in, V: 2}, {Key: "responses", T: &out, V: 100}}
	b := sess("b", "Claude Code", "app", ws, 2, 30)
	b.Meas = []core.Measure{{Key: "responses", V: 5}, {Key: "context_used", V: 0.42}}
	c := sess("c", "Kiro CLI", "web", ws, 3, 30) // 観測なし
	got := nativeGroups([]*core.Session{a, b, c}, float64(ws.Unix()), float64(we.Unix()))
	if len(got) != 1 || got[0].Source != "Claude Code" || got[0].Sessions != 2 {
		t.Fatalf("nativeGroups = %+v", got)
	}
	vals := map[string]core.NativeValue{}
	for _, v := range got[0].Values {
		vals[v.LabelEn] = v
	}
	if v := vals["Responses"]; v.V != 10 || v.N != 3 {
		t.Errorf("Responses = %+v, want 10 from 3", v)
	}
	if v := vals["Peak context usage"]; v.V != 42 || v.N != 1 {
		t.Errorf("Peak context usage = %+v, want 42 from 1", v)
	}
	if _, ok := vals["Tool calls"]; ok {
		t.Error("観測のない指標を出した")
	}
	// 期間の外の観測しかないセッションは、数にも入れない
	if got := nativeGroups([]*core.Session{a}, float64(we.Unix()), float64(AddDays(we, 7).Unix())); len(got) != 1 || got[0].Sessions != 1 {
		t.Errorf("次の週 = %+v", got)
	}
}

// 週や月の境目をまたぐセッションは、動いていたどちらの期間にも数える。動いていない期間は作らない。
func TestAllWeeksAndMonthsAcrossBoundaries(t *testing.T) {
	sun := time.Date(2026, 9, 27, 23, 30, 0, 0, time.Local) // 日曜。次の月曜は 9/28
	s := &core.Session{ID: "x", Source: "Claude Code", Project: "app", Start: float64(sun.Unix()), End: float64(sun.Add(time.Hour).Unix()),
		Segs: [][3]float64{{float64(sun.Unix()), float64(sun.Add(time.Hour).Unix()), 1}}, Prompts: []core.Prompt{}}
	weeks := AllWeeks([]*core.Session{s})
	if len(weeks) != 2 || weeks["2026-09-21"] == nil || weeks["2026-09-28"] == nil {
		t.Fatalf("AllWeeks のキー = %v", keys(weeks))
	}
	if a, b := weeks["2026-09-21"].Active, weeks["2026-09-28"].Active; a != 30 || b != 30 {
		t.Errorf("Active = %d / %d, want 30 / 30", a, b)
	}
	eom := time.Date(2026, 9, 30, 23, 50, 0, 0, time.Local)
	m := &core.Session{ID: "m", Source: "Claude Code", Project: "app", Start: float64(eom.Unix()), End: float64(eom.Add(20 * time.Minute).Unix()),
		Segs: [][3]float64{{float64(eom.Unix()), float64(eom.Add(20 * time.Minute).Unix()), 1}}, Prompts: []core.Prompt{}}
	months := AllMonths([]*core.Session{m})
	if len(months) != 2 || months["2026-09"].Active != 10 || months["2026-10"].Active != 10 {
		t.Errorf("AllMonths = %v", keys(months))
	}
	if len(months["2026-10"].Days) != 31 || len(months["2026-09"].Days) != 30 {
		t.Errorf("日の数 = %d / %d", len(months["2026-09"].Days), len(months["2026-10"].Days))
	}
}

func keys[V any](m map[string]V) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
