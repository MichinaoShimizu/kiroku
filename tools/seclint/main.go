// seclint は、CLAUDE.md の「Security」の決まりのうち、機械的に見つけられる後戻りを探す（tools/check.sh と go test で動く）。
//
//	go run ./tools/seclint [リポジトリのルート]
//
// 調べること（どれも安く、毎回同じ結果になるもの。見逃しがないことは保証しない。レビューと e2e の代わりではない）:
//
//   - js-html: internal/web/js で innerHTML・outerHTML への代入、insertAdjacentHTML、document.write。
//     空にする代入（= ""）は数えない。決まった値だけを入れるものは allow に、
//     いまある esc() を通したテンプレートは baseline に、ファイルごとの数で載せる。数が増えたら失敗する
//   - shell-exec: Go の exec.Command / exec.CommandContext で、シェル（sh・bash など）を動かすか "-c" を渡すもの
//   - file-mode: テスト以外の Go で、os.WriteFile・os.OpenFile が 0600 より、os.Mkdir・os.MkdirAll が 0700 より広い権限で作るもの
//     （権限が定数でないものも）
//   - listen: テスト以外の Go で、http.ListenAndServe・net.Listen などが 127.0.0.1・::1・localhost 以外で待ち受けうるもの
//     （アドレスが定数でないものも）。kiroku serve の決まった道（SECURITY.md に書いたもの）だけを allow に載せる
//   - unpinned-action: .github/workflows の uses: が 40 桁の commit SHA で固定されていないもの（./ で始まる同じリポジトリのものは除く）
//
// allow と baseline の各項目は、ファイルと理由を書く。使われなくなった項目も失敗にする（一覧が古くならないように）。
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// exception は、決まりに当たるが理由があって認めるもの
type exception struct {
	Rule string
	File string // リポジトリのルートからのパス（/ 区切り）
	// Func は Go の関数名（file-mode・listen）。Match は js-html のその行に含まれる文字列。Count は baseline の数
	Func   string
	Match  string
	Count  int
	Reason string
}

// allow は、安全だとわかっていて認めるもの
var allow = []exception{
	{Rule: "js-html", File: "internal/web/js/format.js", Match: `$("#theme").innerHTML = ICON[next]`,
		Reason: "ICON is a constant table of SVG icons in the same file; nothing from history, git or the network"},
	{Rule: "file-mode", File: "internal/cli/autostart.go", Func: "cmdAutostart",
		Reason: "the LaunchAgent plist or systemd user unit (0644, in a 0755 folder) holds only the kiroku path and serve options, no history; launchd and systemd read it as usual"},
	{Rule: "file-mode", File: "tools/prices/main.go", Func: "run",
		Reason: "developer tool: creates docs/upstream (0755), a folder of the repository with public price tables"},
	{Rule: "file-mode", File: "tools/prices/main.go", Func: "write",
		Reason: "developer tool: writes public price tables to docs/upstream/*.md (0644), committed to the repository"},
	{Rule: "file-mode", File: "tools/upstream/main.go", Func: "main",
		Reason: "developer tool: writes public agent versions to docs/upstream/agents.md (0644), committed to the repository"},
	{Rule: "listen", File: "internal/cli/serve.go", Func: "serveLiveWithAI",
		Reason: "kiroku serve: 127.0.0.1 by default; another ADDR is opt-in, keeps the Host check and the key, and is documented in SECURITY.md"},
}

