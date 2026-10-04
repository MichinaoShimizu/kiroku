package core

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Correction は言い直し・差し戻しっぽい依頼（こじれたセッションの目印）。
var Correction = regexp.MustCompile(`(?i)違う|ちがう|そうじゃな|やり直|戻して|元に戻|取り消|じゃなくて|\b(?:wrong|incorrect|nope|revert|undo|roll ?back|start over|not what|that's not|try again|(?:doesn't|does not|didn't|did not|still not|isn't|is not) work(?:ing)?|still (?:broken|failing|fails)|you broke)\b`)

var editTools = map[string]bool{
	"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true, // Claude Code
	"fsWrite": true, "fsAppend": true, "strReplace": true, "fs_write": true, "write": true, "editCode": true, // Kiro
}

// Prompt は人が出した依頼。
type Prompt struct {
	T    *float64 `json:"t"`
	Text string   `json:"text"`
}

// Subagent はサブエージェント（子のエージェント）の 1 回の実行。
type Subagent struct {
	Type   string     `json:"type"`
	Desc   string     `json:"desc"`
	BG     bool       `json:"bg"`
	Start  *float64   `json:"start"`
	End    *float64   `json:"end"`
	Model  *string    `json:"model"`
	Tools  float64    `json:"tools"`
	Usage  UsageTotal `json:"usage"`
	Events []Event    `json:"-"`
}

// Credit は Kiro などのクレジット。
type Credit struct {
	T *float64
	V float64
}

// Builder は 1 つのセッションを読みながら組み立てる。アダプターはこれだけを触る。
type Builder struct {
	Source, ID                     string
	Key                            string // 同じ会話が 2 つの場所に残るとき、先に読んだほうだけを使うための鍵
	Project, Branch, Title, Resume string
	Prompts                        []Prompt
	Times                          []float64
	AgentTimes                     []float64
	Interrupts                     int
	Limits                         []float64 // 利用上限（使用量の上限・レート制限）に当たった時刻
	FixTS                          []float64
	Usage                          *Usage
	Subagents                      []Subagent
	Credits                        []Credit
	Measures                       []Measure      // そのエージェントだけが記録している数字（native.go）
	Outputs                        []Output       // 成果の印（output.go）。成功したツール呼び出しだけを入れる
	Reported                       []ReportedCost // エージェント自身が記録した使用料（reported.go）
	File                           string         // 履歴のファイル（画面から開けるように）
	toolOrder                      []string
	tools                          map[string]int
	files                          map[string]bool
	modelOrder                     []string
	models                         map[string]int
}

func NewBuilder(source, id string) *Builder {
	return &Builder{Source: source, ID: id, Usage: NewUsage(), tools: map[string]int{}, files: map[string]bool{}, models: map[string]int{}}
}

func (s *Builder) Tick(ts *float64) {
	if ts != nil {
		s.Times = append(s.Times, *ts)
	}
}

// Agent は AI が動いていた時刻（待たせ時間の計算用）。
func (s *Builder) Agent(ts *float64) {
	s.Tick(ts)
	if ts != nil {
		s.AgentTimes = append(s.AgentTimes, *ts)
	}
}

func (s *Builder) Prompt(ts *float64, text string) {
	if text != "" && strings.HasPrefix(strings.TrimLeft(text, " \t\r\n"), "[Request interrupted") {
		s.Interrupts++
		if ts != nil {
			s.FixTS = append(s.FixTS, *ts)
		}
	}
	if text != "" && !IsNoise(text) {
		s.Prompts = append(s.Prompts, Prompt{T: ts, Text: Runes(strings.TrimSpace(text), 400)})
		if Correction.MatchString(text) && ts != nil {
			s.FixTS = append(s.FixTS, *ts)
		}
	}
}

// Limit は利用上限のエラーを記録する。同じ上限で続けて出たもの（1 分以内）は 1 回と数える。
func (s *Builder) Limit(ts *float64) {
	if ts == nil {
		return
	}
	if n := len(s.Limits); n > 0 && *ts-s.Limits[n-1] < 60 {
		return
	}
	s.Limits = append(s.Limits, *ts)
}

func (s *Builder) Tool(name string, args any) {
	if name == "" {
		name = "?"
	}
	if _, ok := s.tools[name]; !ok {
		s.toolOrder = append(s.toolOrder, name)
	}
	s.tools[name]++
	m := Map(args)
	fp := Str(m["file_path"])
	if fp == "" {
		fp = Str(m["path"])
	}
	if fp == "" {
		fp = Str(m["targetFile"])
	}
	if fp != "" && editTools[name] {
		s.files[fp] = true
	}
}

