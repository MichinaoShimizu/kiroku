package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// mdView（AI の返事を整えて見せる）は、要素を作って文字を textContent で入れるだけで、HTML として入れない。
// 返事に HTML・画像・リンクがあっても、文字のまま出る。見出し・箇条書き・表などの形も確かめる。
func TestMdView(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node がないので省略")
	}
	code, err := jsFiles.ReadFile("js/mdview.js")
	if err != nil {
		t.Fatal(err)
	}
	cases, err := os.ReadFile(filepath.Join("testdata", "mdview_test.js"))
	if err != nil {
		t.Fatal(err)
	}
	// 要素だけを持つ document。innerHTML・insertAdjacentHTML・setAttribute は持たせない（使えば落ちる）
	const dom = `const el = tag => ({tag, className: "", children: [], append(...xs){ this.children.push(...xs); },
  set textContent(v){ this.children = [{nodeType: 3, textContent: String(v)}]; }});
const document = {createElement: el, createDocumentFragment: () => el("#frag"), createTextNode: v => ({nodeType: 3, textContent: String(v)})};
`
	f := filepath.Join(t.TempDir(), "mdview.js")
	if err := os.WriteFile(f, append([]byte(dom+string(code)+"\n"), cases...), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, f).CombinedOutput(); err != nil {
		t.Errorf("mdView:\n%s", out)
	}
}
