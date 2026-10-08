package report

import (
	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
)

// Cache は、前回の週と月の集計を覚えておき、セッションもコミットも変わっていない期間を集計し直さない
// （kiroku serve 用）。エージェントが動いている間に変わるのはふつう今週と今月だけなので、履歴が長くなっても
// 読み直しのたびの集計が重くならないようにする。
//
// セッションはポインタで比べる。kiroku serve の読み込み（loadCache）は、変わっていない履歴のセッションを
// 同じポインタのまま返し、読み直したものは新しく作る。セッションは集計から書きかえない。
// Cache は前回のセッションを持ち続けるので、解放されたあとで同じ場所に作られた別のセッションと取り違えることはない。
// 同時には使えない（kiroku serve の読み直しは 1 本ずつ）。
type Cache struct {
	weeks, months map[string]*memoEntry
}

func NewCache() *Cache {
	return &Cache{weeks: map[string]*memoEntry{}, months: map[string]*memoEntry{}}
}

// AllWeeks は report.AllWeeks と同じ結果を返す。
func (c *Cache) AllWeeks(data []*core.Session, commits ...gitlog.Commit) map[string]*Week {
	return allPeriods(data, commits, weekly, c.weeks)
}

// AllMonths は report.AllMonths と同じ結果を返す。
func (c *Cache) AllMonths(data []*core.Session, commits ...gitlog.Commit) map[string]*Summary {
	return allPeriods(data, commits, monthly, c.months)
}

// memoEntry は 1 つの期間の集計と、そのもとになったセッション・コミット。
type memoEntry struct {
	sessions []*core.Session
	commits  []gitlog.Commit
	sum      *Summary // 動いていた時間がなければ nil
}

// same は、期間のセッションとコミットが前回と同じか。コミットは Summarize が見るものだけを比べる。
func (e *memoEntry) same(sessions []*core.Session, commits []gitlog.Commit) bool {
	if len(sessions) != len(e.sessions) || len(commits) != len(e.commits) {
		return false
	}
	for i, s := range sessions {
		if s != e.sessions[i] {
			return false
		}
	}
	for i, c := range commits {
		p := e.commits[i]
		if c.T != p.T || c.Project != p.Project || c.AI != p.AI || c.Added != p.Added || c.Removed != p.Removed {
			return false
		}
	}
	return true
}
