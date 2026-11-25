package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/MichinaoShimizu/kiroku/internal/archive"
	"github.com/MichinaoShimizu/kiroku/internal/core"
	"github.com/MichinaoShimizu/kiroku/internal/gitlog"
	"github.com/MichinaoShimizu/kiroku/internal/report"
	"github.com/MichinaoShimizu/kiroku/internal/source"
	"github.com/MichinaoShimizu/kiroku/internal/web"
)

const helpText = `kiroku %s - see your AI coding agent history as a calendar (Claude Code, Kiro, Kiro Crew, Codex)

Usage:
  kiroku <command> [flags] [args]

Commands:
  serve [ADDR]          Open the view in your browser and keep it updated as new history arrives (default 127.0.0.1:8484);
                        run it again while it runs to open the running view
  serve --check         Check what kiroku can read, whether any history will be deleted, and what to run next
  serve --archive [on|off]
                        Keep compressed copies of history that agents delete (Claude Code, Kiro Crew); no argument shows the status
  update                Update kiroku to the latest release
  help                  Show this help (--version prints only the version)

Run "kiroku serve --help", "kiroku serve --check --help", "kiroku serve --archive --help"
or "kiroku update --help" for the flags.
Get started: kiroku serve --check, then kiroku serve
`

// serveModes は、kiroku serve の最初の引数で選ぶ、ほかの仕事（5.0 より前はサブコマンドだったもの）。
var serveModes = map[string]func([]string) error{"--check": cmdDoctor, "--archive": cmdArchive}

// removedCmds は、5.0 で消したサブコマンドと、代わりに何を使うか。打つと、この 1 行を出して止まる（0 でない終了コード）。
// autostart だけは、4 で登録したログイン時の起動を外す（消したコマンドでは外せなくなるので）
var removedCmds = map[string]string{
	"open":      `run "kiroku serve" (running it again while it runs opens the running view in your browser)`,
	"html":      `use "kiroku serve"; kiroku no longer writes the view to a file`,
	"doctor":    `use "kiroku serve --check"`,
	"archive":   `use "kiroku serve --archive"`,
	"version":   `use "kiroku --version"`,
	"autostart": `run "kiroku serve" when you want the view`,
}

// WriteHTML は、画面を 1 つの HTML ファイルに書く（引数は 4 までの kiroku html と同じ）。利用者のコマンドではなく、
// e2e・スクリーンショット・GitHub Pages のデモを作る tools/html から使う
func WriteHTML(args []string) error { return cmdHTML(args) }

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
	if hint, ok := removedCmds[args[0]]; ok {
		if args[0] == "autostart" {
			removeAutostart()
		}
		return fmt.Errorf("\"kiroku %s\" was removed in kiroku 5.0; %s", args[0], hint)
	}
	switch args[0] {
	case "serve":
		if len(args) > 1 {
			if f, ok := serveModes[args[1]]; ok {
				return f(args[2:])
			}
		}
		return cmdServe(args[1:])
	case "json", "stats":
		return fmt.Errorf("\"kiroku %s\" was removed in kiroku 3.0; use \"kiroku serve\" to see your history", args[0])
	case "--version", "-version", "-v":
		return runVersion()
	case "update":
		return runUpdate(args[1:])
	case "help", "--help", "-help", "-h":
		printHelp(os.Stdout)
		return nil
	}
	if strings.HasPrefix(args[0], "-") {
		return legacyForm(args[0])
	}
	return fmt.Errorf("unknown command: %s (run \"kiroku help\" for the list)", args[0])
}

// common は、履歴を読むコマンドに共通のオプション。
type common struct {
	claudeRoot, kiroHome, crewHome, kiroCLIDB, amazonQDB, codexHome, sources, prices, archiveDir *string
	gap                                                                                          *int
	fs                                                                                           *flag.FlagSet // 何を明示したかを見る（--kiro-home）
}

