package source

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeFiles は path（dir からの相対）→ 中身 のファイルを作る。
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Kiro IDE v1.0 以降の作り（sessions/<hash>/sess_<id>/{session.json,messages.jsonl}）に合わせた合成データ。
func TestKiroIDE(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_1/session.json": `{"id": "sess_1", "title": "ログインを直す", "rootPaths": ["/Users/me/app", "/Users/me/lib"], "createdAt": "2026-09-29T01:00:00Z", "modelId": "claude-sonnet-4.5"}`,
		"sessions/abc/sess_1/messages.jsonl": `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": [{"type": "text", "text": "ログインを直して"}]}}
{"timestamp": "2026-09-29T01:02:00Z", "payload": {"type": "tool_call", "toolName": "strReplace", "args": {"path": "src/login.ts"}}}
{"timestamp": "2026-09-29T01:03:00Z", "payload": {"type": "tool_call", "toolName": "readFile", "args": {"path": "src/util.ts"}}}
broken line
{"timestamp": "2026-09-29T01:04:00Z", "payload": {"type": "usage_summary", "promptTurnSummaries": [{"usage": 1.5, "unit": "Credits"}, {"usage": 0.25}, {"usage": 99, "unit": "token"}]}}
{"timestamp": "2026-09-29T01:05:00Z", "payload": {"type": "user", "content": "違う、元に戻して"}}
{"timestamp": "2026-09-29T01:06:00Z", "payload": {"type": "usage_summary", "promptTurnSummaries": [{"usage": 0, "unit": "credit"}]}}
`,
		// id がなければディレクトリ名、作業場所がなければ (unknown)
		"sessions/abc/sess_2/session.json":   `{"createdAt": "2026-09-30T00:00:00Z"}`,
		"sessions/abc/sess_2/messages.jsonl": `{"timestamp": "2026-09-30T00:01:00Z", "payload": {"type": "user", "content": "テストを足して"}}` + "\n",
		// Kiro CLI の置き場所（sessions/cli/<x>/session.json）は Kiro IDE として読まない
		"sessions/cli/x/session.json":   `{"id": "cli-x", "createdAt": "2026-09-30T00:00:00Z"}`,
		"sessions/cli/x/messages.jsonl": "",
	})
	bs := load(t, &KiroIDE{Home: home})
	if len(bs) != 2 {
		t.Fatalf("セッション数 = %d, want 2（cli の下は読まない）", len(bs))
	}
	s := find(bs, "sess_1")
	if s == nil {
		t.Fatal("sess_1 がない")
	}
	if s.Title != "ログインを直す" || s.Project != "/Users/me/app" {
		t.Errorf("title/project = %q %q（workspacePaths がなければ rootPaths の先頭）", s.Title, s.Project)
	}
	if s.File != filepath.Join(home, "sessions", "abc", "sess_1", "messages.jsonl") {
		t.Errorf("file = %q", s.File)
	}
	f := s.Finish(15)
	if f.NPrompts != 2 || len(f.Fix) != 1 {
		t.Errorf("依頼 = %d, 言い直し = %d", f.NPrompts, len(f.Fix))
	}
	if f.Prompts[0].Text != "ログインを直して" {
		t.Errorf("ブロック配列の依頼 = %q", f.Prompts[0].Text)
	}
	if f.Credits != 1.75 || len(s.Credits) != 1 {
		t.Errorf("クレジット = %v（%d 件）, want 1.75（単位の表記揺れ・単位なしは数え、token は数えない。0 は記録しない）", f.Credits, len(s.Credits))
	}
	if len(f.Files) != 1 || f.Files[0] != "src/login.ts" {
		t.Errorf("編集したファイル = %v（読んだだけのファイルは入れない）", f.Files)
	}
	if len(f.Models) != 1 || f.Models[0][0] != "claude-sonnet-4.5" || f.Models[0][1] != 2 {
		t.Errorf("model = %v（usage_summary ごとに 1 回）", f.Models)
	}
	want := map[string]float64{"tool_calls": 2, "credits": 1.75, "turns": 2}
	got := map[string]float64{}
	for _, m := range s.Measures {
		got[m.Key] += m.V
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("measure %s = %v, want %v", k, got[k], v)
		}
	}
	start, _ := time.Parse(time.RFC3339, "2026-09-29T01:00:00Z")
	end, _ := time.Parse(time.RFC3339, "2026-09-29T01:06:00Z")
	if f.Start != float64(start.Unix()) || f.End != float64(end.Unix()) {
		t.Errorf("start/end = %v %v（session.json の createdAt から最後の行まで）", f.Start, f.End)
	}

	s2 := find(bs, "sess_2")
	if s2 == nil {
		t.Fatal("id のない session.json はディレクトリ名で読む")
	}
	if f2 := s2.Finish(15); f2.ProjectPath != "(unknown)" || f2.Title != "テストを足して" {
		t.Errorf("project/title = %q %q", f2.ProjectPath, f2.Title)
	}
}

