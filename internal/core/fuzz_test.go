package core

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
)

// 履歴のファイルは信用しない。どんな中身でも ReadLine は行を正しく切り、max より長い行は持たずに読み飛ばす。
func FuzzReadLine(f *testing.F) {
	f.Add([]byte("{\"a\":1}\n{\"b\":2}\n"), 8)
	f.Add([]byte("short\n"+strings.Repeat("x", 100)+"\nlast"), 16)
	f.Add([]byte(""), 1)
	f.Fuzz(func(t *testing.T, data []byte, max int) {
		if max < 1 || max > 1<<12 {
			return
		}
		var want [][]byte
		for rest := data; len(rest) > 0; {
			i := bytes.IndexByte(rest, '\n') + 1
			if i == 0 {
				i = len(rest)
			}
			want = append(want, rest[:i])
			rest = rest[i:]
		}
		r := bufio.NewReaderSize(bytes.NewReader(data), 16) // 小さいバッファで、ErrBufferFull の続きも通す
		for n := 0; ; n++ {
			line, long, err := ReadLine(r, max)
			if err != nil && err != io.EOF {
				t.Fatal(err)
			}
			if err == io.EOF && len(line) == 0 && !long {
				if n != len(want) {
					t.Fatalf("read %d lines, want %d", n, len(want))
				}
				return
			}
			if n >= len(want) {
				t.Fatalf("more lines than the input has")
			}
			if long != (len(want[n]) > max) {
				t.Fatalf("line %d: long = %v for %d bytes (max %d)", n, long, len(want[n]), max)
			}
			if !long && !bytes.Equal(line, want[n]) {
				t.Fatalf("line %d = %q, want %q", n, line, want[n])
			}
			if long && line != nil {
				t.Fatalf("line %d: kept %d bytes of a long line", n, len(line))
			}
			if err == io.EOF {
				if n+1 != len(want) {
					t.Fatalf("read %d lines, want %d", n+1, len(want))
				}
				return
			}
		}
	})
}

// 時刻の値は履歴のファイルから来る。どんな値でも、受けいれるのはありうる時刻だけ。
func FuzzParseTS(f *testing.F) {
	for _, s := range []string{"2026-09-30T01:00:00Z", "2026-09-30 01:00:00.123", "1759194000", "1759194000123", "-1", "0001-01-01T00:00:00Z", "2026-09-30"} {
		f.Add(s, 0.0)
	}
	f.Add("", 1759194000.5)
	f.Add("", 1e300)
	f.Fuzz(func(t *testing.T, s string, x float64) {
		for _, v := range []any{s, []byte(s), x} {
			if ts, ok := ParseTS(v); ok && !plausibleTS(ts) {
				t.Fatalf("ParseTS(%#v) = %v: not a plausible time", v, ts)
			}
		}
	})
}
