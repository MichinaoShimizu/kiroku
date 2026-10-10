package core

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// 言い直し・差し戻しっぽい依頼（こじれたセッションの目印）の言い回し。依頼の冒頭（humanHead）だけを見る。
// 「違う色にして」「undo ボタンを足して」のような、言葉が同じだけのふつうの依頼は拾わない。
var (
	correctionStart = regexp.MustCompile(`(?im)^\s*(?:(?:違う|ちがう|違います|違いました)(?:[、。,.!！?？\s]|よ|って|ね|$)|(?:いや|いいえ)[、。,\s]|(?:no|nope)(?:[,.!]|\s*$)|nope\b|(?:wrong|incorrect|not quite)\b)`)
	correctionAny   = regexp.MustCompile(`(?i)それは違|そうじゃな|そうではな|じゃなくて|ではなくて|やり直して|やりなおして|元に戻|戻して|取り消して|まだ直って|直ってな|直っていな|\b(?:(?:that|this|it)(?:'s| is) (?:not (?:what|right|it)\b|wrong\b|incorrect\b)|not what i (?:asked|wanted|meant)|(?:revert|undo|roll ?back) (?:it|that|this|these|those|your|the (?:last|previous|latest)|what you|everything|all)\b|start over|you broke|(?:still|is still|it's still) (?:not working|broken|failing|fails|wrong|not fixed)|(?:doesn't|does not|didn't|did not|isn't|is not|still doesn't|still does not) work(?:ing)?\b)|(?:^|[.!,;]\s*)try again\b`)
)

// humanHead は依頼の冒頭の、人が書いた部分（コードブロック・引用・字下げしたログより前の 3 行まで）。
// 貼り付けたエラーやログの中の言葉を、言い直しと取り違えないため。
func humanHead(text string) string {
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			break
		}
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, ">") || strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t") {
			continue
		}
		if lines = append(lines, l); len(lines) == 3 {
			break
		}
	}
	return Runes(strings.Join(lines, "\n"), 200)
}

// IsCorrection は、依頼が言い直し・差し戻しっぽいか。会話の最初の依頼（first）は、まだ直すものがないので拾わない。
func IsCorrection(text string, first bool) bool {
	if first {
		return false
	}
	h := humanHead(text)
	return correctionStart.MatchString(h) || (mayCorrect(h) && correctionAny.MatchString(h))
}

// correctionHints は、correctionAny のどの言い回しにも必ず入っている語句（小文字）。correctionAny は大きく、
// 依頼ごとに試すと履歴の読み込みの半分ほどを使っていたので、どれも入っていない依頼では試さない。
// correctionAny を変えたらここも見直す（TestCorrectionHints と FuzzCorrectionHints が確かめる）。
var correctionHints = []string{
	"それは違", "そうじゃな", "そうではな", "じゃなくて", "ではなくて", "やり直して", "やりなおして", "元に戻", "戻して", "取り消して", "直って",
	"s not ", "s wrong", "s incorrect", // (that|this|it)('s| is) ...
	"not what i ", "revert ", "undo ", "roll back ", "rollback ", "start over", "you broke", "still ",
	"n't work", "not work", "try again",
}

