package source

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
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
	if want := "cd /Users/me/app && kiro-cli chat --v3 --resume-id sess_1"; s.Resume != want {
		t.Errorf("resume = %q, want %q", s.Resume, want)
	}
	// 作業場所がわからない会話には、再開のコマンドを出さない
	if s2 := find(bs, "sess_2"); s2 == nil || s2.Resume != "" {
		t.Errorf("sess_2 = %+v, want 再開のコマンドなし", s2)
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

// messages.jsonl・<id>.jsonl がない会話（作りかけ・消えたもの）は、読めないファイルとして数えない。
func TestKiroMissingMessages(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_1/session.json": `{"id": "sess_1", "createdAt": "2026-09-29T01:00:00Z"}`,
		"sessions/cli/c1.json": `{"session_id": "c1", "created_at": "2026-09-29T02:00:00Z",
  "session_state": {"conversation_metadata": {"user_turn_metadatas": [{"end_timestamp": "2026-09-29T02:05:00Z", "metering_usage": [{"value": 0.5, "unit": "credit"}]}]}}}`,
	})
	if bs := load(t, &KiroIDE{Home: home}); len(bs) != 1 || len(bs[0].Prompts) != 0 {
		t.Errorf("Kiro IDE: %d 件（会話は残し、依頼はなし）", len(bs))
	}
	bs := load(t, &KiroCLI{Home: home})
	if len(bs) != 1 || bs[0].Finish(15).Credits != 0.5 {
		t.Errorf("Kiro CLI: %d 件（会話は残し、メタのクレジットは数える）", len(bs))
	}
}

// 共有の保存場所（sessions/<bucket>/sess_*。Kiro IDE v1.0 以降と Kiro CLI V3）の会話は "Kiro IDE / CLI (V3)"、
// V2 の sessions/cli/ の会話は "Kiro CLI (V2)" として出す。
func TestKiroSourceLabels(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/0123456789abcdef/sess_3f2a/session.json":   `{"schemaVersion": 1, "id": "sess_3f2a", "workspacePaths": ["/Users/me/app"], "createdAt": "2026-10-01T01:00:00Z"}`,
		"sessions/0123456789abcdef/sess_3f2a/messages.jsonl": `{"timestamp": "2026-10-01T01:01:00Z", "payload": {"type": "user", "content": "ビルドを直して"}}` + "\n",
		"sessions/cli/c1.json": `{"session_id": "c1", "cwd": "/Users/me/app", "created_at": "2026-10-01T02:00:00Z",
  "session_state": {"conversation_metadata": {"user_turn_metadatas": [{"end_timestamp": "2026-10-01T02:05:00Z"}]}}}`,
	})
	for _, c := range []struct {
		src  Source
		want string
	}{
		{&KiroIDE{Home: home}, "Kiro IDE / CLI (V3)"},
		{&KiroCLI{Home: home}, "Kiro CLI (V2)"},
	} {
		if c.src.Name() != c.want {
			t.Errorf("Name() = %q, want %q", c.src.Name(), c.want)
		}
		if _, ok := core.AgentOf(c.want); !ok {
			t.Errorf("core.Agents に %q がない", c.want)
		}
		bs := load(t, c.src)
		if len(bs) != 1 {
			t.Fatalf("%s: %d 件, want 1", c.want, len(bs))
		}
		if bs[0].Source != c.want || bs[0].Finish(15).Source != c.want {
			t.Errorf("source = %q, want %q", bs[0].Source, c.want)
		}
	}
}

// Kiro IDE v1.0 以降: 中身が「...」だけの応答の行は書いている途中の仮の文なので、応答にしない。
func TestKiroIDEPlaceholderReply(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_1/session.json": `{"id": "sess_1", "createdAt": "2026-09-29T01:00:00Z"}`,
		"sessions/abc/sess_1/messages.jsonl": `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": "ログインを直して"}}
{"timestamp": "2026-09-29T01:02:00Z", "payload": {"type": "assistant", "content": "直しました"}}
{"timestamp": "2026-09-29T01:03:00Z", "payload": {"type": "assistant", "content": "..."}}
`,
	})
	s := find(load(t, &KiroIDE{Home: home}), "sess_1")
	if s == nil {
		t.Fatal("sess_1 がない")
	}
	if r := s.Finish(15).Prompts[0].Reply; r == nil || r.Text != "直しました" {
		t.Errorf("応答 = %+v, want 直しました（「...」は入れない）", r)
	}
}

