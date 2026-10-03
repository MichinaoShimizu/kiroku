package report

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// 重点・ガードレール・判断ログ。
// 重点は本人が週ごとに 1 つ選ぶ。ガードレールは目標ではなく閾値で、本人の過去の週から決める。
// 率には分母を添え、データがないときは 0 ではなく「不明」にする。

const MetricsVersion = "1" // 指標の定義を変えたら上げる。版が違う週は比べない

type MetricDef struct {
	Label  string `json:"label"`
	Unit   string `json:"unit"`
	Better string `json:"better"`
	Note   string `json:"note"`
}

var Metrics = map[string]MetricDef{
	"fix_rate": {"言い直し・中断のあった依頼の割合", "%", "down",
		"依頼文の言葉（「違う」「やり直して」など）と中断から推定した目安"},
	"focus_blocks":    {"集中ブロック（60分以上）", "回", "up", ""},
	"switches":        {"1日のプロジェクト切り替え（平均）", "回", "down", "作業した日の平均"},
	"cost_per_prompt": {"1依頼あたりの目安コスト", "$", "down", "API 換算の目安。サブスクの請求額とは別。成果あたりの費用ではありません"},
	"night":           {"深夜（22〜6時）の作業", "分", "down", "働き方の健全さの代理"},
	"weekend":         {"週末の作業", "分", "down", "働き方の健全さの代理"},
}

// Focus は (key, その週に強めたいこと, 見る指標)。
var Focus = [][3]string{
	{"delegate", "任せられる依頼にする", "fix_rate"},
	{"deep", "まとまった時間を守る", "focus_blocks"},
	{"switch", "切り替えを減らす", "switches"},
	{"cost", "費用に見合う使い方にする", "cost_per_prompt"},
}

var (
	Guards        = []string{"night", "weekend", "fix_rate", "cost_per_prompt"}
	BaselineWeeks = 8
	BaselineMin   = 3
	Decisions     = []string{"継続", "変更", "中止", "保留", "判断なし"}
	Did           = []string{"やった", "一部", "やっていない", "なし"}
)

const Mark = "<!-- kiroku: この線より下は自分で書く欄です。--weekly で作り直しても残ります -->"

var journalFields = map[string]string{"重点": "focus", "先週の一手": "did", "判断": "decision", "理由": "reason",
	"次の一手": "next", "次に確認する日": "checkOn", "まだ確かめていない仮説": "hypothesis"}

// Entry は 1 週ぶんの判断ログ。
type Entry struct {
	Focus      *string `json:"focus"`
	Did        *string `json:"did"`
	Decision   *string `json:"decision"`
	Reason     string  `json:"reason,omitempty"`
	Next       string  `json:"next,omitempty"`
	CheckOn    string  `json:"checkOn,omitempty"`
	Hypothesis string  `json:"hypothesis,omitempty"`
	File       string  `json:"file"`
}

func FocusOf(text string) []string {
	t := strings.TrimSpace(text)
	if t == "" {
		return nil
	}
	for _, f := range Focus {
		if t == f[0] || t == f[1] || strings.Contains(t, f[1]) {
			return f[:]
		}
	}
	return nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

var (
	secRe  = regexp.MustCompile(`(?ms)^##\s*今週の判断\s*$(.*?)(?:^##\s|\z)`)
	lineRe = regexp.MustCompile(`^\s*[-*]\s*([^:：]+)[:：]\s*(.*)$`)
)

// ReadJournal は kiroku-week-YYYY-MM-DD.md の「今週の判断」を読む。
func ReadJournal(folder string) map[string]*Entry {
	out := map[string]*Entry{}
	paths, _ := filepath.Glob(filepath.Join(folder, "kiroku-week-*.md"))
	sort.Strings(paths)
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		text := strings.ReplaceAll(string(b), "\r\n", "\n")
		mine := text
		if i := strings.Index(text, Mark); i >= 0 {
			mine = text[i+len(Mark):]
		}
		m := secRe.FindStringSubmatch(mine)
		if m == nil {
			continue
		}
		vals := map[string]string{}
		for _, line := range strings.Split(m[1], "\n") {
			if lm := lineRe.FindStringSubmatch(line); lm != nil {
				if f, ok := journalFields[strings.TrimSpace(lm[1])]; ok {
					vals[f] = strings.TrimSpace(lm[2])
				}
			}
		}
		e := &Entry{Reason: vals["reason"], Next: vals["next"], CheckOn: vals["checkOn"], Hypothesis: vals["hypothesis"], File: filepath.Base(p)}
		if f := FocusOf(vals["focus"]); f != nil {
			e.Focus = &f[0]
		}
		if d := vals["decision"]; contains(Decisions, d) {
			e.Decision = &d
		}
		if d := vals["did"]; contains(Did, d) {
			e.Did = &d
		}
		key := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "kiroku-week-"), ".md")
		out[key] = e
	}
	return out
}

