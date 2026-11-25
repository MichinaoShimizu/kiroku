package archive

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// コピーを残すのは copyLimit までの大きさのファイルだけ。履歴の名前でリンクされた大きなファイル（ここでは穴のあいた sparse ファイル）は、
// リンクでもそうでなくても圧縮せず、名前をつけたエラー（ErrTooLarge）で知らせる。上限ちょうどのファイルと、ほかのファイルは残す。
func TestSyncSkipsFilesOverTheCap(t *testing.T) {
	old := maxCopy
	maxCopy = 1 << 20
	t.Cleanup(func() { maxCopy = old })
	if got := copyLimit(); got != 1<<20 {
		t.Fatalf("copyLimit = %d", got)
	}

	src, dst, elsewhere := t.TempDir(), t.TempDir(), t.TempDir()
	proj := filepath.Join(src, "proj")
	if err := os.MkdirAll(proj, 0o700); err != nil {
		t.Fatal(err)
	}
	sparse := func(p string, size int64) {
		t.Helper()
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Truncate(size); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	sparse(filepath.Join(proj, "big.jsonl"), maxCopy+1)
	sparse(filepath.Join(proj, "edge.jsonl"), maxCopy)
	os.WriteFile(filepath.Join(proj, "ok.jsonl"), []byte(`{"n":1}`+"\n"), 0o600)
	linked := false
	if runtime.GOOS != "windows" {
		target := filepath.Join(elsewhere, "disk.img")
		sparse(target, 64<<20)
		linked = os.Symlink(target, filepath.Join(proj, "linked.jsonl")) == nil
	}

	n, err := Sync(src, dst)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Sync error = %v, want ErrTooLarge", err)
	}
	if !strings.Contains(err.Error(), "1 MiB") {
		t.Errorf("error %q does not name the limit", err)
	}
	if n != 2 {
		t.Errorf("Sync kept %d files, want 2 (edge and ok)", n)
	}
	for name, want := range map[string]bool{"big": false, "linked": false, "edge": true, "ok": true} {
		if name == "linked" && !linked {
			continue
		}
		_, err := os.Stat(filepath.Join(dst, "proj", name+".jsonl.zst"))
		if got := err == nil; got != want {
			t.Errorf("copy of %s.jsonl exists = %v, want %v", name, got, want)
		}
	}
	if left := tmpLeft(t, dst); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}

	// compress も、開いたファイルの大きさで確かめる（Sync が見たあとでリンクの先が変わっても）
	if err := compress(filepath.Join(proj, "big.jsonl"), filepath.Join(dst, "x.zst")); !errors.Is(err, ErrTooLarge) {
		t.Errorf("compress = %v, want ErrTooLarge", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "x.zst")); err == nil {
		t.Error("compress wrote a copy of a file over the cap")
	}
}
