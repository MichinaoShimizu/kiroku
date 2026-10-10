package source

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

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
				// 中身が「...」だけの行は、書いている途中に置く仮の文（参考実装 kiro-history の server/ide-v1.ts も飛ばす）。応答にしない
				if text := replyText(p["content"]); core.Str(p["operationType"]) != "Reasoning" && strings.TrimSpace(text) != "..." {
					s.Reply(t, "", text)
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
	CrewArchive string   // kiroku archive の Crew のコピーの場所（<保存場所>/crew）
	crew        int      // Crew から動かした kiro-cli の会話
	crewFixed   int      // Crew の使用量の記録でクレジットを補った会話
	crewOnly    int      // kiro-cli の会話に結びつかない Crew の記録
	crewCr      float64  // そのクレジット
	crewText    int      // Crew の会話の記録だけにある会話
	crewElse    int      // ほかの履歴（Claude Code・Codex）が記録しているので、トークンとドル額を足さなかった Crew の行
	withheld    []string // 中身を出さない会話の core.Builder.Key（Withheld）
}

// Withheld は、中身を出さない Crew の会話の core.Builder.Key（kiro-cli の会話 ID の "kiro-cli:<id>" をふくむ）。
// Kiro CLI の SQLite（QStore）は同じ会話を同じ Key で出し、Crew を見ないので、読み込む側が HideWithheld で隠す。
// Crew は会話を閉じるときに sessions/cli/<id>.json を消すので、SQLite の写しだけが残ることがある。
func (k *KiroCLI) Withheld() []string { return k.withheld }

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
// Crew は退避した会話の記録を新しく書いたとき（1 つのプロセスで 1 時間に 1 回まで）に、sessions/archive/ のうち更新時刻がそれより古いものを消す
// （history.py の _archive_lines → _cleanup_old_archives。閉じた会話の crew log も同じ設定で消す）。決まった間隔で動くのではない。
// 消さない設定（null か負の数）なら nil。Crew は config.json に全部のキーを書き出すので、キーがあっても利用者が決めたとは限らない。
// そこで、既定の 30 日と違う値のときだけ設定済み（Set）にする。設定を足す先は、config.json より勝つ config.local.json を案内する。
// 0 日なら、Crew は次の片付け（次に記録を退避したとき）で退避した記録を全部消す（history.py の _cleanup_old_archives は cutoff = 今）。
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
		// kiro-cli の JSON の履歴がなくても、SQLite の写しを隠せるように、中身を出さない会話は調べる
		k.withheld = nil
		for sid, info := range loadCrew(k.CrewHome, nil, newCrewPrivacy(k.CrewHome, k.CrewArchive)) {
			if info.Private {
				k.withheld = append(k.withheld, "kiro-cli:"+sid)
			}
		}
		sort.Strings(k.withheld)
		return nil
	}
	var errs fileErrs
	priv := newCrewPrivacy(k.CrewHome, k.CrewArchive) // 中身を残さない会話（loadCrew・loadCrewSpawns と同じものを使う）
	crew := loadCrew(k.CrewHome, &errs, priv)
	usage := loadCrewUsage(k.CrewHome, &errs)
	k.crew, k.crewFixed, k.crewOnly, k.crewCr, k.crewText, k.crewElse, k.withheld = 0, 0, 0, 0, 0, 0, nil
	slotInfo := map[string]*CrewInfo{} // 使用量の記録の slot → Crew の情報（session_map の会話と、サブエージェント）
	sids := make([]string, 0, len(crew))
	for sid := range crew {
		sids = append(sids, sid)
	}
	sort.Strings(sids) // 同じ slot のサブエージェントが何件かあっても（続きから動かしたもの）、毎回同じものを使う
	for _, sid := range sids {
		i := crew[sid]
		slot := i.usageSlot()
		// 会話キーの今の会話を、サブエージェントや前の会話（discarded_sid）より先に使う
		if prev := slotInfo[slot]; slot != "" && (prev == nil || prev.Subagent && !i.Subagent || prev.Former && !i.Former && !i.Subagent) {
			slotInfo[slot] = &i
		}
	}
	// state.json が消えた（Crew が片付けた）サブエージェントは、親の会話の crew-log に残る起動の記録で親を見つける
	for slot, info := range loadCrewSpawns(k.CrewHome, &errs, priv) {
		if slotInfo[slot] == nil {
			slotInfo[slot] = &info
		}
	}
	// Crew の会話は、サブエージェントを親の会話にまとめてから出す
	held := &crewHeld{parents: map[string]*core.Builder{}, byPath: map[string]*core.Builder{}}
	owners := map[string][]*crewOwner{} // 使用量の記録の slot → Crew から動かした kiro-cli の会話
	var plain []*core.Builder           // Crew の目印のない kiro-cli の会話（前の会話を見つけてから出す）
	seenRows := map[string]bool{}       // 会話の記録を使った（kiro-cli の会話に結びついた）会話キー
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
			info := crew[s.ID]
			// 使用量の記録。サブエージェントは自分の slot（conversation_key か subagent:<id>）で記録される（親の会話キーではない）。
			// 続きから動かしたサブエージェントは元の slot を使い続け、作り直した会話は同じ会話キーを使うので、1 つの slot に会話が何件か結びつく
			if slot := info.usageSlot(); slot != "" {
				owners[slot] = append(owners[slot], newCrewOwner(s, info))
			} else {
				held.add(s, &info, info.Key)
			}
			continue
		}
		plain = append(plain, s)
	}
	slots := make([]string, 0, len(usage)+len(owners))
	for slot := range usage {
		slots = append(slots, slot)
	}
	for slot := range owners {
		if usage[slot] == nil {
			slots = append(slots, slot)
		}
	}
	sort.Strings(slots)
	// session_map から消えた前の会話（作り直す前の会話・backend を切りかえる前の会話）を、使用量の記録の時刻と作業場所で見つける
	taken := map[*core.Builder]bool{}
	for _, slot := range slots {
		if info := slotInfo[slot]; info != nil && len(usage[slot]) > 0 {
			sortCrewOwners(owners[slot])
			owners[slot] = claimCrewFormer(owners[slot], usage[slot], plain, *info, taken)
		}
	}
	for _, s := range plain {
		if taken[s] {
			k.crew++
			continue
		}
		emit(s)
	}
	for _, slot := range slots {
		ow := owners[slot]
		sortCrewOwners(ow)
		// 使用量の記録を、そのころに動いていた会話に分ける。どれにも入らない行は Crew のセッションにする
		var rest []crewTurn
		for _, x := range usage[slot] {
			if i := crewOwnerAt(ow, x.t); i >= 0 {
				ow[i].turns = append(ow[i].turns, x)
			} else {
				rest = append(rest, x)
			}
		}
		// 会話の記録。Crew から動かした会話は、kiro-cli の履歴に依頼が残らないことがある。
		// 行は使用量の記録と同じく時刻で分ける（時刻のない行は前の行と同じ会話に）。Crew のセッションがなければ、どの会話にも入らない行は今の会話に入れる
		var cm crewMeta // 会話の記録のメタデータ
		var rows, restRows []crewRow
		live := -1 // 今の会話（Former でない）
		need := len(rest) > 0
		for i, o := range ow {
			if o.info.Subagent {
				continue
			}
			if !o.info.Former {
				live = i
			}
			need = need || len(o.s.Prompts) == 0
		}
		if len(ow) > 0 && ow[0].info.Subagent { // サブエージェントの会話の記録は読まない
			need = len(rest) > 0
		}
		if need && !crewBackground(slot) && k.CrewHome != "" {
			cm, rows = readCrewKey(k.CrewHome, k.CrewArchive, slot, &errs)
		}
		if live < 0 {
			live = len(ow) - 1
		}
		at := live // 時刻のない最初の行
		if len(rest) > 0 {
			at = -1
		}
		for _, r := range rows {
			if r.t != nil {
				if at = crewOwnerAt(ow, *r.t); at < 0 && len(rest) == 0 {
					at = live
				}
			}
			if at >= 0 && !ow[at].info.Subagent {
				ow[at].rows = append(ow[at].rows, r)
			} else {
				restRows = append(restRows, r)
			}
		}
		for _, o := range ow {
			if !o.info.Subagent {
				seenRows[o.info.Key] = true
				if len(o.s.Prompts) == 0 {
					addCrewRows(o.s, o.rows)
				}
			}
			if len(o.turns) > 0 {
				if useCrewCredits(o.s, o.turns) {
					k.crewFixed++
				}
				addCrewNative(o.s, o.turns)
			}
		}
		// 会話キーの今の会話を、前の会話より先に預ける（サブエージェントの親にする）
		sort.SliceStable(ow, func(i, j int) bool { return !ow[i].info.Former && ow[j].info.Former })
		// 会話の記録（退避した記録も）が中身を残さない会話だと示すか。依頼が kiro-cli に残っていて会話の記録を読まなくても調べる
		private := cm.private || !crewBackground(slot) && priv.key(slot)
		for _, o := range ow {
			info := o.info
			if private && !info.Subagent {
				info.Private = true
			}
			held.add(o.s, &info, info.Key)
		}
		if len(rest) == 0 {
			continue
		}
		// kiro-cli の会話に結びつかない Crew の記録
		seenRows[slot] = true
		ss, skipped := crewOnly(k.CrewHome, slot, rest, slotInfo[slot], cm.title, restRows)
		k.crewElse += skipped
		for _, s := range ss {
			k.crewOnly++
			k.crewCr += sumCredits(s.Credits)
			if crewBackground(slot) {
				emit(s)
				continue
			}
			held.add(s, slotInfo[slot], slot)
			if private {
				held.hide(s)
			}
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
			// 退避した古い行も読む。fork なら、元の会話から写した行は除く
			stem := strings.TrimSuffix(filepath.Base(p), ".jsonl")
			cm, rows := readCrewStem(k.CrewHome, k.CrewArchive, stem, &errs)
			if len(rows) == 0 {
				continue
			}
			s := core.NewBuilder("Kiro Crew", "crew:"+stem)
			s.File = p
			s.Key = "kiro-crew:" + stem
			s.Title, s.Project = firstNonEmpty(cm.title, "Kiro Crew: "+stem), "(Kiro Crew)"
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
			held.byPath[p] = s
			held.list = append(held.list, heldCrew{b: s})
			if info := byFile[p]; cm.private || priv.stem(stem) || info != nil && info.Private {
				held.hide(s)
			}
		}
	}
	held.flush(k.CrewHome, emit)
	seen := map[string]bool{}
	withhold := func(key string) {
		if key != "" && !seen[key] {
			seen[key] = true
			k.withheld = append(k.withheld, key)
		}
	}
	for _, sid := range sids {
		if crew[sid].Private {
			withhold("kiro-cli:" + sid)
		}
	}
	for b := range held.private {
		withhold(b.Key)
	}
	sort.Strings(k.withheld)
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
//
// ワークスペースのフォルダの直下の <名前>.chat（参考実装 kiro-history の server/ide.ts と codeburn の parseChatFile の形）も読む:
//
//	{"executionId", "actionId", "chat": [{"role", "content"}], "metadata": {"modelId", "workflowId", "startTime", "endTime"}}
//
// クレジットはないので、同じ実行 ID の実行ファイルにないモデル・時刻を埋めるのと、実行ファイルのない実行を 1 つとして数えるのに使う。
// 会話とは、kiro-history と同じく実行 ID（会話の history[].executionId）でだけ結ぶ（metadata.workflowId が会話の ID と同じものかはわからない）。
type kiroExec struct {
	id, session string
	dir         string // ファイルのあるフォルダの名前（どの会話のものかわからない実行をまとめる鍵）
	start, end  *float64
	credits     float64
	model       string
	mtime       float64 // ファイルの更新時刻（会話に結べず、時刻もない実行の時刻に使う）
}

