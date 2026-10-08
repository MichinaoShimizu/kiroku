package core

import "testing"

// 料金表の引き方: 版の印を除いて同じキーなら Exact、先頭一致だけなら Exact=false（画面の「Data sources」で知らせる）。
func TestPriceOfExactAndPrefix(t *testing.T) {
	cases := []struct {
		model, key string
		exact      bool
	}{
		{"claude-opus-5-5", "claude-opus-5-5", true},
		{"claude-haiku-4-5-20251001", "claude-haiku-4-5", true},
		{"claude-opus-4-20250514", "claude-opus-4", true},
		{"anthropic.claude-sonnet-4-5-20250929-v1:0", "claude-sonnet-4-5", true},
		{"us.anthropic.claude-sonnet-4-5-20250929-v1:0", "claude-sonnet-4-5", true},
		{"eu.anthropic.claude-opus-4-1-20250805-v1:0", "claude-opus-4-1", true},
		{"apac.anthropic.claude-3-5-haiku-20241022-v1:0", "claude-3-5-haiku", true},
		{"global.anthropic.claude-opus-5-5-v1", "claude-opus-5-5", true},
		{"claude-opus-4-5@20251101", "claude-opus-4-5", true},
		{"Claude-Sonnet-4.5", "claude-sonnet-4-5", true},
		{"claude-3-5-haiku-latest", "claude-3-5-haiku", true},
		{"claude-opus-4-6[1m]", "claude-opus-4-6", true},
		{"claude-haiku-5-5", "claude-haiku-5-5", true},
		{"claude-haiku-5-5-20261001", "claude-haiku-5-5", true},
		{"us.anthropic.claude-haiku-5-5-v1:0", "claude-haiku-5-5", true},
		// 新しいモデルに近い名前の料金を当てたもの
		{"claude-opus-5-6", "claude-opus-5", false},
		{"claude-sonnet-4-9-20270101", "claude-sonnet-4", false},
		// OpenAI
		{"gpt-5.5", "gpt-5.5", true},
		{"gpt-5.3-codex", "gpt-5.3-codex", true},
		{"gpt-5.5-cyber", "gpt-5.5-cyber", true}, // gpt-5.5 の料金にしない（Cyber models の表）
		{"gpt-5.6-cyber", "gpt-5.6-cyber", true},
		{"gpt-5.4-mini-2026-03-17", "gpt-5.4-mini", true},
		{"gpt-5-codex", "gpt-5", false},
		{"gpt-5.2-codex", "gpt-5.2", false},
	}
	for _, c := range cases {
		p, ok := PriceOf(c.model)
		if !ok || p.Key != c.key || p.Exact != c.exact {
			t.Errorf("PriceOf(%q) = %q exact=%v ok=%v, want %q exact=%v", c.model, p.Key, p.Exact, ok, c.key, c.exact)
		}
	}
	// 区切りのない先頭一致や、別の版の番号には当てない
	for _, m := range []string{"gpt-5.7-sol", "gpt-50", "gpt-6.2-sol", "claude-opus-45", "claude-haiku-5-55", "claude-haiku-5", "gpt-x", "", "anthropic."} {
		if p, ok := PriceOf(m); ok {
			t.Errorf("PriceOf(%q) = %q, want none", m, p.Key)
		}
	}
}

// --prices で足した Prices のキーは、同じ名前の OpenAI の料金より優先する（長いコンテキストの料金も使わない）。
func TestPriceOfOverrideWins(t *testing.T) {
	old, had := Prices["gpt-5.5"]
	Prices["gpt-5.5"] = [5]float64{1, 2, 3, 4, 5}
	defer func() {
		if had {
			Prices["gpt-5.5"] = old
		} else {
			delete(Prices, "gpt-5.5")
		}
	}()
	p, ok := PriceOf("gpt-5.5")
	if !ok || p.Rates[0] != 1 || p.Long != nil {
		t.Errorf("PriceOf = %+v", p)
	}
}

// Codex のトークン（In はキャッシュなしの入力、CR はキャッシュ済み入力、CW はキャッシュ書き込み）に OpenAI の料金をかける。
// 1 回の応答の入力が 272K を超えたら、その応答の全部を長いコンテキストの料金にする。
func TestCostOfRequestLongContext(t *testing.T) {
	short := Tokens{In: 100_000, CR: 100_000, CW: 50_000, Out: 10_000}
	c, ok := CostOfRequest("gpt-5.5", short)
	// 0.1M*5 + 0.1M*0.5 + 0.05M*5（gpt-5.5 は書き込みの料金がないので入力と同じ）+ 0.01M*30
	if want := 0.5 + 0.05 + 0.25 + 0.3; !ok || !near(c, want) {
		t.Errorf("short = %v, want %v", c, want)
	}
	long := Tokens{In: 200_000, CR: 100_000, Out: 10_000}
	c, _ = CostOfRequest("gpt-5.5", long)
	if want := 0.2*10 + 0.1*1 + 0.01*45; !near(c, want) {
		t.Errorf("long = %v, want %v", c, want)
	}
	// CostOf（何回分かの合計かもしれないもの）は短いコンテキストの料金のまま
	c, _ = CostOf("gpt-5.5", long)
	if want := 0.2*5 + 0.1*0.5 + 0.01*30; !near(c, want) {
		t.Errorf("CostOf = %v, want %v", c, want)
	}
	// 長いコンテキストの料金がないモデルは境目を超えても同じ
	c, _ = CostOfRequest("gpt-5.4-mini", long)
	if want := 0.2*0.75 + 0.1*0.075 + 0.01*4.5; !near(c, want) {
		t.Errorf("gpt-5.4-mini = %v, want %v", c, want)
	}
	c, _ = CostOfRequest("gpt-6-sol", Tokens{In: 1_000_000, CW: 1_000_000, CR: 1_000_000, Out: 1_000_000})
	if want := 4.0 + 5 + 0.4 + 15; !near(c, want) {
		t.Errorf("gpt-6-sol long = %v, want %v", c, want)
	}
}