func TestKiroIDENoHistory(t *testing.T) {
	if bs := load(t, &KiroIDE{Home: filepath.Join(t.TempDir(), "none")}); len(bs) != 0 {
		t.Errorf("履歴がなければ何も出さない: %d 件", len(bs))
	}
}

// Kiro IDE v1.0 より前の作り（workspace-sessions/<ws>/sessions.json + <sessionId>.json）。
func TestKiroIDELegacy(t *testing.T) {
	gs := t.TempDir()
	writeFiles(t, gs, map[string]string{
		"workspace-sessions/d3M=/sessions.json": `[
  {"sessionId": "a", "title": "一覧のタイトル", "dateCreated": 1759100000000, "workspaceDirectory": "/Users/me/ws"},
  {"sessionId": "b", "dateCreated": "2026-09-29T00:00:00Z"},
  {"sessionId": "hidden", "hidden": true, "dateCreated": 1759100000000},
  {"title": "id なし"},
  "壊れた項目"
]`,
		"workspace-sessions/d3M=/a.json": `{"title": "本体のタイトル", "workspacePath": "/Users/me/app", "history": [
  {"message": {"role": "user", "content": "画面を作って"}},
  {"message": {"role": "assistant", "content": "作りました"}},
  {"message": {"role": "user", "content": [{"type": "text", "text": "色を変えて"}]}}
]}`,
		"workspace-sessions/d3M=/b.json":      `{"title": "本体だけのタイトル", "history": []}`,
		"workspace-sessions/d3M=/hidden.json": `{"history": [{"message": {"role": "user", "content": "見せない"}}]}`,
	})
	end := time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(gs, "workspace-sessions", "d3M=", "a.json"), end, end); err != nil {
		t.Fatal(err)
	}
	k := &KiroIDELegacy{Storages: []string{gs, filepath.Join(t.TempDir(), "none")}}
	bs := load(t, k)
	if len(bs) != 2 {
		t.Fatalf("セッション数 = %d, want 2（hidden・id なしは読まない）", len(bs))
	}
	a := find(bs, "a")
	if a == nil {
		t.Fatal("a がない")
	}
	if a.Title != "一覧のタイトル" || a.Project != "/Users/me/app" {
		t.Errorf("title/project = %q %q（一覧のタイトルを優先、作業場所は本体を優先）", a.Title, a.Project)
	}
	f := a.Finish(15)
	if f.NPrompts != 2 || f.Prompts[1].Text != "色を変えて" {
		t.Errorf("依頼 = %+v", f.Prompts)
	}
	if f.Start != 1759100000 || f.End != float64(end.Unix()) {
		t.Errorf("start/end = %v %v（開始は dateCreated、終了はファイルの更新時刻）", f.Start, f.End)
	}
	if b := find(bs, "b"); b == nil || b.Title != "本体だけのタイトル" {
		t.Errorf("一覧にタイトルがなければ本体のタイトル: %+v", b)
	}
	if k.Where() != gs+" / "+k.Storages[1] {
		t.Errorf("where = %q", k.Where())
	}
	if (&KiroIDELegacy{}).Where() != "none" {
		t.Error("置き場所がなければ「none」")
	}
}

