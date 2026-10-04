package source

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

// LoadCrew は kiro-cli の会話 ID → Crew の情報。Crew がなければ空。
func LoadCrew(home string) map[string]CrewInfo {
	out := map[string]CrewInfo{}
	if home == "" || !isDir(home) {
		return out
	}
	for key, v := range core.Map(core.ReadJSON(filepath.Join(home, "session_map.json"))) {
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
		core.ReadJSONL(path, func(e core.Obj) {
			if info.Title == "" && core.Str(e["_type"]) == "metadata" {
				info.Title = core.Str(e["title"])
				info.Agent = core.Str(e["agent"])
			}
		})
		out[sid] = info
	}
	for _, p := range glob(filepath.Join(home, "subagents", "*", "state.json")) {
		st := core.Map(core.ReadJSON(p))
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
// （既定で 7 日残す）ので、残っていればそちらも古い順に読む。
func readCrewKey(home, key string) (title string, rows []crewRow) {
	stem := unsafeKey.ReplaceAllString(key, "_")
	segs := glob(filepath.Join(home, "sessions", "archive", stem+"__*.jsonl"))
	sort.Strings(segs)
	for _, p := range append(segs, crewTranscriptPath(home, key)) {
		t, rs := readCrewTranscript(p)
		title = firstNonEmpty(title, t)
		rows = append(rows, rs...)
	}
	return title, rows
}

// readCrewTranscript は Crew の会話の記録を読む。title は 1 行目のメタデータのタイトル。
func readCrewTranscript(path string) (title string, rows []crewRow) {
	core.ReadJSONL(path, func(e core.Obj) {
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
	})
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
		title := "サブエージェント"
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
func loadCrewUsage(home string) map[string][]crewTurn {
	out := map[string][]crewTurn{}
	if home == "" {
		return out
	}
	for _, p := range glob(filepath.Join(home, "usage", "tokens", "*.jsonl")) {
		core.ReadJSONL(p, func(e core.Obj) {
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
		})
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
func crewOnly(home, slot string, turns []crewTurn, info *CrewInfo) []*core.Builder {
	var title string
	var rows []crewRow
	if slot != "_bg" && home != "" {
		title, rows = readCrewKey(home, slot)
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
			s.Title = "Kiro Crew の裏方の処理"
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
