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
		// 実在する ID とパスはそのまま。
		{"/Users/me/web", "claude --resume", "3f2a9c1e-7b4d-4e8a-9f0c-1d2e3f4a5b6c", "cd /Users/me/web && claude --resume 3f2a9c1e-7b4d-4e8a-9f0c-1d2e3f4a5b6c"},
		{"/Users/me/-web", "codex resume", "thr-main-", "cd /Users/me/-web && codex resume thr-main-"},
		// - で始まる値は、囲んでもコマンドのオプションとして読まれるので出さない。
		{"/Users/me/web", "claude --resume", "--dangerously-skip-permissions", ""},
		{"/Users/me/web", "codex resume", "-csandbox_mode=danger-full-access", ""},
		{"/Users/me/web", "claude --resume", "--", ""},
		{"/Users/me/web", "claude --resume", "-", ""},
		{"/Users/me/web", "claude --resume", "- x", ""},
		{"-", "claude --resume", "abc", ""},
		{"-P", "q chat --resume", "", ""},
		{"--help me", "claude --resume", "abc", ""},
		{`C:\Users\me\web`, "codex resume", "-csandbox_mode=danger-full-access", ""},
		// 表示と貼られる中身が違って見える Unicode の書式文字（双方向の制御、幅ゼロの文字）は出さない。
		{"/Users/me/web\u202e", "claude --resume", "abc", ""},
		{"/Users/me/web", "claude --resume", "abc\u202edcba", ""},
		{"/Users/me/web", "claude --resume", "a\u2066b\u2069", ""},
		{"/Users/me/web", "claude --resume", "abc\u200b", ""},
		{"/Users/me/\ufeffweb", "claude --resume", "abc", ""},
		{"C:\\Users\\me\\web\u200b", "codex resume", "thr", ""},
	}
	for _, c := range cases {
		if got := ResumeCmd(c.dir, c.cmd, c.id); got != c.want {
			t.Errorf("ResumeCmd(%q, %q, %q) = %q, want %q", c.dir, c.cmd, c.id, got, c.want)
		}
	}
}

// コミットと push の詳細でコピーする git のコマンドも、リポジトリのパスや ref をシェルが解釈しないこと（セキュリティ）。
func TestGitCmd(t *testing.T) {
	cases := []struct {
		repo string
		cmd  string
		args []string
		want string
	}{
		{"/Users/me/web", "show", []string{"4917468abc"}, "git -C /Users/me/web show 4917468abc"},
		{"/Users/me/My Project", "show", []string{"abc"}, "git -C '/Users/me/My Project' show abc"},
		{"/tmp/r$(touch pwned-dollar)`touch pwned-tick`;touch pwned-semi", "show", []string{"abc"}, "git -C '/tmp/r$(touch pwned-dollar)`touch pwned-tick`;touch pwned-semi' show abc"},
		{"/tmp/it's", "log --oneline", []string{"aaa..bbb"}, `git -C '/tmp/it'\''s' log --oneline aaa..bbb`},
		{"/Users/me/web", "show", []string{"abc; curl evil"}, "git -C /Users/me/web show 'abc; curl evil'"},
		{`C:\Users\John Smith\a & calc`, "show", []string{"abc"}, `git -C "C:\Users\John Smith\a & calc" show abc`},
		{`C:\Users\me\$(calc)`, "show", []string{"abc"}, ""},
		{`C:\Users\me\%PATH%`, "show", []string{"abc"}, ""},
		{"/tmp/a\nrm -rf ~", "show", []string{"abc"}, ""},
		{"/Users/me/web\u202e", "show", []string{"abc"}, ""},
		{"-c", "show", []string{"abc"}, ""},
		{"/Users/me/web", "show", []string{"--output=/tmp/x"}, ""},
		{"", "show", []string{"abc"}, ""},
	}
	for _, c := range cases {
		if got := GitCmd(c.repo, c.cmd, c.args...); got != c.want {
			t.Errorf("GitCmd(%q, %q, %q) = %q, want %q", c.repo, c.cmd, c.args, got, c.want)
		}
	}
}
