package source

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Codex は OpenAI Codex CLI の履歴: <CODEX_HOME か ~/.codex>/sessions/YYYY/MM/DD/rollout-*.jsonl（新しい版は .jsonl.zst）
// と archived_sessions/。1 行は {timestamp, type, payload}。
//
//	session_meta  … id（スレッド ID）、cwd、git.branch、source（サブエージェントなら source.subagent.thread_spawn）
//	turn_context  … model
//	event_msg     … user_message（依頼）、agent_message、token_count（info.total_token_usage / last_token_usage）
//	response_item … function_call / custom_tool_call（ツール）、message
//	token_usage_record … 新しい版の使用量（あればこちらを使い、token_count は使わない）
//
// サブエージェントやフォークのファイルは、親の履歴を先頭にそのまま写している。
// そのため、ファイル自身の session_meta の時刻より前の行は数えない。
type Codex struct {
	Home string

	mu    sync.Mutex
	heads map[string]codexHead // ファイルごとの session_meta（Units で親子をまとめるため。印が同じなら読み直さない）
}

// codexHead は、ファイルの先頭の session_meta から読んだスレッド ID と親のスレッド ID。
type codexHead struct {
	stamp      string
	ok         bool // session_meta がある
	id, parent string
	err        error // 先頭を読めなかった（壊れた .zst など）
}

func (c *Codex) Name() string   { return "Codex" }
func (c *Codex) Family() string { return "codex" }
func (c *Codex) Where() string  { return c.Home }

// Watch は会話が入っている場所だけ。~/.codex の下にはログ（log/codex-tui.log）や history.jsonl などもあり、
// ずっと書き換わるので、Home ごと見張ると何か書かれるたびに全部を読み直してしまう。
func (c *Codex) Watch() []string {
	return []string{filepath.Join(c.Home, "sessions"), filepath.Join(c.Home, "archived_sessions"), filepath.Join(c.Home, "session_index.jsonl")}
}

// DefaultCodexHome は CODEX_HOME か ~/.codex。
func DefaultCodexHome() string {
	if d := os.Getenv("CODEX_HOME"); d != "" {
		return d
	}
	return filepath.Join(home(), ".codex")
}

type codexFile struct {
	path                  string
	id, parent, role, cwd string
	branch, model, title  string
	start                 *float64
	b                     *core.Builder
	events                []core.Event
}

func (c *Codex) files() []string {
	var out []string
	seen := map[string]bool{} // archived_sessions と同じファイル名は 1 回だけ
	add := func(ps []string) {
		for _, p := range ps {
			base := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(p), ".zst"), ".jsonl")
			if !seen[base] {
				seen[base] = true
				out = append(out, p)
			}
		}
	}
	for _, ext := range []string{"*.jsonl", "*.jsonl.zst"} {
		add(glob(filepath.Join(c.Home, "sessions", "*", "*", "*", ext)))
	}
	for _, ext := range []string{"*.jsonl", "*.jsonl.zst"} {
		add(glob(filepath.Join(c.Home, "archived_sessions", ext)))
	}
	return out
}

// titles は session_index.jsonl（{id, thread_name}、後のほうが新しい）。
func (c *Codex) titles() map[string]string {
	out := map[string]string{}
	core.ReadJSONL(filepath.Join(c.Home, "session_index.jsonl"), func(e core.Obj) {
		if id, name := core.Str(e["id"]), core.Str(e["thread_name"]); id != "" && name != "" {
			out[id] = name
		}
	})
	return out
}

// codexTokens は Codex の使用量 → Tokens。cached_input_tokens は input_tokens に含まれている。
// itemText は item_completed の発言の文（新しい形は message、古い形は content の text）。
func itemText(item core.Obj) string {
	if text := firstNonEmpty(core.Str(item["message"]), core.Str(item["text"])); text != "" {
		return text
	}
	var parts []string
	for _, b := range core.List(item["content"]) {
		m := core.Map(b)
		parts = append(parts, firstNonEmpty(core.Str(m["text"]), core.Str(m["message"])))
	}
	return strings.Join(parts, "\n")
}

func codexTokens(u any) core.Tokens {
	m := core.Map(u)
	in, cached := core.NumOr0(m["input_tokens"]), core.NumOr0(m["cached_input_tokens"])
	return core.Tokens{In: max(0, in-cached), Out: core.NumOr0(m["output_tokens"]), CW: core.NumOr0(m["cache_write_input_tokens"]), CR: cached}
}

