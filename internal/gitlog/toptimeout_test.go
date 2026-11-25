package gitlog

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// どのリポジトリにあるかを調べる git rev-parse が時間切れになっても、「リポジトリでない」とは覚えない。
// そのときは stale で知らせ、次の読み直しで調べ直してコミットを出す（TopTTL のあいだ黙って落とさない）
func TestCacheRevParseTimeoutIsNotRemembered(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	dir, _ := newRepo(t, "app")
	hanging := true
	calls := spyGit(t, func(dir string, args []string) bool { return hanging && args[0] == "rev-parse" })
	RepoTimeout = 200 * time.Millisecond
	data := []*core.Session{{ID: "s1", Project: "app", ProjectPath: dir, Start: float64(time.Now().Add(-time.Hour).Unix())}}
	c := NewCache()
	cs, _, stale := c.Collect(data)
	if len(cs) != 0 || len(stale) != 1 || stale[0] != dir {
		t.Fatalf("rev-parse が時間切れ: コミット %d、stale %v, want 0 と [%s]", len(cs), stale, dir)
	}

	hanging = false
	RepoTimeout = 30 * time.Second
	clear(calls)
	cs, _, stale = c.Collect(data)
	if len(cs) != 1 || stale != nil || calls["rev-parse"] != 1 {
		t.Errorf("git が戻ったあと: コミット %d、stale %v、rev-parse %d, want 1・なし・1", len(cs), stale, calls["rev-parse"])
	}

	// リポジトリだとわかっている作業場所は、調べ直しが時間切れでも前の答えで読む
	TopTTL = 0
	t.Cleanup(func() { TopTTL = 10 * time.Minute })
	hanging = true
	RepoTimeout = 3 * time.Second
	cs, _, stale = c.Collect(data)
	if len(cs) != 1 || stale != nil {
		t.Errorf("前の答えがあるときの時間切れ: コミット %d、stale %v, want 1 となし", len(cs), stale)
	}
}

// git が「リポジトリでない」と答えた作業場所は確かな答え（覚えておく）。時間切れ・取り消しは確かでない
func TestRepoOfSure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	plain := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(plain)) // 一時フォルダの上にあるリポジトリを見つけない
	if tc, sure := repoOf(context.Background(), plain); tc[0] != "" || !sure {
		t.Errorf("リポジトリでない場所: %v %v, want 空で確か", tc, sure)
	}
	if _, sure := repoOf(context.Background(), filepath.Join(plain, "missing")); !sure {
		t.Error("ない場所: 確かでない, want 確か")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if tc, sure := repoOf(ctx, plain); tc[0] != "" || sure {
		t.Errorf("取り消した: %v %v, want 空で確かでない", tc, sure)
	}
}

// 同じ秒のコミットも、毎回同じ順（親が先）に並ぶ
func TestCollectSameSecondCommitsAreOrdered(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	dir := filepath.Join(t.TempDir(), "app")
	os.MkdirAll(dir, 0o755)
	run(t, dir, nil, "init", "-q", "-b", "main")
	run(t, dir, nil, "config", "user.email", "me@example.com")
	run(t, dir, nil, "config", "user.name", "me")
	at := time.Now().Add(-time.Hour).Truncate(time.Second).Format(time.RFC3339)
	env := []string{"GIT_AUTHOR_DATE=" + at, "GIT_COMMITTER_DATE=" + at}
	const n = 30 // 12 件を超えると、安定でない並べ替えは同じ値の順を崩す
	for i := range n {
		name := fmt.Sprintf("f%02d.txt", i)
		os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644)
		run(t, dir, nil, "add", name)
		run(t, dir, env, "commit", "-q", "-m", name)
	}
	s := &core.Session{ID: "s1", Project: "app", ProjectPath: dir, Start: float64(time.Now().Add(-2 * time.Hour).Unix())}
	cs, _, _ := NewCache().Collect([]*core.Session{s})
	if len(cs) != n {
		t.Fatalf("コミット = %d, want %d", len(cs), n)
	}
	for i, c := range cs {
		if want := fmt.Sprintf("f%02d.txt", i); c.Subject != want {
			t.Fatalf("%d 番目 = %s, want %s（古い順）", i, c.Subject, want)
		}
	}
}

// 絶対パスでない作業場所（相対パス・空）は、git に渡さない（今のフォルダのリポジトリを読まない）
func TestCollectSkipsRelativeWorkingDirs(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がない")
	}
	dir, _ := newRepo(t, "app")
	t.Chdir(dir)
	calls := spyGit(t, nil)
	start := float64(time.Now().Add(-time.Hour).Unix())
	data := []*core.Session{{ID: "s1", Project: "app", ProjectPath: ".", Start: start}, {ID: "s2", Project: "x", ProjectPath: "app", Start: start},
		{ID: "s3", Project: "y", ProjectPath: "", Start: start}}
	cs, ps, stale := NewCache().Collect(data)
	if len(cs) != 0 || len(ps) != 0 || stale != nil || len(calls) != 0 {
		t.Errorf("相対パス: コミット %d、push %d、stale %v、git の呼び出し %v, want なし", len(cs), len(ps), stale, calls)
	}
}
