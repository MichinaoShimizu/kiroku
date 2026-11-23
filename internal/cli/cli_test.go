package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// サブコマンドがなければヘルプだけ（履歴は読まない）。
func TestHelpWithoutSubcommand(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"-h"}, {"--help"}} {
		if err := dispatch(args); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
	if err := dispatch([]string{"serv"}); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("打ちまちがい: %v", err)
	}
}

// 位置引数はオプションの前にも後ろにも置ける。多すぎればエラー。
func TestParseInterspersed(t *testing.T) {
	for in, want := range map[string]string{
		"2026-09 --md-dir x":          "[2026-09] x",
		"--md-dir x 2026-09":          "[2026-09] x",
		"--md-dir x":                  "[] x",
		"--gap 20 2026-09 --md-dir x": "[2026-09] x",
	} {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		dir := fs.String("md-dir", ".", "")
		fs.Int("gap", 15, "")
		pos, err := parse(fs, strings.Fields(in), 1)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprint(pos) + " " + *dir; got != want {
			t.Errorf("%q → %s, want %s", in, got, want)
		}
	}
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	if _, err := parse(fs, []string{"a", "b"}, 1); err == nil {
		t.Error("位置引数が多すぎるのにエラーにならない")
	}
}

// 前の書き方（kiroku --serve、--json、-o など）は v1.0 の前に消した。履歴は読まず、いまの書き方を教える。
func TestLegacyFormsRemoved(t *testing.T) {
	for in, want := range map[string]string{
		"--serve":           `use "kiroku serve"`,
		"--serve=:9000":     `use "kiroku serve"`,
		"-serve":            `use "kiroku serve"`,
		"--json x.json":     `use "kiroku serve --html -o FILE"`,
		"-o x.html":         `use "kiroku serve --html -o FILE"`,
		"--out=x.html":      `use "kiroku serve --html -o FILE"`,
		"--weekly":          `use "kiroku serve"`,
		"--monthly 2026-09": `use "kiroku serve"`,
		"--sources kiro":    "unknown command: --sources",
		"--no-open --serve": "unknown command: --no-open",
	} {
		err := dispatch(strings.Fields(in))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", in, err, want)
		}
	}
}

// --root は --claude-root になった。前の名前では、新しい名前を教える。
func TestRootRenamed(t *testing.T) {
	for _, args := range [][]string{{"html", "--root", "x"}, {"doctor", "--root=x"}, {"serve", "-root", "x"}} {
		var err error
		captureOutput(t, func() { err = dispatch(args) })
		if err == nil || err.Error() != "--root has been renamed to --claude-root" {
			t.Errorf("%v: %v", args, err)
		}
	}
}

// --ai（$KIROKU_AI）は 4.0 で外した。付けて起動しても止めず、外したことと代わりの「Copy prompt」を 1 行で伝える。付けなければ何も言わない
func TestAIRemovedNote(t *testing.T) {
	t.Setenv("KIROKU_AI", "")
	if n := aiRemovedNote(false); n != "" {
		t.Errorf("without --ai: %q", n)
	}
	if n := aiRemovedNote(true); !strings.Contains(n, "removed") || !strings.Contains(n, "Copy prompt") {
		t.Errorf("with --ai: %q", n)
	}
	t.Setenv("KIROKU_AI", "1")
	if n := aiRemovedNote(false); n == "" {
		t.Error("KIROKU_AI=1 gave no note")
	}
}

// --ai と --no-ai は 4.0 で外したが、kiroku serve --autostart on で登録した起動などのために受け付ける（約束: docs/compatibility.md）
func TestServeAcceptsRemovedAIFlags(t *testing.T) {
	for _, f := range []string{"--ai", "--no-ai"} {
		if err := cmdServe([]string{f, "--help"}); err != nil {
			t.Errorf("kiroku serve %s: %v", f, err)
		}
	}
}

// 4.7 で、serve・update・help のほかのサブコマンドは serve の最初の引数（--check・--open・--archive・--autostart・--html）に移した。
// 古いサブコマンドは kiroku 5 まで同じように動き、使うたびに代わりを 1 行で知らせる。新しい形では知らせない。
func TestServeModesAndDeprecatedCommands(t *testing.T) {
	stderr := func(f func()) string {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		old := os.Stderr
		os.Stderr = w
		f()
		os.Stderr = old
		w.Close()
		b, _ := io.ReadAll(r)
		return string(b)
	}
	for _, c := range []struct {
		old, now string
	}{{"archive", "serve --archive"}, {"autostart", "serve --autostart"}} {
		var errOld, errNow error
		note := stderr(func() { errOld = dispatch([]string{c.old, "bogus"}) })
		quietNow := stderr(func() { errNow = dispatch(append(strings.Fields(c.now), "bogus")) })
		if errOld == nil || errNow == nil || errOld.Error() != errNow.Error() || !strings.Contains(errNow.Error(), "kiroku "+c.now) {
			t.Errorf("%s: old %v, new %v; want the same error naming \"kiroku %s\"", c.old, errOld, errNow, c.now)
		}
		if !strings.Contains(note, `"kiroku `+c.old+`" is deprecated`) || !strings.Contains(note, `use "kiroku `+c.now+`"`) {
			t.Errorf("%s: no deprecation note on stderr: %q", c.old, note)
		}
		if strings.Contains(quietNow, "deprecated") {
			t.Errorf("%s: the new form %q should not print a deprecation note: %q", c.old, c.now, quietNow)
		}
	}
	for name, now := range deprecatedCmds {
		if _, ok := serveModes[strings.TrimPrefix(now, "serve ")]; !ok && now != "--version" {
			t.Errorf("%s: its replacement %q is not a serve mode", name, now)
		}
		if !strings.Contains(helpText, "kiroku "+now) && !strings.Contains(helpText, now) {
			t.Errorf("help does not mention %q", now)
		}
	}
	if strings.Contains(helpText, "\n  doctor ") || strings.Contains(helpText, "\n  open ") {
		t.Errorf("help still lists the deprecated commands as commands:\n%s", helpText)
	}
}
