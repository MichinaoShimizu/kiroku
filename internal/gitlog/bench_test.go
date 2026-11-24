package gitlog

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// benchData は、nRepo 個のリポジトリ（それぞれ nDir 個のサブディレクトリでセッションがある）を作る。
// kiroku serve の読み直しで、変わっていないリポジトリを確かめるのにかかる時間を測るため。
func benchData(b *testing.B, nRepo, nDir int) []*core.Session {
	b.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		b.Skip("git がない")
	}
	root, start := b.TempDir(), float64(time.Now().Add(-time.Hour).Unix())
	var data []*core.Session
	for i := range nRepo {
		dir := filepath.Join(root, fmt.Sprintf("r%d", i))
		os.MkdirAll(dir, 0o755)
		for _, a := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "me@example.com"}, {"config", "user.name", "me"}} {
			if out, err := exec.Command("git", append([]string{"-C", dir}, a...)...).CombinedOutput(); err != nil {
				b.Fatalf("git %v: %v %s", a, err, out)
			}
		}
		os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
		for _, a := range [][]string{{"add", "a.txt"}, {"commit", "-q", "-m", "a"}} {
			if out, err := exec.Command("git", append([]string{"-C", dir}, a...)...).CombinedOutput(); err != nil {
				b.Fatalf("git %v: %v %s", a, err, out)
			}
		}
		for j := range nDir {
			p := filepath.Join(dir, fmt.Sprintf("d%d", j))
			os.MkdirAll(p, 0o755)
			data = append(data, &core.Session{ID: fmt.Sprintf("s%d-%d", i, j), Project: fmt.Sprintf("r%d", i), ProjectPath: p, Start: start})
		}
	}
	return data
}

// 変わっていないリポジトリの読み直し（kiroku serve で履歴が変わるたびに走る）
func BenchmarkCollectUnchanged(b *testing.B) {
	data := benchData(b, 20, 5)
	c := NewCache()
	c.Collect(data)
	b.ResetTimer()
	for range b.N {
		c.Collect(data)
	}
}

// 最初の読み込み
func BenchmarkCollectFirst(b *testing.B) {
	data := benchData(b, 20, 5)
	b.ResetTimer()
	for range b.N {
		NewCache().Collect(data)
	}
}