// Measure はそのエージェントだけが記録している数字を 1 つ足す。
func (s *Builder) Measure(key string, t *float64, v float64) {
	s.Measures = append(s.Measures, Measure{Key: key, T: t, V: v})
}

// ToolCounts はツールごとの回数。
func (s *Builder) ToolCounts() map[string]int { return s.tools }

// AddEvent は、メッセージ ID で重ねる必要のない使用量（Codex など）をそのまま足す。モデルも数える。
func (s *Builder) AddEvent(e Event) {
	s.Usage.order = append(s.Usage.order, "_e"+itoa(len(s.Usage.order)))
	ev := e
	s.Usage.byMsg[s.Usage.order[len(s.Usage.order)-1]] = &ev
	s.Model(e.Model)
}

// Model は使ったモデルを 1 回数える。
func (s *Builder) Model(m string) {
	if m == "" || m == "<synthetic>" {
		return
	}
	if _, ok := s.models[m]; !ok {
		s.modelOrder = append(s.modelOrder, m)
	}
	s.models[m]++
}

// Waits は AI が最後に動いてから、人が次の依頼を出すまでの秒数（30 分以内のものだけ）。
func (s *Builder) Waits() [][2]float64 {
	agent := append([]float64(nil), s.AgentTimes...)
	sort.Float64s(agent)
	var ps []float64
	for _, p := range s.Prompts {
		if p.T != nil && *p.T != 0 {
			ps = append(ps, *p.T)
		}
	}
	sort.Float64s(ps)
	out := [][2]float64{}
	var prev *float64
	for i := range ps {
		p := ps[i]
		last, found := 0.0, false
		for _, a := range agent {
			if (prev == nil || a > *prev) && a < p && (!found || a > last) {
				last, found = a, true
			}
		}
		if found && p-last > 0 && p-last <= 1800 {
			out = append(out, [2]float64{p, Round(p-last, 0)})
		}
		prev = &ps[i]
	}
	return out
}

func (s *Builder) Corrections() int {
	n := 0
	for _, p := range s.Prompts {
		if Correction.MatchString(p.Text) {
			n++
		}
	}
	return n
}

// Session は画面と JSON に出すセッション。
type Session struct {
	ID           string        `json:"id"`
	Source       string        `json:"source"`
	Project      string        `json:"project"`
	ProjectPath  string        `json:"projectPath"`
	Branch       *string       `json:"branch"`
	Title        string        `json:"title"`
	Start        float64       `json:"start"`
	End          float64       `json:"end"`
	Events       int           `json:"events"`
	Segs         [][3]float64  `json:"segs"`
	Prompts      []Prompt      `json:"prompts"`
	NPrompts     int           `json:"nPrompts"`
	Tools        [][2]any      `json:"tools"`
	Files        []string      `json:"files"`
	NFiles       int           `json:"nFiles"`
	Resume       *string       `json:"resume"`
	Waits        [][2]float64  `json:"waits"`
	Interrupts   int           `json:"interrupts"`
	Limits       []float64     `json:"limits"` // 利用上限に当たった時刻
	Ctx          []float64     `json:"ctx"`    // 1 回の応答で読んだ入力（文脈）の大きさ [前半, 後半, 最大]（context.go）
	Corrections  int           `json:"corrections"`
	Models       [][2]any      `json:"models"`
	Usage        UsageTotal    `json:"usage"`
	Subagents    []Subagent    `json:"subagents"`
	Credits      float64       `json:"credits"`
	Cost         float64       `json:"cost"`
	Native       []NativeValue `json:"native"`                 // このセッションの参考指標
	Outputs      OutputTotal   `json:"outputs"`                // コミット・PR・変更した行（output.go）
	CostReported bool          `json:"costReported,omitempty"` // 目安コストにエージェント自身の記録を使った
	File         string        `json:"file,omitempty"`         // 履歴のファイル
	PRs          []string      `json:"prs"`                    // AI が作った PR の URL（わかったもの）
	UEv          []Event       `json:"-"`                      // 週ごとの集計用（HTML には入れない）
	CEv          []Credit      `json:"-"`
	OEv          []Output      `json:"-"`
	Fix          []float64     `json:"-"`
	Meas         []Measure     `json:"-"`
}

