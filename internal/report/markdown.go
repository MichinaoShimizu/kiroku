package report

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// HM は分を「3時間05分」のように書く。
func HM(minutes float64) string {
	m := int(core.Round(minutes, 0))
	if m >= 60 {
		return fmt.Sprintf("%d時間%02d分", m/60, m%60)
	}
	return fmt.Sprintf("%d分", m)
}

func Secs(v *float64) string {
	if v == nil {
		return "—"
	}
	s := int(*v)
	if s < 60 {
		return fmt.Sprintf("%d秒", s)
	}
	return fmt.Sprintf("%d分%02d秒", s/60, s%60)
}

// comma は 3 桁区切り。decimals<0 なら Python の str(float) と同じ桁で書く。
func comma(v float64, decimals int) string {
	var s string
	if decimals >= 0 {
		s = strconv.FormatFloat(v, 'f', decimals, 64)
	} else {
		s = pyFloat(v)
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i:]
	}
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	out := b.String() + frac
	if neg {
		return "-" + out
	}
	return out
}

// pyFloat は Python の float の表示（17.0, 4.2）と同じ。
func pyFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

func intComma(v float64) string { return comma(math.Round(v), 0) }

// nativeText は参考指標の値を単位つきで書く。
func nativeText(v core.NativeValue) string {
	switch v.Unit {
	case "%":
		return strconv.FormatFloat(core.Round(v.V, 1), 'f', -1, 64) + "%"
	case "秒":
		return strconv.FormatFloat(core.Round(v.V, 1), 'f', -1, 64) + "秒"
	case "クレジット":
		return strconv.FormatFloat(core.Round(v.V, 2), 'f', -1, 64) + " クレジット"
	case "トークン", "文字":
		return intComma(v.V) + " " + v.Unit
	}
	return intComma(v.V) + " " + v.Unit
}

func local(ts float64) time.Time { return time.Unix(0, int64(ts*1e9)).In(time.Local) }

// WeeklyMarkdown は週次サマリーを書き出す。
func WeeklyMarkdown(st *Summary, report []source.Report) string {
	we := st.end.AddDate(0, 0, -1)
	return summaryMarkdown(st, report, fmt.Sprintf("# kiroku 週次サマリー %s〜%s", st.start.Format("2006/01/02"), we.Format("01/02")), "今週")
}

// MonthlyMarkdown は月次サマリーを書き出す。
func MonthlyMarkdown(st *Summary, report []source.Report) string {
	return summaryMarkdown(st, report, fmt.Sprintf("# kiroku 月次サマリー %d年%d月", st.start.Year(), int(st.start.Month())), "今月")
}

