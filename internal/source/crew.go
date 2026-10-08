package source

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Kiro Crew（github.com/kirodotdev/KiroCrew）は kiro-cli を ACP で動かすので、会話そのものは
// ~/.kiro/sessions/cli に kiro-cli の履歴として残る。Crew 側のデータは「Crew から動かした会話」という
// 目印とタイトルに使う。
//
// クレジットは、Crew のダッシュボードから動かした会話だと kiro-cli の履歴に残らないことがある。
// Crew は 1 ターンごとのクレジットを usage/tokens/<日付>.jsonl に書くので、そちらも読む。
// 同じ会話は二重に数えないよう、会話ごとに kiro-cli の記録と Crew の記録の多いほうを使う。Crew の記録は会話キーごとなので、
// キーの会話を差しかえたときは、時刻でそのころの kiro-cli の会話に分ける（crewOwner）。
// kiro-cli の会話に結びつかない記録（Crew の裏方の処理 _bg など）は、Crew のセッションとして数える。
//
//	<KIROCREW_HOME か ~/.kiro/crew>/session_map.json        {会話キー: {"sid": kiro-cli の会話 ID, "cwd": …, "provider"?, "discarded_sid"?} か "sid"}
//	<…>/sessions/<キー>.jsonl                               1 行目 {"_type": "metadata", "title", "agent", "model", …}
//	                                                         2 行目から {"role": "user"|"assistant"|…, "content", "ts", "tools"?: [ツール名]}
//	<…>/subagents/<id>/state.json                          {"task", "agent", "parent_session", "session_id"?, "provider"?, …}
//	<…>/usage/tokens/<YYYY-MM-DD>.jsonl                     {"_type": "tokens", "ts", "slot": 会話キー, "provider", "model", "input", "output",
//	                                                         "cache_create", "cache_read", "cost", "credits", "duration_ms", "context_used",
//	                                                         "context_window", "stop_reason", …}
//
// usage/tokens の行は Crew の dashboard/handlers/usage.py の _build_token_record が書く。backend ごとに埋まる項目が違う
// （acp/types.py の TurnUsage）: kiro-cli はクレジットだけ、Claude Code（claude-agent-acp）などはトークンと USD の cost。
// トークンの 4 つは重ならない（Crew 自身も合計を input + output + cache_create + cache_read で出す。usage.py の context_usage の total）。
// cost は backend が会話の累計で返す額の差分で、届いたターンにまとめて載る（acp/types.py の apply_cost_cumulative）。

// CrewInfo は 1 つの kiro-cli の会話についての Crew の情報。
type CrewInfo struct {
	Subagent bool
	Key      string // Crew の会話キー（サブエージェントなら親の会話キー）
	Title    string
	Agent    string
	Task     string
	Cwd      string
	// Provider は Crew の backend（session_map の "provider"、サブエージェントなら state.json の "provider"。
	// 空か "acp" は kiro-cli、"claude_code"・"codex"・"kas" など）
	Provider string
	// UsageSlot はサブエージェントの使用量の記録の slot（subagentSlot）。サブエージェントでなければ空（使用量は Key で記録される）
	UsageSlot string
	// Former は、その会話キーの今の会話ではない kiro-cli の会話（session_map の discarded_sid か、時刻で見つけた前の会話。crewOwner）
	Former bool
}

// usageSlot は、その会話の使用量の記録の slot。
func (i CrewInfo) usageSlot() string {
	if i.Subagent {
		return i.UsageSlot
	}
	return i.Key
}

// DefaultCrewHome は KIROCREW_HOME か ~/.kiro/crew。
// Crew は KIRO_HOME を見ない（config/paths.py の _default_home は Path.home()/.kiro/crew。KIRO_HOME が動かすのは kiro-cli の履歴だけ）。
func DefaultCrewHome() string {
	if d := crewHomeOverride(os.Getenv("KIROCREW_HOME")); d != "" {
		return d
	}
	return filepath.Join(home(), ".kiro", "crew")
}

// crewHomeOverride は KIROCREW_HOME の値を Crew と同じく読む（config/paths.py の _valid_override_home）。
// 先頭の ~ と ~<ユーザー名> はホームにし（Python の expanduser）、絶対パスにする。
// ルートや /usr・/System・/etc・/private/etc の下は、Crew が無視して既定の場所を使うので、空を返す。
func crewHomeOverride(v string) string {
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "~") {
		name, rest, _ := strings.Cut(filepath.ToSlash(v[1:]), "/") // Windows では \ も区切り
		dir := ""
		if name == "" {
			dir = home()
		} else if u, err := user.Lookup(name); err == nil {
			dir = u.HomeDir
		}
		if dir != "" {
			v = filepath.Join(dir, filepath.FromSlash(rest))
		}
	}
	if a, err := filepath.Abs(v); err == nil {
		v = a
	}
	if unsafeCrewHome(v) {
		return ""
	}
	return v
}

// unsafeCrewHome は、Crew が KIROCREW_HOME として使わない場所か（config/paths.py の _is_unsafe_home）。
func unsafeCrewHome(p string) bool {
	if filepath.Dir(p) == p {
		return true
	}
	if !strings.HasPrefix(p, "/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	switch parts[0] {
	case "usr", "System", "etc":
		return true
	}
	return len(parts) > 1 && parts[0] == "private" && parts[1] == "etc"
}

// unsafeKey は、会話キーをファイル名にするときに _ にする文字。Crew の history._safe_key は Python の re.sub(r"[^\w\-.]", "_", key) で、
// \w は Unicode の文字・数字と _（str.isalnum() か _。結合文字などの記号は入らない）。Go の \w は ASCII だけなので \p{L}・\p{N} で書く。
// 残るのは文字・数字・_・-・. だけなので、/ や \ もなく sessions/ の外は指せず（.. も後ろに .jsonl がつく）、glob の特別な文字（* ? [ \）も残らない。
var unsafeKey = regexp.MustCompile(`[^\p{L}\p{N}_\-.]`)

// safeKey は会話キーの記録のファイル名（拡張子なし。Crew の history._safe_key と同じ）。
func safeKey(key string) string { return unsafeKey.ReplaceAllString(key, "_") }

