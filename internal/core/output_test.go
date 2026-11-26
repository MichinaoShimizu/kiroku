package core

import (
	"strings"
	"testing"
)

func TestOutputs(t *testing.T) {
	sum := func(name string, input any) OutputTotal {
		var o OutputTotal
		for _, x := range Outputs(name, input, nil) {
			o.Add(x)
		}
		return o
	}
	cases := []struct {
		name  string
		input any
		want  OutputTotal
	}{
		{"Bash", Obj{"command": `git add -A && git commit -m "fix"`}, OutputTotal{Commits: 1}},
		{"Bash", Obj{"command": `git -C /repo commit -qm x && git push && gh pr create --title t`}, OutputTotal{Commits: 1, PRs: 1}},
		{"Bash", Obj{"command": `git commit --dry-run`}, OutputTotal{}},
		{"Bash", Obj{"command": `git log --oneline | grep commit`}, OutputTotal{}},
		{"Bash", Obj{"command": `echo "git commit"`}, OutputTotal{}},
		{"mcp__github__create_pull_request", Obj{"title": "t"}, OutputTotal{PRs: 1}},
		{"Edit", Obj{"old_string": "a\nb\nc", "new_string": "a\nB\nB2\nc"}, OutputTotal{Added: 2, Removed: 1}},
		{"MultiEdit", Obj{"edits": []any{Obj{"old_string": "x", "new_string": "y"}, Obj{"old_string": "", "new_string": "z\nw"}}}, OutputTotal{Added: 3, Removed: 1}},
		{"Write", Obj{"content": "1\n2\n3\n"}, OutputTotal{Added: 3}},
		{"Read", Obj{"file_path": "x"}, OutputTotal{}},
	}
	for _, c := range cases {
		if got := sum(c.name, c.input); got != c.want {
			t.Errorf("%s %v: got %+v, want %+v", c.name, c.input, got, c.want)
		}
	}
}

// PR の操作（作成・マージ・レビュー）は、シェルのコマンドとツールの名前から、エージェントによらず同じに決める。
// 書いた順に返し、マージ・レビューした PR（番号・URL・-R・MCP の repo と pullNumber）がわかれば Ref に入れる。
func TestPRActions(t *testing.T) {
	cmd := func(c any) map[string]any { return map[string]any{"command": c} }
	for _, c := range []struct {
		name  string
		input any
		want  string
	}{
		{"Bash", cmd("gh pr create --fill"), "pr"},
		{"Bash", cmd("gh pr merge 12 --squash --delete-branch"), "merge:#12"},
		{"Bash", cmd("gh pr merge --squash"), "merge"}, // 今のブランチの PR（番号はわからない）
		{"Bash", cmd("gh pr merge fix/login --squash"), "merge"},
		{"Bash", cmd("gh pr merge https://github.com/o/web-app/pull/7 --rebase"), "merge:web-app#7"},
		{"Bash", cmd("gh pr merge 12 --auto --squash"), ""}, // 自動マージの予約
		{"Bash", cmd("gh pr merge --disable-auto"), ""},
		{"shell", cmd([]any{"gh", "pr", "review", "12", "--approve"}), "review:#12 approve:#12"},
		{"exec_command", map[string]any{"cmd": "gh pr review 12 -a -b ok"}, "review:#12 approve:#12"},
		{"Bash", cmd("gh pr review -R acme/api 3 --request-changes -b 'fix 9'"), "review:api#3"},
		{"Bash", cmd("gh pr review 12 --comment -b hi"), "review:#12"},
		{"Bash", cmd("gh pr view 12 && gh pr checks 12"), ""},
		{"Bash", cmd("echo gh-pr-merge; rg gh pr create docs"), ""},
		{"Bash", cmd(`git commit -m "docs: explain gh pr create"`), ""},
		{"Bash", cmd("gh pr create --help"), ""},
		// 書いた順（承認してからマージ）。--auto は、そのコマンドだけに効く
		{"Bash", cmd("gh pr review 55 --approve && gh pr merge 55 --squash"), "review:#55 approve:#55 merge:#55"},
		{"Bash", cmd("git push && GH_PROMPT_DISABLED=1 gh pr create --fill && gh pr merge --auto --squash; gh pr merge 3"), "pr merge:#3"},
		{"notes__save", cmd("gh pr create"), ""}, // シェルでないツール
		{"mcp__github__create_pull_request", nil, "pr"},
		{"github__merge_pull_request", map[string]any{"owner": "o", "repo": "app", "pullNumber": 38.0}, "merge:app#38"},
		{"github__merge_pull_request", map[string]any{"repo": "a b", "pullNumber": 1.5}, "merge"},
		{"mcp__github__pull_request_review_write", map[string]any{"method": "create", "event": "APPROVE", "repo": "app", "pullNumber": 40.0}, "review:app#40 approve:app#40"},
		{"mcp__github__pull_request_review_write", map[string]any{"method": "create"}, ""}, // 保留中のレビューを作っただけ
		{"mcp__github__pull_request_review_write", map[string]any{"method": "submit_pending", "event": "REQUEST_CHANGES"}, "review"},
		{"mcp__github__pull_request_review_write", map[string]any{"method": "delete_pending"}, ""},
		{"mcp__github__create_pull_request_review", map[string]any{"event": "approve", "pull_number": 2.0}, "review:#2 approve:#2"},
		{"mcp__github__create_pull_request_review", map[string]any{}, ""},
		{"mcp__github__submit_pending_pull_request_review", map[string]any{"event": "COMMENT"}, "review"},
		{"mcp__github__update_pull_request", map[string]any{"state": "closed"}, "ghop"}, // PR の作成・マージ・レビューでない、GitHub への書き込み
		{"mcp__github__add_issue_comment", map[string]any{"issue_number": 3.0}, "ghop"},
		{"github__add_reply_to_pull_request_comment", nil, "ghop"},
		{"mcp__github__issue_read", nil, ""}, // 読むだけ
		{"mcp__github__list_pull_requests", nil, ""},
		{"Bash", cmd("gh pr comment 12 -b 'thanks'"), "ghop"},
		{"Bash", cmd("gh issue create --title x --body y && gh pr close 3"), "ghop ghop"},
		{"Bash", cmd("gh issue view 3; gh issue list"), ""},
		{"Bash", cmd("gh pr comment --help"), ""},
	} {
		var got []string
		for _, a := range PRActions(c.name, c.input) {
			if a.Ref != "" {
				got = append(got, a.Kind+":"+a.Ref)
			} else {
				got = append(got, a.Kind)
			}
		}
		if g := strings.Join(got, " "); g != c.want {
			t.Errorf("PRActions(%s, %v) = %q, want %q", c.name, c.input, g, c.want)
		}
	}
}

