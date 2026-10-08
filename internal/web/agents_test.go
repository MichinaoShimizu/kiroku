package web

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/MichinaoShimizu/kiroku/internal/core"
)

// エージェントごとの色や頭文字は core.Agents（__AGENTS__）から取り、画面のスクリプトに表として書かない
// （エージェントを足したりやめたりするときに、直し忘れる場所を作らない）。
func TestNoAgentTablesInScripts(t *testing.T) {
	files := map[string]string{"loading.html": loading}
	all, _ := fs.Glob(jsFiles, "js/*.js")
	for _, p := range all {
		b, _ := jsFiles.ReadFile(p)
		files[p] = string(b)
	}
	for p, body := range files {
		for _, a := range core.Agents {
			if strings.Contains(body, `"`+a.Name+`":`) {
				t.Errorf("%s に %q をキーにした表がある。core.Agents に足して __AGENTS__ から読む", p, a.Name)
			}
		}
	}
	if !strings.Contains(string(Loading), `"name":"Claude Code"`) || strings.Contains(string(Loading), "__AGENTS__") {
		t.Error("loading.html に core.Agents が入っていない")
	}
}

// 使い方のガイドに、どのエージェントの名前も出てくる（エージェントを足したときの書き忘れ）。
func TestGuideNamesEveryAgent(t *testing.T) {
	b, err := os.ReadFile("../../docs/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range core.Agents {
		if !strings.Contains(string(b), a.Name) {
			t.Errorf("docs/guide.md に %q が出てこない", a.Name)
		}
	}
}
