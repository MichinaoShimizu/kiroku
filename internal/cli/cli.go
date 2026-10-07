package cli

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

	"github.com/MichinaoShimizu/kiroku/internal/archive"
	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
	"github.com/MichinaoShimizu/kiroku/internal/web"
)

const helpText = `kiroku %s - see your AI coding agent history as a calendar (Claude Code, Kiro, Kiro Crew, Amazon Q, Codex)

Usage:
  kiroku <command> [flags] [args]

Commands:
  serve [ADDR]          Open the view in your browser and keep it updated as new history arrives (default 127.0.0.1:8484)
  open                  Open the view of a running "kiroku serve" with its key (--print shows the address for another device)
  html                  Write the view as a single static HTML file (default kiroku.html) and open it
                        (--week or --month writes only that period, for sharing)
  json                  Write the aggregated data as JSON (default kiroku.json, "-" for stdout)
  archive [on|off]      Keep compressed copies of history that agents delete (Claude Code, Kiro Crew); no argument shows the status
  autostart [on|off]    Start "kiroku serve" in the background each time you log in; no argument shows the status
  doctor                Check what kiroku can read and whether your history will be deleted, and what to run next
  version               Print the version
  update                Update kiroku to the latest release
  help                  Show this help

Run "kiroku <command> --help" for a command's flags.
Get started: kiroku doctor, then kiroku serve
`

// Main は kiroku のコマンドを動かす。v はリリース時に入る版（ルートの main.go から渡す）。
func Main(v string) {
	version = v
	cleanupOldExe()
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, styleFor(os.Stderr).bad(err.Error()))
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
	case "archive":
		return cmdArchive(args[1:])
	case "open":
		return cmdOpen(args[1:])
	case "autostart":
		return cmdAutostart(args[1:])
	case "doctor":
		return cmdDoctor(args[1:])
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
	return fmt.Errorf("unknown command: %s (run \"kiroku help\" for the list)", args[0])
}

// common は、履歴を読むコマンドに共通のオプション。
type common struct {
	root, kiroHome, crewHome, kiroCLIDB, amazonQDB, codexHome, sources, prices, archiveDir *string
	gap                                                                                    *int
}

func addCommon(fs *flag.FlagSet) *common {
	return &common{
		root:       fs.String("root", source.DefaultClaudeRoot(), "Claude Code history directory ($CLAUDE_CONFIG_DIR/projects or ~/.claude/projects)"),
		kiroHome:   fs.String("kiro-home", source.DefaultKiroHome(), "Kiro data directory ($KIRO_HOME)"),
		crewHome:   fs.String("crew-home", "", "Kiro Crew data directory (default $KIROCREW_HOME or <kiro-home>/crew)"),
		kiroCLIDB:  fs.String("kiro-cli-db", "", "path to the legacy Kiro CLI data.sqlite3 (default: OS-specific)"),
		amazonQDB:  fs.String("amazonq-db", "", "path to the Amazon Q Developer CLI data.sqlite3 (default: OS-specific)"),
		codexHome:  fs.String("codex-home", "", "Codex data directory (default $CODEX_HOME or ~/.codex)"),
		sources:    fs.String("sources", "claude,kiro,amazonq,codex", "comma-separated sources to read: claude, kiro (includes Kiro Crew), amazonq, codex"),
		prices:     fs.String("prices", "", "JSON file overriding the model price table"),
		gap:        fs.Int("gap", 15, "idle `minutes` that split a session into separate blocks"),
		archiveDir: fs.String("archive-dir", archive.DefaultDir(), "where \"kiroku archive\" keeps copies of deleted history ($KIROKU_ARCHIVE_DIR)"),
	}
}

// picked は選んだ履歴。
func (c *common) picked() ([]source.Source, map[string]bool) {
	want := map[string]bool{}
	for _, s := range strings.Split(*c.sources, ",") {
		want[strings.ToLower(strings.TrimSpace(s))] = true
	}
	var picked []source.Source
	for _, s := range source.All(source.Options{ClaudeRoot: *c.root, KiroHome: *c.kiroHome, KiroCLIDB: *c.kiroCLIDB,
		AmazonQDB: *c.amazonQDB, CrewHome: *c.crewHome, CodexHome: *c.codexHome, Archive: *c.archiveDir}) {
		if want[s.Family()] {
			picked = append(picked, s)
		}
	}
	return picked, want
}

