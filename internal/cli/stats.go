package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
	"github.com/MichinaoShimizu/kiroku/internal/report"
)

// cmdStats は kiroku stats。1 日・1 週・1 か月の集計を、前の期間と並べて端末に出す（ブラウザを開かずに見る）。
// 進行中の期間は、前の期間の同じところ（同じ曜日・同じ日付）までと比べる。
func cmdStats(args []string) error {
	fs := newFS("stats", "stats [flags]\n\nShows a day, week or month of your AI work in the terminal, next to the period before.\nWith none of --day, --week or --month it shows this week. A period still in progress\nis compared with the period before up to the same day.")
	c := addCommon(fs)
	day := fs.String("day", "", "show this `day` (today, yesterday, or a date such as 2026-10-05)")
	week := fs.String("week", "", "show this `week` (this, last, or any date in it such as 2026-10-05)")
	month := fs.String("month", "", "show this `month` (this, last, or 2026-09)")
	project := fs.String("project", "", "only these comma-separated `projects` (as named in the view; case does not matter)")
	top := fs.Int("top", 5, "how many `rows` to list for projects, models, sessions and the rest")
	asJSON := fs.Bool("json", false, "print the two periods as JSON instead of text")
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	if *top < 1 {
		return fmt.Errorf("--top must be 1 or more: %d", *top)
	}
	now := time.Now()
	sp, err := parseStatsPeriod(*day, *week, *month, now)
	if err != nil {
		return errors.New(clean(err.Error())) // 引数をそのまま返すので、制御文字を端末に出さない
	}
	if *asJSON {
		logw = io.Discard // 標準出力を JSON だけにする
	}
	snap, err := loadNonEmpty(c)
	if err != nil {
		return err
	}
	data, commits := snap.data, snapCommits(snap)
	if *project != "" {
		data, commits, err = onlyProjects(data, commits, splitList(*project))
		if err != nil {
			return err
		}
	}
	cur := report.Summarize(data, sp.from, sp.to, commits...)
	prev := report.Summarize(data, sp.prevFrom, sp.prevTo, commits...)
	if *asJSON {
		return writeStatsJSON(os.Stdout, sp, cur, prev)
	}
	writeStats(os.Stdout, styleFor(os.Stdout), sp, cur, prev, *top)
	return nil
}

// statsPeriod は kiroku stats で見る期間と、比べる前の期間（どちらもローカル時刻の 0 時から 0 時まで）。
type statsPeriod struct {
	mode             string // "day"・"week"・"month"
	key              string // 2026-10-05、2026-10
	from, to         time.Time
	prevFrom, prevTo time.Time
	partial          bool // 期間がまだ終わっていない（今日を含む）。前の期間も同じ日数までにしてある
}

// parseStatsPeriod は --day・--week・--month を読む。どれもなければ今週。now はテストで差しかえる。
func parseStatsPeriod(day, week, month string, now time.Time) (*statsPeriod, error) {
	n := 0
	for _, s := range []string{day, week, month} {
		if s != "" {
			n++
		}
	}
	if n > 1 {
		return nil, fmt.Errorf("choose one of --day, --week or --month")
	}
	var sp statsPeriod
	switch {
	case day != "":
		var d time.Time
		today := report.Midnight(now.Year(), now.Month(), now.Day(), time.Local)
		switch day {
		case "today":
			d = today
		case "yesterday":
			d = report.AddDays(today, -1)
		default:
			var err error
			if d, err = localDay("2006-01-02", day); err != nil {
				return nil, fmt.Errorf("--day takes today, yesterday or a date such as 2026-10-05: %s", day)
			}
		}
		sp = statsPeriod{mode: "day", key: d.Format("2006-01-02"), from: d, to: report.AddDays(d, 1), prevFrom: report.AddDays(d, -1), prevTo: d}
	default:
		if week == "" && month == "" {
			week = "this"
		}
		p, err := parsePeriod(week, month, now)
		if err != nil {
			return nil, err
		}
		sp = statsPeriod{mode: p.mode, key: p.key, from: p.from, to: p.to, prevTo: p.from}
		if p.mode == "week" {
			sp.prevFrom = report.AddDays(p.from, -7)
		} else {
			sp.prevFrom = report.AddMonths(p.from, -1)
		}
	}
	// 進行中の期間は、前の期間も同じ日数（今日まで）にそろえて比べる。1 日のときは前の日をまるごと使う
	if !now.Before(sp.from) && now.Before(sp.to) {
		sp.partial = true
		if sp.mode != "day" {
			elapsed := 1
			for d := sp.from; report.AddDays(d, 1).Before(now) || report.AddDays(d, 1).Equal(now); d = report.AddDays(d, 1) {
				elapsed++
			}
			if end := report.AddDays(sp.prevFrom, elapsed); end.Before(sp.prevTo) {
				sp.prevTo = end
			}
		}
	}
	return &sp, nil
}