// baseline は、いまある esc() を通したテンプレートの HTML（決まった値ではない）。XSS の e2e（tools/screenshots/hostile.py・xss.mjs）が
// 履歴の文字列が HTML として動かないことを確かめている。数が増えたら失敗し、減ったら数を下げるよう知らせる（失敗にはしない。新しい箇所は textContent か
// 決まった値で書く。どうしても要るなら、レビューを経てここを上げる）
var baseline = []exception{
	{Rule: "js-html", File: "internal/web/js/boot.js", Count: 1, Reason: "the scope note, built from t() and esc()"},
	{Rule: "js-html", File: "internal/web/js/calendar.js", Count: 15, Reason: "period heading, legend, KPIs, week calendar, tooltips, Journey and Project views; history strings go through esc()"},
	{Rule: "js-html", File: "internal/web/js/git.js", Count: 3, Reason: "commit, PR and push panels; git strings go through esc()"},
	{Rule: "js-html", File: "internal/web/js/guide.js", Count: 1, Reason: "the metrics guide dialog, built from HELP and esc()"},
	{Rule: "js-html", File: "internal/web/js/metric.js", Count: 1, Reason: "the metric breakdown dialog; history strings go through esc()"},
	{Rule: "js-html", File: "internal/web/js/panels.js", Count: 1, Reason: "search results; history strings go through esc() and the highlighter"},
	{Rule: "js-html", File: "internal/web/js/review.js", Count: 3, Reason: "the Worth a look dialog and its markers, built from t() and esc()"},
	{Rule: "js-html", File: "internal/web/js/session.js", Count: 1, Reason: "the session panel; history strings go through esc() and md()"},
	{Rule: "js-html", File: "internal/web/js/summary.js", Count: 5, Reason: "the period summary, report bar, Worth a look and metric hints; history strings go through esc()"},
}

type finding struct {
	Rule, File string
	Line       int
	Func, Text string
	Msg        string
}

func main() {
	root := "."
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/seclint [repository root]")
		os.Exit(2)
	}
	if len(os.Args) == 2 {
		root = os.Args[1]
	}
	os.Exit(run(root, allow, baseline, os.Stdout))
}

// run は root を調べ、問題を w に書く。問題があれば 1、読めなければ 2 を返す
func run(root string, allow, baseline []exception, w io.Writer) int {
	found, err := scan(root)
	if err != nil {
		fmt.Fprintf(w, "seclint: %v\n", err)
		return 2
	}
	problems, notes := apply(found, allow, baseline)
	for _, n := range notes {
		fmt.Fprintln(w, "note: "+n)
	}
	for _, p := range problems {
		fmt.Fprintln(w, p)
	}
	if len(problems) > 0 {
		fmt.Fprintf(w, "seclint: %d problem(s). Fix them, or add an entry with the file and a reason to allow in tools/seclint/main.go (see CLAUDE.md, Security)\n", len(problems))
		return 1
	}
	fmt.Fprintf(w, "seclint: ok (%d allowed, %d files with a baseline)\n", len(allow), len(baseline))
	return 0
}

// apply は allow と baseline を当て、残った問題と使われなかった allow の項目（problems）と、
// baseline より減ったファイル（notes。失敗にはしない。HTML を減らした PR や、ほかの PR を止めないように）を返す
func apply(found []finding, allow, baseline []exception) (problems, notes []string) {
	usedAllow := make([]bool, len(allow))
	perFile := map[string][]finding{} // baseline と比べる js-html
	var out []string
	for _, f := range found {
		ok := false
		for i, a := range allow {
			if a.Rule != f.Rule || a.File != f.File {
				continue
			}
			if (a.Func != "" && a.Func == f.Func) || (a.Match != "" && strings.Contains(f.Text, a.Match)) {
				usedAllow[i] = true
				ok = true
				break
			}
		}
		if ok {
			continue
		}
		if f.Rule == "js-html" {
			perFile[f.File] = append(perFile[f.File], f)
			continue
		}
		where := ""
		if f.Func != "" {
			where = " (in " + f.Func + ")"
		}
		out = append(out, fmt.Sprintf("%s:%d: %s%s: %s", f.File, f.Line, f.Rule, where, f.Msg))
	}
	seen := map[string]bool{}
	for _, b := range baseline {
		seen[b.File] = true
		got := perFile[b.File]
		switch {
		case len(got) > b.Count:
			for _, f := range got {
				out = append(out, fmt.Sprintf("%s:%d: %s: %s", f.File, f.Line, f.Rule, f.Msg))
			}
			out = append(out, fmt.Sprintf("%s: js-html: %d HTML insertions, the baseline allows %d; use textContent or DOM nodes for the new one", b.File, len(got), b.Count))
		case len(got) < b.Count:
			notes = append(notes, fmt.Sprintf("%s: js-html: %d HTML insertions, fewer than the baseline of %d; lower Count in tools/seclint/main.go", b.File, len(got), b.Count))
		}
	}
	for file, got := range perFile {
		if seen[file] {
			continue
		}
		for _, f := range got {
			out = append(out, fmt.Sprintf("%s:%d: %s: %s", f.File, f.Line, f.Rule, f.Msg))
		}
	}
	for i, a := range allow {
		if !usedAllow[i] {
			out = append(out, fmt.Sprintf("%s: %s: the allow entry (%s%s) matches nothing; remove it", a.File, a.Rule, a.Func, a.Match))
		}
	}
	sort.Strings(out)
	sort.Strings(notes)
	return out, notes
}

