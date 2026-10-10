// Package archive は、エージェントが自動で消してしまう履歴のコピーを、kiroku の保存場所に圧縮して残す（kiroku archive）。
//
// 保存場所の下に、元の場所からの相対パスのまま <名前>.zst で置く。元のファイルの更新時刻をコピーにも付けて、
// 次からは更新時刻が変わったファイルだけを圧縮し直す。保存するかどうかは、保存場所の enabled というファイルで決める
// （kiroku archive on / off）。止めても、すでに保存したコピーは読む。
package archive

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// marker があれば保存する。
const marker = "enabled"

// stamp は、kiroku archive の保存場所だという印。オフにしても消さない（kiroku archive off でコピーを消す前に確かめる）。
const stamp = ".kiroku-archive"

// DefaultDir は保存場所。KIROKU_ARCHIVE_DIR があればそこ。
// Linux は $XDG_DATA_HOME か ~/.local/share、macOS は ~/Library/Application Support、Windows は %LocalAppData% の下の kiroku/archive。
func DefaultDir() string {
	if d := os.Getenv("KIROKU_ARCHIVE_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	var base string
	switch runtime.GOOS {
	case "darwin":
		base = filepath.Join(home, "Library", "Application Support")
	case "windows":
		base = os.Getenv("LocalAppData")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			base = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(base, "kiroku", "archive")
}

// Enabled は保存がオンか。
func Enabled(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, marker))
	return err == nil
}

// Enable は保存をオンにする（保存場所がなければ作る）。
func Enable(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, stamp), []byte("this folder holds kiroku archive copies of agent history\n"), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, marker), []byte("kiroku keeps compressed copies of agent history here (kiroku archive off to stop)\n"), 0o600)
}

// Marked は、dir が kiroku archive の保存場所らしいか（オンになっているか、前にオンにした印があるか）。
// まちがった --archive-dir で、ほかのフォルダのファイルを消さないために確かめる。
func Marked(dir string) bool {
	if Enabled(dir) {
		return true
	}
	st, err := os.Lstat(filepath.Join(dir, stamp))
	return err == nil && st.Mode().IsRegular()
}

// Clear は、dir の下の subs のフォルダにある、保存したコピー（.zst）だけを消す。ほかのファイルは消さず、
// 空になったフォルダだけを消す。シンボリックリンクはたどらない。消した数を返す。
func Clear(dir string, subs []string) (int, error) {
	n := 0
	var first error
	for _, sub := range subs {
		root := filepath.Join(dir, sub)
		var dirs []string
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if !os.IsNotExist(err) && first == nil {
					first = err
				}
				return nil
			}
			if d.IsDir() {
				dirs = append(dirs, p)
				return nil
			}
			if !d.Type().IsRegular() || !strings.HasSuffix(p, ".zst") {
				return nil // リンクやほかのファイルは残す
			}
			if err := os.Remove(p); err != nil {
				if first == nil {
					first = err
				}
				return nil
			}
			n++
			return nil
		})
		if first == nil {
			first = err
		}
		for i := len(dirs) - 1; i >= 0; i-- { // 深いところから。空でなければ消えない
			_ = os.Remove(dirs[i])
		}
	}
	return n, first
}

// Disable は保存をやめる。すでに保存したコピーは残す。
func Disable(dir string) error {
	err := os.Remove(filepath.Join(dir, marker))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Usage は保存したコピーの数と、圧縮後の大きさ（バイト）。
func Usage(dir string) (files int, bytes int64) {
	_ = filepath.WalkDir(resolve(dir), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".zst") {
			return nil
		}
		if info, err := d.Info(); err == nil {
			files++
			bytes += info.Size()
		}
		return nil
	})
	return files, bytes
}

// Path は、src の下のファイル file のコピーの場所（dst の下の同じ相対パス + .zst）。
func Path(src, dst, file string) (string, bool) {
	rel, err := filepath.Rel(src, file)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.Join(dst, rel) + ".zst", true
}

// Sync は src の下の .jsonl を dst の下に圧縮して残す。前回から変わっていないファイルは圧縮し直さない。
// 元のファイルが消えても、コピーは消さない。保存した（圧縮し直した）数を返す。
func Sync(src, dst string) (int, error) { return SyncSkip(src, dst, nil) }

// SyncSkip は Sync と同じ。skip（nil でなければ）が true を返すファイルは残さず、前に残したそのコピーがあれば消す。
// skip には、ファイルの場所と、src からの相対パスを渡す
// （Kiro Crew の incognito・temporary の会話の記録。あとからそうなった会話の前のコピーも残さない）。
func SyncSkip(src, dst string, skip func(path, rel string) bool) (int, error) {
	if src == "" {
		return 0, nil
	}
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return 0, nil
	}
	src = resolve(src) // WalkDir は起点のシンボリックリンクをたどらない（~/.claude/projects がリンクでも中を残す）
	n := 0
	var first error
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 読めないフォルダは飛ばす
		}
		if d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		out, ok := Path(src, dst, p)
		if !ok {
			return nil
		}
		if rel, err := filepath.Rel(src, p); skip != nil && err == nil && skip(p, rel) {
			if err := os.Remove(out); err != nil && !errors.Is(err, fs.ErrNotExist) && first == nil {
				first = err
			}
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() == 0 {
			return nil
		}
		if prev, err := os.Stat(out); err == nil && prev.ModTime().Equal(info.ModTime()) {
			return nil
		}
		if err := compress(p, out, info); err != nil {
			if first == nil {
				first = err
			}
			return nil
		}
		n++
		return nil
	})
	if first == nil {
		first = err
	}
	return n, first
}

// resolve は、シンボリックリンクをたどった本当の場所（たどれなければそのまま）。
func resolve(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// compress は src を zstd で圧縮して dst に書く。途中で失敗しても前のコピーは残る（一時ファイルに書いてから置きかえる）。
func compress(src, dst string, info fs.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".kiroku-tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // 置きかえたあとは何もしない
	enc, err := zstd.NewWriter(tmp)
	if err != nil {
		tmp.Close()
		return err
	}
	// 書いている途中のファイルは、読んだところまでを残す（続きは次に保存したときに入る）
	if _, err := io.Copy(enc, io.LimitReader(in, info.Size())); err != nil {
		enc.Close()
		tmp.Close()
		return err
	}
	if err := enc.Close(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chtimes(tmp.Name(), info.ModTime(), info.ModTime()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}
