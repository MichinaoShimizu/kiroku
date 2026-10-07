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
		map[string]any{"user": prompt("MCP のプロンプト", -1), "assistant": response("MCP の応答")}, // /prompts が入れた行（request_metadata なし）
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
	if strings.Join(texts, "|") != "テストを直して|続けて|MCP のプロンプト" {
		t.Errorf("プロンプト = %q（CLI が入れた文は数えない）", texts)
	}
	if b.Interrupts != 1 || len(b.InterruptTS) != 1 || b.InterruptTS[0] != float64(t0.Unix()+120) {
		t.Errorf("中断 = %d %v, want 1 回（Ctrl+C の時刻）", b.Interrupts, b.InterruptTS)
	}
	if len(b.Notes) != 2 || b.Notes[0].Kind != "meta" || !strings.HasPrefix(b.Notes[0].Text, "I deny") || !strings.HasPrefix(b.Notes[1].Text, "In a few words") {
		t.Errorf("Notes = %+v（断ったときと --resume の文）", b.Notes)
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
	if len(toolTimes) != 3 || toolTimes[1] != float64(t0.Unix()+60) {
		t.Errorf("ツール呼び出しの時刻 = %v（結果のターンは request_start_timestamp_ms）", toolTimes)
	}
	f := b.Finish(15)
	if len(f.Models) != 1 || f.Models[0][0] != "m-real" {
		t.Errorf("モデル = %v（request_metadata のない行は数えない）", f.Models)
	}
	if r := f.Prompts[0].Reply; r == nil || r.Text != "見ます" {
		t.Errorf("最初の依頼の応答 = %+v（中断の決まった文は応答にしない）", r)
	}
	if r := f.Prompts[1].Reply; r == nil || r.Text != "どうしますか" {
		t.Errorf("2 つ目の依頼の応答 = %+v（--resume の要約は付けない）", r)
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