func ptr[T any](v T) *T { return &v }

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Segments はイベント時刻の列を、gap 以上あいたところで帯に切る。
func Segments(times []float64, gap float64) [][3]float64 {
	segs := [][3]float64{}
	start, prev, n := times[0], times[0], 1.0
	for _, t := range times[1:] {
		if t-prev > gap {
			segs = append(segs, [3]float64{start, math.Max(prev, start+60), n})
			start, n = t, 0
		}
		prev, n = t, n+1
	}
	return append(segs, [3]float64{start, math.Max(prev, start+60), n})
}

func basename(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// Finish はセッションを画面用の形にする。時刻がなければ nil。
func (s *Builder) Finish(gapMin int) *Session {
	var times []float64
	for _, t := range s.Times {
		if t != 0 {
			times = append(times, t)
		}
	}
	if len(times) == 0 {
		return nil
	}
	sort.Float64s(times)
	project := s.Project
	if project == "" {
		project = "(不明)"
	}
	title := s.Title
	if title == "" {
		if len(s.Prompts) > 0 {
			title = Runes(strings.SplitN(s.Prompts[0].Text, "\n", 2)[0], 80)
		} else {
			title = "(無題)"
		}
	}
	name := basename(project)
	if name == "" {
		name = project
	}
	tools := sortedCounts(s.toolOrder, s.tools, 8)
	models := sortedCounts(s.modelOrder, s.models, 0)
	var files []string
	for f := range s.files {
		files = append(files, f)
	}
	sort.Strings(files)
	nFiles := len(files)
	if len(files) > 30 {
		files = files[:30]
	}
	prompts := s.Prompts
	if len(prompts) > 50 {
		prompts = prompts[:50]
	}
	main := s.Usage.Events()
	ctx := ContextGrowth(main)
	if len(s.Reported) > 0 {
		groups := [][]Event{main}
		for _, a := range s.Subagents {
			groups = append(groups, a.Events)
		}
		main = append(main, applyReported(s.Reported, groups...)...)
		for i := range s.Subagents {
			if len(s.Subagents[i].Events) > 0 {
				rt := s.Subagents[i].Usage.ReportedTokens
				s.Subagents[i].Usage = SumUsage(s.Subagents[i].Events)
				s.Subagents[i].Usage.ReportedTokens = rt
			}
		}
	}
	mainSum := SumUsage(main)
	cost := mainSum.Cost
	uev := append([]Event(nil), main...)
	for _, a := range s.Subagents {
		cost += a.Usage.Cost
		uev = append(uev, a.Events...)
	}
	credits := 0.0
	for _, c := range s.Credits {
		credits += c.V
	}
	var outs OutputTotal
	prs := []string{}
	for _, o := range s.Outputs {
		outs.Add(o)
		if o.Kind == "pr" && o.URL != "" {
			prs = append(prs, o.URL)
		}
	}
	if files == nil {
		files = []string{}
	}
	if prompts == nil {
		prompts = []Prompt{}
	}
	subs := s.Subagents
	if subs == nil {
		subs = []Subagent{}
	}
	return &Session{
		ID: s.ID, Source: s.Source, Project: name, ProjectPath: project, Branch: strOrNil(s.Branch), Title: title,
		Start: times[0], End: times[len(times)-1], Events: len(times), Segs: Segments(times, float64(gapMin*60)),
		Prompts: prompts, NPrompts: len(s.Prompts), Tools: tools, Files: files, NFiles: nFiles, Resume: strOrNil(s.Resume),
		Waits: s.Waits(), Interrupts: s.Interrupts, Limits: limits(s.Limits), Ctx: ctx, Corrections: s.Corrections(), Models: models,
		Usage: mainSum, Subagents: subs, Credits: Round(credits, 3), Cost: Round(cost, 4),
		UEv: uev, CEv: s.Credits, OEv: s.Outputs, Outputs: outs, Fix: s.FixTS, CostReported: len(s.Reported) > 0, File: s.File, PRs: prs,
		Native: AggregateNative(s.Source, s.Measures), Meas: s.Measures,
	}
}

// sortedCounts は多い順（同数なら出てきた順）に並べる。limit 0 は全部。
func sortedCounts(order []string, counts map[string]int, limit int) [][2]any {
	keys := append([]string(nil), order...)
	sort.SliceStable(keys, func(i, j int) bool { return counts[keys[i]] > counts[keys[j]] })
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}
	out := [][2]any{}
	for _, k := range keys {
		out = append(out, [2]any{k, counts[k]})
	}
	return out
}

var _ = ptr[int]

func limits(xs []float64) []float64 {
	if xs == nil {
		return []float64{}
	}
	return xs
}
