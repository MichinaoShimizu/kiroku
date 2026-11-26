package source

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

func prOutputs(b *core.Builder) (n float64, urls []string) {
	for _, o := range b.Outputs {
		if o.Kind == "pr" {
			n += o.V
			urls = append(urls, o.URL)
		}
	}
	return n, urls
}

// Codex: シェル（shell・shell_command・exec_command・local_shell）の gh pr create と、MCP の create_pull_request を、
// 結果（function_call_output）が成功のときだけ PR として数える。終了コードが 0 でない・結果のない呼び出しは数えない。
func TestCodexPullRequests(t *testing.T) {
	home := t.TempDir()
	call := func(sec int, typ, extra string) string {
		return `{"timestamp":"2026-10-06T00:00:` + fmt.Sprintf("%02d", sec) + `.000Z","type":"response_item","payload":{"type":"` + typ + `",` + extra + `}}`
	}
	out := func(sec int, id, output string) string {
		return `{"timestamp":"2026-10-06T00:00:` + fmt.Sprintf("%02d", sec) + `.000Z","type":"response_item","payload":{"type":"function_call_output","call_id":"` + id + `","output":` + output + `}}`
	}
	writeCodex(t, home, "thr-pr",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-pr","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/project"}}`,
		`{"timestamp":"2026-10-06T00:00:01.000Z","type":"event_msg","payload":{"type":"user_message","message":"Open a PR"}}`,
		// shell（command は配列）。古い版の JSON の結果
		call(2, "function_call", `"name":"shell","call_id":"a","arguments":"{\"command\":[\"bash\",\"-lc\",\"gh pr create --fill\"]}"`),
		out(3, "a", `"{\"output\":\"https://github.com/o/r/pull/1\\n\",\"metadata\":{\"exit_code\":0}}"`),
		// exec_command（cmd）。新しい版の文の結果
		call(4, "function_call", `"name":"exec_command","call_id":"b","arguments":"{\"cmd\":\"git push && gh pr create --title x\"}"`),
		out(5, "b", `"Exit code: 0\nWall time: 1.2 seconds\nOutput:\nhttps://github.com/o/r/pull/2\n"`),
		// 失敗した gh pr create（終了コード 1、古い版の JSON）は数えない
		call(6, "function_call", `"name":"shell","call_id":"c","arguments":"{\"command\":[\"gh\",\"pr\",\"create\"]}"`),
		out(7, "c", `"{\"output\":\"a pull request already exists\",\"metadata\":{\"exit_code\":1}}"`),
		// 失敗した gh pr create（新しい版の文）も数えない
		call(8, "function_call", `"name":"shell_command","call_id":"d","arguments":"{\"command\":\"gh pr create\"}"`),
		out(9, "d", `"Exit code: 1\nOutput:\nhttps://github.com/o/r/pull/9 already exists\n"`),
		// MCP（サーバー__ツール）。結果は内容の並び
		call(10, "function_call", `"name":"github__create_pull_request","call_id":"e","arguments":"{\"title\":\"x\"}"`),
		out(11, "e", `[{"type":"input_text","text":"{\"html_url\":\"https://github.com/o/r/pull/3\"}"}]`),
		// apply_patch の入力に gh pr create の文があっても、PR を作る呼び出しではない
		call(12, "custom_tool_call", `"name":"apply_patch","call_id":"p","input":"*** Begin Patch\n*** Add File: docs/x.md\n+run gh pr create\n*** End Patch"`),
		`{"timestamp":"2026-10-06T00:00:12.500Z","type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"p","output":"https://github.com/o/r/pull/6"}}`,
		// local_shell（action.command）
		call(12, "local_shell_call", `"call_id":"f","status":"completed","action":{"type":"exec","command":["gh","pr","create","--fill"]}`),
		out(13, "f", `"https://github.com/o/r/pull/4"`),
		// 結果のない呼び出し・PR を作らない呼び出しは数えない
		call(14, "function_call", `"name":"shell","call_id":"g","arguments":"{\"command\":[\"gh\",\"pr\",\"create\"]}"`),
		call(15, "function_call", `"name":"shell","call_id":"h","arguments":"{\"command\":[\"gh\",\"pr\",\"view\"]}"`),
		out(16, "h", `"https://github.com/o/r/pull/5"`),
	)
	bs := load(t, &Codex{Home: home})
	if len(bs) != 1 {
		t.Fatalf("sessions = %d, want 1", len(bs))
	}
	n, urls := prOutputs(bs[0])
	want := "https://github.com/o/r/pull/1|https://github.com/o/r/pull/2|https://github.com/o/r/pull/3|https://github.com/o/r/pull/4"
	if n != 4 || strings.Join(urls, "|") != want {
		t.Errorf("PR = %v %q, want 4 %q", n, urls, want)
	}
	if s := bs[0].Finish(15); len(s.PRAt) != 4 || s.Outputs.PRs != 4 {
		t.Errorf("PRAt = %+v, outputs = %+v", s.PRAt, s.Outputs)
	}
	// 結果のツールの行は読むもの（知らない種類として数えない）
	if !codexItems["function_call_output"] || !codexItems["custom_tool_call_output"] {
		t.Error("function_call_output・custom_tool_call_output は codexItems に入れる")
	}
}

