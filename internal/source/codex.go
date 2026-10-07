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
//	event_msg     … user_message（依頼）、agent_message、token_count（info.total_token_usage / last_token_usage）、
//	                item_completed（history_mode が paginated の版の発言。item.type は UserMessage / AgentMessage）
//	response_item … function_call / custom_tool_call（ツール）、message（role が assistant なら応答）
//	token_usage_record … 新しい版の使用量（あればこちらを使い、token_count は使わない）
//
// サブエージェントやフォークのファイルは、親の履歴を自分の session_meta のあとにそのまま写している。
// 写した行の時刻は写したときのものなので、時刻では分けられない。写しの終わりは次の印で見分ける（codexFile.skip）。
//
//   - session_meta.subagent_history_start_ordinal があれば（paginated のサブエージェント）、行の ordinal がそれより前の行
//   - フォークかサブエージェントなら、自分のスレッド ID の thread_settings_applied より前の行
//     （写しと一緒に書く。写した親の thread_settings_applied は親のスレッド ID のまま。
//     印のない古い版で作ったファイルも再開すると書くので、写しと一緒に書いたものだけを印とみなす: codexSameWrite）
//   - どちらの印もない古い版は、これまでどおり session_meta の時刻より前の行
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
	skip                  codexSkip
	startOrdinal          float64 // skip が codexSkipOrdinal のとき、自分の最初の行の ordinal
}

// codexSkip は、親から写した行をどう見分けるか。
type codexSkip int

const (
	codexSkipTime    codexSkip = iota // session_meta の時刻より前の行（印のない古い版）
	codexSkipOrdinal                  // ordinal が subagent_history_start_ordinal より前の行
	codexSkipMarker                   // 自分のスレッド ID の thread_settings_applied より前の行（見つからなければ時刻で分ける）
)

// codexOwnSettings は、ファイル自身のスレッドの thread_settings_applied か（フォークやサブエージェントの写しの終わりの印）。
func codexOwnSettings(e core.Obj, id string) bool {
	p := core.Map(e["payload"])
	return core.Str(e["type"]) == "event_msg" && core.Str(p["type"]) == "thread_settings_applied" &&
		id != "" && core.Str(p["thread_id"]) == id
}

// codexCopyWindow は、写しと印を 1 回でまとめて書いたとみなす幅（秒）。
const codexCopyWindow = 5 * 60

