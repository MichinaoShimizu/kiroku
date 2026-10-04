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

	none := Summarize([]*core.Session{{ID: "s2", Source: "Kiro IDE (旧)", Project: "app", Start: start, End: start + 60,
		Segs: [][3]float64{{start, start + 60, 1}}, Prompts: []core.Prompt{}}}, ws, ws.AddDate(0, 0, 7))
	if b, _ := json.Marshal(none.Usage); !strings.Contains(string(b), `"unpricedModels":[]`) {
		t.Errorf("料金表にないモデルがなければ空の一覧（null にしない）: %s", b)
	}
}
