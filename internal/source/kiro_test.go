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
