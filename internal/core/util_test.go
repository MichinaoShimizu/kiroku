package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// もうあるフォルダも 0700 にする。シンボリックリンクの先は変えない
func TestPrivateDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows doesn't use these permission bits")
	}
	root := t.TempDir()
	open := filepath.Join(root, "open")
	if err := os.Mkdir(open, 0o755); err != nil {
		t.Fatal(err)
	}
	os.Chmod(open, 0o755) // umask に左右されないように
	if err := PrivateDir(open); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(open); fi.Mode().Perm() != 0o700 {
		t.Fatalf("existing folder = %v, want 0700", fi.Mode().Perm())
	}
	if err := PrivateDir(filepath.Join(root, "a", "b")); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(filepath.Join(root, "a", "b")); fi.Mode().Perm() != 0o700 {
		t.Fatalf("new folder = %v, want 0700", fi.Mode().Perm())
	}
	target := filepath.Join(root, "target")
	os.Mkdir(target, 0o755)
	os.Chmod(target, 0o755)
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	if err := PrivateDir(link); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(target); fi.Mode().Perm() != 0o755 {
		t.Fatalf("symlink target = %v, want unchanged 0755", fi.Mode().Perm())
	}
}

// 履歴の負のトークン数は 0 にする（"Tokens -5"・"$-0.00" と出さない）
func TestReadUsageNegative(t *testing.T) {
	u := ReadUsage(map[string]any{"input_tokens": -5.0, "output_tokens": -1e9, "cache_read_input_tokens": -3.0, "cache_creation_input_tokens": -2.0,
		"cache_creation": map[string]any{"ephemeral_1h_input_tokens": -7.0, "ephemeral_5m_input_tokens": -1.0}})
	if u.In != 0 || u.Out != 0 || u.CR != 0 || u.CW != 0 || u.CW1h != 0 {
		t.Fatalf("usage = %+v, want all 0", u)
	}
	if Count(12.0) != 12 || Count("x") != 0 {
		t.Fatal("Count")
	}
}
