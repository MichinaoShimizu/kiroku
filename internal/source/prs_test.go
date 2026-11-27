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
	p := newPRCalls()
	for i := 0; i < maxPRCalls+50; i++ {
		p.call(strconv.Itoa(i), "shell", map[string]any{"command": "gh pr create"}, nil)
	}
	if len(p.calls) != maxPRCalls {
		t.Errorf("覚えた呼び出し = %d, want %d", len(p.calls), maxPRCalls)
	}
}

// PR の操作でない呼び出しは数えない: apply_patch の配列（パッチの本文）、--help、rg やコミットのメッセージの中の文、
// シェルでないツールの command。終了コードの珍しい書き方（1e400・タブ・文の success）は失敗として扱う。
// 同じ call_id を使い直して同じ URL が 2 回出ても、PR は 1 つ。サブエージェントの PR は親のセッションに入る。
func TestCodexPullRequestFalsePositives(t *testing.T) {
	home := t.TempDir()
	n := 0
	at := func() string { n++; return fmt.Sprintf("2026-10-06T00:%02d:%02d.000Z", n/60, n%60) }
	call := func(id, name, args string) string {
		return `{"timestamp":"` + at() + `","type":"response_item","payload":{"type":"function_call","name":"` + name + `","call_id":"` + id + `","arguments":` + args + `}}`
	}
	out := func(id, output string) string {
		return `{"timestamp":"` + at() + `","type":"response_item","payload":{"type":"function_call_output","call_id":"` + id + `","output":` + output + `}}`
	}
	ok := `"https://github.com/o/r/pull/1"`
	writeCodex(t, home, "thr-fp",
		`{"timestamp":"2026-10-06T00:00:00.000Z","type":"session_meta","payload":{"id":"thr-fp","timestamp":"2026-10-06T00:00:00.000Z","cwd":"/Users/me/project"}}`,
		call("a", "shell", `"{\"command\":[\"apply_patch\",\"*** Begin Patch\\n+ gh pr create\\n*** End Patch\"]}"`), out("a", ok),
		call("b", "shell", `"{\"command\":[\"gh\",\"pr\",\"create\",\"--help\"]}"`), out("b", ok),
		call("c", "shell", `"{\"command\":[\"bash\",\"-lc\",\"rg -n gh pr create docs\"]}"`), out("c", ok),
		call("d", "shell", `"{\"command\":[\"bash\",\"-lc\",\"git commit -m \\\"docs: explain gh pr create\\\"\"]}"`), out("d", ok),
		call("e", "notes__save", `"{\"command\":\"remember to gh pr create\"}"`), out("e", ok),
		call("f", "shell", `"{\"command\":[\"gh\",\"pr\",\"create\"]}"`), out("f", `"{\"output\":\"x\",\"metadata\":{\"exit_code\":1e400}}"`),
		call("g", "shell", `"{\"command\":[\"gh\",\"pr\",\"create\"]}"`), out("g", `"Exit code:\t1\nOutput:\nhttps://github.com/o/r/pull/9 already exists"`),
		call("h", "shell", `"{\"command\":[\"gh\",\"pr\",\"create\"]}"`), out("h", `{"content":"https://github.com/o/r/pull/9","success":"false"}`),
		call("i", "exec_command", `"{\"cmd\":\"gh pr create\"}"`), out("i", `"Process running with session ID 3\nOutput:\n? Title"`),
		// 数えるもの: bash -lc の中・VAR=値 の前置き。同じ call_id を使い直して同じ URL でも 1 回
		call("j", "shell", `"{\"command\":[\"bash\",\"-lc\",\"git push && GH_PROMPT_DISABLED=1 gh pr create --fill\"]}"`), out("j", `"https://github.com/o/r/pull/2"`),
		call("j", "shell", `"{\"command\":[\"gh\",\"pr\",\"create\"]}"`), out("j", `"https://github.com/o/r/pull/2"`),
		call("k", "shell", `"{\"command\":[\"gh\",\"pr\",\"merge\",\"2\",\"--squash\"]}"`), out("k", `"Exit code: 0\nOutput:\nMerged"`),
	)
	bs := load(t, &Codex{Home: home})
	if len(bs) != 1 {
		t.Fatalf("sessions = %d, want 1", len(bs))
	}
	var tot core.OutputTotal
	for _, o := range bs[0].Outputs {
		tot.Add(o)
	}
	if tot.PRs != 1 || tot.Merges != 1 {
		t.Errorf("PR・マージ = %v %v, want 1 1（PR の操作でない呼び出し・失敗・同じ URL は数えない）", tot.PRs, tot.Merges)
	}
}