// kiroWorkspaceDir は実行ファイルを置くワークスペースのフォルダ名（32 桁の 16 進）。
var kiroWorkspaceDir = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// kiroExecMax は実行ファイルとして読む大きさの上限。会話の全文が入るので大きめにする。これより大きいファイルは読まない。
const kiroExecMax = 64 << 20

// kiroExecKeys は実行ファイルのうち kiroku が読む項目（topFields で取り出す）。
var kiroExecKeys = []string{"executions", "executionId", "chatSessionId", "sessionId", "modelId", "startTime", "endTime", "metadata", "usageSummary"}

// kiroChatKeys は .chat のうち kiroku が読む項目（会話の全文の chat は組み立てない）。
var kiroChatKeys = []string{"executionId", "metadata"}

// realDir は、シンボリックリンクでないふつうのフォルダか。
func realDir(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.IsDir()
}

// readKiroExecFile は実行ファイルか .chat を 1 つ読み、keys の項目を取り出す。シンボリックリンクでないふつうのファイルで、
// kiroExecMax 以下のものだけ。mtime はファイルの更新時刻。読めないファイルは errs に入れる。JSON のオブジェクトでなければ ok = false。
func readKiroExecFile(p string, keys []string, errs *fileErrs) (data map[string]any, mtime float64, ok bool) {
	st, err := os.Lstat(p)
	if err != nil || !st.Mode().IsRegular() || st.Size() > kiroExecMax {
		return nil, 0, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		errs.file(p, err)
		return nil, 0, false
	}
	// 実行ファイルには会話の全文（context）が入っていて大きい。全体を組み立てると読み込みがとても遅くなるので、
	// 使う項目だけを取り出す
	data, ok = topFields(b, keys)
	return data, float64(st.ModTime().UnixNano()) / 1e9, ok
}

