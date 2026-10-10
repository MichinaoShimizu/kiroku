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
	"sync"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

var version = "dev" // Main で、ルートの main.go の version を入れる

// addrRe は kiroku serve のあとに書ける待ち受け先（:8484、127.0.0.1:8484、localhost:8484 など）。
var addrRe = regexp.MustCompile(`^[\w.\-\[\]:]*:\d+$`)

// defaultAddr は kiroku serve の待ち受け先。履歴は人に見せたくないものなので、手元からしか開けないようにする。
const defaultAddr = "127.0.0.1:8484"

// logw は読み込みの経過を書く先。serve の読み直しや json -o - では黙らせる。
var logw io.Writer = os.Stderr

func collect(all []source.Source, want map[string]bool, gap int) ([]*core.Session, []source.Report) {
	return collectCached(all, want, gap, nil)
}

// collectCached は collect と同じ。cache があれば、前回から変わっていない履歴は読み直さない（kiroku serve 用）。
func collectCached(all []source.Source, want map[string]bool, gap int, cache *loadCache) ([]*core.Session, []source.Report) {
	var data []*core.Session
	var rep []source.Report
	var picked []source.Source
	for _, s := range all {
		if want[s.Family()] {
			picked = append(picked, s)
		}
	}
	// エージェントどうしは互いに関係なく読めるので、並べて読む（いちばん遅いエージェントの時間で済む）。
	// 同じ会話を 2 度数えないための突き合わせは、読み終えてから all の順に行う
	type result struct {
		outs []loaded
		err  error
		took time.Duration
	}
	res := make([]result, len(picked))
	var wg sync.WaitGroup
	for i, s := range picked {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t0 := time.Now()
			outs, err := cache.load(s, gap)
			res[i] = result{outs, err, time.Since(t0)}
			tracker.agentDone(s.Name(), sessions(outs), res[i].took)
		}()
	}
	wg.Wait()
	// 中身を出してはいけない会話（Kiro Crew の incognito・temporary）は、同じエージェントの別の履歴に残る写し
	// （Kiro CLI の SQLite）も隠す。Amazon Q は同じ形の Key を使うが別のエージェントなので、エージェントの家族（Family）が同じものだけ
	withheld := map[string]bool{} // Family + "\x00" + Key
	for _, s := range picked {
		if w, ok := s.(source.Withholder); ok {
			for _, key := range w.Withheld() {
				withheld[s.Family()+"\x00"+key] = true
			}
		}
	}
	seen := map[string]bool{} // 同じ会話が 2 か所に残っていたら、先に読んだほうを使う
	// Claim した会話のうち、セッションになったもの（時刻のない空の会話では、ほかの場所の写しを隠さない）
	claimed := map[string]bool{}
	for i := range picked {
		for _, o := range res[i].outs {
			if o.claim != "" && o.sess != nil {
				claimed[o.claim] = true
			}
		}
	}
	for i, s := range picked {
		n, dup, archived, oldest := 0, 0, 0, 0.0
		var ids []string
		_, keeps := s.(source.Keeper)
		outs, err := res[i].outs, res[i].err
		for _, o := range outs {
			if o.yield != "" && claimed[o.yield] {
				dup++
				continue
			}
			if o.key != "" {
				if seen[o.key] {
					dup++
					continue
				}
				seen[o.key] = true
			}
			n++
			if sess := o.sess; sess != nil {
				if o.key != "" && withheld[s.Family()+"\x00"+o.key] {
					// 写しを隠す。sess は kiroku serve のキャッシュにあり、前の画面のデータとしても読まれているので変えない
					data = append(data, source.HideWithheld(sess))
				} else {
					data = append(data, sess)
				}
				ids = append(ids, sess.ID)
				if keeps && strings.HasSuffix(sess.File, ".zst") { // 元が消えて、kiroku archive のコピーから読んだ
					archived++
				}
				if sess.Start > 0 && (oldest == 0 || sess.Start < oldest) {
					oldest = sess.Start
				}
			}
		}
		r := source.Report{Name: s.Name(), N: n, Dup: dup, Archived: archived, Where: s.Where(), Oldest: oldest, IDs: ids, Took: res[i].took}
		if k, ok := s.(source.Retainer); ok {
			r.Keep = k.Retention()
		}
		if d, ok := s.(source.Detailer); ok {
			r.Detail, r.DetailEn = d.Detail(), d.DetailEn()
		}
		if err != nil {
			msg := err.Error()
			r.Error = &msg
			fmt.Fprintf(logw, "  %s %s: skipped unreadable files (%s)\n", styleFor(logw).warn("!"), s.Name(), msg)
		}
		rep = append(rep, r)
	}
	logSources(rep, len(data) > 0)
	sort.SliceStable(data, func(i, j int) bool { return data[i].Start < data[j].Start })
	return data, rep
}

// sessions は outs のうち、セッションになったもの（時刻のあるもの）の数。
func sessions(outs []loaded) int {
	n := 0
	for _, o := range outs {
		if o.sess != nil {
			n++
		}
	}
	return n
}

// logSources は、読んだエージェントの一覧を logw に書く。履歴があったものだけを並べ、残りは1行にまとめる
// （0 件が並ぶと、serve の address や key の行がその上に押し出されてしまうため）。
// どれも 0 件のときは、どこを探したのか分かるように全部書く。
func logSources(rep []source.Report, found bool) {
	st := styleFor(logw)
	var none []string
	for _, r := range rep {
		if found && r.N == 0 {
			none = append(none, r.Name)
			continue
		}
		fmt.Fprintf(logw, "  %s: %s %s %s\n", r.Name, plural(r.N, "session"), st.dim("("+where(r.Where)+")"), st.dim("in "+took(r.Took).String()))
	}
	if len(none) > 0 {
		fmt.Fprintf(logw, "  %s\n", st.dim("no history yet: "+strings.Join(none, ", ")))
	}
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
		delete(core.LongPrices, strings.ToLower(k)) // 足した料金はプロンプトの長さによらず使う
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