// Kiro の応答: Kiro IDE は payload の assistant、Kiro CLI は AssistantMessage の text から、
// 1 つの依頼につき最後の文を拾う。
func TestKiroReplies(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_1/session.json": `{"id": "sess_1", "rootPaths": ["/Users/me/app"], "createdAt": "2026-09-29T01:00:00Z"}`,
		"sessions/abc/sess_1/messages.jsonl": `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": "ログインを直して"}}
{"timestamp": "2026-09-29T01:02:00Z", "payload": {"type": "assistant", "content": [{"type": "text", "text": "見てみます"}]}}
{"timestamp": "2026-09-29T01:03:00Z", "payload": {"type": "tool_call", "toolName": "strReplace", "args": {"path": "src/login.ts"}}}
{"timestamp": "2026-09-29T01:04:00Z", "payload": {"type": "assistant", "content": "直しました"}}
`,
		"sessions/cli/c1.json": `{"id": "c1", "cwd": "/Users/me/app", "created_at": "2026-09-29T02:00:00Z"}`,
		"sessions/cli/c1.jsonl": `{"kind": "Prompt", "timestamp": "2026-09-29T02:01:00Z", "data": {"content": [{"kind": "text", "data": "テストを足して"}]}}
{"kind": "AssistantMessage", "timestamp": "2026-09-29T02:02:00Z", "data": {"content": [{"kind": "text", "data": "足します"}, {"kind": "toolUse", "data": {"name": "fs_write", "input": {"path": "t_test.go"}}}]}}
{"kind": "AssistantMessage", "timestamp": "2026-09-29T02:03:00Z", "data": {"content": [{"kind": "text", "data": "足して通しました"}]}}
`,
	})
	ide := find(load(t, &KiroIDE{Home: home}), "sess_1")
	if ide == nil {
		t.Fatal("sess_1 がない")
	}
	if r := ide.Finish(15).Prompts[0].Reply; r == nil || r.Text != "直しました" {
		t.Errorf("Kiro IDE の応答 = %+v, want 直しました", r)
	}
	cli := find(load(t, &KiroCLI{Home: home}), "c1")
	if cli == nil {
		t.Fatal("c1 がない")
	}
	if r := cli.Finish(15).Prompts[0].Reply; r == nil || r.Text != "足して通しました" {
		t.Errorf("Kiro CLI の応答 = %+v, want 足して通しました（ツール呼び出しは入れない）", r)
	}
}

// 実際の Kiro CLI の会話は、依頼（Prompt）の時刻が data.meta.timestamp にあり、応答（AssistantMessage）の行には時刻がない。
// 応答は、その回の終わり（user_turn_metadatas の end_timestamp）、なければ直前の依頼の時刻で、その依頼に結びつける。
// Kiro IDE は、考えている途中の文（operationType: Reasoning）を応答にせず、入れ子の content の文も拾う。
func TestKiroRepliesWithoutTimes(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_2/session.json": `{"id": "sess_2", "rootPaths": ["/Users/me/app"], "createdAt": "2026-09-29T01:00:00Z"}`,
		"sessions/abc/sess_2/messages.jsonl": `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": "ログインを直して"}}
{"timestamp": "2026-09-29T01:02:00Z", "payload": {"type": "assistant", "content": [{"content": [{"text": "直しました"}]}]}}
{"timestamp": "2026-09-29T01:03:00Z", "payload": {"type": "assistant", "operationType": "Reasoning", "content": "次はテストを見るべきか考える"}}
`,
		// 2 回目の依頼は、その回の終わりの時刻がない（直前の依頼の時刻を使う）
		"sessions/cli/c2.json": `{"session_id": "c2", "cwd": "/Users/me/app", "created_at": "2026-09-29T02:00:00Z",
  "session_state": {"conversation_metadata": {"user_turn_metadatas": [{"end_timestamp": "2026-09-29T02:05:00Z"}, {}]}}}`,
		"sessions/cli/c2.jsonl": `{"version": "v1", "kind": "Prompt", "data": {"message_id": "m1", "content": [{"kind": "text", "data": "テストを足して"}], "meta": {"timestamp": 1790647260}}}
{"version": "v1", "kind": "AssistantMessage", "data": {"message_id": "m2", "content": [{"kind": "text", "data": "足します"}, {"kind": "toolUse", "data": {"name": "fs_write", "input": {"path": "t_test.go"}}}]}}
{"version": "v1", "kind": "ToolResults", "data": {"message_id": "m3", "content": [], "results": {}}}
{"version": "v1", "kind": "AssistantMessage", "data": {"message_id": "m4", "content": [{"kind": "text", "data": "足して通しました"}]}}
{"version": "v1", "kind": "Prompt", "data": {"message_id": "m5", "content": [{"kind": "text", "data": "コミットして"}], "meta": {"timestamp": 1790647560}}}
{"version": "v1", "kind": "AssistantMessage", "data": {"message_id": "m6", "content": [{"kind": "text", "data": "コミットしました"}]}}
`,
	})
	ide := find(load(t, &KiroIDE{Home: home}), "sess_2")
	if ide == nil {
		t.Fatal("sess_2 がない")
	}
	if r := ide.Finish(15).Prompts[0].Reply; r == nil || r.Text != "直しました" {
		t.Errorf("Kiro IDE の応答 = %+v, want 直しました（考えている途中の文は入れない）", r)
	}
	cli := find(load(t, &KiroCLI{Home: home}), "c2")
	if cli == nil {
		t.Fatal("c2 がない")
	}
	ps := cli.Finish(15).Prompts
	if len(ps) != 2 {
		t.Fatalf("依頼の数 = %d, want 2", len(ps))
	}
	if r := ps[0].Reply; r == nil || r.Text != "足して通しました" || r.T == nil || *r.T != 1790647500 {
		t.Errorf("1 回目の応答 = %+v, want 足して通しました（時刻はその回の終わり 02:05）", r)
	}
	if r := ps[1].Reply; r == nil || r.Text != "コミットしました" {
		t.Errorf("2 回目の応答 = %+v, want コミットしました（時刻は依頼の時刻）", r)
	}
}