// bareChatSlot は使用量の記録に残るダッシュボードの会話キー（chat-<連番>-<UNIX 秒>）。
// 会話そのものは dashboard:chat-… のキーで扱われる（Crew の usage.spend_key_for_slot と同じ規則）。
var bareChatSlot = regexp.MustCompile(`^chat-\d+-\d+$`)

// spendKey は使用量の記録の slot を、会話のキー（session_map や会話の記録と同じ形）にそろえる。
func spendKey(slot string) string {
	if bareChatSlot.MatchString(slot) {
		return "dashboard:" + slot
	}
	return slot
}

// memoryRun は記憶の整理（memory consolidation）の 1 回ごとの slot（memory-consolidation:<記憶の場所>:<uuid4 の 32 桁>。
// Crew の llm_helpers.py が回ごとに作る）。
var memoryRun = regexp.MustCompile(`^(memory-consolidation:.*):[0-9a-f]{32}$`)

// crewSlot は使用量の記録の slot を、kiroku がまとめる単位にする。記憶の整理は回ごとに slot が変わるので、
// 記憶の場所ごとにまとめる（crewOnly で、裏方の処理 _bg と同じく 1 日ごとのセッションにする）。
func crewSlot(slot string) string {
	slot = spendKey(slot)
	if m := memoryRun.FindStringSubmatch(slot); m != nil {
		return m[1]
	}
	return slot
}

// crewBackground は、会話ではなく裏方の処理の slot か（1 日ごとにまとめ、会話の記録は読まない）。
func crewBackground(slot string) bool {
	return slot == "_bg" || strings.HasPrefix(slot, "memory-consolidation:")
}

// loadCrew は kiro-cli の会話 ID → Crew の情報。Crew がなければ空。
// 読めなかったファイルは errs に足す（nil なら黙って飛ばす）。
func loadCrew(home string, errs *fileErrs) map[string]CrewInfo {
	out := map[string]CrewInfo{}
	if home == "" || !isDir(home) {
		return out
	}
	mapPath := filepath.Join(home, "session_map.json")
	sm, err := core.ReadJSONFile(mapPath)
	errs.file(mapPath, err)
	former := map[string]CrewInfo{} // discarded_sid → Crew の情報（今の sid の会話がなければ使う）
	entries := core.Map(sm)
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys) // 同じ sid が 2 つのキーにあっても、毎回同じほうを使う
	for _, key := range keys {
		v := entries[key]
		sid, cwd, provider, discarded := core.Str(v), "", "", "" // 古い形は文字列だけ
		if m := core.Map(v); m != nil {
			sid, cwd, provider = core.Str(m["sid"]), core.Str(m["cwd"]), core.Str(m["provider"])
			// 会話を差しかえたとき（backend の切りかえ・会話の破棄・記録が消えた sid の片付け）に sid を空にして残す、前の sid
			// （session_map.py の _stash_and_clear_sid。SessionMap.set はこれを消さないので、新しい sid と並んで残ることもある）
			discarded = core.Str(m["discarded_sid"])
		}
		if sid == "" && discarded == "" {
			continue
		}
		info := CrewInfo{Key: key, Cwd: cwd, Provider: provider}
		// 会話キーをファイル名にしたもの（Crew の history._safe_key と同じ）
		path := crewTranscriptPath(home, key)
		errs.file(path, core.ReadJSONL(path, func(e core.Obj) {
			if info.Title == "" && core.Str(e["_type"]) == "metadata" {
				info.Title = core.Str(e["title"])
				info.Agent = core.Str(e["agent"])
			}
		}))
		if sid != "" {
			out[sid] = info
		}
		if _, dup := former[discarded]; discarded != "" && discarded != sid && !dup {
			f := info
			f.Former = true
			former[discarded] = f
		}
	}
	for _, p := range glob(filepath.Join(home, "subagents", "*", "state.json")) {
		v, err := core.ReadJSONFile(p)
		errs.file(p, err)
		st := core.Map(v)
		sid := core.Str(st["session_id"])
		if sid == "" {
			continue
		}
		out[sid] = CrewInfo{Subagent: true, Key: core.Str(st["parent_session"]), Agent: core.Str(st["agent"]),
			Task: core.Runes(strings.TrimSpace(core.Str(st["task"])), 120), Cwd: core.Str(st["cwd"]), Provider: core.Str(st["provider"]),
			UsageSlot: subagentSlot(st, filepath.Base(filepath.Dir(p)))}
	}
	for sid, info := range former {
		if _, ok := out[sid]; !ok {
			out[sid] = info
		}
	}
	return out
}

// subagentSlot は、サブエージェントの使用量の記録の slot（loadCrewUsage のキーと同じ形）。
// Crew は conversation_key（残す（keep）サブエージェントだけ state.json に書く）か "subagent:<id>" で記録する
// （subagent_manager/run.py の _run_impl）。id は state.json の "id"、なければフォルダの名前（subagents/<id>/。subagent_persistence.py の _agent_dir）。
func subagentSlot(st core.Obj, dir string) string {
	if k := core.Str(st["conversation_key"]); k != "" {
		return crewSlot(k)
	}
	return crewSlot("subagent:" + firstNonEmpty(core.Str(st["id"]), dir))
}

// crewLogSegment は crew-log の 1 つのファイルの名前（log.jsonl か、後ろの分かれ log.<最初の seq>.jsonl。Crew の crew_log/store.py の segment_paths）。
var crewLogSegment = regexp.MustCompile(`^log(?:\.([0-9]{1,18}))?\.jsonl$`)

// crewSpawnedMark は subagent/spawned の行にだけある文字。ほかの行（ターン・ツールなど、ずっと多い）は JSON として読まない。
var crewSpawnedMark = []byte(`"subagent/spawned"`)

