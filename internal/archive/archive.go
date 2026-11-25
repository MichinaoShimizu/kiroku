// Package archive は、エージェントが自動で消してしまう履歴のコピーを、kiroku の保存場所に圧縮して残す（kiroku archive）。
//
// 保存場所の下に、元の場所からの相対パスのまま <名前>.zst で置く。元のファイルの更新時刻をコピーにも付けて、
// 次からは更新時刻が変わったファイルだけを圧縮し直す。保存するかどうかは、保存場所の enabled というファイルで決める
// （kiroku serve --archive on / off）。止めても、すでに保存したコピーは読む。
package archive

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/klauspost/compress/zstd"
)

// marker があれば保存する。
const marker = "enabled"

// stamp は、kiroku archive の保存場所だという印。オフにしても消さない（kiroku serve --archive off でコピーを消す前に確かめる）。
const stamp = ".kiroku-archive"

// DefaultDir は保存場所。KIROKU_ARCHIVE_DIR があればそこ。
// Linux は $XDG_DATA_HOME か ~/.local/share、macOS は ~/Library/Application Support、Windows は %LocalAppData% の下の kiroku/archive。
// ホームがわからず（HOME が空など）、環境変数でも決まらなければ ""（今のフォルダの下に履歴のコピーを書かない。使う側が断る）。
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return defaultDir(runtime.GOOS, home, os.Getenv)
}

