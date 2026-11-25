package archive

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 止められて残った書きかけ（.kiroku-tmp-*）は、古ければ次の書き込みで、archive off ではすぐに消える。新しいものは残す
func TestStaleTempsRemoved(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, tmpPrefix+"old")
	fresh := filepath.Join(dir, tmpPrefix+"new")
	other := filepath.Join(dir, "notes.txt")
	for _, p := range []string{old, fresh, other} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-2 * staleTempAge)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	removeStaleTemps(dir, time.Now())
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old temp still there: %v", err)
	}
	for _, p := range []string{fresh, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s removed: %v", filepath.Base(p), err)
		}
	}

	root := t.TempDir()
	sub := filepath.Join(root, "claude", "-p")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	left := filepath.Join(sub, tmpPrefix+"123")
	if err := os.WriteFile(left, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if n, err := Clear(root, []string{"claude"}); err != nil || n != 0 {
		t.Fatalf("Clear = %d, %v; want 0, nil (temps aren't counted)", n, err)
	}
	if _, err := os.Stat(left); !os.IsNotExist(err) {
		t.Errorf("Clear left the temp: %v", err)
	}
}
