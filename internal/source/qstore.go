package source

import (
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	_ "modernc.org/sqlite" // C を使わない SQLite。どの OS にもそのままビルドできる
)

// QStore は Kiro CLI（古い版）と Amazon Q Developer CLI の data.sqlite3。
// どちらも同じ作り（amazon-q-developer-cli が元）:
//
//	conversations(key = 作業フォルダ, value = 会話の JSON)          … フォルダごとに 1 行を上書き
//	conversations_v2(key, conversation_id, value, created_at, updated_at) … Kiro CLI だけ。フォルダに会話が何件も入る
//
// value.history[i] = {user: {content, timestamp(RFC3339)}, assistant: {Response|ToolUse}, request_metadata: {...}}。
// value.latest_summary = [要約, request_metadata]（最後のコンパクション。qCompaction）。
// value.model_info = {model_id, context_window_tokens}、value.context_message_length（文脈の文字数）。
// トークンやクレジットは入っていない。
type QStore struct {
	Label   string // 画面の名前
	Fam     string // --sources の名前
	DB      string // data.sqlite3 の場所
	Command string // conversations の会話の再開コマンド（例: kiro-cli chat --resume。フォルダの最新の会話を開く）
	// conversations_v2 の会話を ID で開くコマンド（例: kiro-cli chat --resume-id）。
	// v2 はフォルダに会話が何件もあり、--resume では最新のものしか開けない。空なら v2 の会話には出さない
	ResumeID string
}

func (q *QStore) Name() string   { return q.Label }
func (q *QStore) Family() string { return q.Fam }
func (q *QStore) Where() string  { return q.DB }

// DataDir は Rust の dirs::data_local_dir() と同じ場所。
func DataDir(app string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home(), "Library", "Application Support", app)
	case "windows":
		d := os.Getenv("LOCALAPPDATA")
		if d == "" {
			d = filepath.Join(home(), "AppData", "Local")
		}
		return filepath.Join(d, app)
	default:
		d := os.Getenv("XDG_DATA_HOME")
		if d == "" {
			d = filepath.Join(home(), ".local", "share")
		}
		return filepath.Join(d, app)
	}
}

// sqliteDSN は読み取り専用で開くための URI。Windows は file:///C:/… の形にする必要がある。
func sqliteDSN(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	p := filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// URI の中で意味を持つ文字は %HH にする（macOS の "Application Support" の空白など）
	p = strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23", " ", "%20").Replace(p)
	return "file://" + p + "?mode=ro&_pragma=busy_timeout(2000)"
}

type qRow struct {
	key, id, value   string
	created, updated any
	v2               bool // conversations_v2 の行
}

