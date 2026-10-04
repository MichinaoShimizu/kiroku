package source

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Claude は Claude Code の履歴: <root>/<project>/<sessionId>.jsonl と <sessionId>/subagents/agent-<id>.jsonl。
type Claude struct{ Root string }

func (c *Claude) Name() string   { return "Claude Code" }
func (c *Claude) Family() string { return "claude" }
func (c *Claude) Where() string  { return c.Root }

// サブエージェントのツール名。v2.1.63 で Task から Agent に変わった。
var subagentTools = map[string]bool{"Task": true, "Agent": true}

type subFile struct {
	agentID    string
	start, end *float64
	events     []core.Event
	modelOrder []string
	models     map[string]int
	tools      int
}

func loadSubagentFile(path string) subFile {
	u := core.NewUsage()
	f := subFile{models: map[string]int{}}
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
				if core.Str(core.Map(b)["type"]) == "tool_use" {
					f.tools++
				}
			}
		}
	})
	stem := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	f.agentID = strings.TrimPrefix(stem, "agent-")
	if len(times) > 0 {
		sort.Float64s(times)
		f.start, f.end = &times[0], &times[len(times)-1]
	}
	f.events = u.Events()
	return f
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
	for _, path := range glob(filepath.Join(c.Root, "*", "*.jsonl")) {
		stem := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		s := core.NewBuilder("Claude Code", stem)
		s.File = path
		var summaries []string
		calls := map[string]*call{}
		var callOrder []*call
		side := core.NewUsage()
		pending := map[string][]core.Output{} // 成果の印は、ツールの結果が成功だったときだけ数える
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
			if typ == "user" && !meta && !sidechain {
				s.Prompt(t, core.TextOf(msg["content"]))
				for _, b := range core.List(msg["content"]) {
					bm := core.Map(b)
					if id := core.Str(bm["tool_use_id"]); core.Str(bm["type"]) == "tool_result" && pending[id] != nil {
						if failed, _ := bm["is_error"].(bool); !failed {
							url := core.PRURL(core.TextOf(bm["content"]) + " " + resultText(e["toolUseResult"]))
							for _, o := range pending[id] {
								if o.Kind == "pr" {
									o.URL = url
								}
								s.Outputs = append(s.Outputs, o)
							}
						}
						delete(pending, id)
					}
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
					if o := core.Outputs(name, bm["input"], t); len(o) > 0 {
						pending[core.Str(bm["id"])] = o
					}
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
		for _, f := range glob(filepath.Join(filepath.Dir(path), stem, "subagents", "*.jsonl")) {
			files = append(files, loadSubagentFile(f))
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
		s.Resume = "cd " + s.Project + " && claude --resume " + s.ID
		emit(s)
	}
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
