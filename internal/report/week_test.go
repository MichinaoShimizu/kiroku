package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// 料金表にないモデルのトークンとそのモデルを、画面に渡す JSON に入れる。
// 以前は json:"-" で落ちていて、画面の「目安コストに入っていません」が出なかった。
func TestWeekUsageUnpriced(t *testing.T) {
	ws := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	start := float64(ws.Add(10 * time.Hour).Unix())
	cost := 0.5
	s := &core.Session{ID: "s1", Source: "Codex", Project: "app", Start: start, End: start + 600,
		Segs: [][3]float64{{start, start + 600, 3}}, Prompts: []core.Prompt{},
		UEv: []core.Event{
			{T: &start, Model: "claude-sonnet-4-5", U: core.Tokens{In: 100}, Cost: &cost},
			{T: &start, Model: "gpt-mini", U: core.Tokens{In: 50}},
			{T: &start, Model: "gpt-big", U: core.Tokens{In: 300}},
			{T: &start, Model: "gpt-mini", U: core.Tokens{In: 100}},
		}}
	w := Summarize([]*core.Session{s}, ws, ws.AddDate(0, 0, 7))
	if w == nil {
		t.Fatal("集計がない")
	}
	if w.Usage.Unpriced != 450 || strings.Join(w.Usage.UnpricedM, ",") != "gpt-big,gpt-mini" {
		t.Errorf("unpriced = %v %v, want 450 [gpt-big gpt-mini]（料金のあるモデルは入れず、トークンの多い順）", w.Usage.Unpriced, w.Usage.UnpricedM)
	}
	b, _ := json.Marshal(w.Usage)
	for _, k := range []string{`"unpriced":450`, `"unpricedModels":["gpt-big","gpt-mini"]`} {
		if !strings.Contains(string(b), k) {
			t.Errorf("JSON に %s がない: %s", k, b)
		}
	}

	none := Summarize([]*core.Session{{ID: "s2", Source: "Kiro IDE (legacy)", Project: "app", Start: start, End: start + 60,
		Segs: [][3]float64{{start, start + 60, 1}}, Prompts: []core.Prompt{}}}, ws, ws.AddDate(0, 0, 7))
	if b, _ := json.Marshal(none.Usage); !strings.Contains(string(b), `"unpricedModels":[]`) {
		t.Errorf("料金表にないモデルがなければ空の一覧（null にしない）: %s", b)
	}
}

// 期間の集計は、期間に関わるセッションだけを選んで行う（span.go）。セッションの区間の外にある記録
// （例: 区間より後に記録された使用量）も、その記録の期間に入る。
func TestPeriodsIncludeRecordsOutsideSessionRange(t *testing.T) {
	w1 := time.Date(2026, 9, 21, 0, 0, 0, 0, time.Local)
	w2 := w1.AddDate(0, 0, 7)
	a0, b0 := float64(w1.Add(34*time.Hour).Unix()), float64(w2.Add(34*time.Hour).Unix())
	late := b0 + 60 // a の使用量だが、時刻は次の週
	cost := 1.0
	a := &core.Session{ID: "a", Source: "Claude Code", Project: "app", Start: a0, End: a0 + 600,
		Segs: [][3]float64{{a0, a0 + 600, 2}}, Prompts: []core.Prompt{},
		UEv: []core.Event{{T: &late, Model: "m", U: core.Tokens{In: 1000}, Cost: &cost}}}
	b := &core.Session{ID: "b", Source: "Claude Code", Project: "app", Start: b0, End: b0 + 600,
		Segs: [][3]float64{{b0, b0 + 600, 2}}, Prompts: []core.Prompt{}}
	weeks := AllWeeks([]*core.Session{a, b})
	w := weeks[w2.Format("2006-01-02")]
	if w == nil {
		t.Fatal("次の週の集計がない")
	}
	if w.Usage.Tokens != 1000 {
		t.Errorf("次の週の使用量 = %v, want 1000（区間の外の記録も、その時刻の週に入れる）", w.Usage.Tokens)
	}
}

// 「コミットまで行ったセッション」の割合と「1 コミットあたりの目安コスト」は、アウトプットを記録できる
// エージェント（Claude Code）のセッションとコストだけで出す。ほかのエージェントが混ざっても変わらない。
func TestOutputsOnlyFromTrackedSessions(t *testing.T) {
	ws := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	start := float64(ws.Add(10 * time.Hour).Unix())
	c1, c2 := 2.0, 30.0
	seg := [][3]float64{{start, start + 600, 2}}
	claude := &core.Session{ID: "c", Source: "Claude Code", Project: "app", Start: start, End: start + 600, Segs: seg, Prompts: []core.Prompt{}, OutTracked: true,
		UEv: []core.Event{{T: &start, Model: "m", U: core.Tokens{In: 1}, Cost: &c1}},
		OEv: []core.Output{{T: &start, Kind: "commit", V: 1}}}
	idle := &core.Session{ID: "i", Source: "Claude Code", Project: "app", Start: start, End: start + 600, Segs: seg, Prompts: []core.Prompt{}, OutTracked: true}
	other := &core.Session{ID: "o", Source: "Codex", Project: "app", Start: start, End: start + 600, Segs: seg, Prompts: []core.Prompt{},
		UEv: []core.Event{{T: &start, Model: "m", U: core.Tokens{In: 1}, Cost: &c2}}}
	w := Summarize([]*core.Session{claude, idle, other}, ws, ws.AddDate(0, 0, 7))
	if w.Sessions != 3 || w.OutBase != 2 || w.OutSessions != 1 {
		t.Errorf("sessions/outBase/outSessions = %d %d %d, want 3 2 1", w.Sessions, w.OutBase, w.OutSessions)
	}
	if w.CostPerCommit == nil || *w.CostPerCommit != 2 {
		t.Errorf("1 コミットあたり = %v, want 2（Codex のコストは入れない）", w.CostPerCommit)
	}
}