// Kiro CLI V2 から 3.0 へ移した会話の、切り替える前の応答には使える時刻がない（時刻がない、またはその回の外の時刻）。
// 行の順で、直前の依頼の応答にする。時刻がその回の中にある応答は、そのまま使う。
func TestKiroIDERepliesWithoutUsableTime(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_u1/session.json": `{"id": "sess_u1"}`,
		"sessions/abc/sess_u1/messages.jsonl": `{"timestamp": "2026-09-29T10:00:00Z", "payload": {"type": "user", "content": "一"}}
{"payload": {"type": "assistant", "content": "一の答え", "operationType": "Say"}}
{"timestamp": "2026-09-29T10:10:00Z", "payload": {"type": "user", "content": "二"}}
{"timestamp": "2026-09-29T10:59:00Z", "payload": {"type": "assistant", "content": "二の答え", "operationType": "Say"}}
{"timestamp": "2026-09-29T10:20:00Z", "payload": {"type": "user", "content": "三"}}
{"timestamp": "2026-09-29T10:00:00Z", "payload": {"type": "assistant", "content": "三の答え"}}
{"timestamp": "2026-09-29T11:00:00Z", "payload": {"type": "user", "content": "四"}}
{"timestamp": "2026-09-29T11:01:00Z", "payload": {"type": "assistant", "content": "四の答え", "operationType": "Say"}}
`,
	})
	s := find(load(t, &KiroIDE{Home: home}), "sess_u1")
	if s == nil {
		t.Fatal("sess_u1 がない")
	}
	ps := s.Finish(15).Prompts
	want := []string{"一の答え", "二の答え", "三の答え", "四の答え"}
	if len(ps) != len(want) {
		t.Fatalf("依頼 = %d, want %d", len(ps), len(want))
	}
	for i, w := range want {
		if r := ps[i].Reply; r == nil || r.Text != w {
			t.Errorf("%d 番目の応答 = %+v, want %s", i+1, r, w)
		}
	}
}

func TestKiroUserText(t *testing.T) {
	for _, c := range []struct{ in, prompt string }{
		{"直して", "直して"},
		{"## Included Rules\na\n\n\n直して", "直して"},                 // 空行が続いても、最初の「空白でない文字」の前で終わる
		{"前\n## Included Rules\na\n\n## b\n\n直して", "前\n\n\n直して"}, // 「#」で始まる段落は規則の続き
		{"## Included Rules\na\n\n  インデント", ""},                  // 空白で始まる段落も規則の続き
	} {
		if p, _ := kiroUserText(c.in); p != c.prompt {
			t.Errorf("kiroUserText(%q) = %q, want %q", c.in, p, c.prompt)
		}
	}
}

func TestKiroIDENoHistory(t *testing.T) {
	if bs := load(t, &KiroIDE{Home: filepath.Join(t.TempDir(), "none")}); len(bs) != 0 {
		t.Errorf("履歴がなければ何も出さない: %d 件", len(bs))
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

// usage_summary の行がない会話も、会話のモデル（modelId）を 1 回数える。
func TestKiroIDEModelWithoutUsage(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_1/session.json":   `{"id": "sess_1", "createdAt": "2026-09-29T01:00:00Z", "modelId": "claude-sonnet-4.5"}`,
		"sessions/abc/sess_1/messages.jsonl": `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": "画面を作って"}}` + "\n",
	})
	f := load(t, &KiroIDE{Home: home})[0].Finish(15)
	if len(f.Models) != 1 || f.Models[0][0] != "claude-sonnet-4.5" {
		t.Errorf("モデル = %v", f.Models)
	}
}