// loadCrewSpawns は、crew-log に残るサブエージェントの起動の記録（使用量の記録の slot "subagent:<id>" → Crew の情報）。
// Crew は届け終えたサブエージェントのフォルダ（subagents/<id>/。state.json）を 1 時間で消す（subagent_persistence.py の
// prune_stale_tombstones。異常終了は 7 日）。一時的な（temporary・incognito の）サブエージェントは state.json をそもそも書かない。
// そのあとも使用量の記録は "subagent:<id>" の slot で残るので、親がわからないと、ひとりで Crew のセッションになってしまう。
// 親の会話の crew-log（追記だけで書きかえない）には、親が起動した子が残る:
//
//	<Crew の場所>/crew-log/sessions/<名前>/log.jsonl（と log.<seq>.jsonl）
//	1 行目 {"type": "session", "id": ACP の会話 ID, "slot": 親の会話キー, "cwd", …}（crew_log/schema.py の SessionHeader）
//	2 行目から {"type": "subagent/spawned", "time", "src", "data": {"agent_id", "agent"?, "task"?, …}}（crew_log/emit.py の on_subagent_spawned）
//
// crew-log はダッシュボードの会話にだけある（サブエージェント自身の会話にはない）。同じ id が何度も出たら、最初に読んだものを使う。
func loadCrewSpawns(home string, errs *fileErrs) map[string]CrewInfo {
	out := map[string]CrewInfo{}
	if home == "" {
		return out
	}
	for _, dir := range glob(filepath.Join(home, "crew-log", "sessions", "*")) {
		if !realDir(dir) {
			continue
		}
		var slot, cwd string
		var spawns []crewSpawn
		seen := map[string]bool{} // 同じ会話の中で読んだ id（中身は最初のものだけ持つ）
		for _, p := range crewLogSegments(dir) {
			s, c, err := readCrewLog(p, seen, &spawns)
			errs.file(p, err)
			slot, cwd = firstNonEmpty(slot, s), firstNonEmpty(cwd, c)
		}
		if slot == "" {
			continue
		}
		for _, x := range spawns {
			key := crewSlot("subagent:" + x.id)
			if _, dup := out[key]; dup {
				continue
			}
			out[key] = CrewInfo{Subagent: true, Key: slot, Agent: x.agent, Task: x.task, Cwd: cwd, UsageSlot: key}
		}
	}
	return out
}

// crewSpawn は subagent/spawned の 1 行のうち kiroku が使うもの（読んだときに切りつめ、ほかの項目は持たない）。
type crewSpawn struct{ id, agent, task string }

// crewLogSegments は crew-log の 1 つの会話のファイルを、書いた順（seq の順。log.jsonl が先）に並べる。
func crewLogSegments(dir string) []string {
	type seg struct {
		p string
		n uint64
	}
	var segs []seg
	for _, p := range glob(filepath.Join(dir, "log*.jsonl")) {
		m := crewLogSegment.FindStringSubmatch(filepath.Base(p))
		if m == nil {
			continue
		}
		n, _ := strconv.ParseUint(m[1], 10, 64) // log.jsonl は 0
		segs = append(segs, seg{p, n})
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].n < segs[j].n })
	out := make([]string, len(segs))
	for i, x := range segs {
		out[i] = x.p
	}
	return out
}

// readCrewLog は crew-log のファイル 1 つから、会話キー（1 行目の slot か、session/opened の data.slot）と作業場所を読み、
// subagent/spawned の行を spawns に足す（seen にある id は飛ばす）。シンボリックリンクでないふつうのファイルだけ。
// 長すぎる行は飛ばし、その数をエラーで返す（ほかの履歴と同じく「Data sources」に出る）。
func readCrewLog(p string, seen map[string]bool, spawns *[]crewSpawn) (slot, cwd string, err error) {
	if st, err := os.Lstat(p); err != nil || !st.Mode().IsRegular() {
		return "", "", nil
	}
	f, err := os.Open(p)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<16)
	long := 0
	for first := true; ; first = false {
		line, tooLong, err := core.ReadLine(r, core.MaxLine)
		var e core.Obj
		switch {
		case tooLong:
			long++
		case first && json.Unmarshal(line, &e) == nil && core.Str(e["type"]) == "session":
			slot, cwd = core.Str(e["slot"]), core.Str(e["cwd"])
		case slot == "" && bytes.Contains(line, []byte(`"session/opened"`)):
			if json.Unmarshal(line, &e) == nil && core.Str(e["type"]) == "session/opened" {
				slot = core.Str(core.Map(e["data"])["slot"])
			}
		case bytes.Contains(line, crewSpawnedMark):
			if json.Unmarshal(line, &e) == nil && core.Str(e["type"]) == "subagent/spawned" {
				d := core.Map(e["data"])
				if id := core.Str(d["agent_id"]); id != "" && !seen[id] {
					seen[id] = true
					*spawns = append(*spawns, crewSpawn{id: id, agent: core.Runes(core.Str(d["agent"]), 120),
						task: core.Runes(strings.TrimSpace(core.Str(d["task"])), 120)})
				}
			}
		}
		if err == io.EOF {
			if long > 0 {
				return slot, cwd, fmt.Errorf("skipped %d line(s) longer than %d bytes", long, core.MaxLine)
			}
			return slot, cwd, nil
		}
		if err != nil {
			return slot, cwd, err
		}
	}
}

// subagentTitle はサブエージェントの会話のタイトル（"Subagent <エージェント>: <依頼>"）。
func subagentTitle(info CrewInfo) string {
	title := "Subagent"
	if info.Agent != "" {
		title += " " + info.Agent
	}
	if info.Task != "" {
		title += ": " + info.Task
	}
	return title
}

// crewHeld は、出す前の Crew の会話。サブエージェントの会話は、親の会話（parent_session の会話キー）が見つかれば
// その会話のサブエージェントにまとめる（Claude Code・Codex と同じく、親のセッションの中に出す）。親が見つからなければ、そのまま出す。
type crewHeld struct {
	list    []heldCrew
	parents map[string]*core.Builder // 会話キー（crewSlot でそろえたもの）→ 親になれる会話
	byPath  map[string]*core.Builder // 会話の記録だけがある会話（記録のファイル → 会話）
}

type heldCrew struct {
	b    *core.Builder
	info *CrewInfo // わからなければ nil
}

