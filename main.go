// kiroku — AI エージェント（Claude Code・Kiro・Kiro Crew・Amazon Q・Codex）の作業履歴を週カレンダーで振り返る。
//
// 使い方:
//
//	kiroku                         # kiroku.html を作ってブラウザで開く
//	kiroku --sources kiro          # Kiro だけ
//	kiroku --weekly                # 最新の週のふりかえりを Markdown で書き出す
//	kiroku --weekly 2026-09-30     # その日を含む週
package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	"github.com/MichinaoShimizu/kiroku/internal/web"
)

var version = "dev" // リリース時に -ldflags で入れる

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// normalizeArgs は「--weekly」だけ（日付なし）を --weekly=latest に直す。flag パッケージは値の省略ができないため。
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
		out = append(out, a)
	}
	return out
}

func main() {
	if err := run(normalizeArgs(os.Args[1:])); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("kiroku", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "kiroku %s — AI エージェントの作業履歴を週カレンダーで振り返る\n\n", version)
		fs.PrintDefaults()
	}
	root := fs.String("root", source.DefaultClaudeRoot(), "Claude Code の履歴の場所")
	kiroHome := fs.String("kiro-home", source.DefaultKiroHome(), "Kiro のデータの場所（KIRO_HOME）")
	out := fs.String("out", "kiroku.html", "書き出す HTML")
	fs.StringVar(out, "o", "kiroku.html", "--out の短い形")
	gap := fs.Int("gap", 15, "何分あいたら帯を分けるか")
	sources := fs.String("sources", "claude,kiro,amazonq,codex", "読む履歴（カンマ区切り）")
	codexHome := fs.String("codex-home", "", "Codex のデータの場所（空なら CODEX_HOME か ~/.codex）")
	crewHome := fs.String("crew-home", "", "Kiro Crew のデータの場所（空なら KIROCREW_HOME か <kiro-home>/crew）")
	kiroCLIDB := fs.String("kiro-cli-db", "", "Kiro CLI（古い版）の data.sqlite3 の場所（空なら OS ごとの場所）")
	amazonQDB := fs.String("amazonq-db", "", "Amazon Q Developer CLI の data.sqlite3 の場所（空なら OS ごとの場所）")
	noOpen := fs.Bool("no-open", false, "ブラウザを開かない")
	prices := fs.String("prices", "", "料金表の上書き（JSON）")
	journal := fs.String("journal", ".", "週のふりかえり（判断ログ）を置くフォルダ")
	weekly := fs.String("weekly", "", "その日を含む週のふりかえりを Markdown で書き出す（日付なしなら最新の週）")
	jsonOut := fs.String("json", "", "集計結果を JSON で書き出す（テストや他のツール向け）")
	showVersion := fs.Bool("version", false, "版を表示する")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *showVersion {
		fmt.Println("kiroku", version)
		return nil
	}
	if *prices != "" {
		if err := loadPrices(*prices); err != nil {
			return fmt.Errorf("料金表を読めなかったよ: %w", err)
		}
	}
	want := map[string]bool{}
	for _, s := range strings.Split(*sources, ",") {
		want[strings.ToLower(strings.TrimSpace(s))] = true
	}
	data, rep := collect(source.All(source.Options{ClaudeRoot: *root, KiroHome: *kiroHome, KiroCLIDB: *kiroCLIDB, AmazonQDB: *amazonQDB, CrewHome: *crewHome, CodexHome: *codexHome}), want, *gap)
	if len(data) == 0 {
		return fmt.Errorf("履歴が 1 件も見つからなかったよ。--root や KIRO_HOME を確認してね")
	}
	weeks := report.AllWeeks(data)
	report.Annotate(weeks, report.ReadJournal(*journal))

	if *weekly != "" {
		return writeWeekly(weeks, rep, *weekly, *journal)
	}
	meta := map[string]any{"report": rep, "version": report.MetricsVersion, "journal": filepath.Clean(*journal),
		"metrics": report.Metrics, "focus": report.Focus, "decisions": report.Decisions,
		"baselineMin": report.BaselineMin, "baselineWeeks": report.BaselineWeeks}
	if *jsonOut != "" {
		b, err := json.MarshalIndent(map[string]any{"sessions": data, "weeks": weeks, "meta": meta}, "", " ")
		if err != nil {
			return err
		}
		return os.WriteFile(*jsonOut, b, 0o644)
	}
	html, err := web.Render(data, weeks, meta, float64(time.Now().UnixNano())/1e9)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, []byte(html), 0o644); err != nil {
		return err
	}
	fmt.Printf("%d セッション → %s\n", len(data), *out)
	if !*noOpen {
		openBrowser(*out)
	}
	return nil
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
			fmt.Fprintf(os.Stderr, "  %s: 読めないファイルがあったのでスキップ (%s)\n", s.Name(), msg)
		}
		fmt.Fprintf(os.Stderr, "  %s: %d セッション (%s)\n", s.Name(), n, s.Where())
		rep = append(rep, r)
	}
	sort.SliceStable(data, func(i, j int) bool { return data[i].Start < data[j].Start })
	return data, rep
}

func writeWeekly(weeks map[string]*report.Week, rep []source.Report, when, folder string) error {
	key := ""
	if when == "latest" {
		for k := range weeks {
			if k > key {
				key = k
			}
		}
	} else {
		t, err := time.ParseInLocation("2006-01-02", when, time.Local)
		if err != nil {
			return fmt.Errorf("--weekly の日付は YYYY-MM-DD で書いてね: %s", when)
		}
		key = report.MondayOf(float64(t.Unix())).Format("2006-01-02")
	}
	st := weeks[key]
	if st == nil {
		return fmt.Errorf("%s の週には履歴がないよ", key)
	}
	path := filepath.Join(folder, "kiroku-week-"+key+".md")
	existing := ""
	if b, err := os.ReadFile(path); err == nil {
		existing = string(b)
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(report.WeeklyMarkdown(st, rep, existing)), 0o644); err != nil {
		return err
	}
	note := ""
	if strings.Contains(existing, report.Mark) {
		note = "（自分で書いた欄はそのまま残しました）"
	}
	fmt.Printf("週のふりかえり → %s%s\n", path, note)
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

func openBrowser(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", abs)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", abs)
	default:
		cmd = exec.Command("xdg-open", abs)
	}
	_ = cmd.Start()
}
