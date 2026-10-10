package cli

import (
	"runtime"
	"sync"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// loaded は、Source が出した 1 つの会話（Finish したもの。時刻がなければ sess は nil）。
type loaded struct {
	key          string // core.Builder.Key（同じ会話を 2 度数えないため）
	claim, yield string // core.Builder.Claim と Yield
	sess         *core.Session
}

// loadCache は、前回読んだ履歴を覚えておき、変わっていないものを読み直さない（kiroku serve 用）。
// エージェントが動いている間は、変わるのはふつう 1 つの会話のファイルだけなので、
// 履歴が長くなっても読み直しが重くならないようにする。
//   - Source ごと: 見張る場所（source.WatchPaths）の指紋が同じなら、前回の結果をそのまま使う
//   - source.Splitter なら Unit ごと: 変わった Unit だけ読み直す
//
// 読んだセッションは画面の集計から書き換えられないので、そのまま使い回せる。
type loadCache struct {
	mu      sync.Mutex // エージェントごとに並べて読むので、sources を守る
	gap     int
	sources map[source.Source]*sourceCache
}

type sourceCache struct {
	fp    string
	outs  []loaded
	err   error
	units map[string]unitCache
}

type unitCache struct {
	stamp string
	outs  []loaded
	err   error
}

func newLoadCache() *loadCache { return &loadCache{sources: map[source.Source]*sourceCache{}} }

// load は s の会話を読む。c が nil なら、いつも全部を読む。ほかのエージェントの load と同時に呼んでよい。
func (c *loadCache) load(s source.Source, gap int) ([]loaded, error) {
	if c == nil {
		return loadAll(s, gap)
	}
	fp := source.Fingerprint(source.WatchPaths(s))
	c.mu.Lock()
	if c.gap != gap { // 区切りの分数が変わると Finish の結果も変わる
		c.gap, c.sources = gap, map[source.Source]*sourceCache{}
	}
	prev := c.sources[s]
	c.mu.Unlock()
	if prev != nil && prev.fp == fp {
		return prev.outs, prev.err
	}
	next := &sourceCache{fp: fp}
	if sp, ok := s.(source.Splitter); ok {
		next.units = map[string]unitCache{}
		units := sp.Units()
		ucs := make([]unitCache, len(units))
		var todo []int // 変わった Unit（読み直すもの）
		for i, u := range units {
			ucs[i].stamp = source.Stamp(u.Files) + "\x00" + u.Tag
			if prev != nil {
				if uc, ok := prev.units[u.Key]; ok && uc.stamp == ucs[i].stamp {
					ucs[i] = uc
					continue
				}
			}
			todo = append(todo, i)
		}
		parallel(len(todo), func(j int) {
			i := todo[j]
			ucs[i].err = sp.LoadUnit(units[i], func(b *core.Builder) { ucs[i].outs = append(ucs[i].outs, finish(b, gap)) })
		})
		for i, u := range units {
			next.units[u.Key] = ucs[i]
			next.outs = append(next.outs, ucs[i].outs...)
			if next.err == nil {
				next.err = ucs[i].err
			}
		}
	} else {
		next.outs, next.err = loadAll(s, gap)
	}
	c.mu.Lock()
	c.sources[s] = next
	c.mu.Unlock()
	return next.outs, next.err
}

// loadAll は s の会話を全部読む。Unit ごとに読める Source は、Unit を並べて読む（出す順は Load と同じ）。
func loadAll(s source.Source, gap int) ([]loaded, error) {
	sp, ok := s.(source.Splitter)
	if !ok {
		var outs []loaded
		err := s.Load(func(b *core.Builder) { outs = append(outs, finish(b, gap)) })
		return outs, err
	}
	units := sp.Units()
	per := make([][]loaded, len(units))
	errs := make([]error, len(units))
	parallel(len(units), func(i int) {
		errs[i] = sp.LoadUnit(units[i], func(b *core.Builder) { per[i] = append(per[i], finish(b, gap)) })
	})
	var outs []loaded
	for _, o := range per {
		outs = append(outs, o...)
	}
	return outs, source.UnitErrs(errs)
}

// parallel は f(0)…f(n-1) を、CPU の数だけ並べて呼ぶ（会話のファイルは互いに関係なく読めるので、履歴が多いと速くなる）。
// 結果は呼ぶ側が i ごとの場所に入れるので、順番は変わらない。
func parallel(n int, f func(i int)) {
	workers := min(runtime.GOMAXPROCS(0), n)
	if workers <= 1 {
		for i := range n {
			f(i)
		}
		return
	}
	var next sync.Mutex
	i := 0
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				next.Lock()
				j := i
				i++
				next.Unlock()
				if j >= n {
					return
				}
				f(j)
			}
		}()
	}
	wg.Wait()
}

func finish(b *core.Builder, gap int) loaded {
	return loaded{key: b.Key, claim: b.Claim, yield: b.Yield, sess: b.Finish(gap)}
}
