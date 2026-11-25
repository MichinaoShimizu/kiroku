package source

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// noHomeCwd は、今のフォルダに、ホームの下と同じ形の履歴（.claude/projects・.kiro・.kiro/crew・.codex・.local/share/kiro-cli）と、
// その場所が "" のときに読んでしまう相対パスの形（<project>/*.jsonl・sessions/…・usage/…）を置く。
func noHomeCwd(t *testing.T) string {
	t.Helper()
	td, err := filepath.Abs(filepath.Join("..", "..", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	cp := func(src, dst string) {
		t.Helper()
		if err := os.CopyFS(filepath.Join(cwd, dst), os.DirFS(filepath.Join(td, src))); err != nil {
			t.Fatal(err)
		}
	}
	cp("home/.claude/projects", ".claude/projects")
	cp("home/.kiro", ".kiro")
	cp("crew", ".kiro/crew")
	cp("codex", ".codex")
	cp("home/.claude/projects", ".")      // Root "" → "*/*.jsonl"
	cp("home/.kiro/sessions", "sessions") // Kiro の Home "" → "sessions/…"
	cp("codex/sessions", "sessions")      // Codex の Home "" → "sessions/…"
	cp("crew/usage", "usage")             // Crew の場所 "" → "usage/tokens/…"
	t.Chdir(cwd)
	for _, k := range []string{"CLAUDE_CONFIG_DIR", "KIRO_HOME", "KIROCREW_HOME", "CODEX_HOME", "XDG_DATA_HOME", "LOCALAPPDATA"} {
		t.Setenv(k, "")
	}
	// 古い Kiro CLI の data.sqlite3 は OS ごとの場所（今のフォルダを HOME にしたときの DataDir）に置く
	setHome(t, cwd)
	dbPath := kiroCLIDB()
	db, err := os.ReadFile(filepath.Join(td, "sqlite", "kiro-cli.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, db, 0o600); err != nil {
		t.Fatal(err)
	}
	return cwd
}

// setHome は、ホームを h にする（Windows の os.UserHomeDir は USERPROFILE を見る）。
func setHome(t *testing.T, h string) {
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
}

func defaultSources() []Source {
	return All(Options{ClaudeRoot: DefaultClaudeRoot(), KiroHome: DefaultKiroHome(), KiroLegacy: DefaultKiroLegacyHome()})
}

func countSessions(s Source) int {
	n := 0
	_ = s.Load(func(*core.Builder) { n++ })
	return n
}

// HOME が空で、場所を決める環境変数もないとき、既定の履歴の場所は今のフォルダからの相対パスにならない。
// 今のフォルダに履歴があっても、どのエージェントも何も読まず、見張りもしない。
func TestNoHomeReadsNothingFromCwd(t *testing.T) {
	cwd := noHomeCwd(t)
	// 同じフォルダを HOME にすれば、どのエージェントも読める（置いた履歴が読める形であることの確かめ）
	for _, s := range defaultSources() {
		if n := countSessions(s); n == 0 {
			t.Errorf("HOME=%s: %s read nothing; the fixture should be readable", cwd, s.Name())
		}
	}

	setHome(t, "")
	for name, got := range map[string]string{
		"DefaultClaudeRoot": DefaultClaudeRoot(), "DefaultKiroHome": DefaultKiroHome(), "DefaultKiroLegacyHome": DefaultKiroLegacyHome(),
		"DefaultCodexHome": DefaultCodexHome(), "DefaultCrewHome": DefaultCrewHome(), "kiroCLIDB": kiroCLIDB(),
	} {
		if got != "" {
			t.Errorf("%s = %q with HOME unset, want \"\"", name, got)
		}
	}
	for _, s := range defaultSources() {
		if n := countSessions(s); n != 0 {
			t.Errorf("%s read %d sessions from the current folder with HOME unset", s.Name(), n)
		}
		if w := s.Where(); w != "" {
			t.Errorf("%s: Where = %q, want \"\"", s.Name(), w)
		}
		if ps := WatchPaths(s); len(ps) != 0 {
			t.Errorf("%s watches %q", s.Name(), ps)
		}
		if sp, ok := s.(Splitter); ok {
			if us := sp.Units(); len(us) != 0 {
				t.Errorf("%s: %d units from the current folder", s.Name(), len(us))
			}
		}
		if r, ok := s.(interface{ Retention() *Retention }); ok && r.Retention() != nil {
			t.Errorf("%s: retention read from the current folder: %+v", s.Name(), r.Retention())
		}
		if k, ok := s.(interface{ Keep() []Kept }); ok {
			if kept := k.Keep(); len(kept) != 0 {
				t.Errorf("%s keeps %v", s.Name(), kept)
			}
		}
	}
}