func addCommon(fs *flag.FlagSet) *common {
	return &common{
		fs:         fs,
		claudeRoot: fs.String("claude-root", source.DefaultClaudeRoot(), "Claude Code history directory ($CLAUDE_CONFIG_DIR/projects or ~/.claude/projects)"),
		kiroHome:   fs.String("kiro-home", source.DefaultKiroHome(), "Kiro data directory ($KIRO_HOME)"),
		crewHome:   fs.String("crew-home", "", "Kiro Crew data directory (default $KIROCREW_HOME or ~/.kiro/crew; not moved by --kiro-home or $KIRO_HOME)"),
		kiroCLIDB:  fs.String("kiro-cli-db", "", "path to data.sqlite3 of older Kiro CLI versions, shown as Kiro CLI (SQLite) (default: OS-specific)"),
		amazonQDB:  fs.String("amazonq-db", "", "does nothing: kiroku no longer reads Amazon Q history (kept so existing scripts keep working)"),
		codexHome:  fs.String("codex-home", "", "Codex data directory (default $CODEX_HOME or ~/.codex)"),
		sources:    fs.String("sources", strings.Join(core.Families(), ","), "comma-separated sources to read: "+core.FamiliesHelp()),
		prices:     fs.String("prices", "", "JSON file overriding the model price table"),
		gap:        fs.Int("gap", 15, "idle `minutes` that split a session into separate blocks"),
		archiveDir: fs.String("archive-dir", archive.DefaultDir(), "where \"kiroku serve --archive\" keeps copies of deleted history ($KIROKU_ARCHIVE_DIR)"),
	}
}

// Amazon Q の読み取りはやめた。--amazonq-db と --sources amazonq は、互換のために受け付けて何もしない（1 回だけ知らせる。終了コードは変えない）
const droppedAmazonQ = "kiroku: Amazon Q support was dropped; --amazonq-db and --sources amazonq do nothing (Kiro CLI history is still read)"

var (
	droppedOnce sync.Once
	droppedW    io.Writer = os.Stderr // テストで差しかえる
)

// picked は選んだ履歴。all は選んでいないものもふくめた全部（collect に渡す。選んでいない Kiro Crew からも、
// 中身を出してはいけない会話の、Codex・Claude Code に残る写しを調べて隠すため。source.ClaimScanner）。
func (c *common) picked() (picked []source.Source, want map[string]bool, all []source.Source) {
	want = map[string]bool{}
	for _, s := range strings.Split(*c.sources, ",") {
		want[strings.ToLower(strings.TrimSpace(s))] = true
	}
	if *c.amazonQDB != "" || want["amazonq"] {
		droppedOnce.Do(func() { fmt.Fprintln(droppedW, droppedAmazonQ) })
	}
	// KIRO_HOME の場所を読むときは、~/.kiro に残る V2・Kiro IDE・KIRO_HOME を設定する前の会話も読む。
	// --kiro-home を付けたときは（$KIRO_HOME と同じ場所でも）、その場所だけ
	explicit := false
	if c.fs != nil {
		c.fs.Visit(func(f *flag.Flag) { explicit = explicit || f.Name == "kiro-home" })
	}
	var legacy string
	if !explicit && *c.kiroHome == source.DefaultKiroHome() {
		legacy = source.DefaultKiroLegacyHome()
	}
	all = source.All(source.Options{ClaudeRoot: *c.claudeRoot, KiroHome: *c.kiroHome, KiroLegacy: legacy, KiroCLIDB: *c.kiroCLIDB,
		CrewHome: *c.crewHome, CodexHome: *c.codexHome, Archive: *c.archiveDir})
	for _, s := range all {
		if want[s.Family()] {
			picked = append(picked, s)
		}
	}
	return picked, want, all
}

// keepFn は、kiroku serve の画面から kiroku archive をオンにする関数（コピーは次の読み直しで残す）。
func (c *common) keepFn() func() error {
	dir := *c.archiveDir
	return func() error {
		if dir == "" {
			return errNoArchiveDir
		}
		return archive.Enable(dir)
	}
}

