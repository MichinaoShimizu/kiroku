package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// period は kiroku html --week / --month で選んだ期間（ローカル時刻の from 以上 to 未満）。
type period struct {
	mode     string // "week" か "month"
	key      string // 週は月曜の日付（2026-10-05）、月は 2026-10
	from, to time.Time
}

// localDay は日付（YYYY-MM-DD か YYYY-MM）を、ローカル時刻のその日の始まりにする。
// time.ParseInLocation だと、0 時のない日（夏時間が 0 時に始まる地域）は前の日の 23 時になる。
func localDay(layout, s string) (time.Time, error) {
	d, err := time.Parse(layout, s)
	if err != nil {
		return d, err
	}
	return report.Midnight(d.Year(), d.Month(), d.Day(), time.Local), nil
}

// parsePeriod は --week（this・last・その週のどれかの日 YYYY-MM-DD）と --month（this・last・YYYY-MM）を読む。
// どちらもなければ nil。now はテストで差しかえる。
func parsePeriod(week, month string, now time.Time) (*period, error) {
	if week != "" && month != "" {
		return nil, fmt.Errorf("choose either --week or --month, not both")
	}
	t := float64(now.Unix())
	switch {
	case week != "":
		var w time.Time
		switch week {
		case "this":
			w = report.MondayOf(t)
		case "last":
			w = report.AddDays(report.MondayOf(t), -7)
		default:
			d, err := localDay("2006-01-02", week)
			if err != nil {
				return nil, fmt.Errorf("--week takes this, last or a date such as 2026-10-05: %s", week)
			}
			w = report.MondayOf(float64(d.Unix()))
		}
		return &period{mode: "week", key: w.Format("2006-01-02"), from: w, to: report.AddDays(w, 7)}, nil
	case month != "":
		var m time.Time
		switch month {
		case "this":
			m = report.MonthOf(t)
		case "last":
			m = report.AddMonths(report.MonthOf(t), -1)
		default:
			d, err := localDay("2006-01", month)
			if err != nil {
				return nil, fmt.Errorf("--month takes this, last or a month such as 2026-09: %s", month)
			}
			m = d
		}
		return &period{mode: "month", key: m.Format("2006-01"), from: m, to: report.AddMonths(m, 1)}, nil
	}
	return nil, nil
}

// file は、-o を書かなかったときの出力先（kiroku-2026-10-05.html など）。
func (p *period) file() string { return "kiroku-" + p.key + ".html" }

// label は案内に出す期間の名前。
func (p *period) label() string {
	if p.mode == "week" {
		return "the week of " + p.key
	}
	return p.from.Format("January 2006")
}

// scoped は、期間に重なるセッションと、期間のうちのコミット・push だけを残した集計にする。
// 人に渡す HTML に、ほかの期間のプロンプトやコミットを入れないため。期間をまたぐセッションは丸ごと入る。
func scoped(snap snapshot, p *period) snapshot {
	from, to := float64(p.from.Unix()), float64(p.to.Unix())
	data := []*core.Session{}
	for _, d := range snap.data {
		if d.Start < to && d.End >= from {
			data = append(data, d)
		}
	}
	commits := []gitlog.Commit{}
	if cs, ok := snap.meta["git"].([]gitlog.Commit); ok {
		for _, c := range cs {
			if c.T >= from && c.T < to {
				commits = append(commits, c)
			}
		}
	}
	pushes := []gitlog.Push{}
	if ps, ok := snap.meta["push"].([]gitlog.Push); ok {
		for _, x := range ps {
			if x.T >= from && x.T < to {
				pushes = append(pushes, x)
			}
		}
	}
	// 週・月の集計は、残したものだけで作り直し、期間に重なる週・月だけを残す
	weeks := report.AllWeeks(data, commits...)
	for k := range weeks {
		w, _ := localDay("2006-01-02", k)
		if !w.Before(p.to) || !report.AddDays(w, 7).After(p.from) {
			delete(weeks, k)
		}
	}
	months := report.AllMonths(data, commits...)
	for k := range months {
		m, _ := localDay("2006-01", k)
		if !m.Before(p.to) || !report.AddMonths(m, 1).After(p.from) {
			delete(months, k)
		}
	}
	// 「Data sources」の件数も期間の分だけにし、いちばん古い記録と保存期間の設定は出さない（渡す相手には関係がなく、ほかの期間のことを渡さないため）
	in := map[string]*core.Session{}
	for _, d := range data {
		in[d.ID] = d
	}
	rep := make([]source.Report, len(snap.rep))
	for i, r := range snap.rep {
		r.N, r.Dup, r.Archived, r.Oldest, r.Keep = 0, 0, 0, 0, nil
		for _, id := range r.IDs {
			d := in[id]
			if d == nil {
				continue
			}
			r.N++
			if strings.HasSuffix(d.File, ".zst") && snap.rep[i].Archived > 0 {
				r.Archived++
			}
		}
		rep[i] = r
	}
	kept := rep[:0]
	for _, r := range rep {
		if r.N > 0 || r.Error != nil { // 使っていないエージェントは、渡す相手には関係がない
			kept = append(kept, r)
		}
	}
	rep = kept
	meta := map[string]any{}
	for k, v := range snap.meta {
		meta[k] = v
	}
	meta["report"] = rep
	delete(meta, "archive") // kiroku archive の保存場所と量も、渡す相手には関係がない
	meta["git"], meta["push"] = commits, pushes
	// 週・月・日の区切りは書き出した人の時間帯なので、ほかの時間帯で開いても、画面はこの時計で見せる（デモと同じしくみ）
	zone, off := p.from.Zone()
	meta["scope"] = map[string]any{"mode": p.mode, "key": p.key, "from": from, "to": to, "offset": off, "zone": zone}
	snap.data, snap.weeks, snap.months, snap.meta, snap.rep = data, weeks, months, meta, rep
	return snap
}