// title は見出しに出す期間の名前。
func (sp *statsPeriod) title() string {
	switch sp.mode {
	case "day":
		return sp.from.Format("Mon Jan 2, 2006")
	case "week":
		last := report.AddDays(sp.to, -1)
		return "Week of " + sp.from.Format("Jan 2") + " – " + last.Format("Jan 2, 2006")
	}
	return sp.from.Format("January 2006")
}

// versus は比べる前の期間の名前。
func (sp *statsPeriod) versus() string {
	switch sp.mode {
	case "day":
		return "vs " + sp.prevFrom.Format("Mon Jan 2")
	case "week":
		if sp.partial {
			return "vs last week to " + report.AddDays(sp.prevTo, -1).Format("Mon")
		}
		return "vs the week before"
	}
	if sp.partial {
		return "vs " + sp.prevFrom.Format("January") + " to the " + ordinal(report.AddDays(sp.prevTo, -1).Day())
	}
	return "vs " + sp.prevFrom.Format("January")
}

func ordinal(n int) string {
	suf := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suf = "st"
		case 2:
			suf = "nd"
		case 3:
			suf = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suf)
}

// snapCommits は集計に入っている git のコミット。
func snapCommits(snap snapshot) []gitlog.Commit {
	cs, _ := snap.meta["git"].([]gitlog.Commit)
	return cs
}

// onlyProjects は、名前が names のどれかと一致する（大文字・小文字は区別しない）プロジェクトのセッションとコミットだけを残す。
func onlyProjects(data []*core.Session, commits []gitlog.Commit, names []string) ([]*core.Session, []gitlog.Commit, error) {
	want := map[string]bool{}
	for _, n := range names {
		want[strings.ToLower(n)] = true
	}
	var ds []*core.Session
	for _, d := range data {
		if want[strings.ToLower(d.Project)] {
			ds = append(ds, d)
		}
	}
	if len(ds) == 0 {
		return nil, nil, fmt.Errorf("no history for the project %s; recent projects: %s", nameList(names), nameList(recentProjects(data, 8)))
	}
	var cs []gitlog.Commit
	for _, c := range commits {
		if want[strings.ToLower(c.Project)] {
			cs = append(cs, c)
		}
	}
	return ds, cs, nil
}

// recentProjects は、最後に動いていたのが新しい順に、プロジェクトの名前を n 個まで返す。
func recentProjects(data []*core.Session, n int) []string {
	last := map[string]float64{}
	for _, d := range data {
		last[d.Project] = max(last[d.Project], d.End)
	}
	names := make([]string, 0, len(last))
	for p := range last {
		names = append(names, p)
	}
	sort.Slice(names, func(i, j int) bool {
		if last[names[i]] != last[names[j]] {
			return last[names[i]] > last[names[j]]
		}
		return names[i] < names[j]
	})
	return limit(names, n)
}

// statsJSONSchema は kiroku stats --json の形の版。項目を消したり、名前や意味を変えたりしたら上げる。
const statsJSONSchema = 1

