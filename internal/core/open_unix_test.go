//go:build unix

package core

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// 名前つきパイプは開くだけで止まるので、ReadJSONL・ReadJSONFile は開かずにファイル名のついたエラーを返す。
func TestReadFIFO(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jsonl", "a.jsonl.zst", "session.json"} {
		p := filepath.Join(dir, name)
		if err := syscall.Mkfifo(p, 0o600); err != nil {
			t.Skip(err)
		}
		done := make(chan error, 2)
		go func() {
			done <- ReadJSONL(p, func(Obj) { t.Error("パイプから行を読んだ") })
			_, err := ReadJSONFile(p)
			done <- err
		}()
		for i := 0; i < 2; i++ {
			select {
			case err := <-done:
				if !errors.Is(err, ErrNotRegular) {
					t.Errorf("%s: err = %v, want ErrNotRegular", name, err)
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("%s: 名前つきパイプを開いて止まった", name)
			}
		}
	}
}
