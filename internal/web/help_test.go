package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// 画面の「?」の説明（HELP）と、ガイドの「指標の読み方」の表が食い違わないようにする。
func TestHelpMatchesGuide(t *testing.T) {
	guide, err := os.ReadFile("../../docs/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`\{n:"([^"]+)", d:"[^"]+", c:"([^"]+)", x:"([^"]+)", a:"([^"]+)"\}`)
	rows := re.FindAllStringSubmatch(template, -1)
	if len(rows) == 0 {
		t.Fatal("HELP が見つからない")
	}
	for _, r := range rows {
		want := "| " + r[1] + " | " + r[2] + " | " + r[3] + " | " + r[4] + " |"
		if !strings.Contains(string(guide), want) {
			t.Errorf("docs/guide.md の「指標の読み方」に %s の行がない（または画面と違う）", r[1])
		}
	}
}
