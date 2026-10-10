package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

func branchLine(sid, uuid, at, text string) string {
	return fmt.Sprintf(`{"type":"user","sessionId":%q,"uuid":%q,"timestamp":"2026-10-05T%s:00Z","cwd":"/w/app","message":{"role":"user","content":%q}}`, sid, uuid, at, text)
}

func writeBranchFile(t *testing.T, p string, lines ...string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o700)
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func promptTexts(data []*core.Session) map[string]string {
	out := map[string]string{}
	for _, s := range data {
		var ps []string
		for _, p := range s.Prompts {
			ps = append(ps, p.Text)
		}
		out[s.ID] = strings.Join(ps, ",")
	}
	return out
}

// kiroku serve: 元の会話が書き換わり、写した行がもう元の会話になければ、写した先を読み直してその行を数える
// （元の会話のファイルの印が Unit.Tag に入る）。読み直した結果は、はじめから読んだときと同じ。
func TestLoadCacheRereadsBranchWhenOriginChanges(t *testing.T) {
	old := logw
	logw = io.Discard
	t.Cleanup(func() { logw = old })
	root := t.TempDir()
	o, b := filepath.Join(root, "p", "O.jsonl"), filepath.Join(root, "q", "B.jsonl")
	writeBranchFile(t, o, branchLine("O", "o1", "01:00", "O-1"), branchLine("O", "o2", "01:01", "O-2"))
	writeBranchFile(t, b, branchLine("O", "o1", "01:00", "O-1"), branchLine("O", "o2", "01:01", "O-2"), branchLine("B", "b1", "02:00", "B-1"))
	all := []source.Source{&source.Claude{Root: root}}
	want := map[string]bool{"claude": true}
	cache := newLoadCache()
	d, _ := collectCached(all, want, 15, cache)
	if got := promptTexts(d); got["B"] != "B-1" || got["O"] != "O-1,O-2" {
		t.Fatalf("はじめ = %v", got)
	}
	writeBranchFile(t, o, branchLine("O", "o1", "01:00", "O-1"), branchLine("O", "zz", "01:05", "O-new"))
	os.Chtimes(o, time.Now(), time.Now().Add(time.Minute)) // 更新時刻が変わる
	d, _ = collectCached(all, want, 15, cache)
	fresh, _ := collect([]source.Source{&source.Claude{Root: root}}, want, 15)
	if got, w := promptTexts(d), promptTexts(fresh); got["B"] != "O-2,B-1" || got["B"] != w["B"] {
		t.Errorf("元の会話が変わったあと = %v, want はじめから読んだとき %v（O-2 を B で数える）", got, w)
	}
}
