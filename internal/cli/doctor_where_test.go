package cli

import (
	"os"
	"path/filepath"
	"testing"
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