func (q *QStore) Load(emit func(*core.Builder)) error {
	if _, err := os.Stat(q.DB); err != nil {
		return nil
	}
	db, err := sql.Open("sqlite", sqliteDSN(q.DB))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return err // 開けなかったことを「計測の状態」に出す
	}
	var rows []qRow
	var errs fileErrs // 読めなかった行は飛ばして残りを読み、最後に「計測の状態」に出す
	// 新しいほう（v2）を先に読み、同じ会話 ID は古いほうから読まない。
	// /load で同じ会話（同じ conversation_id）が別のフォルダの行にも入るので、どちらの表も新しく保存した行から読む
	// （v2 は updated_at、v1 は INSERT OR REPLACE で新しく保存した行ほど rowid が大きい）
	if has(db, "conversations_v2") {
		r, err := db.Query(`SELECT key, conversation_id, value, created_at, updated_at FROM conversations_v2 ORDER BY updated_at DESC`)
		if err != nil {
			return err
		}
		for r.Next() {
			x := qRow{v2: true}
			if err := r.Scan(&x.key, &x.id, &x.value, &x.created, &x.updated); err != nil {
				errs.file(q.DB, err)
				continue
			}
			rows = append(rows, x)
		}
		errs.file(q.DB, r.Err())
		r.Close()
	}
	if has(db, "conversations") {
		r, err := db.Query(`SELECT key, value FROM conversations ORDER BY rowid DESC`)
		if err != nil {
			return err
		}
		for r.Next() {
			var x qRow
			if err := r.Scan(&x.key, &x.value); err != nil {
				errs.file(q.DB, err)
				continue
			}
			rows = append(rows, x)
		}
		errs.file(q.DB, r.Err())
		r.Close()
	}
	seen := map[string]bool{} // 読んだ会話 ID（同じ会話は最初に読んだ、いちばん新しい行だけ使う）
	for _, x := range rows {
		var v any
		if err := json.Unmarshal([]byte(x.value), &v); err != nil {
			errs.file(q.DB, err) // 壊れた行は飛ばして残りを読み、最後に「計測の状態」に出す
			continue
		}
		conv := core.Map(v)
		id := firstNonEmpty(x.id, core.Str(conv["conversation_id"]))
		if id == "" {
			id = x.key
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		s := core.NewBuilder(q.Label, id)
		s.File = q.DB
		s.Key = "kiro-cli:" + id // 新しい形式（~/.kiro/sessions/cli）と同じ会話なら、そちらを使う
		s.Project = x.key
		s.Tick(ts(x.created))
		s.Tick(ts(x.updated))
		model := firstNonEmpty(core.Str(core.Get(conv, "model_info", "model_id")), core.Str(conv["model"]))
		window := qContextWindow(conv)
		chars := qContextChars(conv) // 文脈（要約・コンテキストファイル）の分から数え始める
		var lastT *float64           // 時刻のない行の観測に使う、直前の時刻
		for _, h := range core.List(conv["history"]) {
			user, asst, meta := historyEntry(h)
			// ツールの結果を返したターン（ToolUseResults）には user の時刻がない（new_tool_use_results）。
			// そのときは依頼を送った時刻を使う（ないと週の画面でセッションの頭に寄ってしまう）
			t := ts(user["timestamp"])
			if t == nil {
				t = ts(meta["request_start_timestamp_ms"])
			}
			// コンテキストの使用率の見積もり（CLI と同じく文字数 ÷ 4。ここまでの履歴の大きさ ÷ 上限）
			chars += qUserChars(user) + qAssistantChars(asst)
			if t != nil {
				lastT = t
			}
			if lastT != nil {
				s.Measure("context_used", lastT, min(float64(qTokens(chars))/window, 1))
			}
			content := core.Map(user["content"])
			reply := true
			switch {
			case content["Prompt"] != nil:
				text := core.Str(core.Get(content, "Prompt", "prompt"))
				trimmed := strings.TrimSpace(text)
				s.Tick(t)
				switch k := qInjected[trimmed]; {
				case k != "":
					s.Inject(t, k, text)
					// --resume の要約は、前の依頼への応答ではないので付けない
					reply = trimmed != qResumeSummary
				case strings.HasPrefix(trimmed, qSystemNote): // /todos resume が入れた依頼
					s.Inject(t, "meta", text)
				case qCancelledOnly(trimmed):
					// ツールを断った・止めたターン（CancelledToolUses）が履歴の先頭に残ったとき
					// （/compact --messages-to-exclude など）、CLI がツールの結果の決まった文をつないだ Prompt に書き換えたもの
					// （message.rs の replace_content_with_tool_use_results）。人が打った文は消え、時刻だけ残る。下と同じく数えない
					s.Inject(t, "output", text)
					s.Agent(t)
				case user["timestamp"] == nil && meta == nil && !isLegacyEntry(h):
					// MCP の /prompts get が入れた user と assistant の組（conversation.rs の append_prompts）。
					// 時刻も request_metadata もない。応答も MCP サーバーの文で、モデルの応答ではないので付けない
					s.Inject(t, "other", text)
					reply = false
				case user["timestamp"] == nil && meta != nil && !isLegacyEntry(h):
					// コンパクションで残したターンの先頭や、応答のあとに返したツールの結果（ToolUseResults）を、
					// CLI が出力をつないだ Prompt に書き換えたもの（conversation.rs の enforce_conversation_invariants・
					// message.rs の replace_content_with_tool_use_results）。時刻はツールの結果のまま（なし）。
					// 人が打った依頼には必ず時刻がある（set_next_user_message）。ツールの出力として残し、自動で返したターンにする
					s.Inject(t, "output", text)
					s.Agent(t)
				default:
					s.Prompt(t, text)
				}
			case content["CancelledToolUses"] != nil:
				// ツールを断った・止めたときの発言。prompt は人が打った文か、CLI が入れた決まった文
				text := core.Str(core.Get(content, "CancelledToolUses", "prompt"))
				trimmed := strings.TrimSpace(text)
				s.Tick(t)
				switch {
				case trimmed == qInterrupted: // Ctrl+C でツールを止めた。応答も CLI が入れた決まった文なので残さない
					s.Interrupt(t)
					reply = false
				case trimmed == qDenied, qForbidden(trimmed): // 「n」で断った・設定で禁じた引数のツールを CLI が断った
					s.Inject(t, "meta", text)
				default:
					s.Prompt(t, text)
				}
			default: // ToolUseResults は自動で返したもの
				s.Agent(t)
			}
			if meta == nil && core.Str(core.Get(asst, "Response", "content")) == qTimedOut {
				reply = false // 応答のタイムアウトで CLI が入れた決まった文（モデルの応答ではない）
			}
			if tu := core.Map(asst["ToolUse"]); tu != nil {
				for _, u := range core.List(tu["tool_uses"]) {
					um := core.Map(u)
					s.Tool(firstNonEmpty(core.Str(um["name"]), core.Str(um["orig_name"])), um["args"])
					s.Measure("tool_calls", t, 1)
				}
			}
			if meta != nil {
				start, okS := core.Num(meta["request_start_timestamp_ms"])
				end, okE := core.Num(meta["stream_end_timestamp_ms"])
				if okS && okE && end >= start {
					s.Measure("latency", t, (end-start)/1000)
				}
				if v, ok := duration(meta["time_to_first_chunk"]); ok {
					s.Measure("ttfc", t, v)
				}
				if v, ok := core.Num(meta["response_size"]); ok {
					s.Measure("response_size", t, v) // バイト数（content の len とツール入力の JSON）
				}
			}
			s.Agent(ts(meta["request_start_timestamp_ms"]))
			s.Agent(ts(meta["stream_end_timestamp_ms"]))
			// モデルは、実際にモデルへ依頼したもの（request_metadata がある）だけ数える。
			// 中断・MCP の /prompts・タイムアウトのときに CLI が足した行は request_metadata がない。
			// 古い [user, assistant] の形には request_metadata 自体がないので、中断以外は数える
			if asst != nil && (meta != nil || (isLegacyEntry(h) && reply)) {
				s.Model(firstNonEmpty(core.Str(meta["model_id"]), model))
			}
			if asst != nil && reply {
				// 人に返した文（Response は答え、ToolUse はツールを使う前に書いた文）。
				// 応答の時刻は記録にないので、わかれば返し終わった時刻、なければ依頼の時刻にする
				rt := ts(meta["stream_end_timestamp_ms"])
				if rt == nil {
					rt = t
				}
				s.Reply(rt, "", firstNonEmpty(core.Str(core.Get(asst, "Response", "content")), core.Str(core.Get(asst, "ToolUse", "content"))))
			}
		}
		if lastT != nil {
			s.Measure("context_window", lastT, window)
		}
		qCompaction(s, conv)
		switch {
		case s.Project == "":
		case x.v2:
			if q.ResumeID != "" && x.id != "" {
				s.Resume = core.ResumeCmd(s.Project, q.ResumeID, x.id)
			}
		case q.Command != "":
			s.Resume = core.ResumeCmd(s.Project, q.Command, "")
		}
		emit(s)
	}
	return errs.err()
}