func summaryMarkdown(st *Summary, report []source.Report, title, this string) string {
	ws := st.start
	longest := 0
	for _, b := range st.Focus {
		longest = max(longest, b.Min)
	}
	L := []string{title, "",
		"> 自分の使い方を振り返るための数字です。人と比べたり、評価に使ったりするためのものではありません。", ""}
	u := st.Usage
	cache := "不明"
	if u.CacheHit != nil {
		cache = strconv.Itoa(int(core.Round(*u.CacheHit*100, 0))) + "%"
	}
	pct := func(v *float64, unit string) string {
		if v == nil {
			return "不明"
		}
		if unit == "$" {
			return "$" + comma(*v, 3)
		}
		return strconv.FormatFloat(*v, 'f', -1, 64) + unit
	}
	L = append(L, "## まとめ", "",
		"| 指標 | "+this+" | メモ |", "|---|---|---|",
		fmt.Sprintf("| 作業していた時間 | %s | どれかのセッションが動いていた時間 |", HM(float64(st.Active))),
		fmt.Sprintf("| AI の延べ稼働 | %s | 並列ぶんも足した合計。人の削減時間ではありません |", HM(float64(st.AI))),
		fmt.Sprintf("| 集中ブロック（%d分以上） | %d 回 | 最長 %s |", FocusMin, len(st.Focus), HM(float64(longest))),
		fmt.Sprintf("| 1 日の切り替え | 平均 %s 回 | 最大 %d 回 |", pyFloat(st.SwitchesAvg), st.SwitchesMax),
		fmt.Sprintf("| 並列で動かしていた時間 | %s | 最大 %d 本同時 |", HM(float64(st.Parallel)), st.MaxConc),
		fmt.Sprintf("| 待たせ時間 | 中央値 %s（n=%d） | 90%%点 %s。短いほど良いとは限りません |", Secs(st.WaitMedian), st.WaitCount, Secs(st.WaitP90)),
		fmt.Sprintf("| セッション / 依頼 | %d / %d | |", st.Sessions, st.Prompts),
		fmt.Sprintf("| 目安コスト（API 換算） | $%s | サブスクの請求額とは別 |", comma(u.Cost, 2)),
		fmt.Sprintf("| トークン | %s | 使った量の説明材料。多いほど良いわけではありません |", intComma(u.Tokens)),
		fmt.Sprintf("| キャッシュから読んだ割合 | %s | 入力のうち |", cache),
		fmt.Sprintf("| サブエージェント | %d 回 | 延べ %s |", u.Subagents, HM(u.SubMin)),
		fmt.Sprintf("| 深夜（%d〜%d時）/ 週末 | %s / %s | |", NightFrom, NightTo, HM(float64(st.Night)), HM(float64(st.Weekend))),
		fmt.Sprintf("| 言い直し・中断のあった依頼 | %s | 依頼文の言葉と中断から推定した目安（n=%d） |", pct(st.FixRate, "%"), st.Prompts),
		fmt.Sprintf("| 1 依頼あたりの目安コスト | %s | |", pct(st.CostPerAsk, "$")))
	if u.Credits != 0 {
		L = append(L, fmt.Sprintf("| Kiro クレジット | %s | |", comma(u.Credits, -1)))
	}
	total := 0.0
	for _, p := range st.Projects {
		total += p[1].(float64)
	}
	if total == 0 {
		total = 1
	}
	L = append(L, projectMarkdown(st, total)...)
	L = append(L, "", "モデル別:", "")
	if len(u.Models) == 0 {
		L = append(L, "- なし")
	}
	for _, m := range u.Models {
		L = append(L, fmt.Sprintf("- %s: $%s・%s トークン・%d 応答", m[0], comma(m[1].(float64), 2), intComma(m[2].(float64)), int(m[3].(float64))))
	}
	L = append(L, "", "日ごと:", "", "| 日 | 作業 | 深夜 | 依頼 | 切り替え | トークン | 目安コスト | クレジット |", "|---|---|---|---|---|---|---|---|")
	wd := []rune("日月火水木金土")
	for i, x := range st.Days {
		d := ws.AddDate(0, 0, i)
		z := func(v int, hm bool) string {
			if v == 0 {
				return "—"
			}
			if hm {
				return HM(float64(v))
			}
			return strconv.Itoa(v)
		}
		zf := func(v float64, f func(float64) string) string {
			if v == 0 {
				return "—"
			}
			return f(v)
		}
		L = append(L, fmt.Sprintf("| %s(%c) | %s | %s | %s | %s | %s | %s | %s |", d.Format("01/02"), wd[d.Weekday()], z(x.Active, true), z(x.Night, true), z(x.Prompts, false), z(x.Switches, false),
			zf(x.Tokens, intComma), zf(x.Cost, func(v float64) string { return "$" + comma(v, 2) }), zf(x.Credits, func(v float64) string { return comma(v, -1) })))
	}
	L = append(L, "", "こじれたかもしれないセッション:", "")
	if len(st.Friction) == 0 {
		L = append(L, "- なし")
	}
	for _, x := range st.Friction {
		L = append(L, fmt.Sprintf("- %s [%s] %s — %s", local(x.Start).Format("01/02 15:04"), x.Project, x.Title, strings.Join(x.Why, "、")))
	}
	if len(st.Native) > 0 {
		L = append(L, "", "## エージェント別の参考指標", "",
			"_それぞれのエージェントが記録している数字です。定義がエージェントごとに違うので、エージェント同士では比べないでください。_")
		for _, g := range st.Native {
			L = append(L, "", fmt.Sprintf("**%s**（%d セッション）", g.Source, g.Sessions), "", "| 指標 | "+this+" | n |", "|---|---|---|")
			for _, v := range g.Values {
				L = append(L, fmt.Sprintf("| %s | %s | %d |", v.Label, nativeText(v), v.N))
			}
		}
	}
	// 計測の状態
	L = append(L, "", "## 計測の状態", "")
	for _, r := range report {
		line := fmt.Sprintf("- %s: %d セッション", r.Name, r.N)
		if r.Detail != "" {
			line += "（" + r.Detail + "）"
		}
		if r.Dup > 0 {
			line += fmt.Sprintf("（ほかの場所と同じ会話 %d 件は数えていません）", r.Dup)
		}
		if r.Error != nil {
			line += "（読めなかったファイルあり: " + *r.Error + "）"
		}
		L = append(L, line)
	}
	if st.Usage.Unpriced > 0 {
		L = append(L, fmt.Sprintf("- 料金表にないモデルのトークン %s は目安コストに入っていません", intComma(st.Usage.Unpriced)))
	}
	L = append(L, "- 時刻はこのマシンのタイムゾーンで数えています。どれも履歴から推定した目安です", "")
	return strings.Join(L, "\n")
}

