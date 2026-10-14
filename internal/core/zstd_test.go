package core

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// writeZst は text を zstd で圧縮して path に書く。
func writeZst(t *testing.T, path, text string) {
	t.Helper()
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(w, text)
	w.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

// 小さくても大きくふくらむ .zst（展開爆弾）は、MaxZstd でほどくのをやめ、黙って切らずにエラーを返す（読めないファイルとして出る）。
// 上限ちょうどまでのファイルは最後まで読む。
func TestReadJSONLZstdBomb(t *testing.T) {
	old := MaxZstd
	MaxZstd = 1 << 20
	defer func() { MaxZstd = old }()
	line := `{"n":1,"text":"` + strings.Repeat("a", 1000) + `"}` + "\n"
	dir := t.TempDir()

	// 上限の 8 倍にふくらむもの（圧縮すると数 KB）
	bomb := filepath.Join(dir, "bomb.jsonl.zst")
	writeZst(t, bomb, strings.Repeat(line, 8*int(MaxZstd)/len(line)))
	if fi, _ := os.Stat(bomb); fi.Size() > 64<<10 {
		t.Fatalf("爆弾が大きすぎる: %d bytes", fi.Size())
	}
	n := 0
	err := ReadJSONL(bomb, func(Obj) { n++ })
	if !errors.Is(err, ErrZstdTooLarge) || !strings.Contains(err.Error(), "decompressed size exceeds the limit of 1 MiB") {
		t.Errorf("err = %v, want ErrZstdTooLarge", err)
	}
	if max := int(MaxZstd) / len(line); n == 0 || n > max {
		t.Errorf("読んだ行 = %d, want 上限までの 1〜%d 行", n, max)
	}

	// 上限ちょうどの大きさは最後まで読む
	exact := filepath.Join(dir, "exact.jsonl.zst")
	text := strings.Repeat(line, int(MaxZstd)/len(line))
	text += `{"n":2,"text":"` + strings.Repeat("b", int(MaxZstd)-len(text)-len(`{"n":2,"text":""}`)-1) + `"}` + "\n"
	if int64(len(text)) != MaxZstd {
		t.Fatalf("len = %d", len(text))
	}
	writeZst(t, exact, text)
	n = 0
	if err := ReadJSONL(exact, func(Obj) { n++ }); err != nil || n != int(MaxZstd)/len(line)+1 {
		t.Errorf("上限ちょうど: n = %d, err = %v", n, err)
	}

	// 1 バイト超えると読めない
	writeZst(t, exact, text+"\n")
	if err := ReadJSONL(exact, func(Obj) {}); !errors.Is(err, ErrZstdTooLarge) {
		t.Errorf("上限を 1 バイト超える: err = %v", err)
	}
}

// ふつうの .zst（上限より小さい）は、上限に触れず最後まで同じ中身を読む。
func TestZstdReaderNormal(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.jsonl.zst")
	text := strings.Repeat(`{"n":1}`+"\n", 100000)
	writeZst(t, p, text)
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := NewZstdReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	b, err := io.ReadAll(d)
	if err != nil || string(b) != text {
		t.Errorf("len = %d, err = %v", len(b), err)
	}
}
