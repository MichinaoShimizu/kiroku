package core

import (
	"encoding/json"
	"math"
	"testing"
)

// ウェブ検索は 1,000 回あたり $10。同じ応答の行は最大値でまとめ、fast モードなどの倍率はトークンの料金にだけ掛ける。
func TestWebSearchCost(t *testing.T) {
	ws := func(n float64) map[string]any {
		return map[string]any{"input_tokens": 1e6, "server_tool_use": map[string]any{"web_search_requests": n}}
	}
	if got := ReadUsage(ws(3)); got.WebSearches != 3 || got.Total() != 1e6 {
		t.Errorf("ReadUsage = %+v, want 3 回（トークンの合計には入れない）", got)
	}
	if got := ReadUsage(ws(-5)); got.WebSearches != 0 {
		t.Errorf("負の回数は 0 にする: %+v", got)
	}
	if c, ok := CostOf("claude-opus-5-5", Tokens{In: 1e6, WebSearches: 100}); !ok || math.Abs(c-5) > 1e-9 {
		t.Errorf("CostOf = %v, want $4 + 100 回 × $0.01 = $5", c)
	}
	if _, ok := CostOf("gpt-x", Tokens{WebSearches: 100}); ok {
		t.Error("料金表にないモデルは ok=false のまま")
	}
	u := NewUsage()
	ts := 1.0
	u.Add("m1", &ts, "claude-opus-5-5", ws(2))
	u.Add("m1", &ts, "claude-opus-5-5", ws(2)) // 同じ応答の別の行（二重に数えない）
	u.Add("m2", &ts, "claude-opus-5-5", map[string]any{"input_tokens": 1e6, "speed": "fast", "server_tool_use": map[string]any{"web_search_requests": 10.0}})
	evs := u.Events()
	if len(evs) != 2 || math.Abs(*evs[0].Cost-4.02) > 1e-9 || math.Abs(*evs[1].Cost-8.1) > 1e-9 {
		t.Errorf("コスト = %v・%v, want 4.02・8.1（倍率は検索に掛けない）", *evs[0].Cost, *evs[1].Cost)
	}
	if s := SumUsage(evs); s.WebSearches != 12 || math.Abs(s.Cost-12.12) > 1e-9 {
		t.Errorf("SumUsage = %+v, want 12 回・$12.12", s)
	}
}

// Claude Code 自身の記録（cost-state）を使うときは、その合計にウェブ検索も入っているので、検索の料金を足さない。
func TestWebSearchNotAddedToReported(t *testing.T) {
	u := NewUsage()
	ts := 100.0
	u.Add("m1", &ts, "claude-opus-5-5", map[string]any{"input_tokens": 1e6, "server_tool_use": map[string]any{"web_search_requests": 100.0}})
	evs := u.Events()
	rs := []ReportedCost{{From: 50, To: 500, Models: map[string]ReportedModel{"claude-opus-5-5": {Cost: 7}}}}
	if _, used := applyReported(rs, evs); !used || math.Abs(*evs[0].Cost-7) > 1e-9 {
		t.Errorf("コスト = %v, want 記録どおりの 7（検索の $1 を重ねない）", *evs[0].Cost)
	}
}