// amazon-q-developer-cli が人の代わりに入れる決まった文（crates/chat-cli/src/cli/chat の mod.rs・conversation.rs）。
// どれも人が打ったプロンプトではない。
const (
	qInterrupted   = "The user interrupted the tool execution."                                          // ツールの実行中に Ctrl+C（CancelledToolUses）
	qDenied        = "I deny this tool request. Ask a follow up question clarifying the expected action" // ツールの確認に「n」（CancelledToolUses）
	qResumeSummary = "In a few words, summarize our conversation so far."                                // 入力なしの --resume（Prompt）
	qSystemNote    = "[SYSTEM NOTE: This is an automated request, not from the user]"                    // /todos resume の依頼の頭（Prompt）
	qTimedOut      = "Response timed out - message took too long to generate"                            // 応答のタイムアウト（mod.rs の RESPONSE_TIMEOUT_CONTENT。Response）
)

// qForbidden は、設定で禁じた引数のツールを CLI が自動で断ったときの文か（mod.rs:
// "Tool use with {name} was rejected because the arguments supplied were forbidden"。CancelledToolUses）。
func qForbidden(text string) bool {
	const prefix, suffix = "Tool use with ", " was rejected because the arguments supplied were forbidden"
	return len(text) > len(prefix)+len(suffix) && strings.HasPrefix(text, prefix) && strings.HasSuffix(text, suffix)
}

