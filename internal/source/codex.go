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
//	response_item … function_call / custom_tool_call / local_shell_call / web_search_call / tool_search_call /
//	                image_generation_call（ツール）、message（role が assistant なら応答。user に heartbeat の印）
//	token_usage_record … 新しい版の使用量（あればこちらを使い、token_count は使わない）
//	compacted     … コンパクション（会話を要約して文脈を空けた）1 回につき 1 行
//
// サブエージェントやフォークのファイルは、親の履歴を自分の session_meta のあとにそのまま写している。
// 写した行の時刻は写したときのものなので、時刻では分けられない。写しの終わりは次の印で見分ける（codexFile.skip）。
//
//   - session_meta.subagent_history_start_ordinal があれば（paginated のサブエージェント）、行の ordinal がそれより前の行
//   - フォークかサブエージェントなら、自分のスレッド ID の thread_settings_applied より前の行
//     （写しと一緒に書く。写した親の thread_settings_applied は親のスレッド ID のまま。
//     印のない古い版で作ったファイルも再開すると書くので、写しと一緒に書いたものだけを印とみなす: codexSameWrite）
//   - どちらの印もない古い版は、これまでどおり session_meta の時刻より前の行
//
// thread/revert（巻き戻し）は同じスレッド ID の新しいファイルを作り、残す前半は session_meta.history_base で古いファイルを指す
// （写さない）。そういうファイルは codexChains で 1 つのスレッドにつなぐ。
type Codex struct {
	Home string

	mu    sync.Mutex
	heads map[string]codexHead // ファイルごとの session_meta（Units で親子をまとめるため。印が同じなら読み直さない）
}

// codexHead は、ファイルの先頭の session_meta から読んだスレッド ID と親のスレッド ID など。
type codexHead struct {
	stamp      string
	ok         bool // session_meta がある
	id, parent string
	start      *float64 // session_meta.timestamp（巻き戻しで作ったファイルは、作ったときの時刻）
	base       string   // history_base.thread_id（名前は thread_id だが、前半を持つファイルの rollout ID）
	baseEnd    float64  // history_base.end_ordinal_exclusive（前半として読む ordinal の上限。含まない）
	err        error    // 先頭を読めなかった（壊れた .zst など）
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
	windows               map[string]float64 // モデルごとの model_context_window（いちばん大きい値）
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
			h = readCodexHead(p)
			h.stamp = stamp
		}
		next[p], out[i] = h, h
	}
	c.heads = next
	return out
}

// head は、1 つのファイルの session_meta（印が同じなら前に読んだものを使う）。
func (c *Codex) head(p string) codexHead {
	c.mu.Lock()
	defer c.mu.Unlock()
	stamp := Stamp([]string{p})
	if h, ok := c.heads[p]; ok && h.stamp == stamp {
		return h
	}
	h := readCodexHead(p)
	h.stamp = stamp
	if c.heads == nil {
		c.heads = map[string]codexHead{}
	}
	c.heads[p] = h
	return h
}

// readCodexHead は、最初の session_meta までだけ読む（ふつうは 1 行目）。
// err は、session_meta にたどり着く前に読めなくなったとき（消えていたときは nil）。
func readCodexHead(path string) codexHead {
	f, err := os.Open(path)
	if err != nil {
		return codexHead{err: fileErr(path, err)}
	}
	defer f.Close()
	var src io.Reader = f
	if strings.HasSuffix(path, ".zst") {
		d, err := core.NewZstdReader(f)
		if err != nil {
			return codexHead{err: fileErr(path, err)}
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
			h := codexHead{ok: true, id: core.Str(p["id"]), parent: codexParent(p), start: ts(p["timestamp"])}
			if h.start == nil {
				h.start = ts(e["timestamp"])
			}
			base := core.Map(p["history_base"])
			if end, ok := core.Num(base["end_ordinal_exclusive"]); ok {
				h.base, h.baseEnd = core.Str(base["thread_id"]), end
			}
			return h
		}
		if err == io.EOF {
			return codexHead{}
		}
		if err != nil {
			return codexHead{err: fileErr(path, err)}
		}
	}
}