// loader は、選んだ履歴を読んで集計する関数を作る。kiroku archive がオンなら、読む前に消える履歴のコピーを残す。
func (c *common) loader() ([]source.Source, func() snapshot, error) {
	if *c.prices != "" {
		if err := loadPrices(*c.prices); err != nil {
			return nil, nil, fmt.Errorf("could not read the price table: %w", err)
		}
	}
	picked, want, all := c.picked()
	dir := *c.archiveDir
	gap := *c.gap
	cache := newLoadCache()     // kiroku serve の読み直しで、変わっていない履歴を読み直さない
	gcache := gitlog.NewCache() // git も、変わっていないリポジトリは読み直さない
	rcache := report.NewCache() // 週と月も、セッションとコミットが変わっていない期間は集計し直さない
	load := func() snapshot {
		on := archiveOn(dir)
		if on {
			keepCopies(picked, dir)
		}
		data, rep := collectCached(all, want, gap, cache)
		if data == nil {
			data = []*core.Session{} // 画面では null ではなく空の一覧として扱う
		}
		tracker.setStage("git")
		t0 := time.Now()
		commits, pushes, stale := gcache.Collect(data) // git がなければ空
		if len(commits) > 0 || len(stale) > 0 {
			fmt.Fprintf(logw, "  git: %s %s\n", plural(len(commits), "commit"), styleFor(logw).dim("in "+took(time.Since(t0)).String()))
		}
		if commits == nil {
			commits = []gitlog.Commit{}
		}
		if pushes == nil {
			pushes = []gitlog.Push{}
		}
		for _, r := range stale {
			fmt.Fprint(logw, gitTimeoutLine(r))
		}
		tracker.setStage("view")
		files, size := archiveUsage(dir)
		meta := map[string]any{"report": rep, "git": commits, "push": pushes, "prices": map[string]any{"asOf": core.PricesAsOf, "custom": *c.prices != ""},
			"archive": map[string]any{"on": on, "dir": dir, "files": files, "bytes": size}, "version": version,
			"repo": projectURL(), "os": runtime.GOOS + "/" + runtime.GOARCH} // os は「Report a problem」と「Copy diagnostics」に入れる（履歴ではない）
		if len(stale) > 0 { // 時間切れで読み終わらなかったリポジトリ（前回の結果を使った）
			meta["gitTimeout"] = stale
		}
		if os.Getenv("KIROKU_DEMO") != "" { // デモ（ダミーデータ）：画面は、この時間帯の時計で見せる
			_, off := time.Now().Zone()
			meta["demo"] = map[string]any{"offset": off}
		}
		return snapshot{data: data, weeks: rcache.AllWeeks(data, commits...), months: rcache.AllMonths(data, commits...), meta: meta, rep: rep, gen: float64(time.Now().UnixNano()) / 1e9}
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
	for _, a := range args { // v0.20 で名前を変えた（flag のエラーと使い方の一覧より先に、新しい名前を教える）
		if a == "--" {
			break
		}
		if name, _, _ := strings.Cut(a, "="); name == "-root" || name == "--root" {
			return nil, fmt.Errorf("--root has been renamed to --claude-root")
		}
	}
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
		if n > 0 { // オプションがないコマンドでは、空の「Flags:」を出さない
			fmt.Fprint(fs.Output(), "\nFlags:\n")
			fs.PrintDefaults()
		}
	}
	return fs
}

// langFlag は画面の言語。見る人は画面で切り替えられる。既定は $KIROKU_LANG、なければブラウザの言語（日本語でなければ英語）。
// $KIROKU_LANG の値が使えないときは、止めずに注意だけ出して、ブラウザの言語に任せる（autostart の serve が起動のたびに落ちないように）。
func langFlag(fs *flag.FlagSet) *string {
	env := os.Getenv("KIROKU_LANG")
	if env != "" && !web.ValidLang(env) {
		fmt.Fprintf(logw, "ignoring KIROKU_LANG=%q: use one of %s\n", env, strings.Join(web.Langs, ", "))
		env = ""
	}
	return fs.String("lang", env, "language of the view: en or ja ($KIROKU_LANG; default: the browser's language, English if it isn't Japanese); ?lang=en or ?lang=ja in the address also picks it")
}

// aiRemovedNote は、4.0 で外した「Analyze with AI」の --ai（$KIROKU_AI）を付けて起動したときに出す 1 行（なければ ""）。
// 止めずに続ける（kiroku 4 の autostart で登録した起動が、古い値のまま動き続けられるように）。kiroku は AI のエージェントを動かさない
func aiRemovedNote(ai bool) string {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("KIROKU_AI")))
	if !ai && env != "1" && env != "true" && env != "yes" && env != "on" {
		return ""
	}
	return "--ai ($KIROKU_AI) was removed in 4.0: kiroku no longer runs AI agents. \"Copy prompt\" in the view copies the review and report prompts to paste into your agent"
}

func checkLang(lang string) error {
	if lang != "" && !web.ValidLang(lang) {
		return fmt.Errorf("--lang must be one of %s: %s", strings.Join(web.Langs, ", "), lang)
	}
	return nil
}

