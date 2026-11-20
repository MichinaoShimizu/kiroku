//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly && !windows

package cli

import "os"

// ほかの OS では、端末かどうかを確かめられないので、端末ではないとみなす（新しい版のお知らせは出さない）。
func isTerminal(*os.File) bool { return false }
func flushInput(*os.File)      {}