// Load は全部の会話を読む。読めないファイルがあっても残りは読み、最初のエラー（と残りの数）を返す。
func (c *Codex) Load(emit func(*core.Builder)) error {
	var errs fileErrs
	for _, u := range c.Units() {
		errs.add(c.LoadUnit(u, emit))
	}
	return errs.err()
}

// Units は、親のスレッドのファイルと、そのサブエージェント（孫も）のファイルのまとまり。
// サブエージェントは親のセッションにまとめるので、親子は一緒に読み直す。
// スレッド名（session_index.jsonl）は Tag に入れる。名前が変わったまとまりだけ読み直す。
func (c *Codex) Units() []Unit {
	if !isDir(c.Home) {
		return nil
	}
	files := c.files()
	heads := c.readHeads(files)
	parentOf := map[string]string{}
	for _, h := range heads {
		if h.ok && h.id != "" {
			parentOf[h.id] = h.parent
		}
	}
	// root はいちばん上の親のスレッド ID（親のファイルがなければ自分）
	root := func(id string) string {
		seen := map[string]bool{}
		for !seen[id] {
			seen[id] = true
			p, ok := parentOf[id]
			if !ok || p == "" {
				break
			}
			if _, ok := parentOf[p]; !ok {
				break
			}
			id = p
		}
		return id
	}
	titles := c.titles()
	var out []Unit
	at := map[string]int{}
	for i, path := range files {
		h := heads[i]
		if !h.ok {
			if h.err != nil { // 先頭すら読めなかったファイルは、それだけで 1 つにして LoadUnit でエラーを返す
				out = append(out, Unit{Key: path, Files: []string{path}})
			}
			continue // session_meta のないファイルは読まない（Load と同じ）
		}
		key := path
		if h.id != "" {
			key = root(h.id)
		}
		if j, ok := at[key]; ok {
			out[j].Files = append(out[j].Files, path)
			continue
		}
		at[key] = len(out)
		out = append(out, Unit{Key: path, Files: []string{path}, Tag: titles[key]})
	}
	return out
}

// readHeads は files それぞれの session_meta。前回と印が同じファイルは読み直さない。
func (c *Codex) readHeads(files []string) []codexHead {
	c.mu.Lock()
	defer c.mu.Unlock()
	next := map[string]codexHead{}
	out := make([]codexHead, len(files))
	for i, p := range files {
		stamp := Stamp([]string{p})
		h, ok := c.heads[p]
		if !ok || h.stamp != stamp {
			h = codexHead{stamp: stamp}
			h.id, h.parent, h.ok, h.err = readCodexHead(p)
		}
		next[p], out[i] = h, h
	}
	c.heads = next
	return out
}

// readCodexHead は、最初の session_meta までだけ読む（ふつうは 1 行目）。
// err は、session_meta にたどり着く前に読めなくなったとき（消えていたときは nil）。
func readCodexHead(path string) (id, parent string, ok bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", false, fileErr(path, err)
	}
	defer f.Close()
	var src io.Reader = f
	if strings.HasSuffix(path, ".zst") {
		d, err := core.NewZstdReader(f)
		if err != nil {
			return "", "", false, fileErr(path, err)
		}
		defer d.Close()
		src = d
	}
	r := bufio.NewReaderSize(src, 1<<16)
	for {
		line, long, err := core.ReadLine(r, core.MaxLine) // 長すぎる行は飛ばす（LoadUnit が読めない行として知らせる）
		var e core.Obj
		if !long && json.Unmarshal(line, &e) == nil && core.Str(e["type"]) == "session_meta" {
			p := core.Map(e["payload"])
			return core.Str(p["id"]), codexParent(p), true, nil
		}
		if err == io.EOF {
			return "", "", false, nil
		}
		if err != nil {
			return "", "", false, fileErr(path, err)
		}
	}
}

// codexParent は、サブエージェントのスレッドの親のスレッド ID（サブエージェントでなければ空）。
func codexParent(p core.Obj) string {
	spawn := core.Map(core.Get(p, "source", "subagent", "thread_spawn"))
	return firstNonEmpty(core.Str(spawn["parent_thread_id"]), core.Str(p["parent_thread_id"]))
}

