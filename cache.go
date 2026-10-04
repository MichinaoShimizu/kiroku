package main

import (
	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// loaded は、Source が出した 1 つの会話（Finish したもの。時刻がなければ sess は nil）。
type loaded struct {
	key  string // core.Builder.Key（同じ会話を 2 度数えないため）
	sess *core.Session
}

// loadCache は、前回読んだ履歴を覚えておき、変わっていないものを読み直さない（kiroku serve 用）。
// エージェントが動いている間は、変わるのはふつう 1 つの会話のファイルだけなので、
// 履歴が長くなっても読み直しが重くならないようにする。
//   - Source ごと: 見張る場所（source.WatchPaths）の指紋が同じなら、前回の結果をそのまま使う
//   - source.Splitter なら Unit ごと: 変わった Unit だけ読み直す
//
// 読んだセッションは画面の集計から書き換えられないので、そのまま使い回せる。
type loadCache struct {
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

// load は s の会話を読む。c が nil なら、いつも全部を読む。
func (c *loadCache) load(s source.Source, gap int) ([]loaded, error) {
	if c == nil {
		return loadAll(s, gap)
	}
	if c.gap != gap { // 区切りの分数が変わると Finish の結果も変わる
		c.gap, c.sources = gap, map[source.Source]*sourceCache{}
	}
	fp := source.Fingerprint(source.WatchPaths(s))
	prev := c.sources[s]
	if prev != nil && prev.fp == fp {
		return prev.outs, prev.err
	}
	next := &sourceCache{fp: fp}
	if sp, ok := s.(source.Splitter); ok {
		next.units = map[string]unitCache{}
		for _, u := range sp.Units() {
			stamp := source.Stamp(u.Files)
			uc, ok := unitCache{}, false
			if prev != nil {
				uc, ok = prev.units[u.Key]
			}
			if !ok || uc.stamp != stamp {
				uc = unitCache{stamp: stamp}
				uc.err = sp.LoadUnit(u, func(b *core.Builder) { uc.outs = append(uc.outs, finish(b, gap)) })
			}
			next.units[u.Key] = uc
			next.outs = append(next.outs, uc.outs...)
			if next.err == nil {
				next.err = uc.err
			}
		}
	} else {
		next.outs, next.err = loadAll(s, gap)
	}
	c.sources[s] = next
	return next.outs, next.err
}

func loadAll(s source.Source, gap int) ([]loaded, error) {
	var outs []loaded
	err := s.Load(func(b *core.Builder) { outs = append(outs, finish(b, gap)) })
	return outs, err
}

func finish(b *core.Builder, gap int) loaded { return loaded{key: b.Key, sess: b.Finish(gap)} }