// add は Crew の会話を 1 つ預かる。key はその会話の会話キー（サブエージェントなら親の会話キー）。
func (h *crewHeld) add(b *core.Builder, info *CrewInfo, key string) {
	h.list = append(h.list, heldCrew{b: b, info: info})
	if (info == nil || !info.Subagent) && key != "" {
		if k := crewSlot(key); h.parents[k] == nil {
			h.parents[k] = b
		}
	}
}

// flush はサブエージェントを親の会話にまとめてから、全部の会話を出す。
func (h *crewHeld) flush(home string, emit func(*core.Builder)) {
	folded := map[*core.Builder]bool{}
	for _, x := range h.list {
		if x.info == nil || !x.info.Subagent || x.info.Key == "" {
			continue
		}
		parent := h.parents[crewSlot(x.info.Key)]
		if parent == nil && home != "" {
			parent = h.byPath[crewTranscriptPath(home, x.info.Key)]
		}
		if parent == nil || parent == x.b {
			continue
		}
		foldCrewSubagent(parent, x.b, *x.info)
		folded[x.b] = true
	}
	for _, x := range h.list {
		if folded[x.b] {
			continue
		}
		sort.SliceStable(x.b.Subagents, func(i, j int) bool { return lessTime(x.b.Subagents[i].Start, x.b.Subagents[j].Start) })
		emit(x.b)
	}
}

// lessTime は、時刻のあるものを先に、早い順に並べる。
func lessTime(a, b *float64) bool {
	if a == nil || b == nil {
		return a != nil && b == nil
	}
	return *a < *b
}

// foldCrewSubagent はサブエージェントの会話を、親の会話のサブエージェントとして入れる。
// クレジット・Crew の参考指標（Crew から動かした会話・うちサブエージェントも）・編集したファイルは親の会話の数字に入れる
// （まとめる前と、合計は変わらない）。トークンとドル額はサブエージェントの使用量として入り、親のセッションの合計に足される。
// サブエージェントが動いた時刻は、親の会話で AI が動いていた時刻にする（Claude Code と同じく、作業時間に入り、待たせ時間にならない）。
func foldCrewSubagent(parent, sub *core.Builder, info CrewInfo) {
	for i := range sub.Times {
		parent.Agent(&sub.Times[i])
	}
	parent.Measures = append(parent.Measures, sub.Measures...)
	parent.Credits = append(parent.Credits, sub.Credits...)
	for _, f := range sub.EditedFiles() {
		parent.Edited(f)
	}
	var start, end *float64
	if len(sub.Times) > 0 {
		ts := append([]float64(nil), sub.Times...)
		sort.Float64s(ts)
		start, end = &ts[0], &ts[len(ts)-1]
	}
	var model *string
	if m := sub.TopModel(); m != "" {
		model = &m
	}
	tools := 0.0
	for _, n := range sub.ToolCounts() {
		tools += float64(n)
	}
	evs := sub.Usage.Events()
	parent.Subagents = append(parent.Subagents, core.Subagent{Type: firstNonEmpty(info.Agent, "subagent"), Desc: info.Task,
		Start: start, End: end, Model: model, Tools: tools, Usage: core.SumUsage(evs), Events: evs})
}

// crewRow は Crew の会話の 1 行。
type crewRow struct {
	t     *float64
	role  string
	text  string
	tools []string
	human bool // meta.human が true（人が書いた行。Crew の history.HUMAN_TURN_META_KEY）
}

// crewMeta は Crew の会話の記録のメタデータ（1 行目）。
type crewMeta struct {
	title      string
	forkedFrom string   // 会話を分けて（fork して）作ったなら、元の会話キー（"forked_from"）
	created    *float64 // "created_at"（その記録を作った時刻。fork なら分けた時刻）
}

// add は、まだ決まっていない項目を m2 で埋める。
func (m *crewMeta) add(m2 crewMeta) {
	m.title = firstNonEmpty(m.title, m2.title)
	m.forkedFrom = firstNonEmpty(m.forkedFrom, m2.forkedFrom)
	if m.created == nil {
		m.created = m2.created
	}
}

// ownRows は、fork した会話から、元の会話から写した行を除く。
// Crew は fork するとき、新しい会話（created_at はそのときの時刻）に元の会話の行を元の ts と meta のまま写す
// （dashboard/chat_fork.py。tail fork も同じ）。そのあとの行の ts は、写した行より後で created_at より後になる
// （state.py の _ChatSlot.append と history.py の monotonic_transcript_ts）。なので created_at より前の行は写した行。
// 元の会話は kiroku が別に読むので、写した行も数えると同じ依頼を 2 度数える。
// created_at がない・読めない fork は見分けられないので、全部の行を使う。ts のない行も残す。
func ownRows(m crewMeta, rows []crewRow) []crewRow {
	if m.forkedFrom == "" || m.created == nil {
		return rows
	}
	out := rows[:0:0]
	for _, r := range rows {
		if r.t != nil && *r.t < *m.created {
			continue
		}
		out = append(out, r)
	}
	return out
}

// crewTranscriptPath は会話キーの記録ファイル（Crew の history._safe_key と同じ名前）。
func crewTranscriptPath(home, key string) string {
	return filepath.Join(home, "sessions", safeKey(key)+".jsonl")
}

// readCrewKey は会話キーの記録を読む（readCrewStem）。
func readCrewKey(home, arch, key string, errs *fileErrs) (title string, rows []crewRow) {
	return readCrewStem(home, arch, safeKey(key), errs)
}

// archiveSeg は退避した記録のファイル名の <名前>__ より後（<YYYYMMDD-HHMMSS>.jsonl。同じ秒に重なれば <日時>-<番号>.jsonl）。
// Crew の history._archive_lines が書く（番号は 1 から。番号なしが先）。
var archiveSeg = regexp.MustCompile(`^(\d{8}-\d{6})(?:-(\d{1,9}))?\.jsonl$`)

// crewSeg は退避した記録のファイル 1 つ。
type crewSeg struct {
	path, stamp string
	n           int
}

