package archive

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// unzst は保存したコピーをほどいた中身。
func unzst(t *testing.T, p string) []byte {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := core.NewZstdReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	b, err := io.ReadAll(d)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// tmpLeft は、dir の下に置きかえ前の一時ファイルが残っていないか。
func tmpLeft(t *testing.T, dir string) []string {
	t.Helper()
	left, _ := filepath.Glob(filepath.Join(dir, "*", ".kiroku-tmp-*"))
	more, _ := filepath.Glob(filepath.Join(dir, ".kiroku-tmp-*"))
	return append(left, more...)
}

// コピーは元と同じ中身・同じ更新時刻で、ほかのユーザーには読めない（ファイル 0600、フォルダ 0700）。
func TestSyncCopyContentModeAndTime(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	p := filepath.Join(src, "proj", "deep", "s1.jsonl")
	body := []byte(`{"n":1,"text":"日本語とバイナリ\u0000"}` + "\n" + `{"n":2}` + "\n")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, body, 0o644)
	mt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	os.Chtimes(p, mt, mt)
	if n, err := Sync(src, dst); n != 1 || err != nil {
		t.Fatalf("Sync = %d %v, want 1", n, err)
	}
	out := filepath.Join(dst, "proj", "deep", "s1.jsonl.zst")
	if got := unzst(t, out); !bytes.Equal(got, body) {
		t.Errorf("中身 = %q, want %q", got, body)
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if !st.ModTime().Equal(mt) {
		t.Errorf("更新時刻 = %v, want %v", st.ModTime(), mt)
	}
	if runtime.GOOS == "windows" {
		return // 権限のビットは Windows では意味がない
	}
	if m := st.Mode().Perm(); m != 0o600 {
		t.Errorf("コピーの権限 = %o, want 600", m)
	}
	for _, d := range []string{filepath.Join(dst, "proj"), filepath.Join(dst, "proj", "deep")} {
		if st, err := os.Stat(d); err != nil || st.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s の権限 = %v %v, want ほかのユーザーに開かない", d, st.Mode().Perm(), err)
		}
	}
}

// 1 つのファイルを残せなくても、ほかのファイルは残し、最初の失敗を返す。一時ファイルは残さない。
func TestSyncContinuesAfterFailure(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(src, "a"), 0o755)
	os.MkdirAll(filepath.Join(src, "b"), 0o755)
	os.WriteFile(filepath.Join(src, "a", "bad.jsonl"), []byte(`{"n":1}`+"\n"), 0o644)
	os.WriteFile(filepath.Join(src, "b", "good.jsonl"), []byte(`{"n":2}`+"\n"), 0o644)
	// コピーの場所が中身のあるフォルダだと、置きかえ（rename）に失敗する
	os.MkdirAll(filepath.Join(dst, "a", "bad.jsonl.zst", "x"), 0o700)
	// 更新時刻が同じだと「変わっていない」として飛ばすので、元のファイルの時刻をずらす
	mt := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(src, "a", "bad.jsonl"), mt, mt)
	n, err := Sync(src, dst)
	if n != 1 || err == nil {
		t.Fatalf("Sync = %d %v, want 1 とエラー", n, err)
	}
	if got := unzst(t, filepath.Join(dst, "b", "good.jsonl.zst")); string(got) != `{"n":2}`+"\n" {
		t.Errorf("ほかのファイルのコピー = %q", got)
	}
	if left := tmpLeft(t, dst); len(left) > 0 {
		t.Errorf("一時ファイルが残った: %v", left)
	}
	// 失敗したファイルは、次の Sync でもう一度試す（成功したものとして覚えない）
	os.RemoveAll(filepath.Join(dst, "a", "bad.jsonl.zst"))
	if n, err := Sync(src, dst); n != 1 || err != nil {
		t.Errorf("直したあと Sync = %d %v, want 1", n, err)
	}
}

// 圧縮し直しに失敗しても、前のコピーはそのまま読める。
func TestSyncFailureKeepsPreviousCopy(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("書けないフォルダを作れない（Windows か root）")
	}
	src, dst := t.TempDir(), t.TempDir()
	p := filepath.Join(src, "proj", "s1.jsonl")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(`{"n":1}`+"\n"), 0o644)
	if n, err := Sync(src, dst); n != 1 || err != nil {
		t.Fatalf("1 回目: %d %v", n, err)
	}
	os.WriteFile(p, []byte(`{"n":1}`+"\n"+`{"n":2}`+"\n"), 0o644)
	later := time.Now().Add(time.Minute)
	os.Chtimes(p, later, later)
	out := filepath.Join(dst, "proj")
	os.Chmod(out, 0o500) // 一時ファイルを作れない
	defer os.Chmod(out, 0o700)
	if n, err := Sync(src, dst); n != 0 || err == nil {
		t.Errorf("書けないとき Sync = %d %v, want 0 とエラー", n, err)
	}
	if got := unzst(t, filepath.Join(out, "s1.jsonl.zst")); string(got) != `{"n":1}`+"\n" {
		t.Errorf("前のコピー = %q", got)
	}
}

// 読めない元のファイルは飛ばして、エラーを返す。
func TestSyncUnreadableSource(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("読めないファイルを作れない（Windows か root）")
	}
	src, dst := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(src, "locked.jsonl"), []byte(`{"n":1}`+"\n"), 0o000)
	os.WriteFile(filepath.Join(src, "ok.jsonl"), []byte(`{"n":2}`+"\n"), 0o644)
	if n, err := Sync(src, dst); n != 1 || err == nil {
		t.Errorf("Sync = %d %v, want 1 とエラー", n, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "locked.jsonl.zst")); !os.IsNotExist(err) {
		t.Errorf("読めないファイルのコピーがある: %v", err)
	}
}

// Enable が作る印のファイルはほかのユーザーに読めず、作れない場所ではエラーを返す。
func TestEnableModeAndError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "archive")
	if err := Enable(dir); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(dir); st.Mode().Perm() != 0o700 {
			t.Errorf("保存場所の権限 = %o, want 700", st.Mode().Perm())
		}
		for _, f := range []string{marker, stamp} {
			if st, _ := os.Stat(filepath.Join(dir, f)); st.Mode().Perm() != 0o600 {
				t.Errorf("%s の権限 = %o, want 600", f, st.Mode().Perm())
			}
		}
	}
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0o600)
	if err := Enable(filepath.Join(file, "archive")); err == nil {
		t.Error("ファイルの下に保存場所を作れた")
	}
	if Enabled(filepath.Join(file, "archive")) {
		t.Error("作れなかったのにオン")
	}
}
