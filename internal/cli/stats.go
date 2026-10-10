package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
	"github.com/MichinaoShimizu/kiroku/internal/report"
)

// kiroku stats は、1 週か 1 か月の要点を端末に 1 画面で出す。数字は画面（kiroku serve）の上の数字・週のまとめと同じ集計（report.Summarize）。
// ファイルは書かない。プロジェクト名とモデル名は履歴から来るので、端末の制御文字を取り除いてから出す（termSafe）。
func cmdStats(args []string) error {
	fs := newFS("stats", "stats [flags]")
	c := addCommon(fs)
	week := fs.String("week", "", "show this `week` (this, last, or any date in it such as 2026-10-05); the default is this week")
	month := fs.String("month", "", "show this `month` (this, last, or 2026-09)")
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	now := time.Now()
	if *week == "" && *month == "" {
		*week = "this"
	}
	p, err := parsePeriod(*week, *month, now)
	if err != nil {
		return err
	}
	snap, err := loadNonEmpty(c)
	if err != nil {
		return err
	}
	commits, _ := snap.meta["git"].([]gitlog.Commit)
	return writeStats(os.Stdout, snap.data, commits, p, now)
}

// writeStats は期間の要点を w に書く。期間に記録がなければ、そう書いて終わる（エラーにはしない）。
func writeStats(w io.Writer, data []*core.Session, commits []gitlog.Commit, p *period, now time.Time) error {
	st := styleFor(w)
	s := report.Summarize(data, p.from, p.to, commits...)
	fmt.Fprintf(w, "%s · %s\n\n", st.bold("kiroku"), statsTitle(p, now))
	if s == nil {
		fmt.Fprintf(w, "  No activity in %s.\n", p.label())
		return nil
	}

	// 上の数字（画面の上の数字と同じ並び）
	type row struct{ k, v, note string }
	rows := []row{{"Active time", hm(float64(s.Active)), activeDays(s, p, now)}}
	sub := ""
	if s.Usage.Subagents > 0 {
		sub = " · " + plural(s.Usage.Subagents, "subagent")
	}
	rows = append(rows, row{"Sessions", fmt.Sprint(s.Sessions), plural(s.Prompts, "prompt") + sub})
	if s.Usage.Tokens > 0 {
		note := ""
		if s.Usage.CacheHit != nil {
			note = fmt.Sprintf("%.0f%% cache reads", *s.Usage.CacheHit*100)
		}
		rows = append(rows, row{"Tokens", tokS(s.Usage.Tokens), note})
		note = ""
		if s.CostPerCommit != nil {
			note = fmt.Sprintf("$%.2f per AI commit", *s.CostPerCommit)
		}
		rows = append(rows, row{"Estimated cost", fmt.Sprintf("$%.2f", s.Usage.Cost), note})
	}
	if s.Usage.Credits > 0 {
		rows = append(rows, row{"Kiro credits", credits(s.Usage.Credits), ""})
	}
	if s.Git.Commits > 0 || s.Outputs.PRs > 0 {
		var note []string
		if s.Git.AI > 0 {
			note = append(note, fmt.Sprintf("%d by AI", s.Git.AI))
		}
		if s.Outputs.PRs > 0 {
			note = append(note, plural(int(s.Outputs.PRs), "pull request"))
		}
		rows = append(rows, row{"Git commits", fmt.Sprint(s.Git.Commits), strings.Join(note, " · ")})
	}
	for _, r := range rows {
		// 色の文字は幅に数えないので、値の桁は見かけの長さで合わせる
		fmt.Fprintln(w, strings.TrimRight(fmt.Sprintf("  %-16s%s%s%s", r.k, st.bold(r.v), strings.Repeat(" ", max(1, 10-len([]rune(r.v)))), st.dim(r.note)), " "))
	}

	// エージェントごと（画面の「配分」の帯と同じくくり）
	sessions := map[string]int{}
	for _, d := range data {
		if d.End >= float64(p.from.Unix()) && d.Start < float64(p.to.Unix()) {
			sessions[d.Source]++
		}
	}
	var tbl [][]string
	for _, a := range s.Shares["source"] {
		tbl = append(tbl, []string{termSafe(a.Key, 24), hm(a.Minutes), fmt.Sprint(sessions[a.Key]), orDash(a.Tokens, tokS), orDash(a.Cost, money), orDash(a.Credits, credits)})
	}
	table(w, st, []string{"Agent", "Time", "Sessions", "Tokens", "Cost", "Credits"}, tbl)

	// モデル（トークンの多い順、上位 5）
	if len(s.Usage.Models) > 0 {
		ms := append([][4]any(nil), s.Usage.Models...)
		sort.SliceStable(ms, func(i, j int) bool { return num(ms[i][2]) > num(ms[j][2]) })
		tbl = nil
		for _, m := range ms[:min(5, len(ms))] {
			share := ""
			if s.Usage.Tokens > 0 {
				share = fmt.Sprintf("%.0f%%", num(m[2])*100/s.Usage.Tokens)
			}
			tbl = append(tbl, []string{termSafe(fmt.Sprint(m[0]), 32), tokS(num(m[2])), share, orDash(num(m[1]), money)})
		}
		table(w, st, []string{"Model", "Tokens", "Share", "Cost"}, tbl)
	}

	// プロジェクト（作業時間の多い順、上位 5）
	if len(s.ProjectStats) > 0 {
		ps := append([]report.ProjectStat(nil), s.ProjectStats...)
		sort.SliceStable(ps, func(i, j int) bool { return ps[i].Minutes > ps[j].Minutes })
		tbl = nil
		for _, x := range ps[:min(5, len(ps))] {
			tbl = append(tbl, []string{termSafe(x.Project, 28), hm(x.Minutes), fmt.Sprint(x.Sessions), orDash(x.Cost, money), fmt.Sprint(x.Git.Commits)})
		}
		if len(ps) > 5 {
			tbl = append(tbl, []string{fmt.Sprintf("and %d more", len(ps)-5), "", "", "", ""})
		}
		table(w, st, []string{"Project", "Time", "Sessions", "Cost", "Commits"}, tbl)
	}

	notes := []string{}
	if s.Usage.Tokens > 0 {
		notes = append(notes, "Cost is estimated at public API rates.")
	}
	if s.Usage.Unpriced > 0 {
		notes = append(notes, fmt.Sprintf("%s tokens from models without a known price are not in the cost.", tokS(s.Usage.Unpriced)))
	}
	notes = append(notes, "Project names are shown as they are; check before sharing. Details: kiroku serve")
	fmt.Fprintln(w)
	for _, n := range notes {
		fmt.Fprintln(w, "  "+st.dim(n))
	}
	return nil
}