// codexRolloutID は、ファイル名の rollout ID。ふつうは rollout-<時刻>-<スレッド ID>.jsonl でスレッド ID と同じ。
// thread/revert（巻き戻し）で作ったファイルは rollout-<時刻>-<スレッド ID>_<rollout ID>.jsonl（rollout_file_name.rs）。
func codexRolloutID(path string) string {
	base := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), ".zst"), ".jsonl")
	rest, ok := strings.CutPrefix(base, "rollout-")
	if !ok || len(rest) <= 20 || rest[19] != '-' {
		return ""
	}
	ids := rest[20:]
	if _, r, ok := strings.Cut(ids, "_"); ok {
		return r
	}
	return ids
}

// codexSegment は、1 つのスレッドとして続けて読むファイルの 1 つ。limit が 0 以上なら、ordinal がそれより前の行だけを読む。
type codexSegment struct {
	path  string
	limit float64
}

// codexChains は、まとまりのファイルを、1 つのスレッドとして読むファイルの並び（古いほうから）に分ける。
// thread/revert（巻き戻し、paginated だけ）は、残す前半を写さずに新しいファイルを作り、session_meta.history_base で
// 古いファイルの前半（ordinal が end_ordinal_exclusive より前の行）を指す。古いファイルはそのまま残り、どちらも同じスレッド ID。
// 同じスレッド ID のファイルが history_base でつながっていれば、いまのファイル（ほかのどれからも指されていない、
// いちばん新しいもの）だけを、指している前半とつないで 1 つにする。巻き戻して消したターン（前半より後の行）と、
// いまのファイルからたどれない古いファイルは読まない。ほかのスレッドを指す history_base（paginated のフォーク）はたどらない。
func (c *Codex) codexChains(files []string) [][]codexSegment {
	if len(files) < 2 {
		var out [][]codexSegment
		for _, p := range files {
			out = append(out, []codexSegment{{path: p, limit: -1}})
		}
		return out
	}
	heads := make([]codexHead, len(files))
	at := map[[2]string]int{} // スレッド ID と rollout ID → files の位置
	for i, p := range files {
		heads[i] = c.head(p)
		if h := heads[i]; h.ok && h.id != "" {
			at[[2]string{h.id, codexRolloutID(p)}] = i
		}
	}
	referenced := map[int]bool{}
	reverted := map[string]bool{} // 巻き戻したことのあるスレッド ID
	for i, h := range heads {
		if h.base == "" {
			continue
		}
		if j, ok := at[[2]string{h.id, h.base}]; ok && j != i {
			referenced[j], reverted[h.id] = true, true
		}
	}
	// 巻き戻したスレッドごとに、いまのファイル（指されていないもののうち、いちばん新しいもの）
	current := map[string]int{}
	for i, h := range heads {
		if !h.ok || !reverted[h.id] || referenced[i] {
			continue
		}
		j, ok := current[h.id]
		if !ok || codexNewer(heads[i], files[i], heads[j], files[j]) {
			current[h.id] = i
		}
	}
	var out [][]codexSegment
	for i, p := range files {
		h := heads[i]
		if !h.ok || !reverted[h.id] {
			out = append(out, []codexSegment{{path: p, limit: -1}})
			continue
		}
		if current[h.id] != i {
			continue // 前半として読むか、巻き戻しで使われなくなったファイル
		}
		chain := []codexSegment{{path: p, limit: -1}}
		seen := map[int]bool{i: true}
		limit := -1.0
		for h.base != "" {
			j, ok := at[[2]string{h.id, h.base}]
			if !ok || seen[j] {
				break
			}
			seen[j] = true
			if limit < 0 || h.baseEnd < limit {
				limit = h.baseEnd
			}
			chain = append([]codexSegment{{path: files[j], limit: limit}}, chain...)
			h = heads[j]
		}
		out = append(out, chain)
	}
	return out
}

