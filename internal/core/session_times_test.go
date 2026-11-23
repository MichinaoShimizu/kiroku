package core

import (
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"
)

// naiveWaits と naivePromptTimes は、前の書き方（依頼ごとに全部を見る）。速くした書き方と同じ結果になることを比べる。
func naiveWaits(agent, ps []float64) [][2]float64 {
	out := [][2]float64{}
	var prev *float64
	for i := range ps {
		p := ps[i]
		last, found := 0.0, false
		for _, a := range agent {
			if (prev == nil || a > *prev) && a < p && (!found || a > last) {
				last, found = a, true
			}
		}
		if found && p-last > 0 && p-last <= 1800 {
			out = append(out, [2]float64{p, Round(p-last, 0)})
		}
		prev = &ps[i]
	}
	return out
}

func naiveNext(ps []Prompt, i int) float64 {
	p := *ps[i].T
	for _, q := range ps[i+1:] {
		if q.T != nil && *q.T > p {
			return *q.T
		}
	}
	return math.Inf(1)
}

func TestPromptTimesMatchNaive(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for round := 0; round < 300; round++ {
		b := &Builder{}
		n := 1 + r.Intn(30)
		var ps []Prompt
		for i := 0; i < n; i++ {
			if r.Intn(8) == 0 {
				ps = append(ps, Prompt{}) // 時刻のない依頼
				continue
			}
			v := float64(1_000_000 + r.Intn(40)*60) // 同じ時刻がよく重なるよう、粗く選ぶ
			ps = append(ps, Prompt{T: &v})
		}
		for i := 0; i < r.Intn(40); i++ {
			b.AgentTimes = append(b.AgentTimes, float64(1_000_000+r.Intn(2400*60)/60*60+r.Intn(60)))
		}
		b.Prompts = ps
		got := b.promptTimes(ps)
		agent := append([]float64(nil), b.AgentTimes...)
		sort.Float64s(agent)
		for i := range ps {
			if ps[i].T == nil {
				continue
			}
			p, next := *ps[i].T, naiveNext(ps, i)
			var work, wait float64
			j := sort.Search(len(agent), func(k int) bool { return agent[k] >= next }) - 1
			if j >= 0 && agent[j] > p {
				work = Round(agent[j]-p, 0)
				if !math.IsInf(next, 1) {
					wait = Round(next-agent[j], 0)
				}
			}
			if got[i].Work != work || got[i].Wait != wait {
				t.Fatalf("round %d prompt %d: work/wait = %v/%v, want %v/%v", round, i, got[i].Work, got[i].Wait, work, wait)
			}
		}
		var sorted []float64
		for _, p := range ps {
			if p.T != nil {
				sorted = append(sorted, *p.T)
			}
		}
		sort.Float64s(sorted)
		want, gotW := naiveWaits(agent, sorted), b.Waits()
		if len(want) != len(gotW) {
			t.Fatalf("round %d: waits = %v, want %v", round, gotW, want)
		}
		for i := range want {
			if want[i] != gotW[i] {
				t.Fatalf("round %d: waits = %v, want %v", round, gotW, want)
			}
		}
	}
}

// 同じ時刻の依頼ばかりのセッション（壊れた・わざと作った履歴）でも、依頼の数にほぼ比例する時間で終わる（セキュリティ）。
// 前は依頼の数の 2 乗かかり、20 MiB の履歴で 50 秒ほど止まっていた。機械の速さに頼らないよう、8 倍にしたときの時間の比で確かめる。
func TestPromptTimesSameTimestampScales(t *testing.T) {
	run := func(n int) time.Duration {
		b := &Builder{}
		v := 1_000_000.0
		for i := 0; i < n; i++ {
			b.Prompts = append(b.Prompts, Prompt{T: &v})
			b.AgentTimes = append(b.AgentTimes, v)
		}
		best := time.Duration(math.MaxInt64)
		for k := 0; k < 5; k++ {
			start := time.Now()
			b.promptTimes(b.Prompts)
			b.Waits()
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}
	// 8 倍にする。線形なら約 8 倍（n log n でも 10 倍ほど）、2 乗なら 64 倍。どちらも時計の粗さ（Windows）より十分長くなる数にし、
	// 小さいほうがキャッシュに収まって速く出ることもあるので、下限を置いて余裕のある閾値で比べる
	small, large := run(50_000), run(400_000)
	// 下限は 5ms。50k が 3.5ms ほどで終わる速い機械（Windows の CI）では、400k だけがキャッシュに収まらず 32 倍ほどになることがある。
	// 2 乗の計算なら 50k でも秒の単位になるので、下限を上げても見逃さない
	if small < 5*time.Millisecond {
		small = 5 * time.Millisecond
	}
	t.Logf("50k: %v, 400k: %v", small, large)
	if large > 30*small {
		t.Errorf("50k prompts: %v, 400k prompts: %v (more than 30x)", small, large)
	}
}