// defaultDir は OS ごとの DefaultDir（テストで OS を変えられるよう、ホームと環境変数を受け取る）。
func defaultDir(goos, home string, getenv func(string) string) string {
	if d := getenv("KIROKU_ARCHIVE_DIR"); d != "" {
		return d
	}
	var base string
	switch goos {
	case "darwin":
		if home != "" {
			base = filepath.Join(home, "Library", "Application Support")
		}
	case "windows":
		base = getenv("LocalAppData")
		if base == "" && home != "" {
			base = filepath.Join(home, "AppData", "Local")
		}
	default:
		base = getenv("XDG_DATA_HOME")
		if !strings.HasPrefix(base, "/") { // XDG Base Directory: 相対パスの XDG_DATA_HOME は無視する（今のフォルダに書かない・読まない）
			base = ""
		}
		if base == "" && home != "" {
			base = filepath.Join(home, ".local", "share")
		}
	}
	if base == "" {
		return ""
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
	if err := core.PrivateDir(dir); err != nil { // 前からあって、ほかの人も読めるフォルダも自分だけにする
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, stamp), []byte("this folder holds kiroku archive copies of agent history\n"), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, marker), []byte("kiroku keeps compressed copies of agent history here (kiroku serve --archive off to stop)\n"), 0o600)
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
			if isTemp(d) { // 止められて残った書きかけ（数えない）
				_ = os.Remove(p)
				return nil
			}
			if !isCopy(d) {
				return nil // リンクやほかのファイル（kiroku が書かない名前）は残す
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

// isCopy は、kiroku が書くコピー（Sync が書く、ふつうのファイルの .jsonl.zst）か。Clear と Kept が同じ決まりで数える。
func isCopy(d fs.DirEntry) bool {
	return d.Type().IsRegular() && strings.HasSuffix(d.Name(), ".jsonl.zst")
}

// tmpPrefix は compress が書きかけのコピーに付ける名前。置きかえる前に止められる（kill -9 など）と残る
const tmpPrefix = ".kiroku-tmp-"

// staleTempAge より古い書きかけは、ほかの kiroku が書いている最中ではないので消してよい
const staleTempAge = time.Hour

func isTemp(d fs.DirEntry) bool {
	return d.Type().IsRegular() && strings.HasPrefix(d.Name(), tmpPrefix)
}

// removeStaleTemps は、フォルダ dir に残った古い書きかけ（中身は履歴なので残さない）を消す
func removeStaleTemps(dir string, now time.Time) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range es {
		if !isTemp(e) {
			continue
		}
		if fi, err := e.Info(); err == nil && now.Sub(fi.ModTime()) > staleTempAge {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// Kept は、Clear が消すもの（dir の下の subs の中の、kiroku が書いたコピー）の数と大きさ。消すかを聞くときに使う。
func Kept(dir string, subs []string) (files int, bytes int64) {
	for _, sub := range subs {
		_ = filepath.WalkDir(filepath.Join(dir, sub), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !isCopy(d) {
				return nil
			}
			if info, err := d.Info(); err == nil {
				files++
				bytes += info.Size()
			}
			return nil
		})
	}
	return files, bytes
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

// ErrTooLarge は、コピーを残すには大きすぎるファイル（copyLimit より大きい）なので残さなかったこと。
var ErrTooLarge = errors.New("too large to keep a copy")

// maxCopy は、コピーを残すファイル 1 つの大きさの上限（バイト）。0 なら core.MaxZstd（kiroku がコピーを読むときにほどく上限。
// それより大きいファイルのコピーは、残しても読めない）。履歴の名前でリンクされたほかの大きなファイルを、読み直しのたびに
// 何 GB も圧縮して保存場所を埋めないため。テストで小さくする。
var maxCopy int64

func copyLimit() int64 {
	if maxCopy > 0 {
		return maxCopy
	}
	return core.MaxZstd
}

// tooLarge は、大きすぎて残さなかったファイルのエラー（読めなかったファイルと同じく、ファイル名をつけて知らせる）。
func tooLarge(path string) error {
	return fmt.Errorf("%s: %w (larger than %s)", path, ErrTooLarge, core.SizeLimit(copyLimit()))
}

// Sync は src の下の .jsonl を dst の下に圧縮して残す。前回から変わっていないファイルは圧縮し直さない。
// 元のファイルが消えても、コピーは消さない。保存した（圧縮し直した）数を返す。
func Sync(src, dst string) (int, error) {
	if src == "" {
		return 0, nil
	}
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return 0, nil
	}
	src = resolve(src) // WalkDir は起点のシンボリックリンクをたどらない（~/.claude/projects がリンクでも中を残す）
	// 前の版などで作られて、ほかの人も読めるエージェントごとのフォルダ（claude/ など）は、自分だけが読めるようにする。
	// 中のフォルダの名前はプロジェクトのパスなので、一覧を見せない
	if st, err := os.Lstat(dst); err == nil && st.IsDir() {
		if err := core.PrivateDir(dst); err != nil {
			return 0, err
		}
	}
	n := 0
	var first error
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 読めないフォルダは飛ばす
		}
		if d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		// 大きさと更新時刻はリンクの先のもので見る（d.Info は Lstat なので、.jsonl がシンボリックリンクだとリンク自身の
		// 大きさで切ってしまい、先が変わっても残し直さない）。先がない・読めないものは飛ばす
		info, err := os.Stat(p)
		if err != nil || info.Size() == 0 {
			return nil
		}
		if info.Size() > copyLimit() { // 開かずに飛ばす（compress も、開いたファイルの大きさで確かめ直す）
			if first == nil {
				first = tooLarge(p)
			}
			return nil
		}
		out, ok := Path(src, dst, p)
		if !ok {
			return nil
		}
		if prev, err := os.Stat(out); err == nil && prev.ModTime().Equal(info.ModTime()) {
			return nil
		}
		if err := compress(p, out); err != nil {
			if errors.Is(err, core.ErrNotRegular) {
				return nil // ふつうのファイルでないもの（名前つきパイプなど）は残さない（大きさ 0 のものと同じ）
			}
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
// 大きさと更新時刻は、開いたファイル（シンボリックリンクなら、その先）から読む。
func compress(src, dst string) error {
	in, err := core.OpenRegular(src) // シンボリックリンクの先が名前つきパイプでも止まらない
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if info.Size() > copyLimit() { // Sync が大きさを見たあとで、リンクの先が変わっていても
		return tooLarge(src)
	}
	if err := core.PrivateDir(filepath.Dir(dst)); err != nil {
		return err
	}
	removeStaleTemps(filepath.Dir(dst), time.Now())
	tmp, err := os.CreateTemp(filepath.Dir(dst), tmpPrefix+"*")
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
