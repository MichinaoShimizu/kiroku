package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
	"github.com/MichinaoShimizu/kiroku/internal/web"
)

const helpText = `kiroku %s — AI エージェント（Claude Code・Kiro・Kiro Crew・Amazon Q・Codex）の作業履歴を振り返る

使い方:
  kiroku serve [待ち受け先]   画面を開く。作業中に増えた履歴もその場で反映する（いちばんよく使う）
  kiroku html [-o ファイル]   1 つの HTML に書き出す（持ち運び・共有用）
  kiroku weekly [日付]        週次サマリーを Markdown に書き出す（日付なしなら最新の週）
  kiroku monthly [YYYY-MM]    月次サマリーを Markdown に書き出す（なしなら最新の月）
  kiroku json [-o ファイル]   集計を JSON に書き出す（ほかのツール向け）
  kiroku version              版を表示する
  kiroku update [--check]     最新の版に入れかえる

まずは:  kiroku serve
コマンドごとのオプション:  kiroku <コマンド> --help
`

func main() {
	cleanupOldExe()
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printHelp(w io.Writer) { fmt.Fprintf(w, helpText, version) }

// dispatch はサブコマンドを振り分ける。サブコマンドがなければヘルプを出すだけ。
func dispatch(args []string) error {
	if len(args) == 0 {
		printHelp(os.Stdout)
		return nil
	}
	switch args[0] {
	case "serve":
		return cmdServe(args[1:])
	case "html":
		return cmdHTML(args[1:])
	case "weekly":
		return cmdWeekly(args[1:])
	case "monthly":
		return cmdMonthly(args[1:])
	case "json":
		return cmdJSON(args[1:])
	case "version", "--version", "-version", "-v":
		return runVersion()
	case "update":
		return runUpdate(args[1:])
	case "help", "--help", "-help", "-h":
		printHelp(os.Stdout)
		return nil
	}
	if strings.HasPrefix(args[0], "-") {
		return runLegacy(args) // 前の書き方（kiroku --serve など）
	}
	return fmt.Errorf("知らないサブコマンドです: %s（kiroku help で一覧）", args[0])
}

// common は、履歴を読むコマンドに共通のオプション。
type common struct {
	root, kiroHome, crewHome, kiroCLIDB, amazonQDB, codexHome, sources, prices *string
	gap                                                                        *int
}

func addCommon(fs *flag.FlagSet) *common {
	return &common{
		root:      fs.String("root", source.DefaultClaudeRoot(), "Claude Code の履歴の場所"),
		kiroHome:  fs.String("kiro-home", source.DefaultKiroHome(), "Kiro のデータの場所（KIRO_HOME）"),
		crewHome:  fs.String("crew-home", "", "Kiro Crew のデータの場所（空なら KIROCREW_HOME か <kiro-home>/crew）"),
		kiroCLIDB: fs.String("kiro-cli-db", "", "Kiro CLI（古い版）の data.sqlite3 の場所（空なら OS ごとの場所）"),
		amazonQDB: fs.String("amazonq-db", "", "Amazon Q Developer CLI の data.sqlite3 の場所（空なら OS ごとの場所）"),
		codexHome: fs.String("codex-home", "", "Codex のデータの場所（空なら CODEX_HOME か ~/.codex）"),
		sources:   fs.String("sources", "claude,kiro,amazonq,codex", "読む履歴（カンマ区切り）"),
		prices:    fs.String("prices", "", "料金表の上書き（JSON）"),
		gap:       fs.Int("gap", 15, "何分あいたら帯を分けるか"),
	}
}

// loader は、選んだ履歴を読んで集計する関数を作る。
func (c *common) loader(mdDir string) ([]source.Source, func() snapshot, error) {
	if *c.prices != "" {
		if err := loadPrices(*c.prices); err != nil {
			return nil, nil, fmt.Errorf("料金表を読めなかったよ: %w", err)
		}
	}
	want := map[string]bool{}
	for _, s := range strings.Split(*c.sources, ",") {
		want[strings.ToLower(strings.TrimSpace(s))] = true
	}
	var picked []source.Source
	for _, s := range source.All(source.Options{ClaudeRoot: *c.root, KiroHome: *c.kiroHome, KiroCLIDB: *c.kiroCLIDB,
		AmazonQDB: *c.amazonQDB, CrewHome: *c.crewHome, CodexHome: *c.codexHome}) {
		if want[s.Family()] {
			picked = append(picked, s)
		}
	}
	gap := *c.gap
	load := func() snapshot {
		data, rep := collect(picked, want, gap)
		if data == nil {
			data = []*core.Session{} // 画面では null ではなく空の一覧として扱う
		}
		meta := map[string]any{"report": rep, "mdDir": filepath.Clean(mdDir)}
		return snapshot{data: data, weeks: report.AllWeeks(data), months: report.AllMonths(data), meta: meta, rep: rep, gen: float64(time.Now().UnixNano()) / 1e9}
	}
	return picked, load, nil
}

// parse はオプションと、オプションの前後に置いた位置引数（最大 maxPos 個）を読む。
func parse(fs *flag.FlagSet, args []string, maxPos int) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if len(pos) > maxPos {
		return nil, fmt.Errorf("引数が多すぎるよ: %s（kiroku %s --help）", strings.Join(pos[maxPos:], " "), fs.Name())
	}
	return pos, nil
}

