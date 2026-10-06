// Package cli は kiroku のコマンド（serve・html・json・archive・autostart・doctor・update など）。
// 入口はリポジトリ直下の main.go で、使い方は cli.go（kiroku help）を見てね。
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

var version = "dev" // Main で、ルートの main.go の version を入れる

var (
	dateRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	monthRe = regexp.MustCompile(`^\d{4}-\d{2}$`)
)

// addrRe は kiroku serve のあとに書ける待ち受け先（:8484、127.0.0.1:8484、localhost:8484 など）。
var addrRe = regexp.MustCompile(`^[\w.\-\[\]:]*:\d+$`)

// defaultAddr は kiroku serve の待ち受け先。履歴は人に見せたくないものなので、手元からしか開けないようにする。
const defaultAddr = "127.0.0.1:8484"

// logw は読み込みの経過を書く先。serve の読み直しや json -o - では黙らせる。
var logw io.Writer = os.Stderr

// normalizeArgs は前の書き方（runLegacy）用。「--weekly」「--monthly」「--serve」だけ（値なし）を --weekly=latest、--monthly=latest、--serve=127.0.0.1:8484 に直す。flag パッケージは値の省略ができないため。
func normalizeArgs(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--weekly" || a == "-weekly" {
			if i+1 < len(args) && dateRe.MatchString(args[i+1]) {
				out = append(out, "--weekly="+args[i+1])
				i++
			} else {
				out = append(out, "--weekly=latest")
			}
			continue
		}
		if a == "--monthly" || a == "-monthly" {
			if i+1 < len(args) && monthRe.MatchString(args[i+1]) {
				out = append(out, "--monthly="+args[i+1])
				i++
			} else {
				out = append(out, "--monthly=latest")
			}
			continue
		}
		if a == "--serve" || a == "-serve" {
			if i+1 < len(args) && addrRe.MatchString(args[i+1]) {
				out = append(out, "--serve="+args[i+1])
				i++
			} else {
				out = append(out, "--serve="+defaultAddr)
			}
			continue
		}
		out = append(out, a)
	}
	return out
}

func collect(all []source.Source, want map[string]bool, gap int) ([]*core.Session, []source.Report) {
	return collectCached(all, want, gap, nil)
}

// collectCached は collect と同じ。cache があれば、前回から変わっていない履歴は読み直さない（kiroku serve 用）。
func collectCached(all []source.Source, want map[string]bool, gap int, cache *loadCache) ([]*core.Session, []source.Report) {
	var data []*core.Session
	var rep []source.Report
	seen := map[string]bool{} // 同じ会話が 2 か所に残っていたら、先に読んだほうを使う
	for _, s := range all {
		if !want[s.Family()] {
			continue
		}
		n, dup, archived, oldest := 0, 0, 0, 0.0
		var ids []string
		_, keeps := s.(source.Keeper)
		outs, err := cache.load(s, gap)
		for _, o := range outs {
			if o.key != "" {
				if seen[o.key] {
					dup++
					continue
				}
				seen[o.key] = true
			}
			n++
			if sess := o.sess; sess != nil {
				data = append(data, sess)
				ids = append(ids, sess.ID)
				if keeps && strings.HasSuffix(sess.File, ".zst") { // 元が消えて、kiroku archive のコピーから読んだ
					archived++
				}
				if sess.Start > 0 && (oldest == 0 || sess.Start < oldest) {
					oldest = sess.Start
				}
			}
		}
		r := source.Report{Name: s.Name(), N: n, Dup: dup, Archived: archived, Where: s.Where(), Oldest: oldest, IDs: ids}
		if k, ok := s.(source.Retainer); ok {
			r.Keep = k.Retention()
		}
		if d, ok := s.(source.Detailer); ok {
			r.Detail, r.DetailEn = d.Detail(), d.DetailEn()
		}
		if err != nil {
			msg := err.Error()
			r.Error = &msg
			fmt.Fprintf(logw, "  %s: skipped unreadable files (%s)\n", s.Name(), msg)
		}
		fmt.Fprintf(logw, "  %s: %d sessions (%s)\n", s.Name(), n, s.Where())
		rep = append(rep, r)
	}
	sort.SliceStable(data, func(i, j int) bool { return data[i].Start < data[j].Start })
	return data, rep
}

func loadPrices(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		var p [5]float64
		switch x := v.(type) {
		case []any:
			for i := 0; i < 5 && i < len(x); i++ {
				p[i] = core.NumOr0(x[i])
			}
		case map[string]any:
			for i, n := range []string{"input", "output", "cache_write", "cache_write_1h", "cache_read"} {
				p[i] = core.NumOr0(x[n])
			}
		}
		core.Prices[strings.ToLower(k)] = p
	}
	return nil
}

// openBrowser は、ファイルの絶対パスか URL を既定のブラウザで開く。
var openBrowser = func(target string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	_ = cmd.Start()
}

// cleanupOldExe は、Windows で update したときに残る kiroku.exe.old を消す（使っていなければ）。
func cleanupOldExe() {
	if runtime.GOOS != "windows" {
		return
	}
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
	}
}
