package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/archive"
)

// kiroku archive off で「消す」と答えると、kiroku が書いたコピー（.zst）だけを消し、ほかのファイルは残す。
// kiroku archive on をしたことのないフォルダ（まちがった --archive-dir）では、消すかどうかを聞きもしない。
func TestArchiveOffDeletesOnlyCopies(t *testing.T) {
	old := askYes
	defer func() { askYes = old }()
	asked := 0
	askYes = func(string) bool { asked++; return true }

	root, dir := t.TempDir(), filepath.Join(t.TempDir(), "archive")
	writeClaude(t, root, "s1")
	flags := []string{"--root", root, "--sources", "claude", "--archive-dir", dir}
	captureOutput(t, func() {
		if err := dispatch(append([]string{"archive", "on"}, flags...)); err != nil {
			t.Fatal(err)
		}
	})
	note := filepath.Join(dir, "claude", "-Users-me-app", "notes.txt")
	if err := os.WriteFile(note, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := captureOutput(t, func() {
		if err := dispatch(append([]string{"archive", "off"}, flags...)); err != nil {
			t.Fatal(err)
		}
	})
	if asked != 1 || !strings.Contains(out, "deleted the kept copies (1 file)") {
		t.Errorf("聞いた回数 %d、出力 %q", asked, out)
	}
	if files, _ := archive.Usage(dir); files != 0 {
		t.Errorf("コピーが残った: %d", files)
	}
	if b, err := os.ReadFile(note); err != nil || string(b) != "mine" {
		t.Errorf(".zst でないファイルを消した: %v", err)
	}

	// kiroku archive on をしたことのないフォルダ
	other := t.TempDir()
	keep := filepath.Join(other, "claude", "photos", "a.zst")
	os.MkdirAll(filepath.Dir(keep), 0o700)
	os.WriteFile(keep, []byte("x"), 0o600)
	asked = 0
	out = captureOutput(t, func() {
		if err := dispatch([]string{"archive", "off", "--archive-dir", other}); err != nil {
			t.Fatal(err)
		}
	})
	if asked != 0 || !strings.Contains(out, "does not look like a kiroku archive folder") {
		t.Errorf("聞いた回数 %d、出力 %q", asked, out)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("ほかのフォルダのファイルを消した: %v", err)
	}
}