// LoadUnit は 1 つのまとまり（Units の 1 つ）を読む。
// 読めないファイルがあっても、読めた分は出す。
func (c *Codex) LoadUnit(u Unit, emit func(*core.Builder)) error {
	var errs fileErrs
	var all []*codexFile
	for _, path := range u.Files {
		cf := &codexFile{path: path}
		var userMsgs, fallback []struct {
			t    *float64
			text string
		}
		var lastTotal float64 = -1
		var lastInfo string
		var prevTotal core.Tokens
		records := map[string]bool{}
		var fromCounts, fromRecords []core.Event
		var countMeas, recordMeas []core.Measure
		errs.file(path, core.ReadJSONL(path, func(e core.Obj) {
			t := ts(e["timestamp"])
			p := core.Map(e["payload"])
			typ := core.Str(e["type"])
			if typ == "session_meta" {
				if cf.id == "" {
					cf.id = core.Str(p["id"])
					cf.cwd = core.Str(p["cwd"])
					cf.branch = core.Str(core.Get(p, "git", "branch"))
					cf.start = ts(p["timestamp"])
					if cf.start == nil {
						cf.start = t
					}
					spawn := core.Map(core.Get(p, "source", "subagent", "thread_spawn"))
					cf.parent = codexParent(p)
					cf.role = firstNonEmpty(core.Str(spawn["agent_role"]), core.Str(p["agent_role"]), core.Str(spawn["agent_nickname"]), core.Str(p["agent_nickname"]))
					cf.b = core.NewBuilder("Codex", cf.id)
					cf.b.File = cf.path
				}
				return
			}
			if cf.b == nil || t == nil || (cf.start != nil && *t < *cf.start) {
				return // 親から写した履歴は数えない
			}
			s := cf.b
			switch typ {
			case "turn_context":
				if m := core.Str(p["model"]); m != "" {
					cf.model = m
				}
				if cwd := core.Str(p["cwd"]); cwd != "" && cf.cwd == "" {
					cf.cwd = cwd
				}
				s.Agent(t)
			case "event_msg":
				switch core.Str(p["type"]) {
				case "user_message":
					s.Tick(t)
					s.Turn() // 依頼はあとでまとめて足すので、ここで応答の区切りを入れる
					userMsgs = append(userMsgs, struct {
						t    *float64
						text string
					}{t, core.Str(p["message"])})
				case "agent_message": // 人に返した文（依頼の流れに出す）
					s.Agent(t)
					s.Reply(t, "", core.Str(p["message"]))
				case "token_count":
					s.Agent(t)
					if v, ok := core.Num(core.Get(p, "rate_limits", "primary", "used_percent")); ok {
						s.Measure("rate_limit", t, v)
					}
					info := core.Map(p["info"])
					if info == nil {
						return
					}
					raw, _ := json.Marshal(info)
					total := core.NumOr0(core.Get(info, "total_token_usage", "total_tokens"))
					if string(raw) == lastInfo || total == lastTotal {
						return // 同じ値の書き直し
					}
					cur := codexTokens(info["total_token_usage"])
					u := cur // 1 回目は合計がそのまま 1 回分
					if last := core.Map(info["last_token_usage"]); last != nil {
						u = codexTokens(last)
					} else if lastTotal >= 0 { // 古い形: 前回の合計との差
						u = core.Tokens{In: max(0, cur.In-prevTotal.In), Out: max(0, cur.Out-prevTotal.Out),
							CW: max(0, cur.CW-prevTotal.CW), CR: max(0, cur.CR-prevTotal.CR)}
					}
					lastInfo, lastTotal, prevTotal = string(raw), total, cur
					fromCounts = append(fromCounts, core.Event{T: t, U: u, Model: cf.model})
					countMeas = append(countMeas, codexMeasures(t, core.Map(info["last_token_usage"]), core.NumOr0(info["model_context_window"]))...)
				case "item_completed":
					// Paginated rollouts persist user turns as ItemCompleted(UserMessage)
					// instead of the legacy UserMessage event.
					item := core.Map(p["item"])
					switch core.Str(item["type"]) {
					case "user_message":
						s.Tick(t)
						s.Turn() // 依頼はあとでまとめて足すので、ここで応答の区切りを入れる
						if text := itemText(item); text != "" {
							userMsgs = append(userMsgs, struct {
								t    *float64
								text string
							}{t, text})
						}
					case "agent_message": // 人に返した文（依頼の流れに出す）
						s.Agent(t)
						s.Reply(t, "", itemText(item))
					default:
						s.Agent(t)
					}
				default:
					s.Agent(t)
				}
			case "token_usage_record":
				s.Agent(t)
				id := core.Str(p["response_id"])
				if id != "" && records[id] {
					return
				}
				records[id] = true
				fromRecords = append(fromRecords, core.Event{T: t, U: codexTokens(p["usage"]), Model: cf.model})
				recordMeas = append(recordMeas, codexMeasures(t, core.Map(p["usage"]), 0)...)
			case "response_item":
				switch core.Str(p["type"]) {
				case "function_call":
					var args any
					json.Unmarshal([]byte(core.Str(p["arguments"])), &args)
					s.Tool(core.Str(p["name"]), args)
					s.Measure("tool_calls", t, 1)
					s.Agent(t)
				case "custom_tool_call":
					s.Tool(core.Str(p["name"]), nil)
					s.Measure("tool_calls", t, 1)
					s.Agent(t)
				case "message":
					var parts []string
					for _, b := range core.List(p["content"]) {
						parts = append(parts, core.Str(core.Map(b)["text"]))
					}
					if core.Str(p["role"]) == "user" {
						s.Tick(t)
						s.Turn() // 依頼はあとでまとめて足すので、ここで応答の区切りを入れる
						fallback = append(fallback, struct {
							t    *float64
							text string
						}{t, strings.Join(parts, "\n")})
					} else {
						s.Agent(t)
						s.Reply(t, "", strings.Join(parts, "\n")) // 人に返した文
					}
				default:
					s.Agent(t)
				}
			default:
				s.Agent(t)
			}
		}))
		if cf.b == nil {
			continue
		}
		pending, meas := fromCounts, countMeas
		if len(fromRecords) > 0 { // 新しい版は token_usage_record だけを使う（token_count と同じものを重ねて書いている）
			pending, meas = fromRecords, recordMeas
			for _, m := range countMeas { // コンテキストの使用率は token_count にしかない
				if m.Key == "context_used" {
					meas = append(meas, m)
				}
			}
		}
		cf.b.Measures = append(cf.b.Measures, meas...)
		// 依頼の文は event_msg.user_message を使う。ない古い版だけ response_item の user を使う
		msgs := userMsgs
		if len(msgs) == 0 {
			msgs = fallback
		}
		for _, m := range msgs {
			cf.b.Prompt(m.t, m.text)
		}
		for i := range pending {
			if c, ok := core.CostOf(pending[i].Model, pending[i].U); ok {
				pending[i].Cost = &c
			}
		}
		cf.events = pending
		cf.title = u.Tag
		all = append(all, cf)
	}
	// サブエージェントのファイルは親のセッションにまとめる
	byID := map[string]*codexFile{}
	for _, cf := range all {
		byID[cf.id] = cf
	}
	for _, cf := range all {
		if cf.parent == "" || byID[cf.parent] == nil {
			continue
		}
		parent := byID[cf.parent].b
		parent.Measures = append(parent.Measures, cf.b.Measures...) // サブエージェントの分も親のセッションの数字に入れる
		var start, end *float64
		if len(cf.b.Times) > 0 {
			ts := append([]float64(nil), cf.b.Times...)
			sort.Float64s(ts)
			start, end = &ts[0], &ts[len(ts)-1]
		}
		var model *string
		if cf.model != "" {
			model = &cf.model
		}
		tools := 0.0
		for _, n := range cf.b.ToolCounts() {
			tools += float64(n)
		}
		parent.Subagents = append(parent.Subagents, core.Subagent{Type: firstNonEmpty(cf.role, "subagent"), Desc: firstPrompt(cf.b),
			Start: start, End: end, Model: model, Tools: tools, Usage: core.SumUsage(cf.events), Events: cf.events})
	}
	for _, cf := range all {
		if cf.parent != "" && byID[cf.parent] != nil {
			continue
		}
		s := cf.b
		s.Project, s.Branch = cf.cwd, cf.branch
		s.Title = cf.title
		for _, ev := range cf.events {
			s.AddEvent(ev)
		}
		if s.Project != "" {
			s.Resume = core.ResumeCmd(s.Project, "codex resume", s.ID)
		}
		emit(s)
	}
	return errs.err()
}

// codexMeasures は 1 回の応答の使用量から参考指標を作る。
func codexMeasures(t *float64, u core.Obj, window float64) []core.Measure {
	if u == nil {
		return nil
	}
	in := core.NumOr0(u["input_tokens"])
	ms := []core.Measure{
		{Key: "responses", T: t, V: 1},
		{Key: "reasoning", T: t, V: core.NumOr0(u["reasoning_output_tokens"])},
		{Key: "output", T: t, V: core.NumOr0(u["output_tokens"])},
	}
	if window > 0 {
		ms = append(ms, core.Measure{Key: "context_used", T: t, V: in / window})
	}
	return ms
}

func firstPrompt(b *core.Builder) string {
	if len(b.Prompts) == 0 {
		return ""
	}
	return core.Runes(strings.SplitN(b.Prompts[0].Text, "\n", 2)[0], 120)
}