// advisorUsage は advisor ツールを使った応答の usage（公式の例と同じ形）。上の項目は executor の 2 回の呼び出しの和で、
// advisor の呼び出し（advisor_message）の分は入らない。
func advisorUsage(t *testing.T, advisorModel string) any {
	model := ""
	if advisorModel != "" {
		model = `"model":"` + advisorModel + `",`
	}
	var v any
	err := json.Unmarshal([]byte(`{"input_tokens":1760,"cache_read_input_tokens":412,"cache_creation_input_tokens":0,"output_tokens":531,"iterations":[
		{"type":"message","input_tokens":412,"cache_read_input_tokens":0,"cache_creation_input_tokens":0,"output_tokens":89},
		{"type":"advisor_message",`+model+`"input_tokens":823,"cache_read_input_tokens":0,"cache_creation_input_tokens":0,"output_tokens":1612},
		{"type":"message","input_tokens":1348,"cache_read_input_tokens":412,"cache_creation_input_tokens":0,"output_tokens":442}]}`), &v)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// advisor ツールの呼び出し（usage.iterations の advisor_message）は、advisor のモデルの応答として、そのモデルの料金で数える。
// 同じ応答の何行にも同じ iterations が書かれても 1 回だけ。呼び出しにモデルがなければ、行の advisorModel を使う。
func TestUsageAdvisorIterations(t *testing.T) {
	ts := 1.0
	u := NewUsage()
	u.AddAdvised("m1", &ts, "claude-sonnet-5-5", "claude-opus-5-5", advisorUsage(t, "claude-opus-5"))
	u.AddAdvised("m1", &ts, "claude-sonnet-5-5", "claude-opus-5-5", advisorUsage(t, "claude-opus-5"))
	u.AddAdvised("m2", &ts, "claude-sonnet-5-5", "claude-opus-5-5", advisorUsage(t, ""))
	evs := u.Events()
	if len(evs) != 4 {
		t.Fatalf("events = %+v, want executor 2 つ・advisor 2 つ", evs)
	}
	exec, adv := evs[0], evs[1]
	if exec.Advisor || exec.Model != "claude-sonnet-5-5" || exec.U.In != 1760 || exec.U.Out != 531 || exec.U.CR != 412 {
		t.Errorf("executor = %+v, want 上の項目（executor の呼び出しの和）のまま", exec)
	}
	if want := (1760*2 + 412*0.10 + 531*10) / 1e6; !near(*exec.Cost, want) {
		t.Errorf("executor のコスト = %v, want %v", *exec.Cost, want)
	}
	if !adv.Advisor || adv.Model != "claude-opus-5" || adv.U.In != 823 || adv.U.Out != 1612 {
		t.Errorf("advisor = %+v, want claude-opus-5 の 823 入力・1612 出力", adv)
	}
	if want := (823*5 + 1612*25.0) / 1e6; !near(*adv.Cost, want) {
		t.Errorf("advisor のコスト = %v, want advisor のモデルの料金 %v", *adv.Cost, want)
	}
	if evs[3].Model != "claude-opus-5-5" || !evs[3].Advisor {
		t.Errorf("モデルのない advisor の呼び出し = %+v, want 行の advisorModel（claude-opus-5-5）", evs[3])
	}
	if s := SumUsage(evs); s.Out != 2*(531+1612) {
		t.Errorf("出力の合計 = %v, want advisor の分も入れた %v", s.Out, 2*(531+1612))
	}
	// 文脈は executor のいちばん大きい 1 回（1348 + 412）。上の項目の和（2172）でも advisor の呼び出しでもない
	if got := []float64{contextOf(evs[0]), contextOf(evs[1])}; got[0] != 1760 || got[1] != 0 {
		t.Errorf("文脈 = %v, want executor 1760・advisor 0", got)
	}
	if ms := ContextUsed(evs); len(ms) != 2 || !near(ms[0].V, 1760/1e6) {
		t.Errorf("ContextUsed = %+v, want executor の応答だけ 2 つ（1760 ÷ 1M）", ms)
	}
}

// advisor ツールを使った Haiku 5.5 の応答は、usage が executor の呼び出しの和なので、和ではなく 1 回のプロンプトの大きさで
// 100K 超えの料金かを決める（合計の数え方は変えない）。
func TestUsageHaiku55LongPromptIterations(t *testing.T) {
	var raw any
	json.Unmarshal([]byte(`{"input_tokens":120000,"output_tokens":2000,"iterations":[
		{"type":"message","input_tokens":60000,"output_tokens":1000},
		{"type":"message","input_tokens":60000,"output_tokens":1000}]}`), &raw)
	u := NewUsage()
	u.Add("m1", nil, "claude-haiku-5-5", raw)
	evs := u.Events()
	if len(evs) != 1 || evs[0].U.In != 120000 {
		t.Fatalf("events = %+v, want 入力は和の 120000", evs)
	}
	if want := (120000*0.10 + 2000*0.50) / 1e6; !near(*evs[0].Cost, want) {
		t.Errorf("コスト = %v, want 100K 以下の料金 %v（1 回は 60K）", *evs[0].Cost, want)
	}
	// executor の呼び出しが 1 回だけなら、上の項目がそのまま 1 回のプロンプト
	json.Unmarshal([]byte(`{"input_tokens":120000,"output_tokens":2000,"iterations":[{"type":"message","input_tokens":120000,"output_tokens":2000}]}`), &raw)
	u = NewUsage()
	u.Add("m1", nil, "claude-haiku-5-5", raw)
	if want := (120000*0.50 + 2000*2.50) / 1e6; !near(*u.Events()[0].Cost, want) {
		t.Errorf("1 回だけ = %v, want 100K 超えの料金 %v", *u.Events()[0].Cost, want)
	}
}

// Claude Code 自身の記録（cost-state）に advisor のモデルの分があれば、advisor の呼び出しの見積もりをその記録に合わせる
// （記録の分を別に足さないので、二重に数えない）。記録がなければ見積もりのまま。
func TestAdvisorReconciledWithReported(t *testing.T) {
	ts := 100.0
	u := NewUsage()
	u.AddAdvised("m1", &ts, "claude-sonnet-5-5", "", advisorUsage(t, "claude-opus-5"))
	evs := u.Events()
	rs := []ReportedCost{{From: 50, To: 500, Models: map[string]ReportedModel{"claude-sonnet-5-5": {Cost: 0.01}, "claude-opus-5": {Cost: 0.05}}}}
	extra, used := applyReported(rs, evs)
	if !used || len(extra) != 0 {
		t.Fatalf("used = %v・extra = %+v, want 記録を使い、別に足すものはない", used, extra)
	}
	if s := SumUsage(evs); !near(s.Cost, 0.06) {
		t.Errorf("合計 = %v, want 記録どおりの 0.06", s.Cost)
	}
	evs = u.Events()
	rs = []ReportedCost{{From: 50, To: 500, Models: map[string]ReportedModel{"claude-sonnet-5-5": {Cost: 0.01}}}}
	extra, _ = applyReported(rs, evs)
	if want := Round(0.01+(823*5+1612*25.0)/1e6, 4); len(extra) != 0 || !near(SumUsage(evs).Cost, want) {
		t.Errorf("合計 = %v, want executor は記録の 0.01、advisor は見積もり（%v）", SumUsage(evs).Cost, want)
	}
}
