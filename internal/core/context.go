package core

import "sort"

// ContextGrowth は、会話が長くなるにつれて 1 回の応答で読む入力（文脈）がどれだけ大きくなったかを返す。
// 入力 = 新しい入力 + キャッシュへの書き込み + キャッシュからの読み取り。
// [前半 4 分の 1 の中央値, 後半 4 分の 1 の中央値, 最大]。応答が 8 回未満なら空。
func ContextGrowth(evs []Event) []float64 {
	var xs []float64
	for _, e := range evs {
		if in := contextOf(e); in > 0 {
			xs = append(xs, in)
		}
	}
	if len(xs) < 8 {
		return []float64{}
	}
	q := len(xs) / 4
	peak := 0.0
	for _, x := range xs {
		peak = max(peak, x)
	}
	return []float64{median(xs[:q]), median(xs[len(xs)-q:]), peak}
}

func median(xs []float64) float64 {
	ys := append([]float64(nil), xs...)
	sort.Float64s(ys)
	n := len(ys)
	if n%2 == 1 {
		return ys[n/2]
	}
	return (ys[n/2-1] + ys[n/2]) / 2
}

// contextOf は 1 回の応答で読んだ入力（文脈）の大きさ = 新しい入力 + キャッシュへの書き込み + キャッシュからの読み取り。
// 3 つともコンテキストウィンドウに入る。出典: https://platform.claude.com/docs/en/build-with-claude/context-windows
func contextOf(e Event) float64 { return e.U.In + e.U.CW + e.U.CW1h + e.U.CR }

// ContextWindowAsOf は、収録したコンテキストウィンドウの時点。表を更新したら合わせて変える。
const ContextWindowAsOf = "2026-10"

const (
	window200K = 200_000
	window1M   = 1_000_000
)

// ContextWindows は、Claude Code がモデルごとに使うコンテキストウィンドウ（トークン）。モデル ID の先頭一致で引く（長いキーが優先）。
// 出典（2026-10 時点）:
//   - https://platform.claude.com/docs/en/about-claude/models/overview （Fable 5.1・Opus 5.5・Sonnet 5.5 は 1M、Haiku 4.5 は 200K）
//   - https://platform.claude.com/docs/en/build-with-claude/context-windows （Fable・Mythos 5 / 5.1、Opus 4.6 以降、Sonnet 4.6 以降は 1M、
//     ほかのモデルは Sonnet 4.5 も含めて 200K）
//   - https://code.claude.com/docs/en/model-config （Claude Code では Fable 5.1・Fable 5・Sonnet 5 以降・Opus 4.7 以降・Haiku 5.5 が 1M。
//     Opus 4.6・Sonnet 4.6 は [1m] の版を選んだときだけ 1M で、ふつうは 200K）
//
// Opus 4.6・Sonnet 4.6 の [1m] の版は履歴のモデル ID からは見分けられないので 200K とし、200K を超える文脈を読んだ応答があれば
// 1M とみなす（contextWindows）。Bedrock・Google Cloud・Foundry で動かした Opus 4.8 以降や、CLAUDE_CODE_DISABLE_1M_CONTEXT を
// 設定したときは 200K のことがあるが、履歴からは見分けられないので表の値のまま（使用率は低めに出る）。
var ContextWindows = map[string]float64{
	"claude-fable-5-1":  window1M,
	"claude-fable-5":    window1M,
	"claude-mythos-5-1": window1M,
	"claude-mythos-5":   window1M,
	"claude-opus-5-5":   window1M,
	"claude-opus-5":     window1M,
	"claude-opus-4-8":   window1M,
	"claude-opus-4-7":   window1M,
	"claude-opus-4-6":   window200K,
	"claude-opus-4-5":   window200K,
	"claude-opus-4-1":   window200K,
	"claude-opus-4":     window200K,
	"claude-sonnet-5-5": window1M,
	"claude-sonnet-5":   window1M,
	"claude-sonnet-4-6": window200K,
	"claude-sonnet-4-5": window200K,
	"claude-sonnet-4":   window200K,
	"claude-haiku-5-5":  window1M,
	"claude-haiku-4-5":  window200K,
	"claude-3-5-haiku":  window200K,
}

// ContextWindowOf はモデルのコンテキストウィンドウ。表にないモデルは ok=false。
func ContextWindowOf(model string) (float64, bool) { return byModel(ContextWindows, model) }

// contextWindows は、応答に出てくるモデルごとのウィンドウ。表の値を超える文脈を読んだモデルは 1M とみなす
// （Opus 4.6・Sonnet 4.6 の [1m] の版。使用率が 100% を超えて見えないように）。表にないモデルは入れない。
func contextWindows(evs []Event) map[string]float64 {
	w := map[string]float64{}
	for _, e := range evs {
		win, ok := ContextWindowOf(e.Model)
		if !ok {
			continue
		}
		if contextOf(e) > win {
			win = max(win, window1M)
		}
		w[e.Model] = max(w[e.Model], win)
	}
	return w
}

// ContextUsed は、応答ごとのコンテキストの使用率（文脈 ÷ モデルのウィンドウ）を "context_used" の観測にする。
// ウィンドウのわからないモデルと、文脈が 0 の応答は入れない。
func ContextUsed(evs []Event) []Measure {
	w := contextWindows(evs)
	var ms []Measure
	for _, e := range evs {
		if win, in := w[e.Model], contextOf(e); win > 0 && in > 0 {
			ms = append(ms, Measure{Key: "context_used", T: e.T, V: in / win})
		}
	}
	return ms
}

// PeakContextWindow は、文脈がいちばん大きかった応答（ContextGrowth の最大）のモデルのウィンドウ。わからなければ 0。
func PeakContextWindow(evs []Event) float64 { return PeakContextWindowFrom(evs, contextWindows(evs)) }

// PeakContextWindowFrom は PeakContextWindow と同じだが、モデルごとのウィンドウを w から取る
// （ウィンドウを履歴に書くエージェント（Codex の model_context_window）のため）。
func PeakContextWindowFrom(evs []Event, w map[string]float64) float64 {
	peak, win := 0.0, 0.0
	for _, e := range evs {
		if in := contextOf(e); in > peak {
			peak, win = in, w[e.Model]
		}
	}
	return win
}