// readCrewStem は会話の記録（sessions/<stem>.jsonl。stem は safeKey の名前）を読む。Crew は古い行を sessions/archive/<名前>__<日時>.jsonl に退避する
// （残す期間は session.archive_retention_days で決まる。既定 30 日。crewRetentionDays）ので、残っていればそちらも書いた順に読む。
// 退避した記録が消えていても、kiroku archive のコピー（arch の下の同じ並び。.jsonl.zst）があればそれを読む。
// 名前が <stem>__ で始まっても、そのあとが日時でないもの（<stem>__x という別の会話のもの）は読まない。
// fork した会話なら、元の会話から写した行は除く（ownRows）。
func readCrewStem(home, arch, stem string, errs *fileErrs) (title string, rows []crewRow) {
	var segs []crewSeg
	add := func(p, name string) {
		m := archiveSeg.FindStringSubmatch(strings.TrimPrefix(name, stem+"__"))
		if m == nil || !strings.HasPrefix(name, stem+"__") {
			return
		}
		n, _ := strconv.Atoi(m[2]) // 番号なしは 0
		segs = append(segs, crewSeg{path: p, stamp: m[1], n: n})
	}
	for _, p := range glob(filepath.Join(home, "sessions", "archive", stem+"__*.jsonl")) {
		add(p, filepath.Base(p))
	}
	if arch != "" {
		for _, p := range glob(filepath.Join(arch, "sessions", "archive", stem+"__*.jsonl.zst")) {
			name := strings.TrimSuffix(filepath.Base(p), ".zst")
			if !isFile(filepath.Join(home, "sessions", "archive", name)) {
				add(p, name)
			}
		}
	}
	// 書いた順: 日時の順、同じ日時なら番号の順（名前の順だと "-1" が番号なしより先、"-10" が "-2" より先になる）
	sort.Slice(segs, func(i, j int) bool {
		if segs[i].stamp != segs[j].stamp {
			return segs[i].stamp < segs[j].stamp
		}
		return segs[i].n < segs[j].n
	})
	paths := make([]string, 0, len(segs)+1)
	for _, s := range segs {
		paths = append(paths, s.path)
	}
	var meta crewMeta
	for _, p := range append(paths, filepath.Join(home, "sessions", stem+".jsonl")) {
		m, rs := readCrewTranscript(p, errs)
		meta.add(m)
		rows = append(rows, rs...)
	}
	return meta.title, ownRows(meta, rows)
}

// readCrewTranscript は Crew の会話の記録を読む。meta は 1 行目のメタデータ。
// 途中で読めなくなっても、読めた行までは返す。fork の写した行は除かない（ownRows）。
func readCrewTranscript(path string, errs *fileErrs) (meta crewMeta, rows []crewRow) {
	errs.file(path, core.ReadJSONL(path, func(e core.Obj) {
		if core.Str(e["_type"]) == "metadata" {
			meta.add(crewMeta{title: core.Str(e["title"]), forkedFrom: core.Str(e["forked_from"]), created: ts(e["created_at"])})
			return
		}
		r := crewRow{t: ts(e["ts"]), role: core.Str(e["role"]), text: core.TextOf(e["content"])}
		r.human, _ = core.Map(e["meta"])["human"].(bool) // Crew と同じく true だけ（"true" や 1 は数えない）
		for _, x := range core.List(e["tools"]) {
			if n := core.Str(x); n != "" {
				r.tools = append(r.tools, n)
			}
		}
		if r.role != "" {
			rows = append(rows, r)
		}
	}))
	return meta, rows
}

// taskRunnerRow は、タスクの実行が会話の記録に書く user の行（taskrunner.py の _log_task: "[Task: <spec の名前>] Task <番号>: <題>"）。
var taskRunnerRow = regexp.MustCompile(`^\[Task: [^\n]*\] Task \d+: `)

// addCrewRows は Crew の会話の記録から、依頼の流れ・時刻・使ったツールを足す。
// kiro-cli の履歴に依頼が残っていない会話（Crew のダッシュボードから動かしたものなど）のため。
//
// role が user の行は、人が書いたとは限らない。Crew は予定の実行（"# Cron Run: …"）・Issue Radar の呼び出し・
// タスクの実行・auto-go なども user の行として書き、人が書いた行にだけ meta.human: true をつける
// （history.py の HUMAN_TURN_META_KEY。目印のない行は人のものと数えない、という allowlist）。
// 目印は新しい Crew だけが書くので、最初に目印のある行からあとは、目印のある行だけを依頼に数え、
// ほかの user の行は仕組みが入れたもの（kind "agent"。Claude Code のほかのエージェントや予定から送られたものと同じ）として残す。
// それより前の行（目印ができる前の記録）は、ほぼ見分けられないので今までどおり依頼に数える。
// ただしタスクの実行の行（taskRunnerRow）は形で見分けられ、目印も書かれない（タスクの実行だけの記録には目印のある行がない）ので、仕組みが入れたものにする。
// Crew 自身は目印のない記録で "[" で始まる user の行をすべて仕組みのものとみなす（dashboard/handlers/members.py）が、
// 人が "[WIP] …" のように書いた依頼まで落とさないよう、kiroku は形の決まったタスクの実行の行だけを除く。
func addCrewRows(s *core.Builder, rows []crewRow) {
	marked := false // 目印を書く Crew の記録に入ったか
	for _, r := range rows {
		marked = marked || r.human
		switch r.role {
		case "user":
			s.Tick(r.t)
			if marked && !r.human || !marked && taskRunnerRow.MatchString(r.text) {
				s.InjectAll(r.t, "agent", r.text)
			} else {
				s.Prompt(r.t, r.text)
			}
		case "assistant":
			s.Agent(r.t)
			s.Reply(r.t, "", r.text)
		default: // tool / tool_call / tool_result など
			s.Agent(r.t)
		}
		for _, n := range r.tools {
			s.Tool(n, nil)
		}
	}
}

// tagCrew は Crew から動かした kiro-cli の会話に目印とタイトルをつける。つけたら true。
func tagCrew(s *core.Builder, crew map[string]CrewInfo) bool {
	info, ok := crew[s.ID]
	if ok {
		tagCrewInfo(s, info)
	}
	return ok
}