// Kiro IDE v1.0 以降: session_metadata の contextUsage（参考実装 codeburn のテストデータの形）からコンテキストの最大使用率、
// usage_summary の requestIds（参考実装 kiro-history の形）からモデルへのリクエストの数。どちらもなければ出さない。
func TestKiroIDEContextAndRequests(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_1/session.json": `{"id": "sess_1", "createdAt": "2026-09-29T01:00:00Z", "modelId": "auto"}`,
		"sessions/abc/sess_1/messages.jsonl": `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": "直して"}}
{"timestamp": "2026-09-29T01:02:00Z", "payload": {"type": "session_metadata", "key": "contextUsage", "value": {"usagePercentage": 12.5}, "executionId": "e1"}}
{"timestamp": "2026-09-29T01:02:00Z", "payload": {"type": "usage_summary", "promptTurnSummaries": [{"unit": "credit", "usage": 1}], "requestIds": ["r1", "r2", "r3"], "executionId": "e1"}}
{"timestamp": "2026-09-29T01:03:00Z", "payload": {"type": "session_metadata", "key": "contextUsage", "value": {"usagePercentage": 40}}}
{"timestamp": "2026-09-29T01:03:00Z", "payload": {"type": "session_metadata", "key": "contextUsage", "value": {"usagePercentage": 250}}}
{"timestamp": "2026-09-29T01:03:00Z", "payload": {"type": "session_metadata", "key": "contextUsage", "value": {"usagePercentage": "90"}}}
{"timestamp": "2026-09-29T01:03:00Z", "payload": {"type": "session_metadata", "key": "otherKey", "value": {"usagePercentage": 80}}}
{"timestamp": "2026-09-29T01:04:00Z", "payload": {"type": "usage_summary", "promptTurnSummaries": [{"unit": "credit", "usage": 1}], "requestIds": []}}
{"timestamp": "2026-09-29T01:05:00Z", "payload": {"type": "usage_summary", "promptTurnSummaries": [{"unit": "credit", "usage": 1}], "requestIds": "r9"}}
`,
		// どちらの項目もない会話
		"sessions/abc/sess_2/session.json":   `{"id": "sess_2", "createdAt": "2026-09-29T01:00:00Z"}`,
		"sessions/abc/sess_2/messages.jsonl": `{"timestamp": "2026-09-29T01:04:00Z", "payload": {"type": "usage_summary", "promptTurnSummaries": [{"unit": "credit", "usage": 1}]}}` + "\n",
	})
	bs := load(t, &KiroIDE{Home: home})
	n := nativeOf(find(bs, "sess_1").Finish(15))
	if v := n["コンテキストの最大使用率"]; v.V != 40 || v.N != 2 {
		t.Errorf("コンテキストの最大使用率 = %+v, want 40%%（0〜100 の数だけ。ほかの key は読まない）", v)
	}
	if v := n["モデルへのリクエスト"]; v.V != 3 || v.N != 2 {
		t.Errorf("モデルへのリクエスト = %+v, want 3（並びでない requestIds は数えない）", v)
	}
	n2 := nativeOf(find(bs, "sess_2").Finish(15))
	if _, ok := n2["コンテキストの最大使用率"]; ok {
		t.Errorf("contextUsage がないのに出ている: %+v", n2)
	}
	if _, ok := n2["モデルへのリクエスト"]; ok {
		t.Errorf("requestIds がないのに出ている: %+v", n2)
	}
}