// Kiro のエージェントが user の行として書く通知とエージェントの依頼は、人の依頼として数えない
// （kiro-cli 2.29.0 の @kiro/agent の見分け方: _meta.kiro.notification・kind・"[notification/…]" で始まる steer・syntheticUserMessageReason）。
// 人の依頼（source のないもの、chat、通知でない steer）は今までどおり数える。負のクレジットは 0 とみなす。
func TestKiroIDENotUserAndCredits(t *testing.T) {
	home := t.TempDir()
	lines := []string{
		`{"timestamp": "2026-10-01T01:01:00Z", "payload": {"type": "user", "content": "ビルドを直して"}}`,
		`{"timestamp": "2026-10-01T01:02:00Z", "payload": {"type": "user", "content": "[notification/info] workflow finished", "source": "steer"}}`,
		`{"timestamp": "2026-10-01T01:03:00Z", "payload": {"type": "user", "content": "task done", "source": "steer", "_meta": {"kiro": {"notification": {"kind": "system-notification"}}}}}`,
		`{"timestamp": "2026-10-01T01:04:00Z", "payload": {"type": "user", "content": "continue", "kind": "x"}}`,
		`{"timestamp": "2026-10-01T01:05:00Z", "payload": {"type": "user", "content": "next step", "_meta": {"kiro": {"syntheticUserMessageReason": "workflow-continuation"}}}}`,
		`{"timestamp": "2026-10-01T01:06:00Z", "payload": {"type": "user", "content": "テストも足して", "source": "steer"}}`,
		`{"timestamp": "2026-10-01T01:07:00Z", "payload": {"type": "user", "content": "README を直して", "source": "chat"}}`,
		`{"timestamp": "2026-10-01T01:08:00Z", "payload": {"type": "usage_summary", "promptTurnSummaries": [{"usage": 2, "unit": "credit"}, {"usage": -5, "unit": "credit"}]}}`,
	}
	writeFiles(t, home, map[string]string{
		"sessions/0123456789abcdef/sess_n1/session.json":   `{"schemaVersion": "1.0.0", "id": "sess_n1", "workspacePaths": ["/Users/me/app"], "createdAt": "2026-10-01T01:00:00Z"}`,
		"sessions/0123456789abcdef/sess_n1/messages.jsonl": strings.Join(lines, "\n") + "\n",
	})
	bs := load(t, &KiroIDE{Home: home})
	if len(bs) != 1 {
		t.Fatalf("%d 件, want 1", len(bs))
	}
	s := bs[0].Finish(15)
	var got []string
	for _, p := range s.Prompts {
		got = append(got, p.Text)
	}
	if want := []string{"ビルドを直して", "テストも足して", "README を直して"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("依頼 = %q, want %q", got, want)
	}
	if c := s.Credits; c != 2 {
		t.Errorf("credits = %v, want 2（負の値は 0）", c)
	}
}

// Kiro のエージェントと同じく、値ではなくキーがあるかで見る（null でも当てはまる）。前の空白は JavaScript の \s の範囲。
// 壊れた・ありえない大きさのクレジットは 0 とみなし、合計が +Inf にならない（JSON に書けず画面全体が出なくなるため）。
func TestKiroNotUserEdges(t *testing.T) {
	for _, c := range []struct {
		line string
		want string
	}{
		{`{"type": "user", "content": "x", "kind": null}`, "notice"},
		{`{"type": "user", "content": "x", "kind": false}`, "notice"},
		{`{"type": "user", "content": "x", "_meta": {"kiro": {"notification": null}}}`, "notice"},
		{`{"type": "user", "content": "x", "_meta": {"kiro": {"syntheticUserMessageReason": null}}}`, "agent"},
		{`{"type": "user", "content": "\u00a0\ufeff[notification/warning] w", "source": "steer"}`, "notice"},
		{`{"type": "user", "content": "[NOTIFICATION/INFO] x", "source": "steer"}`, ""},
		{`{"type": "user", "content": "[notification/debug] x", "source": "steer"}`, ""},
		{`{"type": "user", "content": "[notification/info] x", "source": "chat"}`, ""},
		{`{"type": "user", "content": "x", "_meta": "str"}`, ""},
		{`{"type": "user", "content": "x", "_meta": {"kiro": ["a"]}}`, ""},
	} {
		var p core.Obj
		if err := json.Unmarshal([]byte(c.line), &p); err != nil {
			t.Fatal(err)
		}
		if got := kiroNotUser(p); got != c.want {
			t.Errorf("%s: %q, want %q", c.line, got, c.want)
		}
	}
	if got := creditsOf([]any{map[string]any{"usage": 1.7e308}, map[string]any{"usage": 1.7e308}, map[string]any{"usage": 1.5}}, "unit", "usage"); got != 1.5 {
		t.Errorf("credits = %v, want 1.5（ありえない大きさは 0）", got)
	}
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/cli/c1.json": `{"session_id": "c1", "created_at": "2026-10-05T00:00:00Z", "session_state": {"conversation_metadata": {"user_turn_metadatas": [
  {"end_timestamp": "2026-10-05T00:04:00Z", "metering_usage": [{"value": 1.7e308, "unit": "credit"}, {"value": 1.7e308, "unit": "credit"}, {"value": 2, "unit": "credit"}]}]}}}`,
		"sessions/cli/c1.jsonl": `{"version": "v1", "kind": "Prompt", "data": {"content": [{"kind": "text", "data": "hi"}]}}` + "\n",
	})
	bs := load(t, &KiroCLI{Home: home})
	if len(bs) != 1 {
		t.Fatalf("%d 件, want 1", len(bs))
	}
	s := bs[0].Finish(15)
	if s.Credits != 2 {
		t.Errorf("credits = %v, want 2", s.Credits)
	}
	if _, err := json.Marshal(s); err != nil {
		t.Errorf("セッションを JSON に書けない: %v", err)
	}
}

// Kiro CLI 2.30 からのモデルの切り替え（session_metadata の modelFallback）。scope が turn ならそのターンだけ、
// session なら切り替える前のターンは from、以後は session.json の modelId（切り替え先に書き換わっている）。
func TestKiroIDEModelFallback(t *testing.T) {
	home := t.TempDir()
	us := func(ts, ex string) string {
		return `{"timestamp": "` + ts + `", "payload": {"type": "usage_summary", "promptTurnSummaries": [{"unit": "credit", "usage": 1}], "executionId": "` + ex + `"}}` + "\n"
	}
	fb := func(ts, ex, from, to, scope string) string {
		return `{"timestamp": "` + ts + `", "payload": {"type": "session_metadata", "key": "modelFallback", "executionId": "` + ex + `", "value": {"from": "` + from + `", "to": "` + to + `", "scope": "` + scope + `", "trigger": "availability"}}}` + "\n"
	}
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_t/session.json":   `{"id": "sess_t", "createdAt": "2026-09-29T01:00:00Z", "modelId": "claude-sonnet-4.5"}`,
		"sessions/abc/sess_t/messages.jsonl": `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": "a"}}` + "\n" + us("2026-09-29T01:02:00Z", "e1") + fb("2026-09-29T01:03:00Z", "e2", "claude-sonnet-4.5", "claude-haiku-4.5", "turn") + us("2026-09-29T01:04:00Z", "e2"),
		"sessions/abc/sess_s/session.json":   `{"id": "sess_s", "createdAt": "2026-09-29T01:00:00Z", "modelId": "claude-haiku-4.5"}`,
		"sessions/abc/sess_s/messages.jsonl": `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": "a"}}` + "\n" + us("2026-09-29T01:02:00Z", "e1") + fb("2026-09-29T01:03:00Z", "e2", "claude-sonnet-4.5", "claude-haiku-4.5", "session") + us("2026-09-29T01:04:00Z", "e2"),
	})
	bs := load(t, &KiroIDE{Home: home})
	for _, id := range []string{"sess_t", "sess_s"} {
		got := map[string]float64{}
		for _, m := range find(bs, id).Finish(15).Models {
			got[m[0].(string)] += float64(m[1].(int))
		}
		if got["claude-sonnet-4.5"] != 1 || got["claude-haiku-4.5"] != 1 {
			t.Errorf("%s: モデル = %v, want sonnet 1・haiku 1", id, got)
		}
	}
}

