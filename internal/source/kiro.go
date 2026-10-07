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

// Load は全部の会話を読む。読めないファイルがあっても読めた分は出し、最初のエラー（と残りの数）を返す。
func (k *KiroIDE) Load(emit func(*core.Builder)) error {
	base := filepath.Join(k.Home, "sessions")
	if !isDir(base) {
		return nil
	}
	var errs fileErrs
	for _, metaPath := range glob(filepath.Join(base, "*", "*", "session.json")) {
		dir := filepath.Dir(metaPath)
		if filepath.Base(filepath.Dir(dir)) == "cli" {
			continue
		}
		raw, err := core.ReadJSONFile(metaPath)
		errs.file(metaPath, err)
		meta := core.Map(raw)
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
		errs.file(s.File, core.ReadJSONL(s.File, func(e core.Obj) {
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
			case "assistant": // 人に返した文（依頼の流れに出す）。考えている途中の文（operationType: Reasoning）は入れない
				if core.Str(p["operationType"]) != "Reasoning" {
					s.Reply(t, "", replyText(p["content"]))
				}
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
		}))
		emit(s)
	}
	return errs.err()
}

// replyText は Kiro IDE の応答の文。ふつうは文字列か text の塊の並び（core.TextOf）だが、
// それで拾えない形（{"content": …} や {"text": …} の入れ子）も、中の文字をたどって拾う。
func replyText(v any) string {
	if t := core.TextOf(v); t != "" {
		return t
	}
	var walk func(any, int) string
	walk = func(v any, depth int) string {
		if depth > 4 {
			return ""
		}
		switch x := v.(type) {
		case string:
			return x
		case []any:
			var parts []string
			for _, y := range x {
				if t := walk(y, depth+1); t != "" {
					parts = append(parts, t)
				}
			}
			return strings.Join(parts, "\n")
		case map[string]any:
			for _, k := range []string{"text", "content", "message", "value", "parts"} {
				if t := walk(x[k], depth+1); t != "" {
					return t
				}
			}
		}
		return ""
	}
	return walk(v, 0)
}

// replyAt は Kiro CLI の応答の時刻。行に時刻があればそれ、なければその回（n 番目の依頼）の終わり、
// それもなければ直前の依頼の時刻。依頼より前になる時刻は使わない（ほかの依頼の応答になってしまうため）。
func replyAt(t *float64, turnEnds []*float64, n int, asked *float64) *float64 {
	if t != nil {
		return t
	}
	if n >= 1 && n <= len(turnEnds) && turnEnds[n-1] != nil && (asked == nil || *turnEnds[n-1] >= *asked) {
		return turnEnds[n-1]
	}
	return asked
}

// KiroCLI は Kiro CLI: <KIRO_HOME>/sessions/cli/<id>.json（メタ）+ <id>.jsonl（Prompt/AssistantMessage）。
// Kiro Crew から動かした会話には、Crew の目印とタイトルをつける（crew.go）。
type KiroCLI struct {
	Home        string
	CrewHome    string
	CrewArchive string  // kiroku archive の Crew のコピーの場所（<保存場所>/crew）
	crew        int     // Crew から動かした kiro-cli の会話
	crewFixed   int     // Crew の使用量の記録でクレジットを補った会話
	crewOnly    int     // kiro-cli の会話に結びつかない Crew の記録
	crewCr      float64 // そのクレジット
	crewText    int     // Crew の会話の記録だけにある会話
}

