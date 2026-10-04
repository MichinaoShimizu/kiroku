package core

import "sort"

// ContextGrowth は、会話が長くなるにつれて 1 回の応答で読む入力（文脈）がどれだけ大きくなったかを返す。
// 入力 = 新しい入力 + キャッシュへの書き込み + キャッシュからの読み取り。
// [前半 4 分の 1 の中央値, 後半 4 分の 1 の中央値, 最大]。応答が 8 回未満なら空。
func ContextGrowth(evs []Event) []float64 {
	var xs []float64
	for _, e := range evs {
		if in := e.U.In + e.U.CW + e.U.CW1h + e.U.CR; in > 0 {
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