// KIRO_HOME は、Kiro が使う値だけ使う。相対パス・根・ホームとその上は無視して ~/.kiro。
func TestKiroHomeOverride(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	abs := filepath.Join(h, "elsewhere", "kiro")
	for v, want := range map[string]string{
		"rel/dir":                  filepath.Join(h, ".kiro"),
		string(filepath.Separator): filepath.Join(h, ".kiro"),
		h:                          filepath.Join(h, ".kiro"),
		filepath.Dir(h):            filepath.Join(h, ".kiro"),
		abs:                        abs,
		h + "-x":                   h + "-x",
	} {
		t.Setenv("KIRO_HOME", v)
		if got := DefaultKiroHome(); got != want {
			t.Errorf("KIRO_HOME=%q: %q, want %q", v, got, want)
		}
	}
}

// KIRO_HOME を設定しても、Kiro の説明で KIRO_HOME が動かすのは V3 の保存場所だけ（https://kiro.dev/docs/configuration ）。
// 設定する前の会話・V2・Kiro IDE の会話は ~/.kiro に残るので、そこも読む。同じ会話は 1 回だけ出す（KIRO_HOME のほうを使う）。
func TestKiroHomeLegacy(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	work := filepath.Join(h, "work-kiro")
	legacy := filepath.Join(h, ".kiro")
	for _, v := range []string{"", "rel", legacy, legacy + string(filepath.Separator)} { // 設定なし・Kiro が使わない値・~/.kiro そのもの
		t.Setenv("KIRO_HOME", v)
		if got := DefaultKiroLegacyHome(); got != "" {
			t.Errorf("KIRO_HOME=%q: legacy = %q, want \"\"", v, got)
		}
	}
	t.Setenv("KIRO_HOME", work)
	if got := DefaultKiroLegacyHome(); got != legacy {
		t.Errorf("legacy = %q, want %q", got, legacy)
	}
	v2 := func(id, at, credit string) string {
		return `{"session_id": "` + id + `", "cwd": "/Users/me/app", "created_at": "` + at + `",
  "session_state": {"conversation_metadata": {"user_turn_metadatas": [{"end_timestamp": "` + at + `", "metering_usage": [{"value": ` + credit + `, "unit": "credit"}]}]}}}`
	}
	writeFiles(t, legacy, map[string]string{
		"sessions/cli/old.json":                 v2("old", "2026-09-01T00:00:00Z", "1"),
		"sessions/cli/old.jsonl":                "",
		"sessions/cli/dup.json":                 v2("dup", "2026-09-02T00:00:00Z", "9"),
		"sessions/cli/mv.json":                  v2("mv", "2026-09-03T00:00:00Z", "2"),
		"sessions/aaa/sess_ide/session.json":    `{"id": "sess_ide", "createdAt": "2026-09-04T00:00:00Z"}`,
		"sessions/aaa/sess_ide/messages.jsonl":  `{"timestamp": "2026-09-04T00:01:00Z", "payload": {"type": "user", "content": "IDE で"}}` + "\n",
		"sessions/aaa/sess_both/session.json":   `{"id": "sess_both", "title": "old copy", "createdAt": "2026-09-05T00:00:00Z"}`,
		"sessions/aaa/sess_both/messages.jsonl": "",
	})
	writeFiles(t, work, map[string]string{
		"sessions/cli/dup.json":                 v2("dup", "2026-09-02T00:00:00Z", "3"),
		"sessions/cli/dup.jsonl":                "",
		"sessions/bbb/sess_both/session.json":   `{"id": "sess_both", "title": "new", "createdAt": "2026-09-05T00:00:00Z"}`,
		"sessions/bbb/sess_both/messages.jsonl": "",
		// ~/.kiro の V2 の会話 mv を、KIRO_HOME を設定してから V3 で開いた写し（usage_summary はない）
		"sessions/bbb/sess_mv/session.json":   `{"id": "sess_mv", "createdAt": "2026-09-03T00:00:00Z"}`,
		"sessions/bbb/sess_mv/messages.jsonl": `{"timestamp": "2026-09-06T00:00:00Z", "payload": {"type": "user", "content": "続き"}}` + "\n",
	})

	byID := func(bs []*core.Builder) map[string]*core.Builder {
		m := map[string]*core.Builder{}
		for _, b := range bs {
			if m[b.ID] != nil {
				t.Errorf("%s の %s が 2 回", b.Source, b.ID)
			}
			m[b.ID] = b
		}
		return m
	}
	cli := byID(load(t, &KiroCLI{Home: work, Legacy: legacy}))
	if len(cli) != 3 || cli["old"] == nil || cli["dup"] == nil || cli["mv"] == nil {
		t.Fatalf("V2 = %v, want old・dup・mv", cli)
	}
	if c := cli["dup"].Finish(15).Credits; c != 3 {
		t.Errorf("dup のクレジット = %v, want 3（KIRO_HOME のほう）", c)
	}
	if y := cli["mv"].Yield; y != kiroClassicYield("mv") {
		t.Errorf("mv の Yield = %q（KIRO_HOME の V3 の写しに譲る）", y)
	}

	ide := byID(load(t, &KiroIDE{Home: work, Legacy: legacy}))
	if len(ide) != 3 || ide["sess_ide"] == nil || ide["sess_both"] == nil || ide["sess_mv"] == nil {
		t.Fatalf("V3 = %v, want sess_ide・sess_both・sess_mv", ide)
	}
	if ide["sess_both"].Title != "new" {
		t.Errorf("sess_both = %q, want KIRO_HOME のほう", ide["sess_both"].Title)
	}
	if c := ide["sess_mv"].Finish(15).Credits; c != 2 {
		t.Errorf("sess_mv のクレジット = %v, want 2（~/.kiro の V2 の会話から足す）", c)
	}

	// 同じ場所（KIRO_HOME が ~/.kiro）なら、2 回読まない
	if bs := load(t, &KiroCLI{Home: legacy, Legacy: legacy}); len(bs) != 3 {
		t.Errorf("Home = Legacy: %d 件, want 3", len(bs))
	}
	// 見張る場所に ~/.kiro も入る
	w := strings.Join(append(WatchPaths(&KiroIDE{Home: work, Legacy: legacy}), WatchPaths(&KiroCLI{Home: work, Legacy: legacy})...), "\n")
	for _, p := range []string{filepath.Join(legacy, "sessions"), filepath.Join(legacy, "sessions", "cli")} {
		if !strings.Contains(w+"\n", p+"\n") {
			t.Errorf("watch %q に %q がない", w, p)
		}
	}
}
