package cli

import (
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// loaded は、Source が出した 1 つの会話（Finish したもの。時刻がなければ sess は nil）。
type loaded struct {
	key          string // core.Builder.Key（同じ会話を 2 度数えないため）
	claim, yield string // core.Builder.Claim と Yield
	sess         *core.Session
	format       *core.Format // core.Builder.Format（読んだファイルの形の手がかり）
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
	scans   map[source.ClaimScanner]scanCache // 選んでいない Source の ScanWithheldClaims の結果
	disk    *diskCache                        // 前に起動したときの結果（diskcache.go。nil なら使わない）
	dirty   bool                              // ディスクに残したものから変わった（読み直した Unit・消えた Unit がある）
	// 中身を出さない会話（Kiro Crew の incognito・temporary）の写し。ディスクには残さない（collectCached が入れる）
	withheld, withheldClaims map[string]bool
	unsure                   bool                           // 隠すものを調べきれなかった（ディスクに何も残さず、前に残したものも消す）
	withheldFP               string                         // 前の読み込みで隠した会話の印（変わったらディスクのものを書き直す）
	saved                    map[source.Source]*sourceCache // 読み込みを終えたときの sources（withheld と組で diskSnapshot が使う）
}

// setWithheld は、最後の読み込みで隠した会話を覚える（diskSnapshot が外す）。
func (c *loadCache) setWithheld(withheld, claims map[string]bool, unsure bool) {
	if c == nil {
		return
	}
	keys := make([]string, 0, len(withheld)+len(claims))
	for k := range withheld {
		keys = append(keys, "w\x00"+k)
	}
	for k := range claims {
		keys = append(keys, "c\x00"+k)
	}
	sort.Strings(keys)
	fp := "set\x00" + strings.Join(keys, "\x01")
	c.mu.Lock()
	defer c.mu.Unlock()
	c.withheld, c.withheldClaims, c.unsure = withheld, claims, unsure
	if fp != c.withheldFP { // 隠す会話が変わった（あとから incognito にした会話など）。前に書いたものから外すため書き直す
		c.withheldFP, c.dirty = fp, true
	}
	c.saved = make(map[source.Source]*sourceCache, len(c.sources))
	for s, sc := range c.sources { // 中身は読み直しのたびに作り直すので、書き換わらない
		c.saved[s] = sc
	}
}

// scanCache は ScanWithheldClaims の結果と、そのときの ScanPaths の指紋。
type scanCache struct {
	fp     string
	claims []string
	err    error
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

// saveToDisk は、前にディスクに残してから変わっていれば、裏で書き直す。
func (c *loadCache) saveToDisk() {
	if c == nil || c.disk == nil {
		return
	}
	c.mu.Lock()
	unsure := c.unsure
	dirty := c.dirty && !unsure
	if dirty {
		c.dirty = false
	}
	c.mu.Unlock()
	if unsure { // 前に書いたものに、いまは隠す会話が入っているかもしれないので消す
		c.disk.remove()
		return
	}
	if dirty {
		c.disk.save(func() (*diskFile, error) { return c.diskSnapshot(c.disk.build) })
	}
}

func newLoadCache() *loadCache {
	return &loadCache{sources: map[source.Source]*sourceCache{}, scans: map[source.ClaimScanner]scanCache{}}
}

// scan は s.ScanWithheldClaims。c があれば、調べる場所（ScanPaths）の指紋が前回と同じなら、前回の結果を使う
// （kiroku serve の読み直しのたびに、Crew の会話の記録を全部読み直さないように）。指紋は調べる前に取るので、
// 調べている間に変わったら、次の読み直しで調べ直す。
func (c *loadCache) scan(s source.ClaimScanner) ([]string, error) {
	if c == nil {
		return s.ScanWithheldClaims()
	}
	fp := source.Fingerprint(s.ScanPaths())
	c.mu.Lock()
	prev, ok := c.scans[s]
	c.mu.Unlock()
	if ok && prev.fp == fp {
		return prev.claims, prev.err
	}
	claims, err := s.ScanWithheldClaims()
	c.mu.Lock()
	c.scans[s] = scanCache{fp: fp, claims: claims, err: err}
	c.mu.Unlock()
	return claims, err
}

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
	if prev == nil {
		prev = c.disk.take(s, gap) // 起動して初めて読むときは、前に起動したときの結果（fp は空なので Unit ごとに確かめる）
	}
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
		errs := make([]error, 0, len(units))
		for i, u := range units {
			next.units[u.Key] = ucs[i]
			next.outs = append(next.outs, ucs[i].outs...)
			errs = append(errs, ucs[i].err)
		}
		next.err = source.UnitErrs(errs) // 最初のエラーと、ほかに読めなかった数（読み直さないときと同じ形）
		if len(todo) > 0 || prev == nil || len(prev.units) != len(units) {
			c.mu.Lock()
			c.dirty = true
			c.mu.Unlock()
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
	return loaded{key: b.Key, claim: b.Claim, yield: b.Yield, sess: b.Finish(gap), format: b.Format}
}
