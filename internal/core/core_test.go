package core

import (
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
	}
	for _, c := range cases {
		got, ok := ParseTS(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("ParseTS(%v) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
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

// 言い直しの判定は、日本語と英語のよくある言い回しを拾い、ふつうの依頼は拾わない。
func TestCorrection(t *testing.T) {
	for _, s := range []string{"違う、そうじゃなくて", "やり直して", "No, that's wrong", "Revert that and try again", "It still doesn't work", "The build is still failing", "nope", "Roll back the last change"} {
		if !Correction.MatchString(s) {
			t.Errorf("拾うべき: %q", s)
		}
	}
	for _, s := range []string{"Fix validation on the login form", "Add a working example", "Write the release notes", "Make the tests pass too", "テストも通して"} {
		if Correction.MatchString(s) {
			t.Errorf("拾わないべき: %q", s)
		}
	}
}
