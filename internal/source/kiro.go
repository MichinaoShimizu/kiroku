package source

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
				// requestIds（そのターンでモデルへ送ったリクエストの ID の並び）。参考実装 kiro-history（server/ide-v1.ts）だけが読む項目。
				// 並びでなければ（古い版などで項目がなければ）数えない
				if ids, ok := p["requestIds"].([]any); ok {
					s.Measure("requests", t, float64(len(ids)))
				}
				s.Model(model)
			case "session_metadata":
				// {key: "contextUsage", value: {usagePercentage: 0〜100}}。参考実装 codeburn のテストデータ（tests/providers/kiro.test.ts）だけにある形。
				// 0〜100 の数でなければ使わない
				if core.Str(p["key"]) == "contextUsage" {
					if v, ok := core.Num(core.Map(p["value"])["usagePercentage"]); ok && v >= 0 && v <= 100 {
						s.Measure("context_used", t, v/100)
					}
				}
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
	crewElse    int     // ほかの履歴（Claude Code・Codex）が記録しているので、トークンとドル額を足さなかった Crew の行
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
	if k.crewElse > 0 {
		parts = append(parts, fmt.Sprintf("Claude Code・Codex で動かした %d ターンのトークンとコストは、それぞれの履歴で数える", k.crewElse))
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
	if k.crewElse > 0 {
		parts = append(parts, fmt.Sprintf("tokens and cost of %d turns run on Claude Code or Codex are counted from their own history", k.crewElse))
	}
	return strings.Join(parts, "; ")
}

func (k *KiroCLI) Name() string { return "Kiro CLI" }

