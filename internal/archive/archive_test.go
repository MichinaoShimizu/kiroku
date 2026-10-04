package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

func TestSync(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	p := filepath.Join(src, "proj", "s1.jsonl")
	os.MkdirAll(filepath.Join(src, "proj", "s1", "subagents"), 0o755)
	os.WriteFile(p, []byte(`{"n":1}`+"\n"), 0o644)
	os.WriteFile(filepath.Join(src, "proj", "s1", "subagents", "agent-a.jsonl"), []byte(`{"n":2}`+"\n"), 0o644)
	os.WriteFile(filepath.Join(src, "proj", "notes.txt"), []byte("x"), 0o644) // .jsonl だけを残す
	os.WriteFile(filepath.Join(src, "proj", "empty.jsonl"), nil, 0o644)       // 空のファイルは残さない
	if n, err := Sync(src, dst); n != 2 || err != nil {
		t.Fatalf("1 回目: %d %v, want 2", n, err)
	}
	read := func(rel string) []float64 {
		var out []float64
		core.ReadJSONL(filepath.Join(dst, rel), func(e core.Obj) { out = append(out, core.NumOr0(e["n"])) })
		return out
	}
	if got := read("proj/s1.jsonl.zst"); len(got) != 1 || got[0] != 1 {
		t.Errorf("コピー = %v", got)
	}
	if got := read("proj/s1/subagents/agent-a.jsonl.zst"); len(got) != 1 || got[0] != 2 {
		t.Errorf("サブエージェントのコピー = %v", got)
	}
	if n, _ := Sync(src, dst); n != 0 {
		t.Errorf("変わっていないのに %d 件残し直した", n)
	}
	// 追記されたら、残し直す
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"n":3}` + "\n")
	f.Close()
	later := time.Now().Add(time.Minute)
	os.Chtimes(p, later, later)
	if n, _ := Sync(src, dst); n != 1 {
		t.Errorf("追記のあと %d 件, want 1", n)
	}
	if got := read("proj/s1.jsonl.zst"); len(got) != 2 {
		t.Errorf("追記のあとのコピー = %v", got)
	}
	// 元が消えても、コピーは残る
	os.RemoveAll(filepath.Join(src, "proj"))
	if n, _ := Sync(src, dst); n != 0 {
		t.Errorf("消えたあと %d 件", n)
	}
	if files, bytes := Usage(dst); files != 2 || bytes == 0 {
		t.Errorf("Usage = %d %d", files, bytes)
	}
	// 元の場所がなければ何もしない
	if n, err := Sync(filepath.Join(src, "nope"), dst); n != 0 || err != nil {
		t.Errorf("ない場所: %d %v", n, err)
	}
}

func TestEnable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "kiroku", "archive")
	if Enabled(dir) {
		t.Fatal("作る前からオン")
	}
	if err := Enable(dir); err != nil || !Enabled(dir) {
		t.Fatalf("Enable: %v", err)
	}
	if err := Disable(dir); err != nil || Enabled(dir) {
		t.Fatalf("Disable: %v", err)
	}
	if err := Disable(dir); err != nil {
		t.Errorf("2 回止めた: %v", err)
	}
}

func TestDefaultDir(t *testing.T) {
	t.Setenv("KIROKU_ARCHIVE_DIR", "/x/y")
	if DefaultDir() != "/x/y" {
		t.Errorf("KIROKU_ARCHIVE_DIR を使っていない: %s", DefaultDir())
	}
	t.Setenv("KIROKU_ARCHIVE_DIR", "")
	if d := DefaultDir(); !strings.HasSuffix(d, filepath.Join("kiroku", "archive")) {
		t.Errorf("DefaultDir = %s", d)
	}
}

func TestPath(t *testing.T) {
	if p, ok := Path("/a", "/b", "/a/p/s.jsonl"); !ok || p != filepath.Join("/b", "p", "s.jsonl.zst") {
		t.Errorf("Path = %s %v", p, ok)
	}
	if _, ok := Path("/a", "/b", "/c/s.jsonl"); ok {
		t.Error("元の場所の外のファイル")
	}
}

// 元の場所がシンボリックリンクでも、中のファイルを残す（WalkDir は起点のリンクをたどらない）
func TestSyncSymlinkRoot(t *testing.T) {
	real, dst := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(real, "proj"), 0o755)
	os.WriteFile(filepath.Join(real, "proj", "s1.jsonl"), []byte(`{"n":1}`+"\n"), 0o644)
	link := filepath.Join(t.TempDir(), "projects")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("シンボリックリンクを作れない:", err)
	}
	if n, err := Sync(link, dst); n != 1 || err != nil {
		t.Fatalf("Sync = %d %v, want 1", n, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "proj", "s1.jsonl.zst")); err != nil {
		t.Error("コピーがない:", err)
	}
}
