//go:build unix

package core

import (
	"os"
	"syscall"
)

// openNoBlock は、名前つきパイプでも止まらないように O_NONBLOCK で開く（ふつうのファイルの読み方は変わらない）。
func openNoBlock(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
