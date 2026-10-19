package core

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
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

// 細工した .zst（klauspost/compress の GHSA-fpmh-frq8-7rj8、上流の TestDecoderHuffWeightsTableLog の入力）は、
// 読めないファイルとして誤りを返す。1.20.2 より前は、ほどくゴルーチンが panic して kiroku ごと落ちていた。
func TestZstdReaderCraftedHuffWeights(t *testing.T) {
	frame := slices.Concat(
		[]byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, 0x38, 0x74, 0x06, 0x00, 0x42, 0xb8, 0x32, 0x0d, 0x1a, 0x00, 0x00, 0x00, 0x90, 0xaf, 0xfa, 0xff, 0x03, 0x00, 0x80, 0xff, 0x7f},
		bytes.Repeat([]byte{0xa5, 0x94, 0x52, 0x4a, 0x29}, 37),
		[]byte{0xa5, 0x94, 0x12, 0x00, 0xad, 0x56, 0x03, 0xee, 0xff, 0xdf, 0xb3, 0x1a, 0x1c, 0x1a, 0x00, 0xe0, 0xff, 0x1f, 0x0f, 0xb1, 0x9c, 0x5a, 0xcf, 0x4d, 0xe4, 0x1d, 0x7b, 0xab, 0x99, 0x5e, 0x01, 0x51, 0xa4, 0x73, 0xd8, 0xb1, 0xf3, 0x39, 0x4b, 0x67, 0xf6, 0xab, 0x1a, 0xab, 0x1a, 0xab, 0x1a},
		bytes.Repeat([]byte{0x8c, 0x31, 0xc6, 0x18, 0x63}, 1365),
		[]byte{0x8c, 0x31},
		bytes.Repeat([]byte{0x63, 0x8c, 0x31, 0xc6, 0x18}, 1365),
		[]byte{0x63, 0x2c},
		bytes.Repeat([]byte{0x31, 0xc6, 0x18, 0x63, 0x8c}, 1365),
		[]byte{0x31, 0x46},
		bytes.Repeat([]byte{0x31, 0xc6, 0x18, 0x63, 0x8c}, 1365),
		[]byte{0x31, 0x06, 0x00},
	)
	z, err := NewZstdReader(bytes.NewReader(frame))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if _, err := io.ReadAll(z); err == nil {
		t.Error("細工した .zst を誤りなしで読んだ")
	}
	// 履歴として読んでも、落ちずに読めないファイルになる
	path := filepath.Join(t.TempDir(), "rollout.jsonl.zst")
	if err := os.WriteFile(path, frame, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReadJSONL(path, func(Obj) {}); err == nil {
		t.Error("細工した .jsonl.zst を誤りなしで読んだ")
	}
}
