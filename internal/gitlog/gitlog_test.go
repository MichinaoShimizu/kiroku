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
	base := time.Now().Add(-2 * time.Hour).Unix()
	commit := func(file, msg, email string, at int64) {
		os.WriteFile(filepath.Join(dir, file), []byte("a\nb\n"), 0o644)
		run(t, dir, nil, "add", file)
		date := time.Unix(at, 0).Format(time.RFC3339)
		run(t, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date, "GIT_AUTHOR_EMAIL=" + email, "GIT_COMMITTER_EMAIL=" + email},
			"commit", "-q", "-m", msg)
	}
	commit("a.txt", "by agent", "me@example.com", base+600)
	commit("b.txt", "by hand", "me@example.com", base+3600)
	commit("c.txt", "someone else", "other@example.com", base+4000)

	aiAt := float64(base + 590)
	s := &core.Session{ID: "s1", Project: filepath.Base(dir), ProjectPath: dir, Start: float64(base), End: float64(base + 4000),
		OEv: []core.Output{{T: &aiAt, Kind: "commit", V: 1}}}
	cs := Collect([]*core.Session{s, {ID: "s2", ProjectPath: filepath.Join(dir, "missing"), Start: float64(base)}})
	if len(cs) != 2 {
		t.Fatalf("コミット数 = %d, want 2（ほかの人のコミットは読まない）: %+v", len(cs), cs)
	}
	if cs[0].Subject != "by agent" || !cs[0].AI || cs[0].Added != 2 || cs[0].Project != filepath.Base(dir) {
		t.Errorf("1 つめ = %+v, want by agent・AI・+2", cs[0])
	}
	if cs[1].Subject != "by hand" || cs[1].AI {
		t.Errorf("2 つめ = %+v, want by hand・手で", cs[1])
	}
}