// tagCrewInfo は kiro-cli の会話に、Crew から動かした目印とタイトルをつける。
func tagCrewInfo(s *core.Builder, info CrewInfo) {
	s.Source = "Kiro Crew"
	var first *float64
	for i := range s.Times {
		if first == nil || s.Times[i] < *first {
			first = &s.Times[i]
		}
	}
	s.Measure("crew_sessions", first, 1)
	if info.Subagent {
		s.Measure("crew_subagents", first, 1)
	}
	if info.Subagent {
		s.Title = subagentTitle(info)
	} else if info.Title != "" {
		s.Title = info.Title
	}
}

// crewOwner は、使用量の記録の slot に結びつく kiro-cli の会話。
//
// Crew は会話キーの sid を差しかえる（SessionMap.set。容量がいっぱいになった会話の作り直し・backend の切りかえ・seed）が、
// 使用量の記録は会話キー（slot）で書き、どの kiro-cli の会話のものかは書かない（usage.py の _build_token_record）。
// 前の会話は session_map から消えても、kiro-cli の履歴に自分のクレジットとともに残る（persistent でない会話は Crew が消す）。
// なので slot の行を全部今の会話に足すと 2 度数える。行（と会話の記録の行）は時刻で、そのころに動いていた会話に結びつける（crewOwnerAt）。
type crewOwner struct {
	s          *core.Builder
	info       CrewInfo
	start, end *float64 // 会話の最初と最後の時刻（なければ nil）
	turns      []crewTurn
	rows       []crewRow
}

// newCrewOwner は kiro-cli の会話 s を、info の会話キーの会話にする。
func newCrewOwner(s *core.Builder, info CrewInfo) *crewOwner {
	o := &crewOwner{s: s, info: info}
	o.start, o.end = timeSpan(s.Times)
	return o
}

// timeSpan は時刻の最初と最後（なければ nil）。
func timeSpan(ts []float64) (first, last *float64) {
	for i := range ts {
		if first == nil || ts[i] < *first {
			first = &ts[i]
		}
		if last == nil || ts[i] > *last {
			last = &ts[i]
		}
	}
	return first, last
}

// crewTurnSlack は、kiro-cli の会話の最後の時刻（ターンの終わり）から、Crew がそのターンの使用量の記録を書くまでの余裕（秒）。
const crewTurnSlack = 120

// sortCrewOwners は会話を始まりの順に並べる（時刻のない会話が先）。
func sortCrewOwners(ow []*crewOwner) {
	sort.SliceStable(ow, func(i, j int) bool {
		a, b := ow[i].start, ow[j].start
		if a == nil || b == nil {
			return a == nil && b != nil
		}
		return *a < *b
	})
}

// crewOwnerAt は、時刻 t の行を結びつける会話の番号（ow は sortCrewOwners の順）。どれでもなければ -1。
// 行は、t より前に始まった会話のうち、t がその会話の間に入る最後のものに結びつける。今の会話（Former でない）の間は始まりから先の全部
// （kiro-cli に記録が残らないターンがあるので）、前の会話の間は最後の時刻 + crewTurnSlack まで
// （backend を切りかえたあとの行は、kiro-cli の会話のものではない）。時刻のない前の会話には結びつけない。
func crewOwnerAt(ow []*crewOwner, t float64) int {
	for i := len(ow) - 1; i >= 0; i-- {
		o := ow[i]
		if o.info.Former {
			if o.start == nil || t < *o.start || t > *o.end+crewTurnSlack {
				continue
			}
			return i
		}
		if o.start == nil || t >= *o.start {
			return i
		}
	}
	return -1
}

// crewKiroRow は kiro-cli の行らしいか（トークンもドル額もない。kiro-cli はクレジットだけを返す）。
func crewKiroRow(x crewTurn) bool {
	return x.u.Total() == 0 && x.cost == 0
}

// claimCrewFormer は、session_map から消えた前の kiro-cli の会話（Crew の目印のない会話）を、どの会話にも結びつかない行から見つけて ow に足す。
// 行の時刻が会話の最初から最後 + crewTurnSlack に入り、作業場所（session_map の cwd）が同じ会話が 1 つだけなら、その会話にする。
// 2 つ以上あれば決めない（その行は Crew のセッションになる）。taken は、もう別の会話キーの前の会話にしたもの。
// 作業場所がわからない会話キー（古い session_map）とサブエージェントでは探さない。
func claimCrewFormer(ow []*crewOwner, turns []crewTurn, plain []*core.Builder, info CrewInfo, taken map[*core.Builder]bool) []*crewOwner {
	if info.Subagent || info.Cwd == "" {
		return ow
	}
	spans := make([][2]*float64, len(plain))
	for i, p := range plain {
		spans[i][0], spans[i][1] = timeSpan(p.Times)
	}
	for _, x := range turns {
		if crewOwnerAt(ow, x.t) >= 0 || !crewKiroRow(x) {
			continue
		}
		found := -1
		for i, p := range plain {
			first, last := spans[i][0], spans[i][1]
			if p.Project != info.Cwd || first == nil || x.t < *first || x.t > *last+crewTurnSlack {
				continue
			}
			if found >= 0 {
				found = -2 // 2 つ以上
				break
			}
			found = i
		}
		if found < 0 || taken[plain[found]] {
			continue
		}
		taken[plain[found]] = true
		f := info
		f.Former = true
		tagCrewInfo(plain[found], f)
		ow = append(ow, newCrewOwner(plain[found], f))
		sortCrewOwners(ow)
	}
	return ow
}

// crewTurn は usage/tokens の 1 行（Crew が記録した 1 ターン）。
type crewTurn struct {
	t, start, credits float64
	model, provider   string      // provider は行の "provider"（"acp"・"claude_code"・"codex" など。crewElsewhere）
	u                 core.Tokens // input → In、output → Out、cache_create → CW、cache_read → CR（重ならない）
	cost              float64     // backend が返した USD（kiro-cli の行は 0）
	ctx               float64     // context_used ÷ context_window（わからなければ -1）
	stop              string      // stop_reason（"" は記録なし）
}

