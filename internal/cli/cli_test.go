package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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

// json サブコマンドで集計を書き出せる（-o はオプションの前後どちらでも）。weekly / monthly はもうない。
func TestJSONSubcommand(t *testing.T) {
	setup(t) // タイムゾーンと古い Kiro IDE の更新時刻をそろえる
	h := filepath.Join("testdata", "home")
	out := filepath.Join(t.TempDir(), "k.json")
	args := []string{"json", "--claude-root", filepath.Join(h, ".claude", "projects"), "--kiro-home", filepath.Join(h, ".kiro"), "--sources", "claude,kiro", "-o", out}
	if err := dispatch(args); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(b), `"months"`) || !strings.Contains(string(b), `"weeks"`) {
		t.Fatalf("JSON が書き出されていない: %v", err)
	}
	for _, sub := range []string{"weekly", "monthly"} {
		if err := dispatch([]string{sub}); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Errorf("%s: %v", sub, err)
		}
	}
}

// 前の書き方（kiroku --serve、--json、-o など）は v1.0 の前に消した。履歴は読まず、いまの書き方を教える。
func TestLegacyFormsRemoved(t *testing.T) {
	for in, want := range map[string]string{
		"--serve":           `use "kiroku serve"`,
		"--serve=:9000":     `use "kiroku serve"`,
		"-serve":            `use "kiroku serve"`,
		"--json x.json":     `use "kiroku json -o FILE"`,
		"-o x.html":         `use "kiroku html -o FILE"`,
		"--out=x.html":      `use "kiroku html -o FILE"`,
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
	for _, args := range [][]string{{"json", "--root", "x"}, {"doctor", "--root=x"}, {"serve", "-root", "x"}} {
		var err error
		captureOutput(t, func() { err = dispatch(args) })
		if err == nil || err.Error() != "--root has been renamed to --claude-root" {
			t.Errorf("%v: %v", args, err)
		}
	}
}

// kiroku json の形は schemaVersion 1 のあいだ、docs/compatibility.md に書いた項目の名前と型を変えない。
func TestJSONSchema(t *testing.T) {
	setup(t)
	h := filepath.Join("testdata", "home")
	out := filepath.Join(t.TempDir(), "k.json")
	args := []string{"json", "--claude-root", filepath.Join(h, ".claude", "projects"), "--kiro-home", filepath.Join(h, ".kiro"), "--sources", "claude,kiro", "-o", out}
	if err := dispatch(args); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		SchemaVersion *float64         `json:"schemaVersion"`
		Sessions      []map[string]any `json:"sessions"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion == nil || *got.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %v, want 1", got.SchemaVersion)
	}
	if len(got.Sessions) == 0 {
		t.Fatal("セッションがない")
	}
	str := func(v any) bool { _, ok := v.(string); return ok }
	num := func(v any) bool { _, ok := v.(float64); return ok }
	strOrNull := func(v any) bool { return v == nil || str(v) }
	fields := map[string]func(any) bool{
		"id": str, "source": str, "project": str, "projectPath": str, "branch": strOrNull, "title": str,
		"start": num, "end": num, "nPrompts": num, "nFiles": num, "interrupts": num, "corrections": num, "cost": num, "credits": num,
	}
	for _, s := range got.Sessions {
		for k, ok := range fields {
			v, has := s[k]
			if !has || !ok(v) {
				t.Errorf("session %v: %s = %#v (has %v)", s["id"], k, v, has)
			}
		}
	}
}
