package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 画面のスクリプトに構文の誤りがないこと（文字列の中に ${} を書いてしまうなど）。
// Node があるときだけ node --check で確かめる（CI のランナーには入っている）。
func TestScriptParses(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	html, err := Render([]any{}, map[string]any{}, map[string]any{}, map[string]any{"report": []any{}, "git": []any{}}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindAllStringSubmatch(html, -1)
	if len(m) == 0 {
		t.Fatal("script が見つからない")
	}
	f := filepath.Join(t.TempDir(), "page.js")
	if err := os.WriteFile(f, []byte(m[len(m)-1][1]), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, "--check", f).CombinedOutput(); err != nil {
		t.Errorf("画面のスクリプトに構文の誤り:\n%s", out)
	}
}

// 書き出す Markdown に入れた履歴の文字が、リンク・強調・HTML・メンションとして働かないこと。
// markdown.js はほかのファイルに頼らないので、それだけを Node で動かして確かめる。
func TestMarkdownEscape(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/markdown.js")
	if err != nil {
		t.Fatal(err)
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "markdown_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "md.js")
	if err := os.WriteFile(f, append(append(src, '\n'), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("Markdown の打ち消しが違う:\n%s", out)
	}
}

// 計測の状態の保存期間の一言（keepRow）。0 日（now）を「わからない期間のあとで消える」や「0 日残す」と出さないこと。
// panels.js はほかのファイルに頼るので、keepRow の関数だけを取り出して Node で動かす。
func TestKeepRow(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/panels.js")
	if err != nil {
		t.Fatal(err)
	}
	// Windows のチェックアウトでは CRLF になり、$ が行末に当たらないので LF にそろえる
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	fn := regexp.MustCompile(`(?ms)^function keepRow\(r\)\{.*?\); \}$`).FindString(text)
	if fn == "" {
		t.Fatal("panels.js に keepRow が見つからない")
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "keeprow_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "keep.js")
	if err := os.WriteFile(f, append([]byte(fn+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("keepRow が違う:\n%s", out)
	}
}

// 目安コストを出せないとき（料金表にないモデルのトークンだけ）に $0.00 と出さないこと。
// format.js は画面の DOM を触るので、costOf の 1 行だけを取り出して Node で動かす。
func TestCostOf(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	src, err := jsFiles.ReadFile("js/format.js")
	if err != nil {
		t.Fatal(err)
	}
	line := regexp.MustCompile(`(?m)^const costOf = .*$`).FindString(string(src))
	if line == "" {
		t.Fatal("format.js に costOf が見つからない")
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "costof_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "cost.js")
	if err := os.WriteFile(f, append([]byte(line+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("costOf が違う:\n%s", out)
	}
}