// crewCount は行のトークン数やドル額。数でない・負の数・有限でない値は 0。
func crewCount(v any) float64 {
	f, ok := core.Num(v)
	if !ok || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

// loadCrewUsage は usage/tokens/*.jsonl を会話キー（slot）ごとにまとめる。時刻の古い順。
func loadCrewUsage(home string, errs *fileErrs) map[string][]crewTurn {
	out := map[string][]crewTurn{}
	if home == "" {
		return out
	}
	for _, p := range glob(filepath.Join(home, "usage", "tokens", "*.jsonl")) {
		errs.file(p, core.ReadJSONL(p, func(e core.Obj) {
			if core.Str(e["_type"]) != "tokens" {
				return
			}
			t := ts(e["ts"])
			c, ok := core.Num(e["credits"])
			if t == nil || !ok || c < 0 {
				return
			}
			start := *t
			if d, ok := core.Num(e["duration_ms"]); ok && d > 0 {
				start -= d / 1000
			}
			x := crewTurn{t: *t, start: start, credits: c, model: core.Str(e["model"]), provider: core.Str(e["provider"]),
				u:    core.Tokens{In: crewCount(e["input"]), Out: crewCount(e["output"]), CW: crewCount(e["cache_create"]), CR: crewCount(e["cache_read"])},
				cost: crewCount(e["cost"]), ctx: -1, stop: core.Str(e["stop_reason"])}
			// コンテキストの使用率。窓がない・0 の行は使わない（Crew の context_occupancy と同じ）
			if used, ok := core.Num(e["context_used"]); ok && used >= 0 && !math.IsInf(used, 0) {
				if w, ok := core.Num(e["context_window"]); ok && w > 0 && !math.IsInf(w, 0) {
					x.ctx = min(used/w, 1)
				}
			}
			slot := crewSlot(core.Str(e["slot"]))
			out[slot] = append(out[slot], x)
		}))
	}
	for k := range out {
		sort.SliceStable(out[k], func(i, j int) bool { return out[k][i].t < out[k][j].t })
	}
	return out
}

// crewElsewhere は、その行のトークンとドル額を、kiroku が別に読む履歴が記録しているか（crewOwnHistory）。
// Crew の Claude Code の seam は claude-agent-acp で Claude Code を動かし、Claude Code が ~/.claude/projects に自分の記録を書く
// （Crew の providers/acp.py の cleanup_session の注記）。Codex の backend も Codex 自身が ~/.codex に記録を書くものとみなす。
// Crew の会話 ID（session_map の sid）とそれらの履歴の会話 ID が同じかは、手元の根拠（Crew と claude-agent-acp の公開ソース）では確かめられない。
// 結びつけられないまま足すと同じトークンとコストを 2 度数えるので、これらの行ではトークンとドル額を足さない（ターン・時刻・モデルは数える）。
//
// 行の provider は書いたところで意味が違う: ダッシュボード・裏方の処理・記憶の整理は backend の名前（providers/acp.py の provider_label。
// "acp"・"claude_code"・"codex"・"kas" など）、サブエージェントは "claude_code" か "acp"（subagent_manager/run.py）、
// タスクの実行と webhook は設定の agent.provider（いつも "acp"）。なので label（session_map の provider か、サブエージェントの state.json の
// "provider"。わからなければ空）も見る。タスクの実行と webhook の行は、Claude Code で動いても "acp" で、会話キーが session_map から
// 消えていると見分けられないので、トークンを数える（2 度数えることがある）。
func crewElsewhere(x crewTurn, label string) bool {
	return crewOwnHistory(x.provider) || crewOwnHistory(label)
}

// crewOwnHistory は、Crew の backend の名前が、kiroku が自分の履歴を別に読むエージェント（Claude Code と Codex）か。
func crewOwnHistory(provider string) bool {
	return provider == "claude_code" || provider == "codex"
}

func sumCredits(cs []core.Credit) float64 {
	v := 0.0
	for _, c := range cs {
		v += c.V
	}
	return v
}

// useCrewCredits は、Crew の記録のほうが多ければ、その会話のクレジットを Crew の記録に置きかえる。置きかえたら true。
func useCrewCredits(s *core.Builder, turns []crewTurn) bool {
	crew := 0.0
	for _, x := range turns {
		crew += x.credits
	}
	if crew <= sumCredits(s.Credits)+1e-9 {
		return false
	}
	s.Credits = nil
	ms := s.Measures[:0]
	for _, m := range s.Measures {
		if m.Key != "credits" && m.Key != "turns" {
			ms = append(ms, m)
		}
	}
	s.Measures = ms
	addCrewTurns(s, turns, "")
	return true
}

// addCrewTurns は Crew の行をターンとして足す。label は会話の session_map の provider（わからなければ空）。
// トークンかドル額のある行（kiro-cli 以外の backend）は使用量としても足す（crewElsewhere の行は除く）。
// ドル額は会話の累計の差分なので、会話のどこかに cost があれば、cost が 0 の行も 0 ドルとして扱う（料金表で見積もると 2 度数えるため）。
// どの行にも cost がなければ、料金表で見積もる（料金表にないモデルは見積もらない）。
// 戻り値は、crewElsewhere でトークンとドル額を足さなかった行の数。
func addCrewTurns(s *core.Builder, turns []crewTurn, label string) (skipped int) {
	hasCost := false
	for _, x := range turns {
		if x.cost > 0 && !crewElsewhere(x, label) {
			hasCost = true
		}
	}
	for _, x := range turns {
		t, st := x.t, x.start
		s.Tick(&st)
		s.Agent(&t)
		if x.credits != 0 {
			s.Credits = append(s.Credits, core.Credit{T: &t, V: x.credits})
		}
		s.Measure("credits", &t, x.credits)
		s.Measure("turns", &t, 1)
		billed := x.u.Total() > 0 || x.cost > 0
		switch {
		case billed && crewElsewhere(x, label):
			skipped++
			s.Model(x.model)
		case billed:
			e := core.Event{T: &t, Model: x.model, U: x.u}
			if hasCost {
				c := x.cost
				e.Cost = &c
			}
			s.AddEvent(e) // モデルも数える
		default:
			s.Model(x.model)
		}
	}
	return skipped
}

// addCrewNative は Crew の行から、Crew だけが記録している数字（コンテキストの使用率・正常に終わらなかったターン）を足す。
// stop_reason が end_turn 以外（cancelled・refusal・max_tokens・"error: …" など）のターンを数える。stop_reason のない行は数えない。
// Crew は何も使わなかったターンの行を書かない（usage_has_billing）ので、そうしたターンは入らない。
func addCrewNative(s *core.Builder, turns []crewTurn) {
	for _, x := range turns {
		t := x.t
		if x.ctx >= 0 {
			s.Measure("context_used", &t, x.ctx)
		}
		if x.stop != "" {
			v := 0.0
			if x.stop != "end_turn" {
				v = 1
			}
			s.Measure("stopped_early", &t, v)
		}
	}
}

// crewOnly は kiro-cli の会話に結びつかない Crew の記録を、Crew のセッションにする。
// 裏方の処理（_bg と、記憶の場所ごとの記憶の整理。crewBackground）は 1 日ごとにまとめる。skipped は、crewElsewhere でトークンとドル額を足さなかった行の数。
// title と rows は、その slot の会話の記録（のうち kiro-cli の会話に結びつかなかった行。readCrewKey）。裏方の処理では使わない。
func crewOnly(home, slot string, turns []crewTurn, info *CrewInfo, title string, rows []crewRow) (out []*core.Builder, skipped int) {
	bg := crewBackground(slot)
	if bg {
		title, rows = "", nil
	}
	groups := map[string][]crewTurn{}
	var order []string
	for _, x := range turns {
		g := ""
		if bg {
			g = time.Unix(int64(x.t), 0).In(time.Local).Format("2006-01-02")
		}
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], x)
	}
	label := ""
	if info != nil {
		label = info.Provider
	}
	for _, g := range order {
		id := "crew:" + slot
		if g != "" {
			id += ":" + g
		}
		s := core.NewBuilder("Kiro Crew", id)
		if !bg && home != "" && len(rows) > 0 {
			s.File = crewTranscriptPath(home, slot)
		}
		s.Key = "kiro-crew:" + id[len("crew:"):]
		switch {
		case slot == "_bg":
			s.Title = "Kiro Crew background work"
		case bg:
			s.Title = "Kiro Crew memory consolidation (" + strings.TrimPrefix(slot, "memory-consolidation:") + ")"
		case info != nil && info.Subagent: // kiro-cli の会話が消えたサブエージェント（残さないものは Crew が消す）
			s.Title = subagentTitle(*info)
		case info != nil && info.Title != "":
			s.Title = info.Title
		case title != "":
			s.Title = title
		default:
			s.Title = "Kiro Crew: " + slot
		}
		s.Project = "(Kiro Crew)"
		if info != nil && info.Cwd != "" {
			s.Project = info.Cwd
		}
		skipped += addCrewTurns(s, groups[g], label)
		addCrewNative(s, groups[g])
		addCrewRows(s, rows)
		first := groups[g][0].start
		s.Measure("crew_sessions", &first, 1)
		if info != nil && info.Subagent {
			s.Measure("crew_subagents", &first, 1)
		}
		out = append(out, s)
	}
	return out, skipped
}