func newFS(name, usage string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "使い方: kiroku %s\n\nオプション:\n", usage)
		fs.PrintDefaults()
	}
	return fs
}

// quiet は --help のときにエラーにしない。
func quiet(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

func cmdServe(args []string) error {
	fs := newFS("serve", "serve [待ち受け先]   （既定 "+defaultAddr+"。:8485 のようにポートだけでも）")
	c := addCommon(fs)
	interval := fs.Duration("interval", 5*time.Second, "履歴の変化を確かめる間隔")
	noOpen := fs.Bool("no-open", false, "ブラウザを開かない")
	mdDir := fs.String("md-dir", ".", "画面に出す Markdown の書き出し先（コマンドの例に使う）")
	pos, err := parse(fs, args, 1)
	if err != nil {
		return quiet(err)
	}
	addr := defaultAddr
	if len(pos) == 1 {
		if !addrRe.MatchString(pos[0]) {
			return fmt.Errorf("待ち受け先は :8485 や 127.0.0.1:8485 の形で書いてね: %s", pos[0])
		}
		addr = pos[0]
	}
	picked, load, err := c.loader(*mdDir)
	if err != nil {
		return err
	}
	return serveLive(addr, *interval, picked, load, !*noOpen)
}

func cmdHTML(args []string) error {
	fs := newFS("html", "html [-o ファイル]")
	c := addCommon(fs)
	out := fs.String("o", "kiroku.html", "書き出す HTML")
	fs.StringVar(out, "out", "kiroku.html", "-o と同じ")
	noOpen := fs.Bool("no-open", false, "ブラウザを開かない")
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	_, load, err := c.loader(".")
	if err != nil {
		return err
	}
	return writeHTML(load(), *out, !*noOpen)
}

func cmdWeekly(args []string) error {
	fs := newFS("weekly", "weekly [YYYY-MM-DD]   （その日を含む週。なければ最新の週）")
	c := addCommon(fs)
	mdDir := fs.String("md-dir", ".", "Markdown を置くフォルダ")
	pos, err := parse(fs, args, 1)
	if err != nil {
		return quiet(err)
	}
	when := "latest"
	if len(pos) == 1 {
		when = pos[0]
	}
	snap, err := loadNonEmpty(c, *mdDir)
	if err != nil {
		return err
	}
	return writeWeekly(snap.weeks, snap.rep, when, *mdDir)
}

func cmdMonthly(args []string) error {
	fs := newFS("monthly", "monthly [YYYY-MM]   （なければ最新の月）")
	c := addCommon(fs)
	mdDir := fs.String("md-dir", ".", "Markdown を置くフォルダ")
	pos, err := parse(fs, args, 1)
	if err != nil {
		return quiet(err)
	}
	when := "latest"
	if len(pos) == 1 {
		when = pos[0]
	}
	snap, err := loadNonEmpty(c, *mdDir)
	if err != nil {
		return err
	}
	return writeMonthly(snap.months, snap.rep, when, *mdDir)
}

func cmdJSON(args []string) error {
	fs := newFS("json", "json [-o ファイル]   （- なら標準出力）")
	c := addCommon(fs)
	out := fs.String("o", "kiroku.json", "書き出す JSON（- なら標準出力）")
	fs.StringVar(out, "out", "kiroku.json", "-o と同じ")
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	if *out == "-" {
		logw = io.Discard // 標準出力を JSON だけにする
	}
	snap, err := loadNonEmpty(c, ".")
	if err != nil {
		return err
	}
	return writeJSON(snap, *out)
}

func loadNonEmpty(c *common, mdDir string) (snapshot, error) {
	_, load, err := c.loader(mdDir)
	if err != nil {
		return snapshot{}, err
	}
	snap := load()
	if len(snap.data) == 0 {
		return snap, fmt.Errorf("履歴が 1 件も見つからなかったよ。--root や KIRO_HOME を確認してね")
	}
	return snap, nil
}

func writeHTML(snap snapshot, out string, open bool) error {
	if len(snap.data) == 0 {
		return fmt.Errorf("履歴が 1 件も見つからなかったよ。--root や KIRO_HOME を確認してね")
	}
	html, err := web.Render(snap.data, snap.weeks, snap.months, snap.meta, snap.gen, false)
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, []byte(html), 0o644); err != nil {
		return err
	}
	fmt.Printf("%d セッション → %s\n", len(snap.data), out)
	if open {
		if abs, err := filepath.Abs(out); err == nil {
			openBrowser(abs)
		}
	}
	return nil
}