func writeStatsJSON(w io.Writer, sp *statsPeriod, cur, prev *report.Summary) error {
	day := func(t time.Time) string { return t.Format("2006-01-02") }
	b, err := json.MarshalIndent(map[string]any{
		"schemaVersion": statsJSONSchema,
		"period": map[string]any{"mode": sp.mode, "key": sp.key, "from": day(sp.from), "to": day(sp.to), "partial": sp.partial,
			"previousFrom": day(sp.prevFrom), "previousTo": day(sp.prevTo)},
		"current": cur, "previous": prev,
	}, "", " ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// writeStats は 2 つの期間を、端末で読む表にする。履歴と git から来た文字列（プロジェクト・タイトル・モデル）は
// 端末を操作できないよう、clean で制御文字を除いてから出す。
func writeStats(w io.Writer, st style, sp *statsPeriod, cur, prev *report.Summary, top int) {
	head := sp.title()
	if sp.partial {
		head += " (so far)"
	}
	fmt.Fprintf(w, "%s  %s\n", st.bold(head), st.dim(sp.versus()))
	if cur == nil {
		fmt.Fprintln(w, "\n  No AI activity in this period.")
		if prev != nil {
			fmt.Fprintf(w, "  %s\n", st.dim(fmt.Sprintf("The period before had %s of active time in %s.", dur(float64(prev.Active)), plural(prev.Sessions, "session"))))
		}
		return
	}
	var p report.Summary // 前の期間に動きがなければ、数は 0 と比べる（割合や 1 コミットあたりは比べない）
	if prev != nil {
		p = *prev
	}

	// 主な数字
	fmt.Fprintln(w)
	row := func(label, val, delta string) {
		fmt.Fprintln(w, strings.TrimRight(fmt.Sprintf("  %-15s %14s  %s", label, val, st.dim(delta)), " "))
	}
	row("Active time", dur(float64(cur.Active)), deltaDur(float64(cur.Active), float64(p.Active), true))
	row("Sessions", count(float64(cur.Sessions)), deltaNum(float64(cur.Sessions), float64(p.Sessions), true, count))
	row("Prompts", count(float64(cur.Prompts)), deltaNum(float64(cur.Prompts), float64(p.Prompts), true, count))
	if cur.Usage.Tokens > 0 || p.Usage.Tokens > 0 {
		row("Tokens", tokens(cur.Usage.Tokens), deltaNum(cur.Usage.Tokens, p.Usage.Tokens, true, tokens))
	}
	if cur.Usage.Cost > 0 || p.Usage.Cost > 0 {
		row("Est. cost", money(cur.Usage.Cost), deltaNum(cur.Usage.Cost, p.Usage.Cost, true, money))
	}
	if cur.Usage.Credits > 0 || p.Usage.Credits > 0 {
		row("Kiro credits", fmt.Sprintf("%.1f", cur.Usage.Credits), deltaNum(cur.Usage.Credits, p.Usage.Credits, true, func(v float64) string { return fmt.Sprintf("%.1f", v) }))
	}
	if cur.Git.Commits > 0 || p.Git.Commits > 0 {
		row("Git commits", count(float64(cur.Git.Commits)), deltaNum(float64(cur.Git.Commits), float64(p.Git.Commits), true, count))
		fmt.Fprintf(w, "  %-15s %14s  %s\n", "", st.dim(fmt.Sprintf("%d by AI", cur.Git.AI)), st.dim(fmt.Sprintf("+%s −%s lines", count(float64(cur.Git.Added)), count(float64(cur.Git.Removed)))))
	}
	if cur.CostPerCommit != nil {
		var pv float64
		if p.CostPerCommit != nil {
			pv = *p.CostPerCommit
		}
		row("Cost / commit", money(*cur.CostPerCommit), deltaNum(*cur.CostPerCommit, pv, p.CostPerCommit != nil, money))
	}
	if cur.FixRate != nil {
		var pv float64
		if p.FixRate != nil {
			pv = *p.FixRate
		}
		row("Rework rate", fmt.Sprintf("%.0f%%", *cur.FixRate), deltaNum(*cur.FixRate, pv, p.FixRate != nil, func(v float64) string { return fmt.Sprintf("%.0f pt", v) }))
	}
	if cur.MaxConc >= 2 {
		row("In parallel", dur(float64(cur.Parallel)), fmt.Sprintf("up to %d sessions at once", cur.MaxConc))
	}

	// 日ごとの帯（週と月だけ）
	if sp.mode != "day" && len(cur.Days) > 0 {
		fmt.Fprintf(w, "\n%s\n", st.bold("Days"))
		most := 0
		for _, d := range cur.Days {
			most = max(most, d.Active)
		}
		for i, d := range cur.Days {
			t := report.AddDays(sp.from, i)
			if sp.partial && t.After(time.Now()) {
				break
			}
			label := t.Format("Mon 01/02")
			if d.Active == 0 {
				fmt.Fprintf(w, "  %s  %s\n", label, st.dim("·"))
				continue
			}
			extra := []string{}
			if d.Cost > 0 {
				extra = append(extra, money(d.Cost))
			}
			if d.Commits > 0 {
				extra = append(extra, plural(d.Commits, "commit"))
			}
			fmt.Fprintln(w, strings.TrimRight(fmt.Sprintf("  %s  %-20s %8s  %s", label, bar(float64(d.Active), float64(most), 20), dur(float64(d.Active)), st.dim(strings.Join(extra, " · "))), " "))
		}
	}

	// プロジェクト
	if len(cur.ProjectStats) > 0 {
		fmt.Fprintf(w, "\n%s\n", st.bold("Projects"))
		fmt.Fprintf(w, "  %s\n", st.dim(fmt.Sprintf("%-24s %8s %8s %8s %10s %8s", "", "time", "sessions", "prompts", "cost", "commits")))
		for _, ps := range limit(cur.ProjectStats, top) {
			fmt.Fprintf(w, "  %s %8s %8d %8d %10s %8s\n", fit(ps.Project, 24), dur(ps.Minutes), ps.Sessions, ps.Prompts, moneyOr(ps.Cost), countOr(ps.Git.Commits))
		}
		more(w, st, len(cur.ProjectStats), top, "project")
	}

	// エージェント
	if src := cur.Shares["source"]; len(src) > 1 {
		fmt.Fprintf(w, "\n%s\n", st.bold("Agents"))
		for _, s := range src {
			fmt.Fprintf(w, "  %s %8s %10s\n", fit(s.Key, 24), dur(s.Minutes), moneyOr(s.Cost))
		}
	}

	// モデル
	if len(cur.Usage.Models) > 0 {
		fmt.Fprintf(w, "\n%s\n", st.bold("Models"))
		for _, m := range limit(cur.Usage.Models, top) {
			name, _ := m[0].(string)
			fmt.Fprintf(w, "  %s %10s %10s\n", fit(name, 24), tokens(num(m[2])), moneyOr(num(m[1])))
		}
		more(w, st, len(cur.Usage.Models), top, "model")
		if cur.Usage.Unpriced > 0 {
			fmt.Fprintf(w, "  %s\n", st.warn(fmt.Sprintf("! %s tokens of models without a price are not in the cost: %s", tokens(cur.Usage.Unpriced), nameList(limit(cur.Usage.UnpricedM, top)))))
		}
	}

	// 長く動いていたセッション
	var ses []report.TopSession
	for _, ps := range cur.ProjectStats {
		ses = append(ses, ps.Top...)
	}
	sort.SliceStable(ses, func(i, j int) bool { return ses[i].Minutes > ses[j].Minutes })
	if len(ses) > 0 {
		fmt.Fprintf(w, "\n%s\n", st.bold("Longest sessions"))
		for _, s := range limit(ses, top) {
			fmt.Fprintf(w, "  %s  %s %8s  %s\n", time.Unix(int64(s.Start), 0).Format("01/02 15:04"), fit(titleOf(s.Title), 44), dur(s.Minutes), st.dim(clean(s.Source)))
		}
	}

	// 手間がかかったセッション（言い直し・中断・依頼の多さ）
	if len(cur.Friction) > 0 {
		fmt.Fprintf(w, "\n%s\n", st.bold("Worth a look"))
		for _, f := range limit(cur.Friction, top) {
			fmt.Fprintf(w, "  %s %s  %s\n", st.warn("!"), fit(titleOf(f.Title), 44), st.dim(strings.TrimSpace(fit(f.Project, 24))+" · "+strings.Join(f.WhyEn, ", ")))
		}
	}
	fmt.Fprintf(w, "\n%s\n", st.dim("Estimated cost is at public API rates. Open the full view with: kiroku serve"))
}

func titleOf(t string) string {
	if strings.TrimSpace(t) == "" {
		return "(untitled)"
	}
	return t
}

func limit[T any](xs []T, n int) []T {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

func more(w io.Writer, st style, n, top int, word string) {
	if n > top {
		fmt.Fprintf(w, "  %s\n", st.dim(fmt.Sprintf("… and %s (--top shows more)", plural(n-top, "more "+word))))
	}
}

// num は集計の any（float64 か int）を数にする。
func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return 0
}

// dur は分を「3h 05m」「45m」にする。
func dur(min float64) string {
	m := int(math.Round(min))
	if m < 60 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %02dm", m/60, m%60)
}

// count は 1234 を「1,234」にする。
func count(v float64) string {
	if odd, ok := oddNum(v); ok {
		return odd
	}
	s := fmt.Sprintf("%d", int64(math.Round(math.Abs(v))))
	var b strings.Builder
	if v < 0 && math.Round(v) != 0 {
		b.WriteString("-")
	}
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func countOr(n int) string {
	if n == 0 {
		return "—"
	}
	return count(float64(n))
}

// tokens は 45200000 を「45.2M」にする。
func tokens(v float64) string {
	if odd, ok := oddNum(v); ok {
		return odd
	}
	a := math.Abs(v)
	switch {
	case a >= 1e9:
		return fmt.Sprintf("%.1fB", v/1e9)
	case a >= 1e6:
		return fmt.Sprintf("%.1fM", v/1e6)
	case a >= 1e3:
		return fmt.Sprintf("%.1fk", v/1e3)
	}
	return fmt.Sprintf("%.0f", v)
}

func money(v float64) string {
	if odd, ok := oddNum(v); ok {
		return odd
	}
	if math.Abs(v) >= 1000 {
		return "$" + count(v)
	}
	return fmt.Sprintf("$%.2f", v)
}

func moneyOr(v float64) string {
	if v == 0 {
		return "—"
	}
	return money(v)
}

// oddNum は、壊れた履歴から来た数（無限大・NaN・ありえない大きさ）を、桁があふれないように書く。
func oddNum(v float64) (string, bool) {
	switch {
	case math.IsNaN(v) || math.IsInf(v, 0):
		return "?", true
	case math.Abs(v) >= 1e15:
		return fmt.Sprintf("%.1e", v), true
	}
	return "", false
}

// deltaNum は前の期間との差（「+12」「−$3.10」）。比べる値がなければ空。
func deltaNum(cur, prev float64, has bool, f func(float64) string) string {
	d := cur - prev
	if !has || math.IsNaN(d) || math.IsInf(d, 0) {
		return ""
	}
	if math.Abs(d) < 1e-9 || f(math.Abs(d)) == f(0) {
		return "±0"
	}
	if d > 0 {
		return "+" + f(d)
	}
	return "−" + f(-d)
}

func deltaDur(cur, prev float64, has bool) string { return deltaNum(cur, prev, has, dur) }

// bar は v を width 文字ぶんの帯にする（1/8 刻み）。
func bar(v, most float64, width int) string {
	if most <= 0 {
		return ""
	}
	eighths := int(math.Round(v / most * float64(width*8)))
	if eighths == 0 && v > 0 {
		eighths = 1
	}
	parts := []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}
	return strings.Repeat("█", eighths/8) + parts[eighths%8]
}

// nameList は、名前の並びを、1 つずつ幅をそろえて「, 」でつなぐ（1 つが長くても行が伸びすぎないように）。
func nameList(list []string) string {
	out := make([]string, len(list))
	for i, n := range list {
		out[i] = strings.TrimSpace(fit(n, 40))
	}
	return strings.Join(out, ", ")
}

// clean は、履歴や git から来た文字列を端末に出せる 1 行にする。制御文字（エスケープシーケンスの ESC、改行、C1 の CSI など）と、
// 表示の向きを変える文字（Trojan Source）は、端末の色やカーソルを動かしたり、表示を偽ったりできるので「?」にする。
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			return ' '
		case unicode.IsControl(r), unicode.Is(unicode.Bidi_Control, r), r == '\u2028' || r == '\u2029':
			return '?'
		case unicode.Is(unicode.Cf, r) && r != '\u200d': // ゼロ幅の文字やタグ文字は、名前の中に見えない文字を隠せるので除く（絵文字をつなぐ ZWJ は残す）
			return -1
		}
		return r
	}, s)
}

