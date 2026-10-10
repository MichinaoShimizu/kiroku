package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

func userLine(session, uuid, text string) string {
	return fmt.Sprintf(`{"type":"user","sessionId":%q,"uuid":%q,"timestamp":"2026-09-30T01:00:00Z","cwd":"/Users/me/app","message":{"role":"user","content":%q}}`, session, uuid, text)
}

func promptsBy(t *testing.T, c *Claude) map[string]int {
	t.Helper()
	got := map[string]int{}
	for _, b := range load(t, c) {
		if s := b.Finish(15); s != nil {
			got[s.ID] = len(s.Prompts)
		}
	}
	return got
}

// 2 つの会話が、同じ行（uuid）を互いを元と書いて持っていても、両方で数えなくなることはない。
// 行を数えないのは、元の会話にその行が元の会話自身の行（ほかの会話を元と書いていない行）としてあるときだけ。
func TestClaudeBranchMutualReference(t *testing.T) {
	root := t.TempDir()
	writeLines(t, filepath.Join(root, "p1", "AAAA.jsonl"), []string{userLine("BBBB", "x1", "一"), userLine("BBBB", "x2", "二")})
	writeLines(t, filepath.Join(root, "p2", "BBBB.jsonl"), []string{userLine("AAAA", "x1", "一"), userLine("AAAA", "x2", "二")})
	if got := promptsBy(t, &Claude{Root: root}); got["AAAA"] != 2 || got["BBBB"] != 2 {
		t.Errorf("依頼の数 = %v, want どちらも 2（どちらの行も自分の行として持つ会話がないので、両方で数える）", got)
	}
}

// 1 つのファイルに、別々の会話を元と書いた行がたくさんあっても、会話のファイルの一覧は 1 回だけ作り、
// 元の会話を読むのは maxOriginsPerUnit 個まで（超えた分の行は数える）。プロジェクトのフォルダが多くても遅くならない。
func TestClaudeBranchManyOrigins(t *testing.T) {
	root := t.TempDir()
	for i := range 300 {
		os.MkdirAll(filepath.Join(root, fmt.Sprintf("proj%d", i)), 0o700)
	}
	// 元の会話が 10 個あり、写した先にはそれぞれの行が 1 行ずつ。ほかに、ない会話を元と書いた行が 5,000 行
	// （ない会話を元と書いた行は先にあっても、読む数の上限を使わない）
	var lines []string
	for i := range 5000 {
		lines = append(lines, userLine(fmt.Sprintf("id%07d", i), fmt.Sprintf("u%d", i), "x"))
	}
	for i := range 10 {
		id := fmt.Sprintf("orig%02d", i)
		writeLines(t, filepath.Join(root, "proj1", id+".jsonl"), []string{userLine(id, "o"+id, "元")})
		lines = append(lines, userLine(id, "o"+id, "元"))
	}
	writeLines(t, filepath.Join(root, "proj0", "DOS.jsonl"), lines)
	before := originReads.Load()
	start := time.Now()
	got := promptsBy(t, &Claude{Root: root})
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("読むのに %v かかった", d)
	}
	if n := originReads.Load() - before; n > maxOriginsPerUnit {
		t.Errorf("元の会話を %d 回読んだ, want %d 回まで", n, maxOriginsPerUnit)
	}
	if want := 5000 + 10 - maxOriginsPerUnit; got["DOS"] != want {
		t.Errorf("DOS の依頼 = %d, want %d（確かめた %d 個の元の会話の行だけ数えない）", got["DOS"], want, maxOriginsPerUnit)
	}
}

func BenchmarkClaudeBranchManyOrigins(b *testing.B) {
	root := b.TempDir()
	for i := range 999 {
		os.MkdirAll(filepath.Join(root, fmt.Sprintf("proj%d", i)), 0o700)
	}
	var sb strings.Builder
	for i := range 20000 {
		sb.WriteString(userLine(fmt.Sprintf("id%07d", i), fmt.Sprintf("u%d", i), "x") + "\n")
	}
	os.WriteFile(filepath.Join(root, "proj0", "DOS.jsonl"), []byte(sb.String()), 0o600)
	for b.Loop() {
		(&Claude{Root: root}).Load(func(*core.Builder) {})
	}
}

// 元の会話は、それを元と書いた会話がいくつあっても 1 回だけ読む。長すぎる uuid の行は突き合わせず数え、
// 覚える行の印は maxOriginKeys 個まで（超えた分の行は、写した先で数える）。
func TestClaudeBranchOriginBounds(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("a", maxKeyLen+1)
	orig := []string{userLine("ORIG", long, "長い"), userLine("ORIG", "k1", "一"), userLine("ORIG", "k2", "二"), userLine("ORIG", "k3", "三")}
	writeLines(t, filepath.Join(root, "p", "ORIG.jsonl"), orig)
	for i := range 4 {
		writeLines(t, filepath.Join(root, "p", fmt.Sprintf("REF%d.jsonl", i)), orig)
	}
	old := maxOriginKeys
	maxOriginKeys = 2 // 長い行は印にならないので、k1 と k2 だけを覚える
	t.Cleanup(func() { maxOriginKeys = old })
	before := originReads.Load()
	got := promptsBy(t, &Claude{Root: root})
	if n := originReads.Load() - before; n != 1 {
		t.Errorf("元の会話を %d 回読んだ, want 1 回", n)
	}
	for i := range 4 {
		if id := fmt.Sprintf("REF%d", i); got[id] != 2 {
			t.Errorf("%s の依頼 = %d, want 2（長い uuid の行と、覚えきれなかった k3 は数える）", id, got[id])
		}
	}
	if got["ORIG"] != 4 {
		t.Errorf("ORIG の依頼 = %d, want 4", got["ORIG"])
	}
}
