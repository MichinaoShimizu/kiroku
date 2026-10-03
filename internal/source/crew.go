package source

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Kiro Crew（github.com/kirodotdev/KiroCrew）は kiro-cli を ACP で動かすので、会話そのものは
// ~/.kiro/sessions/cli に kiro-cli の履歴として残る。そちらにクレジットも時刻もあるので、
// Crew 側のデータは「Crew から動かした会話」という目印とタイトルにだけ使う（二重に数えない）。
//
//	<KIROCREW_HOME か ~/.kiro/crew>/session_map.json        {会話キー: {"sid": kiro-cli の会話 ID, "cwd": …} か "sid"}
//	<…>/sessions/<キー>.jsonl                               1 行目 {"_type": "metadata", "title", "agent", "model", …}
//	<…>/subagents/<id>/state.json                          {"task", "agent", "parent_session", "session_id"?, …}

// CrewInfo は 1 つの kiro-cli の会話についての Crew の情報。
type CrewInfo struct {
	Subagent bool
	Key      string // Crew の会話キー（サブエージェントなら親の会話キー）
	Title    string
	Agent    string
	Task     string
}

// DefaultCrewHome は KIROCREW_HOME か <KIRO_HOME か ~/.kiro>/crew。
func DefaultCrewHome(kiroHome string) string {
	if d := os.Getenv("KIROCREW_HOME"); d != "" {
		return d
	}
	return filepath.Join(kiroHome, "crew")
}

var unsafeKey = regexp.MustCompile(`[^\w\-.]`)

// LoadCrew は kiro-cli の会話 ID → Crew の情報。Crew がなければ空。
func LoadCrew(home string) map[string]CrewInfo {
	out := map[string]CrewInfo{}
	if home == "" || !isDir(home) {
		return out
	}
	for key, v := range core.Map(core.ReadJSON(filepath.Join(home, "session_map.json"))) {
		sid := core.Str(v) // 古い形は文字列だけ
		if m := core.Map(v); m != nil {
			sid = core.Str(m["sid"])
		}
		if sid == "" {
			continue
		}
		info := CrewInfo{Key: key}
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

// tagCrew は Crew から動かした kiro-cli の会話に目印とタイトルをつける。つけたら true。
func tagCrew(s *core.Builder, crew map[string]CrewInfo) bool {
	info, ok := crew[s.ID]
	if !ok {
		return false
	}
	s.Source = "Kiro Crew"
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
