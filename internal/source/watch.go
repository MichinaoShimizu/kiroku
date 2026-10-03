package source

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
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