// width は端末での見かけの幅（全角の文字は 2）。
func width(r rune) int {
	switch {
	case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || r == '\u200b' || r == '‍' || (r >= '︀' && r <= '️'):
		return 0
	case unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r),
		r >= 0x3000 && r <= 0x303f, r >= 0xff01 && r <= 0xff60, r >= 0xffe0 && r <= 0xffe6, r >= 0x1f000 && r <= 0x1faff, r == 0x26a1:
		return 2
	}
	return 1
}

// fit は s を clean したうえで、見かけの幅 n にそろえる（長ければ「…」で切り、短ければ空白で埋める）。
func fit(s string, n int) string {
	s = strings.Join(strings.Fields(clean(s)), " ")
	var b strings.Builder
	w := 0
	rs := []rune(s)
	if len(rs) > 4*n { // 幅 0 の文字（結合文字）を大量に並べても、行が伸びすぎないように
		rs = append(rs[:4*n:4*n], '…')
		s = string(rs)
	}
	total := 0
	for _, r := range rs {
		total += width(r)
	}
	if total <= n {
		return s + strings.Repeat(" ", n-total)
	}
	for _, r := range rs {
		if w+width(r) > n-1 {
			break
		}
		b.WriteRune(r)
		w += width(r)
	}
	b.WriteString("…")
	w++
	return b.String() + strings.Repeat(" ", n-w)
}
