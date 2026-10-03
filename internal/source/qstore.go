package source

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	_ "modernc.org/sqlite" // C を使わない SQLite。どの OS にもそのままビルドできる
)

// QStore は Kiro CLI（古い版）と Amazon Q Developer CLI の data.sqlite3。
// どちらも同じ作り（amazon-q-developer-cli が元）:
//
//	conversations(key = 作業フォルダ, value = 会話の JSON)          … フォルダごとに 1 行を上書き
//	conversations_v2(key, conversation_id, value, created_at, updated_at) … Kiro CLI だけ
//
// value.history[i] = {user: {content, timestamp(RFC3339)}, assistant: {Response|ToolUse}, request_metadata: {...}}。
// トークンやクレジットは入っていない。
type QStore struct {
	Label   string // 画面の名前
	Fam     string // --sources の名前
	DB      string // data.sqlite3 の場所
	Command string // 再開コマンド（例: kiro-cli chat --resume）
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

func sqliteDSN(path string) string {
	p := filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		p = "/" + p
	}
	return "file:" + p + "?mode=ro&_pragma=busy_timeout(2000)"
}

type qRow struct {
	key, id, value   string
	created, updated any
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
	var rows []qRow
	seen := map[string]bool{}
	// 新しいほう（v2）を先に読み、同じ会話 ID は古いほうから読まない
	if has(db, "conversations_v2") {
		r, err := db.Query(`SELECT key, conversation_id, value, created_at, updated_at FROM conversations_v2`)
		if err != nil {
			return err
		}
		for r.Next() {
			var x qRow
			if r.Scan(&x.key, &x.id, &x.value, &x.created, &x.updated) == nil {
				rows = append(rows, x)
				seen[x.id] = true
			}
		}
		r.Close()
	}
	if has(db, "conversations") {
		r, err := db.Query(`SELECT key, value FROM conversations`)
		if err != nil {
			return err
		}
		for r.Next() {
			var x qRow
			if r.Scan(&x.key, &x.value) == nil {
				rows = append(rows, x)
			}
		}
		r.Close()
	}
	for _, x := range rows {
		var v any
		if json.Unmarshal([]byte(x.value), &v) != nil {
			continue
		}
		conv := core.Map(v)
		id := firstNonEmpty(x.id, core.Str(conv["conversation_id"]))
		if x.id == "" && seen[id] {
			continue
		}
		if id == "" {
			id = x.key
		}
		s := core.NewBuilder(q.Label, id)
		s.Key = "kiro-cli:" + id // 新しい形式（~/.kiro/sessions/cli）と同じ会話なら、そちらを使う
		s.Project = x.key
		s.Tick(ts(x.created))
		s.Tick(ts(x.updated))
		model := firstNonEmpty(core.Str(core.Get(conv, "model_info", "model_id")), core.Str(conv["model"]))
		for _, h := range core.List(conv["history"]) {
			user, asst, meta := historyEntry(h)
			t := ts(user["timestamp"])
			content := core.Map(user["content"])
			switch {
			case content["Prompt"] != nil:
				s.Tick(t)
				s.Prompt(t, core.Str(core.Get(content, "Prompt", "prompt")))
			case content["CancelledToolUses"] != nil:
				s.Tick(t)
				s.Prompt(t, core.Str(core.Get(content, "CancelledToolUses", "prompt")))
			default: // ToolUseResults は自動で返したもの
				s.Agent(t)
			}
			if tu := core.Map(asst["ToolUse"]); tu != nil {
				for _, u := range core.List(tu["tool_uses"]) {
					um := core.Map(u)
					s.Tool(firstNonEmpty(core.Str(um["name"]), core.Str(um["orig_name"])), um["args"])
				}
			}
			s.Agent(ts(meta["request_start_timestamp_ms"]))
			s.Agent(ts(meta["stream_end_timestamp_ms"]))
			if asst != nil {
				s.Model(firstNonEmpty(core.Str(meta["model_id"]), model))
			}
		}
		if q.Command != "" && s.Project != "" {
			s.Resume = "cd " + s.Project + " && " + q.Command
		}
		emit(s)
	}
	return nil
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
