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

func FmtMetric(key string, v *float64) string {
	if v == nil {
		return "不明"
	}
	switch Metrics[key].Unit {
	case "分":
		return HM(*v)
	case "$":
		if *v < 1 {
			return "$" + comma(*v, 3)
		}
		return "$" + comma(*v, 2)
	}
	return strconv.FormatFloat(*v, 'g', 6, 64) + Metrics[key].Unit
}

func withN(key string, v *float64, n *int) string {
	s := FmtMetric(key, v)
	if n != nil {
		s += fmt.Sprintf("（n=%d）", *n)
	}
	return s
}

var stateJA = map[string]string{"ok": "範囲内", "over": "**超過**", "building": "基準づくり中", "unknown": "不明"}

func UserSection(focusLabel string) string {
	var fl, dl []string
	for _, f := range Focus {
		fl = append(fl, f[1])
	}
	dl = append(dl, Decisions...)
	return strings.Join([]string{
		Mark, "",
		"## 今週の判断", "",
		"<!-- 重点は 1 つ: " + strings.Join(fl, " / ") + " -->",
		"- 重点: " + focusLabel,
		"<!-- 先週の一手は: " + strings.Join(Did, " / ") + " -->",
		"- 先週の一手: ",
		"<!-- 判断は: " + strings.Join(dl, " / ") + "（変化がなければ「判断なし」と理由だけで OK） -->",
		"- 判断: ", "- 理由: ", "- 次の一手: ", "- 次に確認する日: ", "- まだ確かめていない仮説: ", "",
		"## ふりかえりメモ", "", "- よかったこと：", "- 詰まったこと：", ""}, "\n")
}