// mayCorrect は、h が correctionAny に合うかもしれないか（false なら合わない）。
// (?i) は ASCII の文字では k を K（U+212A）に、s を ſ（U+017F）にも合わせる。K は ToLower で k になるので、ſ だけを s にしてから比べる。
func mayCorrect(h string) bool {
	l := strings.ToLower(strings.ReplaceAll(h, "ſ", "s"))
	for _, w := range correctionHints {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

var editTools = map[string]bool{
	"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true, // Claude Code
	"fsWrite": true, "fsAppend": true, "strReplace": true, "fs_write": true, "write": true, "editCode": true, // Kiro
}

// Prompt は人が出した依頼。Text は先頭 PromptRunes 文字まで（HTML に入れるので）。
type Prompt struct {
	T     *float64 `json:"t"`
	Text  string   `json:"text"`
	Len   int      `json:"len,omitempty"`   // Text を切ったとき、元の文字数
	Work  float64  `json:"work,omitempty"`  // 依頼から、次の依頼までに AI が最後に動いた時刻までの秒数（推定）
	Wait  float64  `json:"wait,omitempty"`  // AI が最後に動いてから、次の依頼までの秒数（待たせ）
	Kind  string   `json:"kind,omitempty"`  // "" は書いたもの、command はスラッシュコマンド、shell は ! で打ったコマンド（kind.go）
	Reply *Reply   `json:"reply,omitempty"` // この依頼に対してエージェントが最後に返した文（なければ nil）
	full  string   // 切る前の依頼文（kiroku serve で全文を見せるため。HTML・JSON には入れない）
}

// PromptRunes は、HTML に入れる依頼文の長さ（文字数）。
const PromptRunes = 400

// fullRunes は、kiroku serve が全文として覚えておく長さの上限（貼り付けた長いログでメモリを使い切らないように）。
const fullRunes = 100000

// Full は切る前の依頼文。切っていなければ Text と同じ。
func (p Prompt) Full() string {
	if p.full != "" {
		return p.full
	}
	return p.Text
}

// Reply は、1 つの依頼に対してエージェントが人に返した文（ツール呼び出しや思考ではなく、会話に出た文）のうち、
// 最後のもの。Text は先頭 ReplyRunes 文字まで（HTML に入れるので）。
type Reply struct {
	T    *float64 `json:"t"`
	Text string   `json:"text"`
	Len  int      `json:"len,omitempty"` // Text を切ったとき、元の文字数
	full string   // 切る前の応答（kiroku serve で全文を見せるため。HTML・JSON には入れない）
}

// ReplyRunes は、HTML に入れる応答の長さ（文字数）。依頼（PromptRunes）より短いのは、
// 応答のほうが長くなりがちで、HTML は 1 ファイルに全部入るため。
const ReplyRunes = 160

// Full は切る前の応答。切っていなければ Text と同じ。
func (r Reply) Full() string {
	if r.full != "" {
		return r.full
	}
	return r.Text
}

// pendingReply は、まだどの依頼のものか決めていない応答（Finish で時刻から割り当てる）。
// text は切る前の文（fullRunes まで）。turn は読んだときのターンの番号で、同じターンの応答は後のもので置き換える。
type pendingReply struct {
	t    *float64
	turn int
	key  string
	text string
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
	Notes                          []Note // エージェントや仕組みが会話に入れたもの（kind.go）
	Times                          []float64
	AgentTimes                     []float64
	Interrupts                     int
	InterruptTS                    []float64 // 中断した時刻
	Limits                         []float64 // 利用上限（使用量の上限・レート制限）に当たった時刻
	LimitResets                    []string  // Limits と同じ並びで、エラー文にあった解除の時刻の文（"3:45pm" など。なければ空）
	Compactions                    []float64 // 会話を要約して文脈を空けた（コンパクション）時刻
	CompactKinds                   []string  // Compactions と同じ並びで、自動か手動か（"auto"・"manual"。わからなければ空）
	CtxWindow                      float64   // 文脈がいちばん大きかった応答のモデルのコンテキストウィンドウ（わからなければ 0。PeakContextWindow）
	FixTS                          []float64
	Usage                          *Usage
	Subagents                      []Subagent
	Credits                        []Credit
	Measures                       []Measure      // そのエージェントだけが記録している数字（native.go）
	Outputs                        []Output       // アウトプット（output.go）。成功したツール呼び出しだけを入れる
	Reported                       []ReportedCost // エージェント自身が記録した使用料（reported.go）
	File                           string         // 履歴のファイル（画面から開けるように）
	replies                        []pendingReply // 応答（Reply で入れ、Finish で依頼に割り当てる）
	turn                           int            // 人の発言を読んだ回数（1 ターンの最後の応答だけを残すための区切り）
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
		s.Interrupt(ts)
	}
	pt, notes := splitUser(text)
	for _, n := range notes {
		s.Inject(ts, n.kind, n.text)
	}
	if pt != nil {
		s.turn++
		first := len(s.Prompts) == 0
		t := pt.text
		p := Prompt{T: ts, Text: Runes(t, PromptRunes), Kind: pt.kind}
		if n := utf8.RuneCountInString(t); n > PromptRunes {
			p.Len, p.full = n, Runes(t, fullRunes)
		}
		s.Prompts = append(s.Prompts, p)
		if pt.kind == "" && IsCorrection(text, first) && ts != nil {
			s.FixTS = append(s.FixTS, *ts)
		}
	}
}

// Interrupt は、人が AI を途中で止めたことを記録する（プロンプトには数えない）。
// 中断の文を Prompt に渡さないアダプター（Amazon Q の CancelledToolUses など）が直接呼ぶ。
func (s *Builder) Interrupt(ts *float64) {
	s.Interrupts++
	if ts != nil {
		s.FixTS = append(s.FixTS, *ts)
		s.InterruptTS = append(s.InterruptTS, *ts)
	}
}

// Turn は、人の発言をここで読んだという合図。依頼を Prompt であとからまとめて足すアダプター（Codex）が、
// 応答を 1 ターンにつき 1 つに絞れるように呼ぶ。Prompt を呼ぶアダプターは呼ばなくてよい。
func (s *Builder) Turn() { s.turn++ }

// Reply は、エージェントが人に返した文を残す（ツール呼び出しや思考ではなく、会話に出た文）。
// 1 つの依頼につき最後の文だけを見せるので、同じターンの文は後から来たもので置き換える。
// key は 1 つの応答を何行かに分けて書き出すエージェント（Claude Code のメッセージ ID）のためのもので、
// 同じ key が続いたときはつなぐ。key が空なら置き換える。
func (s *Builder) Reply(ts *float64, key, text string) {
	if ts == nil || *ts == 0 {
		return
	}
	if text = strings.TrimSpace(text); text == "" {
		return
	}
	if n := len(s.replies); n > 0 && s.replies[n-1].turn == s.turn {
		if key != "" && s.replies[n-1].key == key {
			s.replies[n-1].text = Runes(s.replies[n-1].text+"\n"+text, fullRunes)
			return
		}
		s.replies[n-1] = pendingReply{t: ts, turn: s.turn, key: key, text: Runes(text, fullRunes)}
		return
	}
	s.replies = append(s.replies, pendingReply{t: ts, turn: s.turn, key: key, text: Runes(text, fullRunes)})
}

// withReplies は、依頼ごとに最後の応答を割り当てる（その依頼より後、次の依頼より前のもの）。
// 読んだ順ではなく時刻で決めるので、依頼をあとからまとめて足すアダプター（Codex）でも合う。
// 最初の依頼より前の応答（続きのセッションの頭など）は、どの依頼のものかわからないので捨てる。
func (s *Builder) withReplies(ps []Prompt) []Prompt {
	if len(s.replies) == 0 || len(ps) == 0 {
		return ps
	}
	type slot struct {
		t float64
		i int
	}
	var slots []slot
	for i := range ps {
		if ps[i].T != nil && *ps[i].T != 0 {
			slots = append(slots, slot{*ps[i].T, i})
		}
	}
	if len(slots) == 0 {
		return ps
	}
	sort.SliceStable(slots, func(i, j int) bool { return slots[i].t < slots[j].t })
	rs := append([]pendingReply(nil), s.replies...)
	sort.SliceStable(rs, func(i, j int) bool { return *rs[i].t < *rs[j].t })
	out := make([]Prompt, len(ps))
	copy(out, ps)
	for _, r := range rs {
		j := sort.Search(len(slots), func(k int) bool { return slots[k].t > *r.t }) - 1
		if j < 0 {
			continue
		}
		rep := Reply{T: r.t, Text: Runes(r.text, ReplyRunes)}
		if n := utf8.RuneCountInString(r.text); n > ReplyRunes {
			rep.Len, rep.full = n, r.text
		}
		out[slots[j].i].Reply = &rep // 時刻の順に見るので、同じ依頼では後の応答が残る
	}
	return out
}

// Limit は利用上限のエラーを記録する。同じ上限で続けて出たもの（1 分以内）は 1 回と数える。
// reset はエラー文にあった解除の時刻の文（LimitReset。なければ空）。
func (s *Builder) Limit(ts *float64, reset string) {
	if ts == nil {
		return
	}
	if n := len(s.Limits); n > 0 && *ts-s.Limits[n-1] < 60 {
		if s.LimitResets[n-1] == "" {
			s.LimitResets[n-1] = reset
		}
		return
	}
	s.Limits = append(s.Limits, *ts)
	s.LimitResets = append(s.LimitResets, reset)
}

// Compact は会話のコンパクション（要約して文脈を空けたこと）を記録する。
// 1 回のコンパクションを何行かで書くエージェント（Claude Code の compact_boundary と要約の行）のために、
// 続けて出たもの（1 分以内）は 1 回と数える。kind は "auto" か "manual"（ほかの値やわからないときは空）。
func (s *Builder) Compact(ts *float64, kind string) {
	if ts == nil || *ts == 0 {
		return
	}
	if kind != "auto" && kind != "manual" {
		kind = ""
	}
	if n := len(s.Compactions); n > 0 && math.Abs(*ts-s.Compactions[n-1]) < 60 {
		if s.CompactKinds[n-1] == "" {
			s.CompactKinds[n-1] = kind
		}
		return
	}
	s.Compactions = append(s.Compactions, *ts)
	s.CompactKinds = append(s.CompactKinds, kind)
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

// Edited は、ツールの引数に file_path の形で書かれない編集（Codex の apply_patch など）で変えたファイルを足す。
func (s *Builder) Edited(path string) {
	if path != "" {
		s.files[path] = true
	}
}

// HideText は、会話の中身から読んだものを消す（中身を残してはいけない会話のため。Kiro Crew の incognito・temporary の会話）。
// 依頼は時刻だけを残して文を text にする（依頼の数・作業時間・待ち時間は残る）。応答・仕組みが入れたもの・編集したファイル・
// 言い直しと中断（依頼の文から決めたもの）・利用上限の解除の時刻の文・PR の URL・サブエージェントへの依頼・履歴のファイル
// （kiroku serve が中身をそのまま見せるので）は消す。時刻・ツールの回数・モデル・使用量・クレジット・参考指標は残す。
// タイトルは呼ぶ側が決める。何度呼んでも同じ。
func (s *Builder) HideText(text string) {
	for i, p := range s.Prompts {
		s.Prompts[i] = Prompt{T: p.T, Text: text}
	}
	s.Notes, s.replies = nil, nil
	s.files = map[string]bool{}
	s.FixTS = nil
	s.Interrupts, s.InterruptTS = 0, nil
	for i := range s.LimitResets {
		s.LimitResets[i] = ""
	}
	for i := range s.Outputs {
		s.Outputs[i].URL = ""
	}
	for i := range s.Subagents {
		s.Subagents[i].Desc = ""
	}
	s.File = ""
}

// HideText は、組み立て終えたセッションから、会話の中身から読んだものを消す（Builder.HideText と同じものを消し、タイトルを title にする）。
// 別のエージェントの履歴に残る同じ会話（Kiro CLI の SQLite の写しなど）を、あとから隠すため。何度呼んでも同じ。
func (s *Session) HideText(text, title string) {
	for i, p := range s.Prompts {
		s.Prompts[i] = Prompt{T: p.T, Text: text, Work: p.Work, Wait: p.Wait}
	}
	s.Title, s.Notes, s.File = title, nil, ""
	s.Files, s.NFiles = []string{}, 0
	s.Fix, s.Corrections = nil, 0
	s.Interrupts, s.InterruptsAt = 0, []float64{}
	s.LimitResets = nil
	s.PRs = []string{}
	for i := range s.PRAt {
		s.PRAt[i].URL = ""
	}
	for i := range s.OEv {
		s.OEv[i].URL = ""
	}
	for i := range s.Subagents {
		s.Subagents[i].Desc = ""
	}
}

// Measure はそのエージェントだけが記録している数字を 1 つ足す。
func (s *Builder) Measure(key string, t *float64, v float64) {
	s.Measures = append(s.Measures, Measure{Key: key, T: t, V: v})
}

// ToolCounts はツールごとの回数。
func (s *Builder) ToolCounts() map[string]int { return s.tools }

// EditedFiles は編集したファイル（名前の順）。
func (s *Builder) EditedFiles() []string {
	out := make([]string, 0, len(s.files))
	for f := range s.files {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// TopModel はいちばん多く使ったモデル（同じ回数なら先に出てきたもの。なければ空）。
func (s *Builder) TopModel() string {
	top := ""
	for _, m := range s.modelOrder {
		if top == "" || s.models[m] > s.models[top] {
			top = m
		}
	}
	return top
}

// AddEvent は、メッセージ ID で重ねる必要のない使用量（Codex など）をそのまま足す。モデルも数える。
// e.Cost を入れておくと、そのコストを使う（nil なら料金表で見積もる。Usage.Events）。
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

// promptTimes は、依頼ごとに AI が動いていた秒数（Work）と、そのあと人が次の依頼を出すまでの秒数（Wait）を入れる。
// 依頼から次の依頼（最後の依頼なら終わり）までの間で、AI が最後に動いた時刻を区切りにする。時刻のない依頼と、その間に AI が動いていない依頼には入れない。
// サブエージェントが終わった時刻も、AI が動いていた時刻に入れる（親の会話に結果が残らなくても、待たせに数えないように）。
func (s *Builder) promptTimes(ps []Prompt) []Prompt {
	agent := append([]float64(nil), s.AgentTimes...)
	for _, a := range s.Subagents {
		if a.End != nil && *a.End != 0 {
			agent = append(agent, *a.End)
		}
	}
	sort.Float64s(agent)
	out := make([]Prompt, len(ps))
	copy(out, ps)
	for i := range out {
		if out[i].T == nil || *out[i].T == 0 {
			continue
		}
		p := *out[i].T
		next := math.Inf(1)
		for _, q := range out[i+1:] {
			if q.T != nil && *q.T > p {
				next = *q.T
				break
			}
		}
		// p より後で next より前の、いちばん遅い AI の時刻
		j := sort.Search(len(agent), func(k int) bool { return agent[k] >= next }) - 1
		if j < 0 || agent[j] <= p {
			continue
		}
		out[i].Work = Round(agent[j]-p, 0)
		if !math.IsInf(next, 1) {
			out[i].Wait = Round(next-agent[j], 0)
		}
	}
	return out
}

func (s *Builder) Corrections() int {
	n := 0
	for i, p := range s.Prompts {
		if IsCorrection(p.Text, i == 0) {
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
	Notes        []Note        `json:"notes,omitempty"` // エージェントや仕組みが会話に入れたもの（プロンプトには数えない）
	NPrompts     int           `json:"nPrompts"`
	Tools        [][2]any      `json:"tools"`
	Files        []string      `json:"files"`
	NFiles       int           `json:"nFiles"`
	Resume       *string       `json:"resume"`
	Waits        [][2]float64  `json:"waits"`
	Interrupts   int           `json:"interrupts"`
	InterruptsAt []float64     `json:"interruptsAt"`           // 中断した時刻
	Limits       []float64     `json:"limits"`                 // 利用上限に当たった時刻
	LimitResets  []string      `json:"limitResets,omitempty"`  // limits と同じ並びで、解除の時刻の文（"3:45pm" など。日付や時間帯のないこともある。なければ空）
	Compactions  []float64     `json:"compactions,omitempty"`  // 会話を要約して文脈を空けた（コンパクション）時刻
	CompactKinds []string      `json:"compactKinds,omitempty"` // compactions と同じ並びで "auto"・"manual"（わからなければ空。どれもわからなければ出さない）
	Ctx          []float64     `json:"ctx"`                    // 1 回の応答で読んだ入力（文脈）の大きさ [前半, 後半, 最大]（context.go）
	CtxWindow    float64       `json:"ctxWindow,omitempty"`    // ctx の最大の応答のモデルのコンテキストウィンドウ（わからなければ 0）
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
	PRAt         []PRAt        `json:"prAt"`                   // AI が PR を作った時刻（依頼の流れに出す）
	UEv          []Event       `json:"-"`                      // 週ごとの集計用（HTML には入れない）
	CEv          []Credit      `json:"-"`
	OEv          []Output      `json:"-"`
	Fix          []float64     `json:"-"`
	Meas         []Measure     `json:"-"`
	OutTracked   bool          `json:"-"` // アウトプットを記録できるエージェントのセッション（Records の outputs）
}

// PRAt は AI が PR を作った時刻と、わかれば URL。
type PRAt struct {
	T   float64 `json:"t"`
	URL string  `json:"url,omitempty"`
}

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
		project = "(unknown)"
	}
	title := s.Title
	if title == "" {
		if len(s.Prompts) > 0 {
			title = Runes(strings.SplitN(s.Prompts[0].Text, "\n", 2)[0], 80)
		} else {
			title = "(untitled)"
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
	prompts := s.withReplies(s.promptTimes(s.Prompts))
	main := s.Usage.Events()
	ctx := ContextGrowth(main)
	reported := false // エージェント自身の記録した使用料を目安コストに使ったか
	if len(s.Reported) > 0 {
		groups := [][]Event{main}
		for _, a := range s.Subagents {
			groups = append(groups, a.Events)
		}
		ex, used := applyReported(s.Reported, groups...)
		main, reported = append(main, ex...), used
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
	prs, prAt := []string{}, []PRAt{}
	for _, o := range s.Outputs {
		outs.Add(o)
		if o.Kind == "pr" && o.URL != "" {
			prs = append(prs, o.URL)
		}
		if o.Kind == "pr" && o.T != nil && *o.T != 0 {
			prAt = append(prAt, PRAt{T: *o.T, URL: o.URL})
		}
	}
	compactions, compactKinds := s.compactions()
	interrupts := append([]float64{}, s.InterruptTS...)
	sort.Float64s(interrupts)
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
		Prompts: prompts, Notes: s.Notes, NPrompts: len(s.Prompts), Tools: tools, Files: files, NFiles: nFiles, Resume: strOrNil(s.Resume),
		Waits: s.Waits(), Interrupts: s.Interrupts, InterruptsAt: interrupts, Limits: limits(s.Limits), LimitResets: limitResets(s.LimitResets), Compactions: compactions, CompactKinds: compactKinds, Ctx: ctx, CtxWindow: s.CtxWindow, Corrections: s.Corrections(), Models: models,
		Usage: mainSum, Subagents: subs, Credits: Round(credits, 3), Cost: Round(cost, 4),
		UEv: uev, CEv: s.Credits, OEv: s.Outputs, Outputs: outs, Fix: s.FixTS, CostReported: reported, File: s.File, PRs: prs, PRAt: prAt,
		Native: AggregateNative(s.Source, s.Measures), Meas: s.Measures, OutTracked: RecordsOf(s.Source, "outputs"),
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

func limits(xs []float64) []float64 {
	if xs == nil {
		return []float64{}
	}
	return xs
}

// compactions は、コンパクションの時刻を古い順に並べ、種類（auto・manual）も同じ順にそろえる。
// 種類がどれもわからなければ、種類は返さない（JSON に出さない）。
func (s *Builder) compactions() ([]float64, []string) {
	if len(s.Compactions) == 0 {
		return nil, nil
	}
	idx := make([]int, len(s.Compactions))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return s.Compactions[idx[a]] < s.Compactions[idx[b]] })
	ts, kinds := make([]float64, len(idx)), make([]string, len(idx))
	for i, j := range idx {
		ts[i] = s.Compactions[j]
		if j < len(s.CompactKinds) {
			kinds[i] = s.CompactKinds[j]
		}
	}
	return ts, limitResets(kinds)
}

// limitResets は、解除の時刻がどれか 1 つでもわかっていれば返す（なければ JSON に出さない）。
func limitResets(xs []string) []string {
	for _, x := range xs {
		if x != "" {
			return xs
		}
	}
	return nil
}