// マージ・レビュー・アプルーブは、合計とセッションの流れ（PRActs）に入る。アプルーブは流れで review と重ねない。
func TestPRActsInSession(t *testing.T) {
	b := NewBuilder("Codex", "s")
	t1, t2, t3 := 100.0, 200.0, 300.0
	b.Tick(&t1)
	b.Tick(&t3)
	b.Outputs = append(b.Outputs, Output{T: &t1, Kind: "review", V: 1, Ref: "#4"}, Output{T: &t1, Kind: "approve", V: 1, Ref: "#4"},
		Output{T: &t2, Kind: "review", V: 1}, Output{T: &t3, Kind: "merge", V: 1})
	s := b.Finish(15)
	if s.Outputs.Reviews != 2 || s.Outputs.Approvals != 1 || s.Outputs.Merges != 1 || s.Outputs.PRs != 0 {
		t.Errorf("合計 = %+v", s.Outputs)
	}
	var got []string
	for _, a := range s.PRActs {
		got = append(got, a.K)
	}
	if strings.Join(got, " ") != "approve review merge" || s.PRActs[0].Ref != "#4" {
		t.Errorf("PRActs = %+v, want approve(#4) review merge", s.PRActs)
	}
	if h := s.Hidden("x", "x"); len(h.PRActs) != 3 || h.PRActs[0].Ref != "" {
		t.Errorf("隠した会話の PRActs = %+v（どの PR かは出さない）", h.PRActs)
	}
}

// 見た目と行き先が食い違う URL（向きを変える文字・幅のない文字）は、PR の URL にしない。
func TestPRURLRejectsInvisible(t *testing.T) {
	for in, want := range map[string]string{
		"created https://github.com/o/r/pull/12\n":     "https://github.com/o/r/pull/12",
		"https://github.com/o/r\u202e/pull/12":         "",
		"https://github\u200b.com/o/r/pull/12":         "",
		"https://evil.example/github.com/o/r/pull/1 x": "https://evil.example/github.com/o/r/pull/1",
		"<a href=\"javascript:alert(1)//pull/1\">":     "",
	} {
		if got := PRURL(in); got != want {
			t.Errorf("PRURL(%q) = %q, want %q", in, got, want)
		}
	}
}
