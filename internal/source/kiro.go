package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// KiroIDE は Kiro IDE v1.0 以降: <KIRO_HOME>/sessions/<hash>/sess_<id>/{session.json,messages.jsonl}（行ごとに timestamp あり）。
type KiroIDE struct{ Home string }

func (k *KiroIDE) Name() string   { return "Kiro IDE" }
func (k *KiroIDE) Family() string { return "kiro" }
func (k *KiroIDE) Where() string  { return filepath.Join(k.Home, "sessions") }

func creditsOf(list any, unitKey, valueKey string) float64 {
	used := 0.0
	for _, x := range core.List(list) {
		m := core.Map(x)
		unit, ok := m[unitKey].(string)
		if !ok {
			unit = "credit"
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(unit)), "credit") { // "credit"・"Credits" など、表記の揺れを許す
			used += core.NumOr0(m[valueKey])
		}
	}
	return used
}

func (k *KiroIDE) Load(emit func(*core.Builder)) error {
	base := filepath.Join(k.Home, "sessions")
	if !isDir(base) {
		return nil
	}
	for _, metaPath := range glob(filepath.Join(base, "*", "*", "session.json")) {
		dir := filepath.Dir(metaPath)
		if filepath.Base(filepath.Dir(dir)) == "cli" {
			continue
		}
		meta := core.Map(core.ReadJSON(metaPath))
		s := core.NewBuilder("Kiro IDE", firstNonEmpty(core.Str(meta["id"]), filepath.Base(dir)))
		s.File = filepath.Join(dir, "messages.jsonl")
		s.Title = core.Str(meta["title"])
		for _, key := range []string{"workspacePaths", "rootPaths"} {
			if l := core.List(meta[key]); len(l) > 0 {
				s.Project = core.Str(l[0])
				break
			}
		}
		model := core.Str(meta["modelId"])
		s.Tick(ts(meta["createdAt"]))
		core.ReadJSONL(filepath.Join(dir, "messages.jsonl"), func(e core.Obj) {
			t := ts(e["timestamp"])
			p := core.Map(e["payload"])
			typ := core.Str(p["type"])
			if typ == "user" {
				s.Tick(t)
				s.Prompt(t, core.TextOf(p["content"]))
				return
			}
			s.Agent(t)
			switch typ {
			case "tool_call":
				s.Tool(core.Str(p["toolName"]), p["args"])
				s.Measure("tool_calls", t, 1)
			case "usage_summary":
				used := creditsOf(p["promptTurnSummaries"], "unit", "usage")
				if used != 0 {
					s.Credits = append(s.Credits, core.Credit{T: t, V: used})
				}
				s.Measure("credits", t, used)
				s.Measure("turns", t, 1)
				s.Model(model)
			}
		})
		emit(s)
	}
	return nil
}

// KiroCLI は Kiro CLI: <KIRO_HOME>/sessions/cli/<id>.json（メタ）+ <id>.jsonl（Prompt/AssistantMessage）。
// Kiro Crew から動かした会話には、Crew の目印とタイトルをつける（crew.go）。
type KiroCLI struct {
	Home      string
	CrewHome  string
	crew      int     // Crew から動かした kiro-cli の会話
	crewFixed int     // Crew の使用量の記録でクレジットを補った会話
	crewOnly  int     // kiro-cli の会話に結びつかない Crew の記録
	crewCr    float64 // そのクレジット
	crewText  int     // Crew の会話の記録だけにある会話
}

// Detail は計測の状態に添える一言。
func (k *KiroCLI) Detail() string {
	var parts []string
	if k.crew > 0 {
		parts = append(parts, fmt.Sprintf("うち Kiro Crew から %d 件", k.crew))
	}
	if k.crewFixed > 0 {
		parts = append(parts, fmt.Sprintf("Crew の使用量の記録でクレジットを補った会話 %d 件", k.crewFixed))
	}
	if k.crewText > 0 {
		parts = append(parts, fmt.Sprintf("Crew の会話の記録だけにある %d 件", k.crewText))
	}
	if k.crewOnly > 0 {
		parts = append(parts, fmt.Sprintf("Crew の使用量の記録だけにある %d 件（%.2f クレジット）", k.crewOnly, k.crewCr))
	}
	return strings.Join(parts, "・")
}

