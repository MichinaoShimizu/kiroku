//go:build !windows

package source

import (
	"path/filepath"
	"syscall"
	"testing"
)

// 名前付きパイプの DB は開かない（開くと SQLite が書き手を待って止まる）。
func TestQStoreFifo(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "fifo.sqlite3")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	checkNotRegular(t, fifo)
}