// statsTitle は「Sep 28 – Oct 4, 2026 (last week)」のような見出し。
func statsTitle(p *period, now time.Time) string {
	if p.mode == "month" {
		rel := ""
		switch p.key {
		case report.MonthOf(float64(now.Unix())).Format("2006-01"):
			rel = " (this month)"
		case report.AddMonths(report.MonthOf(float64(now.Unix())), -1).Format("2006-01"):
			rel = " (last month)"
		}
		return p.from.Format("January 2006") + rel
	}
	end := report.AddDays(p.to, -1)
	rel := ""
	switch mon := report.MondayOf(float64(now.Unix())); {
	case p.from.Equal(mon):
		rel = " (this week)"
	case p.from.Equal(report.AddDays(mon, -7)):
		rel = " (last week)"
	}
	switch {
	case p.from.Year() != end.Year():
		return fmt.Sprintf("%s – %s%s", p.from.Format("Jan 2, 2006"), end.Format("Jan 2, 2006"), rel)
	case p.from.Month() != end.Month():
		return fmt.Sprintf("%s – %s%s", p.from.Format("Jan 2"), end.Format("Jan 2, 2006"), rel)
	}
	return fmt.Sprintf("%s – %d, %d%s", p.from.Format("Jan 2"), end.Day(), end.Year(), rel)
}

// activeDays は「5 of 7 days」。期間がまだ終わっていなければ、今日までの日数で数える（画面の「Active days」と同じ）。
func activeDays(s *report.Summary, p *period, now time.Time) string {
	n, total := 0, len(s.Days)
	for _, d := range s.Days {
		if d.Active > 0 {
			n++
		}
	}
	if now.Before(p.to) && !now.Before(p.from) {
		y, m, d := p.from.Date()
		ny, nm, nd := now.Date()
		total = int(time.Date(ny, nm, nd, 0, 0, 0, 0, time.UTC).Sub(time.Date(y, m, d, 0, 0, 0, 0, time.UTC)).Hours()/24) + 1
		return fmt.Sprintf("%d of %d days so far", n, total)
	}
	return fmt.Sprintf("%d of %d days", n, total)
}

// table は見出しつきの表を書く。1 列目は左寄せ、ほかは右寄せ。幅は見かけの文字数で合わせる。
func table(w io.Writer, st style, head []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	width := make([]int, len(head))
	for i, h := range head {
		width[i] = len([]rune(h))
	}
	for _, r := range rows {
		for i, c := range r {
			width[i] = max(width[i], len([]rune(c)))
		}
	}
	line := func(r []string, f func(string) string) {
		var b strings.Builder
		b.WriteString("  ")
		for i, c := range r {
			pad := strings.Repeat(" ", width[i]-len([]rune(c)))
			if i == 0 {
				b.WriteString(f(c) + pad)
			} else {
				b.WriteString("   " + pad + f(c))
			}
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
	fmt.Fprintln(w)
	line(head, st.dim)
	for _, r := range rows {
		line(r, func(s string) string { return s })
	}
}

// termSafe は、履歴から来た名前を端末にそのまま出せるようにする。制御文字（ESC で始まる色や画面の操作など）と、
// Unicode の書式文字（文字の向きを変える U+202E、幅のない空白など）、行・段落の区切りを取り除き、長い名前は n 文字で切る。
func termSafe(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			return -1
		}
		return r
	}, s)
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	if s == "" {
		return "—"
	}
	return s
}

// hm は分を「33h 14m」「45m」にする。
func hm(min float64) string {
	m := int(min + 0.5)
	if m < 60 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %dm", m/60, m%60)
}

// tokS はトークンを「58.5M」「167K」にする（100M までは小数 1 桁）。
func tokS(v float64) string {
	switch {
	case v >= 1e11:
		return fmt.Sprintf("%.0fB", v/1e9)
	case v >= 1e9:
		return fmt.Sprintf("%.1fB", v/1e9)
	case v >= 1e8:
		return fmt.Sprintf("%.0fM", v/1e6)
	case v >= 1e6:
		return fmt.Sprintf("%.1fM", v/1e6)
	case v >= 1e4:
		return fmt.Sprintf("%.0fK", v/1e3)
	}
	return fmt.Sprintf("%.0f", v)
}

func money(v float64) string   { return fmt.Sprintf("$%.2f", v) }
func credits(v float64) string { return fmt.Sprintf("%.1f cr", v) }

// orDash は、記録のない値（0）を「—」にする。
func orDash(v float64, f func(float64) string) string {
	if v == 0 {
		return "—"
	}
	return f(v)
}

// num は集計の [][4]any に入った数を float64 にする。
func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return 0
}
