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
// 読む側（glob・ファイルを開く）はシンボリックリンクの先も読むので、指紋もそれに合わせる: paths 自体がリンクならその先を、
// 中のリンクは os.Stat でその先の大きさ・更新時刻を入れ、フォルダへのリンクはその中も見る（もう見た場所の中を指すリンクは
// 2 度見ない。ループしない）。os.Stat とフォルダを並べるだけで、ファイルは開かない（名前つきパイプで止まらない）。
// リンクの先が大きすぎる（大きなフォルダを指している）ときは、fingerprintLinkBudget 件で見るのをやめる。
func Fingerprint(paths []string) string {
	h := sha256.New()
	budget := fingerprintLinkBudget // リンクの先で見てよいものの数
	var done []string               // 見たフォルダ（リンクを解いた本当の場所）
	seen := func(p string) bool {
		for _, d := range done {
			if rel, err := filepath.Rel(d, p); err == nil && (rel == "." || filepath.IsLocal(rel)) {
				return true
			}
		}
		return false
	}
	var walk func(root string, viaLink bool)
	walk = func(root string, viaLink bool) {
		real, err := filepath.EvalSymlinks(root)
		if err != nil {
			fmt.Fprintf(h, "%s\x00-\n", root) // ない・リンクの先がない（あとからできたら変わる）
			return
		}
		if seen(real) {
			return
		}
		var links []string // 中のフォルダへのリンク（このフォルダを見終えてから見る）
		_ = filepath.WalkDir(real, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // 消えた・読めないものは飛ばす
			}
			if viaLink {
				if budget--; budget < 0 {
					return fs.SkipAll
				}
			}
			if d.IsDir() {
				return nil
			}
			var info fs.FileInfo
			if d.Type()&fs.ModeSymlink != 0 {
				info, err = os.Stat(p) // リンクの先（開かない）
				if err != nil {
					fmt.Fprintf(h, "%s\x00-\n", p)
					return nil
				}
				if info.IsDir() {
					links = append(links, p)
					return nil
				}
			} else if info, err = d.Info(); err != nil {
				return nil
			}
			fmt.Fprintf(h, "%s\x00%d\x00%d\x00%d\n", p, info.Size(), info.ModTime().UnixNano(), info.Mode())
			return nil
		})
		done = append(done, real)
		for _, l := range links {
			fmt.Fprintf(h, "%s\x00>\n", l)
			walk(l, true)
		}
	}
	for _, root := range paths {
		if root != "" {
			walk(root, false)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// fingerprintLinkBudget は、Fingerprint がフォルダへのリンクの先で見るものの数の上限（1 回の指紋につき）。
const fingerprintLinkBudget = 100000

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
