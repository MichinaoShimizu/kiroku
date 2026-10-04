package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
