// kiroku — AI エージェント（Claude Code・Kiro・Kiro Crew・Amazon Q・Codex）の作業履歴を週カレンダーで振り返る。
//
// 使い方は cli.go（kiroku help）を見てね。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
)

var version = "dev" // リリース時に -ldflags で入れる

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
	var data []*core.Session
	var rep []source.Report
	seen := map[string]bool{} // 同じ会話が 2 か所に残っていたら、先に読んだほうを使う
	for _, s := range all {
		if !want[s.Family()] {
			continue
		}
		n, dup := 0, 0
		err := s.Load(func(b *core.Builder) {
			if b.Key != "" {
				if seen[b.Key] {
					dup++
					return
				}
				seen[b.Key] = true
			}
			n++
			if sess := b.Finish(gap); sess != nil {
				data = append(data, sess)
			}
		})
		r := source.Report{Name: s.Name(), N: n, Dup: dup, Where: s.Where()}
		if d, ok := s.(source.Detailer); ok {
			r.Detail = d.Detail()
		}
		if err != nil {
			msg := err.Error()
			r.Error = &msg
			fmt.Fprintf(logw, "  %s: 読めないファイルがあったのでスキップ (%s)\n", s.Name(), msg)
		}
		fmt.Fprintf(logw, "  %s: %d セッション (%s)\n", s.Name(), n, s.Where())
		rep = append(rep, r)
	}
	sort.SliceStable(data, func(i, j int) bool { return data[i].Start < data[j].Start })
	return data, rep
}

func writeWeekly(weeks map[string]*report.Week, rep []source.Report, when, folder string) error {
	key := ""
	if when == "latest" {
		key = latest(weeks)
	} else {
		t, err := time.ParseInLocation("2006-01-02", when, time.Local)
		if err != nil {
			return fmt.Errorf("週の日付は YYYY-MM-DD で書いてね: %s", when)
		}
		key = report.MondayOf(float64(t.Unix())).Format("2006-01-02")
	}
	st := weeks[key]
	if st == nil {
		return fmt.Errorf("%s の週には履歴がないよ", key)
	}
	return writeMarkdown(filepath.Join(folder, "kiroku-week-"+key+".md"), report.WeeklyMarkdown(st, rep), "週次サマリー")
}

func writeMonthly(months map[string]*report.Summary, rep []source.Report, when, folder string) error {
	key := when
	if when == "latest" {
		key = latest(months)
	} else if !monthRe.MatchString(when) {
		return fmt.Errorf("月は YYYY-MM で書いてね: %s", when)
	}
	st := months[key]
	if st == nil {
		return fmt.Errorf("%s には履歴がないよ", key)
	}
	return writeMarkdown(filepath.Join(folder, "kiroku-month-"+key+".md"), report.MonthlyMarkdown(st, rep), "月次サマリー")
}

func latest(m map[string]*report.Summary) string {
	key := ""
	for k := range m {
		if k > key {
			key = k
		}
	}
	return key
}

func writeMarkdown(path, body, what string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s → %s\n", what, path)
	return nil
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
func openBrowser(target string) {
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