// projectMarkdown はプロジェクト別のまとめ: 表と、プロジェクトごとの「長く動いた」「いちばん重い」セッション。
func projectMarkdown(st *Summary, total float64) []string {
	dash := func(v float64, f func(float64) string) string {
		if v == 0 {
			return "—"
		}
		return f(v)
	}
	usd := func(v float64) string { return "$" + comma(v, 2) }
	cr := func(v float64) string { return comma(v, -1) }
	use := func(t TopSession) string {
		var p []string
		if t.Cost > 0 {
			p = append(p, usd(t.Cost))
		}
		if t.Credits > 0 {
			p = append(p, cr(t.Credits)+" クレジット")
		}
		if t.Tokens > 0 && t.Cost == 0 {
			p = append(p, intComma(t.Tokens)+" トークン")
		}
		return strings.Join(p, "・")
	}
	L := []string{"", "## プロジェクト別", "",
		"| プロジェクト | 時間 | 割合 | セッション / 依頼 | トークン | 目安コスト | クレジット | 主なモデル |",
		"|---|---|---|---|---|---|---|---|"}
	if len(st.ProjectStats) == 0 {
		return append(L[:2], "- なし")
	}
	for _, p := range st.ProjectStats {
		var models []string
		for _, m := range p.Models {
			models = append(models, m.Model)
		}
		L = append(L, fmt.Sprintf("| %s | %s | %d%% | %d / %d | %s | %s | %s | %s |", p.Project, dash(p.Minutes, HM),
			int(core.Round(p.Minutes*100/total, 0)), p.Sessions, p.Prompts, dash(p.Tokens, intComma), dash(p.Cost, usd),
			dash(p.Credits, cr), orNone(strings.Join(models, "、"))))
	}
	L = append(L, "")
	for _, p := range st.ProjectStats {
		var top []string
		for _, t := range p.Top {
			top = append(top, fmt.Sprintf("%s（%s）", t.Title, HM(t.Minutes)))
		}
		line := fmt.Sprintf("- **%s**: %s", p.Project, orNone(strings.Join(top, "、")))
		if h := p.Heavy; h != nil && use(*h) != "" {
			line += fmt.Sprintf("。いちばん重い: %s（%s）", h.Title, use(*h))
		}
		L = append(L, line)
	}
	return L
}

func orNone(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