// loadKiroExecs は globalStorage の下の実行ファイルを読む。実行 ID ごとと、会話の ID ごと。
// 読むのは <32 桁の 16 進>/<フォルダ>/<拡張子のない名前> と <32 桁の 16 進>/<名前>.chat だけ。フォルダとファイルは、
// シンボリックリンクでない（ほかの場所を読まない）ふつうのものに限る。パスは履歴の中身からは作らない（中身の ID は結びつけるための鍵にしか使わない）。
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
		data, mtime, ok := readKiroExecFile(p, kiroExecKeys, errs)
		if !ok || data["executions"] != nil {
			continue
		}
		meta := core.Map(data["metadata"])
		x := &kiroExec{
			id:      firstNonEmpty(core.Str(data["executionId"]), name),
			session: firstNonEmpty(core.Str(data["chatSessionId"]), core.Str(data["sessionId"])),
			dir:     filepath.Base(sess),
			model:   firstNonEmpty(core.Str(data["modelId"]), core.Str(meta["modelId"])),
			start:   firstTS(data["startTime"], meta["startTime"]),
			end:     firstTS(data["endTime"], meta["endTime"]),
			credits: creditsOf(data["usageSummary"], "unit", "usage"),
			mtime:   mtime,
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
	// .chat は実行ファイルのあとに読む（同じ実行なら実行ファイルの値を使い、ないものだけ埋める）
	for _, p := range glob(filepath.Join(gs, "*", "*.chat")) {
		ws := filepath.Dir(p)
		name := filepath.Base(p)
		if !kiroWorkspaceDir.MatchString(filepath.Base(ws)) || !safeName(name) || strings.HasPrefix(name, ".") || !realDir(ws) {
			continue
		}
		data, mtime, ok := readKiroExecFile(p, kiroChatKeys, errs)
		meta := core.Map(data["metadata"])
		id := core.Str(data["executionId"])
		if !ok || meta == nil || id == "" { // 実行 ID がなければ会話と結べない
			continue
		}
		model, start, end := core.Str(meta["modelId"]), ts(meta["startTime"]), ts(meta["endTime"])
		if end != nil && start != nil && *end < *start {
			end = nil
		}
		if x := byID[id]; x != nil {
			x.model = firstNonEmpty(x.model, model)
			if x.start == nil && start != nil && (x.end == nil || *start <= *x.end) {
				x.start = start
			}
			if x.end == nil && end != nil && (x.start == nil || *end >= *x.start) {
				x.end = end
			}
			continue
		}
		if start == nil && end == nil && model == "" {
			continue
		}
		byID[id] = &kiroExec{id: id, dir: filepath.Base(ws), model: model, start: start, end: end, mtime: mtime}
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
		claimed := map[*kiroExec]bool{} // 見える会話に結んだ実行
		hiddenWS := map[string]string{} // 一覧で隠した会話の ID → 作業場所
		for _, index := range indexes {
			list, err := core.ReadJSONFile(index)
			errs.file(index, err)
			for _, ent := range core.List(list) {
				em := core.Map(ent)
				hidden, _ := em["hidden"].(bool)
				id := core.Str(em["sessionId"])
				if em == nil || id == "" {
					continue
				}
				if hidden { // 隠した会話は一覧に出さない（kiro-history と同じ）。実行のクレジットはあとで数える
					hiddenWS[id] = firstNonEmpty(hiddenWS[id], core.Str(em["workspaceDirectory"]))
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
				// この会話の実行（history の executionId と、実行ファイルの会話 ID で結ぶ）。ほかの会話に結んだ実行は二度数えない
				var execs []*kiroExec
				link := func(x *kiroExec) {
					if x != nil && !claimed[x] {
						claimed[x] = true
						execs = append(execs, x)
					}
				}
				// 依頼の時刻: 依頼のあと（次の依頼より前）に出てくる最初の実行の開始時刻。わからなければ会話の作成時刻
				type ask struct {
					t     *float64
					text  string
					notes []string // Kiro が依頼に足した文（steering の指示・作業環境など）
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
						text, notes := kiroUserText(core.TextOf(m["content"]))
						asks = append(asks, ask{text: text, notes: notes})
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
					for _, n := range a.notes {
						s.Inject(t, "reminder", n)
					}
					if a.text != "" {
						s.Prompt(t, a.text)
					}
				}
				for _, x := range execBySession[id] {
					link(x)
				}
				selected := core.Str(data["selectedModel"]) // 会話で選んでいたモデル（実行ファイルにモデルがなければこれ）
				addKiroExecs(s, execs, selected, start)
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
		emitKiroLeftovers(execByID, claimed, hiddenWS, emit)
	}
	return errs.err()
}

// addKiroExecs は会話の実行を足す。1 つの実行が 1 ターンで、クレジットは終わりの時刻に数える。
// モデルは実行のもの、なければ selected。時刻のない実行は start に数える。
func addKiroExecs(s *core.Builder, execs []*kiroExec, selected string, start *float64) {
	sort.SliceStable(execs, func(i, j int) bool {
		a, b := firstNonNil(execs[i].start, execs[i].end), firstNonNil(execs[j].start, execs[j].end)
		return a != nil && (b == nil || *a < *b)
	})
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
}

// emitKiroLeftovers は、見える会話に結べなかった実行（一覧で隠した会話・一覧にない会話・会話の ID のないもの）を、
// 依頼のないセッションとして出す。codeburn はすべての実行ファイルを数えるので、クレジットの合計をそろえるため。
// 隠した会話の依頼は出さない。まとめる鍵は実行ファイルの会話の ID、なければファイルのあるフォルダの名前（codeburn と同じ）。
func emitKiroLeftovers(byID map[string]*kiroExec, claimed map[*kiroExec]bool, hiddenWS map[string]string, emit func(*core.Builder)) {
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	groups := map[string][]*kiroExec{}
	var keys []string
	for _, id := range ids {
		x := byID[id]
		if claimed[x] {
			continue
		}
		if x.start == nil && x.end == nil && x.mtime > 0 { // 時刻がなければファイルの更新時刻（ないとセッションごと出ない）
			m := x.mtime
			x.end = &m
		}
		key := firstNonEmpty(x.session, x.dir)
		if groups[key] == nil {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], x)
	}
	sort.Strings(keys)
	for _, key := range keys {
		s := core.NewBuilder("Kiro IDE (legacy)", "exec:"+key)
		s.Project = "(Kiro IDE)"
		if ws, hidden := hiddenWS[key]; hidden {
			s.Title = "Kiro IDE hidden chat"
			s.Project = firstNonEmpty(ws, s.Project)
		} else {
			s.Title = "Kiro IDE executions without a chat"
		}
		addKiroExecs(s, groups[key], "", nil)
		emit(s)
	}
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

// Kiro IDE（v1.0 より前）が依頼の文に足すもの。参考実装 kiro-history（server/ide.ts の cleanUserContent）が外すものと同じ。
var (
	kiroSteeringRe = regexp.MustCompile(`(?s)<steering-reminder>(.*?)</steering-reminder>`)
	kiroEnvRe      = regexp.MustCompile(`(?s)<EnvironmentContext>(.*?)</EnvironmentContext>`)
)

// kiroRulesHead は、Kiro IDE が依頼に足す steering の規則の見出し。
const kiroRulesHead = "## Included Rules"

// kiroUserText は Kiro IDE（v1.0 より前）の依頼の文を、人が打った文と、Kiro が足した文（notes）に分ける。
// 足した文は <steering-reminder>…</steering-reminder>・<EnvironmentContext>…</EnvironmentContext>・
// 「## Included Rules」から始まる塊。塊の終わりは kiroRulesEnd。
// kiro-history の正規表現 /## Included Rules[\s\S]*?(?=\n\n[^#\s]|\n\n$|$)/g と同じ（Go の regexp には先読みがないので、手で探す）。
func kiroUserText(text string) (prompt string, notes []string) {
	for _, re := range []*regexp.Regexp{kiroSteeringRe, kiroEnvRe} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			notes = append(notes, m[1])
		}
		text = re.ReplaceAllString(text, "")
	}
	var b strings.Builder
	for {
		i := strings.Index(text, kiroRulesHead)
		if i < 0 {
			b.WriteString(text)
			break
		}
		b.WriteString(text[:i])
		end := kiroRulesEnd(text, i+len(kiroRulesHead))
		notes = append(notes, text[i:end])
		text = text[end:]
	}
	return strings.TrimSpace(b.String()), notes
}

// kiroRulesEnd は、from から探して「## Included Rules」の塊が終わる位置。空行（\n\n）のあとに「#」でも空白でもない文字が来るか、
// 空行で文が終わるなら、その空行の前。どちらもなければ文の終わり。
func kiroRulesEnd(text string, from int) int {
	for j := from; j < len(text); j++ {
		k := strings.Index(text[j:], "\n\n")
		if k < 0 {
			break
		}
		j += k
		rest := text[j+2:]
		if rest == "" {
			return j
		}
		if r, _ := utf8.DecodeRuneInString(rest); r != '#' && !unicode.IsSpace(r) {
			return j
		}
	}
	return len(text)
}