type Baseline struct {
	Weeks  int      `json:"weeks"`
	Median *float64 `json:"median,omitempty"`
	Q1     *float64 `json:"q1,omitempty"`
	Q3     *float64 `json:"q3,omitempty"`
	Limit  *float64 `json:"limit,omitempty"`
}

type Guard struct {
	Key   string   `json:"key"`
	V     *float64 `json:"v"`
	N     *int     `json:"n"`
	Base  Baseline `json:"base"`
	State string   `json:"state"`
}

type FocusView struct {
	Key    string   `json:"key"`
	Label  string   `json:"label"`
	Metric string   `json:"metric"`
	V      *float64 `json:"v"`
	N      *int     `json:"n"`
	Base   Baseline `json:"base"`
	Series [][2]any `json:"series"`
}

type Playbook struct {
	Focus   *FocusView `json:"focus"`
	Guards  []Guard    `json:"guards"`
	Journal *Entry     `json:"journal"`
	Prev    *Entry     `json:"prev"`
}

// quantiles は Python の statistics.quantiles(vals, n=4, method="inclusive") と同じ。
func quantiles(vals []float64) (q1, med, q3 float64) {
	d := append([]float64(nil), vals...)
	sort.Float64s(d)
	m := len(d) - 1
	q := func(i int) float64 {
		j := i * m / 4
		delta := i*m - j*4
		if delta == 0 {
			return d[j]
		}
		return (d[j]*float64(4-delta) + d[j+1]*float64(delta)) / 4
	}
	return q(1), q(2), q(3)
}

func r3(v float64) *float64 { return fptr(roundN(v, 3)) }

// Annotate は各週に重点・ガードレールの判定・先週の判断を足す。基準値は本人の直前の週から。
func Annotate(weeks map[string]*Week, journal map[string]*Entry) {
	keys := make([]string, 0, len(weeks))
	for k := range weeks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	jkeys := make([]string, 0, len(journal))
	for k := range journal {
		jkeys = append(jkeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(jkeys)))
	var focus *string
	for i, k := range keys {
		st := weeks[k]
		lo := max(0, i-BaselineWeeks)
		prior := make([]*Week, 0)
		for _, x := range keys[lo:i] {
			prior = append(prior, weeks[x])
		}
		base := func(mk string) Baseline {
			var vals []float64
			for _, w := range prior {
				if v := w.Metrics[mk].V; v != nil {
					vals = append(vals, *v)
				}
			}
			if len(vals) < BaselineMin {
				return Baseline{Weeks: len(vals)}
			}
			q1, med, q3 := quantiles(vals)
			return Baseline{Weeks: len(vals), Median: r3(med), Q1: r3(q1), Q3: r3(q3), Limit: r3(q3 + 1.5*(q3-q1))}
		}
		j := journal[k]
		if j != nil && j.Focus != nil {
			focus = j.Focus // 書いていない週は前の週の重点を引き継ぐ
		}
		var f []string
		if focus != nil {
			for _, x := range Focus {
				if x[0] == *focus {
					f = x[:]
				}
			}
		}
		var guards []string
		for _, g := range Guards {
			if f == nil || g != f[2] {
				guards = append(guards, g)
			}
		}
		if f != nil && contains(Guards, f[2]) {
			guards = append(guards, "switches")
		}
		gl := []Guard{}
		for _, g := range guards {
			m, b := st.Metrics[g], base(g)
			state := "ok"
			switch {
			case m.V == nil:
				state = "unknown"
			case b.Limit == nil:
				state = "building"
			case *m.V > *b.Limit && *m.V > 0:
				state = "over"
			}
			gl = append(gl, Guard{Key: g, V: m.V, N: m.N, Base: b, State: state})
		}
		var prev *Entry
		for _, x := range jkeys {
			e := journal[x]
			if x < k && (e.Decision != nil || e.Next != "" || e.Reason != "") {
				prev = e
				break
			}
		}
		pb := &Playbook{Guards: gl, Journal: j, Prev: prev}
		if f != nil {
			m := st.Metrics[f[2]]
			series := [][2]any{}
			from := max(0, len(prior)-7)
			for _, w := range append(prior[from:], st) {
				series = append(series, [2]any{w.Week, w.Metrics[f[2]].V})
			}
			pb.Focus = &FocusView{Key: f[0], Label: f[1], Metric: f[2], V: m.V, N: m.N, Base: base(f[2]), Series: series}
		}
		st.Playbook = pb
	}
}