// Sonnet 5.5 のキャッシュ読み込みは入力の 0.05 倍（$0.10）。
func TestSonnet55CacheRead(t *testing.T) {
	c, ok := CostOf("claude-sonnet-5-5", Tokens{CR: 1_000_000})
	if !ok || !near(c, 0.10) {
		t.Errorf("Sonnet 5.5 cache read = %v, want 0.10", c)
	}
}

// Haiku 5.5 は、1 回の応答のプロンプト（入力 + キャッシュの書き込み + 読み込み）が 100K を超えると、その応答の全部が高い料金になる。
// Haiku 4.5 は境目がなく、いつも同じ料金。
func TestHaiku55PromptTiers(t *testing.T) {
	// 40K + 30K + 20K + 10K = 100K ちょうどは安い料金
	at := Tokens{In: 40_000, CW: 30_000, CW1h: 20_000, CR: 10_000, Out: 10_000}
	c, ok := CostOfRequest("claude-haiku-5-5", at)
	if want := (40_000*0.10 + 30_000*0.125 + 20_000*0.20 + 10_000*0.01 + 10_000*0.50) / 1e6; !ok || !near(c, want) {
		t.Errorf("100K = %v, want %v", c, want)
	}
	// 入力は 1K でも、キャッシュの読み込みを足して 100K を超えれば高い料金
	over := Tokens{In: 1_000, CR: 100_000, Out: 10_000}
	c, _ = CostOfRequest("claude-haiku-5-5-20261001", over)
	if want := (1_000*0.50 + 100_000*0.05 + 10_000*2.50) / 1e6; !near(c, want) {
		t.Errorf("over 100K = %v, want %v", c, want)
	}
	all := Tokens{In: 1e6, CW: 1e6, CW1h: 1e6, CR: 1e6, Out: 1e6}
	c, _ = CostOfRequest("claude-haiku-5-5", all)
	if want := 0.50 + 0.625 + 1 + 0.05 + 2.50; !near(c, want) {
		t.Errorf("long = %v, want %v", c, want)
	}
	// 何回分かの合計かもしれないもの（CostOf）は安い料金のまま
	c, _ = CostOf("claude-haiku-5-5", all)
	if want := 0.10 + 0.125 + 0.20 + 0.01 + 0.50; !near(c, want) {
		t.Errorf("CostOf = %v, want %v", c, want)
	}
	c, _ = CostOfRequest("claude-haiku-4-5", all)
	if want := 1 + 1.25 + 2 + 0.10 + 5.0; !near(c, want) {
		t.Errorf("Haiku 4.5 = %v, want %v", c, want)
	}
}

// Claude Code の応答（メッセージ ID ごとにまとめたもの）は 1 回分なので、Haiku 5.5 の 100K 超えの料金が当たる。
func TestUsageHaiku55LongPrompt(t *testing.T) {
	u := NewUsage()
	u.Add("m1", nil, "claude-haiku-5-5-20261001", map[string]any{"input_tokens": 10.0, "cache_read_input_tokens": 50_000.0, "output_tokens": 100.0})
	u.Add("m1", nil, "claude-haiku-5-5-20261001", map[string]any{"input_tokens": 10.0, "cache_read_input_tokens": 150_000.0, "output_tokens": 1000.0})
	u.Add("m2", nil, "claude-haiku-5-5-20261001", map[string]any{"input_tokens": 1000.0, "output_tokens": 1000.0})
	evs := u.Events()
	if len(evs) != 2 || evs[0].Cost == nil || evs[1].Cost == nil {
		t.Fatalf("events = %+v", evs)
	}
	if want := (10*0.50 + 150_000*0.05 + 1000*2.50) / 1e6; !near(*evs[0].Cost, want) {
		t.Errorf("m1 = %v, want %v", *evs[0].Cost, want)
	}
	if want := (1000*0.10 + 1000*0.50) / 1e6; !near(*evs[1].Cost, want) {
		t.Errorf("m2 = %v, want %v", *evs[1].Cost, want)
	}
}

func near(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }
