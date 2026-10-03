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
