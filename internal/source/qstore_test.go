package source

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

func load(t *testing.T, s Source) []*core.Builder {
	t.Helper()
	var out []*core.Builder
	if err := s.Load(func(b *core.Builder) { out = append(out, b) }); err != nil {
		t.Fatal(err)
	}
	return out
}

func find(bs []*core.Builder, id string) *core.Builder {
	for _, b := range bs {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// testdata/sqlite/*.sqlite3 は amazon-q-developer-cli の作りに合わせた合成データ。
func TestKiroCLISQLite(t *testing.T) {
	time.Local = time.UTC
	db := filepath.Join("..", "..", "testdata", "sqlite", "kiro-cli.sqlite3")
	bs := load(t, &QStore{Label: "Kiro CLI (SQLite)", Fam: "kiro", DB: db, Command: "kiro-cli chat --resume"})
	// v2 の 2 件 + v1 の 1 件（v1 の conv-a は v2 と同じ会話なので読まない）
	if len(bs) != 3 {
		t.Fatalf("会話の数 = %d, want 3", len(bs))
	}
	a := find(bs, "conv-a")
	if a == nil {
		t.Fatal("conv-a がない")
	}
	if a.Project != "/Users/me/app" || a.Key != "kiro-cli:conv-a" {
		t.Errorf("project/key = %q %q", a.Project, a.Key)
	}
	if len(a.Prompts) != 2 || !strings.HasPrefix(a.Prompts[1].Text, "違う") {
		t.Errorf("依頼 = %+v（ツールの結果は依頼に数えない）", a.Prompts)
	}
	if len(a.FixTS) != 1 {
		t.Errorf("言い直し = %d, want 1", len(a.FixTS))
	}
	f := a.Finish(15)
	// 応答（assistant の Response・ToolUse の content）は、返し終わった時刻で依頼に付く
	if r := f.Prompts[0].Reply; r == nil || r.Text != "ok" || r.T == nil || *r.T != 1790643625 {
		t.Errorf("応答 = %+v, want ok（stream_end_timestamp_ms の時刻）", r)
	}
	if f.NFiles != 1 || f.Files[0] != "/Users/me/app/test/login.spec.ts" {
		t.Errorf("変更したファイル = %v", f.Files)
	}
	if len(f.Models) != 1 || f.Models[0][0] != "claude-sonnet-4.5" {
		t.Errorf("モデル = %v", f.Models)
	}
	if want := float64(time.Date(2026, 9, 29, 0, 59, 0, 0, time.UTC).Unix()); f.Start != want {
		t.Errorf("開始 = %v, want %v（created_at はミリ秒）", f.Start, want)
	}
	if len(f.Waits) == 0 {
		t.Error("待たせ時間が取れていない（request_metadata の時刻）")
	}
	if n := nativeOf(f); n["応答にかかった時間（中央値）"].V != 20 || n["ツール呼び出し"].V != 1 {
		t.Errorf("参考指標 = %+v", f.Native)
	}
	b := find(bs, "conv-b")
	if b == nil || b.Project != "/Users/me/ci" || b.Resume != "cd /Users/me/ci && kiro-cli chat --resume" {
		t.Fatalf("conv-b = %+v", b)
	}
	if fb := b.Finish(15); fb.Models[0][0] != "claude-opus-4.5" || fb.Tools[0][0] != "execute_bash" {
		t.Errorf("conv-b のモデル/ツール = %v %v", fb.Models, fb.Tools)
	}
}

func TestAmazonQ(t *testing.T) {
	time.Local = time.UTC
	db := filepath.Join("..", "..", "testdata", "sqlite", "amazon-q.sqlite3")
	bs := load(t, &QStore{Label: "Amazon Q", Fam: "amazonq", DB: db, Command: "q chat --resume"})
	if len(bs) != 2 {
		t.Fatalf("会話の数 = %d, want 2", len(bs))
	}
	if q := find(bs, "q-1"); q == nil || len(q.Prompts) != 1 || q.Finish(15).Models[0][0] != "claude-3-7-sonnet" {
		t.Errorf("q-1 = %+v", q)
	}
	old := find(bs, "q-old") // 古い [user, assistant] の形
	if old == nil || len(old.Prompts) != 1 || old.Prompts[0].Text != "古い形の会話" {
		t.Errorf("古い形の会話が読めていない: %+v", old)
	}
}

func TestSQLiteDSN(t *testing.T) {
	dsn := sqliteDSN(filepath.Join("..", "..", "testdata", "sqlite", "kiro-cli.sqlite3"))
	if !strings.HasPrefix(dsn, "file:///") || !strings.Contains(dsn, "/testdata/sqlite/kiro-cli.sqlite3?mode=ro") {
		t.Errorf("DSN = %q（絶対パスにして file:/// で始める）", dsn)
	}
}

// macOS の既定の場所（Application Support）のように、空白などが入っていても開ける。
func TestSQLitePathWithSpaces(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Application Support", "kiro-cli #1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sqlite", "kiro-cli.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(dir, "data.sqlite3")
	os.WriteFile(db, b, 0o644)
	if bs := load(t, &QStore{Label: "Kiro CLI (SQLite)", DB: db}); len(bs) != 3 {
		t.Fatalf("会話の数 = %d, want 3", len(bs))
	}
}

func TestMissingDatabaseIsNotAnError(t *testing.T) {
	bs := load(t, &QStore{Label: "Amazon Q", DB: filepath.Join(t.TempDir(), "none.sqlite3")})
	if len(bs) != 0 {
		t.Fatal("ないファイルは 0 件")
	}
}

// qHistoryDB は history を 1 つの会話として入れた data.sqlite3（conversations_v2）を作る。
func qHistoryDB(t *testing.T, history []any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v, err := json.Marshal(map[string]any{"conversation_id": "conv-x", "model_info": map[string]any{"model_id": "m-conv"}, "history": history})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE conversations_v2 (key TEXT, conversation_id TEXT, value TEXT, created_at INTEGER, updated_at INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO conversations_v2 VALUES (?, ?, ?, ?, ?)`, "/Users/me/app", "conv-x", string(v), nil, nil); err != nil {
		t.Fatal(err)
	}
	return path
}

// CLI が人の代わりに入れた発言（Ctrl+C の中断・「n」で断ったとき・入力なしの --resume）は、プロンプトに数えない。
// ツールの結果のターンは user の時刻がないので、依頼を送った時刻で数える。モデルは request_metadata があるものだけ数える。
func TestQStoreSyntheticEntries(t *testing.T) {
	t0 := time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC)
	at := func(sec int) string { return t0.Add(time.Duration(sec) * time.Second).Format(time.RFC3339) }
	ms := func(sec int) int64 { return t0.Add(time.Duration(sec) * time.Second).UnixMilli() }
	meta := func(start, end int) map[string]any {
		return map[string]any{"model_id": "m-real", "request_start_timestamp_ms": ms(start), "stream_end_timestamp_ms": ms(end), "response_size": 120}
	}
	toolUse := func(text, name string) map[string]any {
		return map[string]any{"ToolUse": map[string]any{"content": text, "tool_uses": []any{map[string]any{"id": "t", "name": name, "args": map[string]any{}}}}}
	}
	response := func(text string) map[string]any { return map[string]any{"Response": map[string]any{"content": text}} }
	prompt := func(text string, sec int) map[string]any {
		u := map[string]any{"content": map[string]any{"Prompt": map[string]any{"prompt": text}}}
		if sec >= 0 {
			u["timestamp"] = at(sec)
		}
		return u
	}
	cancelled := func(text string, sec int) map[string]any {
		return map[string]any{"timestamp": at(sec), "content": map[string]any{"CancelledToolUses": map[string]any{"prompt": text, "tool_use_results": []any{}}}}
	}
	results := map[string]any{"content": map[string]any{"ToolUseResults": map[string]any{"tool_use_results": []any{}}}} // timestamp なし
	history := []any{
		map[string]any{"user": prompt("テストを直して", 0), "assistant": toolUse("見ます", "fs_read"), "request_metadata": meta(1, 3)},
		map[string]any{"user": results, "assistant": toolUse("", "execute_bash"), "request_metadata": meta(60, 62)},
		map[string]any{"user": cancelled("The user interrupted the tool execution.", 120), "assistant": response("Tool uses were interrupted, waiting for the next user prompt")},
		map[string]any{"user": prompt("続けて", 200), "assistant": toolUse("", "execute_bash"), "request_metadata": meta(201, 203)},
		map[string]any{"user": cancelled("I deny this tool request. Ask a follow up question clarifying the expected action", 300), "assistant": response("どうしますか"), "request_metadata": meta(301, 305)},
		map[string]any{"user": prompt("In a few words, summarize our conversation so far.", 3600), "assistant": response("要約です"), "request_metadata": meta(3601, 3603)},
		map[string]any{"user": prompt("MCP のプロンプト", -1), "assistant": response("MCP の応答")}, // /prompts が入れた行（時刻も request_metadata もない）
		map[string]any{"user": cancelled("Tool use with execute_bash was rejected because the arguments supplied were forbidden", 4000), "assistant": response("別の方法にします"), "request_metadata": meta(4001, 4003)},
		map[string]any{"user": prompt("[SYSTEM NOTE: This is an automated request, not from the user]\n\n Read the TODO list contents below", 4100), "assistant": toolUse("", "todo_list"), "request_metadata": meta(4101, 4103)},
		map[string]any{"user": prompt("ありがとう", 4200), "assistant": response("どういたしまして"), "request_metadata": meta(4201, 4203)},
	}
	bs := load(t, &QStore{Label: "Amazon Q", DB: qHistoryDB(t, history)})
	if len(bs) != 1 {
		t.Fatalf("会話の数 = %d, want 1", len(bs))
	}
	b := bs[0]
	var texts []string
	for _, p := range b.Prompts {
		texts = append(texts, p.Text)
	}
	if strings.Join(texts, "|") != "テストを直して|続けて|ありがとう" {
		t.Errorf("プロンプト = %q（CLI が入れた文は数えない）", texts)
	}
	if b.Interrupts != 1 || len(b.InterruptTS) != 1 || b.InterruptTS[0] != float64(t0.Unix()+120) {
		t.Errorf("中断 = %d %v, want 1 回（Ctrl+C の時刻）", b.Interrupts, b.InterruptTS)
	}
	var notes []string
	for _, n := range b.Notes {
		notes = append(notes, n.Kind+":"+strings.SplitN(n.Text, " ", 2)[0])
	}
	if got := strings.Join(notes, "|"); got != "meta:I|meta:In|other:MCP|meta:Tool|meta:[SYSTEM" {
		t.Errorf("Notes = %q（断ったとき・--resume・MCP の /prompts・禁じた引数で CLI が断ったとき・/todos resume の文）", got)
	}
	var toolTimes []float64
	for _, m := range b.Measures {
		if m.T == nil {
			t.Errorf("時刻のない観測: %+v", m)
			continue
		}
		if m.Key == "tool_calls" {
			toolTimes = append(toolTimes, *m.T)
		}
	}
	if len(toolTimes) != 4 || toolTimes[1] != float64(t0.Unix()+60) {
		t.Errorf("ツール呼び出しの時刻 = %v（結果のターンは request_start_timestamp_ms）", toolTimes)
	}
	f := b.Finish(15)
	if len(f.Models) != 1 || f.Models[0][0] != "m-real" {
		t.Errorf("モデル = %v（request_metadata のない行は数えない）", f.Models)
	}
	if r := f.Prompts[0].Reply; r == nil || r.Text != "見ます" {
		t.Errorf("最初の依頼の応答 = %+v（中断の決まった文は応答にしない）", r)
	}
	if r := f.Prompts[1].Reply; r == nil || r.Text != "別の方法にします" {
		t.Errorf("2 つ目の依頼の応答 = %+v（--resume の要約は付けない。禁じた引数で断ったあとの応答は付く）", r)
	}
	if r := f.Prompts[2].Reply; r == nil || r.Text != "どういたしまして" {
		t.Errorf("3 つ目の依頼の応答 = %+v", r)
	}
}

// 古い [user, assistant] の形には時刻も request_metadata もないことがあるが、人が打ったプロンプトとして数える。
func TestQStoreLegacyEntryWithoutTimestamp(t *testing.T) {
	history := []any{
		[]any{map[string]any{"content": map[string]any{"Prompt": map[string]any{"prompt": "古い形"}}}, map[string]any{"Response": map[string]any{"content": "ok"}}},
	}
	b := load(t, &QStore{Label: "Amazon Q", DB: qHistoryDB(t, history)})[0]
	if len(b.Prompts) != 1 || b.Prompts[0].Text != "古い形" || len(b.Notes) != 0 {
		t.Errorf("プロンプト = %+v, Notes = %+v", b.Prompts, b.Notes)
	}
}

// qPromptConv は、依頼 texts を 1 つずつ入れた会話の JSON。
func qPromptConv(t *testing.T, id string, texts ...string) string {
	t.Helper()
	var history []any
	for i, text := range texts {
		history = append(history, map[string]any{
			"user":             map[string]any{"timestamp": time.Date(2026, 9, 29, 1, i, 0, 0, time.UTC).Format(time.RFC3339), "content": map[string]any{"Prompt": map[string]any{"prompt": text}}},
			"assistant":        map[string]any{"Response": map[string]any{"content": "ok"}},
			"request_metadata": map[string]any{"model_id": "m"},
		})
	}
	v, err := json.Marshal(map[string]any{"conversation_id": id, "history": history})
	if err != nil {
		t.Fatal(err)
	}
	return string(v)
}

// /load で同じ会話が別のフォルダの行にも入る。どちらの表でも、いちばん新しく保存した行だけを読む。
func TestQStoreSameConversationUnderSeveralKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE conversations_v2 (key TEXT, conversation_id TEXT, value TEXT, created_at INTEGER, updated_at INTEGER, PRIMARY KEY (key, conversation_id))`,
		`CREATE TABLE conversations (key TEXT PRIMARY KEY, value TEXT)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	// v2: 新しいほう（/new）を先に入れ、古いほう（/old）を後に入れる（rowid の順と updated_at の順を逆にする）
	ins2 := `INSERT INTO conversations_v2 VALUES (?, ?, ?, ?, ?)`
	if _, err := db.Exec(ins2, "/new", "v2", qPromptConv(t, "v2", "一", "二", "三"), 1000, 3000); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ins2, "/old", "v2", qPromptConv(t, "v2", "一"), 1000, 2000); err != nil {
		t.Fatal(err)
	}
	// v1: CLI と同じく INSERT OR REPLACE。/a に保存し、/load して /b に保存し、最後に /a の行をもう一度保存する
	ins1 := `INSERT OR REPLACE INTO conversations (key, value) VALUES (?, ?)`
	for _, kv := range [][2]string{
		{"/a", qPromptConv(t, "v1", "一")},
		{"/b", qPromptConv(t, "v1", "一", "二")},
		{"/a", qPromptConv(t, "v1", "一", "二", "三")},
	} {
		if _, err := db.Exec(ins1, kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}
	bs := load(t, &QStore{Label: "Kiro CLI (SQLite)", DB: path})
	if len(bs) != 2 {
		t.Fatalf("会話の数 = %d, want 2（同じ会話 ID は 1 つ）", len(bs))
	}
	for _, id := range []string{"v1", "v2"} {
		if b := find(bs, id); b == nil || len(b.Prompts) != 3 {
			t.Errorf("%s = %+v（いちばん新しく保存した行を読む）", id, b)
		}
	}
	if b := find(bs, "v2"); b != nil && b.Project != "/new" {
		t.Errorf("v2 のフォルダ = %q, want /new", b.Project)
	}
	if b := find(bs, "v1"); b != nil && b.Project != "/a" {
		t.Errorf("v1 のフォルダ = %q, want /a", b.Project)
	}
}

// 読めない行があっても残りは読み、エラーを返す（「計測の状態」に出す）。
func TestQStoreRowErrorIsReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE conversations (key TEXT, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO conversations VALUES (?, ?), (?, NULL)`, "/ok", qPromptConv(t, "ok", "一"), "/broken"); err != nil {
		t.Fatal(err)
	}
	var out []*core.Builder
	err = (&QStore{Label: "Amazon Q", DB: path}).Load(func(b *core.Builder) { out = append(out, b) })
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("エラー = %v（読めなかった行を知らせる）", err)
	}
	if len(out) != 1 || out[0].ID != "ok" {
		t.Errorf("会話 = %d 件（読めた行は読む）", len(out))
	}
}

// qConvDB は conv を 1 つの会話として入れた data.sqlite3（conversations_v2）を作る。
func qConvDB(t *testing.T, conv map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v, err := json.Marshal(conv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE conversations_v2 (key TEXT, conversation_id TEXT, value TEXT, created_at INTEGER, updated_at INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO conversations_v2 VALUES (?, ?, ?, ?, ?)`, "/Users/me/app", "conv-x", string(v), nil, nil); err != nil {
		t.Fatal(err)
	}
	return path
}

// コンテキストの使用率は CLI と同じ見積もり（token_counter.rs: 文字数 ÷ 4 を 10 単位に丸める）で、
// 保存された履歴（user・assistant）と文脈の長さ（context_message_length）を、model_info.context_window_tokens で割る。
func TestQStoreContextUsage(t *testing.T) {
	t0 := time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC)
	history := []any{
		map[string]any{ // 200 + 400 + 400 = 1000 文字
			"user":      map[string]any{"additional_context": strings.Repeat("b", 200), "timestamp": t0.Format(time.RFC3339), "content": map[string]any{"Prompt": map[string]any{"prompt": strings.Repeat("a", 400)}}},
			"assistant": map[string]any{"Response": map[string]any{"message_id": "m1", "content": strings.Repeat("c", 400)}},
		},
		map[string]any{ // 結果 800 + JSON の値 100 + 1 + 1 + 1、応答 96 + ツール入力 100 = 1099 文字（user の時刻はないので依頼を送った時刻）
			"user": map[string]any{"content": map[string]any{"ToolUseResults": map[string]any{"tool_use_results": []any{
				map[string]any{"tool_use_id": "t1", "status": "Success", "content": []any{
					map[string]any{"Text": strings.Repeat("d", 800)},
					map[string]any{"Json": map[string]any{"k": strings.Repeat("e", 100), "n": 1, "arr": []any{true, nil}}},
				}},
			}}}},
			"assistant":        map[string]any{"ToolUse": map[string]any{"content": strings.Repeat("f", 96), "tool_uses": []any{map[string]any{"id": "t2", "name": "fs_read", "args": map[string]any{"path": strings.Repeat("g", 100)}}}}},
			"request_metadata": map[string]any{"request_start_timestamp_ms": t0.Add(time.Minute).UnixMilli()},
		},
	}
	conv := map[string]any{"conversation_id": "conv-x", "model_info": map[string]any{"model_id": "m", "context_window_tokens": 1000}, "context_message_length": 360, "history": history}
	b := load(t, &QStore{Label: "Amazon Q", DB: qConvDB(t, conv)})[0]
	var used []float64
	for _, m := range b.Measures {
		if m.Key == "context_used" {
			if m.T == nil {
				t.Errorf("時刻のない観測: %+v", m)
			}
			used = append(used, m.V)
		}
	}
	// 文脈 360 + 決まった応答 154 = 514。1 つ目まで 1514 文字 → 380 トークン、2 つ目まで 2613 文字 → 650 トークン
	if len(used) != 2 || used[0] != 0.38 || used[1] != 0.65 {
		t.Errorf("使用率 = %v, want [0.38 0.65]", used)
	}
	n := nativeOf(b.Finish(15))
	if v := n["コンテキストの最大使用率（見積もり）"]; v.V != 65 {
		t.Errorf("コンテキストの最大使用率 = %+v, want 65%%", v)
	}
	if v := n["コンテキストの上限"]; v.V != 1000 || v.N != 1 {
		t.Errorf("コンテキストの上限 = %+v, want 1000", v)
	}

	// model_info がなければ CLI と同じく 200,000 トークン。上限を超えた見積もりは 100%
	conv = map[string]any{"conversation_id": "conv-x", "history": history[:1]}
	n = nativeOf(load(t, &QStore{Label: "Amazon Q", DB: qConvDB(t, conv)})[0].Finish(15))
	if n["コンテキストの上限"].V != 200000 || (n["コンテキストの最大使用率（見積もり）"].V < 0.12 || n["コンテキストの最大使用率（見積もり）"].V > 0.13) {
		t.Errorf("model_info なし = %+v", n)
	}
	conv = map[string]any{"conversation_id": "conv-x", "model_info": map[string]any{"model_id": "m", "context_window_tokens": 10}, "history": history[:1]}
	if n = nativeOf(load(t, &QStore{Label: "Amazon Q", DB: qConvDB(t, conv)})[0].Finish(15)); n["コンテキストの最大使用率（見積もり）"].V != 100 {
		t.Errorf("上限を超えた見積もり = %+v", n)
	}
}

// コンパクションで最近のターンを残すと、先頭に残ったツールの結果（ToolUseResults）を CLI が Prompt に書き換える
// （conversation.rs の enforce_conversation_invariants と message.rs の replace_content_with_tool_use_results）。
// user の時刻はツールの結果のまま（なし）で、request_metadata はある。人のプロンプトではないので数えない。
func TestQStoreToolResultsRewrittenAsPrompt(t *testing.T) {
	t0 := time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC)
	ms := func(sec int) int64 { return t0.Add(time.Duration(sec) * time.Second).UnixMilli() }
	meta := func(start, end int) map[string]any {
		return map[string]any{"model_id": "m-real", "request_start_timestamp_ms": ms(start), "stream_end_timestamp_ms": ms(end)}
	}
	history := []any{
		map[string]any{ // 書き換えられたツールの結果（時刻なし・request_metadata あり）
			"user":             map[string]any{"content": map[string]any{"Prompt": map[string]any{"prompt": "total 8\ndrwxr-xr-x  2 me  staff"}}},
			"assistant":        map[string]any{"Response": map[string]any{"content": "ファイルは 2 つです"}},
			"request_metadata": meta(10, 12),
		},
		map[string]any{
			"user":             map[string]any{"timestamp": t0.Add(60 * time.Second).Format(time.RFC3339), "content": map[string]any{"Prompt": map[string]any{"prompt": "次へ"}}},
			"assistant":        map[string]any{"Response": map[string]any{"content": "はい"}},
			"request_metadata": meta(61, 63),
		},
	}
	b := load(t, &QStore{Label: "Amazon Q", DB: qHistoryDB(t, history)})[0]
	if len(b.Prompts) != 1 || b.Prompts[0].Text != "次へ" {
		t.Errorf("プロンプト = %+v（書き換えられたツールの結果は数えない）", b.Prompts)
	}
	if len(b.Notes) != 1 || b.Notes[0].Kind != "output" || b.Notes[0].T == nil || *b.Notes[0].T != float64(t0.Unix()+10) {
		t.Errorf("Notes = %+v, want ツールの出力 1 つ（依頼を送った時刻）", b.Notes)
	}
	if f := b.Finish(15); len(f.Models) != 1 || f.Models[0][0] != "m-real" {
		t.Errorf("モデル = %v（モデルへの依頼なので数える）", f.Models)
	}
}

// 応答がタイムアウトしたとき CLI が入れる決まった文（mod.rs の RESPONSE_TIMEOUT_CONTENT）は、モデルの応答ではない。
func TestQStoreResponseTimeoutIsNotReply(t *testing.T) {
	t0 := time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC)
	at := func(sec int) string { return t0.Add(time.Duration(sec) * time.Second).Format(time.RFC3339) }
	prompt := func(text string, sec int) map[string]any {
		return map[string]any{"timestamp": at(sec), "content": map[string]any{"Prompt": map[string]any{"prompt": text}}}
	}
	timeout := map[string]any{"Response": map[string]any{"content": "Response timed out - message took too long to generate"}}
	history := []any{
		map[string]any{"user": prompt("大きな表を作って", 0), "assistant": timeout},
		map[string]any{"user": prompt("You took too long to respond - try to split up the work into smaller steps.", 300), "assistant": map[string]any{"Response": map[string]any{"content": "分けて作ります"}}, "request_metadata": map[string]any{"model_id": "m-real"}},
		[]any{prompt("もう一度", 600), timeout},                                // 古い [user, assistant] の形でも同じ
		map[string]any{"user": prompt("小さくして", 900), "assistant": timeout}, // 再試行の前に終わった
	}
	f := load(t, &QStore{Label: "Amazon Q", DB: qHistoryDB(t, history)})[0].Finish(15)
	if len(f.Prompts) != 3 {
		t.Fatalf("プロンプト = %+v", f.Prompts)
	}
	if r := f.Prompts[0].Reply; r == nil || r.Text != "分けて作ります" {
		t.Errorf("応答 = %+v（タイムアウトの決まった文は応答にしない）", r)
	}
	if r := f.Prompts[1].Reply; r != nil {
		t.Errorf("古い形の応答 = %+v（タイムアウトの決まった文は応答にしない）", r)
	}
	if r := f.Prompts[2].Reply; r != nil {
		t.Errorf("最後の応答 = %+v（タイムアウトの決まった文は応答にしない）", r)
	}
	if len(f.Models) != 1 || f.Models[0][0] != "m-real" {
		t.Errorf("モデル = %v（タイムアウトの行は数えない）", f.Models)
	}
}

// JSON の壊れた行は飛ばして残りを読み、エラーを返す（「計測の状態」に出す）。
func TestQStoreBrokenJSONIsReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE conversations (key TEXT, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO conversations VALUES (?, ?), (?, ?)`, "/ok", qPromptConv(t, "ok", "一"), "/broken", `{"history": [`); err != nil {
		t.Fatal(err)
	}
	var out []*core.Builder
	err = (&QStore{Label: "Amazon Q", DB: path}).Load(func(b *core.Builder) { out = append(out, b) })
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("エラー = %v（壊れた行を知らせる）", err)
	}
	if len(out) != 1 || out[0].ID != "ok" {
		t.Errorf("会話 = %d 件（読めた行は読む）", len(out))
	}
}
