package ai

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// 中身を出さない会話（Kiro Crew の incognito・temporary とその写し）のファイルは、Claude Code の Read の deny ルールにする。
// 隣の同じ名前のフォルダ（subagents/）と、Crew が sessions/archive/ に退避した行もふくめる。名前の * などは文字として扱う。
func TestDenyRules(t *testing.T) {
	// Windows では C:\home\me が //c/home/me になる（Claude Code はパスを POSIX の形にしてから比べる）
	root, pre := "/", "//"
	if runtime.GOOS == "windows" {
		root, pre = `C:\`, "//c/"
	}
	in := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	files := []string{
		in("home", "me", ".kiro", "crew", "sessions", "dashboard_chat-1-100.jsonl"),
		in("home", "me", ".claude", "projects", "-w", "abc.jsonl"),
		in("home", "me", ".codex", "sessions", "2026", "11", "01", "rollout-x.jsonl.zst"),
		in("home", "me", "odd", "p(1)", "x.jsonl"),
		"relative.jsonl", "",
	}
	if runtime.GOOS != "windows" { // * と [ はファイル名に使えない
		files = append(files[:3], append([]string{in("home", "me", "odd", "a*b[1].jsonl")}, files[3:]...)...)
		// \ のある名前は、Claude Code が \\ を \ にしてから比べるのでルールに書けない。フォルダごと（上の括弧と同じルールになる）
		files = append(files, in("home", "me", "odd", `back\slash.jsonl`))
	}
	got := denyRules(files)
	want := []string{
		"Read(//home/me/.kiro/crew/sessions/dashboard_chat-1-100.jsonl)",
		"Read(//home/me/.kiro/crew/sessions/dashboard_chat-1-100/**)",
		"Read(//home/me/.kiro/crew/sessions/archive/dashboard_chat-1-100__*)",
		"Read(//home/me/.claude/projects/-w/abc.jsonl)",
		"Read(//home/me/.claude/projects/-w/abc/**)",
		"Read(//home/me/.codex/sessions/2026/11/01/rollout-x.jsonl.zst)",
		"Read(//home/me/.codex/sessions/2026/11/01/rollout-x/**)",
		`Read(//home/me/odd/a\*b\[1\].jsonl)`,
		`Read(//home/me/odd/a\*b\[1\]/**)`,
		"Read(//home/me/odd/**)", // 括弧のある名前は、ルールに書けないので、書ける上のフォルダごと
	}
	if runtime.GOOS == "windows" {
		want = append(want[:7], want[9:]...)
	}
	for i := range want {
		want[i] = strings.Replace(want[i], "Read(//", "Read("+pre, 1)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("denyRules =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

}

// Claude Code には、deny ルールを書いた設定ファイルを --settings で渡す（コマンドの行と画面には、ファイルの場所しか出ない）。
func TestClaudeGetsPrivateDenySettings(t *testing.T) {
	stubAgent(t, "claude", `cat >/dev/null
while [ $# -gt 0 ]; do if [ "$1" = --settings ]; then f=$2; fi; shift; done
printf '{"type":"result","result":%s}\n' "$(cat "$f" | sed 's/\\/\\\\/g; s/"/\\"/g; s/^/"/; s/$/"/')"`)
	priv := "/home/me/.kiro/crew/sessions/secret-chat.jsonl"
	var argv []string
	res, err := (Runner{Enabled: true, Private: func() []string { return []string{priv} }}).Stream(context.Background(), "claude", "hi", func(e Event) {
		if e.T == "cmd" {
			argv = e.Argv
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(argv, "--settings") || strings.Contains(strings.Join(argv, " "), "secret-chat") {
		t.Errorf("argv = %q, want --settings and no private path", argv)
	}
	var cfg struct {
		Permissions struct{ Deny []string } `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(res.Output), &cfg); err != nil {
		t.Fatalf("settings %q: %v", res.Output, err)
	}
	if !slices.Contains(cfg.Permissions.Deny, "Read(/"+priv+")") {
		t.Errorf("deny = %q", cfg.Permissions.Deny)
	}
	// 中身を出さない会話がなければ、設定ファイルは渡さない
	argv = nil
	stubAgent(t, "claude", `cat >/dev/null; echo '{"type":"result","result":"ok"}'`)
	if _, err := (Runner{Enabled: true, Private: func() []string { return nil }}).Stream(context.Background(), "claude", "hi", func(e Event) {
		if e.T == "cmd" {
			argv = e.Argv
		}
	}); err != nil || slices.Contains(argv, "--settings") {
		t.Errorf("argv = %q, %v", argv, err)
	}
}