// qCancelled は、断った・止めたツールの結果として CLI が入れる決まった文（message.rs の new_cancelled_tool_uses）。
const qCancelled = "Tool use was cancelled by the user"

// qCancelledOnly は、text が qCancelled を空白 1 つでつないだだけの文か
// （replace_content_with_tool_use_results はツールの結果を " " でつなぐ）。
func qCancelledOnly(text string) bool {
	for {
		rest, ok := strings.CutPrefix(text, qCancelled)
		if !ok {
			return false
		}
		if rest == "" {
			return true
		}
		if text, ok = strings.CutPrefix(rest, " "); !ok {
			return false
		}
	}
}

// qInjected は Prompt として入る決まった文と、その Note の種類。
var qInjected = map[string]string{
	qResumeSummary: "meta",
	"You took too long to respond - try to split up the work into smaller steps.": "meta", // 応答のタイムアウト
	"The conversation history has overflowed, clearing state":                     "meta", // 履歴があふれて消したとき
}

// qCompaction は、最後のコンパクション（/compact と、文脈があふれたときの自動の要約）を記録する。
// CLI は要約を latest_summary に [要約, request_metadata] で残し（conversation.rs の replace_history_with_summary）、
// その request_metadata の message_meta_tags に Compact が入る（mod.rs の compact_history）。
// 前の要約は次のコンパクションで上書きされ、要約より前の履歴も消える（history.drain）ので、残るのは最後の 1 回だけ。
// 要約だけの古い形（文字列）には時刻がないので数えない。
func qCompaction(s *core.Builder, conv core.Obj) {
	pair := core.List(conv["latest_summary"])
	if len(pair) < 2 {
		return
	}
	meta := core.Map(pair[1])
	if tags, ok := meta["message_meta_tags"]; ok {
		compact := false
		for _, tag := range core.List(tags) {
			compact = compact || core.Str(tag) == "Compact"
		}
		if !compact {
			return
		}
	}
	t := ts(meta["stream_end_timestamp_ms"])
	if t == nil {
		t = ts(meta["request_start_timestamp_ms"])
	}
	s.Tick(t)
	s.Compact(t, "")
}

// isLegacyEntry は、古い版の [user, assistant] の形か（request_metadata を記録しない）。
func isLegacyEntry(h any) bool { return len(core.List(h)) >= 2 }

// duration は Rust の Duration（{secs, nanos}）か秒の数値を秒にする。
func duration(v any) (float64, bool) {
	if f, ok := core.Num(v); ok {
		return f, true
	}
	m := core.Map(v)
	secs, ok := core.Num(m["secs"])
	if !ok {
		return 0, false
	}
	return secs + core.NumOr0(m["nanos"])/1e9, true
}

// historyEntry は {user, assistant, request_metadata} と、古い版の [user, assistant] の両方を読む。
func historyEntry(h any) (user, asst, meta core.Obj) {
	if l := core.List(h); len(l) >= 2 {
		return core.Map(l[0]), core.Map(l[1]), nil
	}
	m := core.Map(h)
	return core.Map(m["user"]), core.Map(m["assistant"]), core.Map(m["request_metadata"])
}

