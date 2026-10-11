//go:build !unix

package core

import "os"

// openNoBlock は、unix でない OS（Windows など）ではそのまま開く。開く前に OpenRegular がふつうのファイルか調べ、
// 開いたあとにもう一度調べる。
func openNoBlock(path string) (*os.File, error) {
	return os.Open(path)
}
