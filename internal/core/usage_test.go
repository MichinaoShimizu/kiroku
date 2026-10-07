package core

import (
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