// crewRetentionDefault は Crew の session.archive_retention_days の既定値（kiro_crew/config/sections.py の SessionConfig）。
const crewRetentionDefault = 30

// crewRetentionNever は、これより長い日数を「消さない」と同じに扱う境目（約 2700 年。int にしても溢れない）。
const crewRetentionNever = 1_000_000

// configMax は設定ファイル（Crew の config.json、Claude Code の managed-settings.json など）として読む大きさの上限。
// これより大きいファイルは読まない。
const configMax = 8 << 20

// crewRetentionDays は Crew が使う session.archive_retention_days（日数）を読む。never は、Crew が古い記録を消さない設定のとき。
// Crew と同じく <Crew の場所>/config.json に config.local.json を重ねて読む（config.local.json が勝つ。
// "session" がどちらも object ならキーごとに重ね、そうでなければ上書き）。書き方は {"session": {"archive_retention_days": 30}}。
// Crew の読み方（config/sections.py の _archive_retention_days と config/validation.py）に合わせて:
//   - null か負の数は消さない（Crew は -1 にそろえて、片付けをしない）
//   - 0 以上の整数（30.0 のような小数点つきの整数や、"45" のような整数の文字列も）はその日数
//   - キーがない・整数でない数・数でない値・"session" が object でない・ファイルがないか壊れているときは既定の 30 日
func crewRetentionDays(home string) (days int, never bool) {
	v, ok := crewSessionSetting(home, "archive_retention_days")
	if !ok {
		return crewRetentionDefault, false
	}
	var f float64
	switch x := v.(type) {
	case nil:
		return 0, true
	case float64:
		f = x
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return crewRetentionDefault, false
		}
		f = float64(n)
	default: // true / false や object など。Crew は型が違う値を捨てて既定値にする
		return crewRetentionDefault, false
	}
	switch {
	case math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f):
		return crewRetentionDefault, false
	case f < 0 || f > crewRetentionNever:
		return 0, true
	}
	return int(f), false
}

// crewSessionSetting は config.json に config.local.json を重ねたときの "session" の key の値。ないなら ok が false。
func crewSessionSetting(home, key string) (v any, ok bool) {
	base := configObject(filepath.Join(home, "config.json"))
	local := configObject(filepath.Join(home, "config.local.json"))
	sec := base["session"]
	if l, has := local["session"]; has {
		lm, lok := l.(map[string]any)
		if bm, bok := sec.(map[string]any); lok && bok {
			if v, ok := lm[key]; ok {
				return v, true
			}
			v, ok := bm[key]
			return v, ok
		}
		sec = l
	}
	m, isMap := sec.(map[string]any)
	if !isMap {
		return nil, false
	}
	v, ok = m[key]
	return v, ok
}

// configObject は設定ファイル（JSON の object。Crew の設定や Claude Code の設定）を読む。ない・ふつうのファイルでない・大きすぎる・
// 読めない・壊れている・object でないなら nil（Crew もそのファイルを無視して既定値で動く）。手で編集したファイルの先頭の BOM は、Crew と同じく読み飛ばす。
func configObject(path string) map[string]any {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() > configMax {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, configMax+1))
	if err != nil || len(b) > configMax {
		return nil
	}
	var v any
	if json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &v) != nil {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}