// scan はリポジトリを歩いて、決まりに当たるところを集める
func scan(root string) ([]finding, error) {
	var out []finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "testdata", "dist", "worktrees":
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		var found []finding
		switch {
		case strings.HasSuffix(rel, ".go"):
			found, err = scanGo(path, rel)
		case strings.HasPrefix(rel, "internal/web/js/") && strings.HasSuffix(rel, ".js"):
			found, err = scanLines(path, rel, scanJS)
		case strings.HasPrefix(rel, ".github/workflows/") && (strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".yaml")):
			found, err = scanLines(path, rel, scanWorkflow)
		}
		out = append(out, found...)
		return err
	})
	return out, err
}

func scanLines(path, rel string, check func(rel string, n int, line string) []finding) ([]finding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []finding
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for n := 1; sc.Scan(); n++ {
		out = append(out, check(rel, n, sc.Text())...)
	}
	return out, sc.Err()
}

var (
	jsSink  = regexp.MustCompile(`\b(innerHTML|outerHTML)\s*\+?=($|[^=])|\binsertAdjacentHTML\s*\(|\bdocument\.write(ln)?\s*\(`)
	jsClear = regexp.MustCompile(`^\b(innerHTML|outerHTML)\s*=\s*(""|''|` + "``" + `)\s*($|[;),])`)
)

func scanJS(rel string, n int, line string) []finding {
	var out []finding
	for _, m := range jsSink.FindAllStringIndex(line, -1) {
		if jsClear.MatchString(line[m[0]:]) {
			continue // 空にするだけ
		}
		sink := strings.TrimRight(strings.Fields(line[m[0]:m[1]])[0], "=(+")
		out = append(out, finding{Rule: "js-html", File: rel, Line: n, Text: line,
			Msg: sink + " inserts HTML; build the DOM with textContent instead (history, git and network strings must never be parsed as HTML)"})
	}
	return out
}

var (
	usesRe   = regexp.MustCompile(`^\s*(?:-\s+)?uses:\s*["']?([^"'\s#]+)`)
	pinnedRe = regexp.MustCompile(`^[^@\s]+@[0-9a-f]{40}$|^docker://[^@\s]+@sha256:[0-9a-f]{64}$`)
)

func scanWorkflow(rel string, n int, line string) []finding {
	m := usesRe.FindStringSubmatch(line)
	if m == nil || strings.HasPrefix(m[1], "./") || pinnedRe.MatchString(m[1]) {
		return nil
	}
	return []finding{{Rule: "unpinned-action", File: rel, Line: n, Text: line,
		Msg: fmt.Sprintf("%q is not pinned to a full 40-character commit SHA (write uses: owner/repo@<sha> # vX.Y.Z)", m[1])}}
}

var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "fish": true, "cmd": true, "cmd.exe": true, "powershell": true, "powershell.exe": true, "pwsh": true}