// Kiro IDE / CLI (V3): tool_call の gh pr create・create_pull_request を、tool_result の success が false でなければ数える。
func TestKiroIDEPullRequests(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_1/session.json": `{"id": "sess_1", "createdAt": "2026-09-29T01:00:00Z"}`,
		"sessions/abc/sess_1/messages.jsonl": `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": "PR を出して"}}
{"timestamp": "2026-09-29T01:02:00Z", "payload": {"type": "tool_call", "toolCallId": "a", "toolName": "executeBash", "args": {"command": "gh pr create --fill"}}}
{"timestamp": "2026-09-29T01:03:00Z", "payload": {"type": "tool_result", "toolCallId": "a", "content": "https://github.com/o/r/pull/7", "success": true}}
{"timestamp": "2026-09-29T01:04:00Z", "payload": {"type": "tool_call", "toolCallId": "b", "toolName": "executeBash", "args": {"command": "gh pr create"}}}
{"timestamp": "2026-09-29T01:05:00Z", "payload": {"type": "tool_result", "toolCallId": "b", "content": "https://github.com/o/r/pull/8 already exists", "success": false}}
{"timestamp": "2026-09-29T01:06:00Z", "payload": {"type": "tool_call", "toolCallId": "c", "toolName": "github___create_pull_request", "args": {"title": "x"}}}
{"timestamp": "2026-09-29T01:07:00Z", "payload": {"type": "tool_result", "toolCallId": "c", "content": "created"}}
{"timestamp": "2026-09-29T01:08:00Z", "payload": {"type": "tool_call", "toolCallId": "d", "toolName": "executeBash", "args": {"command": "gh pr create"}}}
`,
	})
	b := find(load(t, &KiroIDE{Home: home}), "sess_1")
	if b == nil {
		t.Fatal("sess_1 がない")
	}
	n, urls := prOutputs(b)
	if n != 2 || strings.Join(urls, "|") != "https://github.com/o/r/pull/7|" {
		t.Errorf("PR = %v %q, want 2（失敗と結果のないものは数えない。URL のない結果は URL なし）", n, urls)
	}
}

// Kiro CLI (SQLite): ToolUse の execute_bash の gh pr create を、次のターンの ToolUseResults の status が Success なら数える。
func TestQStorePullRequests(t *testing.T) {
	t0 := time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC)
	ms := func(sec int) int64 { return t0.Add(time.Duration(sec) * time.Second).UnixMilli() }
	meta := func(start int) map[string]any {
		return map[string]any{"model_id": "m", "request_start_timestamp_ms": ms(start), "stream_end_timestamp_ms": ms(start + 1)}
	}
	use := func(id, cmd string) map[string]any {
		return map[string]any{"ToolUse": map[string]any{"content": "", "tool_uses": []any{
			map[string]any{"id": id, "name": "execute_bash", "args": map[string]any{"command": cmd}}}}}
	}
	results := func(id, status string, content []any) map[string]any {
		return map[string]any{"content": map[string]any{"ToolUseResults": map[string]any{"tool_use_results": []any{
			map[string]any{"tool_use_id": id, "content": content, "status": status}}}}}
	}
	history := []any{
		map[string]any{"user": map[string]any{"timestamp": t0.Format(time.RFC3339), "content": map[string]any{"Prompt": map[string]any{"prompt": "PR を出して"}}},
			"assistant": use("a", "gh pr create --fill"), "request_metadata": meta(1)},
		map[string]any{"user": results("a", "Success", []any{map[string]any{"Json": map[string]any{"exit_status": "0", "stdout": "https://github.com/o/r/pull/11\n"}}}),
			"assistant": use("b", "gh pr create"), "request_metadata": meta(60)},
		map[string]any{"user": results("b", "Error", []any{map[string]any{"Text": "https://github.com/o/r/pull/12 already exists"}}),
			"assistant": use("c", "gh pr create --fill"), "request_metadata": meta(120)},
		// execute_bash は終了コードが 0 でなくても Success。exit_status で失敗とわかる
		map[string]any{"user": results("c", "Success", []any{map[string]any{"Json": map[string]any{"exit_status": "1", "stderr": "https://github.com/o/r/pull/13 already exists"}}}),
			"assistant": map[string]any{"Response": map[string]any{"content": "既にあります"}}, "request_metadata": meta(180)},
	}
	bs := load(t, &QStore{Label: "Kiro CLI (SQLite)", DB: qHistoryDB(t, history)})
	if len(bs) != 1 {
		t.Fatalf("会話の数 = %d, want 1", len(bs))
	}
	n, urls := prOutputs(bs[0])
	if n != 1 || len(urls) != 1 || urls[0] != "https://github.com/o/r/pull/11" {
		t.Errorf("PR = %v %q, want 1（Error と exit_status が 0 でないものは数えない）", n, urls)
	}
	if o := bs[0].Outputs; len(o) == 1 && (o[0].T == nil || *o[0].T != float64(ms(0))/1000) {
		t.Errorf("PR の時刻 = %v, want 呼び出しの時刻", o[0].T)
	}
}

// 結果を待つ呼び出しの数には上限がある（細工した履歴でメモリを使い切らない）。
func TestPRCallsBounded(t *testing.T) {
	p := prCalls{}
	for i := 0; i < maxPRCalls+50; i++ {
		p.call(strconv.Itoa(i), "shell", map[string]any{"command": "gh pr create"}, nil)
	}
	if len(p) != maxPRCalls {
		t.Errorf("覚えた呼び出し = %d, want %d", len(p), maxPRCalls)
	}
}