// keepFn は、kiroku serve の画面から kiroku archive をオンにする関数（コピーは次の読み直しで残す）。
func (c *common) keepFn() func() error {
	dir := *c.archiveDir
	return func() error { return archive.Enable(dir) }
}

// loader は、選んだ履歴を読んで集計する関数を作る。kiroku archive がオンなら、読む前に消える履歴のコピーを残す。
func (c *common) loader() ([]source.Source, func() snapshot, error) {
	if *c.prices != "" {
		if err := loadPrices(*c.prices); err != nil {
			return nil, nil, fmt.Errorf("could not read the price table: %w", err)
		}
	}
	picked, want := c.picked()
	dir := *c.archiveDir
	gap := *c.gap
	cache := newLoadCache()     // kiroku serve の読み直しで、変わっていない履歴を読み直さない
	gcache := gitlog.NewCache() // git も、変わっていないリポジトリは読み直さない
	load := func() snapshot {
		on := archive.Enabled(dir)
		if on {
			keepCopies(picked, dir)
		}
		data, rep := collectCached(picked, want, gap, cache)
		if data == nil {
			data = []*core.Session{} // 画面では null ではなく空の一覧として扱う
		}
		commits, pushes, stale := gcache.Collect(data) // git がなければ空
		if commits == nil {
			commits = []gitlog.Commit{}
		}
		if pushes == nil {
			pushes = []gitlog.Push{}
		}
		for _, r := range stale {
			fmt.Fprintf(logw, "  git: timed out reading %s (kept the previous result)\n", r)
		}
		files, size := archive.Usage(dir)
		meta := map[string]any{"report": rep, "git": commits, "push": pushes, "prices": map[string]any{"asOf": core.PricesAsOf, "custom": *c.prices != ""},
			"archive": map[string]any{"on": on, "dir": dir, "files": files, "bytes": size}, "version": version}
		if len(stale) > 0 { // 時間切れで読み終わらなかったリポジトリ（前回の結果を使った）
			meta["gitTimeout"] = stale
		}
		if os.Getenv("KIROKU_DEMO") != "" { // デモ（ダミーデータ）：画面は、この時間帯の時計で見せる
			_, off := time.Now().Zone()
			meta["demo"] = map[string]any{"offset": off}
		}
		return snapshot{data: data, weeks: report.AllWeeks(data, commits...), months: report.AllMonths(data, commits...), meta: meta, rep: rep, gen: float64(time.Now().UnixNano()) / 1e9}
	}
	return picked, load, nil
}

// splitList は「a, b」を [a b] にする（空のものは除く）。
func splitList(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
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
		return nil, fmt.Errorf("too many arguments: %s (see \"kiroku %s --help\")", strings.Join(pos[maxPos:], " "), fs.Name())
	}
	return pos, nil
}

func newFS(name, usage string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage:\n  kiroku %s\n", usage)
		n := 0
		fs.VisitAll(func(*flag.Flag) { n++ })
		if n > 0 { // kiroku autostart のようにオプションがないコマンドでは、空の「Flags:」を出さない
			fmt.Fprint(fs.Output(), "\nFlags:\n")
			fs.PrintDefaults()
		}
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
	fs := newFS("serve", "serve [flags] [ADDR]\n\nADDR defaults to "+defaultAddr+". A bare port such as :8485 also listens on 127.0.0.1 only.\nTo let other devices open it, write the address explicitly (e.g. 0.0.0.0:8485); anyone on your network can then see your history.")
	c := addCommon(fs)
	interval := fs.Duration("interval", 5*time.Second, "how often to check the history for changes (reloads after writes settle)")
	noOpen := fs.Bool("no-open", false, "do not open a browser")
	allow := fs.String("allow-host", "", "comma-separated extra host `names` the view may be opened by (e.g. a name in your hosts file); localhost, 127.0.0.1 and ::1 always work, and this computer's own IPs and host name too when ADDR is 0.0.0.0 or another all-interfaces address")
	pos, err := parse(fs, args, 1)
	if err != nil {
		return quiet(err)
	}
	addr := defaultAddr
	if len(pos) == 1 {
		if !addrRe.MatchString(pos[0]) {
			return fmt.Errorf("the address must look like :8485 or 127.0.0.1:8485: %s", pos[0])
		}
		addr = pos[0]
	}
	picked, load, err := c.loader()
	if err != nil {
		return err
	}
	return serveLive(addr, splitList(*allow), *interval, picked, load, c.keepFn(), !*noOpen)
}

