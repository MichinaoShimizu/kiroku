package source

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// Watcher は、Where() だけでは見張る場所が足りない Source が実装する。
type Watcher interface{ Watch() []string }

func (k *KiroCLI) Watch() []string { return []string{k.Where(), k.CrewHome} }

func (k *KiroIDELegacy) Watch() []string { return k.Storages }

// SQLite は書き込みが -wal に先に入るので、それも見る。
func (q *QStore) Watch() []string { return []string{q.DB, q.DB + "-wal"} }

// WatchPaths は Source が読む場所。
func WatchPaths(s Source) []string {
	if w, ok := s.(Watcher); ok {
		return w.Watch()
	}
	return []string{s.Where()}
}

// Fingerprint は、paths の下のファイルの名前・大きさ・更新時刻をまとめた指紋。
// 中身は読まないので、履歴が多くても軽い。ないパスは無視する。
func Fingerprint(paths []string) string {
	h := sha256.New()
	for _, root := range paths {
		if root == "" {
			continue
		}
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // 消えた・読めないものは飛ばす
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			fmt.Fprintf(h, "%s\x00%d\x00%d\n", p, info.Size(), info.ModTime().UnixNano())
			return nil
		})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Unit は、ほかと切り離して読める履歴のひとまとまり（例: Claude Code の 1 つの会話のファイルと、そのサブエージェントのファイル）。
type Unit struct {
	Key   string   // ひとまとまりを見分ける名前（ふつうはおもなファイルのパス）
	Files []string // 読むファイル。どれかが変わったら読み直す
	Tag   string   // Files のほかに読み直すきっかけになるもの（例: Codex のスレッド名）。変わったら読み直す
}

// Splitter は、Unit ごとに読める Source が実装する。kiroku serve は、変わっていない Unit を読み直さない。
// Load は、すべての Unit を LoadUnit したのと同じ結果になるようにする。
type Splitter interface {
	Units() []Unit
	LoadUnit(u Unit, emit func(*core.Builder)) error
}

// Stamp は、files の名前・大きさ・更新時刻をまとめた印（ないファイルは「ない」として入れる）。
func Stamp(files []string) string {
	h := sha256.New()
	for _, p := range files {
		if info, err := os.Stat(p); err == nil {
			fmt.Fprintf(h, "%s\x00%d\x00%d\n", p, info.Size(), info.ModTime().UnixNano())
		} else {
			fmt.Fprintf(h, "%s\x00-\n", p)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
