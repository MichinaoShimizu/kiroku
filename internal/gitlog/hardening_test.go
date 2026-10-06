package gitlog

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// だれかが作ったリポジトリの .git/config に書かれたプログラム（fsmonitor・フック・textconv・外部 diff・ページャー）は、
// kiroku が git を読むときに動かない。
func TestCollectDoesNotRunRepoPrograms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh のスクリプトを使う")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	dir, tools := t.TempDir(), t.TempDir()
	marker := filepath.Join(tools, "ran")
	script := filepath.Join(tools, "evil.sh")
	// 動いたら印を残す（textconv として呼ばれたときのために、ファイルの中身も返す）
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$0 $*\" >> '"+marker+"'\n[ -f \"$1\" ] && cat \"$1\"\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(tools, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"reference-transaction", "post-checkout", "post-index-change", "pre-commit", "post-commit", "fsmonitor-watchman"} {
		if err := os.Symlink(script, filepath.Join(hooks, h)); err != nil {
			t.Fatal(err)
		}
	}

	run(t, dir, nil, "init", "-q")
	run(t, dir, nil, "config", "user.email", "me@example.com")
	run(t, dir, nil, "config", "user.name", "me")
	base := time.Now().Add(-2 * time.Hour).Unix()
	for i, body := range []string{"a\n", "a\nb\nc\n"} {
		os.WriteFile(filepath.Join(dir, "a.txt"), []byte(body), 0o644)
		os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("*.txt diff=evil\n"), 0o644)
		run(t, dir, nil, "add", "-A")
		date := time.Unix(base+int64(i)*600, 0).Format(time.RFC3339)
		run(t, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}, "commit", "-q", "-m", "c")
	}
	// コミットしたあとで、リポジトリの設定にプログラムを書く
	for _, kv := range [][2]string{
		{"core.fsmonitor", script},
		{"core.hooksPath", hooks},
		{"diff.evil.textconv", script},
		{"diff.external", script},
		{"core.pager", script},
		{"pager.log", script},
		{"log.showSignature", "true"},
		{"gpg.program", script},
	} {
		run(t, dir, nil, "config", kv[0], kv[1])
	}
	// 確かめ方が正しいこと: ふつうの git では textconv が動く
	check := exec.Command("git", "-C", dir, "log", "-p", "-n", "1")
	check.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := check.CombinedOutput(); err != nil {
		t.Fatalf("git log: %v\n%s", err, out)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("ふつうの git log で textconv が動かなかった（確かめ方がおかしい）: %v", err)
	}
	os.Remove(marker)

	s := &core.Session{ID: "s1", Project: filepath.Base(dir), ProjectPath: dir, Start: float64(base), End: float64(base + 1200)}
	cs, _, _ := NewCache().Collect([]*core.Session{s})
	if len(cs) != 2 {
		t.Fatalf("コミット数 = %d, want 2: %+v", len(cs), cs)
	}
	if cs[1].Added != 2 {
		t.Errorf("変更行数 = %+v, want +2（textconv を通さない）", cs[1])
	}
	if b, err := os.ReadFile(marker); err == nil {
		t.Errorf("リポジトリのプログラムが動いた:\n%s", b)
	}
	// kiroku がいま使うコマンドでは動かないものもあるので、ふつうなら動かすコマンドも gitCmd で動かして確かめる
	// （status → fsmonitor、log -p → textconv と外部 diff、update-ref → reference-transaction のフック）
	for _, args := range [][]string{{"status"}, {"log", "-p"}, {"update-ref", "refs/heads/x", "HEAD"}} {
		if out, err := gitCmd(context.Background(), dir, args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		if b, err := os.ReadFile(marker); err == nil {
			t.Errorf("git %v でリポジトリのプログラムが動いた:\n%s", args, b)
			os.Remove(marker)
		}
	}
}

// git に渡す設定と環境変数。
func TestGitCmd(t *testing.T) {
	cmd := gitCmd(context.Background(), "/repo", "log", "--all")
	args := strings.Join(cmd.Args[1:], " ")
	for _, want := range []string{"-C /repo", "-c core.fsmonitor=false", "-c core.hooksPath=" + os.DevNull, "-c log.showSignature=false", "-c core.pager=cat", "log --no-ext-diff --no-textconv --all"} {
		if !strings.Contains(args, want) {
			t.Errorf("引数 %q に %q がない", args, want)
		}
	}
	for _, want := range []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0"} {
		if !slices.Contains(cmd.Env, want) {
			t.Errorf("環境変数に %s がない", want)
		}
	}
	if args := strings.Join(gitCmd(context.Background(), "/repo", "rev-parse", "--show-toplevel").Args, " "); strings.Contains(args, "--no-textconv") {
		t.Errorf("log でないのに log の引数: %q", args)
	}
}

// Windows では、ネットワーク上の場所（UNC パス）を読みに行かない。
func TestRemotePath(t *testing.T) {
	for p, want := range map[string]bool{
		`\\server\share\repo`: true, `//server/share/repo`: true, `\\?\UNC\server\share`: true, `\/server/share`: true,
		`\\?\C:\repo`: false, `\\.\C:\repo`: false, `C:\repo`: false, `/home/me/repo`: false, `C:/repo`: false,
	} {
		if got := remotePath("windows", p); got != want {
			t.Errorf("remotePath(windows, %q) = %v, want %v", p, got, want)
		}
	}
	if remotePath("linux", "//server/share") {
		t.Error("Windows でなければ // も手元のパス")
	}
	if runtime.GOOS == "windows" {
		if cs, _, _ := NewCache().Collect([]*core.Session{{ID: "s", ProjectPath: `\\kiroku-test-nonexistent.invalid\share\repo`, Start: 1}}); len(cs) != 0 {
			t.Errorf("UNC パスを読んだ: %+v", cs)
		}
	}
}
