package gitlog

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

func run(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// 自分のコミットだけを読み、エージェントが実行したコミットには印をつける。
func TestCollect(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	dir := t.TempDir()
	run(t, dir, nil, "init", "-q")
	run(t, dir, nil, "config", "user.email", "me@example.com")
	run(t, dir, nil, "config", "user.name", "me")
	run(t, dir, nil, "remote", "add", "origin", "git@github.com:me/app.git")
	base := time.Now().Add(-2 * time.Hour).Unix()
	commit := func(file, msg, email string, at int64) {
		os.WriteFile(filepath.Join(dir, file), []byte("a\nb\n"), 0o644)
		run(t, dir, nil, "add", file)
		date := time.Unix(at, 0).Format(time.RFC3339)
		run(t, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date, "GIT_AUTHOR_EMAIL=" + email, "GIT_COMMITTER_EMAIL=" + email},
			"commit", "-q", "-m", msg)
	}
	commit("a.txt", "by agent", "me@example.com", base+600)
	commit("b.txt", "by hand\n\nbody line", "me@example.com", base+3600)
	commit("c.txt", "someone else", "other@example.com", base+4000)

	aiAt := float64(base + 590)
	s := &core.Session{ID: "s1", Project: filepath.Base(dir), ProjectPath: dir, Start: float64(base), End: float64(base + 4000),
		OEv: []core.Output{{T: &aiAt, Kind: "commit", V: 1}}}
	cs := Collect([]*core.Session{s, {ID: "s2", ProjectPath: filepath.Join(dir, "missing"), Start: float64(base)}})
	if len(cs) != 2 {
		t.Fatalf("コミット数 = %d, want 2（ほかの人のコミットは読まない）: %+v", len(cs), cs)
	}
	if cs[0].Subject != "by agent" || !cs[0].AI || cs[0].Session != "s1" || cs[0].Added != 2 || cs[0].Project != filepath.Base(dir) {
		t.Errorf("1 つめ = %+v, want by agent・AI（s1）・+2", cs[0])
	}
	if c := cs[0]; c.NFiles != 1 || len(c.Files) != 1 || c.Files[0].Path != "a.txt" || c.Files[0].Added != 2 || c.Branch == "" || c.Repo == "" {
		t.Errorf("変更したファイル・ブランチ = %+v", c)
	}
	if c := cs[0]; len(c.Hash) != 40 || c.URL != "https://github.com/me/app/commit/"+c.Hash || c.Files[0].URL != "https://github.com/me/app/blob/"+c.Hash+"/a.txt" {
		t.Errorf("リンク = %q %q", c.URL, c.Files[0].URL)
	}
	if cs[1].Body != "body line" {
		t.Errorf("本文 = %q", cs[1].Body)
	}
	if cs[1].Subject != "by hand" || cs[1].AI {
		t.Errorf("2 つめ = %+v, want by hand・手で", cs[1])
	}
}

func TestWebURL(t *testing.T) {
	for in, want := range map[string]string{
		"git@github.com:owner/repo.git":                "https://github.com/owner/repo",
		"https://github.com/owner/repo.git":            "https://github.com/owner/repo",
		"https://user:token@gitlab.com/g/sub/repo.git": "https://gitlab.com/g/sub/repo",
		"ssh://git@git.example.com:2222/team/repo.git": "https://git.example.com/team/repo",
		"/srv/git/repo.git":                            "",
		"":                                             "",
	} {
		if got := WebURL(in); got != want {
			t.Errorf("WebURL(%q) = %q, want %q", in, got, want)
		}
	}
	if got := commitURL("https://bitbucket.org/o/r", "abc"); got != "https://bitbucket.org/o/r/commits/abc" {
		t.Errorf("bitbucket = %q", got)
	}
	if got := fileURL("https://github.com/o/r", "abc", "src/a.go"); got != "https://github.com/o/r/blob/abc/src/a.go" {
		t.Errorf("fileURL = %q", got)
	}
}

// git worktree は本体と同じリポジトリとしてまとめる。worktree でエージェントがしたコミットは、
// 本体のセッションが先に並んでいても AI のコミットになる。1 回の実行は 1 つのコミットにだけ結びつける。
func TestCollectWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	root := t.TempDir()
	dir, wt := filepath.Join(root, "app"), filepath.Join(root, "app-wt")
	os.MkdirAll(dir, 0o755)
	run(t, dir, nil, "init", "-q")
	run(t, dir, nil, "config", "user.email", "me@example.com")
	run(t, dir, nil, "config", "user.name", "me")
	base := time.Now().Add(-2 * time.Hour).Unix()
	commit := func(in, file, msg string, at int64) {
		os.WriteFile(filepath.Join(in, file), []byte("a\n"), 0o644)
		run(t, in, nil, "add", file)
		date := time.Unix(at, 0).Format(time.RFC3339)
		run(t, in, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}, "commit", "-q", "-m", msg)
	}
	commit(dir, "a.txt", "init", base)
	run(t, dir, nil, "worktree", "add", "-q", "-b", "feat", wt)
	commit(wt, "b.txt", "by agent in worktree", base+600)
	commit(wt, "c.txt", "by hand right after", base+660)

	aiAt := float64(base + 610)
	mainSes := &core.Session{ID: "main", Project: "app", ProjectPath: dir, Start: float64(base), End: float64(base + 100)}
	wtSes := &core.Session{ID: "wt", Project: "app-wt", ProjectPath: wt, Start: float64(base + 500), End: float64(base + 700),
		OEv: []core.Output{{T: &aiAt, Kind: "commit", V: 1}}}
	cs := Collect([]*core.Session{mainSes, wtSes})
	if len(cs) != 3 {
		t.Fatalf("コミット数 = %d, want 3（本体と worktree で重ねない）", len(cs))
	}
	got := map[string]Commit{}
	for _, c := range cs {
		got[c.Subject] = c
	}
	if c := got["by agent in worktree"]; !c.AI || c.Session != "wt" {
		t.Errorf("worktree でのエージェントのコミット = AI %v session %q, want AI・wt", c.AI, c.Session)
	}
	if c := got["by hand right after"]; c.AI {
		t.Error("1 回の実行は 1 つのコミットにだけ結びつける（直後に手で行ったコミットは AI にしない）")
	}
	want, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		// git は本当のパスを返す（macOS の /var → /private/var、Windows の短い名前など）ので、同じフォルダかで比べる
		got, err := os.Stat(c.Repo)
		if c.Project != "app" || err != nil || !os.SameFile(got, want) {
			t.Errorf("%q: project %q repo %q, want 本体の app・%s", c.Subject, c.Project, c.Repo, dir)
		}
	}
}