func cmdHTML(args []string) error {
	fs := newFS("html", "html [flags]")
	c := addCommon(fs)
	out := fs.String("o", "kiroku.html", "output `file`")
	fs.StringVar(out, "out", "kiroku.html", "same as -o")
	noOpen := fs.Bool("no-open", false, "do not open a browser")
	week := fs.String("week", "", "write only this `week` (this, last, or any date in it such as 2026-10-05); default file kiroku-<monday>.html")
	month := fs.String("month", "", "write only this `month` (this, last, or 2026-09); default file kiroku-<month>.html")
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	p, err := parsePeriod(*week, *month, time.Now())
	if err != nil {
		return err
	}
	_, load, err := c.loader()
	if err != nil {
		return err
	}
	snap := load()
	if p == nil {
		return writeHTML(snap, *out, !*noOpen)
	}
	explicit := false
	fs.Visit(func(f *flag.Flag) { explicit = explicit || f.Name == "o" || f.Name == "out" })
	if !explicit {
		*out = p.file()
	}
	if len(snap.data) == 0 {
		return writeHTML(snap, *out, !*noOpen) // 履歴がまったくないときの案内は writeHTML が出す
	}
	first, last := snap.data[0].Start, snap.data[0].End
	for _, d := range snap.data {
		last = max(last, d.End)
	}
	snap = scoped(snap, p)
	if len(snap.data) == 0 {
		day := func(t float64) string { return time.Unix(int64(t), 0).Format("2006-01-02") }
		return fmt.Errorf("no history in %s (your history covers %s to %s)", p.label(), day(first), day(last))
	}
	if err := writeHTML(snap, *out, !*noOpen); err != nil {
		return err
	}
	fmt.Fprintf(logw, "it has only %s, but includes its prompts, the AI's replies, file paths and commit messages as they are; check it before sharing\n", p.label())
	return nil
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
		return snap, fmt.Errorf("no history found; check where your agents keep it (--root, --kiro-home, --codex-home, --amazonq-db; see \"kiroku html --help\")")
	}
	return snap, nil
}

func writeHTML(snap snapshot, out string, open bool) error {
	if len(snap.data) == 0 {
		return fmt.Errorf("no history found; check where your agents keep it (--root, --kiro-home, --codex-home, --amazonq-db; see \"kiroku html --help\")")
	}
	html, err := web.Render(snap.data, snap.weeks, snap.months, snap.meta, snap.gen, false)
	if err != nil {
		return err
	}
	if err := writePrivate(out, []byte(html)); err != nil {
		return err
	}
	fmt.Printf("%s → %s\n", plural(len(snap.data), "session"), out)
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
	return writePrivate(out, b)
}

// writePrivate は、履歴を含むファイルをほかのユーザーが読めないように（0600）書く。
// 同じフォルダの一時ファイルに書いてから名前を変えるので、途中で失敗しても前のファイルは残り、
// out にシンボリックリンクがあっても、その先は書きかえない（リンクを置きかえる）。
func writePrivate(out string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(out), ".kiroku-tmp-*") // CreateTemp は 0600 で作る
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // 置きかえたあとは何もしない
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil { // umask にかかわらず 0600
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), out)
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
		return fmt.Errorf("unknown command: %s (run \"kiroku help\" for the list)", fs.Arg(0))
	}
	explicitOut := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "o" || f.Name == "out" {
			explicitOut = true
		}
	})
	note := func(newForm string) {
		fmt.Fprintf(os.Stderr, "(this form is deprecated; use %s instead)\n", newForm)
	}
	switch {
	case *showVersion:
		return runVersion()
	case *weekly != "" || *monthly != "":
		return fmt.Errorf("the Markdown weekly/monthly summary has been removed; use \"kiroku serve\" instead")
	case *serve != "":
		note("kiroku serve")
		picked, load, err := c.loader()
		if err != nil {
			return err
		}
		return serveLive(*serve, nil, *interval, picked, load, c.keepFn(), !*noOpen)
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
