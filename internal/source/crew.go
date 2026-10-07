package source

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
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
// 同じ会話は二重に数えないよう、会話ごとに kiro-cli の記録と Crew の記録の多いほうを使う。
// kiro-cli の会話に結びつかない記録（Crew の裏方の処理 _bg など）は、Crew のセッションとして数える。
//
//	<KIROCREW_HOME か ~/.kiro/crew>/session_map.json        {会話キー: {"sid": kiro-cli の会話 ID, "cwd": …} か "sid"}
//	<…>/sessions/<キー>.jsonl                               1 行目 {"_type": "metadata", "title", "agent", "model", …}
//	                                                         2 行目から {"role": "user"|"assistant"|…, "content", "ts", "tools"?: [ツール名]}
//	<…>/subagents/<id>/state.json                          {"task", "agent", "parent_session", "session_id"?, …}
//	<…>/usage/tokens/<YYYY-MM-DD>.jsonl                     {"_type": "tokens", "ts", "slot": 会話キー, "model", "credits", "duration_ms", …}

// CrewInfo は 1 つの kiro-cli の会話についての Crew の情報。
type CrewInfo struct {
	Subagent bool
	Key      string // Crew の会話キー（サブエージェントなら親の会話キー）
	Title    string
	Agent    string
	Task     string
	Cwd      string
}

// DefaultCrewHome は KIROCREW_HOME か <KIRO_HOME か ~/.kiro>/crew。
func DefaultCrewHome(kiroHome string) string {
	if d := os.Getenv("KIROCREW_HOME"); d != "" {
		return d
	}
	return filepath.Join(kiroHome, "crew")
}

var unsafeKey = regexp.MustCompile(`[^\w\-.]`)

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
	for key, v := range core.Map(sm) {
		sid, cwd := core.Str(v), "" // 古い形は文字列だけ
		if m := core.Map(v); m != nil {
			sid, cwd = core.Str(m["sid"]), core.Str(m["cwd"])
		}
		if sid == "" {
			continue
		}
		info := CrewInfo{Key: key, Cwd: cwd}
		// 会話キーをファイル名にしたもの（Crew の history._safe_key と同じ）
		path := filepath.Join(home, "sessions", unsafeKey.ReplaceAllString(key, "_")+".jsonl")
		errs.file(path, core.ReadJSONL(path, func(e core.Obj) {
			if info.Title == "" && core.Str(e["_type"]) == "metadata" {
				info.Title = core.Str(e["title"])
				info.Agent = core.Str(e["agent"])
			}
		}))
		out[sid] = info
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
			Task: core.Runes(strings.TrimSpace(core.Str(st["task"])), 120)}
	}
	return out
}

// crewRow は Crew の会話の 1 行。
type crewRow struct {
	t     *float64
	role  string
	text  string
	tools []string
}

// crewTranscriptPath は会話キーの記録ファイル（Crew の history._safe_key と同じ名前）。
func crewTranscriptPath(home, key string) string {
	return filepath.Join(home, "sessions", unsafeKey.ReplaceAllString(key, "_")+".jsonl")
}

// readCrewKey は会話キーの記録を読む。Crew は古い行を sessions/archive/<名前>__<日時>.jsonl に退避する
// （残す期間は session.archive_retention_days で決まる。既定 30 日。crewRetentionDays）ので、残っていればそちらも古い順に読む。
// 退避した記録が消えていても、kiroku archive のコピー（arch の下の同じ並び。.jsonl.zst）があればそれを読む。
func readCrewKey(home, arch, key string, errs *fileErrs) (title string, rows []crewRow) {
	stem := unsafeKey.ReplaceAllString(key, "_")
	segs := glob(filepath.Join(home, "sessions", "archive", stem+"__*.jsonl"))
	if arch != "" {
		for _, p := range glob(filepath.Join(arch, "sessions", "archive", stem+"__*.jsonl.zst")) {
			if !isFile(filepath.Join(home, "sessions", "archive", strings.TrimSuffix(filepath.Base(p), ".zst"))) {
				segs = append(segs, p)
			}
		}
	}
	sort.Slice(segs, func(i, j int) bool { return filepath.Base(segs[i]) < filepath.Base(segs[j]) }) // 名前の日時の順
	for _, p := range append(segs, crewTranscriptPath(home, key)) {
		t, rs := readCrewTranscript(p, errs)
		title = firstNonEmpty(title, t)
		rows = append(rows, rs...)
	}
	return title, rows
}