// 0 時に夏時間が始まる地域（America/Santiago は 2026-09-06 の 0 時が 1 時になる）でも、日の境目は日付どおり。
// 以前は前の日の境目に 1 日足していたので、そのあとの境目が 23 時にずれ、9 月が 31 日になっていた。
func TestDaysWhenMidnightIsSkipped(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Skip("タイムゾーンのデータがない:", err)
	}
	saved := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = saved })

	if d := Midnight(2026, 9, 6, loc); d.Day() != 6 || d.Hour() != 1 {
		t.Errorf("0 時のない日の始まり = %v, want 9/6 1:00", d)
	}
	if d := AddDays(Midnight(2026, 9, 5, loc), 2); d.Day() != 7 || d.Hour() != 0 {
		t.Errorf("0 時のない日をまたいだ 2 日後 = %v, want 9/7 0:00", d)
	}
	at := func(m time.Month, d, h, min int) float64 {
		return float64(time.Date(2026, m, d, h, min, 0, 0, loc).Unix())
	}
	session := func(id string, start float64) *core.Session {
		return &core.Session{ID: id, Source: "Claude Code", Project: "app", Start: start, End: start + 1800,
			Segs: [][3]float64{{start, start + 1800, 1}}, Prompts: []core.Prompt{}}
	}
	data := []*core.Session{session("late", at(9, 15, 23, 30)), session("sun", at(9, 6, 10, 0))}

	m := AllMonths(data)["2026-09"]
	if m == nil {
		t.Fatal("9 月の集計がない")
	}
	if len(m.Days) != 30 {
		t.Fatalf("9 月の日数 = %d, want 30", len(m.Days))
	}
	if m.Days[14].Active != 30 || m.Days[15].Active != 0 {
		t.Errorf("9/15 23:30 の作業 = 15 日 %d 分、16 日 %d 分, want 30 と 0", m.Days[14].Active, m.Days[15].Active)
	}
	w := AllWeeks(data)["2026-08-31"]
	if w == nil || len(w.Days) != 7 || w.Days[6].Active != 30 {
		t.Errorf("8/31 の週 = %+v, want 7 日で日曜（9/6）に 30 分", w)
	}
	// America/Asuncion は 2023-10-01 の 0 時が 1 時になる。月の始まりが前の月の 23 時になると、キーが 9 月になってしまう
	if asu, err := time.LoadLocation("America/Asuncion"); err == nil {
		time.Local = asu
		if k := MonthOf(float64(time.Date(2023, 10, 10, 12, 0, 0, 0, asu).Unix())).Format("2006-01"); k != "2023-10" {
			t.Errorf("1 日に 0 時がない月の MonthOf = %s, want 2023-10", k)
		}
	}
}

// 料金表に同じ ID がなく、先頭一致だけで料金を当てたモデルは、当てたキーと一緒に JSON に入れる（画面の「Data sources」で知らせる）。
func TestWeekUsagePrefixPriced(t *testing.T) {
	ws := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	start := float64(ws.Add(10 * time.Hour).Unix())
	cost := 0.5
	s := &core.Session{ID: "s1", Source: "Claude Code", Project: "app", Start: start, End: start + 600,
		Segs: [][3]float64{{start, start + 600, 3}}, Prompts: []core.Prompt{},
		UEv: []core.Event{
			{T: &start, Model: "claude-opus-5-5", U: core.Tokens{In: 100}, Cost: &cost},
			{T: &start, Model: "claude-opus-5-6", U: core.Tokens{In: 100}, Cost: &cost},
			{T: &start, Model: "us.anthropic.claude-sonnet-4-5-20250929-v1:0", U: core.Tokens{In: 100}, Cost: &cost},
			{T: &start, Model: "gpt-x", U: core.Tokens{In: 50}},
		}}
	w := Summarize([]*core.Session{s}, ws, ws.AddDate(0, 0, 7))
	if len(w.Usage.PrefixPriced) != 1 || w.Usage.PrefixPriced[0] != [2]string{"claude-opus-5-6", "claude-opus-5"} {
		t.Errorf("prefixPriced = %v, want [[claude-opus-5-6 claude-opus-5]]", w.Usage.PrefixPriced)
	}
	b, _ := json.Marshal(w.Usage)
	if !strings.Contains(string(b), `"prefixPriced":[["claude-opus-5-6","claude-opus-5"]]`) {
		t.Errorf("JSON に prefixPriced がない: %s", b)
	}
}
