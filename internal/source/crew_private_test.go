package source

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// crewFilePrivate は、読めない記録（壊れた .zst、途中で切れた .zst）を中身を残さない会話とみなす（閉じる側に倒す）。
// ない・ふつうのファイルでないものだけが false。権限で開けないファイルは root では作れないので、壊れた .zst で確かめる
func TestCrewFilePrivateFailsClosed(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, b []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	zst := func(s string) []byte {
		var b bytes.Buffer
		w, err := zstd.NewWriter(&b)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(s))
		w.Close()
		return b.Bytes()
	}
	plain := `{"_type": "metadata", "title": "dummy", "memory_mode": "persistent"}` + "\n" + `{"_type": "message", "text": "hello"}` + "\n"
	private := `{"_type": "metadata", "title": "dummy", "memory_mode": "incognito"}` + "\n"
	big := plain
	for len(big) < 1<<18 {
		big += `{"_type": "message", "text": "` + string(bytes.Repeat([]byte("y"), 200)) + `"}` + "\n"
	}
	whole := zst(big)
	if err := os.Mkdir(filepath.Join(dir, "adir.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		path string
		want bool
	}{
		{"plain", write("a.jsonl", []byte(plain)), false},
		{"plain zst", write("a.jsonl.zst", zst(plain)), false},
		{"private zst", write("p.jsonl.zst", zst(private)), true},
		{"missing", filepath.Join(dir, "none.jsonl.zst"), false},
		{"directory", filepath.Join(dir, "adir.jsonl"), false},
		{"garbage zst", write("g.jsonl.zst", []byte("this is not zstd at all\n")), true},
		{"truncated zst", write("t.jsonl.zst", whole[:len(whole)/2]), true},
	} {
		if got := crewFilePrivate(c.path); got != c.want {
			t.Errorf("%s: crewFilePrivate = %v, want %v", c.name, got, c.want)
		}
	}
}