// DetailEn は Detail の英語版（英語表示の画面に出す）。
func (k *KiroCLI) DetailEn() string {
	var parts []string
	if k.crew > 0 {
		parts = append(parts, fmt.Sprintf("%d from Kiro Crew", k.crew))
	}
	if k.crewFixed > 0 {
		parts = append(parts, fmt.Sprintf("%d with credits filled in from Crew usage records", k.crewFixed))
	}
	if k.crewText > 0 {
		parts = append(parts, fmt.Sprintf("%d found only in Crew transcripts", k.crewText))
	}
	if k.crewOnly > 0 {
		parts = append(parts, fmt.Sprintf("%d found only in Crew usage records (%.2f credits)", k.crewOnly, k.crewCr))
	}
	return strings.Join(parts, "; ")
}

func (k *KiroCLI) Name() string   { return "Kiro CLI" }
func (k *KiroCLI) Family() string { return "kiro" }
func (k *KiroCLI) Where() string  { return filepath.Join(k.Home, "sessions", "cli") }

func (k *KiroCLI) Load(emit func(*core.Builder)) error {
	base := k.Where()
	if !isDir(base) {
		return nil
	}
	crew := LoadCrew(k.CrewHome)
	usage := loadCrewUsage(k.CrewHome)
	k.crew, k.crewFixed, k.crewOnly, k.crewCr, k.crewText = 0, 0, 0, 0, 0
	slotInfo := map[string]*CrewInfo{}
	for _, info := range crew {
		if !info.Subagent {
			i := info
			slotInfo[info.Key] = &i
		}
	}
	used := map[string]bool{}     // 使用量の記録を kiro-cli の会話に結びつけた会話キー
	seenRows := map[string]bool{} // 会話の記録を使った（kiro-cli の会話に結びついた）会話キー
	for _, metaPath := range glob(filepath.Join(base, "*.json")) {
		meta := core.Map(core.ReadJSON(metaPath))
		stem := strings.TrimSuffix(filepath.Base(metaPath), ".json")
		sid := firstNonEmpty(core.Str(meta["session_id"]), core.Str(meta["id"]), stem)
		s := core.NewBuilder("Kiro CLI", sid)
		s.File = strings.TrimSuffix(metaPath, ".json") + ".jsonl"
		s.Key = "kiro-cli:" + sid
		s.Title, s.Project = core.Str(meta["title"]), core.Str(meta["cwd"])
		s.Tick(ts(meta["created_at"]))
		s.Tick(ts(meta["updated_at"]))
		defaultModel := core.Str(core.Get(meta, "session_state", "rts_model_state", "model_info", "model_id"))
		for _, t := range core.List(core.Get(meta, "session_state", "conversation_metadata", "user_turn_metadatas")) {
			tm := core.Map(t)
			te := ts(tm["end_timestamp"])
			s.Agent(te)
			used := creditsOf(tm["metering_usage"], "unit", "value")
			if used != 0 {
				s.Credits = append(s.Credits, core.Credit{T: te, V: used})
			}
			s.Measure("credits", te, used)
			s.Measure("turns", te, 1)
			if v, ok := core.Num(tm["total_request_count"]); ok {
				s.Measure("requests", te, v)
			}
			if v, ok := core.Num(tm["builtin_tool_uses"]); ok {
				s.Measure("builtin_tools", te, v)
			}
			s.Model(firstNonEmpty(core.Str(tm["model"]), defaultModel))
		}
		core.ReadJSONL(strings.TrimSuffix(metaPath, ".json")+".jsonl", func(e core.Obj) {
			data := core.Map(e["data"])
			raw := e["timestamp"]
			if raw == nil || raw == "" {
				raw = core.Get(data, "meta", "timestamp")
			}
			t := ts(raw)
			switch core.Str(e["kind"]) {
			case "Prompt":
				s.Tick(t)
				s.Prompt(t, core.TextOf(data["content"]))
			case "AssistantMessage":
				s.Agent(t)
				for _, c := range core.List(data["content"]) {
					cm := core.Map(c)
					if core.Str(cm["kind"]) == "toolUse" {
						d := core.Map(cm["data"])
						s.Tool(core.Str(d["name"]), d["input"])
					}
				}
			default:
				s.Agent(t)
			}
		})
		if s.Project != "" {
			s.Resume = "cd " + s.Project + " && kiro-cli chat --resume-id " + sid
		}
		if tagCrew(s, crew) {
			k.crew++
			if info := crew[s.ID]; !info.Subagent {
				seenRows[info.Key] = true
				if len(s.Prompts) == 0 { // Crew から動かした会話は、kiro-cli の履歴に依頼が残らないことがある
					_, rows := readCrewKey(k.CrewHome, info.Key)
					addCrewRows(s, rows)
				}
				if len(usage[info.Key]) > 0 {
					used[info.Key] = true
					if useCrewCredits(s, usage[info.Key]) {
						k.crewFixed++
					}
				}
			}
		}
		emit(s)
	}
	// kiro-cli の会話に結びつかない Crew の記録
	slots := make([]string, 0, len(usage))
	for slot := range usage {
		if !used[slot] {
			slots = append(slots, slot)
		}
	}
	sort.Strings(slots)
	for _, slot := range slots {
		seenRows[slot] = true
		for _, s := range crewOnly(k.CrewHome, slot, usage[slot], slotInfo[slot]) {
			k.crewOnly++
			k.crewCr += sumCredits(s.Credits)
			emit(s)
		}
	}
	// 会話の記録だけがある Crew の会話（使用量の記録も kiro-cli の会話もないもの）
	if k.CrewHome != "" {
		seenFile := map[string]bool{}
		for key := range seenRows {
			seenFile[crewTranscriptPath(k.CrewHome, key)] = true
		}
		byFile := map[string]*CrewInfo{}
		for key, info := range slotInfo {
			byFile[crewTranscriptPath(k.CrewHome, key)] = info
		}
		for _, p := range glob(filepath.Join(k.CrewHome, "sessions", "*.jsonl")) {
			if seenFile[p] {
				continue
			}
			title, rows := readCrewTranscript(p)
			if len(rows) == 0 {
				continue
			}
			stem := strings.TrimSuffix(filepath.Base(p), ".jsonl")
			s := core.NewBuilder("Kiro Crew", "crew:"+stem)
			s.File = p
			s.Key = "kiro-crew:" + stem
			s.Title, s.Project = firstNonEmpty(title, "Kiro Crew: "+stem), "(Kiro Crew)"
			if info := byFile[p]; info != nil && info.Cwd != "" {
				s.Project = info.Cwd
			}
			addCrewRows(s, rows)
			var first *float64
			for i := range s.Times {
				if first == nil || s.Times[i] < *first {
					first = &s.Times[i]
				}
			}
			s.Measure("crew_sessions", first, 1)
			k.crewText++
			emit(s)
		}
	}
	return nil
}