// Kiro IDE / CLI (V3): success が文の "false" や、content の終了コードが 0 でないものは失敗。マージとアプルーブも数える。
func TestKiroIDEPullRequestActions(t *testing.T) {
	home := t.TempDir()
	writeFiles(t, home, map[string]string{
		"sessions/abc/sess_2/session.json": `{"id": "sess_2", "createdAt": "2026-09-29T01:00:00Z"}`,
		"sessions/abc/sess_2/messages.jsonl": `{"timestamp": "2026-09-29T01:01:00Z", "payload": {"type": "user", "content": "レビューしてマージして"}}
{"timestamp": "2026-09-29T01:02:00Z", "payload": {"type": "tool_call", "toolCallId": "a", "toolName": "executeBash", "args": {"command": "gh pr create"}}}
{"timestamp": "2026-09-29T01:03:00Z", "payload": {"type": "tool_result", "toolCallId": "a", "content": "https://github.com/o/r/pull/7", "success": "false"}}
{"timestamp": "2026-09-29T01:04:00Z", "payload": {"type": "tool_call", "toolCallId": "b", "toolName": "executeBash", "args": {"command": "gh pr create"}}}
{"timestamp": "2026-09-29T01:05:00Z", "payload": {"type": "tool_result", "toolCallId": "b", "content": "{\"exitCode\":1,\"stdout\":\"https://github.com/o/r/pull/7 already exists\"}", "success": true}}
{"timestamp": "2026-09-29T01:06:00Z", "payload": {"type": "tool_call", "toolCallId": "c", "toolName": "executeBash", "args": {"command": "gh pr review 7 --approve"}}}
{"timestamp": "2026-09-29T01:07:00Z", "payload": {"type": "tool_result", "toolCallId": "c", "content": "Approved", "success": true}}
{"timestamp": "2026-09-29T01:08:00Z", "payload": {"type": "tool_call", "toolCallId": "d", "toolName": "github___merge_pull_request", "args": {"pullNumber": 7}}}
{"timestamp": "2026-09-29T01:09:00Z", "payload": {"type": "tool_result", "toolCallId": "d", "content": "merged", "success": true}}
`,
	})
	b := find(load(t, &KiroIDE{Home: home}), "sess_2")
	if b == nil {
		t.Fatal("sess_2 がない")
	}
	var tot core.OutputTotal
	for _, o := range b.Outputs {
		tot.Add(o)
	}
	if tot.PRs != 0 || tot.Reviews != 1 || tot.Approvals != 1 || tot.Merges != 1 {
		t.Errorf("合計 = %+v, want PR 0・レビュー 1（アプルーブ 1）・マージ 1", tot)
	}
	s := b.Finish(15)
	if len(s.PRActs) != 2 || s.PRActs[0].K != "approve" || s.PRActs[1].K != "merge" {
		t.Errorf("PRActs = %+v", s.PRActs)
	}
}

// Codex のサブエージェントが作った PR は、親のセッションのアウトプットに入る（Claude Code と同じ）。
// 親から写した行（session_meta より前の時刻）の PR は、子の分として重ねて数えない。
func TestCodexSubagentPullRequest(t *testing.T) {
	home := t.TempDir()
	parentCall := `{"timestamp":"2026-10-06T01:00:05.000Z","type":"response_item","payload":{"type":"function_call","name":"shell","call_id":"p1","arguments":"{\"command\":[\"gh\",\"pr\",\"create\"]}"}}`
	parentOut := `{"timestamp":"2026-10-06T01:00:06.000Z","type":"response_item","payload":{"type":"function_call_output","call_id":"p1","output":"https://github.com/o/r/pull/1"}}`
	writeCodex(t, home, "thr-p",
		`{"timestamp":"2026-10-06T01:00:00.000Z","type":"session_meta","payload":{"id":"thr-p","timestamp":"2026-10-06T01:00:00.000Z","cwd":"/Users/me/web"}}`,
		`{"timestamp":"2026-10-06T01:00:02.000Z","type":"event_msg","payload":{"type":"user_message","message":"親の依頼"}}`,
		parentCall, parentOut,
	)
	writeCodex(t, home, "thr-c",
		`{"timestamp":"2026-10-06T01:01:00.000Z","type":"session_meta","payload":{"id":"thr-c","timestamp":"2026-10-06T01:01:00.000Z","cwd":"/Users/me/web","source":{"subagent":{"thread_spawn":{"parent_thread_id":"thr-p","depth":1,"agent_role":"worker"}}}}}`,
		`{"timestamp":"2026-10-06T01:00:02.000Z","type":"event_msg","payload":{"type":"user_message","message":"親の依頼"}}`,
		parentCall, parentOut,
		`{"timestamp":"2026-10-06T01:01:02.000Z","type":"event_msg","payload":{"type":"user_message","message":"子の依頼"}}`,
		`{"timestamp":"2026-10-06T01:01:10.000Z","type":"response_item","payload":{"type":"function_call","name":"shell","call_id":"c1","arguments":"{\"command\":[\"gh\",\"pr\",\"merge\",\"1\"]}"}}`,
		`{"timestamp":"2026-10-06T01:01:11.000Z","type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":"Exit code: 0\nOutput:\nMerged"}}`,
	)
	p := find(load(t, &Codex{Home: home}), "thr-p")
	if p == nil {
		t.Fatal("thr-p がない")
	}
	s := p.Finish(15)
	if s.Outputs.PRs != 1 || s.Outputs.Merges != 1 || len(s.PRs) != 1 {
		t.Errorf("親のアウトプット = %+v %v, want PR 1（写した行は重ねない）・マージ 1（子の分）", s.Outputs, s.PRs)
	}
}
