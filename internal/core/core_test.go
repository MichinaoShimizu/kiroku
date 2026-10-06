package core

import (
	"strings"
	"testing"
	"time"
)

func TestParseTS(t *testing.T) {
	time.Local = time.UTC
	cases := []struct {
		in   any
		want float64
		ok   bool
	}{
		{"2026-09-28T01:00:00Z", 1790557200, true},
		{"2026-09-28T10:00:00+09:00", 1790557200, true},
		{"2026-09-28T01:00:00.500Z", 1790557200.5, true},
		{1790557200.0, 1790557200, true},
		{1790557200000.0, 1790557200, true}, // ミリ秒
		{"1790557200", 1790557200, true},
		{int64(1790557200000), 1790557200, true}, // SQLite の INTEGER 列（ミリ秒）
		{"", 0, false},
		{nil, 0, false},
		{"きのう", 0, false},
		// ありえない時刻は読めなかったものとする
		{100.0, 0, false},
		{"100", 0, false},
		{0.0, 0, false},
		{-1.0, 0, false},
		{"0001-01-01T00:00:00Z", 0, false},
		{"1970-01-01T00:00:00Z", 0, false},
		{"1999-12-31T23:59:59Z", 0, false},
		{"2000-01-01T00:00:00Z", 946684800, true},
		{"9999-12-31T23:59:59Z", 0, false},
		{1e15, 0, false}, // ミリ秒として読んでも 3 万年後
	}
	for _, c := range cases {
		got, ok := ParseTS(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("ParseTS(%v) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseTSFutureBound(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	defer func(f func() time.Time) { nowForTS = f }(nowForTS)
	nowForTS = func() time.Time { return now }
	for _, c := range []struct {
		in time.Time
		ok bool
	}{
		{now, true},
		{now.Add(23 * time.Hour), true}, // 時計のずれくらいは許す
		{now.Add(25 * time.Hour), false},
		{now.AddDate(5, 0, 0), false},
	} {
		if _, ok := ParseTS(c.in.Format(time.RFC3339)); ok != c.ok {
			t.Errorf("ParseTS(%v) ok=%v want %v", c.in, ok, c.ok)
		}
		if _, ok := ParseTS(float64(c.in.UnixMilli())); ok != c.ok {
			t.Errorf("ParseTS(%v ms) ok=%v want %v", c.in, ok, c.ok)
		}
	}
}

func TestUsageDedupesStreamedLines(t *testing.T) {
	u := NewUsage()
	t1 := 1.0
	u.Add("m1", &t1, "claude-opus-5-5", map[string]any{"input_tokens": 3.0, "output_tokens": 1.0, "cache_read_input_tokens": 100.0})
	u.Add("m1", &t1, "claude-opus-5-5", map[string]any{"input_tokens": 3.0, "output_tokens": 900.0, "cache_read_input_tokens": 100.0})
	s := SumUsage(u.Events())
	if s.Out != 900 || s.CR != 100 || s.In != 3 {
		t.Fatalf("1 回だけ数えるはず: %+v", s)
	}
}

func TestPriceLongestPrefix(t *testing.T) {
	if p, _ := PriceOf("claude-opus-5-5"); p[0] != 4 {
		t.Error("claude-opus-5-5 は claude-opus-5 より長いキーを使う", p)
	}
	if p, _ := PriceOf("claude-opus-4-1-20250805"); p[0] != 15 {
		t.Error(p)
	}
	if _, ok := PriceOf("gpt-x"); ok {
		t.Error("知らないモデルは料金なし")
	}
	if ModelName("claude-haiku-4-5-20251001") != "claude-haiku-4-5" {
		t.Error("日付の版はまとめる")
	}
}

func TestRoundHalfEven(t *testing.T) {
	if Round(2.5, 0) != 2 || Round(3.5, 0) != 4 || Round(0.125, 2) != 0.12 {
		t.Error("Python の round と同じく偶数に丸める")
	}
}

func TestAggregateNative(t *testing.T) {
	ms := []Measure{
		{Key: "credits", V: 1.5}, {Key: "turns", V: 1}, {Key: "credits", V: 0.5}, {Key: "turns", V: 1},
		{Key: "requests", V: 3},
	}
	got := map[string]NativeValue{}
	for _, v := range AggregateNative("Kiro CLI", ms) {
		got[v.Label] = v
	}
	if got["クレジット"].V != 2 || got["1ターンあたりのクレジット"].V != 1 || got["モデルへのリクエスト"].N != 1 {
		t.Errorf("集計 = %+v", got)
	}
	if _, ok := got["組み込みツールの実行"]; ok {
		t.Error("観測のない指標は出さない（0 と見せない）")
	}
	med := AggregateNative("Amazon Q", []Measure{{Key: "latency", V: 3}, {Key: "latency", V: 1}, {Key: "latency", V: 10}, {Key: "latency", V: 2}})
	if len(med) != 1 || med[0].V != 2.5 {
		t.Errorf("中央値 = %+v", med)
	}
	if AggregateNative("知らないエージェント", ms) == nil || len(AggregateNative("知らないエージェント", ms)) != 0 {
		t.Error("定義のないエージェントは空")
	}
}

// 言い直しの判定は、日本語と英語のよくある言い回しを拾い、ふつうの依頼・貼り付けたログ・会話の最初の依頼は拾わない（例は correction_cases_test.go）。
func TestCorrection(t *testing.T) {
	for _, c := range correctionCases {
		if got := IsCorrection(c.text, c.first); got != c.want {
			t.Errorf("IsCorrection(%q, first=%v) = %v, want %v", c.text, c.first, got, c.want)
		}
	}
}

// fast モードは通常の 2 倍、US 内だけの推論は 1.1 倍（重ねて掛かる）。記録が分かれていても、どれかの行にあれば効く。
func TestUsageRateMultipliers(t *testing.T) {
	u := NewUsage()
	ts := 1.0
	u.Add("a", &ts, "claude-opus-5-5", map[string]any{"input_tokens": 1e6})
	u.Add("b", &ts, "claude-opus-5-5", map[string]any{"input_tokens": 1e6, "speed": "fast"})
	u.Add("c", &ts, "claude-opus-5-5", map[string]any{"input_tokens": 1e6})
	u.Add("c", &ts, "claude-opus-5-5", map[string]any{"input_tokens": 1e6, "speed": "fast", "inference_geo": "us"})
	want := map[int]float64{0: 4, 1: 8, 2: 8.8}
	for i, e := range u.Events() {
		if e.Cost == nil || Round(*e.Cost, 4) != want[i] {
			t.Errorf("event %d cost = %v, want %v", i, e.Cost, want[i])
		}
	}
}

func TestPromptFlow(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	b := NewBuilder("Claude Code", "s")
	b.Prompt(f(1000), "最初の依頼")
	b.Agent(f(1010))
	b.Agent(f(1100))
	b.Prompt(f(1160), "[Request interrupted by user]")
	b.Prompt(f(1200), strings.Repeat("あ", PromptRunes+5))
	b.Agent(f(1500))
	b.Prompt(f(5000), "AI が動く前に次の依頼")
	b.Prompt(nil, "時刻のない依頼")
	b.Outputs = append(b.Outputs, Output{T: f(1400), Kind: "pr", V: 1, URL: "https://example.com/pr/1"})
	s := b.Finish(15)
	if len(s.Prompts) != 4 || s.Interrupts != 1 || len(s.InterruptsAt) != 1 || s.InterruptsAt[0] != 1160 {
		t.Fatalf("prompts %d, interrupts %d %v", len(s.Prompts), s.Interrupts, s.InterruptsAt)
	}
	// 1 件目は 1100 まで AI が動き、1200 の依頼まで 100 秒待たせた
	if p := s.Prompts[0]; p.Work != 100 || p.Wait != 100 {
		t.Errorf("1 件目 work %v wait %v", p.Work, p.Wait)
	}
	// 2 件目は切られて元の長さが残り、Full で全文が読める
	if p := s.Prompts[1]; p.Len != PromptRunes+5 || len([]rune(p.Text)) != PromptRunes || len([]rune(p.Full())) != PromptRunes+5 || p.Work != 300 || p.Wait != 3500 {
		t.Errorf("2 件目 len %d text %d full %d work %v wait %v", p.Len, len([]rune(p.Text)), len([]rune(p.Full())), p.Work, p.Wait)
	}
	// 間に AI が動いていない依頼と、時刻のない依頼には入れない
	for _, p := range s.Prompts[2:] {
		if p.Work != 0 || p.Wait != 0 || p.Len != 0 || p.Full() != p.Text {
			t.Errorf("%q: work %v wait %v len %d", p.Text, p.Work, p.Wait, p.Len)
		}
	}
	if len(s.PRAt) != 1 || s.PRAt[0].T != 1400 || s.PRAt[0].URL == "" {
		t.Errorf("prAt %v", s.PRAt)
	}
}

// サブエージェントが動いていた間は、待たせに数えない。
func TestPromptFlowCountsSubagents(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	b := NewBuilder("Claude Code", "s")
	b.Prompt(f(1000), "調べて")
	b.Agent(f(1060))
	b.Subagents = append(b.Subagents, Subagent{Type: "Explore", Start: f(1060), End: f(1500)})
	b.Prompt(f(1600), "次")
	if p := b.Finish(15).Prompts[0]; p.Work != 500 || p.Wait != 100 {
		t.Errorf("work %v wait %v", p.Work, p.Wait)
	}
}

func TestPromptFlowKeepsAllPrompts(t *testing.T) {
	b := NewBuilder("Claude Code", "s")
	for i := 0; i < 80; i++ {
		v := float64(1000 + i*60)
		b.Tick(&v)
		b.Prompt(&v, "依頼")
	}
	if s := b.Finish(15); len(s.Prompts) != 80 || s.NPrompts != 80 {
		t.Errorf("prompts %d / %d", len(s.Prompts), s.NPrompts)
	}
}
