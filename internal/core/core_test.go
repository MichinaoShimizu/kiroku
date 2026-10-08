package core

import (
	"fmt"
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

// AddEvent でコストを決めて入れた使用量（エージェントが記録したドル額）は、料金表で見積もり直さない。nil なら見積もる。
func TestAddEventKeepsRecordedCost(t *testing.T) {
	s := NewBuilder("Kiro Crew", "x")
	t1, zero, rec := 1.0, 0.0, 0.42
	s.AddEvent(Event{T: &t1, Model: "claude-haiku-4-5", U: Tokens{In: 1000}, Cost: &rec})
	s.AddEvent(Event{T: &t1, Model: "claude-haiku-4-5", U: Tokens{In: 1000}, Cost: &zero})
	s.AddEvent(Event{T: &t1, Model: "claude-haiku-4-5", U: Tokens{In: 1000}})
	evs := s.Usage.Events()
	if len(evs) != 3 || *evs[0].Cost != 0.42 || *evs[1].Cost != 0 || *evs[2].Cost != 0.001 {
		t.Fatalf("コスト = %v %v %v", *evs[0].Cost, *evs[1].Cost, *evs[2].Cost)
	}
	*evs[0].Cost = 9 // 返したものを書きかえても、元の記録は変わらない
	if c := *s.Usage.Events()[0].Cost; c != 0.42 {
		t.Errorf("元の記録が変わった: %v", c)
	}
}

func TestPriceLongestPrefix(t *testing.T) {
	if p, _ := PriceOf("claude-opus-5-5"); p.Rates[0] != 4 {
		t.Error("claude-opus-5-5 は claude-opus-5 より長いキーを使う", p)
	}
	if p, _ := PriceOf("claude-opus-4-1-20250805"); p.Rates[0] != 15 {
		t.Error(p)
	}
	if _, ok := PriceOf("gpt-x"); ok {
		t.Error("知らないモデルは料金なし")
	}
	for in, want := range map[string]string{
		"claude-haiku-4-5-20251001": "claude-haiku-4-5",
		"claude-sonnet-4-5":         "claude-sonnet-4-5",
		"gpt-5-2025-08-07":          "gpt-5-2025-08-07", // 日付の形が違うものはそのまま（料金は PriceOf が引く）
		"x-2025100":                 "x-2025100",
		"x20251001":                 "x20251001",
		"-20251001":                 "",
		"m-2025100a":                "m-2025100a",
		"":                          "",
	} {
		if got := ModelName(in); got != want {
			t.Errorf("ModelName(%q) = %q, want %q", in, got, want)
		}
	}
}

// priceID は結果を覚えておくが、覚える数には上限がある（履歴に変わったモデル ID がたくさんあっても、メモリを使い続けない）。
func TestPriceIDCacheIsBounded(t *testing.T) {
	for i := range maxPriceIDs + 100 {
		if got := priceID(fmt.Sprintf("Claude-Model-%d-20251001", i)); got != fmt.Sprintf("claude-model-%d", i) {
			t.Fatalf("priceID = %q", got)
		}
	}
	priceIDs.RLock()
	n := len(priceIDs.m)
	priceIDs.RUnlock()
	if n > maxPriceIDs {
		t.Errorf("覚えている数 = %d, want <= %d", n, maxPriceIDs)
	}
	if priceID("us.anthropic.claude-opus-4-1-20250805-v1:0") != "claude-opus-4-1" {
		t.Error("上限に達したあとも、正しく引ける")
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

// 応答（エージェントが人に返した文）は、1 つの依頼につき最後の 1 つだけが付き、長ければ切られる。
func TestPromptReplies(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	b := NewBuilder("Claude Code", "s")
	b.Reply(f(900), "m0", "依頼の前の応答（続きのセッションの頭）")
	b.Prompt(f(1000), "直して")
	b.Reply(f(1010), "m1", "見てみます") // 同じターンの前の文は残らない
	b.Reply(f(1100), "m2", "直しました")
	b.Agent(f(1100))
	b.Prompt(f(1200), "次")
	b.Reply(f(1210), "m3", strings.Repeat("長", ReplyRunes+7))
	b.Reply(f(1220), "m3", "つづき") // 同じメッセージの続きはつなぐ
	b.Prompt(f(1300), "応答のない依頼")
	s := b.Finish(15)
	if len(s.Prompts) != 3 {
		t.Fatalf("prompts %d", len(s.Prompts))
	}
	if r := s.Prompts[0].Reply; r == nil || r.Text != "直しました" || r.T == nil || *r.T != 1100 || r.Len != 0 {
		t.Errorf("1 件目の応答 %+v", r)
	}
	if r := s.Prompts[1].Reply; r == nil || len([]rune(r.Text)) != ReplyRunes || r.Len != ReplyRunes+7+1+len([]rune("つづき")) ||
		!strings.HasSuffix(r.Full(), "つづき") {
		t.Errorf("2 件目の応答 %+v", r)
	}
	if r := s.Prompts[2].Reply; r != nil {
		t.Errorf("3 件目に応答が付いた %+v", r)
	}
}

// 依頼をあとからまとめて足すアダプター（Codex）でも、時刻から応答が割り当てられる。
func TestPromptRepliesByTime(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	b := NewBuilder("Codex", "s")
	b.Turn()
	b.Agent(f(1100))
	b.Reply(f(1100), "", "1 つめの答え")
	b.Turn()
	b.Agent(f(1300))
	b.Reply(f(1300), "", "2 つめの答え")
	b.Prompt(f(1000), "はじめ")
	b.Prompt(f(1200), "つぎ")
	s := b.Finish(15)
	if r := s.Prompts[0].Reply; r == nil || r.Text != "1 つめの答え" {
		t.Errorf("1 件目の応答 %+v", r)
	}
	if r := s.Prompts[1].Reply; r == nil || r.Text != "2 つめの答え" {
		t.Errorf("2 件目の応答 %+v", r)
	}
}

// 時刻のない応答と空の応答は残さない（HTML に空の行を出さないため）。
func TestReplyNeedsTimeAndText(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	b := NewBuilder("Claude Code", "s")
	b.Tick(f(1000))
	b.Prompt(f(1000), "直して")
	b.Reply(nil, "m1", "時刻がない")
	b.Reply(f(0), "m2", "時刻が 0")
	b.Reply(f(1010), "m3", "   \n ")
	if r := b.Finish(15).Prompts[0].Reply; r != nil {
		t.Errorf("応答が付いた %+v", r)
	}
}

// 利用上限のエラー文。Claude Code の今の文言（https://code.claude.com/docs/en/errors）と古い文言を拾い、
// サーバー側の一時的な絞り込みや会話の長さの上限は数えない。
func TestIsLimitError(t *testing.T) {
	yes := []string{
		"Claude AI usage limit reached|1790000000",
		"5-hour limit reached ∙ resets 3pm",
		`API Error: 429 {"type":"error","error":{"type":"rate_limit_error"}}`,
		"You've hit your individual usage limit",
		"You've hit your session limit · resets 3:45pm",
		"You've hit your weekly limit · resets Mon 12:00am",
		"You've hit your Opus limit · resets 3:45pm",
		"You've hit your Sonnet limit · resets 3:45pm",
		"You've hit your monthly spend limit · raise it at claude.ai/settings/usage",
		"You've hit your individual spend limit · ask your admin for a higher limit",
		"You've hit your org's monthly spend limit · visit claude.ai/admin-settings/usage to raise it",
		"You've hit your team's shared budget · ask your admin to raise it at claude.ai/admin-settings/usage",
		"You've hit your channel's monthly spend limit · an org owner or channel manager can raise it in the channel's Claude settings",
		"API Error: Request rejected (429) · this may be a temporary capacity issue. If it persists, check https://status.claude.com.",
		"spend limit reached (daily; resets 2026-08-09 00:00 UTC)",
		"Usage limit reached · continuing automatically at 3:45pm · esc to cancel",
		"429 Too Many Requests",
		"API Error: status 429",
	}
	for _, s := range yes {
		if !IsLimitError(s) {
			t.Errorf("IsLimitError(%q) = false, want true", s)
		}
	}
	no := []string{
		"API Error: Server is temporarily limiting requests (not your usage limit)",
		"Context limit reached · /compact or /clear to continue",
		"Context limit reached · /clear to continue",
		"Prompt is too long",
		"API Error: Connection error.",
		"API Error: Usage credits required for 1M context · run /usage-credits to turn them on, or /model to switch to standard context",
		// 429 が状態コードではなく数の一部
		"API Error: Output blocked by content filtering policy (~1,429 tokens)",
		"Prompt is too long: 201429 tokens > 200000 maximum",
		// 上限が解除されて続きを始めた知らせ
		"Usage limit reset · continuing automatically",
		"Your usage limit has reset",
	}
	for _, s := range no {
		if IsLimitError(s) {
			t.Errorf("IsLimitError(%q) = true, want false", s)
		}
	}
}

// 利用上限のエラー文から、解除の時刻の文を書いてあるとおりに取り出す（日付や時間帯がなくても時刻に直さない）。
// 曜日・日付・時刻・時間帯の形の文字だけを拾い、それ以外（HTML など）は拾わない。
func TestLimitReset(t *testing.T) {
	cases := map[string]string{
		"You've hit your session limit · resets 3:45pm":                              "3:45pm",
		"You've hit your weekly limit · resets Mon 12:00am":                          "Mon 12:00am",
		"5-hour limit reached ∙ resets 3pm":                                          "3pm",
		"spend limit reached (daily; resets 2026-08-09 00:00 UTC)":                   "2026-08-09 00:00 UTC",
		"You've hit your session limit · resets 3pm (Asia/Tokyo)":                    "3pm (Asia/Tokyo)",
		"You've hit your Opus limit · resets 3:45pm. Run /usage":                     "3:45pm",
		"Claude AI usage limit reached|1790000000":                                   "",
		"You've hit your monthly spend limit · raise it at claude.ai/settings/usage": "",
		"resets <img src=x onerror=alert(1)>":                                        "",
		"Usage limit reached · continuing automatically at 3:45pm · esc to cancel":   "3:45pm",
		"Usage limit reached · limit resets 3:45pm":                                  "3:45pm",
	}
	for in, want := range cases {
		if got := LimitReset(in); got != want {
			t.Errorf("LimitReset(%q) = %q, want %q", in, got, want)
		}
	}
}

// キャッシュ書き込みの内訳。古い Claude Code は cache_creation_input_tokens を 0 にして内訳だけを書く。
func TestReadUsageCacheCreation(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want Tokens
	}{
		{"合計だけ", map[string]any{"cache_creation_input_tokens": 100.0}, Tokens{CW: 100}},
		{"合計と内訳", map[string]any{"cache_creation_input_tokens": 100.0, "cache_creation": map[string]any{"ephemeral_5m_input_tokens": 40.0, "ephemeral_1h_input_tokens": 60.0}}, Tokens{CW: 40, CW1h: 60}},
		{"合計が 0 で内訳だけ", map[string]any{"cache_creation_input_tokens": 0.0, "cache_creation": map[string]any{"ephemeral_5m_input_tokens": 300.0, "ephemeral_1h_input_tokens": 0.0}}, Tokens{CW: 300}},
		{"合計がなく内訳だけ", map[string]any{"cache_creation": map[string]any{"ephemeral_5m_input_tokens": 30.0, "ephemeral_1h_input_tokens": 70.0}}, Tokens{CW: 30, CW1h: 70}},
		{"1 時間の内訳だけ", map[string]any{"cache_creation": map[string]any{"ephemeral_1h_input_tokens": 70.0}}, Tokens{CW1h: 70}},
	}
	for _, c := range cases {
		if got := ReadUsage(c.in); got != c.want {
			t.Errorf("%s: ReadUsage = %+v, want %+v", c.name, got, c.want)
		}
	}
}

// mayCorrect が false のとき、correctionAny は必ず合わない（速くするための前ふるいで、判定は変えない）。
// correctionAny の言い回しを 1 つずつと、(?i) が合わせる K（U+212A）・ſ（U+017F）を含めて確かめる。
func TestCorrectionHints(t *testing.T) {
	texts := []string{
		"それは違う", "そうじゃない", "そうではない", "AじゃなくてB", "AではなくてB", "やり直して", "やりなおして", "元に戻す", "戻して", "取り消して",
		"まだ直ってない", "直ってない", "直っていない",
		"that's not what I asked", "This is not right", "it is not it", "That's wrong", "it's incorrect",
		"not what i wanted", "NOT WHAT I MEANT", "revert it", "Undo that", "roll back these", "rollback the last", "undo what you did",
		"revert everything", "start over", "ſtart over", "You broke it", "still not working", "it's ſtill broken", "is still failing",
		"doesn't work", "does not working", "didn't worK", "isn't work", "still does not work", "try again", "ok. Try again", "TRY AGAIN",
		"Fix the failing test", "Continue", "テストを足して", "",
		"THAT'S NOT IT", "it\u017fs wrong", "it'ſ not right", "this iſ incorrect", "\u212aeep going, \u017ftill broken", "doesn't wor\u212a", "ſtill doesn't work",
		"revert\tit", "undo\u00a0it", "try\u3000again", "no.try again", "直っていない。もう一度",
	}
	for _, c := range correctionCases {
		texts = append(texts, c.text)
	}
	for _, s := range texts {
		h := humanHead(s)
		if correctionAny.MatchString(h) && !mayCorrect(h) {
			t.Errorf("%q は correctionAny に合うのに mayCorrect が false（correctionHints に語が足りない）", s)
		}
	}
	if mayCorrect("Fix the failing test in pkg12 and explain why it broke; the worktree has notes") {
		t.Error("ふつうの依頼でも正規表現を試している（前ふるいが効いていない）")
	}
}
