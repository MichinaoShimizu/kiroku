//go:build windows

package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

// isTerminal は、f がコンソールか（コンソールのモードを読めるか）。NUL やパイプは端末ではない。
func isTerminal(f *os.File) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(f.Fd()), &mode) == nil
}

// flushInput は、聞く前にコンソールに打ちこまれていた入力を捨てる。
func flushInput(f *os.File) { _ = windows.FlushConsoleInputBuffer(windows.Handle(f.Fd())) }
