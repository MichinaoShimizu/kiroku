package source

import (
	"os"
	"path/filepath"
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
		if strings.HasPrefix(unit, "credit") {
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
			case "usage_summary":
				if used := creditsOf(p["promptTurnSummaries"], "unit", "usage"); used != 0 {
					s.Credits = append(s.Credits, core.Credit{T: t, V: used})
				}
				s.Model(model)
			}
		})
		emit(s)
	}
	return nil
}

// KiroCLI は Kiro CLI: <KIRO_HOME>/sessions/cli/<id>.json（メタ）+ <id>.jsonl（Prompt/AssistantMessage）。
type KiroCLI struct{ Home string }

func (k *KiroCLI) Name() string   { return "Kiro CLI" }
func (k *KiroCLI) Family() string { return "kiro" }
func (k *KiroCLI) Where() string  { return filepath.Join(k.Home, "sessions", "cli") }

func (k *KiroCLI) Load(emit func(*core.Builder)) error {
	base := k.Where()
	if !isDir(base) {
		return nil
	}
	for _, metaPath := range glob(filepath.Join(base, "*.json")) {
		meta := core.Map(core.ReadJSON(metaPath))
		stem := strings.TrimSuffix(filepath.Base(metaPath), ".json")
		sid := firstNonEmpty(core.Str(meta["session_id"]), core.Str(meta["id"]), stem)
		s := core.NewBuilder("Kiro CLI", sid)
		s.Key = "kiro-cli:" + sid
		s.Title, s.Project = core.Str(meta["title"]), core.Str(meta["cwd"])
		s.Tick(ts(meta["created_at"]))
		s.Tick(ts(meta["updated_at"]))
		defaultModel := core.Str(core.Get(meta, "session_state", "rts_model_state", "model_info", "model_id"))
		for _, t := range core.List(core.Get(meta, "session_state", "conversation_metadata", "user_turn_metadatas")) {
			tm := core.Map(t)
			te := ts(tm["end_timestamp"])
			s.Agent(te)
			if used := creditsOf(tm["metering_usage"], "unit", "value"); used != 0 {
				s.Credits = append(s.Credits, core.Credit{T: te, V: used})
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
		emit(s)
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