func has(db *sql.DB, table string) bool {
	var n int
	err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n)
	return err == nil && n > 0
}

// qDefaultWindow は model_info がないときのコンテキストの上限（cli/chat/cli/model.rs の default_context_window）。
const qDefaultWindow = 200_000

// qContextWindow は会話のコンテキストの上限（トークン）。CLI と同じく model_info.context_window_tokens、なければ 200,000。
func qContextWindow(conv core.Obj) float64 {
	if v, ok := core.Num(core.Get(conv, "model_info", "context_window_tokens")); ok && v > 0 && !math.IsInf(v, 0) {
		return v
	}
	return qDefaultWindow
}

// qContextAck は、CLI が文脈（/compact の要約・コンテキストファイル）を送るときに添える決まった応答（conversation.rs の context_messages）。
const qContextAck = "I will fully incorporate this information when generating my responses, and explicitly acknowledge relevant parts of the summary when answering questions."

// qContextChars は文脈のメッセージの文字数。CLI が最後に送った文脈の長さ（context_message_length）と、決まった応答の長さ。
// 文脈は依頼ごとに作り直すので、最後の長さしか残らない（どの依頼にも同じ長さを足す）。
func qContextChars(conv core.Obj) int {
	n, ok := core.Num(conv["context_message_length"])
	if !ok || n <= 0 || n > math.MaxInt32 {
		return 0
	}
	return int(n) + len(qContextAck)
}

// qTokens は CLI の文字数からトークン数への見積もり（cli/chat/token_counter.rs の count_tokens_char_count: 文字数 ÷ 4 を 10 単位に丸める）。
func qTokens(chars int) int { return (chars/4 + 5) / 10 * 10 }

// qUserChars は user の文字数（token_counter.rs の UserMessage の char_count。Rust の len と同じくバイト数）。
func qUserChars(user core.Obj) int {
	n := len(core.Str(user["additional_context"]))
	content := core.Map(user["content"])
	if p := core.Map(content["Prompt"]); p != nil {
		n += len(core.Str(p["prompt"]))
	}
	if c := core.Map(content["CancelledToolUses"]); c != nil {
		n += len(core.Str(c["prompt"])) + qResultChars(c["tool_use_results"])
	}
	if r := core.Map(content["ToolUseResults"]); r != nil {
		n += qResultChars(r["tool_use_results"])
	}
	return n
}

// qResultChars はツールの結果の文字数（[ToolUseResult] の char_count）。
func qResultChars(v any) int {
	n := 0
	for _, r := range core.List(v) {
		for _, b := range core.List(core.Map(r)["content"]) {
			bm := core.Map(b)
			if j, ok := bm["Json"]; ok {
				n += qValueChars(j, 0)
			}
			n += len(core.Str(bm["Text"]))
		}
	}
	return n
}

// qAssistantChars は assistant の文字数（AssistantMessage の char_count: 文とツール入力の値）。
func qAssistantChars(asst core.Obj) int {
	if r := core.Map(asst["Response"]); r != nil {
		return len(core.Str(r["content"]))
	}
	tu := core.Map(asst["ToolUse"])
	n := len(core.Str(tu["content"]))
	for _, u := range core.List(tu["tool_uses"]) {
		n += qValueChars(core.Map(u)["args"], 0)
	}
	return n
}

// qValueChars は JSON の値の文字数（token_counter.rs の calculate_value_char_count: 文字列はバイト数、数・真偽値・null は 1、
// 配列とオブジェクトは中身の和でキーは数えない）。深すぎる入れ子はそこで打ち切る。
func qValueChars(v any, depth int) int {
	if depth > 64 {
		return 0
	}
	switch x := v.(type) {
	case string:
		return len(x)
	case []any:
		n := 0
		for _, y := range x {
			n += qValueChars(y, depth+1)
		}
		return n
	case map[string]any:
		n := 0
		for _, y := range x {
			n += qValueChars(y, depth+1)
		}
		return n
	default: // 数・真偽値・null
		return 1
	}
}
