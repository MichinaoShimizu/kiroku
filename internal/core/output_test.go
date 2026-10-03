package core

import "testing"

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