// quiet は --help のときにエラーにしない。
func quiet(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

func cmdServe(args []string) error {
	fs := newFS("serve", "serve [flags] [ADDR]\n\nADDR defaults to "+defaultAddr+". A bare port such as :8485 also listens on 127.0.0.1 only.\nTo let other devices open it, write the address explicitly (e.g. 0.0.0.0:8485); anyone on your network can then see your history.\nIf kiroku serve already runs at ADDR, this opens the running view in your browser instead.\n\nAs its first flag, serve also takes --check or --archive (run \"kiroku serve --check --help\" for theirs).")
	c := addCommon(fs)
	interval := fs.Duration("interval", 5*time.Second, "how often to check the history for changes (reloads after writes settle)")
	noOpen := fs.Bool("no-open", false, "do not open a browser")
	allow := fs.String("allow-host", "", "comma-separated extra host `names` the view may be opened by (e.g. a name in your hosts file); localhost, 127.0.0.1 and ::1 always work, and this computer's own IPs and host name too when ADDR is 0.0.0.0 or another all-interfaces address")
	lang := langFlag(fs)
	ai := fs.Bool("ai", false, "removed in 4.0 (kiroku no longer runs AI agents); accepted and ignored")
	_ = fs.Bool("no-ai", false, "removed in 4.0 (kiroku no longer runs AI agents); accepted and ignored")
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
	if err := checkLang(*lang); err != nil {
		return err
	}
	_, want, all := c.picked()
	if n := aiRemovedNote(*ai); n != "" {
		fmt.Fprintln(os.Stderr, n)
	}
	return serveLiveWith(addr, splitList(*allow), *interval, picked, load, c.keepFn(), !*noOpen, *lang, scanWatchPaths(all, want))
}

func cmdHTML(args []string) error {
	fs := newFS("html", "html [flags]   (go run ./tools/html; not part of the released kiroku)")
	c := addCommon(fs)
	out := fs.String("o", "kiroku.html", "output `file`")
	fs.StringVar(out, "out", "kiroku.html", "same as -o")
	noOpen := fs.Bool("no-open", false, "do not open a browser")
	week := fs.String("week", "", "write only this `week` (this, last, or any date in it such as 2026-10-05); default file kiroku-<monday>.html")
	month := fs.String("month", "", "write only this `month` (this, last, or 2026-09); default file kiroku-<month>.html")
	lang := langFlag(fs)
	if _, err := parse(fs, args, 0); err != nil {
		return quiet(err)
	}
	if err := checkLang(*lang); err != nil {
		return err
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
		return writeHTML(snap, *out, !*noOpen, *lang)
	}
	explicit := false
	fs.Visit(func(f *flag.Flag) { explicit = explicit || f.Name == "o" || f.Name == "out" })
	if !explicit {
		*out = p.file()
	}
	if len(snap.data) == 0 {
		return writeHTML(snap, *out, !*noOpen, *lang) // 履歴がまったくないときの案内は writeHTML が出す
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
	if err := writeHTML(snap, *out, !*noOpen, *lang); err != nil {
		return err
	}
	fmt.Fprintf(logw, "%s has only %s, but includes its prompts, the AI's replies, file paths and commit messages as they are; check it before sharing\n", *out, p.label())
	return nil
}

func writeHTML(snap snapshot, out string, open bool, lang string) error {
	if len(snap.data) == 0 {
		return fmt.Errorf("no history found; check where your agents keep it (--claude-root, --kiro-home, --crew-home, --codex-home, --kiro-cli-db; see \"kiroku serve --check --all\")")
	}
	parts, err := web.Marshal(snap.data, snap.weeks, snap.months, snap.meta, snap.gen)
	if err != nil {
		return err
	}
	parts.Lang = lang
	html, err := parts.HTML(false)
	if err != nil {
		return err
	}
	if err := writePrivate(out, []byte(html)); err != nil {
		return err
	}
	fmt.Printf("%s → %s\n", plural(len(snap.data), "session"), out)
	if open {
		if abs, err := filepath.Abs(out); err == nil {
			if err := openBrowser(abs); err != nil {
				fmt.Fprintf(os.Stderr, "could not open a browser: %v\n", err)
			}
		}
	}
	return nil
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

// legacyForm は、v1.0 の前に消した前の書き方（kiroku --serve、--json、-o など）に、いまの書き方を教える。
func legacyForm(arg string) error {
	name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
	now := map[string]string{"serve": "kiroku serve", "json": "kiroku serve", "o": "kiroku serve", "out": "kiroku serve",
		"weekly": "kiroku serve", "monthly": "kiroku serve"}[name]
	if now == "" {
		return fmt.Errorf("unknown command: %s (run \"kiroku help\" for the list)", arg)
	}
	return fmt.Errorf("\"kiroku %s\" has been removed; use \"%s\" instead", arg, now)
}

// gitTimeoutLine は、git の読み取りが時間切れになったリポジトリを言う 1 行。フォルダ名は履歴の cwd から来るので、端末の制御文字を除く
func gitTimeoutLine(repo string) string {
	return "  git: timed out reading " + termMsg(repo) + " (will try again on the next reload)\n"
}
