package core

import "sort"

// エージェントごとに、そのエージェントだけが記録している数字（参考指標）。
// 定義がエージェントごとに違うので、エージェント同士で比べるためのものではない。

// Measure は 1 つの観測（例: 1 ターンで使ったクレジット）。
type Measure struct {
	Key string
	T   *float64
	V   float64
}

// NativeDef は参考指標の定義。Agg は sum / avg / median / max / ratio（Num÷Den）。
type NativeDef struct {
	Key, Label, Unit, Agg string
	Num, Den              string  // ratio のとき
	Scale                 float64 // 表示用の倍率（割合なら 100）
}

// NativeDefs はエージェント（Source の名前）ごとの参考指標。並びは画面の順。
var NativeDefs = map[string][]NativeDef{
	"Claude Code": {
		{Key: "responses", Label: "応答の数", Unit: "回", Agg: "sum"},
		{Key: "out_per_response", Label: "1応答あたりの出力トークン", Unit: "トークン", Agg: "avg"},
		{Key: "cache_ratio", Label: "入力のうちキャッシュから読んだ割合", Unit: "%", Agg: "ratio", Num: "cache_read", Den: "input_all", Scale: 100},
		{Key: "tool_calls", Label: "ツール呼び出し", Unit: "回", Agg: "sum"},
		{Key: "subagents", Label: "サブエージェントの実行", Unit: "回", Agg: "sum"},
	},
	"Kiro CLI": {
		{Key: "credits", Label: "クレジット", Unit: "クレジット", Agg: "sum"},
		{Key: "turns", Label: "ターン", Unit: "回", Agg: "sum"},
		{Key: "credits_per_turn", Label: "1ターンあたりのクレジット", Unit: "クレジット", Agg: "ratio", Num: "credits", Den: "turns", Scale: 1},
		{Key: "requests", Label: "モデルへのリクエスト", Unit: "回", Agg: "sum"},
		{Key: "builtin_tools", Label: "組み込みツールの実行", Unit: "回", Agg: "sum"},
	},
	"Kiro IDE": {
		{Key: "credits", Label: "クレジット", Unit: "クレジット", Agg: "sum"},
		{Key: "turns", Label: "ターン", Unit: "回", Agg: "sum"},
		{Key: "credits_per_turn", Label: "1ターンあたりのクレジット", Unit: "クレジット", Agg: "ratio", Num: "credits", Den: "turns", Scale: 1},
		{Key: "tool_calls", Label: "ツール呼び出し", Unit: "回", Agg: "sum"},
	},
	"Kiro CLI (SQLite)": qstoreDefs,
	"Amazon Q":          qstoreDefs,
	"Kiro Crew": {
		{Key: "crew_sessions", Label: "Crew から動かした会話", Unit: "件", Agg: "sum"},
		{Key: "crew_subagents", Label: "うちサブエージェント", Unit: "件", Agg: "sum"},
		{Key: "credits", Label: "クレジット", Unit: "クレジット", Agg: "sum"},
		{Key: "turns", Label: "ターン", Unit: "回", Agg: "sum"},
	},
	"Codex": {
		{Key: "responses", Label: "応答の数", Unit: "回", Agg: "sum"},
		{Key: "reasoning", Label: "推論トークン", Unit: "トークン", Agg: "sum"},
		{Key: "reasoning_ratio", Label: "出力のうち推論の割合", Unit: "%", Agg: "ratio", Num: "reasoning", Den: "output", Scale: 100},
		{Key: "cache_ratio", Label: "入力のうちキャッシュから読んだ割合", Unit: "%", Agg: "ratio", Num: "cache_read", Den: "input_all", Scale: 100},
		{Key: "context_used", Label: "コンテキストの最大使用率", Unit: "%", Agg: "max", Scale: 100},
		{Key: "rate_limit", Label: "レート制限の最大使用率", Unit: "%", Agg: "max"},
		{Key: "tool_calls", Label: "ツール呼び出し", Unit: "回", Agg: "sum"},
	},
}

var qstoreDefs = []NativeDef{
	{Key: "ttfc", Label: "最初の返事までの時間（中央値）", Unit: "秒", Agg: "median"},
	{Key: "latency", Label: "応答にかかった時間（中央値）", Unit: "秒", Agg: "median"},
	{Key: "response_size", Label: "応答の長さ（平均）", Unit: "文字", Agg: "avg"},
	{Key: "tool_calls", Label: "ツール呼び出し", Unit: "回", Agg: "sum"},
}

// NativeValue は 1 つの参考指標の値。
type NativeValue struct {
	Label string  `json:"label"`
	Unit  string  `json:"unit"`
	V     float64 `json:"v"`
	N     int     `json:"n"` // 元になった観測の数
}

// AggregateNative は観測を定義に沿ってまとめる。観測のない指標は出さない（0 と見せないため）。
func AggregateNative(source string, ms []Measure) []NativeValue {
	by := map[string][]float64{}
	for _, m := range ms {
		by[m.Key] = append(by[m.Key], m.V)
	}
	sum := func(xs []float64) float64 {
		s := 0.0
		for _, x := range xs {
			s += x
		}
		return s
	}
	out := []NativeValue{}
	for _, d := range NativeDefs[source] {
		scale := d.Scale
		if scale == 0 {
			scale = 1
		}
		var v float64
		var n int
		switch d.Agg {
		case "ratio":
			num, den := by[d.Num], by[d.Den]
			if len(den) == 0 || sum(den) == 0 {
				continue
			}
			v, n = sum(num)/sum(den)*scale, len(den)
		default:
			xs := by[d.Key]
			if len(xs) == 0 {
				continue
			}
			n = len(xs)
			switch d.Agg {
			case "sum":
				v = sum(xs)
			case "avg":
				v = sum(xs) / float64(n)
			case "max":
				v = xs[0]
				for _, x := range xs {
					v = max(v, x)
				}
			case "median":
				s := append([]float64(nil), xs...)
				sort.Float64s(s)
				if n%2 == 1 {
					v = s[n/2]
				} else {
					v = (s[n/2-1] + s[n/2]) / 2
				}
			}
			v *= scale
		}
		out = append(out, NativeValue{Label: d.Label, Unit: d.Unit, V: Round(v, 2), N: n})
	}
	return out
}
