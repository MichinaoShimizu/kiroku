//go:build !windows

package gitlog

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// 他人が作ったかもしれないリポジトリで、画面からコピーした git show を貼っても、そのリポジトリの設定（.git/config・
// .gitattributes）にある外部の diff・textconv・filter.*.process・fsmonitor・ページャーのコマンドが動かない（セキュリティ）。
func TestPastedGitShowRunsNoRepoCommands(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	dir, marks := t.TempDir(), t.TempDir()
	run(t, dir, nil, "init", "-q")
	run(t, dir, nil, "config", "user.email", "me@example.com")
	run(t, dir, nil, "config", "user.name", "me")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("a.txt filter=evil diff=evil\n"), 0o644)
	run(t, dir, nil, "add", ".")
	run(t, dir, nil, "commit", "-q", "-m", "c")
	mark := func(name string) string { return "sh -c 'touch " + filepath.Join(marks, name) + "; cat'" }
	for k, v := range map[string]string{
		"filter.evil.process": mark("process"), "filter.evil.smudge": mark("smudge"), "filter.evil.clean": mark("clean"),
		"diff.evil.textconv": mark("textconv"), "diff.external": mark("external"), "core.pager": mark("pager"),
		"core.fsmonitor": mark("fsmonitor"),
	} {
		run(t, dir, nil, "config", k, v)
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	cmd := core.GitCmd(dir, core.GitShow, strings.TrimSpace(string(out)))
	if cmd == "" {
		t.Fatal("コマンドが出ない")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	sh := exec.CommandContext(ctx, "sh", "-c", cmd) // 人が端末に貼るのと同じ
	sh.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if b, err := sh.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", cmd, err, b)
	}
	if ms, _ := os.ReadDir(marks); len(ms) > 0 {
		var names []string
		for _, m := range ms {
			names = append(names, m.Name())
		}
		t.Errorf("%s がリポジトリの設定のコマンドを動かした: %v", cmd, names)
	}
}