// Keep は、kiroku archive で残す場所（Kiro Crew は退避した古い会話の記録を消すため）。
func (k *KiroCLI) Keep() []Kept {
	if k.CrewArchive == "" || k.CrewHome == "" {
		return nil
	}
	rel := filepath.Join("sessions", "archive")
	return []Kept{{Src: filepath.Join(k.CrewHome, rel), Dst: filepath.Join(k.CrewArchive, rel)}}
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

func (k *KiroCLI) Name() string { return "Kiro CLI" }

// Retention は Kiro Crew の会話の記録の保存期間（session.archive_retention_days）。Crew を使っているときだけ。
// 日数は Crew の版や設定で変わり、kiroku からは読めないので 0（わからない）にする。
func (k *KiroCLI) Retention() *Retention {
	if k.CrewHome == "" || !isDir(k.CrewHome) {
		return nil
	}
	return &Retention{Setting: "session.archive_retention_days", Docs: "https://kiro.dev/docs/crew/configuration/"}
}
func (k *KiroCLI) Family() string { return "kiro" }
func (k *KiroCLI) Where() string  { return filepath.Join(k.Home, "sessions", "cli") }

// Load は全部の会話を読む（Crew の記録も）。読めないファイルがあっても読めた分は出し、最初のエラー（と残りの数）を返す。
func (k *KiroCLI) Load(emit func(*core.Builder)) error {
	base := k.Where()
	if !isDir(base) {
		return nil
	}
	var errs fileErrs
	crew := loadCrew(k.CrewHome, &errs)
	usage := loadCrewUsage(k.CrewHome, &errs)
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
		raw, err := core.ReadJSONFile(metaPath)
		errs.file(metaPath, err)
		meta := core.Map(raw)
		stem := strings.TrimSuffix(filepath.Base(metaPath), ".json")
		sid := firstNonEmpty(core.Str(meta["session_id"]), core.Str(meta["id"]), stem)
		s := core.NewBuilder("Kiro CLI", sid)
		s.File = strings.TrimSuffix(metaPath, ".json") + ".jsonl"
		s.Key = "kiro-cli:" + sid
		s.Title, s.Project = core.Str(meta["title"]), core.Str(meta["cwd"])
		s.Tick(ts(meta["created_at"]))
		s.Tick(ts(meta["updated_at"]))
		defaultModel := core.Str(core.Get(meta, "session_state", "rts_model_state", "model_info", "model_id"))
		var turnEnds []*float64 // 依頼ごとの、その回の終わりの時刻（応答の行には時刻がないので、応答の時刻に使う）
		for _, t := range core.List(core.Get(meta, "session_state", "conversation_metadata", "user_turn_metadatas")) {
			tm := core.Map(t)
			te := ts(tm["end_timestamp"])
			turnEnds = append(turnEnds, te)
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
		var asked *float64 // 直前の依頼の時刻
		nPrompt := 0
		errs.file(s.File, core.ReadJSONL(s.File, func(e core.Obj) {
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
				asked = t
				nPrompt++
			case "AssistantMessage":
				s.Agent(t)
				// 人に返した文（content の kind: text）。kiro-cli は応答の行に時刻を残さないので、
				// その回の終わりの時刻（user_turn_metadatas の end_timestamp）、なければ直前の依頼の時刻を使う。
				// 時刻がないと、どの依頼への応答か決められず、応答が出ない
				s.Reply(replyAt(t, turnEnds, nPrompt, asked), "", core.TextOf(data["content"]))
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
		}))
		if s.Project != "" {
			s.Resume = "cd " + s.Project + " && kiro-cli chat --resume-id " + sid
		}
		if tagCrew(s, crew) {
			k.crew++
			if info := crew[s.ID]; !info.Subagent {
				seenRows[info.Key] = true
				if len(s.Prompts) == 0 { // Crew から動かした会話は、kiro-cli の履歴に依頼が残らないことがある
					_, rows := readCrewKey(k.CrewHome, k.CrewArchive, info.Key, &errs)
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
		for _, s := range crewOnly(k.CrewHome, k.CrewArchive, slot, usage[slot], slotInfo[slot], &errs) {
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
			title, rows := readCrewTranscript(p, &errs)
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
	return errs.err()
}

// KiroIDELegacy は Kiro IDE v1.0 より前: workspace-sessions/<ws>/sessions.json + <sessionId>.json。
// 発言ごとの時刻がないので、開始 = dateCreated、終了 = ファイル更新時刻 のざっくり表示。
type KiroIDELegacy struct{ Storages []string }

func (k *KiroIDELegacy) Name() string   { return "Kiro IDE (legacy)" }
func (k *KiroIDELegacy) Family() string { return "kiro" }
func (k *KiroIDELegacy) Where() string {
	if len(k.Storages) == 0 {
		return "none"
	}
	return strings.Join(k.Storages, " / ")
}

func (k *KiroIDELegacy) Load(emit func(*core.Builder)) error {
	var errs fileErrs
	for _, gs := range k.Storages {
		for _, index := range glob(filepath.Join(gs, "workspace-sessions", "*", "sessions.json")) {
			list, err := core.ReadJSONFile(index)
			errs.file(index, err)
			for _, ent := range core.List(list) {
				em := core.Map(ent)
				hidden, _ := em["hidden"].(bool)
				id := core.Str(em["sessionId"])
				if em == nil || hidden || id == "" {
					continue
				}
				if !safeName(id) { // sessions.json の中身でほかの場所のファイルを読ませない（../../x など）
					errs.file(index, fmt.Errorf("unsafe sessionId %q", id))
					continue
				}
				f := filepath.Join(filepath.Dir(index), id+".json")
				raw, err := core.ReadJSONFile(f)
				errs.file(f, err)
				data := core.Map(raw)
				s := core.NewBuilder("Kiro IDE (legacy)", id)
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
	return errs.err()
}