// codexNewer は、a のファイルが b より新しいか（session_meta の時刻。同じならファイルの名前の後ろのほう）。
func codexNewer(a codexHead, pa string, b codexHead, pb string) bool {
	if a.start != nil && b.start != nil && *a.start != *b.start {
		return *a.start > *b.start
	}
	return pa > pb
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
	for _, chain := range c.codexChains(u.Files) {
		cf := &codexFile{path: chain[len(chain)-1].path, windows: map[string]float64{}}
		var userMsgs, fallback []codexMsg
		sawEvents := false // user_message か item_completed の UserMessage がある（なければ response_item の user を使う）
		// heartbeats は、response_item に印（content_item_kinds が user.heartbeat だけ）のあった、予定（heartbeat）が入れた文。
		// Codex はその response_item を書いてから、同じ文の user_message（paginated は item_completed）を書く。
		// beat は、いまのターンが heartbeat のものか（そのあいだの応答は、人の依頼への応答として割り当てない）
		var heartbeats []string
		beat := false
		// input は、user_message・UserMessage の文。heartbeat の印のあった文なら人の依頼にしない
		input := func(t *float64, text string) {
			sawEvents = true
			m := codexMsg{t: t, text: text}
			for i, h := range heartbeats {
				if h == strings.TrimSpace(text) {
					m.beat = true
					heartbeats = append(heartbeats[:i:i], heartbeats[i+1:]...)
					break
				}
			}
			beat = m.beat
			if m.beat {
				cf.b.Agent(t) // 人が打ったものではない
			} else {
				cf.b.Tick(t)
			}
			cf.b.Turn() // 依頼はあとでまとめて足すので、ここで応答の区切りを入れる
			if text != "" {
				userMsgs = append(userMsgs, m)
			}
		}
		reply := func(t *float64, text string) {
			cf.b.Agent(t)
			if !beat { // heartbeat のターンの応答は、前の人の依頼への応答ではない
				cf.b.Reply(t, "", text)
			}
		}
		var lastTotal float64 = -1
		var lastInfo, lastLimits string
		var prevTotal core.Tokens
		records := map[string]bool{}
		var fromCounts, fromRecords []core.Event
		var countMeas, recordMeas []core.Measure
		// cutEvents・cutMeas は、最初の token_usage_record を読んだときの fromCounts・countMeas の長さ（-1 はまだ無い）。
		// 古い版で始めて新しい版で再開したファイルは、そこまでを token_count から、そこからを token_usage_record から数える
		cutEvents, cutMeas := -1, -1
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
					input(t, core.Str(p["message"]))
				case "agent_message": // 人に返した文（依頼の流れに出す）
					reply(t, core.Str(p["message"]))
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
					if w := core.NumOr0(info["model_context_window"]); w > 0 {
						cf.windows[cf.model] = max(cf.windows[cf.model], w)
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
					if !perRequest && total > lastTotal {
						// last_token_usage のないとても古い版: 合計が増えたら応答 1 回（中身の内訳はない）
						countMeas = append(countMeas, core.Measure{Key: "responses", T: t, V: 1})
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
						input(t, itemText(item))
					case "AgentMessage": // 人に返した文（依頼の流れに出す）
						reply(t, itemText(item))
					default:
						s.Agent(t)
					}
				default:
					s.Agent(t)
				}
			case "compacted":
				// コンパクション（会話を要約して文脈を空けた）。ローカル・リモート（v2 も）・トークン予算のどれも
				// replace_compacted_history で 1 回につき 1 行書き、履歴の形（legacy・paginated）によらず残る
				// （rollout の policy.rs）。同じ 1 回の context_compacted（legacy だけ）や item_completed の
				// ContextCompaction（paginated）は数えない。フォークやサブエージェントが写した親の行は、上で飛ばしている
				s.Agent(t)
				s.Compact(t, "")
			case "token_usage_record":
				s.Agent(t)
				id := core.Str(p["response_id"])
				if id != "" && records[id] {
					return
				}
				records[id] = true
				if cutEvents < 0 {
					cutEvents, cutMeas = len(fromCounts), len(countMeas)
				}
				fromRecords = append(fromRecords, codexEvent(t, cf.model, codexTokens(p["usage"]), true))
				recordMeas = append(recordMeas, codexMeasures(t, core.Map(p["usage"]), 0)...)
			case "response_item":
				switch core.Str(p["type"]) {
				case "function_call":
					var args any
					json.Unmarshal([]byte(core.Str(p["arguments"])), &args)
					s.Tool(core.Str(p["name"]), args)
					codexEdited(s, core.Str(p["name"]), args)
					s.Measure("tool_calls", t, 1)
					s.Agent(t)
				case "custom_tool_call":
					s.Tool(core.Str(p["name"]), nil)
					codexEdited(s, core.Str(p["name"]), core.Str(p["input"]))
					s.Measure("tool_calls", t, 1)
					s.Agent(t)
				case "local_shell_call":
					// モデルが直接呼ぶシェル（Responses API の local_shell）。action は {"type":"exec","command":[…]}
					action := core.Map(p["action"])
					s.Tool("local_shell", nil)
					codexEdited(s, "local_shell", action)
					s.Measure("tool_calls", t, 1)
					s.Agent(t)
				case "web_search_call", "tool_search_call", "image_generation_call":
					// サーバーやクライアントで動くツール（ウェブ検索・ツールの検索・画像の生成）。function_call とは別の項目で残る
					s.Tool(strings.TrimSuffix(core.Str(p["type"]), "_call"), nil)
					s.Measure("tool_calls", t, 1)
					s.Agent(t)
				case "message":
					var parts []string
					for _, b := range core.List(p["content"]) {
						parts = append(parts, core.Str(core.Map(b)["text"]))
					}
					switch core.Str(p["role"]) {
					case "user":
						text := strings.Join(parts, "\n")
						m := codexMsg{t: t, text: text, beat: codexHeartbeat(p)}
						if m.beat {
							heartbeats = append(heartbeats, strings.TrimSpace(text))
							beat = true
							s.Agent(t)
						} else {
							if codexHuman(text) {
								beat = false
							}
							s.Tick(t)
						}
						s.Turn() // 依頼はあとでまとめて足すので、ここで応答の区切りを入れる
						fallback = append(fallback, m)
					case "assistant":
						reply(t, strings.Join(parts, "\n")) // 人に返した文
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
		line := func(e core.Obj) {
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
		}
		for _, seg := range chain {
			errs.file(seg.path, core.ReadJSONL(seg.path, func(e core.Obj) {
				if seg.limit >= 0 && core.Str(e["type"]) != "session_meta" {
					// 巻き戻したスレッドの前半: ordinal が history_base の上限より前の行だけ（後ろは巻き戻して消したターン）
					if n, ok := core.Num(e["ordinal"]); !ok || n >= seg.limit {
						return
					}
				}
				line(e)
			}))
		}
		if cf.b == nil {
			continue
		}
		cf.b.File = cf.path
		for _, e := range copied { // 印のない古い版のフォークやサブエージェントは、時刻で分ける
			if !before(e) {
				handle(e)
			}
		}
		pending, meas := fromCounts, countMeas
		if cutEvents >= 0 {
			// 新しい版は応答ごとに token_usage_record を書いてから token_count を書く（同じ応答を重ねて書いている）。
			// 最初の token_usage_record より前の token_count は、古い版で書いた分（再開すると同じファイルに足していく）
			pending = append(append([]core.Event(nil), fromCounts[:cutEvents]...), fromRecords...)
			meas = append(append([]core.Measure(nil), countMeas[:cutMeas]...), recordMeas...)
			for _, m := range countMeas[cutMeas:] { // コンテキストの使用率は token_count にしかない
				if m.Key == "context_used" {
					meas = append(meas, m)
				}
			}
		}
		cf.b.Measures = append(cf.b.Measures, meas...)
		// 依頼の文は event_msg.user_message（paginated は item_completed の UserMessage）を使う。ない古い版だけ response_item の user を使う
		msgs := userMsgs
		if !sawEvents {
			msgs = fallback
		}
		for _, m := range msgs {
			if m.beat { // 予定（heartbeat）が入れた文。人の依頼ではない
				cf.b.Inject(m.t, "agent", m.text)
				continue
			}
			if !sawEvents && codexInjected(m.text) { // Codex が足した文（<environment_context> などのタグは Prompt が分ける）
				cf.b.Inject(m.t, "other", m.text)
				continue
			}
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
		s.CtxWindow = core.PeakContextWindowFrom(cf.events, cf.windows) // 「長い会話」の最大の目安（ウィンドウの半分）
		if s.Project != "" {
			s.Resume = core.ResumeCmd(s.Project, "codex resume", s.ID)
		}
		emit(s)
	}
	return errs.err()
}

// codexMsg は人の発言の候補（user_message・UserMessage、古い版は response_item の user）。
// beat は、予定（heartbeat）が入れた文か。
type codexMsg struct {
	t    *float64
	text string
	beat bool
}

// codexHeartbeatKind は、予定（heartbeat）が入れた文の印（history の heartbeat.rs の HEARTBEAT_CONTENT_KIND）。
const codexHeartbeatKind = "user.heartbeat"

// codexHeartbeat は、response_item の user の message が、予定（heartbeat）が入れた文か。
// Codex（UserInputOrigin::from_message）と同じく、internal_chat_message_metadata_passthrough.content_item_kinds が
// user.heartbeat 1 つだけのものに限る。ターンの turn_trigger では決めない（heartbeat のターンの途中で人が足した文は人のもの）。
func codexHeartbeat(p core.Obj) bool {
	kinds := core.List(core.Get(p, "internal_chat_message_metadata_passthrough", "content_item_kinds"))
	return len(kinds) == 1 && core.Str(kinds[0]) == codexHeartbeatKind
}

// codexInjectedPrefixes は、タグで始まらないのに Codex が user の発言として会話に入れる文の始まり
// （core の contextual_user_message.rs の CONTEXTUAL_USER_FRAGMENT_MATCHERS のうちタグのないもの）。
var codexInjectedPrefixes = []string{
	"Here is a list of plugins that are available but not installed.",            // RecommendedPluginsInstructions（古い rollout の user の発言）
	"Warning: The maximum number of unified exec processes you can keep open is", // LegacyUnifiedExecProcessLimitWarning
	"Warning: Your account was flagged for potentially high-risk cyber activity", // LegacyModelMismatchWarning
}

// codexInjected は、response_item の user の文が、Codex が会話に入れたものか（user_message のない古い版で使う）。
// core の UserInstructions は「# AGENTS.md instructions（ for ディレクトリ）」で始まり </INSTRUCTIONS> で終わる。
// Codex と同じく、その始まりは大文字と小文字を区別しない。<environment_context> などのタグで始まるものは Prompt が分ける。
func codexInjected(text string) bool {
	const agentsMD = "# AGENTS.md instructions"
	t := strings.TrimSpace(text)
	if len(t) >= len(agentsMD) && strings.EqualFold(t[:len(agentsMD)], agentsMD) {
		return true
	}
	for _, p := range codexInjectedPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	// LegacyApplyPatchExecCommandWarning
	return strings.HasPrefix(t, "Warning: apply_patch was requested via ") &&
		strings.HasSuffix(t, "Use the apply_patch tool instead of exec_command.")
}

// codexHuman は、response_item の user の文が人の打ったものに見えるか（タグで始まるものや Codex が足した文ではない）。
func codexHuman(text string) bool {
	t := strings.TrimSpace(text)
	return t != "" && !strings.HasPrefix(t, "<") && !codexInjected(t)
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

// codexPatchMarkers は apply_patch のパッチで、変えるファイルの名前が続く行の頭（apply-patch の parser.rs）。
var codexPatchMarkers = []string{"*** Add File: ", "*** Delete File: ", "*** Update File: ", "*** Move to: "}

// codexEdited は、apply_patch で変えたファイルを編集したファイルに足す。Codex はファイルを apply_patch で変え、
// その引数はパッチの文なので（file_path の形ではない）、Builder.Tool では数えられない。
// apply_patch は自由形式のツール（custom_tool_call の input）、関数（arguments の input）、
// シェルから（["apply_patch", パッチ] や bash -lc "apply_patch <<'EOF' …"）の形で呼ばれる。
func codexEdited(s *core.Builder, name string, args any) {
	for _, f := range codexPatchFiles(codexPatchText(name, args)) {
		s.Edited(f)
	}
}

// codexPatchText は、ツール呼び出しの引数から apply_patch のパッチの文を取り出す（なければ空）。
func codexPatchText(name string, args any) string {
	if text, ok := args.(string); ok { // custom_tool_call
		if name == "apply_patch" {
			return text
		}
		return ""
	}
	m := core.Map(args)
	if name == "apply_patch" {
		return core.Str(m["input"])
	}
	var argv []string // シェルのコマンド（shell の command は配列、exec_command の cmd は文）
	for _, a := range core.List(m["command"]) {
		argv = append(argv, core.Str(a))
	}
	if cmd := core.Str(m["cmd"]); cmd != "" {
		argv = []string{cmd}
	}
	if len(argv) == 2 && (argv[0] == "apply_patch" || argv[0] == "applypatch") {
		return argv[1]
	}
	if len(argv) > 0 {
		script := argv[len(argv)-1]
		if strings.Contains(script, "apply_patch") || strings.Contains(script, "applypatch") {
			return script
		}
	}
	return ""
}

// codexPatchFiles は、パッチの文から変えるファイルの名前を取り出す（Begin Patch のないものは空）。
func codexPatchFiles(patch string) []string {
	if !strings.Contains(patch, "*** Begin Patch") {
		return nil
	}
	var files []string
	for _, line := range strings.Split(patch, "\n") {
		line = strings.TrimSpace(line)
		for _, mk := range codexPatchMarkers {
			if f, ok := strings.CutPrefix(line, mk); ok {
				if f = strings.TrimSpace(f); f != "" {
					files = append(files, f)
				}
			}
		}
	}
	return files
}