// Retention は Kiro Crew の会話の記録の保存期間（session.archive_retention_days。既定 30 日）。Crew を使っているときだけ。
// Crew は 1 時間に 1 回、退避した会話の記録（sessions/archive/）のうち更新時刻がそれより古いものを消す（閉じた会話の crew log も同じ設定で消す）。
// 消さない設定（null か負の数）なら nil。Crew は config.json に全部のキーを書き出すので、キーがあっても利用者が決めたとは限らない。
// そこで、既定の 30 日と違う値のときだけ設定済み（Set）にする。設定を足す先は、config.json より勝つ config.local.json を案内する。
// 0 日なら、Crew は次の片付け（1 時間に 1 回まで）で退避した記録を全部消す（history.py の _cleanup_old_archives は cutoff = 今）。
// Days の 0 は「わからない」なので、Now で 0 日を表す。
func (k *KiroCLI) Retention() *Retention {
	if k.CrewHome == "" || !isDir(k.CrewHome) {
		return nil
	}
	days, never := crewRetentionDays(k.CrewHome)
	if never {
		return nil
	}
	return &Retention{Days: days, Now: days == 0, Set: days != crewRetentionDefault, Who: "Kiro Crew", Setting: "session.archive_retention_days",
		Snippet: `"session": {"archive_retention_days": 3650}`,
		Docs:    "https://github.com/kirodotdev/kirocrew/blob/main/src/kiro_crew/docs/configuration.md",
		File:    filepath.Join(k.CrewHome, "config.local.json")}
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
	k.crew, k.crewFixed, k.crewOnly, k.crewCr, k.crewText, k.crewElse = 0, 0, 0, 0, 0, 0
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
			s.Resume = core.ResumeCmd(s.Project, "kiro-cli chat --resume-id", sid)
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
					addCrewNative(s, usage[info.Key])
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
		ss, skipped := crewOnly(k.CrewHome, k.CrewArchive, slot, usage[slot], slotInfo[slot], &errs)
		k.crewElse += skipped
		for _, s := range ss {
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
			meta, rows := readCrewTranscript(p, &errs)
			title, rows := meta.title, ownRows(meta, rows) // fork なら、元の会話から写した行は除く
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
// 実行ファイル（kiroExec）があれば、そこからクレジット・モデル・実行の開始と終了の時刻を足す。
type KiroIDELegacy struct{ Storages []string }

func (k *KiroIDELegacy) Name() string   { return "Kiro IDE (legacy)" }
func (k *KiroIDELegacy) Family() string { return "kiro" }
func (k *KiroIDELegacy) Where() string {
	if len(k.Storages) == 0 {
		return "none"
	}
	return strings.Join(k.Storages, " / ")
}

// kiroExec は Kiro IDE（v1.0 より前）の実行ファイル 1 つ（1 回の依頼の実行）のうち、kiroku が使うもの。
// 公式の形式の文書はなく、参考実装 codeburn（src/providers/kiro.ts の parseModernExecution と、その tests/providers/kiro.test.ts）に合わせる:
//
//	<globalStorage>/kiro.kiroagent/<32 桁の 16 進（ワークスペース）>/<セッション>/<実行 ID（拡張子なし）>
//	{"executionId", "chatSessionId" か "sessionId", "startTime", "endTime"（ミリ秒）, "modelId" か "metadata": {"modelId", "startTime", "endTime"},
//	 "usageSummary": [{"usage": クレジット, "unit": "credit", "usedTools": […]}], "context": {"messages": […]}, …}
//
// 同じ場所にある {"executions": […]} だけのファイルは実行の一覧なので読まない。workspace-sessions の会話とは、
// 会話の history[].executionId か、実行ファイルの chatSessionId・sessionId（会話の sessionId と同じもの）で結ぶ。
type kiroExec struct {
	id, session string
	start, end  *float64
	credits     float64
	model       string
}

// kiroWorkspaceDir は実行ファイルを置くワークスペースのフォルダ名（32 桁の 16 進）。
var kiroWorkspaceDir = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// kiroExecMax は実行ファイルとして読む大きさの上限。会話の全文が入るので大きめにする。これより大きいファイルは読まない。
const kiroExecMax = 64 << 20

// kiroExecKeys は実行ファイルのうち kiroku が読む項目（topFields で取り出す）。
var kiroExecKeys = []string{"executions", "executionId", "chatSessionId", "sessionId", "modelId", "startTime", "endTime", "metadata", "usageSummary"}

// realDir は、シンボリックリンクでないふつうのフォルダか。
func realDir(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.IsDir()
}

// loadKiroExecs は globalStorage の下の実行ファイルを読む。実行 ID ごとと、会話の ID ごと。
// 読むのは <32 桁の 16 進>/<フォルダ>/<拡張子のない名前> だけ。フォルダとファイルは、シンボリックリンクでない（ほかの場所を読まない）
// ふつうのものに限る。パスは履歴の中身からは作らない（中身の ID は結びつけるための鍵にしか使わない）。
// JSON でないファイルや、実行ファイルの形でないものは黙って飛ばす（同じ場所に別のファイルがあることがあるため）。
func loadKiroExecs(gs string, errs *fileErrs) (byID map[string]*kiroExec, bySession map[string][]*kiroExec) {
	byID, bySession = map[string]*kiroExec{}, map[string][]*kiroExec{}
	for _, p := range glob(filepath.Join(gs, "*", "*", "*")) {
		sess := filepath.Dir(p)
		ws := filepath.Dir(sess)
		name := filepath.Base(p)
		if !kiroWorkspaceDir.MatchString(filepath.Base(ws)) || !safeName(name) || strings.HasPrefix(name, ".") || filepath.Ext(name) != "" ||
			strings.HasPrefix(filepath.Base(sess), ".") || !realDir(ws) || !realDir(sess) {
			continue
		}
		st, err := os.Lstat(p)
		if err != nil || !st.Mode().IsRegular() || st.Size() > kiroExecMax {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			errs.file(p, err)
			continue
		}
		// 実行ファイルには会話の全文（context）が入っていて大きい。全体を組み立てると読み込みがとても遅くなるので、
		// 使う項目だけを取り出す
		data, ok := topFields(b, kiroExecKeys)
		if !ok || data["executions"] != nil {
			continue
		}
		meta := core.Map(data["metadata"])
		x := &kiroExec{
			id:      firstNonEmpty(core.Str(data["executionId"]), name),
			session: firstNonEmpty(core.Str(data["chatSessionId"]), core.Str(data["sessionId"])),
			model:   firstNonEmpty(core.Str(data["modelId"]), core.Str(meta["modelId"])),
			start:   firstTS(data["startTime"], meta["startTime"]),
			end:     firstTS(data["endTime"], meta["endTime"]),
			credits: creditsOf(data["usageSummary"], "unit", "usage"),
		}
		if x.end != nil && x.start != nil && *x.end < *x.start {
			x.end = nil // 終わりが始まりより前の記録は使わない
		}
		if x.start == nil && x.end == nil && x.credits == 0 && x.model == "" {
			continue
		}
		if _, dup := byID[x.id]; dup {
			continue
		}
		byID[x.id] = x
		if x.session != "" {
			bySession[x.session] = append(bySession[x.session], x)
		}
	}
	return byID, bySession
}

// firstTS は、時刻として読める最初の値。
func firstTS(vs ...any) *float64 {
	for _, v := range vs {
		if t := ts(v); t != nil {
			return t
		}
	}
	return nil
}

func (k *KiroIDELegacy) Load(emit func(*core.Builder)) error {
	var errs fileErrs
	for _, gs := range k.Storages {
		indexes := glob(filepath.Join(gs, "workspace-sessions", "*", "sessions.json"))
		if len(indexes) == 0 {
			continue
		}
		execByID, execBySession := loadKiroExecs(gs, &errs)
		for _, index := range indexes {
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
				// この会話の実行（history の executionId と、実行ファイルの会話 ID で結ぶ）
				var execs []*kiroExec
				linked := map[*kiroExec]bool{}
				link := func(x *kiroExec) {
					if x != nil && !linked[x] {
						linked[x] = true
						execs = append(execs, x)
					}
				}
				// 依頼の時刻: 依頼のあと（次の依頼より前）に出てくる最初の実行の開始時刻。わからなければ会話の作成時刻
				type ask struct {
					t    *float64
					text string
				}
				var asks []ask
				pending := -1 // まだ時刻の決まっていない依頼の始まり
				for _, h := range core.List(data["history"]) {
					hm := core.Map(h)
					m := core.Map(hm["message"])
					if core.Str(m["role"]) == "user" {
						if pending < 0 {
							pending = len(asks)
						}
						asks = append(asks, ask{text: core.TextOf(m["content"])})
					}
					if x := execByID[core.Str(hm["executionId"])]; x != nil {
						link(x)
						if x.start != nil && pending >= 0 {
							for i := pending; i < len(asks); i++ {
								asks[i].t = x.start
							}
							pending = -1
						}
					}
				}
				for _, a := range asks {
					t := a.t
					if t == nil {
						t = start
					}
					s.Prompt(t, a.text)
				}
				for _, x := range execBySession[id] {
					link(x)
				}
				sort.SliceStable(execs, func(i, j int) bool {
					a, b := firstNonNil(execs[i].start, execs[i].end), firstNonNil(execs[j].start, execs[j].end)
					return a != nil && (b == nil || *a < *b)
				})
				selected := core.Str(data["selectedModel"]) // 会話で選んでいたモデル（実行ファイルにモデルがなければこれ）
				for _, x := range execs {
					s.Tick(x.start)
					t := firstNonNil(x.end, x.start, start)
					s.Agent(firstNonNil(x.end, x.start))
					if x.credits != 0 {
						s.Credits = append(s.Credits, core.Credit{T: t, V: x.credits})
					}
					s.Measure("credits", t, x.credits)
					s.Measure("turns", t, 1)
					s.Model(firstNonEmpty(x.model, selected))
				}
				if len(execs) == 0 && len(asks) > 0 {
					s.Model(selected) // 実行ファイルがなければ、会話で選んでいたモデルを 1 回数える
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

// firstNonNil は nil でない最初の時刻。
func firstNonNil(ts ...*float64) *float64 {
	for _, t := range ts {
		if t != nil {
			return t
		}
	}
	return nil
}