// codexSameWrite は、印より前の行（copied）が印と一緒に書かれた写しか。Codex は写しと印を 1 回でまとめて書き、
// 写した行の時刻も書いたときのものなので、先頭の行と印の時刻はほとんど同じになる。
// 先頭の行がずっと前なら、印は古い版で作ったファイルを新しい版で再開したときのもので、その前には自分の行がある。
func codexSameWrite(copied []core.Obj, marker core.Obj) bool {
	t := ts(marker["timestamp"])
	for _, e := range copied {
		if first := ts(e["timestamp"]); first != nil {
			return t != nil && *t-*first <= codexCopyWindow
		}
	}
	return true // 時刻のある行がない（時刻のない行はどのみち数えない）
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

// itemText は item_completed の発言（TurnItem）の文。UserMessage の content は {"type":"text","text":…} のほか画像なども並ぶので、
// 文のあるものだけをつなぐ。AgentMessage の content は {"type":"Text","text":…}。
func itemText(item core.Obj) string {
	var parts []string
	for _, b := range core.List(item["content"]) {
		if text := core.Str(core.Map(b)["text"]); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

// codexTokens は Codex の使用量 → Tokens。cached_input_tokens と cache_write_input_tokens は
// どちらも input_tokens の内訳（Responses API の input_tokens_details.cached_tokens / cache_write_tokens）なので、
// 入力からは両方を引く。
func codexTokens(u any) core.Tokens {
	m := core.Map(u)
	in, cached, written := core.NumOr0(m["input_tokens"]), core.NumOr0(m["cached_input_tokens"]), core.NumOr0(m["cache_write_input_tokens"])
	return core.Tokens{In: max(0, in-cached-written), Out: core.NumOr0(m["output_tokens"]), CW: written, CR: cached}
}

// codexEvent は 1 つの使用量の記録。perRequest は u が応答 1 回分（last_token_usage・token_usage_record）のとき true で、
// そのときだけ入力の量で長いコンテキストの料金を選ぶ（古い形の差分は何回分かの合計なので、短いコンテキストの料金にする）。
func codexEvent(t *float64, model string, u core.Tokens, perRequest bool) core.Event {
	e := core.Event{T: t, U: u, Model: model, Req: perRequest}
	if c, ok := core.EventCost(e); ok {
		e.Cost = &c
	}
	return e
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
		var lastInfo, lastLimits string
		var prevTotal core.Tokens
		records := map[string]bool{}
		var fromCounts, fromRecords []core.Event
		var countMeas, recordMeas []core.Measure
		var copied []core.Obj // codexSkipMarker で、印を待っているあいだの行（印があれば捨てる）
		handle := func(e core.Obj) {
			t := ts(e["timestamp"])
			p := core.Map(e["payload"])
			typ := core.Str(e["type"])
			if t == nil {
				return
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
					if rl := core.Map(p["rate_limits"]); rl != nil {
						// rate_limit_reached_type は使用量の上限の 429（usage_limit_reached）のときだけ入る。
						// Codex は最後のスナップショットを次の token_count にもそのまま書くので、同じものの書き直しは数えない
						raw, _ := json.Marshal(rl)
						if core.Str(rl["rate_limit_reached_type"]) != "" && string(raw) != lastLimits {
							s.Limit(t, "")
						}
						lastLimits = string(raw)
						codexRateLimits(s, t, rl)
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
					if codexFull(info) {
						// コンテキストがあふれた（ContextWindowExceeded）ときに Codex が書く「満杯」の印。応答ではない
						lastInfo, lastTotal, prevTotal = string(raw), total, codexTokens(info["total_token_usage"])
						countMeas = append(countMeas, core.Measure{Key: "context_used", T: t, V: 1})
						return
					}
					cur := codexTokens(info["total_token_usage"])
					u := cur // 1 回目は合計がそのまま 1 回分
					perRequest := false
					if last := core.Map(info["last_token_usage"]); last != nil {
						u, perRequest = codexTokens(last), true
					} else if lastTotal >= 0 { // 古い形: 前回の合計との差
						u = core.Tokens{In: max(0, cur.In-prevTotal.In), Out: max(0, cur.Out-prevTotal.Out),
							CW: max(0, cur.CW-prevTotal.CW), CR: max(0, cur.CR-prevTotal.CR)}
					}
					lastInfo, lastTotal, prevTotal = string(raw), total, cur
					fromCounts = append(fromCounts, codexEvent(t, cf.model, u, perRequest))
					countMeas = append(countMeas, codexMeasures(t, core.Map(info["last_token_usage"]), core.NumOr0(info["model_context_window"]))...)
				case "task_complete", "turn_complete": // 名前は task_complete（turn_complete も同じものとして読む）
					s.Agent(t)
					if e := core.Map(p["error"]); e != nil { // うまく終わらなかったターン（時間は数えない）
						if codexLimitError(e["codex_error_info"]) {
							s.Limit(t, "")
						}
						return
					}
					if v, ok := core.Num(p["time_to_first_token_ms"]); ok && v >= 0 {
						s.Measure("ttft", t, v/1000)
					}
					if v, ok := core.Num(p["duration_ms"]); ok && v >= 0 {
						s.Measure("turn_duration", t, v/1000)
					}
				case "turn_aborted":
					s.Agent(t)
					if core.Str(p["reason"]) == "interrupted" { // 人が止めた（replaced・review_ended・budget_limited は数えない）
						s.Interrupt(t)
					}
				case "item_completed":
					// Paginated rollouts persist user turns as ItemCompleted(UserMessage)
					// instead of the legacy UserMessage event.
					item := core.Map(p["item"])
					// TurnItem は #[serde(tag = "type")] だけで名前を変えていないので、型の名前がそのまま入る
					switch core.Str(item["type"]) {
					case "UserMessage":
						s.Tick(t)
						s.Turn() // 依頼はあとでまとめて足すので、ここで応答の区切りを入れる
						if text := itemText(item); text != "" {
							userMsgs = append(userMsgs, struct {
								t    *float64
								text string
							}{t, text})
						}
					case "AgentMessage": // 人に返した文（依頼の流れに出す）
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
				fromRecords = append(fromRecords, codexEvent(t, cf.model, codexTokens(p["usage"]), true))
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
					switch core.Str(p["role"]) {
					case "user":
						s.Tick(t)
						s.Turn() // 依頼はあとでまとめて足すので、ここで応答の区切りを入れる
						fallback = append(fallback, struct {
							t    *float64
							text string
						}{t, strings.Join(parts, "\n")})
					case "assistant":
						s.Agent(t)
						s.Reply(t, "", strings.Join(parts, "\n")) // 人に返した文
					default: // developer（Codex が足す指示）などは応答ではない
						s.Agent(t)
					}
				default:
					s.Agent(t)
				}
			default:
				s.Agent(t)
			}
		}
		// before は、session_meta の時刻より前の行か（印のない古い版で、親から写した行）。
		before := func(e core.Obj) bool {
			t := ts(e["timestamp"])
			return t != nil && cf.start != nil && *t < *cf.start
		}
		errs.file(path, core.ReadJSONL(path, func(e core.Obj) {
			if core.Str(e["type"]) == "session_meta" {
				if cf.id == "" { // 2 つめからは写した親のもの
					p := core.Map(e["payload"])
					cf.id = core.Str(p["id"])
					cf.cwd = core.Str(p["cwd"])
					cf.branch = core.Str(core.Get(p, "git", "branch"))
					cf.start = ts(p["timestamp"])
					if cf.start == nil {
						cf.start = ts(e["timestamp"])
					}
					spawn := core.Map(core.Get(p, "source", "subagent", "thread_spawn"))
					cf.parent = codexParent(p)
					cf.role = firstNonEmpty(core.Str(spawn["agent_role"]), core.Str(p["agent_role"]), core.Str(spawn["agent_nickname"]), core.Str(p["agent_nickname"]))
					if n, ok := core.Num(p["subagent_history_start_ordinal"]); ok {
						cf.skip, cf.startOrdinal = codexSkipOrdinal, n
					} else if cf.parent != "" || core.Str(p["forked_from_id"]) != "" {
						cf.skip = codexSkipMarker
					}
					cf.b = core.NewBuilder("Codex", cf.id)
					cf.b.File = cf.path
				}
				return
			}
			if cf.b == nil {
				return
			}
			// 親から写した履歴は数えない
			switch cf.skip {
			case codexSkipOrdinal:
				if n, ok := core.Num(e["ordinal"]); ok {
					if n < cf.startOrdinal {
						return
					}
				} else if before(e) {
					return
				}
			case codexSkipMarker:
				if !codexOwnSettings(e, cf.id) {
					copied = append(copied, e)
					return
				}
				if codexSameWrite(copied, e) {
					// 印より前はすべて写しで、印から後は自分の行（時刻では分けない）
					cf.skip, cf.start, copied = codexSkipTime, nil, nil
					break
				}
				// 印のない古い版で作り、新しい版で再開したファイル（再開でも書く）。印より前には自分の行もあるので時刻で分ける
				cf.skip = codexSkipTime
				for _, c := range copied {
					if !before(c) {
						handle(c)
					}
				}
				copied = nil
			default:
				if before(e) {
					return
				}
			}
			handle(e)
		}))
		if cf.b == nil {
			continue
		}
		for _, e := range copied { // 印のない古い版のフォークやサブエージェントは、時刻で分ける
			if !before(e) {
				handle(e)
			}
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
		parent.Limits = append(parent.Limits, cf.b.Limits...)       // サブエージェントが上限に当たっても、止まったのは親のセッションの作業
		parent.LimitResets = append(parent.LimitResets, cf.b.LimitResets...)
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
		// サブエージェントの分を足したので、時刻の順に並べ直して 1 分以内のものをまとめ直す
		hits := append([]float64(nil), s.Limits...)
		sort.Float64s(hits)
		s.Limits, s.LimitResets = nil, nil // Codex の上限には解除の時刻の文がない
		for i := range hits {
			s.Limit(&hits[i], "")
		}
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

// codexBaseline は、Codex がコンテキストの使用率から除くトークン（システムプロンプトやツールの説明など、いつも入っている分）。
// protocol の BASELINE_TOKENS と同じ値。
const codexBaseline = 12000

// codexMeasures は 1 回の応答の使用量から参考指標を作る。
// コンテキストの使用率は Codex の表示（percent_of_context_window_remaining）と同じく、
// last_token_usage.total_tokens と上限の両方から codexBaseline を引いて割る（0〜100% に収める）。
func codexMeasures(t *float64, u core.Obj, window float64) []core.Measure {
	if u == nil {
		return nil
	}
	ms := []core.Measure{
		{Key: "responses", T: t, V: 1},
		{Key: "reasoning", T: t, V: core.NumOr0(u["reasoning_output_tokens"])},
		{Key: "output", T: t, V: core.NumOr0(u["output_tokens"])},
	}
	if window > codexBaseline {
		used, ok := core.Num(u["total_tokens"])
		if !ok {
			used = core.NumOr0(u["input_tokens"]) + core.NumOr0(u["output_tokens"])
		}
		v := min(1, max(0, (used-codexBaseline)/(window-codexBaseline)))
		ms = append(ms, core.Measure{Key: "context_used", T: t, V: v})
	}
	return ms
}

// codexFull は、コンテキストがあふれたあとに Codex が書く token_count か（set_total_tokens_full → fill_to_context_window）。
// total_token_usage は total_tokens だけが上限と同じ値で、ほかは 0。last_token_usage も total_tokens（上限までの残り）のほかは 0。
func codexFull(info core.Obj) bool {
	window := core.NumOr0(info["model_context_window"])
	total, last := core.Map(info["total_token_usage"]), core.Map(info["last_token_usage"])
	if window <= 0 || total == nil || core.NumOr0(total["total_tokens"]) != window {
		return false
	}
	for _, u := range []core.Obj{total, last} {
		for _, k := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens"} {
			if core.NumOr0(u[k]) != 0 {
				return false
			}
		}
	}
	return true
}

// codexLimitError は、ターンを止めたエラー（codex_error_info）が利用上限か。
// usage_limit_exceeded は使用量の上限（ChatGPT のプランの 429 usage_limit_reached、クォータ切れなど）、
// rate_limit_exceeded は API のレート制限。context_window_exceeded（会話が長すぎる）や
// session_budget_exceeded（Codex 自身の予算）は利用上限ではない。
// CodexErrorInfo は snake_case の文字列（フィールドのある種類は {"名前": {...}} なので、ここでは当たらない）。
func codexLimitError(v any) bool {
	switch core.Str(v) {
	case "usage_limit_exceeded", "rate_limit_exceeded":
		return true
	}
	return false
}

// codexRateLimits は token_count.rate_limits の primary と secondary の使用率（%）を、枠の長さごとの指標にする。
// limit_id が codex（古い版は無し）のものだけを使う。ほかの id はモデルごとの別の枠。
func codexRateLimits(s *core.Builder, t *float64, rl core.Obj) {
	if id := core.Str(rl["limit_id"]); id != "" && !strings.EqualFold(id, "codex") {
		return
	}
	for _, slot := range []string{"primary", "secondary"} {
		w := core.Map(rl[slot])
		if v, ok := core.Num(w["used_percent"]); ok {
			s.Measure(codexWindowKey(w["window_minutes"], slot == "secondary"), t, v)
		}
	}
}

// codexWindows は枠の長さ（分）と指標の名前。Codex の画面（get_limits_duration）と同じく、前後 5% までを同じ長さとみなす。
var codexWindows = []struct {
	minutes float64
	key     string
}{
	{5 * 60, "rate_limit_5h"},
	{24 * 60, "rate_limit_daily"},
	{7 * 24 * 60, "rate_limit_weekly"},
	{30 * 24 * 60, "rate_limit_monthly"},
	{365 * 24 * 60, "rate_limit_annual"},
}

// codexWindowKey は枠の指標の名前。window_minutes がないか、どの長さにも近くなければ primary / secondary の名前にする。
func codexWindowKey(minutes any, secondary bool) string {
	if m, ok := core.Num(minutes); ok {
		for _, w := range codexWindows {
			if m >= w.minutes*0.95 && m <= w.minutes*1.05 {
				return w.key
			}
		}
	}
	if secondary {
		return "rate_limit_secondary"
	}
	return "rate_limit"
}

func firstPrompt(b *core.Builder) string {
	if len(b.Prompts) == 0 {
		return ""
	}
	return core.Runes(strings.SplitN(b.Prompts[0].Text, "\n", 2)[0], 120)
}
