package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// 履歴のファイルは信用しない（書きかけ・壊れた・わざと細工したもの）。どんな中身でも、
// 読みこみと集計は止まらず（panic しない）、読むファイルは履歴の場所の外へ出ない。

// seed は testdata の実物に近いファイルを種にする。
func seed(f *testing.F, pattern string) {
	f.Helper()
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", pattern))
	if len(files) == 0 {
		f.Fatalf("no seed files for %s", pattern)
	}
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
}

func put(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// loadAll は Load のあと、出てきた会話を集計まで通す。
func loadAll(t *testing.T, s Source) []*core.Builder {
	t.Helper()
	var out []*core.Builder
	_ = s.Load(func(b *core.Builder) { out = append(out, b) }) // 壊れたファイルのエラーは期待どおり
	for _, b := range out {
		b.Finish(15)
	}
	return out
}

func FuzzClaude(f *testing.F) {
	seed(f, "home/.claude/projects/*/*.jsonl")
	f.Fuzz(func(t *testing.T, data []byte) {
		root := t.TempDir()
		put(t, filepath.Join(root, "-Users-me-app", "s1.jsonl"), data)
		put(t, filepath.Join(root, "-Users-me-app", "s1", "subagents", "agent-1.jsonl"), data)
		loadAll(t, &Claude{Root: root})
	})
}

func FuzzCodex(f *testing.F) {
	seed(f, "codex/sessions/*/*/*/*.jsonl")
	f.Fuzz(func(t *testing.T, data []byte) {
		home := t.TempDir()
		put(t, filepath.Join(home, "sessions", "2026", "09", "30", "rollout-2026-09-30T14-00-00-thr-a.jsonl"), data)
		put(t, filepath.Join(home, "session_index.jsonl"), data)
		loadAll(t, &Codex{Home: home})
	})
}

func FuzzKiroCLI(f *testing.F) {
	seed(f, "home/.kiro/sessions/cli/*")
	f.Fuzz(func(t *testing.T, data []byte) {
		home := t.TempDir()
		put(t, filepath.Join(home, "sessions", "cli", "s1.json"), data)
		put(t, filepath.Join(home, "sessions", "cli", "s1.jsonl"), data)
		loadAll(t, &KiroCLI{Home: home, CrewHome: filepath.Join(home, "crew")})
	})
}

func FuzzKiroIDE(f *testing.F) {
	seed(f, "home/.kiro/sessions/abc/*/*")
	f.Fuzz(func(t *testing.T, data []byte) {
		home := t.TempDir()
		put(t, filepath.Join(home, "sessions", "abc", "sess_1", "session.json"), data)
		put(t, filepath.Join(home, "sessions", "abc", "sess_1", "messages.jsonl"), data)
		loadAll(t, &KiroIDE{Home: home})
	})
}

// 古い Kiro IDE の sessions.json は、読む会話ファイルの名前（sessionId）を持っている。
// どんな中身でも、読むのは同じ workspace-sessions/<ws> の中のファイルだけ。
func FuzzKiroIDELegacy(f *testing.F) {
	seed(f, "home/.config/Kiro/User/globalStorage/kiro.kiroagent/workspace-sessions/*/sessions.json")
	f.Add([]byte(`[{"sessionId":"../../secret","title":"x"}]`))
	f.Add([]byte(`[{"sessionId":"..","title":"x"},{"sessionId":"a/../../b"}]`))
	f.Fuzz(func(t *testing.T, data []byte) {
		gs := t.TempDir()
		ws := filepath.Join(gs, "workspace-sessions", "d3M=")
		put(t, filepath.Join(ws, "sessions.json"), data)
		put(t, filepath.Join(gs, "secret.json"), []byte(`{"title":"outside"}`))
		for _, b := range loadAll(t, &KiroIDELegacy{Storages: []string{gs}}) {
			if rel, err := filepath.Rel(ws, b.File); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || strings.ContainsRune(rel, filepath.Separator) {
				t.Fatalf("read %q outside %q", b.File, ws)
			}
		}
	})
}
