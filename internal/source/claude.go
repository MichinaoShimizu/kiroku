package source

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Claude は Claude Code の履歴: <root>/<project>/<sessionId>.jsonl と <sessionId>/subagents/agent-<id>.jsonl。
// Archive は kiroku archive のコピーの場所（<保存場所>/claude。同じ並びで .jsonl.zst）。元の会話が消えていれば、コピーを読む。
type Claude struct{ Root, Archive string }

func (c *Claude) Name() string   { return "Claude Code" }
func (c *Claude) Family() string { return "claude" }
func (c *Claude) Where() string  { return c.Root }

// Retention は Claude Code の cleanupPeriodDays（既定 30 日。起動後に、それより古い会話の記録を黙って消す）。
// 利用者の設定（projects の隣の settings.json）だけを見る。プロジェクトの設定や組織の設定にある場合は読まない。
func (c *Claude) Retention() *Retention {
	r := &Retention{Days: 30, Setting: "cleanupPeriodDays", Docs: "https://code.claude.com/docs/en/settings-reference#cleanupperioddays"}
	if v, ok := core.Num(core.Map(core.ReadJSON(filepath.Join(filepath.Dir(c.Root), "settings.json")))["cleanupPeriodDays"]); ok && v >= 1 {
		r.Days, r.Set = int(v), true
	}
	return r
}

// サブエージェントのツール名。v2.1.63 で Task から Agent に変わった。
var subagentTools = map[string]bool{"Task": true, "Agent": true}

type subFile struct {
	agentID    string
	start, end *float64
	events     []core.Event
	modelOrder []string
	models     map[string]int
	tools      int
	outputs    []core.Output // サブエージェントが成功させたコミット・PR・編集した行
}

func loadSubagentFile(path string) subFile {
	u := core.NewUsage()
	f := subFile{models: map[string]int{}}
	pending := outputs{}
	var times []float64
	core.ReadJSONL(path, func(e core.Obj) {
		t := ts(e["timestamp"])
		if t != nil {
			times = append(times, *t)
		}
		msg := core.Map(e["message"])
		if core.Str(e["type"]) == "assistant" {
			m := core.ModelName(core.Str(msg["model"]))
			if m != "" && m != "<synthetic>" {
				if _, ok := f.models[m]; !ok {
					f.modelOrder = append(f.modelOrder, m)
				}
				f.models[m]++
			}
			u.Add(firstNonEmpty(core.Str(msg["id"]), core.Str(e["requestId"])), t, core.Str(msg["model"]), msg["usage"])
			for _, b := range core.List(msg["content"]) {
				if bm := core.Map(b); core.Str(bm["type"]) == "tool_use" {
					f.tools++
					pending.use(core.Str(bm["id"]), core.Str(bm["name"]), bm["input"], t)
				}
			}
		}
		if core.Str(e["type"]) == "user" {
			for _, b := range core.List(msg["content"]) {
				f.outputs = append(f.outputs, pending.result(core.Map(b), e, t)...)
			}
		}
	})
	stem := stemOf(path)
	f.agentID = strings.TrimPrefix(stem, "agent-")
	if len(times) > 0 {
		sort.Float64s(times)
		f.start, f.end = &times[0], &times[len(times)-1]
	}
	f.events = u.Events()
	return f
}

// outputs は、ツールの呼び出しから成果の印（コミット・PR・編集した行）を拾い、結果が成功だったときだけ返す。
type outputs map[string][]core.Output

func (p outputs) use(id, name string, input any, t *float64) {
	if o := core.Outputs(name, input, t); len(o) > 0 {
		p[id] = o
	}
}

// result は tool_result の行（block）から、成功した呼び出しの成果の印を返す。時刻は結果の時刻にする
// （許可の確認待ちや長い pre-commit で、実際のコミットは呼び出しより後になるため。git のコミットとの突き合わせに使う）。
func (p outputs) result(block core.Obj, e core.Obj, t *float64) []core.Output {
	id := core.Str(block["tool_use_id"])
	if core.Str(block["type"]) != "tool_result" || p[id] == nil {
		return nil
	}
	defer delete(p, id)
	if failed, _ := block["is_error"].(bool); failed {
		return nil
	}
	url := core.PRURL(core.TextOf(block["content"]) + " " + resultText(e["toolUseResult"]))
	var out []core.Output
	for _, o := range p[id] {
		if o.Kind == "pr" {
			o.URL = url
		}
		if t != nil {
			o.T = t
		}
		out = append(out, o)
	}
	return out
}

