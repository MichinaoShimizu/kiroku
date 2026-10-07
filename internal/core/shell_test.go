package core

import "testing"

// 履歴から来たフォルダ名や ID を、貼ったときにシェルが解釈しないこと（セキュリティ）。
func TestResumeCmd(t *testing.T) {
	cases := []struct{ dir, cmd, id, want string }{
		{"/Users/me/web", "codex resume", "thr-main", "cd /Users/me/web && codex resume thr-main"},
		{"/Users/me/My Project", "claude --resume", "abc", "cd '/Users/me/My Project' && claude --resume abc"},
		{"/tmp/x; rm -rf ~", "claude --resume", "abc", "cd '/tmp/x; rm -rf ~' && claude --resume abc"},
		{"/tmp/$(touch pwned)", "claude --resume", "abc", "cd '/tmp/$(touch pwned)' && claude --resume abc"},
		{"/tmp/it's", "q chat --resume", "", `cd '/tmp/it'\''s' && q chat --resume`},
		{"/tmp/a`id`", "codex resume", "x", "cd '/tmp/a`id`' && codex resume x"},
		{"/Users/me/web", "codex resume", "x && curl evil", "cd /Users/me/web && codex resume 'x && curl evil'"},
		{"/tmp/a\nrm -rf ~", "claude --resume", "abc", ""},
		{"/tmp/a\x1b[2J", "claude --resume", "abc", ""},
		{"/Users/me/web", "claude --resume", "a\rb", ""},
		{`C:\Users\me\web`, "codex resume", "thr", `cd C:\Users\me\web && codex resume thr`},
		{`C:\Users\John Smith\a & calc`, "codex resume", "thr", `cd "C:\Users\John Smith\a & calc" && codex resume thr`},
		{`C:\Users\me\$(calc)`, "codex resume", "thr", ""},
		{`C:\Users\me\%PATH%`, "codex resume", "thr", ""},
		{`C:\Users\me\web`, "codex resume", "x & calc", `cd C:\Users\me\web && codex resume "x & calc"`},
		{`\\server\share\p`, "kiro-cli chat --resume-id", "s1", `cd \\server\share\p && kiro-cli chat --resume-id s1`},
	}
	for _, c := range cases {
		if got := ResumeCmd(c.dir, c.cmd, c.id); got != c.want {
			t.Errorf("ResumeCmd(%q, %q, %q) = %q, want %q", c.dir, c.cmd, c.id, got, c.want)
		}
	}
}