// Kiro IDE v1.0 より前の実行ファイル（<globalStorage>/kiro.kiroagent/<32 桁の 16 進>/<セッション>/<実行 ID>。参考実装 codeburn の形）から、
// クレジット・モデル・実行の時刻を足す。会話とは history の executionId か、実行ファイルの chatSessionId で結ぶ。
func TestKiroIDELegacyExecutions(t *testing.T) {
	gs := t.TempDir()
	ws := "0123456789abcdef0123456789abcdef"
	writeFiles(t, gs, map[string]string{
		"workspace-sessions/d3M=/sessions.json": `[{"sessionId": "a", "dateCreated": 1790643600000}, {"sessionId": "b", "dateCreated": 1790643600000}]`,
		"workspace-sessions/d3M=/a.json": `{"selectedModel": "claude-sonnet-4.5", "history": [
  {"message": {"role": "user", "content": "画面を作って"}},
  {"message": {"role": "assistant", "content": "On it."}, "executionId": "exec-1"},
  {"message": {"role": "user", "content": "色を変えて"}},
  {"message": {"role": "user", "content": "やっぱり青で"}},
  {"message": {"role": "assistant", "content": "On it."}, "executionId": "exec-2"},
  {"message": {"role": "user", "content": "ありがとう"}}
]}`,
		"workspace-sessions/d3M=/b.json": `{"selectedModel": "auto", "history": [{"message": {"role": "user", "content": "テストを足して"}}]}`,
		// history の executionId で結ぶ実行（ミリ秒の時刻、metadata の modelId）
		ws + "/s1/exec-1": `{"executionId": "exec-1", "chatSessionId": "other", "startTime": 1790643660000, "endTime": 1790643720000, "metadata": {"modelId": "claude-opus-4.5"},
  "usageSummary": [{"usage": 0.5, "unit": "credit", "usedTools": ["readFile"]}, {"usage": 1.0}, {"usage": 9, "unit": "token"}], "context": {"messages": []}}`,
		// モデルのない実行は会話で選んでいたモデル。時刻は metadata の秒
		ws + "/s1/exec-2": `{"executionId": "exec-2", "metadata": {"startTime": 1790644000, "endTime": 1790644100}, "usageSummary": [{"usage": 2, "unit": "credit"}]}`,
		// chatSessionId で結ぶ実行（history に executionId がない会話 b）
		ws + "/s2/exec-3": `{"executionId": "exec-3", "chatSessionId": "b", "startTime": 1790650000000, "endTime": 1790650060000, "modelId": "claude-haiku-4.5", "usageSummary": [{"usage": 0.25, "unit": "credit"}]}`,
		// 実行の一覧・JSON でないもの・拡張子のあるもの・32 桁の 16 進でないフォルダは読まない
		ws + "/s2/index":                               `{"executions": [{"executionId": "exec-3"}], "chatSessionId": "b", "usageSummary": [{"usage": 50}]}`,
		ws + "/s2/blob":                                "not json",
		ws + "/s2/exec-4.json":                         `{"executionId": "exec-4", "chatSessionId": "b", "usageSummary": [{"usage": 50}]}`,
		"not-a-workspace/s3/exec-5":                    `{"executionId": "exec-5", "chatSessionId": "b", "usageSummary": [{"usage": 50}]}`,
		"0123456789abcdef0123456789abcdeg/s3/exec-6":   `{"executionId": "exec-6", "chatSessionId": "b", "usageSummary": [{"usage": 50}]}`,
		"workspace-sessions/d3M=/sub/exec-7":           `{"executionId": "exec-7", "chatSessionId": "b", "usageSummary": [{"usage": 50}]}`,
		ws + "/exec-8":                                 `{"executionId": "exec-8", "chatSessionId": "b", "usageSummary": [{"usage": 50}]}`,
		"outside/exec-9":                               `{"executionId": "exec-9", "chatSessionId": "b", "usageSummary": [{"usage": 50}]}`,
		"fedcba9876543210fedcba9876543210/.hidden/x":   `{"executionId": "exec-10", "chatSessionId": "b", "usageSummary": [{"usage": 50}]}`,
		"fedcba9876543210fedcba9876543210/s4/.exec-11": `{"executionId": "exec-11", "chatSessionId": "b", "usageSummary": [{"usage": 50}]}`,
	})
	// シンボリックリンクの実行ファイルとフォルダは読まない（ほかの場所のファイルを読ませない）
	if err := os.Symlink(filepath.Join(gs, "outside", "exec-9"), filepath.Join(gs, ws, "s2", "link")); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink(filepath.Join(gs, "outside"), filepath.Join(gs, ws, "linkdir")); err != nil {
		t.Skip(err)
	}
	bs := load(t, &KiroIDELegacy{Storages: []string{gs}})
	a := find(bs, "a")
	if a == nil {
		t.Fatal("a がない")
	}
	f := a.Finish(15)
	if f.Credits != 3.5 {
		t.Errorf("a のクレジット = %v, want 3.5（usageSummary の credit だけ）", f.Credits)
	}
	models := map[any]any{}
	for _, m := range f.Models {
		models[m[0]] = m[1]
	}
	if len(models) != 2 || models["claude-opus-4.5"] != 1 || models["claude-sonnet-4.5"] != 1 {
		t.Errorf("a のモデル = %v（実行ファイルのモデル、なければ selectedModel）", f.Models)
	}
	want := []float64{1790643660, 1790644000, 1790644000, 1790643600}
	for i, p := range f.Prompts {
		if p.T == nil || *p.T != want[i] {
			t.Errorf("依頼 %d の時刻 = %v, want %v（あとに続く実行の開始。なければ dateCreated）", i, p.T, want[i])
		}
	}
	if f.Start != 1790643600 {
		t.Errorf("a の開始 = %v", f.Start)
	}
	if n := nativeOf(f); n["クレジット"].V != 3.5 || n["ターン"].V != 2 {
		t.Errorf("a の参考指標 = %+v", f.Native)
	}
	b := find(bs, "b")
	if b == nil {
		t.Fatal("b がない")
	}
	fb := b.Finish(15)
	if fb.Credits != 0.25 || len(fb.Models) != 1 || fb.Models[0][0] != "claude-haiku-4.5" {
		t.Errorf("b のクレジット = %v, モデル = %v（chatSessionId で結ぶ。ほかのファイルは読まない）", fb.Credits, fb.Models)
	}
	if fb.End < 1790650060 {
		t.Errorf("b の終わり = %v, want 実行の終わり以降", fb.End)
	}
}

// 実行ファイルがない会話は、会話で選んでいたモデル（selectedModel）を 1 回数える。
func TestKiroIDELegacySelectedModel(t *testing.T) {
	gs := t.TempDir()
	writeFiles(t, gs, map[string]string{
		"workspace-sessions/d3M=/sessions.json": `[{"sessionId": "a", "dateCreated": 1790643600000}]`,
		"workspace-sessions/d3M=/a.json":        `{"selectedModel": "claude-sonnet-4.5", "history": [{"message": {"role": "user", "content": "画面を作って"}}]}`,
	})
	f := load(t, &KiroIDELegacy{Storages: []string{gs}})[0].Finish(15)
	if len(f.Models) != 1 || f.Models[0][0] != "claude-sonnet-4.5" || f.Credits != 0 {
		t.Errorf("モデル = %v, クレジット = %v", f.Models, f.Credits)
	}
}
