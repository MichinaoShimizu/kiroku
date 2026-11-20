//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package cli

import (
	"os"

	"golang.org/x/sys/unix"
)

// isTerminal は、f が本当の端末か（端末の設定を読めるか）。/dev/null や /dev/zero のような
// ほかの文字デバイスは端末ではない（cron や 2>/dev/null で動かしたときに、聞いたり見に行ったりしない）。
func isTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), ioctlGetTermios)
	return err == nil
}

// flushInput は、聞く前に端末に打ちこまれていた文字を捨てる（コマンドが動いているあいだに打った y で、
// 見ていない問いに答えないように）。
func flushInput(f *os.File) {
	fd := int(f.Fd())
	if unix.SetNonblock(fd, true) != nil {
		return
	}
	defer unix.SetNonblock(fd, false)
	buf := make([]byte, 256)
	for i := 0; i < 64; i++ { // 打ちこまれた分だけ。読み切ったら（EAGAIN）やめる
		if n, err := unix.Read(fd, buf); err != nil || n <= 0 {
			return
		}
	}
}
