package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/source"
)

// Kiro は 2 つの場所を「, 」で並べる。どちらも ~ で短くする
func TestWhereShortensEveryPath(t *testing.T) {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		t.Skip("no home folder")
	}
	other := filepath.Join(t.TempDir(), "sessions")
	got := where(filepath.Join(h, ".kiro", "sessions") + ", " + other + ", " + filepath.Join(h, "k2", "sessions"))
	want := filepath.Join("~", ".kiro", "sessions") + ", " + other + ", " + filepath.Join("~", "k2", "sessions")
	if got != want {
		t.Errorf("where = %q, want %q", got, want)
	}
	if got := where("none"); got != "not installed" {
		t.Errorf("where(none) = %q", got)
	}
}

// HOME が空なら、既定の場所はわからない（今のフォルダを読まない）。serve --check は「入っていない」ではなく、そう書く
func TestWhereWithoutHome(t *testing.T) {
	for _, k := range []string{"HOME", "USERPROFILE"} {
		t.Setenv(k, "")
	}
	if got := where(""); got != "folder unknown: HOME is not set" {
		t.Errorf("where(\"\") = %q", got)
	}
	if got := where("none"); got != "not installed" {
		t.Errorf("where(none) = %q", got)
	}
}

// HOME が空で場所がわからないエージェントは「none」ではなく「unknown」と出し、まとめと Next は HOME か場所のオプションを案内する
func TestDoctorWithoutHome(t *testing.T) {
	for _, k := range []string{"HOME", "USERPROFILE"} {
		t.Setenv(k, "")
	}
	fakeAutostart(t, "darwin")
	fakeKirokuPath(t, "/x/bin/kiroku", "/x/bin/kiroku")
	rep := []source.Report{{Name: "Claude Code"}, {Name: "Codex"}}
	var b bytes.Buffer
	doctorReport(&b, rep, 0, t.TempDir(), release{}, false)
	out := b.String()
	for _, want := range []string{"? Claude Code", "unknown", "folder unknown: HOME is not set", "HOME is not set, so kiroku couldn't look", "set HOME, or pass --claude-root, --kiro-home, --codex-home or --crew-home"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q がない:\n%s", want, out)
		}
	}
	for _, bad := range []string{"No history found", "use an agent for a while", " none\n"} {
		if strings.Contains(out, bad) {
			t.Errorf("%q が出ている:\n%s", bad, out)
		}
	}
	// ほかのエージェントは --claude-root などで読めても、場所がわからないものはまとめずに出す
	b.Reset()
	doctorReport(&b, []source.Report{{Name: "Claude Code", N: 2, Where: "/x/projects"}, {Name: "Codex"}}, 2, t.TempDir(), release{}, false)
	if out := b.String(); strings.Contains(out, "no history yet") || !strings.Contains(out, "? Codex") || !strings.Contains(out, "marked ?") {
		t.Errorf("場所がわからないエージェントがまとめられている:\n%s", out)
	}
}
