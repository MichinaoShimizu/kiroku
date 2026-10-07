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
	LabelEn               string  // 英語表示のときのラベル
	Num, Den              string  // ratio のとき
	Scale                 float64 // 表示用の倍率（割合なら 100）
}

// NativeDefs はエージェント（Source の名前）ごとの参考指標。並びは画面の順。
var NativeDefs = map[string][]NativeDef{
	"Claude Code": {
		{Key: "responses", Label: "応答の数", LabelEn: "Responses", Unit: "回", Agg: "sum"},
		{Key: "out_per_response", Label: "1応答あたりの出力トークン", LabelEn: "Output tokens per response", Unit: "トークン", Agg: "avg"},
		{Key: "tool_calls", Label: "ツール呼び出し", LabelEn: "Tool calls", Unit: "回", Agg: "sum"},
	},
	"Kiro CLI": {
		{Key: "credits", Label: "クレジット", LabelEn: "Credits", Unit: "クレジット", Agg: "sum"},
		{Key: "turns", Label: "ターン", LabelEn: "Turns", Unit: "回", Agg: "sum"},
		{Key: "credits_per_turn", Label: "1ターンあたりのクレジット", LabelEn: "Credits per turn", Unit: "クレジット", Agg: "ratio", Num: "credits", Den: "turns", Scale: 1},
		{Key: "requests", Label: "モデルへのリクエスト", LabelEn: "Model requests", Unit: "回", Agg: "sum"},
		{Key: "builtin_tools", Label: "組み込みツールの実行", LabelEn: "Built-in tool runs", Unit: "回", Agg: "sum"},
	},
	"Kiro IDE": {
		{Key: "credits", Label: "クレジット", LabelEn: "Credits", Unit: "クレジット", Agg: "sum"},
		{Key: "turns", Label: "ターン", LabelEn: "Turns", Unit: "回", Agg: "sum"},
		{Key: "credits_per_turn", Label: "1ターンあたりのクレジット", LabelEn: "Credits per turn", Unit: "クレジット", Agg: "ratio", Num: "credits", Den: "turns", Scale: 1},
		{Key: "tool_calls", Label: "ツール呼び出し", LabelEn: "Tool calls", Unit: "回", Agg: "sum"},
	},
	"Kiro CLI (SQLite)": qstoreDefs,
	"Amazon Q":          qstoreDefs,
	"Kiro Crew": {
		{Key: "crew_sessions", Label: "Crew から動かした会話", LabelEn: "Conversations run from Crew", Unit: "件", Agg: "sum"},
		{Key: "crew_subagents", Label: "うちサブエージェント", LabelEn: "Of which subagents", Unit: "件", Agg: "sum"},
		{Key: "credits", Label: "クレジット", LabelEn: "Credits", Unit: "クレジット", Agg: "sum"},
		{Key: "turns", Label: "ターン", LabelEn: "Turns", Unit: "回", Agg: "sum"},
	},
	"Codex": {
		{Key: "responses", Label: "応答の数", LabelEn: "Responses", Unit: "回", Agg: "sum"},
		{Key: "reasoning", Label: "推論トークン", LabelEn: "Reasoning tokens", Unit: "トークン", Agg: "sum"},
		{Key: "reasoning_ratio", Label: "出力のうち推論の割合", LabelEn: "Share of output spent on reasoning", Unit: "%", Agg: "ratio", Num: "reasoning", Den: "output", Scale: 100},
		{Key: "context_used", Label: "コンテキストの最大使用率", LabelEn: "Peak context usage", Unit: "%", Agg: "max", Scale: 100},
		// 利用上限の枠の使用率（%）。枠の長さ（window_minutes）ごとに分け、わからないものは primary / secondary で出す
		{Key: "rate_limit_5h", Label: "5時間枠の最大使用率", LabelEn: "Peak 5-hour limit usage", Unit: "%", Agg: "max"},
		{Key: "rate_limit_daily", Label: "1日枠の最大使用率", LabelEn: "Peak daily limit usage", Unit: "%", Agg: "max"},
		{Key: "rate_limit_weekly", Label: "週の枠の最大使用率", LabelEn: "Peak weekly limit usage", Unit: "%", Agg: "max"},
		{Key: "rate_limit_monthly", Label: "月の枠の最大使用率", LabelEn: "Peak monthly limit usage", Unit: "%", Agg: "max"},
		{Key: "rate_limit_annual", Label: "年の枠の最大使用率", LabelEn: "Peak annual limit usage", Unit: "%", Agg: "max"},
		{Key: "rate_limit", Label: "レート制限の最大使用率", LabelEn: "Peak rate-limit usage", Unit: "%", Agg: "max"},
		{Key: "rate_limit_secondary", Label: "2つめのレート制限の最大使用率", LabelEn: "Peak secondary rate-limit usage", Unit: "%", Agg: "max"},
		{Key: "ttft", Label: "最初のトークンまでの時間（中央値）", LabelEn: "Time to first token (median)", Unit: "秒", Agg: "median"},
		{Key: "turn_duration", Label: "ターンにかかった時間（中央値）", LabelEn: "Turn duration (median)", Unit: "秒", Agg: "median"},
		{Key: "tool_calls", Label: "ツール呼び出し", LabelEn: "Tool calls", Unit: "回", Agg: "sum"},
	},
}

var qstoreDefs = []NativeDef{
	{Key: "ttfc", Label: "最初の返事までの時間（中央値）", LabelEn: "Time to first reply (median)", Unit: "秒", Agg: "median"},
	{Key: "latency", Label: "応答にかかった時間（中央値）", LabelEn: "Response time (median)", Unit: "秒", Agg: "median"},
	{Key: "response_size", Label: "応答の大きさ（平均）", LabelEn: "Response size (average)", Unit: "バイト", Agg: "avg"}, // 応答の文とツール入力の JSON のバイト数（文字数ではない）,
	{Key: "tool_calls", Label: "ツール呼び出し", LabelEn: "Tool calls", Unit: "回", Agg: "sum"},
}

// NativeValue は 1 つの参考指標の値。
type NativeValue struct {
	Label   string  `json:"label"`
	LabelEn string  `json:"labelEn"` // 英語表示のときのラベル
	Unit    string  `json:"unit"`
	V       float64 `json:"v"`
	N       int     `json:"n"` // 元になった観測の数
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
		out = append(out, NativeValue{Label: d.Label, LabelEn: d.LabelEn, Unit: d.Unit, V: Round(v, 2), N: n})
	}
	return out
}