// KiroIDELegacy は Kiro IDE v1.0 より前: workspace-sessions/<ws>/sessions.json + <sessionId>.json。
// 発言ごとの時刻がないので、開始 = dateCreated、終了 = ファイル更新時刻 のざっくり表示。
type KiroIDELegacy struct{ Storages []string }

func (k *KiroIDELegacy) Name() string   { return "Kiro IDE (旧)" }
func (k *KiroIDELegacy) Family() string { return "kiro" }
func (k *KiroIDELegacy) Where() string {
	if len(k.Storages) == 0 {
		return "なし"
	}
	return strings.Join(k.Storages, " / ")
}

func (k *KiroIDELegacy) Load(emit func(*core.Builder)) error {
	for _, gs := range k.Storages {
		for _, index := range glob(filepath.Join(gs, "workspace-sessions", "*", "sessions.json")) {
			for _, ent := range core.List(core.ReadJSON(index)) {
				em := core.Map(ent)
				hidden, _ := em["hidden"].(bool)
				id := core.Str(em["sessionId"])
				if em == nil || hidden || id == "" {
					continue
				}
				f := filepath.Join(filepath.Dir(index), id+".json")
				data := core.Map(core.ReadJSON(f))
				s := core.NewBuilder("Kiro IDE (旧)", id)
				s.File = f
				s.Title = firstNonEmpty(core.Str(em["title"]), core.Str(data["title"]))
				s.Project = firstNonEmpty(core.Str(data["workspacePath"]), core.Str(data["workspaceDirectory"]), core.Str(em["workspaceDirectory"]))
				start := ts(em["dateCreated"])
				s.Tick(start)
				for _, h := range core.List(data["history"]) {
					m := core.Map(core.Map(h)["message"])
					if core.Str(m["role"]) == "user" {
						s.Prompt(start, core.TextOf(m["content"]))
					}
				}
				if st, err := os.Stat(f); err == nil {
					v := float64(st.ModTime().UnixNano()) / 1e9
					s.Tick(&v)
				}
				emit(s)
			}
		}
	}
	return nil
}
