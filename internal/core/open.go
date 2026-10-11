package core

import (
	"errors"
	"io/fs"
	"os"
)

// ErrNotRegular は、ふつうのファイルでない（名前つきパイプ・デバイス・フォルダなど）ので開かなかったこと。
var ErrNotRegular = errors.New("not a regular file")

// OpenRegular はふつうのファイル（シンボリックリンクの先も）だけを読むために開く。
// 名前つきパイプは開くだけで止まってしまう（書き手が来るまで open が返らない）ので、先に調べて開かない。
// 調べてから開くまでの間に置きかえられても止まらないよう、unix では O_NONBLOCK で開き、開いたものをもう一度調べる。
// ふつうのファイルでなければ、ファイル名のついた ErrNotRegular を返す（読めないファイルとして知らせる）。
func OpenRegular(path string) (*os.File, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, &fs.PathError{Op: "open", Path: path, Err: ErrNotRegular}
	}
	f, err := openNoBlock(path)
	if err != nil {
		return nil, err
	}
	if st, err = f.Stat(); err != nil || !st.Mode().IsRegular() {
		f.Close()
		if err == nil {
			err = &fs.PathError{Op: "open", Path: path, Err: ErrNotRegular}
		}
		return nil, err
	}
	return f, nil
}
