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