type call struct {
	typ, desc, agentID string
	bg                 bool
	start, end         *float64
	reported           map[string]float64
	reportedUsage      *core.Tokens
	file               *subFile
	side               []core.Event
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

func (c *Claude) Load(emit func(*core.Builder)) error {
	for _, u := range c.Units() {
		c.LoadUnit(u, emit)
	}
	return nil
}

// Keep は、kiroku archive で残す場所（Claude Code は古い会話を消すため）。
func (c *Claude) Keep() []Kept {
	if c.Archive == "" {
		return nil
	}
	return []Kept{{Src: c.Root, Dst: c.Archive}}
}

// Units は会話ごとのファイルと、そのサブエージェントのファイル。元の会話が消えていて kiroku archive のコピーがあれば、コピーを読む。
func (c *Claude) Units() []Unit {
	var out []Unit
	for _, path := range glob(filepath.Join(c.Root, "*", "*.jsonl")) {
		out = append(out, claudeUnit(path, ".jsonl"))
	}
	if c.Archive != "" {
		for _, path := range glob(filepath.Join(c.Archive, "*", "*.jsonl.zst")) {
			if isFile(filepath.Join(c.Root, filepath.Base(filepath.Dir(path)), stemOf(path)+".jsonl")) {
				continue // 元の会話があれば、そちらを読む
			}
			out = append(out, claudeUnit(path, ".jsonl.zst"))
		}
	}
	return out
}

func claudeUnit(path, ext string) Unit {
	files := append([]string{path}, glob(filepath.Join(filepath.Dir(path), stemOf(path), "subagents", "*"+ext))...)
	return Unit{Key: path, Files: files}
}

// stemOf は履歴ファイルの名前から .jsonl（kiroku archive のコピーなら .jsonl.zst）を除いたもの。
func stemOf(path string) string {
	return strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), ".zst"), ".jsonl")
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// LoadUnit は 1 つの会話（Units の 1 つ）を読む。
func (c *Claude) LoadUnit(u Unit, emit func(*core.Builder)) error {
	path := u.Key
	stem := stemOf(path)
	s := core.NewBuilder("Claude Code", stem)
	s.TracksOutputs = true
	s.File = path
	var summaries []string
	calls := map[string]*call{}
	var callOrder []*call
	side := core.NewUsage()
	pending := outputs{} // 成果の印は、ツールの結果が成功だったときだけ数える
	var lastT *float64
	procs := map[float64]*core.ReportedCost{} // Claude Code 自身の使用料（cost-state）。プロセスの起動時刻ごとに最新の累計
	var procOrder []float64
	core.ReadJSONL(path, func(e core.Obj) {
		typ := core.Str(e["type"])
		if typ == "summary" && core.Str(e["summary"]) != "" {
			summaries = append(summaries, core.Str(e["summary"]))
			return
		}
		if (typ == "custom-title" || typ == "ai-title") && (core.Str(e["title"]) != "" || core.Str(e["customTitle"]) != "") {
			s.Title = firstNonEmpty(core.Str(e["customTitle"]), core.Str(e["title"]))
			return
		}
		if typ == "cost-state" {
			if r := readCostState(e, lastT); r != nil {
				if procs[r.From] == nil {
					procOrder = append(procOrder, r.From)
				}
				procs[r.From] = r
			}
			return
		}
		t := ts(e["timestamp"])
		if t == nil {
			return
		}
		lastT = t
		if typ == "assistant" {
			s.Agent(t)
		} else {
			s.Tick(t)
		}
		if s.Project == "" {
			s.Project = core.Str(e["cwd"])
		}
		if s.Branch == "" {
			s.Branch = core.Str(e["gitBranch"])
		}
		msg := core.Map(e["message"])
		sidechain, _ := e["isSidechain"].(bool)
		meta, _ := e["isMeta"].(bool)
		if typ == "assistant" {
			m := core.ModelName(core.Str(msg["model"]))
			// 古い版はサブエージェントの発言も同じファイルに isSidechain つきで混ざる
			target := s.Usage
			if sidechain {
				target = side
			}
			target.Add(firstNonEmpty(core.Str(msg["id"]), core.Str(e["requestId"])), t, core.Str(msg["model"]), msg["usage"])
			if !sidechain {
				s.Model(m)
			}
			// 利用上限のエラーは、Claude Code が作った発言（isApiErrorMessage・モデル <synthetic>）として残る
			if apiErr, _ := e["isApiErrorMessage"].(bool); (apiErr || core.Str(msg["model"]) == "<synthetic>") && core.IsLimitError(core.TextOf(msg["content"])) {
				s.Limit(t)
			}
		}
		// 作業中に送った依頼は、user の行ではなく queued_command の添付だけに残る（absorbed_mid_turn）。
		// 人が送ったもの（origin.kind が human）だけを依頼に数え、ほかのエージェントや通知からのものは除く
		if a := core.Map(e["attachment"]); typ == "attachment" && core.Str(a["type"]) == "queued_command" && !sidechain {
			human, _ := a["humanTurn"].(bool)
			if k := core.Str(core.Map(a["origin"])["kind"]); (k == "human" || k == "" && human) && core.Str(a["commandMode"]) != "task-notification" {
				s.Prompt(t, core.TextOf(a["prompt"]))
			}
			return
		}
		// 会話が長くなって自動で要約したときの「This session is being continued…」は、依頼ではない
		compact, _ := e["isCompactSummary"].(bool)
		if typ == "user" && !meta && !sidechain && !compact {
			s.Prompt(t, core.TextOf(msg["content"]))
			for _, b := range core.List(msg["content"]) {
				bm := core.Map(b)
				s.Outputs = append(s.Outputs, pending.result(bm, e, t)...)
				c := calls[core.Str(bm["tool_use_id"])]
				if core.Str(bm["type"]) != "tool_result" || c == nil {
					continue
				}
				c.end = t
				if r := core.Map(e["toolUseResult"]); r != nil {
					c.agentID = firstNonEmpty(core.Str(r["agentId"]), c.agentID)
					c.reported = map[string]float64{}
					for _, k := range []string{"totalTokens", "totalDurationMs", "totalToolUseCount"} {
						if v, ok := core.Num(r[k]); ok {
							c.reported[k] = v
						}
					}
					if core.Map(r["usage"]) != nil {
						u := core.ReadUsage(r["usage"])
						c.reportedUsage = &u
					}
				}
			}
		} else if typ == "assistant" && !sidechain {
			for _, b := range core.List(msg["content"]) {
				bm := core.Map(b)
				if core.Str(bm["type"]) != "tool_use" {
					continue
				}
				name := core.Str(bm["name"])
				s.Tool(name, bm["input"])
				s.Measure("tool_calls", t, 1)
				pending.use(core.Str(bm["id"]), name, bm["input"], t)
				if subagentTools[name] {
					inp := core.Map(bm["input"])
					bg, _ := inp["run_in_background"].(bool)
					c := &call{typ: firstNonEmpty(core.Str(inp["subagent_type"]), "general-purpose"),
						desc: core.Runes(core.Str(inp["description"]), 120), bg: bg, start: t}
					calls[core.Str(bm["id"])] = c
					callOrder = append(callOrder, c)
				}
			}
		}
	})
	// サブエージェントの別ファイル（新しい版）を、agentId か時刻でつなぐ
	var files []subFile
	for _, f := range u.Files[1:] {
		sf := loadSubagentFile(f)
		files = append(files, sf)
		s.Outputs = append(s.Outputs, sf.outputs...) // サブエージェントに任せた編集・コミット・PR も、そのセッションの成果
	}
	order := append([]*call(nil), callOrder...)
	sort.SliceStable(order, func(i, j int) bool { return val(order[i].start) < val(order[j].start) })
	for i := range files {
		f := &files[i]
		var c *call
		for _, x := range order {
			if x.agentID != "" && x.agentID == f.agentID {
				c = x
				break
			}
		}
		if c == nil && f.start != nil {
			for _, x := range order {
				end := *f.start
				if x.end != nil {
					end = *x.end
				}
				if x.file == nil && x.start != nil && *x.start-5 <= *f.start && *f.start <= end+5 {
					c = x
					break
				}
			}
		}
		if c == nil {
			c = &call{typ: "subagent", start: f.start, end: f.end}
			order = append(order, c)
		}
		c.file = f
	}
	var sideEvs []core.Event
	if len(files) == 0 {
		// 古い形式: isSidechain の行を、呼び出しの時間帯で振り分ける
		for _, ev := range side.Events() {
			var c *call
			for _, x := range order {
				if ev.T == nil || x.start == nil {
					continue
				}
				end := *x.start
				if x.end != nil {
					end = *x.end
				}
				if *x.start-5 <= *ev.T && *ev.T <= end+5 {
					c = x
					break
				}
			}
			if c != nil {
				c.side = append(c.side, ev)
			} else {
				sideEvs = append(sideEvs, ev)
			}
		}
	}
	for _, c := range order {
		var evs []core.Event
		if c.file != nil {
			evs = c.file.events
		} else {
			evs = c.side
		}
		if len(evs) == 0 && c.reportedUsage != nil {
			evs = []core.Event{{T: c.start, U: *c.reportedUsage}}
		}
		tot := core.SumUsage(evs)
		if c.file == nil && c.reportedUsage == nil && c.reported["totalTokens"] != 0 {
			tot.ReportedTokens = c.reported["totalTokens"]
		}
		end := c.end
		if c.file != nil && c.file.end != nil {
			end = c.file.end
		}
		if end == nil && c.reported["totalDurationMs"] != 0 && c.start != nil {
			v := *c.start + c.reported["totalDurationMs"]/1000
			end = &v
		}
		var model *string
		tools := c.reported["totalToolUseCount"]
		if c.file != nil {
			if len(c.file.modelOrder) > 0 {
				best := c.file.modelOrder[0]
				for _, m := range c.file.modelOrder {
					if c.file.models[m] > c.file.models[best] {
						best = m
					}
				}
				model = &best
			}
			if c.file.tools > 0 {
				tools = float64(c.file.tools)
			}
		} else if len(c.side) > 0 && c.side[0].Model != "" {
			m := c.side[0].Model
			model = &m
		}
		s.Subagents = append(s.Subagents, core.Subagent{Type: c.typ, Desc: c.desc, BG: c.bg, Start: c.start, End: end,
			Model: model, Tools: tools, Usage: tot, Events: evs})
	}
	if len(sideEvs) > 0 { // 古い形式: どの呼び出しにも入らなかった分は 1 つにまとめる
		var lo, hi *float64
		for _, ev := range sideEvs {
			if ev.T == nil {
				continue
			}
			if lo == nil || *ev.T < *lo {
				lo = ev.T
			}
			if hi == nil || *ev.T > *hi {
				hi = ev.T
			}
		}
		s.Subagents = append(s.Subagents, core.Subagent{Type: "sidechain", Start: lo, End: hi, Usage: core.SumUsage(sideEvs), Events: sideEvs})
	}
	for _, ev := range s.Usage.Events() { // 1 つの応答は 1 回だけ（メッセージ ID でまとめたあと）
		s.Measure("responses", ev.T, 1)
		s.Measure("out_per_response", ev.T, ev.U.Out)
		s.Measure("cache_read", ev.T, ev.U.CR)
		s.Measure("input_all", ev.T, ev.U.In+ev.U.CW+ev.U.CW1h+ev.U.CR)
	}
	for _, a := range s.Subagents {
		s.Measure("subagents", a.Start, 1)
	}
	for _, k := range procOrder {
		s.Reported = append(s.Reported, *procs[k])
	}
	if s.Title == "" && len(summaries) > 0 {
		s.Title = summaries[len(summaries)-1]
	}
	if s.Project == "" {
		s.Project = filepath.Base(filepath.Dir(path))
	}
	if !strings.HasSuffix(path, ".zst") { // 消えた会話（kiroku archive のコピー）は Claude Code で再開できない
		s.Resume = "cd " + s.Project + " && claude --resume " + s.ID
	}
	emit(s)
	return nil
}

