package core

import (
	"math"
	"testing"
)

func TestApplyReported(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	main := []Event{
		{T: f(100), Model: "claude-opus-5-5", U: Tokens{Out: 100}, Cost: f(1)},
		{T: f(200), Model: "claude-opus-5-5", U: Tokens{Out: 300}, Cost: f(3)},
		{T: f(300), Model: "claude-new-9", U: Tokens{Out: 10}},                 // 料金表にないモデル
		{T: f(900), Model: "claude-opus-5-5", U: Tokens{Out: 100}, Cost: f(1)}, // 記録の期間の外（あとから）
	}
	sub := []Event{{T: f(250), Model: "claude-opus-5-5", U: Tokens{Out: 100}, Cost: f(4)}}
	rs := []ReportedCost{{From: 50, To: 500, Models: map[string]ReportedModel{
		"claude-opus-5-5":  {Cost: 16},                       // 見積もり 1+3+4=8 の 2 倍
		"claude-new-9":     {Cost: 0.5},                      // 見積もれないモデルはトークンの比で配る
		"claude-haiku-4-5": {Cost: 0.2, U: Tokens{In: 1000}}, // 履歴に応答がない（タイトル付けなど）
	}}}
	extra := applyReported(rs, main, sub)
	near := func(p *float64, want float64) bool { return p != nil && math.Abs(*p-want) < 1e-9 }
	if !near(main[0].Cost, 2) || !near(main[1].Cost, 6) || !near(sub[0].Cost, 8) {
		t.Errorf("比で配っていない: %v %v %v", *main[0].Cost, *main[1].Cost, *sub[0].Cost)
	}
	if !near(main[2].Cost, 0.5) {
		t.Errorf("料金表にないモデル = %v", main[2].Cost)
	}
	if !near(main[3].Cost, 1) {
		t.Errorf("期間の外は見積もりのまま: %v", *main[3].Cost)
	}
	if len(extra) != 1 || extra[0].Model != "claude-haiku-4-5" || !near(extra[0].Cost, 0.2) || *extra[0].T != 500 {
		t.Errorf("履歴にないモデルの分 = %+v", extra)
	}
}
