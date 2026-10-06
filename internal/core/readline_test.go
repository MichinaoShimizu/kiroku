package core

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// MaxLine より長い行は、メモリに読まずに飛ばし、前後の行は読む。飛ばしたことはエラーで知らせる（読めないファイルとして出る）。
func TestReadJSONLLongLine(t *testing.T) {
	old := MaxLine
	MaxLine = 1 << 10
	defer func() { MaxLine = old }()
	long := `{"n":"` + strings.Repeat("x", 3<<20) + `"}`             // バッファ（1 MiB）より長い行も
	text := `{"n":1}` + "\n" + long + "\n" + `{"n":2}` + "\n" + long // 最後の行は改行なし
	var got []string
	err := ReadJSONLFrom(strings.NewReader(text), func(o Obj) { got = append(got, fmt.Sprint(o["n"])) })
	if strings.Join(got, ",") != "1,2" {
		t.Errorf("読めた行 = %v", got)
	}
	if err == nil || !strings.Contains(err.Error(), "skipped 2 line(s) longer than 1024 bytes") {
		t.Errorf("err = %v", err)
	}

	// 上限ちょうどの行は読む
	ok := `{"n":"` + strings.Repeat("y", MaxLine-9) + `"}` + "\n"
	if len(ok) != MaxLine {
		t.Fatalf("len = %d", len(ok))
	}
	n := 0
	if err := ReadJSONLFrom(strings.NewReader(ok), func(Obj) { n++ }); err != nil || n != 1 {
		t.Errorf("上限ちょうど: n = %d, err = %v", n, err)
	}

	// .zst のファイルでも同じ
	p := filepath.Join(t.TempDir(), "a.jsonl.zst")
	var buf bytes.Buffer
	w, _ := zstd.NewWriter(&buf)
	io.WriteString(w, text)
	w.Close()
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	got = nil
	err = ReadJSONL(p, func(o Obj) { got = append(got, fmt.Sprint(o["n"])) })
	if strings.Join(got, ",") != "1,2" || err == nil {
		t.Errorf(".zst: %v, %v", got, err)
	}
}

// ReadLine は長い行の中身を持たない（捨てた分のメモリを使わない）。
func TestReadLine(t *testing.T) {
	r := bufio.NewReaderSize(strings.NewReader(strings.Repeat("a", 100)+"\nbc\nd"), 16)
	for _, want := range []struct {
		line string
		long bool
		err  error
	}{{"", true, nil}, {"bc\n", false, nil}, {"d", false, io.EOF}} {
		line, long, err := ReadLine(r, 10)
		if string(line) != want.line || long != want.long || err != want.err {
			t.Errorf("ReadLine = %q %v %v, want %+v", line, long, err, want)
		}
	}
}