// readCrewTranscript は Crew の会話の記録を読む。title は 1 行目のメタデータのタイトル。
// 途中で読めなくなっても、読めた行までは返す。
func readCrewTranscript(path string, errs *fileErrs) (title string, rows []crewRow) {
	errs.file(path, core.ReadJSONL(path, func(e core.Obj) {
		if core.Str(e["_type"]) == "metadata" {
			title = firstNonEmpty(title, core.Str(e["title"]))
			return
		}
		r := crewRow{t: ts(e["ts"]), role: core.Str(e["role"]), text: core.TextOf(e["content"])}
		for _, x := range core.List(e["tools"]) {
			if n := core.Str(x); n != "" {
				r.tools = append(r.tools, n)
			}
		}
		if r.role != "" {
			rows = append(rows, r)
		}
	}))
	return title, rows
}

// addCrewRows は Crew の会話の記録から、依頼の流れ・時刻・使ったツールを足す。
// kiro-cli の履歴に依頼が残っていない会話（Crew のダッシュボードから動かしたものなど）のため。
func addCrewRows(s *core.Builder, rows []crewRow) {
	for _, r := range rows {
		switch r.role {
		case "user":
			s.Tick(r.t)
			s.Prompt(r.t, r.text)
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
	if !ok {
		return false
	}
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
		title := "Subagent"
		if info.Agent != "" {
			title += " " + info.Agent
		}
		if info.Task != "" {
			title += ": " + info.Task
		}
		s.Title = title
	} else if info.Title != "" {
		s.Title = info.Title
	}
	return true
}

// crewTurn は usage/tokens の 1 行（Crew が記録した 1 ターン）。
type crewTurn struct {
	t, start, credits float64
	model             string
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
			slot := spendKey(core.Str(e["slot"]))
			out[slot] = append(out[slot], crewTurn{t: *t, start: start, credits: c, model: core.Str(e["model"])})
		}))
	}
	for k := range out {
		sort.SliceStable(out[k], func(i, j int) bool { return out[k][i].t < out[k][j].t })
	}
	return out
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
	addCrewTurns(s, turns)
	return true
}

func addCrewTurns(s *core.Builder, turns []crewTurn) {
	for _, x := range turns {
		t, st := x.t, x.start
		s.Tick(&st)
		s.Agent(&t)
		if x.credits != 0 {
			s.Credits = append(s.Credits, core.Credit{T: &t, V: x.credits})
		}
		s.Measure("credits", &t, x.credits)
		s.Measure("turns", &t, 1)
		if x.model != "" {
			s.Model(x.model)
		}
	}
}

// crewOnly は kiro-cli の会話に結びつかない Crew の記録を、Crew のセッションにする。
// 裏方の処理（_bg）は 1 日ごとにまとめる。
func crewOnly(home, arch, slot string, turns []crewTurn, info *CrewInfo, errs *fileErrs) []*core.Builder {
	var title string
	var rows []crewRow
	if slot != "_bg" && home != "" {
		title, rows = readCrewKey(home, arch, slot, errs)
	}
	groups := map[string][]crewTurn{}
	var order []string
	for _, x := range turns {
		g := ""
		if slot == "_bg" {
			g = time.Unix(int64(x.t), 0).In(time.Local).Format("2006-01-02")
		}
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], x)
	}
	var out []*core.Builder
	for _, g := range order {
		id := "crew:" + slot
		if g != "" {
			id += ":" + g
		}
		s := core.NewBuilder("Kiro Crew", id)
		if slot != "_bg" && home != "" && len(rows) > 0 {
			s.File = crewTranscriptPath(home, slot)
		}
		s.Key = "kiro-crew:" + id[len("crew:"):]
		switch {
		case slot == "_bg":
			s.Title = "Kiro Crew background work"
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
		addCrewTurns(s, groups[g])
		addCrewRows(s, rows)
		first := groups[g][0].start
		s.Measure("crew_sessions", &first, 1)
		out = append(out, s)
	}
	return out
}

// crewRetentionDefault は Crew の session.archive_retention_days の既定値（kiro_crew/config/sections.py の SessionConfig）。
const crewRetentionDefault = 30

// crewRetentionNever は、これより長い日数を「消さない」と同じに扱う境目（約 2700 年。int にしても溢れない）。
const crewRetentionNever = 1_000_000

// crewConfigMax は Crew の設定ファイルとして読む大きさの上限。これより大きいファイルは読まない。
const crewConfigMax = 8 << 20

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
	base := crewConfig(filepath.Join(home, "config.json"))
	local := crewConfig(filepath.Join(home, "config.local.json"))
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

// crewConfig は Crew の設定ファイル（JSON の object）を読む。ない・ふつうのファイルでない・大きすぎる・読めない・壊れている・
// object でないなら nil（Crew もそのファイルを無視して既定値で動く）。手で編集したファイルの先頭の BOM は、Crew と同じく読み飛ばす。
func crewConfig(path string) map[string]any {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() > crewConfigMax {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, crewConfigMax+1))
	if err != nil || len(b) > crewConfigMax {
		return nil
	}
	var v any
	if json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &v) != nil {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}
