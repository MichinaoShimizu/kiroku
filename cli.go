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

const helpText = `kiroku %s - review your AI agent work history (Claude Code, Kiro, Kiro Crew, Amazon Q, Codex)

Usage:
  kiroku <command> [flags] [args]

Commands:
  serve [ADDR]          Open the dashboard and keep it live as new history arrives (default 127.0.0.1:8484)
  html                  Write a self-contained HTML report (default kiroku.html) and open it
  json                  Write the aggregated data as JSON (default kiroku.json, "-" for stdout)
  version               Print the version
  update                Update kiroku to the latest release
  help                  Show this help

Run "kiroku <command> --help" for a command's flags.
Get started: kiroku serve
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
		root:      fs.String("root", source.DefaultClaudeRoot(), "Claude Code history directory"),
		kiroHome:  fs.String("kiro-home", source.DefaultKiroHome(), "Kiro data directory ($KIRO_HOME)"),
		crewHome:  fs.String("crew-home", "", "Kiro Crew data directory (default $KIROCREW_HOME or <kiro-home>/crew)"),
		kiroCLIDB: fs.String("kiro-cli-db", "", "path to the legacy Kiro CLI data.sqlite3 (default: OS-specific)"),
		amazonQDB: fs.String("amazonq-db", "", "path to the Amazon Q Developer CLI data.sqlite3 (default: OS-specific)"),
		codexHome: fs.String("codex-home", "", "Codex data directory (default $CODEX_HOME or ~/.codex)"),
		sources:   fs.String("sources", "claude,kiro,amazonq,codex", "comma-separated list of sources to read"),
		prices:    fs.String("prices", "", "JSON file overriding the model price table"),
		gap:       fs.Int("gap", 15, "idle `minutes` that split a session into separate blocks"),
	}
}

// loader は、選んだ履歴を読んで集計する関数を作る。
func (c *common) loader() ([]source.Source, func() snapshot, error) {
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
		meta := map[string]any{"report": rep}
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
		fmt.Fprintf(fs.Output(), "Usage:\n  kiroku %s\n\nFlags:\n", usage)
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
	fs := newFS("serve", "serve [flags] [ADDR]\n\nADDR defaults to "+defaultAddr+"; a bare port such as :8485 also works.")
	c := addCommon(fs)
	interval := fs.Duration("interval", 5*time.Second, "how often to check the history for changes")
	noOpen := fs.Bool("no-open", false, "do not open a browser")
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
	picked, load, err := c.loader()
	if err != nil {
		return err
	}
	return serveLive(addr, *interval, picked, load, !*noOpen)
}

func cmdHTML(args []string) error {
	fs := newFS("html", "html [flags]")
	c := addCommon(fs)
	out := fs.String("o", "kiroku.html", "output `file`")
	fs.StringVar(out, "out", "kiroku.html", "same as -o")
	noOpen := fs.Bool("no-open", false, "do not open a browser")
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	_, load, err := c.loader()
	if err != nil {
		return err
	}
	return writeHTML(load(), *out, !*noOpen)
}

func cmdJSON(args []string) error {
	fs := newFS("json", "json [flags]")
	c := addCommon(fs)
	out := fs.String("o", "kiroku.json", "output `file` (\"-\" for stdout)")
	fs.StringVar(out, "out", "kiroku.json", "same as -o")
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	if *out == "-" {
		logw = io.Discard // 標準出力を JSON だけにする
	}
	snap, err := loadNonEmpty(c)
	if err != nil {
		return err
	}
	return writeJSON(snap, *out)
}

func loadNonEmpty(c *common) (snapshot, error) {
	_, load, err := c.loader()
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

// runLegacy は前の書き方（kiroku --serve、--json、-o など）。何も選ばなければヘルプを出す。
func runLegacy(args []string) error {
	args = normalizeArgs(args)
	fs := flag.NewFlagSet("kiroku", flag.ContinueOnError)
	fs.Usage = func() { printHelp(fs.Output()) }
	c := addCommon(fs)
	out := fs.String("out", "kiroku.html", "")
	fs.StringVar(out, "o", "kiroku.html", "")
	noOpen := fs.Bool("no-open", false, "")
	fs.String("md-dir", ".", "")
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
	case *weekly != "" || *monthly != "":
		return fmt.Errorf("週次・月次サマリーの Markdown 書き出しはなくなりました。kiroku serve の画面で見てね")
	case *serve != "":
		note("kiroku serve")
		picked, load, err := c.loader()
		if err != nil {
			return err
		}
		return serveLive(*serve, *interval, picked, load, !*noOpen)
	case *jsonOut != "":
		note("kiroku json -o " + *jsonOut)
		snap, err := loadNonEmpty(c)
		if err != nil {
			return err
		}
		return writeJSON(snap, *jsonOut)
	case explicitOut:
		note("kiroku html -o " + *out)
		_, load, err := c.loader()
		if err != nil {
			return err
		}
		return writeHTML(load(), *out, !*noOpen)
	}
	printHelp(os.Stdout)
	return nil
}
