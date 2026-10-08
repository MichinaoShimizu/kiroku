package core

import "testing"

func TestContextGrowth(t *testing.T) {
	var evs []Event
	for i := 0; i < 12; i++ { // 文脈が 10k から 1 回ごとに 10k ずつ増える
		evs = append(evs, Event{U: Tokens{In: 1000, CR: float64(9000 + i*10000)}})
	}
	got := ContextGrowth(evs)
	if len(got) != 3 || got[0] != 20000 || got[1] != 110000 || got[2] != 120000 {
		t.Errorf("ContextGrowth = %v, want [20000 110000 120000]", got)
	}
	if got := ContextGrowth(evs[:5]); len(got) != 0 {
		t.Errorf("応答が少ないときは空, got %v", got)
	}
}

// コンテキストウィンドウは、料金表と同じくモデル ID の先頭一致（長いキーが優先）で引く。表にないモデルはわからない。
func TestContextWindowOf(t *testing.T) {
	cases := map[string]float64{
		"claude-fable-5-1":           1e6,
		"claude-opus-5-5":            1e6,
		"claude-opus-4-7":            1e6,
		"claude-opus-4-6":            2e5, // [1m] の版は履歴から見分けられない
		"claude-opus-4-1-20250805":   2e5,
		"claude-sonnet-5":            1e6,
		"claude-sonnet-4-6":          2e5,
		"claude-sonnet-4-5-20250929": 2e5,
		"claude-haiku-4-5-20251001":  2e5,
		"claude-haiku-5-5":           1e6,
		"claude-haiku-5-5-20261001":  1e6,
		"anthropic.claude-opus-5-5":  1e6,
	}
	for m, want := range cases {
		if got, ok := ContextWindowOf(m); !ok || got != want {
			t.Errorf("ContextWindowOf(%q) = %v, %v, want %v", m, got, ok, want)
		}
	}
	for _, m := range []string{"gpt-5", "claude-new-9", ""} {
		if _, ok := ContextWindowOf(m); ok {
			t.Errorf("ContextWindowOf(%q): 表にないモデルは ok=false のはず", m)
		}
	}
}

// 応答ごとの使用率 = (入力 + キャッシュの書き込み・読み込み) ÷ ウィンドウ。200K のモデルで 200K を超えた応答があれば、
// そのモデルは [1m] の版とみなして 1M で割る（100% を超えて見せない）。表にないモデルは数えない。
func TestContextUsed(t *testing.T) {
	ev := func(model string, in, cw, cr float64) Event {
		return Event{Model: model, U: Tokens{In: in, CW: cw, CR: cr, Out: 999}}
	}
	ms := ContextUsed([]Event{
		ev("claude-opus-5-5", 1000, 9000, 490000), // 500K ÷ 1M
		ev("claude-haiku-4-5", 100, 0, 49900),     // 50K ÷ 200K
		ev("gpt-5", 1000, 0, 0),                   // わからない
		ev("claude-opus-5-5", 0, 0, 0),            // 文脈なし
	})
	want := []float64{0.5, 0.25}
	if len(ms) != len(want) {
		t.Fatalf("ContextUsed = %+v, want %v", ms, want)
	}
	for i, m := range ms {
		if m.Key != "context_used" || m.V != want[i] {
			t.Errorf("ContextUsed[%d] = %+v, want context_used %v", i, m, want[i])
		}
	}
	// Opus 4.6: 150K だけなら 200K で割るが、300K を読んだ応答があれば同じモデルは 1M で割る
	if ms := ContextUsed([]Event{ev("claude-opus-4-6", 0, 0, 150000)}); len(ms) != 1 || ms[0].V != 0.75 {
		t.Errorf("200K のまま = %+v, want 0.75", ms)
	}
	ms = ContextUsed([]Event{ev("claude-opus-4-6", 0, 0, 150000), ev("claude-opus-4-6", 0, 0, 300000)})
	if len(ms) != 2 || ms[0].V != 0.15 || ms[1].V != 0.3 {
		t.Errorf("[1m] とみなす = %+v, want 0.15・0.3", ms)
	}
	if got := PeakContextWindow([]Event{ev("claude-opus-4-6", 0, 0, 150000), ev("claude-opus-4-6", 0, 0, 300000)}); got != 1e6 {
		t.Errorf("PeakContextWindow = %v, want 1e6", got)
	}
	if got := PeakContextWindow([]Event{ev("claude-haiku-4-5", 0, 0, 1000), ev("claude-opus-5-5", 0, 0, 5000)}); got != 1e6 {
		t.Errorf("PeakContextWindow = %v, want 最大の応答のモデル（opus-5-5）の 1e6", got)
	}
	if got := PeakContextWindow([]Event{ev("gpt-5", 0, 0, 5000)}); got != 0 {
		t.Errorf("PeakContextWindow = %v, want わからないので 0", got)
	}
}