func writeJSON(snap snapshot, out string) error {
	b, err := json.MarshalIndent(map[string]any{"sessions": snap.data, "weeks": snap.weeks, "months": snap.months, "meta": snap.meta}, "", " ")
	if err != nil {
		return err
	}
	if out == "-" {
		_, err = os.Stdout.Write(append(b, '\n'))
		return err
	}
	return os.WriteFile(out, b, 0o644)
}

// runLegacy は前の書き方（kiroku --serve、--weekly、--json など）。何も選ばなければヘルプを出す。
func runLegacy(args []string) error {
	args = normalizeArgs(args)
	fs := flag.NewFlagSet("kiroku", flag.ContinueOnError)
	fs.Usage = func() { printHelp(fs.Output()) }
	c := addCommon(fs)
	out := fs.String("out", "kiroku.html", "")
	fs.StringVar(out, "o", "kiroku.html", "")
	noOpen := fs.Bool("no-open", false, "")
	mdDir := fs.String("md-dir", ".", "")
	weekly := fs.String("weekly", "", "")
	monthly := fs.String("monthly", "", "")
	jsonOut := fs.String("json", "", "")
	serve := fs.String("serve", "", "")
	interval := fs.Duration("interval", 5*time.Second, "")
	showVersion := fs.Bool("version", false, "")
	if err := fs.Parse(args); err != nil {
		return quiet(err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("知らないサブコマンドです: %s（kiroku help で一覧）", fs.Arg(0))
	}
	explicitOut := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "o" || f.Name == "out" {
			explicitOut = true
		}
	})
	note := func(newForm string) {
		fmt.Fprintf(os.Stderr, "（この書き方は古くなりました。これからは %s を使ってね）\n", newForm)
	}
	switch {
	case *showVersion:
		return runVersion()
	case *serve != "":
		note("kiroku serve")
		picked, load, err := c.loader(*mdDir)
		if err != nil {
			return err
		}
		return serveLive(*serve, *interval, picked, load, !*noOpen)
	case *weekly != "":
		note("kiroku weekly")
		snap, err := loadNonEmpty(c, *mdDir)
		if err != nil {
			return err
		}
		return writeWeekly(snap.weeks, snap.rep, *weekly, *mdDir)
	case *monthly != "":
		note("kiroku monthly")
		snap, err := loadNonEmpty(c, *mdDir)
		if err != nil {
			return err
		}
		return writeMonthly(snap.months, snap.rep, *monthly, *mdDir)
	case *jsonOut != "":
		note("kiroku json -o " + *jsonOut)
		snap, err := loadNonEmpty(c, *mdDir)
		if err != nil {
			return err
		}
		return writeJSON(snap, *jsonOut)
	case explicitOut:
		note("kiroku html -o " + *out)
		_, load, err := c.loader(*mdDir)
		if err != nil {
			return err
		}
		return writeHTML(load(), *out, !*noOpen)
	}
	printHelp(os.Stdout)
	return nil
}
