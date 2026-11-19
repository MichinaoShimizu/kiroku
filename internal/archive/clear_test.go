package archive

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Clear は kiroku が書いたコピー（.jsonl.zst）だけを消し、ほかのファイルとリンクの先は残す。空になったフォルダは消す。
func TestClear(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	files := map[string]string{
		"claude/-Users-me-app/s1.jsonl.zst":             "x",
		"claude/-Users-me-app/s1/subagents/a.jsonl.zst": "x",
		"crew/sessions/archive/k__1.jsonl.zst":          "x",
		"crew/sessions/archive/notes.txt":               "keep",
		"other/keep.zst":                                "keep", // subs にないフォルダは見ない
		"claude/-Users-me-other/important.jsonl":        "keep",
		"claude/-Users-me-other/foreign.zst":            "keep", // kiroku が書かない名前（.jsonl.zst でない）は残す
	}
	for p, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(full), 0o700)
		os.WriteFile(full, []byte(body), 0o600)
	}
	os.WriteFile(filepath.Join(outside, "target.zst"), []byte("keep"), 0o600)
	if runtime.GOOS != "windows" { // シンボリックリンクを作るには権限がいる
		if err := os.Symlink(filepath.Join(outside, "target.zst"), filepath.Join(dir, "claude", "link.zst")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(dir, "claude", "linkdir")); err != nil {
			t.Fatal(err)
		}
	}
	// 消すかを聞くときの数（Kept）は、消す数と同じ
	if k, _ := Kept(dir, []string{"claude", "crew", "missing"}); k != 3 {
		t.Errorf("Kept = %d, want 3", k)
	}
	n, err := Clear(dir, []string{"claude", "crew", "missing"})
	if err != nil || n != 3 {
		t.Errorf("Clear = %d, %v, want 3", n, err)
	}
	for p, body := range files {
		_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p)))
		if gone := os.IsNotExist(err); gone != (body == "x") {
			t.Errorf("%s: 消えた = %v", p, gone)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "claude", "-Users-me-app")); !os.IsNotExist(err) {
		t.Errorf("空になったフォルダが残った: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(outside, "target.zst")); err != nil || string(b) != "keep" {
		t.Errorf("リンクの先を消した: %v", err)
	}
}

// オンにした保存場所には印が残り、オフにしても kiroku archive の保存場所だとわかる。ほかのフォルダはそうでない。
func TestMarked(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "archive")
	if Marked(dir) || Marked(t.TempDir()) {
		t.Fatal("オンにしていないのに印がある")
	}
	if err := Enable(dir); err != nil || !Marked(dir) {
		t.Fatalf("Enable: %v", err)
	}
	if err := Disable(dir); err != nil || !Marked(dir) {
		t.Errorf("オフにしたら印が消えた: %v", err)
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(filepath.Join(dir, stamp))
		if err != nil || st.Mode().Perm() != 0o600 {
			t.Errorf("印の権限 = %v, %v", st.Mode(), err)
		}
	}
}

// 前からあって、ほかの人も読める保存場所とエージェントごとのフォルダは、オンにして残すときに自分だけが読めるようにする（セキュリティ）。
// 中のフォルダの名前はプロジェクトのパスなので、ほかの人に一覧を見せない。
func TestExistingDirsMadePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows には Unix の権限がない")
	}
	dir := filepath.Join(t.TempDir(), "archive")
	dst := filepath.Join(dir, "claude")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "projects")
	if err := os.MkdirAll(filepath.Join(src, "-Users-me-app"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "-Users-me-app", "s1.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Enable(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(src, dst); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{dir, dst} {
		if st, err := os.Stat(p); err != nil || st.Mode().Perm() != 0o700 {
			t.Errorf("%s: mode = %v, %v, want 0700", p, st.Mode().Perm(), err)
		}
	}
}
