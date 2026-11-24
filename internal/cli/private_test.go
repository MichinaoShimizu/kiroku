package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// tools/html が書くファイルは、ほかのユーザーが読めない（0600）。前からある 0644 のファイルも 0600 にし、
// 置き場所にシンボリックリンクがあっても、その先は書きかえない。書きかけの一時ファイルは残さない。
func TestHTMLIsPrivate(t *testing.T) {
	setup(t)
	h := filepath.Join("testdata", "home")
	dir := t.TempDir()
	common := []string{"--claude-root", filepath.Join(h, ".claude", "projects"), "--kiro-home", filepath.Join(h, ".kiro"), "--sources", "claude,kiro"}
	for _, c := range []struct{ cmd, name string }{{"html", "k.html"}} {
		out := filepath.Join(dir, c.name)
		if err := os.WriteFile(out, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		args := append([]string{"-o", out, "--no-open"}, common...)
		captureOutput(t, func() {
			if err := WriteHTML(args); err != nil {
				t.Fatal(err)
			}
		})
		st, err := os.Stat(out)
		if err != nil || st.Size() < 100 {
			t.Fatalf("%s: 書かれていない: %v", c.cmd, err)
		}
		if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
			t.Errorf("%s: 権限 = %v, want 0600", c.cmd, st.Mode().Perm())
		}

		// 置き場所がシンボリックリンクなら、リンクを置きかえる（リンクの先は書きかえない）
		if runtime.GOOS == "windows" {
			continue // シンボリックリンクを作るには権限がいる
		}
		target := filepath.Join(dir, "target-"+c.name)
		os.WriteFile(target, []byte("keep"), 0o644)
		link := filepath.Join(dir, "link-"+c.name)
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		args = append([]string{"-o", link}, args[2:]...)
		captureOutput(t, func() {
			if err := WriteHTML(args); err != nil {
				t.Fatal(err)
			}
		})
		if b, _ := os.ReadFile(target); string(b) != "keep" {
			t.Errorf("%s: リンクの先を書きかえた: %.40q", c.cmd, b)
		}
		if st, err := os.Lstat(link); err != nil || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm() != 0o600 {
			t.Errorf("%s: リンクが 0600 のファイルに置きかわっていない: %v %v", c.cmd, st.Mode(), err)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".kiroku-tmp-*")); len(left) > 0 {
		t.Errorf("一時ファイルが残った: %v", left)
	}
	// 書けない場所ならエラーで、何も作らない
	if err := writePrivate(filepath.Join(dir, "no-such-dir", "k.html"), []byte("x")); err == nil {
		t.Error("ないフォルダに書けた")
	}
}
