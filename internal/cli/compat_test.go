package cli

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/archive"
)

// testdata/compat/<版> は、その版の kiroku が書いたままのファイル。docs/compatibility.md の約束どおり、
// あとの版でも読めることを確かめる。形を変えたら、古い版のフォルダは消さずに新しい版のフォルダを足す。
var compatVersions = []string{"v0.19"}

// copyCompat は testdata/compat/<v>/<name> を一時フォルダに写す（git は 0600 を残さないので、ここで本人だけにする）。
func copyCompat(t *testing.T, v, name string) string {
	t.Helper()
	src := filepath.Join("testdata", "compat", v, name)
	dst := filepath.Join(t.TempDir(), name)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o700)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// 前の版の kiroku archive のコピーを、元の履歴が消えたあとも読める。kiroku archive off も保存場所だとわかる。
func TestCompatArchive(t *testing.T) {
	for _, v := range compatVersions {
		t.Run(v, func(t *testing.T) {
			dir := copyCompat(t, v, "archive")
			if !archive.Enabled(dir) || !archive.Marked(dir) {
				t.Fatalf("保存場所だとわからない: enabled %v, marked %v", archive.Enabled(dir), archive.Marked(dir))
			}
			if files, _ := archive.Usage(dir); files != 1 {
				t.Errorf("コピーの数 %d, want 1", files)
			}
			out := filepath.Join(t.TempDir(), "k.json")
			args := []string{"json", "--root", t.TempDir(), "--sources", "claude", "--archive-dir", dir, "-o", out}
			captureOutput(t, func() {
				if err := dispatch(args); err != nil {
					t.Fatal(err)
				}
			})
			b, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Sessions []struct {
					ID       string `json:"id"`
					Project  string `json:"project"`
					NPrompts int    `json:"nPrompts"`
				} `json:"sessions"`
			}
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Sessions) != 1 {
				t.Fatalf("セッション %d 件, want 1", len(got.Sessions))
			}
			s := got.Sessions[0]
			if s.ID != "0c0a7c1e-0000-4000-8000-00000000c0de" || s.Project != "app" || s.NPrompts != 2 {
				t.Errorf("読んだセッション %+v", s)
			}
		})
	}
}

// 前の版の kiroku serve の鍵をそのまま使う（ブックマークや kiroku open がそのまま使える）。
func TestCompatServeKey(t *testing.T) {
	for _, v := range compatVersions {
		t.Run(v, func(t *testing.T) {
			cfg := t.TempDir()
			t.Setenv("KIROKU_CONFIG_DIR", cfg)
			want, err := os.ReadFile(filepath.Join("testdata", "compat", v, keyFile))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(cfg, keyFile), want, 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := serveKey(false)
			if err != nil || got != strings.TrimSpace(string(want)) {
				t.Errorf("鍵 %q (%v), want %q", got, err, want)
			}
		})
	}
}