func val(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// readCostState は Claude Code の cost-state（プロセスの起動からの累計）を読む。
// 行に時刻がないので、直前の行の時刻までを対象の期間とする。
func readCostState(e core.Obj, lastT *float64) *core.ReportedCost {
	start, ok := core.Num(e["startTime"])
	if !ok || lastT == nil {
		return nil
	}
	r := &core.ReportedCost{From: start / 1000, To: *lastT, Models: map[string]core.ReportedModel{}}
	for name, v := range core.Map(e["modelUsage"]) {
		m := core.Map(v)
		cost, ok := core.Num(m["costUSD"])
		if !ok {
			continue
		}
		u := core.Tokens{In: core.NumOr0(m["inputTokens"]), Out: core.NumOr0(m["outputTokens"]),
			CW: core.NumOr0(m["cacheCreationInputTokens"]), CR: core.NumOr0(m["cacheReadInputTokens"])}
		model := core.ModelName(name)
		prev := r.Models[model] // 日付の版違いは 1 つにまとめる
		prev.Cost += cost
		prev.U = core.Tokens{In: prev.U.In + u.In, Out: prev.U.Out + u.Out, CW: prev.U.CW + u.CW, CR: prev.U.CR + u.CR}
		r.Models[model] = prev
	}
	if len(r.Models) == 0 || r.To < r.From {
		return nil
	}
	return r
}

// resultText はツールの結果（toolUseResult）を文字列にする。PR の URL を探すため。
func resultText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}