func local(ts float64) time.Time { return time.Unix(0, int64(ts*1e9)).In(time.Local) }

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// WeeklyMarkdown は週のふりかえりを書き出す。existing があれば、印の行より下（自分で書いた欄）を残す。
func WeeklyMarkdown(st *Week, report []source.Report, existing string) string {
	ws := st.start
	we := ws.AddDate(0, 0, 6)
	pb := st.Playbook
	longest := 0
	for _, b := range st.Focus {
		longest = max(longest, b.Min)
	}
	L := []string{fmt.Sprintf("# kiroku 週次ふりかえり %s〜%s", ws.Format("2006/01/02"), we.Format("01/02")), "",
		"> 自分のふりかえり用の数字です。人と比べたり、評価に使ったりするためのものではありません。",
		"> 指標の定義 v" + MetricsVersion + "。版が違う週とは比べないでください。", ""}
	// 1. 重点
	L = append(L, "## 重点", "")
	f := pb.Focus
	if f != nil {
		b := f.Base
		baseTxt := fmt.Sprintf("基準づくり中（%d/%d 週）", b.Weeks, BaselineMin)
		if b.Median != nil {
			baseTxt = fmt.Sprintf("直前 %d 週の中央値 %s", b.Weeks, FmtMetric(f.Metric, b.Median))
		}
		L = append(L, fmt.Sprintf("**%s** — %s: %s（%s）", f.Label, Metrics[f.Metric].Label, withN(f.Metric, f.V, f.N), baseTxt), "")
		if note := Metrics[f.Metric].Note; note != "" {
			L = append(L, "_"+note+"_", "")
		}
	} else {
		L = append(L, "まだ選んでいません。下の「今週の判断」の「重点」に 1 つ書いてください。", "")
	}
	// 2. ガードレール
	L = append(L, "## ガードレール", "", "| 指標 | 今週 | 閾値 | 状態 |", "|---|---|---|---|")
	for _, g := range pb.Guards {
		lim := fmt.Sprintf("—（%d/%d 週）", g.Base.Weeks, BaselineMin)
		if g.Base.Limit != nil {
			lim = FmtMetric(g.Key, g.Base.Limit) + " 以下"
		}
		L = append(L, fmt.Sprintf("| %s | %s | %s | %s |", Metrics[g.Key].Label, withN(g.Key, g.V, g.N), lim, stateJA[g.State]))
	}
	L = append(L, "", fmt.Sprintf("_閾値は直前 %d 週までの自分の値の、上側の四分位 + 1.5 × 四分位範囲。目標ではありません。_", BaselineWeeks), "")
	// 3. 計測の状態
	L = append(L, "## 計測の状態", "")
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
	var unknown []string
	for _, k := range []string{"fix_rate", "focus_blocks", "switches", "cost_per_prompt", "night", "weekend"} {
		if st.Metrics[k].V == nil {
			unknown = append(unknown, Metrics[k].Label)
		}
	}
	if len(unknown) > 0 {
		L = append(L, "- 不明: "+strings.Join(unknown, "、"))
	}
	if st.Usage.Unpriced > 0 {
		L = append(L, fmt.Sprintf("- 料金表にないモデルのトークン %s は目安コストに入っていません", intComma(st.Usage.Unpriced)))
	}
	L = append(L, "- 指標の定義: v"+MetricsVersion)
	// 4. 先週の判断
	L = append(L, "", "## 先週の判断", "")
	if pv := pb.Prev; pv != nil {
		dec := "—"
		if pv.Decision != nil {
			dec = *pv.Decision
		}
		L = append(L, fmt.Sprintf("- 判断: %s（%s）", dec, pv.File), "- 理由: "+orDash(pv.Reason),
			"- 次の一手: "+orDash(pv.Next), "- 次に確認する日: "+orDash(pv.CheckOn))
	} else {
		L = append(L, "- まだ記録がありません")
	}
	// 参照値
	u := st.Usage
	cache := "不明"
	if u.CacheHit != nil {
		cache = strconv.Itoa(int(core.Round(*u.CacheHit*100, 0))) + "%"
	}
	L = append(L, "", "## 参照値", "", "目標ではなく、判断の材料として見る数字です。", "",
		"| 指標 | 今週 | メモ |", "|---|---|---|",
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
		fmt.Sprintf("| サブエージェント | %d 回 | 延べ %s |", u.Subagents, HM(u.SubMin)))
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
	L = append(L, "", "プロジェクト別:", "")
	for _, p := range st.Projects {
		v := p[1].(float64)
		L = append(L, fmt.Sprintf("- %s: %s（%d%%）", p[0], HM(v), int(core.Round(v*100/total, 0))))
	}
	L = append(L, "", "モデル別:", "")
	if len(u.Models) == 0 {
		L = append(L, "- なし")
	}
	for _, m := range u.Models {
		L = append(L, fmt.Sprintf("- %s: $%s・%s トークン・%d 応答", m[0], comma(m[1].(float64), 2), intComma(m[2].(float64)), int(m[3].(float64))))
	}
	L = append(L, "", "日ごと:", "", "| 日 | 作業 | 深夜 | 依頼 | 切り替え |", "|---|---|---|---|---|")
	wd := []rune("月火水木金土日")
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
		L = append(L, fmt.Sprintf("| %s(%c) | %s | %s | %s | %s |", d.Format("01/02"), wd[i], z(x.Active, true), z(x.Night, true), z(x.Prompts, false), z(x.Switches, false)))
	}
	L = append(L, "", "こじれたかもしれないセッション:", "")
	if len(st.Friction) == 0 {
		L = append(L, "- なし")
	}
	for _, x := range st.Friction {
		L = append(L, fmt.Sprintf("- %s [%s] %s — %s", local(x.Start).Format("01/02 15:04"), x.Project, x.Title, strings.Join(x.Why, "、")))
	}
	L = append(L, "")
	if i := strings.Index(existing, Mark); existing != "" && i >= 0 {
		L = append(L, Mark+strings.TrimRight(existing[i+len(Mark):], " \t\r\n")+"\n")
	} else {
		label := ""
		if f != nil {
			label = f.Label
		}
		L = append(L, UserSection(label))
	}
	return strings.Join(L, "\n")
}