func scanGo(path, rel string) ([]finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	test := strings.HasSuffix(rel, "_test.go")
	var out []finding
	add := func(rule string, n ast.Node, fn, msg string) {
		out = append(out, finding{Rule: rule, File: rel, Line: fset.Position(n.Pos()).Line, Func: fn, Msg: msg})
	}
	visit := func(fn string, body ast.Node) {
		ast.Inspect(body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			pkg, name := callee(call)
			switch {
			case pkg == "exec" && (name == "Command" || name == "CommandContext"):
				args := call.Args
				if name == "CommandContext" && len(args) > 0 {
					args = args[1:]
				}
				for i, a := range args {
					s, ok := strLit(a)
					if !ok {
						continue
					}
					if (i == 0 && shells[strings.ToLower(filepath.Base(strings.ReplaceAll(s, `\`, "/")))]) || s == "-c" || s == "/c" || s == "/C" {
						add("shell-exec", call, fn, fmt.Sprintf("exec.%s runs a shell (%q); pass the program and its arguments directly, never through a shell", name, s))
						break
					}
				}
			case !test && pkg == "os" && (name == "WriteFile" || name == "OpenFile" || name == "Mkdir" || name == "MkdirAll"):
				idx, max, kind := 2, int64(0o600), "file"
				if strings.HasPrefix(name, "Mkdir") {
					idx, max, kind = 1, 0o700, "directory"
				}
				if idx >= len(call.Args) {
					return true
				}
				mode, ok := intLit(call.Args[idx])
				switch {
				case !ok:
					add("file-mode", call, fn, fmt.Sprintf("os.%s with a mode that is not a constant; history files must be 0600 and folders 0700", name))
				case mode&^max != 0:
					add("file-mode", call, fn, fmt.Sprintf("os.%s makes a %s with mode %#o, wider than %#o; history files must be 0600 and folders 0700", name, kind, mode, max))
				}
			case !test && isListen(pkg, name):
				idx := 0
				if pkg == "net" {
					idx = 1
				}
				if pkg == "" || idx >= len(call.Args) {
					add("listen", call, fn, fmt.Sprintf("%s listens on an address that can't be checked here; kiroku listens only on 127.0.0.1 unless the user opts in (SECURITY.md)", name))
					return true
				}
				addr, ok := strLit(call.Args[idx])
				if !ok || !loopback(addr) {
					add("listen", call, fn, fmt.Sprintf("%s.%s may listen on a non-loopback address; kiroku listens only on 127.0.0.1 unless the user opts in (SECURITY.md)", pkg, name))
				}
			}
			return true
		})
	}
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
			visit(fd.Name.Name, fd.Body)
		} else {
			visit("", d)
		}
	}
	return out, nil
}

// callee は pkg.Name(...) の pkg と Name を返す。メソッド呼び出し（x.ListenAndServe()）は pkg が ""
func callee(call *ast.CallExpr) (pkg, name string) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", ""
	}
	if id, ok := sel.X.(*ast.Ident); ok && (id.Name == "os" || id.Name == "exec" || id.Name == "http" || id.Name == "net") {
		return id.Name, sel.Sel.Name
	}
	return "", sel.Sel.Name
}

func isListen(pkg, name string) bool {
	switch pkg {
	case "http":
		return name == "ListenAndServe" || name == "ListenAndServeTLS"
	case "net":
		return name == "Listen" || name == "ListenTCP" || name == "ListenPacket" || name == "ListenUDP"
	case "":
		return name == "ListenAndServe" || name == "ListenAndServeTLS"
	}
	return false
}

func strLit(e ast.Expr) (string, bool) {
	b, ok := e.(*ast.BasicLit)
	if !ok || b.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(b.Value)
	return s, err == nil
}

// intLit は 0o600 や os.FileMode(0o600)、fs.FileMode(0600) の値を返す
func intLit(e ast.Expr) (int64, bool) {
	if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 {
		if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "FileMode" {
			e = c.Args[0]
		}
	}
	b, ok := e.(*ast.BasicLit)
	if !ok || b.Kind != token.INT {
		return 0, false
	}
	v, err := strconv.ParseInt(b.Value, 0, 64)
	return v, err == nil
}

func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
